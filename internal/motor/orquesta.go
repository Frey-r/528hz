package motor

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrConflicto indica que una ruta ya está arrendada por otro agente.
var ErrConflicto = errors.New("conflicto de arriendo")

// Arriendo da a un agente exclusividad de escritura sobre una ruta hasta que vence (D-0017).
type Arriendo struct {
	Ruta   string    `json:"ruta"`
	Agente string    `json:"agente"`
	Tarea  string    `json:"tarea"`
	Expira time.Time `json:"expira"`
}

// Tarjeta de traspaso entre roles (≤30 líneas, D-0015).
type Tarjeta struct {
	ID      string `json:"id"`
	TareaID string `json:"tarea_id"`
	Rol     string `json:"rol"`
	Agente  string `json:"agente"`
	Estado  string `json:"estado"`
	Texto   string `json:"texto"`
	Creada  string `json:"creada"`
}

const maxLineasTarjeta = 30

var estadoTarjeta = map[string]bool{"listo": true, "fallo": true, "bloqueado": true}

type eventoArriendo struct {
	Rutas  []string `json:"rutas"`
	Agente string   `json:"agente"`
	Tarea  string   `json:"tarea,omitempty"`
	Expira string   `json:"expira,omitempty"`
}

// Los vencimientos se guardan en UTC para que la comparación lexicográfica de SQLite sea correcta.
func utc(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func aplicarTomar(tx *sql.Tx, e eventoArriendo) error {
	for _, r := range e.Rutas {
		if _, err := tx.Exec(`INSERT INTO arriendos (ruta, agente_id, tarea_id, expira_en) VALUES (?, ?, ?, ?)
			ON CONFLICT(ruta) DO UPDATE SET agente_id = excluded.agente_id, tarea_id = excluded.tarea_id,
			expira_en = excluded.expira_en`, r, e.Agente, nulo(e.Tarea), e.Expira); err != nil {
			return err
		}
	}
	return nil
}

func aplicarSoltar(tx *sql.Tx, e eventoArriendo) error {
	if len(e.Rutas) == 0 {
		_, err := tx.Exec(`DELETE FROM arriendos WHERE agente_id = ?`, e.Agente)
		return err
	}
	for _, r := range e.Rutas {
		if _, err := tx.Exec(`DELETE FROM arriendos WHERE agente_id = ? AND ruta = ?`, e.Agente, r); err != nil {
			return err
		}
	}
	return nil
}

// TomarArriendo reserva rutas para un agente; falla con ErrConflicto si otro agente tiene un arriendo vigente.
func (m *Motor) TomarArriendo(rutas []string, agente, tarea string, expira time.Time) error {
	if len(rutas) == 0 || strings.TrimSpace(agente) == "" {
		return fmt.Errorf("%w: rutas y agente", ErrCampos)
	}
	e := eventoArriendo{Rutas: rutas, Agente: agente, Tarea: tarea, Expira: utc(expira)}
	return m.escribir("arriendos.jsonl", "arriendo.tomar", func(tx *sql.Tx) (any, error) {
		ahora := utc(time.Now())
		var conflictos []string
		for _, r := range rutas {
			var titular, suTarea, vence string
			err := tx.QueryRow(`SELECT agente_id, COALESCE(tarea_id,''), expira_en FROM arriendos WHERE ruta = ?`, r).Scan(&titular, &suTarea, &vence)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if titular != agente && vence > ahora {
				conflictos = append(conflictos, fmt.Sprintf("%s está arrendado por %s (tarea %s) hasta %s", r, titular, suTarea, vence))
			}
		}
		if len(conflictos) > 0 {
			return nil, fmt.Errorf("%w: %s; pídeselo a su titular por SendMessage", ErrConflicto, strings.Join(conflictos, "; "))
		}
		return e, aplicarTomar(tx, e)
	})
}

// SoltarArriendos libera las rutas indicadas del agente (todas si no se indican).
func (m *Motor) SoltarArriendos(agente string, rutas []string) error {
	e := eventoArriendo{Rutas: rutas, Agente: agente}
	return m.escribir("arriendos.jsonl", "arriendo.soltar", func(tx *sql.Tx) (any, error) {
		return e, aplicarSoltar(tx, e)
	})
}

// Arriendos devuelve los arriendos vigentes (los vencidos no cuentan).
func (m *Motor) Arriendos() ([]Arriendo, error) {
	filas, err := m.db.Query(`SELECT ruta, agente_id, COALESCE(tarea_id,''), expira_en FROM arriendos
		WHERE expira_en > ? ORDER BY ruta`, utc(time.Now()))
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Arriendo
	for filas.Next() {
		var a Arriendo
		var vence string
		if err := filas.Scan(&a.Ruta, &a.Agente, &a.Tarea, &vence); err != nil {
			return nil, err
		}
		if a.Expira, err = time.Parse(time.RFC3339, vence); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, filas.Err()
}

func aplicarTarjeta(tx *sql.Tx, t Tarjeta) error {
	_, err := tx.Exec(`INSERT INTO traspasos (id, tarea_id, rol, agente_id, estado, tarjeta, creado_en)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, t.ID, t.TareaID, t.Rol, nulo(t.Agente), t.Estado, t.Texto, t.Creada)
	return err
}

// NuevaTarjeta valida (≤30 líneas, estado válido) y registra una tarjeta de traspaso.
func (m *Motor) NuevaTarjeta(t Tarjeta) (string, error) {
	var faltan []string
	if strings.TrimSpace(t.TareaID) == "" {
		faltan = append(faltan, "tarea")
	}
	if strings.TrimSpace(t.Rol) == "" {
		faltan = append(faltan, "rol")
	}
	if !estadoTarjeta[t.Estado] {
		faltan = append(faltan, "estado (listo|fallo|bloqueado)")
	}
	if strings.TrimSpace(t.Texto) == "" {
		faltan = append(faltan, "texto")
	} else if n := len(strings.Split(strings.TrimRight(t.Texto, "\n"), "\n")); n > maxLineasTarjeta {
		faltan = append(faltan, fmt.Sprintf("texto ≤%d líneas (tiene %d): la tarjeta resume, no transcribe", maxLineasTarjeta, n))
	}
	if len(faltan) > 0 {
		return "", fmt.Errorf("%w: %s", ErrCampos, strings.Join(faltan, ", "))
	}
	t.TareaID = strings.ToUpper(t.TareaID)
	err := m.escribir("traspasos.jsonl", "tarjeta.nueva", func(tx *sql.Tx) (any, error) {
		var n int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(id, 4) AS INTEGER)), 0) + 1 FROM traspasos`).Scan(&n); err != nil {
			return nil, err
		}
		t.ID, t.Creada = fmt.Sprintf("TR-%04d", n), ahora()
		return t, aplicarTarjeta(tx, t)
	})
	if err != nil {
		return "", err
	}
	return t.ID, nil
}

// UltimaTarjeta devuelve la tarjeta más reciente de la tarea (opcionalmente de un rol).
func (m *Motor) UltimaTarjeta(tarea, rol string) (Tarjeta, error) {
	consulta := `SELECT id, tarea_id, rol, COALESCE(agente_id,''), estado, tarjeta, creado_en FROM traspasos WHERE tarea_id = ?`
	args := []any{strings.ToUpper(tarea)}
	if rol != "" {
		consulta += ` AND rol = ?`
		args = append(args, rol)
	}
	var t Tarjeta
	err := m.db.QueryRow(consulta+` ORDER BY CAST(SUBSTR(id, 4) AS INTEGER) DESC LIMIT 1`, args...).
		Scan(&t.ID, &t.TareaID, &t.Rol, &t.Agente, &t.Estado, &t.Texto, &t.Creada)
	if errors.Is(err, sql.ErrNoRows) {
		return t, fmt.Errorf("tarjeta de %s %s: %w", tarea, rol, ErrNoExiste)
	}
	return t, err
}
