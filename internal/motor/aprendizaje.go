package motor

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Error ocurrido en un punto de entrega (el ROJO del TDD no es error). docs/diseno/06-tdd-y-evals.md §4.
type Error struct {
	ID           string `json:"id"`
	Cuando       string `json:"cuando"`
	Que          string `json:"que"`
	Fuente       string `json:"fuente"`
	Severidad    string `json:"severidad"`
	Causa        string `json:"causa"`
	Clase        string `json:"clase"`
	Ambito       string `json:"ambito"`
	Patron       string `json:"patron"`
	Tarea        string `json:"tarea"`
	Agente       string `json:"agente"`
	Rol          string `json:"rol"`
	DetectadoPor string `json:"detectado_por"`
	Leccion      string `json:"leccion"`
}

// Clase agrupa errores recurrentes (deduplicación).
type Clase struct {
	ID             string
	Causa          string
	Ambito         string
	Patron         string
	Recurrencia    int
	RequiereSensor bool
}

// Leccion es una skill nacida de una clase de error, con ciclo de vida.
type Leccion struct {
	ID           string `json:"id"`
	Clase        string `json:"clase"`
	Skill        string `json:"skill"`
	Descripcion  string `json:"descripcion"`
	Estado       string `json:"estado"`
	Ambito       string `json:"ambito,omitempty"`
	Motivo       string `json:"motivo,omitempty"`
	Reescrituras int    `json:"reescrituras"`
	Creada       string `json:"creada"`
}

// EventoRedteam es el resultado de un caso de red-team sobre una regla dura.
type EventoRedteam struct {
	Regla     string `json:"regla"`
	Caso      string `json:"caso"`
	Resultado string `json:"resultado"`
	Capa      string `json:"capa"`
	Detalle   string `json:"detalle"`
	En        string `json:"en"`
}

var (
	fuenteValida    = map[string]bool{"verificacion": true, "evaluador": true, "humano": true, "regla_dura": true, "revert": true, "regresion": true}
	severidadValida = map[string]bool{"critico": true, "mayor": true, "menor": true}
	causaValida     = map[string]bool{"contrato_ambiguo": true, "contexto_faltante": true, "regla_ignorada": true, "patron_incorrecto": true,
		"herramienta_mal_usada": true, "test_debil": true, "sobreajuste": true, "otra": true}
	tipoEval      = map[string]bool{"prevencion": true, "interferencia": true, "activacion": true}
	resultadoEval = map[string]bool{"pasa": true, "falla": true}
	resultadoRT   = map[string]bool{"bloqueado": true, "paso": true}
	estadoLeccion = map[string]bool{"en_prueba": true, "activa": true, "reescrita": true, "eliminada": true, "podada": true}
)

func campos(faltan []string) error {
	if len(faltan) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrCampos, strings.Join(faltan, ", "))
}

func siguienteID(tx *sql.Tx, tabla, prefijo string) (string, error) {
	var n int
	err := tx.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(id, ?) AS INTEGER)), 0) + 1 FROM `+tabla, len(prefijo)+1).Scan(&n)
	return fmt.Sprintf("%s%04d", prefijo, n), err
}

// ───────────────────────── errores ─────────────────────────

func aplicarError(tx *sql.Tx, e Error) (int, error) {
	if e.Ambito == "" {
		e.Ambito = "**/*"
	}
	if _, err := tx.Exec(`INSERT INTO clases_error (id, causa_raiz, ambito, patron, recurrencia) VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(id) DO UPDATE SET recurrencia = recurrencia + 1`, e.Clase, e.Causa, e.Ambito, e.Patron); err != nil {
		return 0, err
	}
	detectado := e.DetectadoPor
	if _, err := tx.Exec(`INSERT INTO errores (id, cuando, agente_id, rol, tarea_id, que_hizo, fuente, detectado_por, clase_id,
		severidad, leccion_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, e.ID, e.Cuando, nulo(e.Agente), nulo(e.Rol), nulo(e.Tarea),
		e.Que, e.Fuente, nulo(detectado), e.Clase, e.Severidad, nulo(e.Leccion)); err != nil {
		return 0, err
	}
	var rec int
	err := tx.QueryRow(`SELECT recurrencia FROM clases_error WHERE id = ?`, e.Clase).Scan(&rec)
	return rec, err
}

