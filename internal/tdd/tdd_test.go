package tdd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hz528/internal/motor"
)

func TestClasificarRojo(t *testing.T) {
	casos := []struct {
		nombre string
		r      Resultado
		ok     bool
	}{
		{"pasan", Resultado{Codigo: 0, Salida: "ok"}, false},
		{"asercion python", Resultado{Codigo: 1, Salida: "AssertionError: esperado 3, llegó 2"}, true},
		{"asercion go", Resultado{Codigo: 1, Salida: "--- FAIL: TestSumar (0.00s)\n    sumar_test.go:9: got 2 want 3\nFAIL"}, true},
		{"import python", Resultado{Codigo: 1, Salida: "ModuleNotFoundError: No module named 'app'"}, false},
		{"compilacion go", Resultado{Codigo: 1, Salida: "# hz528/x\n./sumar_test.go:5:9: undefined: Sumar\nFAIL\thz528/x [build failed]"}, false},
		{"sintaxis js", Resultado{Codigo: 1, Salida: "SyntaxError: Unexpected token '}'"}, false},
		{"modulo js", Resultado{Codigo: 1, Salida: "Error: Cannot find module './sumar'"}, false},
	}
	for _, c := range casos {
		ok, motivo := ClasificarRojo(c.r)
		if ok != c.ok {
			t.Errorf("%s: ok=%v (motivo %q), se esperaba %v", c.nombre, ok, motivo, c.ok)
		}
		if !ok && motivo == "" {
			t.Errorf("%s: un rechazo debe explicar su motivo", c.nombre)
		}
	}
}

// proyectoTDD arma un proyecto con un contrato y un test, y abre el motor.
func proyectoTDD(t *testing.T) (Maquina, string) {
	t.Helper()
	raiz := t.TempDir()
	for _, d := range []string{".harness/log", "contratos", "tests"} {
		if err := os.MkdirAll(filepath.Join(raiz, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(raiz, "contratos", "sumar.md"), []byte("sumar(a,b) = a+b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "tests", "sumar_test.txt"), []byte("assert sumar(1,2)==3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Cerrar() })
	return Maquina{M: m, Raiz: raiz, CmdTest: "exit 0"}, raiz
}

const (
	cmdRojoBueno   = "echo 'AssertionError: esperado 3'; exit 1"
	cmdRojoImport  = "echo 'ModuleNotFoundError: No module named app'; exit 1"
	cmdVerde       = "echo ok; exit 0"
	cmdFalla       = "echo 'FAIL'; exit 1"
	testsDeLaTarea = "tests/sumar_test.txt"
)

func TestCicloCompletoYExportaHooks(t *testing.T) {
	q, raiz := proyectoTDD(t)
	if err := q.Contrato("T-1", "sumar", "contratos/sumar.md", true, ""); err != nil {
		t.Fatalf("contrato: %v", err)
	}
	if err := q.Rojo("T-1", []string{testsDeLaTarea}, cmdRojoBueno); err != nil {
		t.Fatalf("rojo: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(raiz, ".harness", "cache", "hooks.json"))
	if err != nil {
		t.Fatalf("no se exportó hooks.json: %v", err)
	}
	var h struct {
		TestsBloqueados []string `json:"tests_bloqueados"`
	}
	if err := json.Unmarshal(b, &h); err != nil || len(h.TestsBloqueados) != 1 || h.TestsBloqueados[0] != testsDeLaTarea {
		t.Fatalf("hooks.json inesperado: %s (%v)", b, err)
	}
	if err := q.Verde("T-1", cmdVerde); err != nil {
		t.Fatalf("verde: %v", err)
	}
	if err := q.Refactor("T-1", cmdVerde); err != nil {
		t.Fatalf("refactor: %v", err)
	}
	ta, err := q.M.Tarea("T-1")
	if err != nil || ta.Estado != "refactor" || ta.RojoEn == "" || ta.VerdeEn == "" || len(ta.Evidencia) < 2 {
		t.Fatalf("tarea final inesperada: %+v (%v)", ta, err)
	}
}

func TestContratoExigeRutaExistenteYIdUnico(t *testing.T) {
	q, _ := proyectoTDD(t)
	if err := q.Contrato("T-1", "x", "contratos/no-existe.md", false, ""); !errors.Is(err, ErrVerificacion) {
		t.Fatalf("contrato inexistente debe fallar con ErrVerificacion, llegó %v", err)
	}
	if err := q.Contrato("T-1", "x", "contratos/sumar.md", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Contrato("T-1", "x", "contratos/sumar.md", false, ""); !errors.Is(err, ErrVerificacion) {
		t.Fatalf("tarea duplicada debe fallar, llegó %v", err)
	}
}

func TestRojoRechazaRazonEquivocadaYTestsQuePasan(t *testing.T) {
	q, _ := proyectoTDD(t)
	if err := q.Contrato("T-1", "x", "contratos/sumar.md", false, ""); err != nil {
		t.Fatal(err)
	}
	err := q.Rojo("T-1", []string{testsDeLaTarea}, cmdRojoImport)
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), "razón equivocada") {
		t.Fatalf("error de import debe rechazarse por razón equivocada, llegó %v", err)
	}
	err = q.Rojo("T-1", []string{testsDeLaTarea}, cmdVerde)
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), "no hay ROJO") {
		t.Fatalf("tests que pasan deben rechazarse, llegó %v", err)
	}
	ta, _ := q.M.Tarea("T-1")
	if ta.Estado != "contrato" {
		t.Fatalf("un ROJO rechazado no debe avanzar el estado: %s", ta.Estado)
	}
}

func TestVerdeDetectaTestBloqueadoModificado(t *testing.T) {
	q, raiz := proyectoTDD(t)
	if err := q.Contrato("T-1", "x", "contratos/sumar.md", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Rojo("T-1", []string{testsDeLaTarea}, cmdRojoBueno); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "tests", "sumar_test.txt"), []byte("assert True  # debilitado\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := q.Verde("T-1", cmdVerde)
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), "sumar_test.txt") {
		t.Fatalf("modificar un test bloqueado debe detectarse nombrando el archivo, llegó %v", err)
	}
}

