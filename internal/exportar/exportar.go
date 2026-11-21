// Package exportar escribe .harness/cache/hooks.json v2: el estado que leen los hooks del plugin
// sin lanzar procesos (D-0034, contratos/plugin.md).
package exportar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"hz528/internal/motor"
	"hz528/internal/proyecto"
)

// Activa es una tarea TDD no cerrada.
type Activa struct {
	ID              string   `json:"id"`
	Estado          string   `json:"estado"`
	TestsBloqueados []string `json:"tests_bloqueados"`
}

// ReglaExportada es la parte de una RD que necesita el hook.
type ReglaExportada struct {
	ID           string   `json:"id"`
	Orden        string   `json:"orden"`
	Remediacion  string   `json:"remediacion"`
	Herramientas []string `json:"herramientas"`
	Rutas        []string `json:"rutas"`
	Bash         []string `json:"bash"`
}

// Hooks es el contrato v2 entre `hz` y el plugin.
type Hooks struct {
	Version         int                 `json:"version"`
	Generado        string              `json:"generado"`
	Nivel           string              `json:"nivel"`
	Modo            string              `json:"modo"`
	Resumen         []string            `json:"resumen"`
	TareasActivas   []Activa            `json:"tareas_activas"`
	TestsBloqueados []string            `json:"tests_bloqueados"`
	Protegidos      []string            `json:"protegidos"`
	ReglasDuras     []ReglaExportada    `json:"reglas_duras"`
	Secretos        []string            `json:"secretos"`
	Arriendos       []ArriendoExportado `json:"arriendos"`
	Lecciones       []LeccionExportada  `json:"lecciones"`
	Misiones        []MisionExportada   `json:"misiones"`
}

// LeccionExportada es una lección activa que el hook ofrece justo a tiempo dentro de su ámbito.
type LeccionExportada struct {
	ID          string `json:"id"`
	Skill       string `json:"skill"`
	Ambito      string `json:"ambito"`
	Descripcion string `json:"descripcion"`
}

// ArriendoExportado es un arriendo vigente que el hook hace respetar.
type ArriendoExportado struct {
	Ruta   string `json:"ruta"`
	Agente string `json:"agente"`
	Tarea  string `json:"tarea"`
	Expira string `json:"expira"`
}

// Construir arma el contenido sin escribirlo.
func Construir(m *motor.Motor, raiz string) (Hooks, error) {
	h := Hooks{Version: 2, Generado: time.Now().Format(time.RFC3339), Nivel: "N1",
		TareasActivas: []Activa{}, TestsBloqueados: []string{}, Protegidos: proyecto.Protegidos,
		ReglasDuras: []ReglaExportada{}, Secretos: []string{}, Arriendos: []ArriendoExportado{}, Lecciones: []LeccionExportada{}, Misiones: []MisionExportada{}}
	perfil, err := proyecto.Cargar(raiz)
	switch {
	case err == nil:
		h.Nivel, h.Modo = perfil.Nivel, perfil.Modo
	case !errors.Is(err, proyecto.ErrSinProyecto):
		return h, err
	}
	reglas, err := proyecto.CargarReglas(raiz)
	if err != nil {
		return h, err
	}
	for _, r := range proyecto.Vigentes(reglas) {
		rem := r.Remediacion
		if rem == "" {
			rem = fmt.Sprintf("Respeta %s: %s", r.ID, r.Orden)
		}
		h.ReglasDuras = append(h.ReglasDuras, ReglaExportada{ID: r.ID, Orden: r.Orden, Remediacion: rem,
			Herramientas: vacioSiNil(r.Patrones.Herramientas), Rutas: vacioSiNil(r.Patrones.Rutas), Bash: vacioSiNil(r.Patrones.Bash)})
		if r.ClaveCatalogo == "secretos" {
			h.Secretos = append(h.Secretos, archivosQueCoinciden(raiz, r.Patrones.Rutas)...)
		}
	}
	h.Secretos = unicos(h.Secretos)
	tareas, err := m.Tareas()
	if err != nil {
		return h, err
	}
	for _, t := range tareas {
		if t.Estado == "cerrada" || t.Estado == "verificada" {
			continue
		}
		a := Activa{ID: t.ID, Estado: t.Estado, TestsBloqueados: []string{}}
		for _, tb := range t.TestsBloqueados {
			a.TestsBloqueados = append(a.TestsBloqueados, tb.Ruta)
			h.TestsBloqueados = append(h.TestsBloqueados, tb.Ruta)
		}
		h.TareasActivas = append(h.TareasActivas, a)
	}
	arriendos, err := m.Arriendos()
	if err != nil {
		return h, err
	}
	for _, a := range arriendos {
		h.Arriendos = append(h.Arriendos, ArriendoExportado{Ruta: a.Ruta, Agente: a.Agente, Tarea: a.Tarea, Expira: a.Expira.Format(time.RFC3339)})
	}
	lecciones, err := m.LeccionesActivas()
	if err != nil {
		return h, err
	}
	for _, l := range lecciones {
		h.Lecciones = append(h.Lecciones, LeccionExportada{ID: l.ID, Skill: l.Skill, Ambito: l.Ambito, Descripcion: l.Descripcion})
	}
	if h.Misiones, err = misiones(m, raiz); err != nil {
		return h, err
	}
	total, err := m.ContarDecisiones()
	if err != nil {
		return h, err
	}
	h.Resumen = resumen(filepath.Base(raiz), h, total)
	return h, nil
}

