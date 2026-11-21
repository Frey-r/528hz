package motor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func proyectoVacio(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, ".harness", "log"), 0o755); err != nil {
		t.Fatal(err)
	}
	return raiz
}

func abrir(t *testing.T, raiz string) *Motor {
	t.Helper()
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	t.Cleanup(func() { _ = m.Cerrar() })
	return m
}

func decisionBase(que string) Decision {
	return Decision{Que: que, PorQue: "porque sí, con evidencia", Decidio: "agente"}
}

func TestAbrirCreaBaseVacia(t *testing.T) {
	raiz := proyectoVacio(t)
	m := abrir(t, raiz)
	if _, err := os.Stat(filepath.Join(raiz, ".harness", "harness.db")); err != nil {
		t.Fatalf("no se creó harness.db: %v", err)
	}
	n, err := m.ContarDecisiones()
	if err != nil || n != 0 {
		t.Fatalf("se esperaban 0 decisiones, llegó %d (%v)", n, err)
	}
}

func TestNuevaDecisionAsignaCorrelativoYEscribeLog(t *testing.T) {
	raiz := proyectoVacio(t)
	m := abrir(t, raiz)
	id1, err := m.NuevaDecision(decisionBase("primera"))
	if err != nil {
		t.Fatal(err)
	}
	id2, err := m.NuevaDecision(decisionBase("segunda"))
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "D-0001" || id2 != "D-0002" {
		t.Fatalf("IDs = %s, %s; se esperaba D-0001, D-0002", id1, id2)
	}
	log, err := os.ReadFile(filepath.Join(raiz, ".harness", "log", "decisiones.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(log), `"evento":"decision.nueva"`); got != 2 {
		t.Fatalf("el log debe tener 2 eventos decision.nueva, tiene %d:\n%s", got, log)
	}
}

func TestNuevaDecisionValidaCamposObligatorios(t *testing.T) {
	m := abrir(t, proyectoVacio(t))
	_, err := m.NuevaDecision(Decision{Decidio: "agente"})
	if !errors.Is(err, ErrCampos) {
		t.Fatalf("se esperaba ErrCampos, llegó %v", err)
	}
	if !strings.Contains(err.Error(), "que") || !strings.Contains(err.Error(), "por_que") {
		t.Fatalf("el error debe nombrar los campos faltantes: %v", err)
	}
	_, err = m.NuevaDecision(Decision{Que: "x", PorQue: "y", Decidio: "nadie"})
	if !errors.Is(err, ErrCampos) {
		t.Fatalf("decidio inválido debe fallar con ErrCampos, llegó %v", err)
	}
}

func TestBuscarEsInsensibleATildesYOrdenaPorRelevancia(t *testing.T) {
	m := abrir(t, proyectoVacio(t))
	if _, err := m.NuevaDecision(decisionBase("Usar SQLite como motor de decisión")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.NuevaDecision(decisionBase("Informes visuales para gerencia")); err != nil {
		t.Fatal(err)
	}
	res, err := m.BuscarDecisiones("decision motor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].ID != "D-0001" {
		t.Fatalf("se esperaba D-0001 primero, llegó %+v", res)
	}
	vacio, err := m.BuscarDecisiones("inexistente", 5)
	if err != nil || len(vacio) != 0 {
		t.Fatalf("búsqueda sin coincidencias debe ser vacía sin error: %+v %v", vacio, err)
	}
	if _, err := m.BuscarDecisiones(`"(* OR`, 5); err != nil {
		t.Fatalf("la entrada con sintaxis FTS no debe romper la búsqueda: %v", err)
	}
}

func TestVerDecisionConservaCampos(t *testing.T) {
	m := abrir(t, proyectoVacio(t))
	d := decisionBase("Elegir Go para el CLI")
	d.Alternativas = []Alternativa{{Opcion: "Bun + TS", DescartadaPor: "preferencia del usuario"}}
	d.Etiquetas = []string{"cli", "go"}
	d.Reversible = true
	id, err := m.NuevaDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.VerDecision(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Que != d.Que || len(got.Alternativas) != 1 || got.Alternativas[0].DescartadaPor != "preferencia del usuario" || got.Estado != "vigente" || got.Cuando == "" {
		t.Fatalf("ficha inesperada: %+v", got)
	}
	if _, err := m.VerDecision("D-9999"); !errors.Is(err, ErrNoExiste) {
		t.Fatalf("se esperaba ErrNoExiste, llegó %v", err)
	}
}

func TestReindexReconstruyeDesdeLog(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := m.NuevaDecision(decisionBase(fmt.Sprintf("decisión %d", i))); err != nil {
			t.Fatal(err)
		}
	}
	_ = m.Cerrar()
	for _, f := range []string{"harness.db", "harness.db-wal", "harness.db-shm"} {
		_ = os.Remove(filepath.Join(raiz, ".harness", f))
	}
	m2 := abrir(t, raiz)
	n, err := m2.ContarDecisiones()
	if err != nil || n != 3 {
		t.Fatalf("tras borrar la base se esperaban 3 decisiones reconstruidas, hay %d (%v)", n, err)
	}
}

func TestReindexAutomaticoSiElLogCambioPorFuera(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.NuevaDecision(decisionBase("local")); err != nil {
		t.Fatal(err)
	}
	_ = m.Cerrar()
	// Simula un merge: llega una decisión escrita en otra rama.
	externa := `{"evento":"decision.nueva","ts":"2026-11-20T18:00:00-03:00","datos":{"id":"D-0099","cuando":"2026-11-20T18:00:00-03:00","que":"externa","por_que":"merge","decidio":"humano","alternativas":[],"alcance":[],"reversible":true,"estado":"vigente","etiquetas":[]}}` + "\n"
	f, err := os.OpenFile(filepath.Join(raiz, ".harness", "log", "decisiones.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(externa); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	m2 := abrir(t, raiz)
	if _, err := m2.VerDecision("D-0099"); err != nil {
		t.Fatalf("la decisión llegada por fuera debía indexarse sola: %v", err)
	}
}

func TestReindexFallaConLineaInvalidaIndicandoUbicacion(t *testing.T) {
	raiz := proyectoVacio(t)
	if err := os.WriteFile(filepath.Join(raiz, ".harness", "log", "decisiones.jsonl"), []byte("esto no es json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Abrir(raiz)
	if err == nil || !strings.Contains(err.Error(), "decisiones.jsonl:1") {
		t.Fatalf("se esperaba un error con archivo:línea, llegó %v", err)
	}
}

func TestConcurrenciaEntreProcesosAsignaIDsUnicos(t *testing.T) {
	raiz := proyectoVacio(t)
	// Dos motores independientes simulan dos agentes del enjambre escribiendo a la vez.
	a, b := abrir(t, raiz), abrir(t, raiz)
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	errs := make(chan error, 20)
	for _, m := range []*Motor{a, b} {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(m *Motor, i int) {
				defer wg.Done()
				id, err := m.NuevaDecision(decisionBase(fmt.Sprintf("paralela %d", i)))
				if err != nil {
					errs <- err
					return
				}
				ids <- id
			}(m, i)
		}
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatalf("escritura concurrente falló: %v", err)
	}
	vistos := map[string]bool{}
	for id := range ids {
		if vistos[id] {
			t.Fatalf("ID duplicado: %s", id)
		}
		vistos[id] = true
	}
	if len(vistos) != 20 {
		t.Fatalf("se esperaban 20 IDs únicos, hay %d", len(vistos))
	}
	c := abrir(t, raiz) // un tercer proceso ve todo tras reindex automático
	if n, _ := c.ContarDecisiones(); n != 20 {
		t.Fatalf("se esperaban 20 decisiones indexadas, hay %d", n)
	}
}

func TestTareasPersistenYSobrevivenReindex(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	ta := Tarea{ID: "T-1", Titulo: "sumar", Estado: "contrato", ContratoRuta: "contratos/sumar.md"}
	if err := m.RegistrarTarea("tdd.contrato", ta); err != nil {
		t.Fatal(err)
	}
	ta.Estado = "rojo"
	ta.TestsBloqueados = []TestBloqueado{{Ruta: "x_test.go", SHA256: "abc"}}
	if err := m.RegistrarTarea("tdd.rojo", ta); err != nil {
		t.Fatal(err)
	}
	_ = m.Cerrar()
	_ = os.Remove(filepath.Join(raiz, ".harness", "harness.db"))
	m2 := abrir(t, raiz)
	got, err := m2.Tarea("T-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Estado != "rojo" || len(got.TestsBloqueados) != 1 || got.TestsBloqueados[0].SHA256 != "abc" {
		t.Fatalf("tarea reconstruida inesperada: %+v", got)
	}
	if _, err := m2.Tarea("T-404"); !errors.Is(err, ErrNoExiste) {
		t.Fatalf("se esperaba ErrNoExiste, llegó %v", err)
	}
}
