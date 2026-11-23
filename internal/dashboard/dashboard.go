// Package dashboard sirve el panel web local del harness: un dashboard central que lee todos los proyectos
// conectados (contratos/cli.md · 3.7 y 3.8, D-0050, D-0053). Árbol de supervisión + carriles de actividad,
// calculados en Go como SVG (D-0044); la página solo refresca el fragmento.
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
	"sync"
	"time"

	"hz528/internal/exportar"
	"hz528/internal/motor"
	"hz528/internal/proyecto"
	"hz528/internal/registro"
)

// Agente es una fila de un snapshot de sesión que escribe el plugin.
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

// Principal es el id de la fila de la sesión principal en cada snapshot.
const Principal = "principal"

const maxUltima = 40

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ───────────────────────── sesiones ─────────────────────────

// DirSesiones es donde el plugin escribe un snapshot por sesión (<sessionId>.json).
const DirSesiones = ".harness/cache/actividad"

// Inactividad es cuánto puede pasar una sesión sin escribir antes de considerarla inactiva.
const Inactividad = 30 * time.Minute

// Sesion es el snapshot de una sesión de Claude Code en un proyecto.
type Sesion struct {
	ID        string                `json:"id"`
	Generado  string                `json:"generado"`
	Terminada bool                  `json:"terminada"`
	Inactiva  bool                  `json:"inactiva"`
	Agentes   []Agente              `json:"agentes"`
	Eventos   map[string][][2]int64 `json:"eventos"`
	t         time.Time
}

// Viva indica si la sesión sigue en curso: ni terminada ni inactiva.
func (s Sesion) Viva() bool { return !s.Terminada && !s.Inactiva }

// instante es cuándo se escribió el snapshot.
func (s Sesion) instante() time.Time {
	if !s.t.IsZero() {
		return s.t
	}
	t, _ := time.Parse(time.RFC3339, s.Generado)
	return t
}

// Corto es el id de la sesión recortado a 8 caracteres.
func (s Sesion) Corto() string {
	if len(s.ID) > 8 {
		return s.ID[:8]
	}
	return s.ID
}

// EstadoTexto es activa, inactiva o terminada.
func (s Sesion) EstadoTexto() string {
	switch {
	case s.Terminada:
		return "terminada"
	case s.Inactiva:
		return "inactiva"
	}
	return "activa"
}

type snapshot struct {
	Sesion    string                `json:"sesion"`
	Generado  string                `json:"generado"`
	Terminada bool                  `json:"terminada"`
	Agentes   []Agente              `json:"agentes"`
	Eventos   map[string][][2]int64 `json:"eventos"`
}

func dirSesiones(raiz string) string { return filepath.Join(raiz, filepath.FromSlash(DirSesiones)) }

