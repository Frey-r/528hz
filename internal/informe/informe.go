// Package informe arma el informe C-level de una misión (docs/diseno/09-informes.md): datos deterministas
// del motor y una página autocontenida (SVG calculado aquí, sin JS) con el design system del harness.
package informe

import (
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"hz528/internal/mision"
	"hz528/internal/motor"
)

// Punto de la serie de una métrica: una ronda y, si la ronda fue promovida, su milestone.
type Punto struct {
	Ronda     string  `json:"ronda"`
	Valor     float64 `json:"valor"`
	Milestone string  `json:"milestone,omitempty"`
}

// Metrica con su serie por ronda.
type Metrica struct {
	ID        string  `json:"id"`
	Vertical  string  `json:"vertical"`
	Familia   string  `json:"familia"`
	Direccion string  `json:"direccion"`
	Meta      float64 `json:"meta"`
	Base      float64 `json:"linea_base"`
	Piso      float64 `json:"piso_ruido"`
	Serie     []Punto `json:"serie"`
	// Calculados por Construir para quien consume el JSON (el HTML los deriva de la serie).
	Actual      *float64 `json:"actual"`
	Cumplida    bool     `json:"cumplida"`
	Comparacion []Punto  `json:"comparacion_milestones"`
}

// Hito es un milestone tal como lo cuenta el informe.
type Hito struct {
	ID      string `json:"id"`
	Ronda   string `json:"ronda"`
	Resumen string `json:"resumen"`
}

// Informe es todo lo que el informe muestra; los números salen del motor, nunca del modelo.
type Informe struct {
	Mision     string    `json:"mision"`
	Titulo     string    `json:"titulo"`
	Objetivo   string    `json:"objetivo"`
	Modo       string    `json:"modo"`
	Estado     string    `json:"estado"`
	Gasto      float64   `json:"gasto_usd"`
	UsdMax     float64   `json:"usd_max"`
	Rondas     int       `json:"rondas"`
	Generado   string    `json:"generado"`
	Milestones []Hito    `json:"milestones"`
	Metricas   []Metrica `json:"metricas"`
}

const eps = 1e-9

func (m Metrica) ultimo() (float64, bool) {
	if len(m.Serie) == 0 {
		return 0, false
	}
	return m.Serie[len(m.Serie)-1].Valor, true
}

// favorable orienta un delta: positivo siempre es mejor, sea la métrica de subir o de bajar.
func (m Metrica) favorable(d float64) float64 {
	if m.Direccion == "baja" {
		return -d
	}
	return d
}

// Cumple indica si el último valor alcanzó la meta según la dirección.
func (m Metrica) Cumple() bool {
	v, hay := m.ultimo()
	return hay && m.favorable(v-m.Meta) >= -eps
}