func TestVerdeFallaSiElComandoFallaYRespetaElOrden(t *testing.T) {
	q, _ := proyectoTDD(t)
	if err := q.Contrato("T-1", "x", "contratos/sumar.md", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Verde("T-1", cmdVerde); !errors.Is(err, ErrVerificacion) {
		t.Fatalf("VERDE sin ROJO debe fallar, llegó %v", err)
	}
	if err := q.Rojo("T-404", []string{testsDeLaTarea}, cmdRojoBueno); !errors.Is(err, ErrVerificacion) {
		t.Fatalf("ROJO de tarea inexistente debe fallar, llegó %v", err)
	}
	if err := q.Rojo("T-1", []string{testsDeLaTarea}, cmdRojoBueno); err != nil {
		t.Fatal(err)
	}
	if err := q.Verde("T-1", cmdFalla); !errors.Is(err, ErrVerificacion) {
		t.Fatalf("VERDE con comando fallido debe fallar, llegó %v", err)
	}
}

func TestEjecutarAcotaLaSalida(t *testing.T) {
	r, err := Ejecutar(t.TempDir(), "for i in $(seq 1 20000); do echo linea-$i; done")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Salida) > maxSalida {
		t.Fatalf("la salida debe acotarse a %d bytes (RG-1.8), tiene %d", maxSalida, len(r.Salida))
	}
	if !strings.Contains(r.Salida, "linea-20000") {
		t.Fatal("la salida acotada debe conservar el final, donde están los fallos")
	}
}

