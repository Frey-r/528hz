package informe

import (
	"bytes"
	"encoding/json"
	"errors"
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
	if tasa.Base != 0.3 || tasa.Actual == nil || *tasa.Actual != 0.8 || !tasa.Cumplida || len(tasa.Comparacion) != 2 {
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