// Construir arma el informe con los datos del motor y la definición de la misión.
func Construir(m *motor.Motor, def mision.Mision) (Informe, error) {
	inf := Informe{Mision: def.ID, Titulo: def.Titulo, Objetivo: def.Objetivo, Modo: def.Modo, UsdMax: def.Presupuesto.UsdMax,
		Generado: time.Now().Format(time.RFC3339), Milestones: []Hito{}, Metricas: []Metrica{}}
	if inf.Titulo == "" {
		inf.Titulo = def.ID
	}
	misiones, err := m.Misiones()
	if err != nil {
		return inf, err
	}
	for _, x := range misiones {
		if x.ID == def.ID {
			inf.Estado = x.Estado
		}
	}
	if inf.Estado == "" {
		return inf, fmt.Errorf("misión %s: %w", def.ID, motor.ErrNoExiste)
	}
	if inf.Gasto, err = m.Gasto(def.ID); err != nil {
		return inf, err
	}
	rondas, err := m.Rondas(def.ID)
	if err != nil {
		return inf, err
	}
	inf.Rondas = len(rondas)
	mss, err := m.Milestones(def.ID)
	if err != nil {
		return inf, err
	}
	porRonda := make(map[string]string, len(mss))
	for _, x := range mss {
		inf.Milestones = append(inf.Milestones, Hito{ID: x.ID, Ronda: x.Ronda, Resumen: x.Resumen})
		porRonda[x.Ronda] = x.ID
	}
	meds, err := m.Mediciones(def.ID, "trabajo")
	if err != nil {
		return inf, err
	}
	series := map[string][]Punto{}
	for _, md := range meds {
		s, p := series[md.Metrica], Punto{Ronda: md.Ronda, Valor: md.Valor, Milestone: porRonda[md.Ronda]}
		if n := len(s); n > 0 && s[n-1].Ronda == md.Ronda {
			s[n-1] = p // la última medición de la ronda manda
		} else {
			s = append(s, p)
		}
		series[md.Metrica] = s
	}
	for _, v := range def.Verticales {
		nombre := v.Nombre
		if nombre == "" {
			nombre = v.ID
		}
		for _, mt := range v.Metricas {
			x := Metrica{ID: mt.ID, Vertical: nombre, Familia: mt.Familia, Direccion: mt.Direccion, Meta: mt.Meta,
				Serie: series[mt.ID], Comparacion: []Punto{}}
			if x.Serie == nil {
				x.Serie = []Punto{}
			}
			switch {
			case mt.LineaBase != nil:
				x.Base = *mt.LineaBase
			case len(x.Serie) > 0:
				x.Base = x.Serie[0].Valor
			}
			piso, err := m.Piso(def.ID, mt.ID)
			switch {
			case err == nil:
				x.Piso = piso
			case errors.Is(err, motor.ErrNoExiste):
				if mt.PisoRuido != nil {
					x.Piso = *mt.PisoRuido
				}
			default:
				return inf, err
			}
			if v, hay := x.ultimo(); hay {
				x.Actual = &v
				x.Comparacion = append(x.Comparacion, Punto{Ronda: "linea_base", Valor: x.Base})
				for _, p := range x.Serie {
					if p.Milestone != "" {
						x.Comparacion = append(x.Comparacion, p)
					}
				}
			}
			x.Cumplida = x.Cumple()
			inf.Metricas = append(inf.Metricas, x)
		}
	}
	return inf, nil
}

// ───────────────────────── página ─────────────────────────

//go:embed plantilla.html
var plantillaHTML string

// La plantilla se compila una sola vez por proceso (RG-1.4).
var plantilla = template.Must(template.New("informe").Parse(plantillaHTML))

// HTML escribe la página autocontenida del informe (sin doctype ni <html>: la pone la publicación del Artifact).
func HTML(w io.Writer, inf Informe) error {
	return plantilla.Execute(w, armarVista(inf))
}

// Geometría de los gráficos (unidades del viewBox).
const (
	gAncho, gAlto       = 320.0, 168.0
	gIzq, gDer          = 44.0, 64.0 // a la derecha va la etiqueta de la meta, despejada del último punto
	gSup, gInf          = 20.0, 26.0
	gPlotAncho          = gAncho - gIzq - gDer
	gPlotAlto           = gAlto - gSup - gInf
	pAncho              = 480.0 // pesas: viewBox ancho y bajo para que no se agiganten en columnas anchas
	pIzq, pPlotAncho    = 80.0, 320.0
	maxEtiquetasRondas  = 6
	umbralCercaDelTope  = 0.8
	sinCambioPuntosPct  = 0.0005
	desplazamientoTexto = 3.5
)

type stat struct{ Etiqueta, Valor, Nota string }

type kpi struct {
	ID, Vertical, Familia, Valor, Base, Meta, Delta, Sentido string
	Estado, Clase, Icono, AvanceTexto                        string
	Avance                                                   template.CSS
}

type marca struct{ Y, YTexto, Texto string }

type banda struct{ Y, Alto, Titulo string }

type punto struct{ X, Y, YTexto, Titulo, Milestone, Ancla string }

type etiqueta struct{ X, Texto string }

type grafico struct {
	ID, Vertical, Resumen string
	Grid                  []marca
	Banda                 *banda
	Meta                  marca
	Ruta                  string
	Puntos                []punto
	Ejes                  []etiqueta
	Vacio                 bool
}

