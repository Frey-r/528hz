package motor

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// evento es una línea de log: {"evento": "...", "ts": "...", "datos": {...}}.
type evento struct {
	Evento string          `json:"evento"`
	Ts     string          `json:"ts"`
	Datos  json.RawMessage `json:"datos"`
}

func ahora() string { return time.Now().Format(time.RFC3339) }

// anexar agrega una línea al log con una sola escritura O_APPEND y devuelve el tamaño final.
func anexar(ruta, nombre string, datos any) (int64, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		Evento string `json:"evento"`
		Ts     string `json:"ts"`
		Datos  any    `json:"datos"`
	}{nombre, ahora(), datos}); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(ruta, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.Write(buf.Bytes()); err != nil {
		return 0, err
	}
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// aplicador reutiliza sentencias preparadas durante el reindex (RG-1.4).
type aplicador struct {
	tx                   *sql.Tx
	decision, fts, tarea *sql.Stmt
}

const (
	sqlDecision = `INSERT INTO decisiones (id, cuando, mision_id, ronda_o_hito, sesion, commit_ref, que, por_que,
		alternativas, decidio, alcance, reversible, estado, reemplazada_por, etiquetas)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	sqlFTS   = `INSERT INTO decisiones_fts (id, que, por_que, etiquetas, alternativas) VALUES (?, ?, ?, ?, ?)`
	sqlTarea = `INSERT INTO tareas_tdd (id, mision_id, titulo, estado, contrato_ruta, contrato_ok_en,
		tests_bloqueados, rojo_en, verde_en, evidencia, celula, owner_agente_id, bloqueada_por, superacion)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET mision_id = excluded.mision_id, titulo = excluded.titulo,
		estado = excluded.estado, contrato_ruta = excluded.contrato_ruta, contrato_ok_en = excluded.contrato_ok_en,
		tests_bloqueados = excluded.tests_bloqueados, rojo_en = excluded.rojo_en, verde_en = excluded.verde_en,
		evidencia = excluded.evidencia, celula = excluded.celula, owner_agente_id = excluded.owner_agente_id,
		bloqueada_por = excluded.bloqueada_por, superacion = excluded.superacion`
)

func prepararAplicador(tx *sql.Tx) (*aplicador, error) {
	ap := &aplicador{tx: tx}
	var err error
	if ap.decision, err = tx.Prepare(sqlDecision); err != nil {
		return nil, err
	}
	if ap.fts, err = tx.Prepare(sqlFTS); err != nil {
		ap.cerrar()
		return nil, err
	}
	if ap.tarea, err = tx.Prepare(sqlTarea); err != nil {
		ap.cerrar()
		return nil, err
	}
	return ap, nil
}

func (ap *aplicador) cerrar() {
	for _, s := range []*sql.Stmt{ap.decision, ap.fts, ap.tarea} {
		if s != nil {
			_ = s.Close()
		}
	}
}

// aplicar indexa un evento; devuelve false si el tipo es desconocido (se cuenta, no falla).
func (ap *aplicador) aplicar(ev evento) (bool, error) {
	switch ev.Evento {
	case "decision.nueva":
		var d Decision
		if err := json.Unmarshal(ev.Datos, &d); err != nil {
			return true, fmt.Errorf("decision.nueva: %w", err)
		}
		return true, insertarDecision(ap.decision, ap.fts, d)
	case "tdd.contrato", "tdd.rojo", "tdd.verde", "tdd.refactor", "tdd.superada":
		var t Tarea
		if err := json.Unmarshal(ev.Datos, &t); err != nil {
			return true, fmt.Errorf("%s: %w", ev.Evento, err)
		}
		return true, guardarTarea(ap.tarea, t)
	case "arriendo.tomar", "arriendo.soltar":
		var a eventoArriendo
		if err := json.Unmarshal(ev.Datos, &a); err != nil {
			return true, fmt.Errorf("%s: %w", ev.Evento, err)
		}
		if ev.Evento == "arriendo.tomar" {
			return true, aplicarTomar(ap.tx, a)
		}
		return true, aplicarSoltar(ap.tx, a)
	case "tarjeta.nueva":
		var t Tarjeta
		if err := json.Unmarshal(ev.Datos, &t); err != nil {
			return true, fmt.Errorf("tarjeta.nueva: %w", err)
		}
		return true, aplicarTarjeta(ap.tx, t)
	case "error.nuevo":
		var e Error
		if err := json.Unmarshal(ev.Datos, &e); err != nil {
			return true, fmt.Errorf("error.nuevo: %w", err)
		}
		_, err := aplicarError(ap.tx, e)
		return true, err
	case "leccion.nueva":
		var l Leccion
		if err := json.Unmarshal(ev.Datos, &l); err != nil {
			return true, fmt.Errorf("leccion.nueva: %w", err)
		}
		return true, aplicarLeccion(ap.tx, l)
	case "leccion.estado":
		var e eventoEstadoLeccion
		if err := json.Unmarshal(ev.Datos, &e); err != nil {
			return true, fmt.Errorf("leccion.estado: %w", err)
		}
		return true, aplicarEstadoLeccion(ap.tx, e)
	case "eval.run":
		var e eventoEval
		if err := json.Unmarshal(ev.Datos, &e); err != nil {
			return true, fmt.Errorf("eval.run: %w", err)
		}
		return true, aplicarEval(ap.tx, e)
	case "verificador.medicion":
		var e eventoMedicion
		if err := json.Unmarshal(ev.Datos, &e); err != nil {
			return true, fmt.Errorf("verificador.medicion: %w", err)
		}
		return true, aplicarMedicion(ap.tx, e)
	case "mision.nueva":
		var x Mision
		if err := json.Unmarshal(ev.Datos, &x); err != nil {
			return true, fmt.Errorf("mision.nueva: %w", err)
		}
		return true, aplicarMisionNueva(ap.tx, x)
	case "mision.cerrar", "ronda.nueva", "ronda.cerrar", "gasto.nuevo", "aceptacion.nueva", "piso.nuevo":
		var e eventoMision
		if err := json.Unmarshal(ev.Datos, &e); err != nil {
			return true, fmt.Errorf("%s: %w", ev.Evento, err)
		}
		return true, aplicarEventoMision(ap.tx, ev.Evento, e)
	case "medicion.nueva":
		var x Medicion
		if err := json.Unmarshal(ev.Datos, &x); err != nil {
			return true, fmt.Errorf("medicion.nueva: %w", err)
		}
		return true, aplicarMedicionRonda(ap.tx, x)
	case "milestone.nuevo":
		var x Milestone
		if err := json.Unmarshal(ev.Datos, &x); err != nil {
			return true, fmt.Errorf("milestone.nuevo: %w", err)
		}
		return true, aplicarMilestone(ap.tx, x)
	case "redteam.resultado":
		var e EventoRedteam
		if err := json.Unmarshal(ev.Datos, &e); err != nil {
			return true, fmt.Errorf("redteam.resultado: %w", err)
		}
		return true, aplicarRedteam(ap.tx, e)
	default:
		return false, nil
	}
}

func jsonTexto(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