// LeerSesiones lee los snapshots del proyecto: las activas primero y luego por la más reciente. Un archivo
// ilegible (por ejemplo, leído a medio escribir) se omite; sin carpeta no hay sesiones.
func LeerSesiones(raiz string, ahora time.Time) ([]Sesion, error) {
	dir := dirSesiones(raiz)
	entradas, err := os.ReadDir(dir)
	if errors.Is(err, iofs.ErrNotExist) {
		return []Sesion{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Sesion, 0, len(entradas))
	for _, e := range entradas {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sn snapshot
		if json.Unmarshal(b, &sn) != nil {
			continue
		}
		s := Sesion{ID: sn.Sesion, Generado: sn.Generado, Terminada: sn.Terminada, Agentes: sn.Agentes, Eventos: sn.Eventos}
		if s.ID == "" {
			s.ID = strings.TrimSuffix(e.Name(), ".json")
		}
		if s.Agentes == nil {
			s.Agentes = []Agente{}
		}
		if s.Eventos == nil {
			s.Eventos = map[string][][2]int64{}
		}
		if t, err := time.Parse(time.RFC3339, sn.Generado); err == nil {
			s.t = t
		} else if info, err := e.Info(); err == nil {
			s.t = info.ModTime()
		}
		s.Inactiva = !s.Terminada && ahora.Sub(s.t) > Inactividad
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Viva() != out[j].Viva() {
			return out[i].Viva()
		}
		if !out[i].t.Equal(out[j].t) {
			return out[i].t.After(out[j].t)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// PodarSesiones borra los snapshots que llevan más de edad sin escribirse y devuelve cuántos borró.
func PodarSesiones(raiz string, ahora time.Time, edad time.Duration) int {
	dir := dirSesiones(raiz)
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entradas {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err == nil && ahora.Sub(info.ModTime()) > edad && os.Remove(filepath.Join(dir, e.Name())) == nil {
			n++
		}
	}
	return n
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

// ───────────────────────── proyectos ─────────────────────────

// Proyecto es todo lo que el dashboard sabe de un proyecto conectado.
type Proyecto struct {
	ID         string                     `json:"id"`
	Nombre     string                     `json:"nombre"`
	Raiz       string                     `json:"raiz"`
	Disponible bool                       `json:"disponible"`
	Error      string                     `json:"error"`
	Nivel      string                     `json:"nivel"`
	Modo       string                     `json:"modo"`
	Tareas     []exportar.Activa          `json:"tareas"`
	Arriendos  int                        `json:"arriendos"`
	Misiones   []exportar.MisionExportada `json:"misiones"`
	Sesiones   []Sesion                   `json:"sesiones"`
}

// Reunir junta el estado del motor (hooks.json v2 en memoria) con las sesiones que escribió el plugin.
func Reunir(m *motor.Motor, raiz string, ahora time.Time) (Proyecto, error) {
	h, err := exportar.Construir(m, raiz)
	if err != nil {
		return Proyecto{}, err
	}
	ss, err := LeerSesiones(raiz, ahora)
	if err != nil {
		return Proyecto{}, err
	}
	p := Proyecto{ID: registro.ID(raiz), Nombre: filepath.Base(raiz), Raiz: raiz, Disponible: true, Nivel: h.Nivel, Modo: h.Modo,
		Tareas: h.TareasActivas, Arriendos: len(h.Arriendos), Misiones: h.Misiones, Sesiones: ss}
	return completo(p), nil
}

// completo cambia los nil por listas vacías para que el JSON sea estable.
func completo(p Proyecto) Proyecto {
	if p.Tareas == nil {
		p.Tareas = []exportar.Activa{}
	}
	if p.Misiones == nil {
		p.Misiones = []exportar.MisionExportada{}
	}
	if p.Sesiones == nil {
		p.Sesiones = []Sesion{}
	}
	return p
}

// Central reúne los proyectos del registro de usuario. Abre un motor por proyecto al primer uso y lo
// reutiliza entre peticiones (RG-1); lo cierra cuando el proyecto sale del registro o falla.
type Central struct {
	hogar   string
	mu      sync.Mutex
	motores map[string]*motor.Motor // por id de proyecto
}

// NuevaCentral crea la central sobre el hogar de hz (donde vive proyectos.json).
func NuevaCentral(hogar string) *Central {
	return &Central{hogar: hogar, motores: map[string]*motor.Motor{}}
}

// Cerrar libera todos los motores.
func (c *Central) Cerrar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id := range c.motores {
		c.soltar(id)
	}
}

func (c *Central) soltar(id string) {
	if m := c.motores[id]; m != nil {
		_ = m.Cerrar()
		delete(c.motores, id)
	}
}

// Proyectos lee el registro y cada proyecto conectado. Un proyecto ilegible sale como no disponible con su
// motivo; solo un registro ilegible es error.
func (c *Central) Proyectos(ahora time.Time) ([]Proyecto, error) {
	lista, err := registro.Lista(c.hogar)
	if err != nil {
		return nil, err
	}
	nombres := registro.Nombres(lista)
	c.mu.Lock()
	defer c.mu.Unlock()
	vigentes := make(map[string]bool, len(lista))
	out := make([]Proyecto, 0, len(lista))
	for _, r := range lista {
		id := registro.ID(r.Raiz)
		vigentes[id] = true
		p, err := c.leer(id, r.Raiz, ahora)
		if err != nil {
			c.soltar(id)
			p = Proyecto{Error: err.Error()}
		}
		p.ID, p.Nombre, p.Raiz = id, nombres[r.Raiz], r.Raiz
		out = append(out, completo(p))
	}
	for id := range c.motores {
		if !vigentes[id] {
			c.soltar(id)
		}
	}
	return out, nil
}

func (c *Central) leer(id, raiz string, ahora time.Time) (Proyecto, error) {
	if _, err := os.Stat(raiz); err != nil {
		return Proyecto{}, errors.New("la carpeta no existe")
	}
	if _, err := os.Stat(filepath.Join(raiz, ".harness", "harness.yaml")); err != nil {
		return Proyecto{}, errors.New("sin harness")
	}
	perfil, err := proyecto.Cargar(raiz)
	if err != nil {
		return Proyecto{}, err
	}
	if !perfil.Enabled {
		return Proyecto{}, errors.New("harness desactivado")
	}
	m := c.motores[id]
	if m == nil {
		if m, err = motor.Abrir(raiz); err != nil {
			return Proyecto{}, fmt.Errorf("motor: %w", err)
		}
		c.motores[id] = m
	}
	return Reunir(m, raiz, ahora)
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

type chip struct {
	ID, Nombre     string
	Activo, Alerta bool
}

type vistaProyecto struct {
	ID, Nombre, Nivel, Modo, Tareas, Error string
	Arriendos                              int
	Disponible, Abierto                    bool
	Misiones                               []misionVista
	Filas                                  []fila
	alertas, vivos, activas                int
	ultima                                 time.Time
}

type vista struct {
	Titulo              string
	Todos               bool
	Chips               []chip
	Proyectos           []vistaProyecto
	EjeInicio, EjeMedio string
}

var claseEvento = map[int64]string{0: "ok", 1: "err", 2: "ini", 3: "fin"}

func num(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func usd(v float64) string {
	return "US$" + strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}

func cuenta(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(n) + " " + plural
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

// enVentana indica si alguno de los eventos cae dentro de la ventana.
func enVentana(eventos [][2]int64, ahora time.Time, ventana time.Duration) bool {
	desde, hasta := ahora.Add(-ventana).UnixMilli(), ahora.UnixMilli()
	for _, ev := range eventos {
		if ev[0] >= desde && ev[0] <= hasta {
			return true
		}
	}
	return false
}

// filasSesion: la sesión es la raíz (con los carriles de la principal) y debajo cuelgan sus subagentes.
func filasSesion(s Sesion, ahora time.Time, ventana time.Duration) []fila {
	viva := s.Viva()
	raiz := fila{Rol: "sesión " + s.Corto() + " · " + s.EstadoTexto(), Glifo: "●", Clase: "sesion"}
	switch {
	case s.Terminada:
		raiz.Glifo = "✓"
	case s.Inactiva:
		raiz.Glifo = "○"
	}
	if !viva {
		raiz.Clase = "sesion fin"
	}
	out := []fila{raiz}
	desplazar := 0
	for _, n := range Arbol(s.Agentes) {
		a := n.Agente
		if n.Nivel == 0 {
			if a.ID == Principal {
				out[0].Resumen = Resumen(a, ahora)
				out[0].Marcas = marcas(s.Eventos[a.ID], ahora, ventana)
				if viva {
					out[0].Glifo = a.Glifo()
				}
				if viva && a.Seguidos >= SeguidosAlerta {
					out[0].Clase = "sesion alerta"
				}
				desplazar = 0
				continue
			}
			// un subagente cuyo supervisor no está en el snapshot cuelga directo de la sesión
			desplazar = 1
		}
		f := fila{Rol: a.Rol, Tarea: a.Tarea, Descripcion: a.Descripcion, Glifo: a.Glifo(), Nivel: min(n.Nivel+desplazar, 5),
			Resumen: Resumen(a, ahora), Marcas: marcas(s.Eventos[a.ID], ahora, ventana)}
		switch {
		case (viva && a.Seguidos >= SeguidosAlerta) || a.Estado == "fallido":
			f.Clase = "alerta"
		case !a.Vivo() || !viva:
			f.Clase = "fin"
		}
		out = append(out, f)
	}
	return out
}

func armarProyecto(p Proyecto, ahora time.Time, ventana time.Duration) vistaProyecto {
	v := vistaProyecto{ID: p.ID, Nombre: p.Nombre, Nivel: p.Nivel, Modo: p.Modo, Arriendos: p.Arriendos,
		Disponible: p.Disponible, Error: p.Error}
	tareas := make([]string, 0, len(p.Tareas))
	for _, t := range p.Tareas {
		tareas = append(tareas, t.ID+" "+strings.ToUpper(t.Estado))
	}
	v.Tareas = "ninguna"
	if len(tareas) > 0 {
		v.Tareas = strings.Join(tareas, ", ")
	}
	for _, m := range p.Misiones {
		mv := misionVista{Texto: fmt.Sprintf("misión %s · %s", m.ID, usd(m.Gasto))}
		if m.UsdMax > 0 {
			r := m.Gasto / m.UsdMax
			mv.ConTope, mv.Ancho = true, template.CSS(fmt.Sprintf("width:%.0f%%", min(1, r)*100))
			mv.Texto += fmt.Sprintf(" de %s · %.0f%%", usd(m.UsdMax), r*100)
		}
		if m.SinPresupuesto {
			mv.Clase, mv.Texto = "sin", mv.Texto+" · sin presupuesto"
			v.alertas++
		}
		v.Misiones = append(v.Misiones, mv)
	}
	for _, s := range p.Sesiones {
		viva := s.Viva()
		if t := s.instante(); t.After(v.ultima) {
			v.ultima = t
		}
		if viva {
			v.activas++
		}
		conEventos := false
		for _, a := range s.Agentes {
			reciente := enVentana(s.Eventos[a.ID], ahora, ventana)
			conEventos = conEventos || reciente
			if viva && a.Vivo() && a.ID != Principal {
				v.vivos++
			}
			if (viva && a.Vivo() && a.Seguidos >= SeguidosAlerta) || (a.Estado == "fallido" && reciente) {
				v.alertas++
			}
		}
		if viva || conEventos {
			v.Filas = append(v.Filas, filasSesion(s, ahora, ventana)...)
		}
	}
	v.Abierto = len(v.Filas) > 0
	return v
}

func armarVista(ps []Proyecto, filtro string, ahora time.Time, ventana time.Duration) vista {
	v := vista{EjeInicio: "−" + Hace(ventana), EjeMedio: "−" + Hace(ventana/2), Todos: true}
	vs := make([]vistaProyecto, 0, len(ps))
	activas, vivos, alertas := 0, 0, 0
	for _, p := range ps {
		vp := armarProyecto(p, ahora, ventana)
		activas, vivos, alertas = activas+vp.activas, vivos+vp.vivos, alertas+vp.alertas
		if vp.ID == filtro {
			v.Todos = false
		}
		vs = append(vs, vp)
	}
	sort.SliceStable(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		if (a.alertas > 0) != (b.alertas > 0) {
			return a.alertas > 0
		}
		if (a.vivos > 0) != (b.vivos > 0) {
			return a.vivos > 0
		}
		if !a.ultima.Equal(b.ultima) {
			return a.ultima.After(b.ultima)
		}
		return a.Nombre < b.Nombre
	})
	v.Titulo = strings.Join([]string{cuenta(len(ps), "proyecto", "proyectos"), cuenta(activas, "sesión activa", "sesiones activas"),
		cuenta(vivos, "agente vivo", "agentes vivos"), cuenta(alertas, "alerta", "alertas")}, " · ")
	for _, vp := range vs {
		v.Chips = append(v.Chips, chip{ID: vp.ID, Nombre: vp.Nombre, Activo: !v.Todos && vp.ID == filtro, Alerta: vp.alertas > 0})
		if v.Todos || vp.ID == filtro {
			v.Proyectos = append(v.Proyectos, vp)
		}
	}
	return v
}

//go:embed fragmento.html
var fragmentoHTML string

// Plantilla compilada una sola vez por proceso (RG-1.4).
var plantilla = template.Must(template.New("fragmento").Parse(fragmentoHTML))

// Fragmento escribe el HTML de todos los proyectos (o solo el del filtro): cabecera global, chips de filtro y,
// por proyecto, su árbol + carriles de la ventana.
func Fragmento(w io.Writer, ps []Proyecto, filtro string, ahora time.Time, ventana time.Duration) error {
	return plantilla.Execute(w, armarVista(ps, filtro, ahora, ventana))
}

// ───────────────────────── servidor ─────────────────────────

//go:embed pagina.html
var pagina []byte

//go:embed app.js
var app []byte

var ventanas = map[string]time.Duration{"5m": 5 * time.Minute, "10m": 10 * time.Minute, "30m": 30 * time.Minute}

// Servicio es el nombre con el que /api/salud identifica al dashboard.
const Servicio = "hz-dashboard"

// Salud es la identidad del proceso que sirve: con ella hz dashboard sabe si ya corre y con qué binario.
type Salud struct {
	Version string
	Pid     int
	Binario string
}

func escribirJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// Servidor es el handler HTTP local de solo lectura del dashboard central. Rechaza todo Host que no sea este
// equipo y este puerto (protección contra DNS rebinding) y todo método que no sea GET o HEAD.
func Servidor(c *Central, puerto int, salud Salud) http.Handler {
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
	mux.HandleFunc("/api/salud", func(w http.ResponseWriter, r *http.Request) {
		escribirJSON(w, map[string]any{"servicio": Servicio, "version": salud.Version, "pid": salud.Pid, "binario": salud.Binario})
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
		ahora := time.Now()
		ps, err := c.Proyectos(ahora)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := Fragmento(w, ps, r.URL.Query().Get("proyecto"), ahora, ventana); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/api/panel", func(w http.ResponseWriter, r *http.Request) {
		ahora := time.Now()
		ps, err := c.Proyectos(ahora)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		escribirJSON(w, map[string]any{"generado": ahora.Format(time.RFC3339), "proyectos": ps})
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