// NuevoError valida, deduplica por clase y registra; devuelve el ID y la recurrencia de su clase.
func (m *Motor) NuevoError(e Error) (string, int, error) {
	var faltan []string
	if strings.TrimSpace(e.Que) == "" {
		faltan = append(faltan, "que")
	}
	if !fuenteValida[e.Fuente] {
		faltan = append(faltan, "fuente (verificacion|evaluador|humano|regla_dura|revert|regresion; el ROJO del TDD no es error)")
	}
	if !severidadValida[e.Severidad] {
		faltan = append(faltan, "severidad (critico|mayor|menor)")
	}
	if !causaValida[e.Causa] {
		faltan = append(faltan, "causa")
	}
	if strings.TrimSpace(e.Clase) == "" {
		faltan = append(faltan, "clase")
	}
	if err := campos(faltan); err != nil {
		return "", 0, err
	}
	var rec int
	err := m.escribir("errores.jsonl", "error.nuevo", func(tx *sql.Tx) (any, error) {
		id, err := siguienteID(tx, "errores", "E-")
		if err != nil {
			return nil, err
		}
		e.ID, e.Cuando = id, ahora()
		rec, err = aplicarError(tx, e)
		return e, err
	})
	return e.ID, rec, err
}

// Clase devuelve una clase de error.
func (m *Motor) Clase(id string) (Clase, error) {
	var c Clase
	err := m.db.QueryRow(`SELECT id, causa_raiz, ambito, patron, recurrencia, requiere_sensor FROM clases_error WHERE id = ?`, id).
		Scan(&c.ID, &c.Causa, &c.Ambito, &c.Patron, &c.Recurrencia, &c.RequiereSensor)
	if errors.Is(err, sql.ErrNoRows) {
		return c, fmt.Errorf("clase %s: %w", id, ErrNoExiste)
	}
	return c, err
}

