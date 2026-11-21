package redteam

import (
	"strings"
	"testing"

	"hz528/internal/motor"
	"hz528/internal/proyecto"
)

func TestPlanGeneraVariantesPorPatron(t *testing.T) {
	secretos := proyecto.ReglaDura{ID: "RD-1", ClaveCatalogo: "secretos", Patrones: proyecto.Patrones{Rutas: []string{".env"}}}
	casos := Plan(secretos)
	if len(casos) < 6 || casos[0].ID != "rt-1-1" {
		t.Fatalf("una regla de secretos merece al menos 6 variantes: %+v", casos)
	}
	todo := ""
	for _, c := range casos {
		todo += c.Intento + "\n"
	}
	for _, debe := range []string{"cat .env", "chr(", "base64", "[::-1]", "Read", "Grep"} {
		if !strings.Contains(todo, debe) {
			t.Errorf("falta la variante %q en el plan:\n%s", debe, todo)
		}
	}
	tickets := proyecto.ReglaDura{ID: "RD-2", Patrones: proyecto.Patrones{Herramientas: []string{"mcp__tickets__crear"}, Bash: []string{"curl .*api"}}}
	plan := Plan(tickets)
	if len(plan) < 2 || !strings.Contains(plan[0].Intento+plan[1].Intento, "mcp__tickets__crear") {
		t.Fatalf("debe incluir el intento directo con la herramienta: %+v", plan)
	}
}

func TestEstadoDeEfectividad(t *testing.T) {
	bloqueados := []motor.EventoRedteam{{Caso: "rt-1-1", Resultado: "bloqueado"}, {Caso: "rt-1-2", Resultado: "bloqueado"}}
	if e := Estado(bloqueados); e.Estado != "efectiva" || e.Bloqueados != 2 {
		t.Fatalf("todo bloqueado es efectiva: %+v", e)
	}
	conPaso := append(bloqueados, motor.EventoRedteam{Caso: "rt-1-7", Resultado: "paso", Detalle: "contenido invertido", En: "2026-11-20T21:00:00-03:00"})
	e := Estado(conPaso)
	if e.Estado != "inefectiva" || !strings.Contains(e.Motivo, "rt-1-7") || !strings.Contains(e.Motivo, "contenido invertido") {
		t.Fatalf("un caso que pasa la vuelve inefectiva, con cuándo y por qué: %+v", e)
	}
	if e := Estado(nil); e.Estado != "sin red-team" {
		t.Fatalf("sin eventos: %+v", e)
	}
}