// Escribir construye y escribe hooks.json de forma atómica (el hook nunca lee un archivo a medias).
func Escribir(m *motor.Motor, raiz string) error {
	h, err := Construir(m, raiz)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(raiz, ".harness", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "hooks-*.json")
	if err != nil {
		return err
	}
	_, errW := tmp.Write(append(b, '\n'))
	if errC := tmp.Close(); errW != nil || errC != nil {
		os.Remove(tmp.Name())
		return errors.Join(errW, errC)
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, "hooks.json"))
}

func resumen(proyectoNombre string, h Hooks, decisiones int) []string {
	modo := h.Modo
	if modo == "" {
		modo = "sin perfil"
	}
	r := []string{fmt.Sprintf("Harness 528hz activo · proyecto %s · nivel %s · modo %s.", proyectoNombre, h.Nivel, modo)}
	if len(h.ReglasDuras) > 0 {
		ids := make([]string, 0, len(h.ReglasDuras))
		for _, rd := range h.ReglasDuras {
			ids = append(ids, rd.ID)
		}
		r = append(r, fmt.Sprintf("Reglas duras vigentes (inquebrantables): %s. Detalle en .harness/reglas.yaml.", strings.Join(ids, ", ")))
	}
	if len(h.TareasActivas) > 0 {
		partes := make([]string, 0, len(h.TareasActivas))
		for i, t := range h.TareasActivas {
			if i == 6 {
				partes = append(partes, fmt.Sprintf("… +%d", len(h.TareasActivas)-6))
				break
			}
			partes = append(partes, t.ID+" "+strings.ToUpper(t.Estado))
		}
		r = append(r, "Tareas TDD activas: "+strings.Join(partes, ", ")+". Los tests en ROJO+ están bloqueados.")
	}
	r = append(r,
		"Trabajo test-driven: CONTRATO → ROJO → VERDE → REFACTOR con `hz tdd …` (RG-2).",
		fmt.Sprintf("Decisiones registradas: %d. Antes de proponer un enfoque: `hz decision buscar <tema>`; al decidir: `hz decision nueva` (RG-3).", decisiones),
		"Para retomar o ver el estado completo: `hz estado`. Procedimiento: skill hz:harness.")
	return r
}

// archivosQueCoinciden resuelve patrones (con `*` y `**`) a archivos reales, relativos y con `/`.
func archivosQueCoinciden(raiz string, patrones []string) []string {
	var out []string
	_ = filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".harness", "vendor", ".venv", "venv":
				if ruta != raiz {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, err := filepath.Rel(raiz, ruta)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		for _, p := range patrones {
			if Coincide(strings.TrimPrefix(filepath.ToSlash(p), "./"), rel) {
				out = append(out, rel)
				break
			}
		}
		return nil
	})
	return out
}

// Coincide evalúa un glob con `*` (dentro de un segmento) y `**` (cero o más segmentos).
// Un patrón sin `/` coincide con el nombre base en cualquier carpeta solo si empieza con `**/`;
// si no, se ancla a la raíz.
func Coincide(patron, rel string) bool {
	return coincideSegmentos(strings.Split(patron, "/"), strings.Split(rel, "/"))
}

func coincideSegmentos(p, r []string) bool {
	for len(p) > 0 {
		if p[0] == "**" {
			for i := 0; i <= len(r); i++ {
				if coincideSegmentos(p[1:], r[i:]) {
					return true
				}
			}
			return false
		}
		if len(r) == 0 {
			return false
		}
		if ok, _ := path.Match(p[0], r[0]); !ok {
			return false
		}
		p, r = p[1:], r[1:]
	}
	return len(r) == 0
}

func unicos(xs []string) []string {
	sort.Strings(xs)
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func vacioSiNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// MisionExportada es el estado de presupuesto de una misión activa: el plugin frena los lanzamientos sin presupuesto.
type MisionExportada struct {
	ID             string  `json:"id"`
	Gasto          float64 `json:"gasto"`
	UsdMax         float64 `json:"usd_max"`
	SinPresupuesto bool    `json:"sin_presupuesto"`
}

// misiones exporta el presupuesto de las misiones activas. Lee presupuesto.usd_max directo de mision.yaml
// (el paquete mision depende de tdd, que depende de exportar: importarlo haría un ciclo).
func misiones(m *motor.Motor, raiz string) ([]MisionExportada, error) {
	todas, err := m.Misiones()
	if err != nil {
		return nil, err
	}
	out := []MisionExportada{}
	for _, x := range todas {
		if x.Estado != "activa" {
			continue
		}
		e := MisionExportada{ID: x.ID}
		if e.Gasto, err = m.Gasto(x.ID); err != nil {
			return nil, err
		}
		var def struct {
			Presupuesto struct {
				UsdMax float64 `yaml:"usd_max"`
			} `yaml:"presupuesto"`
		}
		if b, err := os.ReadFile(filepath.Join(raiz, ".harness", "misiones", x.ID, "mision.yaml")); err == nil {
			if err := yaml.Unmarshal(b, &def); err != nil {
				return nil, fmt.Errorf("mision.yaml de %s: %w", x.ID, err)
			}
		}
		e.UsdMax = def.Presupuesto.UsdMax
		e.SinPresupuesto = e.UsdMax > 0 && e.Gasto >= e.UsdMax
		out = append(out, e)
	}
	return out, nil
}
