package motor

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Mision registrada en el motor (la definición vive en .harness/misiones/<id>/mision.yaml).
type Mision struct {
	ID       string `json:"id"`
	Titulo   string `json:"titulo"`
	Modo     string `json:"modo"`
	Objetivo string `json:"objetivo"`
	Estado   string `json:"estado"`
	Creada   string `json:"creada"`
}

// Ronda de mejora: R-n por misión.
type Ronda struct {
	ID        string `json:"id"`
	Mision    string `json:"mision"`
	Numero    int    `json:"numero"`
	Foco      string `json:"foco"`
	Veredicto string `json:"veredicto"`
	Iniciada  string `json:"iniciada"`
	Cerrada   string `json:"cerrada"`
}

// Medicion de una métrica en una ronda (los ítems quedan en Ruta).
type Medicion struct {
	ID       string  `json:"id"`
	Mision   string  `json:"mision"`
	Ronda    string  `json:"ronda"`
	Metrica  string  `json:"metrica"`
	Conjunto string  `json:"conjunto"`
	Valor    float64 `json:"valor"`
	N        int     `json:"n"`
	Ruta     string  `json:"ruta"`
	En       string  `json:"en"`
}

// Milestone: ronda destacada que define el humano (D-0019).
type Milestone struct {
	ID        string `json:"id"`
	Mision    string `json:"mision"`
	Ronda     string `json:"ronda"`
	Promocion string `json:"promocion"`
	Resumen   string `json:"resumen"`
	Creado    string `json:"creado"`
}

type eventoMision struct {
	Mision    string   `json:"mision"`
	Ronda     string   `json:"ronda,omitempty"`
	Numero    int      `json:"numero,omitempty"`
	Foco      []string `json:"foco,omitempty"`
	Veredicto string   `json:"veredicto,omitempty"`
	Metrica   string   `json:"metrica,omitempty"`
	Valor     float64  `json:"valor,omitempty"`
	Concepto  string   `json:"concepto,omitempty"`
	Criterio  string   `json:"criterio,omitempty"`
	Evidencia string   `json:"evidencia,omitempty"`
	En        string   `json:"en"`
}

const logMisiones = "misiones.jsonl"

var conjuntoValido = map[string]bool{"trabajo": true, "holdout": true, "ruido": true}

// idRonda es la clave interna (las rondas se numeran por misión).
func idRonda(mision, ronda string) string { return mision + "/" + strings.ToUpper(ronda) }

func aplicarMisionNueva(tx *sql.Tx, x Mision) error {
	_, err := tx.Exec(`INSERT INTO misiones (id, titulo, modo, objetivo, estado, creada_en) VALUES (?, ?, ?, ?, 'activa', ?)`,
		x.ID, x.Titulo, x.Modo, x.Objetivo, x.Creada)
	return err
}

func aplicarEventoMision(tx *sql.Tx, evento string, e eventoMision) error {
	switch evento {
	case "mision.cerrar":
		_, err := tx.Exec(`UPDATE misiones SET estado = 'cerrada', cerrada_en = ? WHERE id = ?`, e.En, e.Mision)
		return err
	case "ronda.nueva":
		_, err := tx.Exec(`INSERT INTO rondas (id, mision_id, numero, foco, iniciada_en) VALUES (?, ?, ?, ?, ?)`,
			idRonda(e.Mision, fmt.Sprintf("R-%d", e.Numero)), e.Mision, e.Numero, jsonTexto(e.Foco), e.En)
		return err
	case "ronda.cerrar":
		_, err := tx.Exec(`UPDATE rondas SET veredicto = ?, cerrada_en = ? WHERE id = ?`, e.Veredicto, e.En, idRonda(e.Mision, e.Ronda))
		return err
	case "gasto.nuevo":
		_, err := tx.Exec(`INSERT INTO gastos (mision_id, usd, concepto, en) VALUES (?, ?, ?, ?)`, e.Mision, e.Valor, e.Concepto, e.En)
		return err
	case "aceptacion.nueva":
		_, err := tx.Exec(`INSERT INTO aceptaciones (mision_id, criterio, evidencia, en) VALUES (?, ?, ?, ?)
			ON CONFLICT(mision_id, criterio) DO UPDATE SET evidencia = excluded.evidencia, en = excluded.en`, e.Mision, e.Criterio, e.Evidencia, e.En)
		return err
	case "piso.nuevo":
		_, err := tx.Exec(`INSERT INTO pisos (mision_id, metrica_id, piso, en) VALUES (?, ?, ?, ?)
			ON CONFLICT(mision_id, metrica_id) DO UPDATE SET piso = excluded.piso, en = excluded.en`, e.Mision, e.Metrica, e.Valor, e.En)
		return err
	}
	return fmt.Errorf("evento de misión desconocido %q", evento)
}