type pesa struct {
	ID, Tramo, Resumen, Delta, Clase string
	X1, X2, LX1, LX2, A1, A2         string
	Txt1, Txt2, Titulo1, Titulo2     string
	Medios                           []punto
}

type celda struct{ Valor, Milestone string }

type fila struct {
	ID, Familia, Sentido, Base, Meta, Estado string
	Celdas                                   []celda
}

// geometria lleva a la plantilla las mismas constantes con que se calcularon los puntos.
type geometria struct{ Izq, Fin, Ancho, MetaX, EjeX, EjeY, CentroX, CentroY, PIzq, PFin string }

var geo = geometria{Izq: num(gIzq), Fin: num(gIzq + gPlotAncho), Ancho: num(gPlotAncho), MetaX: num(gIzq + gPlotAncho + 8),
	EjeX: num(gIzq - 6), EjeY: num(gAlto - 8), CentroX: num(gIzq + gPlotAncho/2), CentroY: num(gSup + gPlotAlto/2),
	PIzq: num(pIzq), PFin: num(pIzq + pPlotAncho)}

type vista struct {
	Geo                                             geometria
	Titulo, Sobre, Objetivo, Lede, Mision, Generado string
	Stats                                           []stat
	Gasto                                           struct {
		Clase, Texto string
		Ancho        template.CSS
		ConTope      bool
	}
	KPIs                      []kpi
	Graficos                  []grafico
	Pesas                     []pesa
	TituloPesas, LeyendaHasta string
	Hitos                     []Hito
	Rondas                    []string
	Filas                     []fila
}

func armarVista(inf Informe) vista {
	v := vista{Geo: geo, Titulo: inf.Titulo, Objetivo: inf.Objetivo, Mision: inf.Mision, Hitos: inf.Milestones,
		Sobre:    fmt.Sprintf("Misión %s · modo %s · %s", inf.Mision, inf.Modo, inf.Estado),
		Generado: inf.Generado}
	if t, err := time.Parse(time.RFC3339, inf.Generado); err == nil {
		v.Generado = t.Format("2006-01-02 15:04")
	}
	cumplen := 0
	for _, m := range inf.Metricas {
		if m.Cumple() {
			cumplen++
		}
		v.KPIs = append(v.KPIs, armarKPI(m))
		v.Graficos = append(v.Graficos, armarGrafico(m))
		if p, ok := armarPesa(m); ok {
			v.Pesas = append(v.Pesas, p)
		}
	}
	v.Lede = lede(inf, cumplen)
	v.TituloPesas, v.LeyendaHasta = "De la línea base al estado actual", "actual"
	if len(inf.Milestones) > 0 {
		v.TituloPesas, v.LeyendaHasta = "De la línea base al último milestone", "último milestone"
	}
	ms := "ninguno aún"
	if n := len(inf.Milestones); n > 0 {
		ultimo := inf.Milestones[n-1]
		ms = fmt.Sprintf("último: %s · %s", ultimo.ID, ultimo.Ronda)
	}
	v.Stats = []stat{
		{"Métricas en meta", fmt.Sprintf("%d de %d", cumplen, len(inf.Metricas)), ""},
		{"Milestones", strconv.Itoa(len(inf.Milestones)), ms},
		{"Rondas", strconv.Itoa(inf.Rondas), ""},
		{"Gasto", usd(inf.Gasto), ""},
	}
	v.Gasto.Clase, v.Gasto.Texto = "ok", "sin tope de presupuesto definido"
	if inf.UsdMax > 0 {
		r := inf.Gasto / inf.UsdMax
		v.Gasto.ConTope, v.Gasto.Ancho = true, template.CSS(fmt.Sprintf("width:%.1f%%", math.Min(1, r)*100))
		v.Stats[3].Nota = fmt.Sprintf("de %s · %.0f%%", usd(inf.UsdMax), r*100)
		switch {
		case r >= 1:
			v.Gasto.Clase, v.Gasto.Texto = "crit", "✕ sin presupuesto: el harness frena nuevos subagentes hasta que el humano lo amplíe"
		case r >= umbralCercaDelTope:
			v.Gasto.Clase, v.Gasto.Texto = "warn", "▲ cerca del tope de presupuesto"
		default:
			v.Gasto.Texto = "✓ dentro del presupuesto"
		}
	} else {
		v.Stats[3].Nota = "sin tope definido"
	}
	v.Rondas, v.Filas = armarTabla(inf.Metricas)
	return v
}

