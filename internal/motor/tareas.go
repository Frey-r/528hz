package motor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TestBloqueado fija el contenido de un test al registrar el ROJO.
type TestBloqueado struct {
	Ruta   string `json:"ruta"`
	SHA256 string `json:"sha256"`
}

// Evidencia ejecutada que respalda una transición.
type Evidencia struct {
	Cmd       string `json:"cmd"`
	Resultado string `json:"resultado"`
	Nivel     string `json:"nivel"`
	En        string `json:"en"`
}

// Tarea del ciclo CONTRATO → ROJO → VERDE → REFACTOR. Cada evento guarda la foto completa.
type Tarea struct {
	ID              string          `json:"id"`
	MisionID        string          `json:"mision_id"`
	Titulo          string          `json:"titulo"`
	Estado          string          `json:"estado"`
	ContratoRuta    string          `json:"contrato_ruta"`
	ContratoOKEn    string          `json:"contrato_ok_en"`
	RojoEn          string          `json:"rojo_en"`
	VerdeEn         string          `json:"verde_en"`
	TestsBloqueados []TestBloqueado `json:"tests_bloqueados"`
	Evidencia       []Evidencia     `json:"evidencia"`
	Celula          string          `json:"celula"`
	OwnerAgenteID   string          `json:"owner_agente_id"`
	BloqueadaPor    []string        `json:"bloqueada_por"`
}

var eventosTDD = map[string]bool{"tdd.contrato": true, "tdd.rojo": true, "tdd.verde": true, "tdd.refactor": true}

func guardarTarea(st *sql.Stmt, t Tarea) error {
	if t.TestsBloqueados == nil {
		t.TestsBloqueados = []TestBloqueado{}
	}
	if t.Evidencia == nil {
		t.Evidencia = []Evidencia{}
	}
	if t.BloqueadaPor == nil {
		t.BloqueadaPor = []string{}
	}
	_, err := st.Exec(t.ID, nulo(t.MisionID), t.Titulo, t.Estado, nulo(t.ContratoRuta), nulo(t.ContratoOKEn),
		jsonTexto(t.TestsBloqueados), nulo(t.RojoEn), nulo(t.VerdeEn), jsonTexto(t.Evidencia),
		nulo(t.Celula), nulo(t.OwnerAgenteID), jsonTexto(t.BloqueadaPor))
	if err != nil {
		return fmt.Errorf("tarea %s: %w", t.ID, err)
	}
	return nil
}

// RegistrarTarea guarda la foto de la tarea y anexa el evento a tdd.jsonl.
// La validación de transiciones es responsabilidad de la máquina TDD (paquete tdd).
func (m *Motor) RegistrarTarea(evento string, t Tarea) error {
	if !eventosTDD[evento] {
		return fmt.Errorf("%w: evento TDD desconocido %q", ErrCampos, evento)
	}
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.Titulo) == "" {
		return fmt.Errorf("%w: id, titulo", ErrCampos)
	}
	return m.escribir("tdd.jsonl", evento, func(tx *sql.Tx) (any, error) {
		st, err := tx.Prepare(sqlTarea)
		if err != nil {
			return nil, err
		}
		defer st.Close()
		return t, guardarTarea(st, t)
	})
}

const columnasTarea = `id, COALESCE(mision_id,''), titulo, estado, COALESCE(contrato_ruta,''), COALESCE(contrato_ok_en,''),
	COALESCE(rojo_en,''), COALESCE(verde_en,''), tests_bloqueados, evidencia, COALESCE(celula,''),
	COALESCE(owner_agente_id,''), bloqueada_por`

// Tarea devuelve una tarea por ID.
func (m *Motor) Tarea(id string) (Tarea, error) {
	ts, err := m.consultarTareas(`WHERE id = ?`, strings.ToUpper(id))
	if err != nil {
		return Tarea{}, err
	}
	if len(ts) == 0 {
		return Tarea{}, fmt.Errorf("tarea %s: %w", id, ErrNoExiste)
	}
	return ts[0], nil
}

// Tareas devuelve todas las tareas ordenadas por número.
func (m *Motor) Tareas() ([]Tarea, error) {
	return m.consultarTareas(`ORDER BY CAST(SUBSTR(id, 3) AS INTEGER), id`)
}

func (m *Motor) consultarTareas(sufijo string, args ...any) ([]Tarea, error) {
	filas, err := m.db.Query(`SELECT `+columnasTarea+` FROM tareas_tdd `+sufijo, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var ts []Tarea
	for filas.Next() {
		var t Tarea
		var tb, ev, bp string
		if err := filas.Scan(&t.ID, &t.MisionID, &t.Titulo, &t.Estado, &t.ContratoRuta, &t.ContratoOKEn,
			&t.RojoEn, &t.VerdeEn, &tb, &ev, &t.Celula, &t.OwnerAgenteID, &bp); err != nil {
			return nil, err
		}
		if err := errors.Join(json.Unmarshal([]byte(tb), &t.TestsBloqueados), json.Unmarshal([]byte(ev), &t.Evidencia),
			json.Unmarshal([]byte(bp), &t.BloqueadaPor)); err != nil {
			return nil, fmt.Errorf("tarea %s: %w", t.ID, err)
		}
		ts = append(ts, t)
	}
	return ts, filas.Err()
}