func aplicarMedicionRonda(tx *sql.Tx, x Medicion) error {
	_, err := tx.Exec(`INSERT INTO mediciones (id, ronda_id, metrica_id, conjunto, valor, n_items, resultados, medido_en)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, x.ID, idRonda(x.Mision, x.Ronda), x.Metrica, x.Conjunto, x.Valor, x.N, x.Ruta, x.En)
	return err
}

func aplicarMilestone(tx *sql.Tx, x Milestone) error {
	_, err := tx.Exec(`INSERT INTO milestones (id, mision_id, ronda_id, promocion, resumen, creado_en) VALUES (?, ?, ?, ?, ?, ?)`,
		x.ID, x.Mision, idRonda(x.Mision, x.Ronda), x.Promocion, x.Resumen, x.Creado)
	return err
}

func (m *Motor) existeMision(id string) error {
	var n int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM misiones WHERE id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("misión %s: %w", id, ErrNoExiste)
	}
	return nil
}

func (m *Motor) eventoMision(evento string, e eventoMision) error {
	if err := m.existeMision(e.Mision); err != nil {
		return err
	}
	e.En = ahora()
	return m.escribir(logMisiones, evento, func(tx *sql.Tx) (any, error) { return e, aplicarEventoMision(tx, evento, e) })
}

// NuevaMision registra la misión (su definición se escribe aparte en mision.yaml).
func (m *Motor) NuevaMision(x Mision) error {
	if strings.TrimSpace(x.ID) == "" || (x.Modo != "genesis" && x.Modo != "mejora") {
		return fmt.Errorf("%w: id y modo (genesis|mejora)", ErrCampos)
	}
	if err := m.existeMision(x.ID); err == nil {
		return fmt.Errorf("%w: la misión %s ya existe", ErrCampos, x.ID)
	}
	x.Creada, x.Estado = ahora(), "activa"
	return m.escribir(logMisiones, "mision.nueva", func(tx *sql.Tx) (any, error) { return x, aplicarMisionNueva(tx, x) })
}

// Misiones lista las misiones registradas.
func (m *Motor) Misiones() ([]Mision, error) {
	filas, err := m.db.Query(`SELECT id, titulo, modo, objetivo, estado, creada_en FROM misiones ORDER BY creada_en, id`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Mision
	for filas.Next() {
		var x Mision
		if err := filas.Scan(&x.ID, &x.Titulo, &x.Modo, &x.Objetivo, &x.Estado, &x.Creada); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// CerrarMision marca la misión como cerrada (el contrato se valida en el CLI).
func (m *Motor) CerrarMision(id string) error {
	return m.eventoMision("mision.cerrar", eventoMision{Mision: id})
}

// NuevaRonda abre la siguiente ronda de la misión.
func (m *Motor) NuevaRonda(mision string, foco []string) (string, error) {
	if err := m.existeMision(mision); err != nil {
		return "", err
	}
	e := eventoMision{Mision: mision, Foco: foco}
	err := m.escribir(logMisiones, "ronda.nueva", func(tx *sql.Tx) (any, error) {
		if err := tx.QueryRow(`SELECT COALESCE(MAX(numero), 0) + 1 FROM rondas WHERE mision_id = ?`, mision).Scan(&e.Numero); err != nil {
			return nil, err
		}
		e.En = ahora()
		return e, aplicarEventoMision(tx, "ronda.nueva", e)
	})
	return fmt.Sprintf("R-%d", e.Numero), err
}

// CerrarRonda registra la decisión de la ronda (mantener o revertir).
func (m *Motor) CerrarRonda(mision, ronda, veredicto string) error {
	if veredicto != "mantener" && veredicto != "revertir" {
		return fmt.Errorf("%w: veredicto (mantener|revertir)", ErrCampos)
	}
	return m.eventoMision("ronda.cerrar", eventoMision{Mision: mision, Ronda: strings.ToUpper(ronda), Veredicto: veredicto})
}

// Rondas lista las rondas de una misión.
func (m *Motor) Rondas(mision string) ([]Ronda, error) {
	filas, err := m.db.Query(`SELECT numero, COALESCE(foco,'[]'), COALESCE(veredicto,''), iniciada_en, COALESCE(cerrada_en,'')
		FROM rondas WHERE mision_id = ? ORDER BY numero`, mision)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Ronda
	for filas.Next() {
		r := Ronda{Mision: mision}
		if err := filas.Scan(&r.Numero, &r.Foco, &r.Veredicto, &r.Iniciada, &r.Cerrada); err != nil {
			return nil, err
		}
		r.ID = fmt.Sprintf("R-%d", r.Numero)
		out = append(out, r)
	}
	return out, filas.Err()
}

// RegistrarMedicionRonda guarda el valor de una métrica en una ronda.
func (m *Motor) RegistrarMedicionRonda(x Medicion) error {
	if x.Conjunto == "" {
		x.Conjunto = "trabajo"
	}
	if !conjuntoValido[x.Conjunto] || x.Metrica == "" || x.Ronda == "" {
		return fmt.Errorf("%w: ronda, métrica y conjunto (trabajo|holdout|ruido)", ErrCampos)
	}
	if err := m.existeMision(x.Mision); err != nil {
		return err
	}
	x.Ronda = strings.ToUpper(x.Ronda)
	return m.escribir(logMisiones, "medicion.nueva", func(tx *sql.Tx) (any, error) {
		id, err := siguienteID(tx, "mediciones", "M-")
		if err != nil {
			return nil, err
		}
		x.ID, x.En = id, ahora()
		return x, aplicarMedicionRonda(tx, x)
	})
}

// Mediciones devuelve la serie de mediciones de una misión en un conjunto, en orden de ronda.
func (m *Motor) Mediciones(mision, conjunto string) ([]Medicion, error) {
	filas, err := m.db.Query(`SELECT md.id, r.numero, md.metrica_id, md.conjunto, md.valor, md.n_items, COALESCE(md.resultados,''), md.medido_en
		FROM mediciones md JOIN rondas r ON r.id = md.ronda_id
		WHERE r.mision_id = ? AND md.conjunto = ? ORDER BY r.numero, md.metrica_id, md.id`, mision, conjunto)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Medicion
	for filas.Next() {
		x := Medicion{Mision: mision}
		var numero int
		if err := filas.Scan(&x.ID, &numero, &x.Metrica, &x.Conjunto, &x.Valor, &x.N, &x.Ruta, &x.En); err != nil {
			return nil, err
		}
		x.Ronda = fmt.Sprintf("R-%d", numero)
		out = append(out, x)
	}
	return out, filas.Err()
}

// RegistrarGasto acumula gasto de la misión (FinOps por misión).
func (m *Motor) RegistrarGasto(mision string, usd float64, concepto string) error {
	if usd <= 0 {
		return fmt.Errorf("%w: usd > 0", ErrCampos)
	}
	return m.eventoMision("gasto.nuevo", eventoMision{Mision: mision, Valor: usd, Concepto: concepto})
}

// Gasto total de la misión.
func (m *Motor) Gasto(mision string) (float64, error) {
	var g float64
	err := m.db.QueryRow(`SELECT COALESCE(SUM(usd), 0) FROM gastos WHERE mision_id = ?`, mision).Scan(&g)
	return g, err
}

// Aceptar marca un criterio del contrato con su evidencia.
func (m *Motor) Aceptar(mision, criterio, evidencia string) error {
	if strings.TrimSpace(criterio) == "" || strings.TrimSpace(evidencia) == "" {
		return fmt.Errorf("%w: criterio y evidencia (comando → resultado)", ErrCampos)
	}
	return m.eventoMision("aceptacion.nueva", eventoMision{Mision: mision, Criterio: strings.ToUpper(criterio), Evidencia: evidencia})
}

// Aceptaciones devuelve criterio → evidencia.
func (m *Motor) Aceptaciones(mision string) (map[string]string, error) {
	filas, err := m.db.Query(`SELECT criterio, evidencia FROM aceptaciones WHERE mision_id = ?`, mision)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := map[string]string{}
	for filas.Next() {
		var c, e string
		if err := filas.Scan(&c, &e); err != nil {
			return nil, err
		}
		out[c] = e
	}
	return out, filas.Err()
}

// FijarPiso registra el piso de ruido medido de una métrica.
func (m *Motor) FijarPiso(mision, metrica string, piso float64) error {
	return m.eventoMision("piso.nuevo", eventoMision{Mision: mision, Metrica: metrica, Valor: piso})
}

// Piso devuelve el piso de ruido medido (ErrNoExiste si nunca se midió).
func (m *Motor) Piso(mision, metrica string) (float64, error) {
	var p float64
	err := m.db.QueryRow(`SELECT piso FROM pisos WHERE mision_id = ? AND metrica_id = ?`, mision, metrica).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("piso de %s: %w", metrica, ErrNoExiste)
	}
	return p, err
}

// MarcarMilestone registra un milestone (lo decide el humano; `propuesta` indica si el harness lo había propuesto).
func (m *Motor) MarcarMilestone(mision, ronda, resumen string, propuesta bool) (string, error) {
	if strings.TrimSpace(resumen) == "" {
		return "", fmt.Errorf("%w: resumen de una línea (qué cambió)", ErrCampos)
	}
	if err := m.existeMision(mision); err != nil {
		return "", err
	}
	x := Milestone{Mision: mision, Ronda: strings.ToUpper(ronda), Resumen: resumen, Promocion: "manual"}
	if propuesta {
		x.Promocion = "propuesta_aceptada"
	}
	err := m.escribir(logMisiones, "milestone.nuevo", func(tx *sql.Tx) (any, error) {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM rondas WHERE id = ?`, idRonda(mision, x.Ronda)).Scan(&n); err != nil || n == 0 {
			return nil, fmt.Errorf("ronda %s de %s: %w", x.Ronda, mision, ErrNoExiste)
		}
		// IDs cortos (MS-1, MS-2…): son pocos y se citan en informes
		var siguiente int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(id, 4) AS INTEGER)), 0) + 1 FROM milestones`).Scan(&siguiente); err != nil {
			return nil, err
		}
		x.ID, x.Creado = fmt.Sprintf("MS-%d", siguiente), ahora()
		return x, aplicarMilestone(tx, x)
	})
	return x.ID, err
}

// Milestones de la misión, en orden.
func (m *Motor) Milestones(mision string) ([]Milestone, error) {
	filas, err := m.db.Query(`SELECT ms.id, r.numero, ms.promocion, ms.resumen, ms.creado_en FROM milestones ms
		JOIN rondas r ON r.id = ms.ronda_id WHERE ms.mision_id = ? ORDER BY CAST(SUBSTR(ms.id, 4) AS INTEGER)`, mision)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Milestone
	for filas.Next() {
		x := Milestone{Mision: mision}
		var numero int
		if err := filas.Scan(&x.ID, &numero, &x.Promocion, &x.Resumen, &x.Creado); err != nil {
			return nil, err
		}
		x.Ronda = fmt.Sprintf("R-%d", numero)
		out = append(out, x)
	}
	return out, filas.Err()
}