// tareaEnVerde lleva una tarea nueva de CONTRATO a VERDE bloqueando el test con su contenido actual.
func tareaEnVerde(t *testing.T, q Maquina, id string) {
	t.Helper()
	if err := q.Contrato(id, "golden de "+id, "contratos/sumar.md", true, ""); err != nil {
		t.Fatalf("contrato %s: %v", id, err)
	}
	if err := q.Rojo(id, []string{testsDeLaTarea}, cmdRojoBueno); err != nil {
		t.Fatalf("rojo %s: %v", id, err)
	}
	if err := q.Verde(id, cmdVerde); err != nil {
		t.Fatalf("verde %s: %v", id, err)
	}
}

// golden regenerado por otra tarea tras una aprobación registrada: el caso que motiva `hz tdd superar`.
func superacionArmada(t *testing.T) (Maquina, string, string) {
	t.Helper()
	q, raiz := proyectoTDD(t)
	tareaEnVerde(t, q, "T-1")
	if err := os.WriteFile(filepath.Join(raiz, testsDeLaTarea), []byte("assert sumar(1,2)==3  # golden regenerado\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tareaEnVerde(t, q, "T-2")
	d, err := q.M.NuevaDecision(motor.Decision{Que: "regenerar el golden tras la aprobación visual", PorQue: "el humano aprobó los sprites", Decidio: "humano"})
	if err != nil {
		t.Fatal(err)
	}
	return q, raiz, d
}

func TestSuperarDejaConstanciaSinTocarLosTests(t *testing.T) {
	q, raiz, d := superacionArmada(t)
	antes, _ := q.M.Tarea("T-1")
	contenido, _ := os.ReadFile(filepath.Join(raiz, testsDeLaTarea))
	if err := q.Superar("t-1", "t-2", d, "", "sesion-x"); err != nil {
		t.Fatalf("superar: %v", err)
	}
	ta, err := q.M.Tarea("T-1")
	if err != nil {
		t.Fatal(err)
	}
	s := ta.Superacion
	if ta.Estado != "superada" || s == nil || s.Por != "T-2" || s.Decision != d || s.Decidio != "humano" ||
		s.Motivo != "regenerar el golden tras la aprobación visual" || s.Quien != "sesion-x" || s.EstadoPrevio != "verde" || s.En == "" {
		t.Fatalf("la tarea debe quedar SUPERADA con quién, cuándo, por qué y qué tarea la supera: %+v %+v", ta, s)
	}
	if len(ta.TestsBloqueados) != 1 || ta.TestsBloqueados[0] != antes.TestsBloqueados[0] {
		t.Fatalf("superar no re-bloquea ni cambia los hashes: %+v", ta.TestsBloqueados)
	}
	if ahora, _ := os.ReadFile(filepath.Join(raiz, testsDeLaTarea)); string(ahora) != string(contenido) {
		t.Fatal("superar no toca los archivos de test")
	}
	log, _ := os.ReadFile(filepath.Join(raiz, ".harness", "log", "tdd.jsonl"))
	if !strings.Contains(string(log), `"evento":"tdd.superada"`) {
		t.Fatalf("superar agrega el evento tdd.superada en tdd.jsonl:\n%s", log)
	}
	b, err := os.ReadFile(filepath.Join(raiz, ".harness", "cache", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var h struct {
		Resumen       []string `json:"resumen"`
		TareasActivas []struct {
			ID string `json:"id"`
		} `json:"tareas_activas"`
		TestsBloqueados []string `json:"tests_bloqueados"`
		TareasSuperadas []struct {
			ID, Por, Decision string
		} `json:"tareas_superadas"`
	}
	if err := json.Unmarshal(b, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.TareasActivas) != 1 || h.TareasActivas[0].ID != "T-2" || len(h.TestsBloqueados) != 1 {
		t.Fatalf("la tarea superada deja de ser activa; el test sigue bloqueado solo por T-2: %s", b)
	}
	if len(h.TareasSuperadas) != 1 || h.TareasSuperadas[0].ID != "T-1" || h.TareasSuperadas[0].Por != "T-2" || h.TareasSuperadas[0].Decision != d {
		t.Fatalf("hooks.json lista la tarea superada con su tarea y su decisión: %s", b)
	}
	if !strings.Contains(strings.Join(h.Resumen, "\n"), "T-1 por T-2 ("+d+")") {
		t.Fatalf("el resumen del puntero menciona la superación: %q", h.Resumen)
	}
}

func TestSuperarRechazaConRemediacion(t *testing.T) {
	q, raiz, d := superacionArmada(t)
	if err := q.Contrato("T-3", "sin rojo", "contratos/sumar.md", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "tests", "otro_test.txt"), []byte("assert x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := q.Contrato("T-4", "en rojo", "contratos/sumar.md", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Rojo("T-4", []string{"tests/otro_test.txt"}, cmdRojoBueno); err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		nombre, id, por, decision string
		debe                      []string
	}{
		{"tarea inexistente", "T-404", "T-2", d, []string{"T-404", "no existe"}},
		{"saltarse el ROJO desde CONTRATO", "T-3", "T-2", d, []string{"T-3", "ROJO", "VERDE"}},
		{"saltarse el VERDE desde ROJO", "T-4", "T-2", d, []string{"T-4", "ROJO", "VERDE"}},
		{"la que supera no existe", "T-1", "T-405", d, []string{"T-405", "no existe"}},
		{"la que supera no llegó a VERDE", "T-1", "T-4", d, []string{"T-4", "VERDE", "hz tdd verde T-4"}},
		{"superarse a sí misma", "T-1", "T-1", d, []string{"misma"}},
		{"decisión inexistente", "T-1", "T-2", "D-0404", []string{"D-0404", "hz decision nueva"}},
	}
	for _, c := range casos {
		err := q.Superar(c.id, c.por, c.decision, "", "")
		if !errors.Is(err, ErrVerificacion) {
			t.Errorf("%s: se esperaba ErrVerificacion (código 1), llegó %v", c.nombre, err)
			continue
		}
		for _, s := range c.debe {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: el mensaje debe nombrar %q para remediar: %v", c.nombre, s, err)
			}
		}
	}
	for id, estado := range map[string]string{"T-1": "verde", "T-3": "contrato", "T-4": "rojo"} {
		if ta, _ := q.M.Tarea(id); ta.Estado != estado || ta.Superacion != nil {
			t.Fatalf("un rechazo no cambia el estado de %s: %+v", id, ta)
		}
	}
	if err := q.Superar("T-1", "T-2", d, "golden regenerado", ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Superar("T-1", "T-2", d, "", ""); !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), "ya fue superada") || !strings.Contains(err.Error(), "T-2") {
		t.Fatalf("una tarea ya superada se rechaza nombrando quién la superó: %v", err)
	}
	tareaEnVerde(t, q, "T-5")
	if err := q.Superar("T-5", "T-1", d, "", ""); !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), "--por T-2") {
		t.Fatalf("superar con una tarea ya superada sugiere --por la que la superó: %v", err)
	}
}

