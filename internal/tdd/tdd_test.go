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
