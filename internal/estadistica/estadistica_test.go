package estadistica

import (
	"fmt"
	"math"
	"testing"
)

func binarios(n, verdaderosDesde int) []Item {
	items := make([]Item, n)
	for i := range items {
		v := i >= verdaderosDesde
		items[i] = Item{ID: fmt.Sprintf("t%02d", i), Resultado: &v}
	}
	return items
}

func TestValorEsElPromedio(t *testing.T) {
	if v := Valor(binarios(4, 1)); v != 0.75 {
		t.Fatalf("3 de 4 verdaderos = 0.75, llegó %v", v)
	}
	p1, p2 := 0.2, 0.6
	if v := Valor([]Item{{ID: "a", Puntaje: &p1}, {ID: "b", Puntaje: &p2}}); math.Abs(v-0.4) > 1e-9 {
		t.Fatalf("promedio de puntajes = 0.4, llegó %v", v)
	}
}

func TestSignosExacto(t *testing.T) {
	if p := PSignos(12, 0); math.Abs(p-2*math.Pow(0.5, 12)) > 1e-12 {
		t.Fatalf("12 a 0: p = 2·0.5^12, llegó %v", p)
	}
	if p := PSignos(2, 1); p != 1 {
		t.Fatalf("2 a 1 no es evidencia: p = 1, llegó %v", p)
	}
	if p := PSignos(5, 1); math.Abs(p-14.0/64) > 1e-12 {
		t.Fatalf("5 a 1: p = 2·(1+6)/64, llegó %v", p)
	}
	if p := PSignos(8, 2); math.Abs(p-2*56.0/1024) > 1e-12 {
		t.Fatalf("8 a 2: p = 2·(1+10+45)/1024, llegó %v", p)
	}
	if p := PSignos(0, 0); p != 1 {
		t.Fatalf("sin pares discordantes p = 1, llegó %v", p)
	}
}

func TestCompararMejoraSignificativa(t *testing.T) {
	a, b := binarios(30, 20), binarios(30, 8) // A: 10 verdaderos, B: 22 → 12 ganados, 0 perdidos
	c := Comparar(a, b, "sube", 0.02)
	if c.Pareados != 30 || c.Ganados != 12 || c.Perdidos != 0 || c.Veredicto != "mejora" {
		t.Fatalf("comparación inesperada: %+v", c)
	}
	if math.Abs(c.Delta-0.4) > 1e-9 || c.P > 0.001 {
		t.Fatalf("delta 0.4 y p < 0.001: %+v", c)
	}
}

func TestCompararDentroDelPisoDeRuidoEsRuido(t *testing.T) {
	a, b := binarios(100, 50), binarios(100, 47) // delta 0.03 con piso 0.05
	if c := Comparar(a, b, "sube", 0.05); c.Veredicto != "ruido" {
		t.Fatalf("una diferencia dentro del piso de ruido no es mejora: %+v", c)
	}
}

func TestCompararNoSignificativoEsRuido(t *testing.T) {
	a, b := binarios(10, 7), binarios(10, 5) // 2 ganados, 0 perdidos: delta 0.2 > piso pero p = 0.5
	if c := Comparar(a, b, "sube", 0.01); c.Veredicto != "ruido" {
		t.Fatalf("sin significancia es ruido: %+v", c)
	}
}

func TestCompararRegresionSegunDireccion(t *testing.T) {
	// métrica de fallos (baja es mejor): de 3 fallos a 15 en 30 ítems
	a, b := binarios(30, 27), binarios(30, 15)
	if c := Comparar(a, b, "baja", 0.02); c.Veredicto != "regresion" || c.Ganados != 0 || c.Perdidos != 12 {
		t.Fatalf("subir los fallos es regresión: 12 ítems empeoran y ninguno mejora: %+v", c)
	}
}

func TestCompararSoloParesComunes(t *testing.T) {
	v := true
	a := []Item{{ID: "x", Resultado: &v}, {ID: "solo-a", Resultado: &v}}
	b := []Item{{ID: "x", Resultado: &v}, {ID: "solo-b", Resultado: &v}}
	if c := Comparar(a, b, "sube", 0); c.Pareados != 1 {
		t.Fatalf("solo se comparan ítems presentes en ambas rondas: %+v", c)
	}
}

func TestProponerMilestone(t *testing.T) {
	metricas := []Metrica{{ID: "resolucion", Direccion: "sube", Piso: 0.03}, {ID: "fallos", Direccion: "baja", Piso: 0.02}}
	base := map[string]float64{"resolucion": 0.10, "fallos": 0.20}
	if p := Proponer(base, map[string]float64{"resolucion": 0.22, "fallos": 0.19}, metricas, 2); !p.Propone {
		t.Fatalf("un salto de 0.12 > 2×0.03 sin regresiones se propone: %+v", p)
	}
	if p := Proponer(base, map[string]float64{"resolucion": 0.15, "fallos": 0.20}, metricas, 2); p.Propone {
		t.Fatalf("0.05 < 2×0.03 no alcanza: %+v", p)
	}
	if p := Proponer(base, map[string]float64{"resolucion": 0.30, "fallos": 0.30}, metricas, 2); p.Propone {
		t.Fatalf("una regresión fuera de ruido impide proponer: %+v", p)
	}
}

func TestItemsSinValorSeIgnoran(t *testing.T) {
	v := true
	vacio := Item{ID: "sin-valor"}
	if x := Valor([]Item{{ID: "a", Resultado: &v}, vacio}); x != 1 {
		t.Fatalf("un ítem sin resultado ni puntaje no cuenta en el promedio: %v", x)
	}
	if c := Comparar([]Item{vacio}, []Item{vacio}, "sube", 0); c.Pareados != 0 || c.Veredicto != "sin datos" {
		t.Fatalf("ítems sin valor no se parean: %+v", c)
	}
}
