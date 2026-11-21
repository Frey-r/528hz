// Package dashboard sirve el panel web local del harness (contratos/cli.md · 3.7, D-0050): árbol de
// supervisión + carriles de actividad por agente, calculados en Go como SVG (D-0044). La página solo
// refresca el fragmento.
package dashboard

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	iofs "io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hz528/internal/exportar"
	"hz528/internal/motor"
)

// Agente es una fila del snapshot que escribe el plugin (.harness/cache/actividad.json).
type Agente struct {
	ID          string `json:"id"`
	Rol         string `json:"rol"`
	Descripcion string `json:"descripcion"`
	Padre       string `json:"padre"`
	Tarea       string `json:"tarea"`
	Estado      string `json:"estado"`
	Tools       int    `json:"tools"`
	Errores     int    `json:"errores"`
	Seguidos    int    `json:"seguidos"`
	Ultima      string `json:"ultima"`
	Desde       int64  `json:"desde"`
	Visto       int64  `json:"visto"`
}

// Vivo indica si el agente sigue trabajando.
func (a Agente) Vivo() bool { return a.Estado == "lanzado" || a.Estado == "activo" }

// Glifo de estado (siempre acompañado de texto: nunca solo color).
func (a Agente) Glifo() string {
	switch {
	case a.Estado == "fallido":
		return "✕"
	case a.Estado == "detenido":
		return "■"
	case a.Estado == "terminado":
		return "✓"
	case a.Seguidos >= SeguidosAlerta:
		return "!"
	case a.Estado == "activo":
		return "●"
	}
	return "○"
}

// SeguidosAlerta es la racha de errores que marca a un agente.
const SeguidosAlerta = 3

const maxUltima = 40

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Actividad es el snapshot completo: filas y, por agente, sus últimos eventos [t_ms, tipo].
type Actividad struct {
	Version  int                   `json:"version"`
	Generado string                `json:"generado"`
	Agentes  []Agente              `json:"agentes"`
	Eventos  map[string][][2]int64 `json:"eventos"`
}

// ArchivoActividad es donde el plugin escribe el snapshot.
const ArchivoActividad = ".harness/cache/actividad.json"

// LeerActividad lee el snapshot; sin archivo devuelve nil sin error.
func LeerActividad(raiz string) (*Actividad, error) {
	b, err := os.ReadFile(filepath.Join(raiz, ArchivoActividad))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var a Actividad
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("%s: %w", ArchivoActividad, err)
	}
	return &a, nil
}

// Nodo es un agente en el árbol de supervisión.
type Nodo struct {
	Agente Agente
	Nivel  int
}

// Arbol cuelga cada agente de su supervisor; entre hermanos, los vivos primero y luego por antigüedad.
func Arbol(lista []Agente) []Nodo {
	ids := make(map[string]bool, len(lista))
	for _, a := range lista {
		ids[a.ID] = true
	}
	hijos := map[string][]Agente{}
	var raices []Agente
	for _, a := range lista {
		if a.Padre != "" && a.Padre != a.ID && ids[a.Padre] {
			hijos[a.Padre] = append(hijos[a.Padre], a)
		} else {
			raices = append(raices, a)
		}
	}
	ordenar := func(xs []Agente) {
		sort.SliceStable(xs, func(i, j int) bool {
			if xs[i].Vivo() != xs[j].Vivo() {
				return xs[i].Vivo()
			}
			return xs[i].Desde < xs[j].Desde
		})
	}
	var out []Nodo
	var visitar func(a Agente, nivel int)
	visitar = func(a Agente, nivel int) {
		out = append(out, Nodo{a, nivel})
		hs := hijos[a.ID]
		ordenar(hs)
		for _, h := range hs {
			visitar(h, nivel+1)
		}
	}
	ordenar(raices)
	for _, r := range raices {
		visitar(r, 0)
	}
	return out
}

// Hace dice hace cuánto en la unidad legible.
func Hace(d time.Duration) string {
	switch s := int(d.Seconds()); {
	case s < 60:
		return fmt.Sprintf("%d s", max(0, s))
	case s < 3600:
		return fmt.Sprintf("%d min", s/60)
	default:
		return fmt.Sprintf("%d h", s/3600)
	}
}

