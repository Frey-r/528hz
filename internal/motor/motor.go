// Package motor es la memoria operativa del harness: índice SQLite sobre logs JSONL append-only.
//
// La fuente de verdad versionada son los logs (.harness/log/*.jsonl); la base (.harness/harness.db)
// es un índice reconstruible. Si un log cambió por fuera (git pull, merge), Abrir reindexa solo.
package motor

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"hz528"

	_ "modernc.org/sqlite" // driver SQLite en Go puro (sin CGO)
)

var (
	// ErrCampos indica campos obligatorios faltantes o inválidos.
	ErrCampos = errors.New("campos obligatorios faltantes o inválidos")
	// ErrNoExiste indica que el registro pedido no está en el índice.
	ErrNoExiste = errors.New("no existe")
)

// Motor envuelve la única conexión SQLite del proceso (RG-1.3).
type Motor struct {
	db   *sql.DB
	raiz string
}

// Resumen describe el resultado de un reindex.
type Resumen struct {
	Eventos, Decisiones, Tareas, Desconocidos int
	Duracion                                  time.Duration
}

const maxLinea = 4 << 20 // 4 MB por evento como máximo

// Abrir abre (o crea) el índice del proyecto en `raiz` y lo reindexa si está desactualizado.
func Abrir(raiz string) (*Motor, error) {
	if err := os.MkdirAll(filepath.Join(raiz, ".harness", "log"), 0o755); err != nil {
		return nil, err
	}
	dsn := filepath.ToSlash(filepath.Join(raiz, ".harness", "harness.db")) +
		"?_pragma=busy_timeout(15000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(0)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // una conexión por proceso: los escritores se serializan en SQLite
	m := &Motor{db: db, raiz: raiz}
	if _, err := db.Exec(hz528.SchemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("aplicar esquema: %w", err)
	}
	viejo, err := m.desactualizado()
	if err == nil && viejo {
		_, err = m.Reindex()
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return m, nil
}

// Cerrar libera la conexión.
func (m *Motor) Cerrar() error { return m.db.Close() }

func (m *Motor) rutaLog(nombre string) string {
	return filepath.Join(m.raiz, ".harness", "log", nombre)
}

func (m *Motor) logs() ([]string, error) {
	archivos, err := filepath.Glob(filepath.Join(m.raiz, ".harness", "log", "*.jsonl"))
	sort.Strings(archivos)
	return archivos, err
}

// desactualizado compara el tamaño de cada log con el registrado en la última indexación.
func (m *Motor) desactualizado() (bool, error) {
	registrados := map[string]string{}
	filas, err := m.db.Query(`SELECT clave, valor FROM meta`)
	if err != nil {
		return false, err
	}
	for filas.Next() {
		var k, v string
		if err := filas.Scan(&k, &v); err != nil {
			filas.Close()
			return false, err
		}
		registrados[k] = v
	}
	filas.Close()
	if _, ok := registrados["indexado"]; !ok {
		return true, nil
	}
	archivos, err := m.logs()
	if err != nil {
		return false, err
	}
	vistos := 0
	for _, a := range archivos {
		st, err := os.Stat(a)
		if err != nil {
			return false, err
		}
		if registrados["log:"+filepath.Base(a)] != strconv.FormatInt(st.Size(), 10) {
			return true, nil
		}
		vistos++
	}
	conteoLogs := 0
	for k := range registrados {
		if len(k) > 4 && k[:4] == "log:" {
			conteoLogs++
		}
	}
	return conteoLogs != vistos, nil
}

// Reindex reconstruye el índice completo desde los logs, en una sola transacción.
func (m *Motor) Reindex() (Resumen, error) {
	inicio := time.Now()
	var res Resumen
	tx, err := m.db.Begin()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, t := range []string{"decisiones", "decisiones_fts", "tareas_tdd", "arriendos", "traspasos", "meta"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return res, err
		}
	}
	ap, err := prepararAplicador(tx)
	if err != nil {
		return res, err
	}
	defer ap.cerrar()
	archivos, err := m.logs()
	if err != nil {
		return res, err
	}
	for _, a := range archivos {
		if err := m.reproducir(tx, ap, a, &res); err != nil {
			return res, err
		}
	}
	if _, err := tx.Exec(`INSERT INTO meta(clave, valor) VALUES ('indexado', ?)`, time.Now().Format(time.RFC3339)); err != nil {
		return res, err
	}
	if err := tx.Commit(); err != nil {
		return res, err
	}
	res.Duracion = time.Since(inicio)
	return res, nil
}

// reproducir lee un log en streaming (RG-1.8) y aplica cada evento.
func (m *Motor) reproducir(tx *sql.Tx, ap *aplicador, archivo string, res *Resumen) error {
	f, err := os.Open(archivo)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLinea)
	nombre := filepath.Base(archivo)
	for n := 1; sc.Scan(); n++ {
		linea := sc.Bytes()
		if len(linea) == 0 {
			continue
		}
		var ev evento
		if err := json.Unmarshal(linea, &ev); err != nil {
			return fmt.Errorf("%s:%d: línea JSON inválida: %w", nombre, n, err)
		}
		conocido, err := ap.aplicar(ev)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", nombre, n, err)
		}
		res.Eventos++
		switch {
		case !conocido:
			res.Desconocidos++
		case ev.Evento == "decision.nueva":
			res.Decisiones++
		default:
			res.Tareas++
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%s: %w", nombre, err)
	}
	st, err := f.Stat()
	if err != nil {
		return err
	}
	return registrarTamano(tx, nombre, st.Size())
}

func registrarTamano(tx *sql.Tx, nombre string, tam int64) error {
	_, err := tx.Exec(`INSERT INTO meta(clave, valor) VALUES (?, ?)
		ON CONFLICT(clave) DO UPDATE SET valor = excluded.valor`, "log:"+nombre, strconv.FormatInt(tam, 10))
	return err
}

// escribir aplica `aplicarFn` y anexa el evento al log dentro de una transacción IMMEDIATE:
// los IDs quedan serializados entre procesos y el índice nunca queda detrás de su propio log.
func (m *Motor) escribir(archivo, nombreEvento string, aplicarFn func(tx *sql.Tx) (any, error)) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	datos, err := aplicarFn(tx)
	if err != nil {
		return err
	}
	tam, err := anexar(m.rutaLog(archivo), nombreEvento, datos)
	if err != nil {
		return err
	}
	if err := registrarTamano(tx, archivo, tam); err != nil {
		return err
	}
	return tx.Commit()
}