// Errores lista errores, opcionalmente de una clase.
func (m *Motor) Errores(clase string) ([]Error, error) {
	q := `SELECT e.id, e.cuando, e.que_hizo, e.fuente, e.severidad, c.causa_raiz, e.clase_id, c.ambito, c.patron,
		COALESCE(e.tarea_id,''), COALESCE(e.agente_id,''), COALESCE(e.rol,''), COALESCE(e.detectado_por,''), COALESCE(e.leccion_id,'')
		FROM errores e JOIN clases_error c ON c.id = e.clase_id`
	var args []any
	if clase != "" {
		q += ` WHERE e.clase_id = ?`
		args = append(args, clase)
	}
	filas, err := m.db.Query(q+` ORDER BY CAST(SUBSTR(e.id, 3) AS INTEGER)`, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Error
	for filas.Next() {
		var e Error
		if err := filas.Scan(&e.ID, &e.Cuando, &e.Que, &e.Fuente, &e.Severidad, &e.Causa, &e.Clase, &e.Ambito, &e.Patron,
			&e.Tarea, &e.Agente, &e.Rol, &e.DetectadoPor, &e.Leccion); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, filas.Err()
}

// ───────────────────────── lecciones ─────────────────────────

type eventoEstadoLeccion struct {
	ID     string `json:"id"`
	Estado string `json:"estado"`
	Motivo string `json:"motivo"`
	En     string `json:"en"`
}

func aplicarLeccion(tx *sql.Tx, l Leccion) error {
	_, err := tx.Exec(`INSERT INTO lecciones (id, clase_id, skill_ruta, descripcion, estado, reescrituras, creada_en, actualizada_en)
		VALUES (?, ?, ?, ?, 'en_prueba', 0, ?, ?)`, l.ID, l.Clase, l.Skill, l.Descripcion, l.Creada, l.Creada)
	return err
}

func aplicarEstadoLeccion(tx *sql.Tx, e eventoEstadoLeccion) error {
	suma := 0
	if e.Estado == "reescrita" {
		suma = 1
	}
	_, err := tx.Exec(`UPDATE lecciones SET estado = ?, motivo_estado = ?, actualizada_en = ?, reescrituras = reescrituras + ?
		WHERE id = ?`, e.Estado, e.Motivo, e.En, suma, e.ID)
	return err
}

// NuevaLeccion registra una lección en_prueba para una clase de error existente.
func (m *Motor) NuevaLeccion(l Leccion) (string, error) {
	var faltan []string
	if strings.TrimSpace(l.Clase) == "" {
		faltan = append(faltan, "clase")
	}
	if strings.TrimSpace(l.Skill) == "" {
		faltan = append(faltan, "skill")
	}
	if err := campos(faltan); err != nil {
		return "", err
	}
	err := m.escribir("lecciones.jsonl", "leccion.nueva", func(tx *sql.Tx) (any, error) {
		id, err := siguienteID(tx, "lecciones", "L-")
		if err != nil {
			return nil, err
		}
		l.ID, l.Creada, l.Estado = id, ahora(), "en_prueba"
		return l, aplicarLeccion(tx, l)
	})
	return l.ID, err
}

// FijarEstadoLeccion registra un cambio de estado (las reglas de transición viven en el paquete aprendizaje).
func (m *Motor) FijarEstadoLeccion(id, estado, motivo string) error {
	if !estadoLeccion[estado] {
		return fmt.Errorf("%w: estado de lección %q", ErrCampos, estado)
	}
	if _, err := m.Leccion(id); err != nil {
		return err
	}
	e := eventoEstadoLeccion{ID: strings.ToUpper(id), Estado: estado, Motivo: motivo, En: ahora()}
	return m.escribir("lecciones.jsonl", "leccion.estado", func(tx *sql.Tx) (any, error) {
		return e, aplicarEstadoLeccion(tx, e)
	})
}

const columnasLeccion = `l.id, l.clase_id, l.skill_ruta, COALESCE(l.descripcion,''), l.estado, COALESCE(c.ambito,'**/*'),
	COALESCE(l.motivo_estado,''), l.reescrituras, l.creada_en`

func (m *Motor) consultarLecciones(filtro string, args ...any) ([]Leccion, error) {
	filas, err := m.db.Query(`SELECT `+columnasLeccion+` FROM lecciones l LEFT JOIN clases_error c ON c.id = l.clase_id `+
		filtro+` ORDER BY CAST(SUBSTR(l.id, 3) AS INTEGER)`, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Leccion
	for filas.Next() {
		var l Leccion
		if err := filas.Scan(&l.ID, &l.Clase, &l.Skill, &l.Descripcion, &l.Estado, &l.Ambito, &l.Motivo, &l.Reescrituras, &l.Creada); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, filas.Err()
}

// Leccion devuelve una lección por ID.
func (m *Motor) Leccion(id string) (Leccion, error) {
	ls, err := m.consultarLecciones(`WHERE l.id = ?`, strings.ToUpper(id))
	if err != nil {
		return Leccion{}, err
	}
	if len(ls) == 0 {
		return Leccion{}, fmt.Errorf("lección %s: %w", id, ErrNoExiste)
	}
	return ls[0], nil
}

// Lecciones lista las lecciones, opcionalmente por estado.
func (m *Motor) Lecciones(estado string) ([]Leccion, error) {
	if estado == "" {
		return m.consultarLecciones(``)
	}
	return m.consultarLecciones(`WHERE l.estado = ?`, estado)
}

// LeccionesActivas devuelve las lecciones activas con el ámbito de su clase.
func (m *Motor) LeccionesActivas() ([]Leccion, error) { return m.Lecciones("activa") }

// ───────────────────────── evals ─────────────────────────

type eventoEval struct {
	Leccion   string  `json:"leccion"`
	Tipo      string  `json:"tipo"`
	Resultado string  `json:"resultado"`
	Costo     float64 `json:"costo_usd"`
	Detalle   string  `json:"detalle"`
	En        string  `json:"en"`
}

func aplicarEval(tx *sql.Tx, e eventoEval) error {
	id := e.Leccion + ":" + e.Tipo
	if _, err := tx.Exec(`INSERT INTO evals (id, objeto_tipo, objeto_id, tipo) VALUES (?, 'leccion', ?, ?)
		ON CONFLICT(id) DO NOTHING`, id, e.Leccion, e.Tipo); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO eval_runs (eval_id, resultado, costo_usd, detalle, corrido_en) VALUES (?, ?, ?, ?, ?)`,
		id, e.Resultado, e.Costo, e.Detalle, e.En)
	return err
}

// RegistrarEval guarda una corrida de eval de una lección.
func (m *Motor) RegistrarEval(id, tipo, resultado string, costo float64, detalle string) error {
	var faltan []string
	if !tipoEval[tipo] {
		faltan = append(faltan, "tipo (prevencion|interferencia|activacion)")
	}
	if !resultadoEval[resultado] {
		faltan = append(faltan, "resultado (pasa|falla)")
	}
	if err := campos(faltan); err != nil {
		return err
	}
	if _, err := m.Leccion(id); err != nil {
		return err
	}
	e := eventoEval{Leccion: strings.ToUpper(id), Tipo: tipo, Resultado: resultado, Costo: costo, Detalle: detalle, En: ahora()}
	return m.escribir("evals.jsonl", "eval.run", func(tx *sql.Tx) (any, error) { return e, aplicarEval(tx, e) })
}

// UltimosEvals devuelve el último resultado de cada tipo de eval de una lección.
func (m *Motor) UltimosEvals(id string) (map[string]string, error) {
	filas, err := m.db.Query(`SELECT e.tipo, r.resultado FROM eval_runs r JOIN evals e ON e.id = r.eval_id
		WHERE e.objeto_tipo = 'leccion' AND e.objeto_id = ? ORDER BY r.id`, strings.ToUpper(id))
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := map[string]string{}
	for filas.Next() {
		var tipo, res string
		if err := filas.Scan(&tipo, &res); err != nil {
			return nil, err
		}
		out[tipo] = res
	}
	return out, filas.Err()
}

// ErroresCriticosDeLeccion cuenta los errores críticos atribuidos a una lección.
func (m *Motor) ErroresCriticosDeLeccion(id string) (int, error) {
	var n int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM errores WHERE leccion_id = ? AND severidad = 'critico'`, strings.ToUpper(id)).Scan(&n)
	return n, err
}

// ───────────────────────── verificadores (meta-evals) ─────────────────────────

type eventoMedicion struct {
	ID        string  `json:"id"`
	Deteccion float64 `json:"deteccion"`
	Detalle   string  `json:"detalle"`
	En        string  `json:"en"`
}

func aplicarMedicion(tx *sql.Tx, e eventoMedicion) error {
	_, err := tx.Exec(`INSERT INTO verificadores (id, nivel, comando, deteccion, evaluado_en) VALUES (?, 'V1', ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET comando = excluded.comando, deteccion = excluded.deteccion, evaluado_en = excluded.evaluado_en`,
		e.ID, e.Detalle, e.Deteccion, e.En)
	return err
}

// RegistrarMedicion guarda la tasa de detección de un verificador (p. ej. mutation testing).
func (m *Motor) RegistrarMedicion(id string, deteccion float64, detalle string) error {
	e := eventoMedicion{ID: id, Deteccion: deteccion, Detalle: detalle, En: ahora()}
	return m.escribir("verificadores.jsonl", "verificador.medicion", func(tx *sql.Tx) (any, error) { return e, aplicarMedicion(tx, e) })
}

// Medicion devuelve la última tasa de detección de un verificador.
func (m *Motor) Medicion(id string) (float64, error) {
	var v float64
	err := m.db.QueryRow(`SELECT deteccion FROM verificadores WHERE id = ?`, id).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("verificador %s: %w", id, ErrNoExiste)
	}
	return v, err
}

// ───────────────────────── red-team ─────────────────────────

func aplicarRedteam(tx *sql.Tx, e EventoRedteam) error {
	_, err := tx.Exec(`INSERT INTO regla_eventos (regla_id, tipo, resultado, capa, caso_id, detalle, ocurrido_en)
		VALUES (?, 'red_team', ?, ?, ?, ?, ?)`, e.Regla, e.Resultado, nulo(e.Capa), e.Caso, e.Detalle, e.En)
	return err
}

// RegistrarRedteam guarda el resultado de un caso de red-team.
func (m *Motor) RegistrarRedteam(regla, caso, resultado, capa, detalle string) error {
	var faltan []string
	if strings.TrimSpace(regla) == "" || strings.TrimSpace(caso) == "" {
		faltan = append(faltan, "regla y caso")
	}
	if !resultadoRT[resultado] {
		faltan = append(faltan, "resultado (bloqueado|paso)")
	}
	if err := campos(faltan); err != nil {
		return err
	}
	e := EventoRedteam{Regla: strings.ToUpper(regla), Caso: caso, Resultado: resultado, Capa: capa, Detalle: detalle, En: ahora()}
	return m.escribir("redteam.jsonl", "redteam.resultado", func(tx *sql.Tx) (any, error) { return e, aplicarRedteam(tx, e) })
}

// EventosRedteam devuelve los resultados de red-team (de una regla, o de todas).
func (m *Motor) EventosRedteam(regla string) ([]EventoRedteam, error) {
	q := `SELECT regla_id, COALESCE(caso_id,''), COALESCE(resultado,''), COALESCE(capa,''), COALESCE(detalle,''), ocurrido_en
		FROM regla_eventos WHERE tipo = 'red_team'`
	var args []any
	if regla != "" {
		q += ` AND regla_id = ?`
		args = append(args, strings.ToUpper(regla))
	}
	filas, err := m.db.Query(q+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []EventoRedteam
	for filas.Next() {
		var e EventoRedteam
		if err := filas.Scan(&e.Regla, &e.Caso, &e.Resultado, &e.Capa, &e.Detalle, &e.En); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, filas.Err()
}

// MedicionVerificador es la última medición de un verificador.
type MedicionVerificador struct {
	ID        string  `json:"id"`
	Deteccion float64 `json:"deteccion"`
	Detalle   string  `json:"detalle"`
}

// MedicionesConPrefijo devuelve la última medición de cada verificador cuyo id empieza con el prefijo,
// ordenadas por id (p. ej. "mutacion:T-1:" → la meta-eval de cada archivo de la tarea).
func (m *Motor) MedicionesConPrefijo(prefijo string) ([]MedicionVerificador, error) {
	filas, err := m.db.Query(`SELECT id, deteccion, COALESCE(comando, '') FROM verificadores
		WHERE substr(id, 1, length(?)) = ? ORDER BY id`, prefijo, prefijo)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []MedicionVerificador{}
	for filas.Next() {
		var x MedicionVerificador
		if err := filas.Scan(&x.ID, &x.Deteccion, &x.Detalle); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, filas.Err()
}
