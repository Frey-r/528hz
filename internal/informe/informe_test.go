package informe

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"hz528/internal/mision"
	"hz528/internal/motor"
)

func ejemplo() Informe {
	return Informe{
		Mision: "m1", Titulo: "Resolución autónoma", Objetivo: "subir la resolución", Modo: "mejora", Estado: "activa",
		Gasto: 0.6, UsdMax: 1, Rondas: 3, Generado: "2026-11-20T22:00:00-03:00",
		Milestones: []Hito{{ID: "MS-1", Ronda: "R-2", Resumen: "la tasa subió a 75%"}},
		Metricas: []Metrica{
			{ID: "tasa", Vertical: "Resolución autónoma", Familia: "efectividad", Direccion: "sube", Meta: 0.7, Base: 0.2, Piso: 0.02,
				Serie: []Punto{{Ronda: "R-1", Valor: 0.2}, {Ronda: "R-2", Valor: 0.75, Milestone: "MS-1"}, {Ronda: "R-3", Valor: 0.78}}},
			{ID: "errores", Vertical: "Resolución autónoma", Familia: "fallos", Direccion: "baja", Meta: 0.1, Base: 0.3,
				Serie: []Punto{{Ronda: "R-1", Valor: 0.3}, {Ronda: "R-2", Valor: 0.1, Milestone: "MS-1"}, {Ronda: "R-3", Valor: 0.12}}},
		},
	}
}

func TestHTMLAutocontenidoTemableYConDatos(t *testing.T) {
	var b bytes.Buffer
	if err := HTML(&b, ejemplo()); err != nil {
		t.Fatal(err)
	}
	h := b.String()
	for _, debe := range []string{"<title>Resolución autónoma</title>", "prefers-color-scheme", `data-theme="light"`, "<svg", "MS-1", "78%", "<table", "meta"} {
		if !strings.Contains(h, debe) {
			t.Errorf("el informe debe contener %q", debe)
		}
	}
	for _, prohibido := range []string{"<script", "<!DOCTYPE", "<html", "<body", "ZgotmplZ"} {
		if strings.Contains(strings.ToLower(h), strings.ToLower(prohibido)) {
			t.Errorf("la página de un Artifact no lleva %q (el esqueleto lo pone la publicación, y no necesita JS)", prohibido)
		}
	}
}

func TestKPIsSegunDireccion(t *testing.T) {
	inf := ejemplo()
	tasa, errores := inf.Metricas[0], inf.Metricas[1]
	if !tasa.Cumple() || errores.Cumple() {
		t.Fatal("tasa 0.78 ≥ meta 0.7 (sube) y errores 0.12 > meta 0.1 (baja) no cumple")
	}
}