// sinSuperar comprueba que un rechazo de superar no cambió la tarea.
func sinSuperar(t *testing.T, q Maquina, id string) {
	t.Helper()
	if ta, _ := q.M.Tarea(id); ta.Estado == "superada" || ta.Superacion != nil {
		t.Fatalf("un rechazo no supera la tarea %s: %+v", id, ta)
	}
}

func decisionDe(t *testing.T, q Maquina, decidio string) string {
	t.Helper()
	d, err := q.M.NuevaDecision(motor.Decision{Que: "superar la tarea vieja", PorQue: "la reemplaza la nueva", Decidio: decidio})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Superar exige evidencia (D-0054): T-m bloquea, con su contenido actual, cada test de T-n que cambió.
func TestSuperarExigeQueLaTareaNuevaBloqueeLoQueCambio(t *testing.T) {
	q, raiz, d := superacionArmada(t)
	otro := "tests/otro_test.txt"
	if err := os.WriteFile(filepath.Join(raiz, otro), []byte("assert otro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := q.Contrato("T-3", "otra cosa", "contratos/sumar.md", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Rojo("T-3", []string{otro}, cmdRojoBueno); err != nil {
		t.Fatal(err)
	}
	if err := q.Verde("T-3", cmdVerde); err != nil {
		t.Fatal(err)
	}
	err := q.Superar("T-1", "T-3", d, "", "")
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), testsDeLaTarea) || !strings.Contains(err.Error(), "T-3") {
		t.Fatalf("una T-m que no bloquea el test que cambió no supera a T-1; el error nombra el test y T-m: %v", err)
	}
	sinSuperar(t, q, "T-1")
	if err := os.WriteFile(filepath.Join(raiz, testsDeLaTarea), []byte("assert sumar(1,2)==3  # editado después de T-2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = q.Superar("T-1", "T-2", d, "", "")
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), testsDeLaTarea) || !strings.Contains(err.Error(), "T-2") {
		t.Fatalf("si el test cambió después de que T-m lo bloqueara, T-m no lo respalda: %v", err)
	}
	sinSuperar(t, q, "T-1")
	if err := os.Remove(filepath.Join(raiz, testsDeLaTarea)); err != nil {
		t.Fatal(err)
	}
	err = q.Superar("T-1", "T-2", d, "", "")
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), testsDeLaTarea) || !strings.Contains(err.Error(), "no existe") {
		t.Fatalf("un test borrado no lo puede respaldar T-m: %v", err)
	}
	sinSuperar(t, q, "T-1")
}