// Resumen de un agente: tools, errores (con la racha si alerta), última acción y hace cuánto o cómo terminó.
func Resumen(a Agente, ahora time.Time) string {
	partes := []string{fmt.Sprintf("%d tools", a.Tools)}
	if a.Errores > 0 {
		e := fmt.Sprintf("%d err", a.Errores)
		if a.Seguidos >= SeguidosAlerta {
			e += fmt.Sprintf(" (%d seguidos)", a.Seguidos)
		}
		partes = append(partes, e)
	}
	if a.Ultima != "" {
		partes = append(partes, recortar(a.Ultima, maxUltima))
	}
	if a.Vivo() {
		partes = append(partes, "hace "+Hace(ahora.Sub(time.UnixMilli(a.Visto))))
	} else {
		partes = append(partes, a.Estado)
	}
	return strings.Join(partes, " · ")
}

// Datos es todo lo que dibuja el dashboard.
type Datos struct {
	Proyecto, Nivel, Modo string
	Misiones              []exportar.MisionExportada
	Tareas                []exportar.Activa
	Arriendos             int
	Actividad             *Actividad
}

// Reunir junta el estado del motor (vía hooks.json v2 en memoria) con el snapshot del plugin.
func Reunir(m *motor.Motor, raiz string) (Datos, error) {
	h, err := exportar.Construir(m, raiz)
	if err != nil {
		return Datos{}, err
	}
	act, err := LeerActividad(raiz)
	if err != nil {
		return Datos{}, err
	}
	return Datos{Proyecto: filepath.Base(raiz), Nivel: h.Nivel, Modo: h.Modo, Misiones: h.Misiones,
		Tareas: h.TareasActivas, Arriendos: len(h.Arriendos), Actividad: act}, nil
}

// ───────────────────────── fragmento ─────────────────────────

// Geometría del carril (unidades del viewBox 600×24).
const (
	carrilIzq     = 0.0
	carrilAncho   = 600.0
	carrilAlto    = 24.0
	anchoMarca    = 2.0
	anchoMarcaErr = 3.0
)

// xEvento ubica un instante en el carril; fuera de la ventana devuelve -1.
func xEvento(t int64, ahora time.Time, ventana time.Duration) float64 {
	desde := ahora.Add(-ventana).UnixMilli()
	hasta := ahora.UnixMilli()
	if t < desde || t > hasta {
		return -1
	}
	return carrilIzq + carrilAncho*float64(t-desde)/float64(hasta-desde)
}

type marca struct{ Clase, X, Y, Ancho, Alto string }

type fila struct {
	Rol, Tarea, Descripcion, Glifo, Clase, Resumen string
	Nivel                                          int
	Marcas                                         []marca
}

type misionVista struct {
	Clase, Texto string
	ConTope      bool
	Ancho        template.CSS
}

type vista struct {
	Proyecto, Nivel, Modo, Tareas string
	Arriendos                     int
	Misiones                      []misionVista
	EjeInicio, EjeMedio           string
	Filas                         []fila
}

var claseEvento = map[int64]string{0: "ok", 1: "err", 2: "ini", 3: "fin"}

