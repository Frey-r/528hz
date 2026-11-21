package motor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMisionRondasMedicionesGastoYMilestonesSobrevivenReindex(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.NuevaMision(Mision{ID: "m1", Titulo: "m1", Modo: "mejora", Objetivo: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := m.NuevaMision(Mision{ID: "m2", Titulo: "m2", Modo: "mejora", Objetivo: "y"}); err != nil {
		t.Fatal(err)
	}
	r1, err := m.NuevaRonda("m1", []string{"V-a"})
	if err != nil || r1 != "R-1" {
		t.Fatalf("primera ronda: %s %v", r1, err)
	}
	r2, _ := m.NuevaRonda("m1", nil)
	otra, _ := m.NuevaRonda("m2", nil)
	if r2 != "R-2" || otra != "R-1" {
		t.Fatalf("las rondas se numeran por misión: %s %s", r2, otra)
	}
	for _, md := range []Medicion{
		{Mision: "m1", Ronda: "R-1", Metrica: "tasa", Conjunto: "trabajo", Valor: 0.10, N: 20},
		{Mision: "m1", Ronda: "R-2", Metrica: "tasa", Conjunto: "trabajo", Valor: 0.25, N: 20},
	} {
		if err := m.RegistrarMedicionRonda(md); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.CerrarRonda("m1", "R-2", "mantener"); err != nil {
		t.Fatal(err)
	}
	if err := m.RegistrarGasto("m1", 1.25, "ronda 2"); err != nil {
		t.Fatal(err)
	}
	if err := m.RegistrarGasto("m1", 0.75, "evals"); err != nil {
		t.Fatal(err)
	}
	if err := m.Aceptar("m1", "A1", "go test ./... → ok"); err != nil {
		t.Fatal(err)
	}
	if err := m.FijarPiso("m1", "tasa", 0.03); err != nil {
		t.Fatal(err)
	}
	ms, err := m.MarcarMilestone("m1", "R-2", "subió la tasa a 25%", true)
	if err != nil || ms != "MS-1" {
		t.Fatalf("milestone: %s %v", ms, err)
	}
	_ = m.Cerrar()
	_ = os.Remove(filepath.Join(raiz, ".harness", "harness.db"))
	m2 := abrir(t, raiz)
	serie, err := m2.Mediciones("m1", "trabajo")
	if err != nil || len(serie) != 2 || serie[1].Valor != 0.25 {
		t.Fatalf("serie de mediciones: %+v %v", serie, err)
	}
	if g, _ := m2.Gasto("m1"); g != 2.0 {
		t.Fatalf("gasto acumulado = 2.0, llegó %v", g)
	}
	if a, _ := m2.Aceptaciones("m1"); a["A1"] == "" {
		t.Fatalf("aceptación con evidencia: %v", a)
	}
	if p, _ := m2.Piso("m1", "tasa"); p != 0.03 {
		t.Fatalf("piso de ruido: %v", p)
	}
	if hs, _ := m2.Milestones("m1"); len(hs) != 1 || hs[0].Ronda != "R-2" || hs[0].Promocion != "propuesta_aceptada" {
		t.Fatalf("milestones: %+v", hs)
	}
	if rs, _ := m2.Rondas("m1"); len(rs) != 2 || rs[1].Veredicto != "mantener" {
		t.Fatalf("rondas: %+v", rs)
	}
}