func TestSuperarExigeAlgunTestCambiado(t *testing.T) {
	q, _ := proyectoTDD(t)
	tareaEnVerde(t, q, "T-1")
	tareaEnVerde(t, q, "T-2") // bloquea el mismo test, que no cambió
	err := q.Superar("T-1", "T-2", decisionDe(t, q, "humano"), "", "")
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), "no hay nada que superar") || !strings.Contains(err.Error(), "hz salida") {
		t.Fatalf("sin tests cambiados no hay nada que superar: la tarea se entrega con hz salida: %v", err)
	}
	sinSuperar(t, q, "T-1")
}

func TestSuperarComparaRutasLimpias(t *testing.T) {
	q, raiz := proyectoTDD(t)
	tareaEnVerde(t, q, "T-1")
	if err := os.WriteFile(filepath.Join(raiz, testsDeLaTarea), []byte("assert sumar(1,2)==3  # golden regenerado\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := q.Contrato("T-2", "regenera", "contratos/sumar.md", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := q.Rojo("T-2", []string{"./" + testsDeLaTarea}, cmdRojoBueno); err != nil {
		t.Fatal(err)
	}
	if err := q.Verde("T-2", cmdVerde); err != nil {
		t.Fatal(err)
	}
	if err := q.Superar("T-1", "T-2", decisionDe(t, q, "humano"), "", ""); err != nil {
		t.Fatalf("la misma ruta escrita de otra forma cuenta como bloqueada por T-m: %v", err)
	}
}

func TestSuperarExigeDecisionDelHumanoOAprobada(t *testing.T) {
	q, _, _ := superacionArmada(t)
	d := decisionDe(t, q, "agente")
	err := q.Superar("T-1", "T-2", d, "", "")
	if !errors.Is(err, ErrVerificacion) || !strings.Contains(err.Error(), d) || !strings.Contains(err.Error(), "humano") || !strings.Contains(err.Error(), "agente_aprobado") {
		t.Fatalf("una decisión del propio agente no basta para superar: %v", err)
	}
	sinSuperar(t, q, "T-1")
	if err := q.Superar("T-1", "T-2", decisionDe(t, q, "agente_aprobado"), "", ""); err != nil {
		t.Fatalf("una decisión agente_aprobado basta: %v", err)
	}
}