func lede(inf Informe, cumplen int) string {
	total := len(inf.Metricas)
	var b strings.Builder
	switch {
	case total == 0:
		b.WriteString("La misión aún no tiene verticales medibles")
	case cumplen == total:
		b.WriteString("Todas las métricas cumplen su meta")
	default:
		fmt.Fprintf(&b, "%d de %d métricas cumplen su meta", cumplen, total)
	}
	if n := len(inf.Milestones); n > 0 {
		ms := inf.Milestones[n-1]
		fmt.Fprintf(&b, "; el último salto destacado es %s (%s): %s", ms.ID, ms.Ronda, ms.Resumen)
	}
	b.WriteString(".")
	return b.String()
}

func armarKPI(m Metrica) kpi {
	pct := m.esProporcion()
	k := kpi{ID: m.ID, Vertical: m.Vertical, Familia: m.Familia, Sentido: sentido(m.Direccion),
		Base: "línea base " + valorTxt(m.Base, pct), Meta: "meta " + valorTxt(m.Meta, pct)}
	v, hay := m.ultimo()
	if !hay {
		k.Valor, k.Delta, k.Estado, k.Clase, k.Icono = "—", "aún sin mediciones", "Sin datos", "neutro", "○"
		k.Avance, k.AvanceTexto = "width:0%", "sin avance medido"
		return k
	}
	k.Valor, k.Delta = valorTxt(v, pct), deltaTxt(v-m.Base, pct)+" vs. línea base"
	switch {
	case m.Cumple():
		k.Estado, k.Clase, k.Icono = "Cumplida", "ok", "✓"
	case m.favorable(v-m.Base) < -m.Piso-eps:
		k.Estado, k.Clase, k.Icono = "Retrocede", "crit", "✕"
	default:
		k.Estado, k.Clase, k.Icono = "En curso", "neutro", "→"
	}
	avance := 0.0
	switch {
	case m.Cumple():
		avance = 1
	case math.Abs(m.Meta-m.Base) > eps:
		avance = math.Max(0, math.Min(1, (v-m.Base)/(m.Meta-m.Base)))
	}
	k.Avance = template.CSS(fmt.Sprintf("width:%.1f%%", avance*100))
	k.AvanceTexto = fmt.Sprintf("%.0f%% del camino entre la línea base y la meta", avance*100)
	return k
}