func num(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func usd(v float64) string {
	return "US$" + strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}

func marcas(eventos [][2]int64, ahora time.Time, ventana time.Duration) []marca {
	out := make([]marca, 0, len(eventos))
	for _, ev := range eventos {
		x := xEvento(ev[0], ahora, ventana)
		clase, conocida := claseEvento[ev[1]]
		if x < 0 || !conocida {
			continue
		}
		ancho, y, alto := anchoMarca, 6.0, 12.0
		switch ev[1] {
		case 1:
			ancho, y, alto = anchoMarcaErr, 2, 20
		case 2, 3:
			y, alto = 0, carrilAlto
		}
		// la marca nunca se sale del carril por la derecha
		x = min(x, carrilIzq+carrilAncho-ancho)
		out = append(out, marca{Clase: clase, X: num(x), Y: num(y), Ancho: num(ancho), Alto: num(alto)})
	}
	return out
}

func armarVista(d Datos, ahora time.Time, ventana time.Duration) vista {
	v := vista{Proyecto: d.Proyecto, Nivel: d.Nivel, Modo: d.Modo, Arriendos: d.Arriendos,
		EjeInicio: "−" + Hace(ventana), EjeMedio: "−" + Hace(ventana/2)}
	tareas := make([]string, 0, len(d.Tareas))
	for _, t := range d.Tareas {
		tareas = append(tareas, t.ID+" "+strings.ToUpper(t.Estado))
	}
	v.Tareas = "ninguna"
	if len(tareas) > 0 {
		v.Tareas = strings.Join(tareas, ", ")
	}
	for _, m := range d.Misiones {
		mv := misionVista{Texto: fmt.Sprintf("misión %s · %s", m.ID, usd(m.Gasto))}
		if m.UsdMax > 0 {
			r := m.Gasto / m.UsdMax
			mv.ConTope, mv.Ancho = true, template.CSS(fmt.Sprintf("width:%.0f%%", min(1, r)*100))
			mv.Texto += fmt.Sprintf(" de %s · %.0f%%", usd(m.UsdMax), r*100)
		}
		if m.SinPresupuesto {
			mv.Clase, mv.Texto = "sin", mv.Texto+" · sin presupuesto"
		}
		v.Misiones = append(v.Misiones, mv)
	}
	if d.Actividad == nil {
		return v
	}
	for _, n := range Arbol(d.Actividad.Agentes) {
		a := n.Agente
		f := fila{Rol: a.Rol, Tarea: a.Tarea, Descripcion: a.Descripcion, Glifo: a.Glifo(), Nivel: min(n.Nivel, 5),
			Resumen: Resumen(a, ahora), Marcas: marcas(d.Actividad.Eventos[a.ID], ahora, ventana)}
		switch {
		case a.Seguidos >= SeguidosAlerta || a.Estado == "fallido":
			f.Clase = "alerta"
		case !a.Vivo():
			f.Clase = "fin"
		}
		v.Filas = append(v.Filas, f)
	}
	return v
}

//go:embed fragmento.html
var fragmentoHTML string

// Plantilla compilada una sola vez por proceso (RG-1.4).
var plantilla = template.Must(template.New("fragmento").Parse(fragmentoHTML))

// Fragmento escribe el HTML del árbol + carriles de la ventana.
func Fragmento(w io.Writer, d Datos, ahora time.Time, ventana time.Duration) error {
	return plantilla.Execute(w, armarVista(d, ahora, ventana))
}

// ───────────────────────── servidor ─────────────────────────

//go:embed pagina.html
var pagina []byte

//go:embed app.js
var app []byte

var ventanas = map[string]time.Duration{"5m": 5 * time.Minute, "10m": 10 * time.Minute, "30m": 30 * time.Minute}

// Servidor es el handler HTTP local de solo lectura. Rechaza todo Host que no sea este equipo y este puerto
// (protección contra DNS rebinding) y todo método que no sea GET o HEAD.
func Servidor(m *motor.Motor, raiz string, puerto int) http.Handler {
	p := strconv.Itoa(puerto)
	permitidos := map[string]bool{net.JoinHostPort("127.0.0.1", p): true, net.JoinHostPort("localhost", p): true, net.JoinHostPort("::1", p): true}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(pagina)
	})
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(app)
	})
	mux.HandleFunc("/fragmento", func(w http.ResponseWriter, r *http.Request) {
		clave := r.URL.Query().Get("ventana")
		if clave == "" {
			clave = "10m"
		}
		ventana, ok := ventanas[clave]
		if !ok {
			http.Error(w, "ventana: 5m, 10m o 30m", http.StatusBadRequest)
			return
		}
		d, err := Reunir(m, raiz)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := Fragmento(w, d, time.Now(), ventana); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/api/panel", func(w http.ResponseWriter, r *http.Request) {
		d, err := Reunir(m, raiz)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		salida := map[string]any{"generado": time.Now().Format(time.RFC3339), "actividad_generada": "",
			"agentes": []Agente{}, "eventos": map[string][][2]int64{}, "tareas": d.Tareas, "arriendos": d.Arriendos, "misiones": d.Misiones}
		if d.Actividad != nil {
			salida["actividad_generada"], salida["agentes"] = d.Actividad.Generado, d.Actividad.Agentes
			if d.Actividad.Eventos != nil {
				salida["eventos"] = d.Actividad.Eventos
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(salida)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !permitidos[strings.ToLower(r.Host)] {
			http.Error(w, "hz dashboard solo atiende a este equipo", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "solo lectura", http.StatusMethodNotAllowed)
			return
		}
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		mux.ServeHTTP(w, r)
	})
}