func TestConstruirDesdeElMotor(t *testing.T) {
	m, err := motor.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	if err := m.NuevaMision(motor.Mision{ID: "m1", Modo: "mejora"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := m.NuevaRonda("m1", nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, md := range []motor.Medicion{{Ronda: "R-1", Valor: 0.2}, {Ronda: "R-1", Valor: 0.3}, {Ronda: "R-2", Valor: 0.8}} {
		md.Mision, md.Metrica, md.N = "m1", "tasa", 20
		if err := m.RegistrarMedicionRonda(md); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.MarcarMilestone("m1", "R-2", "salto", false); err != nil {
		t.Fatal(err)
	}
	def := mision.Mision{ID: "m1", Modo: "mejora", Verticales: []mision.Vertical{{ID: "V-a", Metricas: []mision.Metrica{
		{ID: "tasa", Familia: "efectividad", Direccion: "sube", Meta: 0.7},
		{ID: "errores", Familia: "fallos", Direccion: "baja", Meta: 0.1},
	}}}}
	inf, err := Construir(m, def)
	if err != nil {
		t.Fatal(err)
	}
	if inf.Titulo != "m1" || inf.Estado != "activa" || inf.Rondas != 2 || len(inf.Milestones) != 1 {
		t.Fatalf("cabecera: sin título se usa el id; estado, rondas y milestones del motor: %+v", inf)
	}
	tasa, errores := inf.Metricas[0], inf.Metricas[1]
	if tasa.Vertical != "V-a" {
		t.Fatalf("sin nombre, la vertical se muestra por su id: %q", tasa.Vertical)
	}
	if len(tasa.Serie) != 2 || tasa.Serie[0].Valor != 0.3 || tasa.Serie[1].Milestone != "MS-1" {
		t.Fatalf("una ronda medida dos veces vale por su última medición, y la ronda promovida lleva su milestone: %+v", tasa.Serie)
	}
	if tasa.Base != 0.3 || tasa.Actual == nil || *tasa.Actual != 0.8 || !tasa.Cumplida || len(tasa.Comparacion) != 2 || tasa.Comparacion[1].Milestone != "MS-1" {
		t.Fatalf("sin linea_base, la base es la primera ronda; actual 0.8 cumple la meta: %+v", tasa)
	}
	if errores.Cumplida || errores.Actual != nil {
		t.Fatalf("una métrica sin mediciones nunca cumple, ni siquiera si su meta es bajar: %+v", errores)
	}
	b, err := json.Marshal(errores)
	if err != nil || !strings.Contains(string(b), `"serie":[]`) {
		t.Fatalf("una serie vacía se exporta como [] y no como null (el Artifact la recorre): %s", b)
	}
	if _, err := Construir(m, mision.Mision{ID: "otra"}); !errors.Is(err, motor.ErrNoExiste) {
		t.Fatalf("una misión que el motor no conoce es ErrNoExiste: %v", err)
	}
}

func serie(valores ...float64) []Punto {
	out := make([]Punto, len(valores))
	for i, v := range valores {
		out[i] = Punto{Ronda: "R-" + string(rune('1'+i)), Valor: v}
	}
	return out
}

func TestGeometriaDelGrafico(t *testing.T) {
	uno := armarGrafico(Metrica{ID: "a", Direccion: "sube", Meta: 0.5, Base: 0.4, Serie: serie(0.4)})
	if len(uno.Puntos) != 1 || uno.Puntos[0].X != num(gIzq+gPlotAncho/2) {
		t.Fatalf("un único punto va centrado: %+v", uno.Puntos)
	}
	for _, g := range uno.Grid {
		y, _ := strconv.ParseFloat(g.Y, 64)
		if g.YTexto != num(y+desplazamientoTexto) {
			t.Fatalf("la etiqueta de la grilla va %v bajo su línea: %+v", desplazamientoTexto, g)
		}
	}
	s := serie(1, 0.5, 0.8)
	s[0].Milestone, s[2].Milestone = "MS-1", "MS-2"
	g := armarGrafico(Metrica{ID: "b", Direccion: "sube", Meta: 0.9, Base: 1, Piso: 0.05, Serie: s})
	primero, ultimo := g.Puntos[0], g.Puntos[2]
	if primero.Ancla != "start" || ultimo.Ancla != "end" || g.Puntos[1].Ancla != "middle" {
		t.Fatalf("las etiquetas de los extremos se anclan hacia adentro: %+v", g.Puntos)
	}
	if primero.YTexto != num(gSup-10) {
		t.Fatalf("la etiqueta de un milestone en el tope va encima del punto: %+v", primero)
	}
	if g.Banda == nil || g.Ruta == "" || len(g.Ejes) != 3 {
		t.Fatalf("banda de ruido, línea y eje de rondas: %+v", g)
	}
}

func TestPesaSegunDireccion(t *testing.T) {
	sube := Metrica{ID: "tasa", Direccion: "sube", Base: 0.2, Meta: 0.7, Serie: serie(0.2, 0.6)}
	if p, ok := armarPesa(sube); !ok || p.Clase != "ok" || !strings.Contains(p.Delta, "mejora") || !strings.HasPrefix(p.Delta, "↑") {
		t.Fatalf("subir en una métrica de subir es mejora: %+v", p)
	}
	baja := Metrica{ID: "errores", Direccion: "baja", Base: 0.3, Meta: 0.1, Serie: serie(0.3, 0.5)}
	if p, _ := armarPesa(baja); p.Clase != "crit" || !strings.Contains(p.Delta, "empeora") {
		t.Fatalf("subir en una métrica de bajar es empeorar: %+v", p)
	}
	igual := Metrica{ID: "x", Direccion: "sube", Base: 0.3, Meta: 0.5, Serie: serie(0.3, 0.3)}
	if p, _ := armarPesa(igual); p.Clase != "neutro" || p.Delta != "sin cambio" {
		t.Fatalf("sin movimiento: %+v", p)
	}
	if _, ok := armarPesa(Metrica{ID: "vacia"}); ok {
		t.Fatal("sin mediciones no hay pesa")
	}
}

func TestDominioYFormato(t *testing.T) {
	for _, m := range []Metrica{
		{Meta: 2, Serie: serie(0.5)},
		{Meta: 0.5, Base: -0.1, Serie: serie(0.5)},
		{Meta: 0.5, Serie: serie(1.5)},
	} {
		if m.esProporcion() {
			t.Fatalf("valores fuera de [0, 1] no son proporción: %+v", m)
		}
	}
	lo, hi := Metrica{Meta: 8, Serie: serie(0, 10)}.dominio()
	if lo != 0 || math.Abs(hi-11.5) > 1e-9 {
		t.Fatalf("el dominio deja 15%% de aire arriba: %v %v", lo, hi)
	}
	if lo, _ := (Metrica{Meta: 1, Serie: serie(-2, 3)}).dominio(); lo != -2 {
		t.Fatalf("el dominio baja hasta el mínimo negativo: %v", lo)
	}
	for v, esperado := range map[float64]string{0.755: "75,5%", 0.78: "78%", 0: "0%"} {
		if got := valorTxt(v, true); got != esperado {
			t.Errorf("valorTxt(%v) = %q, se esperaba %q", v, got, esperado)
		}
	}
	if got := valorTxt(1.25, false); got != "1,25" {
		t.Errorf("valorTxt sin proporción: %q", got)
	}
	if deltaTxt(-0.1, true) != "−10 pp" || deltaTxt(0.1, true) != "+10 pp" || deltaTxt(0, true) != "±0" {
		t.Errorf("deltas: %q %q %q", deltaTxt(-0.1, true), deltaTxt(0.1, true), deltaTxt(0, true))
	}
}

func TestEstadoDeCadaKPI(t *testing.T) {
	casos := map[string]Metrica{
		"Cumplida":  {Direccion: "sube", Base: 0.2, Meta: 0.5, Serie: serie(0.2, 0.6)},
		"Retrocede": {Direccion: "sube", Base: 0.4, Meta: 0.8, Piso: 0.02, Serie: serie(0.4, 0.3)},
		"En curso":  {Direccion: "baja", Base: 0.4, Meta: 0.1, Serie: serie(0.4, 0.3)},
		"Sin datos": {Direccion: "sube", Base: 0.4, Meta: 0.8},
	}
	for estado, m := range casos {
		if k := armarKPI(m); k.Estado != estado {
			t.Errorf("%+v debe quedar %q, quedó %q", m, estado, k.Estado)
		}
	}
	if k := armarKPI(casos["En curso"]); k.Avance != "width:33.3%" {
		t.Errorf("de 0,4 a 0,3 con meta 0,1 recorrió un tercio: %q", k.Avance)
	}
}

func TestComparacionSoloConMilestonesYEstadoDelPresupuesto(t *testing.T) {
	m := Metrica{ID: "tasa", Direccion: "sube", Meta: 0.7, Serie: serie(0.2, 0.5, 0.8)}
	m.Serie[1].Milestone = "MS-1"
	inf := Informe{Mision: "m", Titulo: "t", Metricas: []Metrica{m}}
	v := armarVista(inf)
	if mx, _ := strconv.ParseFloat(v.Geo.MetaX, 64); mx <= gIzq+gPlotAncho {
		t.Fatalf("la etiqueta de la meta va a la derecha del gráfico, fuera de la zona de puntos: %v", v.Geo)
	}
	if pf, _ := strconv.ParseFloat(v.Geo.PFin, 64); pf != pIzq+pPlotAncho {
		t.Fatalf("el eje de las pesas termina en pIzq + pPlotAncho: %v", v.Geo.PFin)
	}
	for gasto, clase := range map[float64]string{1: "crit", 1.2: "crit", 0.85: "warn", 0.5: "ok"} {
		v := armarVista(Informe{Gasto: gasto, UsdMax: 1})
		if v.Gasto.Clase != clase {
			t.Errorf("gasto %v de 1: clase %q, se esperaba %q", gasto, v.Gasto.Clase, clase)
		}
	}
	if v := armarVista(Informe{Gasto: 3}); v.Gasto.Clase != "ok" || v.Gasto.ConTope || !strings.Contains(v.Gasto.Texto, "sin tope") {
		t.Fatalf("sin tope no hay barra ni alarma: %+v", v.Gasto)
	}
	uno := serie(0.6)
	uno[0].Milestone = "MS-1"
	if g := armarGrafico(Metrica{ID: "x", Direccion: "sube", Meta: 0.7, Serie: uno}); g.Puntos[0].Ancla != "middle" {
		t.Fatalf("con un solo punto, su etiqueta va centrada: %+v", g.Puntos[0])
	}
	if _, hi := (Metrica{Meta: 1, Serie: serie(-2, 3)}).dominio(); math.Abs(hi-3.75) > 1e-9 {
		t.Fatalf("el aire es 15%% del rango completo (de -2 a 3): %v", hi)
	}
}