func armarGrafico(m Metrica) grafico {
	pct := m.esProporcion()
	lo, hi := m.dominio()
	y := func(v float64) float64 {
		return gSup + gPlotAlto*(1-(math.Max(lo, math.Min(hi, v))-lo)/(hi-lo))
	}
	g := grafico{ID: m.ID, Vertical: m.Vertical, Vacio: len(m.Serie) == 0}
	for _, v := range []float64{hi, (lo + hi) / 2, lo} {
		yy := y(v)
		g.Grid = append(g.Grid, marca{Y: num(yy), YTexto: num(yy + desplazamientoTexto), Texto: valorTxt(v, pct)})
	}
	ym := y(m.Meta)
	g.Meta = marca{Y: num(ym), YTexto: num(ym + desplazamientoTexto), Texto: "meta " + valorTxt(m.Meta, pct)}
	if g.Vacio {
		g.Resumen = m.ID + ": aún sin mediciones"
		return g
	}
	if m.Piso > 0 {
		arriba, abajo := y(m.Base+m.Piso), y(m.Base-m.Piso)
		g.Banda = &banda{Y: num(arriba), Alto: num(math.Max(1, abajo-arriba)),
			Titulo: fmt.Sprintf("banda de ruido: línea base %s ± %s", valorTxt(m.Base, pct), magnitudTxt(m.Piso, pct))}
	}
	n := len(m.Serie)
	x := func(i int) float64 {
		if n == 1 {
			return gIzq + gPlotAncho/2
		}
		return gIzq + gPlotAncho*float64(i)/float64(n-1)
	}
	paso := max(1, int(math.Ceil(float64(n)/maxEtiquetasRondas)))
	var ruta strings.Builder
	for i, p := range m.Serie {
		xx, yy := x(i), y(p.Valor)
		if i == 0 {
			fmt.Fprintf(&ruta, "M%s %s", num(xx), num(yy))
		} else {
			fmt.Fprintf(&ruta, " L%s %s", num(xx), num(yy))
		}
		pt := punto{X: num(xx), Y: num(yy), Milestone: p.Milestone, Ancla: "middle",
			Titulo: fmt.Sprintf("%s · %s", p.Ronda, valorTxt(p.Valor, pct))}
		if p.Milestone != "" {
			pt.Titulo += " · " + p.Milestone
			ty := yy - 10
			if ty < 10 {
				ty = yy + 18
			}
			pt.YTexto = num(ty)
			switch {
			case n > 1 && i == n-1:
				pt.Ancla = "end"
			case n > 1 && i == 0:
				pt.Ancla = "start"
			}
		}
		g.Puntos = append(g.Puntos, pt)
		if i%paso == 0 {
			g.Ejes = append(g.Ejes, etiqueta{X: num(xx), Texto: p.Ronda})
		}
	}
	g.Ruta = ruta.String()
	primero, ultimo := m.Serie[0], m.Serie[n-1]
	g.Resumen = fmt.Sprintf("%s: %s en %s → %s en %s; meta %s", m.ID, valorTxt(primero.Valor, pct), primero.Ronda,
		valorTxt(ultimo.Valor, pct), ultimo.Ronda, valorTxt(m.Meta, pct))
	return g
}

// armarPesa compara la línea base con el último milestone (o con el valor actual si aún no hay milestones);
// los milestones intermedios quedan como puntos sobre el tramo.
func armarPesa(m Metrica) (pesa, bool) {
	hasta, hay := m.ultimo()
	if !hay {
		return pesa{}, false
	}
	pct := m.esProporcion()
	lo, hi := m.dominio()
	x := func(v float64) float64 { return pIzq + pPlotAncho*(math.Max(lo, math.Min(hi, v))-lo)/(hi-lo) }
	etHasta := "actual (" + m.Serie[len(m.Serie)-1].Ronda + ")"
	var ms []Punto
	for _, p := range m.Serie {
		if p.Milestone != "" {
			ms = append(ms, p)
		}
	}
	p := pesa{ID: m.ID}
	if n := len(ms); n > 0 {
		hasta, etHasta = ms[n-1].Valor, ms[n-1].Milestone
		for _, q := range ms[:n-1] {
			p.Medios = append(p.Medios, punto{X: num(x(q.Valor)), Titulo: fmt.Sprintf("%s · %s · %s", q.Milestone, q.Ronda, valorTxt(q.Valor, pct))})
		}
	}
	x1, x2 := x(m.Base), x(hasta)
	p.X1, p.X2 = num(x1), num(x2)
	p.LX1, p.A1, p.LX2, p.A2 = num(x1-10), "end", num(x2+10), "start"
	if x1 > x2 {
		p.LX1, p.A1, p.LX2, p.A2 = num(x1+10), "start", num(x2-10), "end"
	}
	p.Txt1, p.Txt2 = valorTxt(m.Base, pct), valorTxt(hasta, pct)
	p.Titulo1, p.Titulo2 = "línea base · "+p.Txt1, etHasta+" · "+p.Txt2
	p.Tramo = "línea base → " + etHasta
	d := hasta - m.Base
	switch f := m.favorable(d); {
	case f > sinCambioPuntosPct:
		p.Clase, p.Delta = "ok", flecha(d)+" "+deltaTxt(d, pct)+" · mejora"
	case f < -sinCambioPuntosPct:
		p.Clase, p.Delta = "crit", flecha(d)+" "+deltaTxt(d, pct)+" · empeora"
	default:
		p.Clase, p.Delta = "neutro", "sin cambio"
	}
	p.Resumen = fmt.Sprintf("%s: línea base %s → %s %s", m.ID, p.Txt1, etHasta, p.Txt2)
	return p, true
}

