package motor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Alternativa considerada y por qué se descartó.
type Alternativa struct {
	Opcion        string `json:"opcion"`
	DescartadaPor string `json:"descartada_por"`
}

// Decision registra cuándo, qué y por qué (RG-3). Las etiquetas JSON coinciden con el log.
type Decision struct {
	ID             string        `json:"id"`
	Cuando         string        `json:"cuando"`
	MisionID       string        `json:"mision_id"`
	RondaOHito     string        `json:"ronda_o_hito"`
	Sesion         string        `json:"sesion"`
	CommitRef      string        `json:"commit_ref"`
	Que            string        `json:"que"`
	PorQue         string        `json:"por_que"`
	Alternativas   []Alternativa `json:"alternativas"`
	Decidio        string        `json:"decidio"`
	Alcance        []string      `json:"alcance"`
	Reversible     bool          `json:"reversible"`
	Estado         string        `json:"estado"`
	ReemplazadaPor string        `json:"reemplazada_por"`
	Etiquetas      []string      `json:"etiquetas"`
}

var decidioValido = map[string]bool{"humano": true, "agente": true, "agente_aprobado": true}

// columnasDecision arma la lista de columnas con un prefijo de tabla opcional ("" o "d.").
func columnasDecision(p string) string {
	return fmt.Sprintf(`%[1]sid, %[1]scuando, COALESCE(%[1]smision_id,''), COALESCE(%[1]sronda_o_hito,''),
	COALESCE(%[1]ssesion,''), COALESCE(%[1]scommit_ref,''), %[1]sque, %[1]spor_que, %[1]salternativas, %[1]sdecidio,
	%[1]salcance, %[1]sreversible, %[1]sestado, COALESCE(%[1]sreemplazada_por,''), %[1]setiquetas`, p)
}

func validarDecision(d Decision) error {
	var faltan []string
	if strings.TrimSpace(d.Que) == "" {
		faltan = append(faltan, "que")
	}
	if strings.TrimSpace(d.PorQue) == "" {
		faltan = append(faltan, "por_que")
	}
	if !decidioValido[d.Decidio] {
		faltan = append(faltan, "decidio (humano|agente|agente_aprobado)")
	}
	if len(faltan) > 0 {
		return fmt.Errorf("%w: %s", ErrCampos, strings.Join(faltan, ", "))
	}
	return nil
}

func normalizar(d *Decision) {
	if d.Alternativas == nil {
		d.Alternativas = []Alternativa{}
	}
	if d.Alcance == nil {
		d.Alcance = []string{}
	}
	if d.Etiquetas == nil {
		d.Etiquetas = []string{}
	}
	if d.Estado == "" {
		d.Estado = "vigente"
	}
}

func insertarDecision(stDec, stFTS *sql.Stmt, d Decision) error {
	normalizar(&d)
	if _, err := stDec.Exec(d.ID, d.Cuando, nulo(d.MisionID), nulo(d.RondaOHito), nulo(d.Sesion), nulo(d.CommitRef),
		d.Que, d.PorQue, jsonTexto(d.Alternativas), d.Decidio, jsonTexto(d.Alcance), d.Reversible, d.Estado,
		nulo(d.ReemplazadaPor), jsonTexto(d.Etiquetas)); err != nil {
		return fmt.Errorf("decisión %s: %w", d.ID, err)
	}
	opciones := make([]string, 0, len(d.Alternativas))
	for _, a := range d.Alternativas {
		opciones = append(opciones, a.Opcion+" "+a.DescartadaPor)
	}
	_, err := stFTS.Exec(d.ID, d.Que, d.PorQue, strings.Join(d.Etiquetas, " "), strings.Join(opciones, " "))
	return err
}

func nulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// NuevaDecision valida, asigna el siguiente D-NNNN, indexa y anexa al log.
func (m *Motor) NuevaDecision(d Decision) (string, error) {
	if err := validarDecision(d); err != nil {
		return "", err
	}
	err := m.escribir("decisiones.jsonl", "decision.nueva", func(tx *sql.Tx) (any, error) {
		var n int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(id, 3) AS INTEGER)), 0) + 1 FROM decisiones`).Scan(&n); err != nil {
			return nil, err
		}
		d.ID = fmt.Sprintf("D-%04d", n)
		d.Cuando = ahora()
		normalizar(&d)
		stDec, err := tx.Prepare(sqlDecision)
		if err != nil {
			return nil, err
		}
		defer stDec.Close()
		stFTS, err := tx.Prepare(sqlFTS)
		if err != nil {
			return nil, err
		}
		defer stFTS.Close()
		return d, insertarDecision(stDec, stFTS, d)
	})
	if err != nil {
		return "", err
	}
	return d.ID, nil
}

// consultaFTS convierte texto libre en una consulta FTS5 segura: cada palabra como prefijo citado, unidas por OR.
func consultaFTS(tema string) string {
	palabras := strings.FieldsFunc(strings.ToLower(tema), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terminos := make([]string, 0, len(palabras))
	for _, p := range palabras {
		terminos = append(terminos, `"`+p+`"*`)
	}
	return strings.Join(terminos, " OR ")
}

// BuscarDecisiones hace búsqueda de texto insensible a tildes, ordenada por relevancia (bm25).
func (m *Motor) BuscarDecisiones(tema string, limite int) ([]Decision, error) {
	q := consultaFTS(tema)
	if q == "" {
		return nil, nil
	}
	if limite <= 0 {
		limite = 5
	}
	filas, err := m.db.Query(`SELECT `+columnasDecision("d.")+` FROM decisiones_fts f
		JOIN decisiones d ON d.id = f.id WHERE decisiones_fts MATCH ? ORDER BY bm25(decisiones_fts) LIMIT ?`, q, limite)
	if err != nil {
		return nil, err
	}
	return escanearDecisiones(filas)
}

// VerDecision devuelve la ficha completa de un ID.
func (m *Motor) VerDecision(id string) (Decision, error) {
	filas, err := m.db.Query(`SELECT `+columnasDecision("")+` FROM decisiones WHERE id = ?`, strings.ToUpper(id))
	if err != nil {
		return Decision{}, err
	}
	ds, err := escanearDecisiones(filas)
	if err != nil {
		return Decision{}, err
	}
	if len(ds) == 0 {
		return Decision{}, fmt.Errorf("decisión %s: %w", id, ErrNoExiste)
	}
	return ds[0], nil
}

// UltimasDecisiones devuelve las n más recientes.
func (m *Motor) UltimasDecisiones(n int) ([]Decision, error) {
	filas, err := m.db.Query(`SELECT `+columnasDecision("")+` FROM decisiones
		ORDER BY CAST(SUBSTR(id, 3) AS INTEGER) DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	return escanearDecisiones(filas)
}

// ContarDecisiones devuelve el total indexado.
func (m *Motor) ContarDecisiones() (int, error) {
	var n int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM decisiones`).Scan(&n)
	return n, err
}

func escanearDecisiones(filas *sql.Rows) ([]Decision, error) {
	defer filas.Close()
	var ds []Decision
	for filas.Next() {
		var d Decision
		var alt, alc, eti string
		if err := filas.Scan(&d.ID, &d.Cuando, &d.MisionID, &d.RondaOHito, &d.Sesion, &d.CommitRef, &d.Que, &d.PorQue,
			&alt, &d.Decidio, &alc, &d.Reversible, &d.Estado, &d.ReemplazadaPor, &eti); err != nil {
			return nil, err
		}
		if err := errors.Join(json.Unmarshal([]byte(alt), &d.Alternativas), json.Unmarshal([]byte(alc), &d.Alcance),
			json.Unmarshal([]byte(eti), &d.Etiquetas)); err != nil {
			return nil, fmt.Errorf("decisión %s: %w", d.ID, err)
		}
		ds = append(ds, d)
	}
	return ds, filas.Err()
}