func armarTabla(ms []Metrica) ([]string, []fila) {
	vistas := map[string]bool{}
	var rondas []string
	for _, m := range ms {
		for _, p := range m.Serie {
			if !vistas[p.Ronda] {
				vistas[p.Ronda] = true
				rondas = append(rondas, p.Ronda)
			}
		}
	}
	sort.Slice(rondas, func(i, j int) bool { return numeroRonda(rondas[i]) < numeroRonda(rondas[j]) })
	filas := make([]fila, 0, len(ms))
	for _, m := range ms {
		pct := m.esProporcion()
		f := fila{ID: m.ID, Familia: m.Familia, Sentido: sentido(m.Direccion), Base: valorTxt(m.Base, pct),
			Meta: valorTxt(m.Meta, pct), Estado: armarKPI(m).Estado}
		porRonda := make(map[string]Punto, len(m.Serie))
		for _, p := range m.Serie {
			porRonda[p.Ronda] = p
		}
		for _, r := range rondas {
			c := celda{Valor: "—"}
			if p, ok := porRonda[r]; ok {
				c = celda{Valor: valorTxt(p.Valor, pct), Milestone: p.Milestone}
			}
			f.Celdas = append(f.Celdas, c)
		}
		filas = append(filas, f)
	}
	return rondas, filas
}

// ───────────────────────── formato ─────────────────────────

// esProporcion: los sensores emiten resultados o puntajes en [0, 1], así que casi siempre es una proporción.
func (m Metrica) esProporcion() bool {
	if m.Meta < 0 || m.Meta > 1 || m.Base < 0 || m.Base > 1 {
		return false
	}
	for _, p := range m.Serie {
		if p.Valor < 0 || p.Valor > 1 {
			return false
		}
	}
	return true
}

func (m Metrica) dominio() (float64, float64) {
	if m.esProporcion() {
		return 0, 1
	}
	lo, hi := math.Min(0, math.Min(m.Meta, m.Base)), math.Max(m.Meta, m.Base)
	for _, p := range m.Serie {
		lo, hi = math.Min(lo, p.Valor), math.Max(hi, p.Valor)
	}
	hi += (hi - lo) * 0.15
	if hi-lo < eps {
		hi = lo + 1
	}
	return lo, hi
}

func numeroRonda(r string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(r), "R-"))
	return n
}

func sentido(direccion string) string {
	if direccion == "baja" {
		return "↓ menos es mejor"
	}
	return "↑ más es mejor"
}

// flecha sigue el signo del número; si eso es bueno o malo lo dice la palabra (mejora/empeora).
func flecha(d float64) string {
	if d < 0 {
		return "↓"
	}
	return "↑"
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// decimal escribe con coma decimal y sin ceros sobrantes.
func decimal(v float64, decimales int) string {
	s := strconv.FormatFloat(v, 'f', decimales, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		s = "0"
	}
	return strings.Replace(s, ".", ",", 1)
}

func valorTxt(v float64, pct bool) string {
	if pct {
		return decimal(v*100, 1) + "%"
	}
	return decimal(v, 2)
}

func magnitudTxt(v float64, pct bool) string {
	if pct {
		return decimal(math.Abs(v)*100, 1) + " pp"
	}
	return decimal(math.Abs(v), 2)
}

func deltaTxt(d float64, pct bool) string {
	switch {
	case d > eps:
		return "+" + magnitudTxt(d, pct)
	case d < -eps:
		return "−" + magnitudTxt(d, pct)
	}
	return "±0"
}

func usd(v float64) string {
	return "US$" + strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}
