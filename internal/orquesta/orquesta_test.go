package orquesta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hz528/internal/motor"
	"hz528/internal/tdd"
)

func ids(ts []motor.Tarea) string {
	var s []string
	for _, t := range ts {
		s = append(s, t.ID)
	}
	return strings.Join(s, ",")
}

func TestFrenteListoRespetaDependencias(t *testing.T) {
	tareas := []motor.Tarea{
		{ID: "T-1", Estado: "contrato"},
		{ID: "T-2", Estado: "contrato", BloqueadaPor: []string{"T-1"}},
		{ID: "T-3", Estado: "rojo"},
		{ID: "T-4", Estado: "contrato", BloqueadaPor: []string{"T-3"}},
	}
	if got := ids(FrenteListo(tareas)); got != "T-1" {
		t.Fatalf("frente listo = %q, se esperaba T-1", got)
	}
	tareas[0].Estado, tareas[2].Estado = "verde", "refactor"
	if got := ids(FrenteListo(tareas)); got != "T-2,T-4" {
		t.Fatalf("con dependencias en verde/refactor el frente debe ser T-2,T-4; llegó %q", got)
	}
	if got := ids(FrenteListo([]motor.Tarea{{ID: "T-5", Estado: "contrato", BloqueadaPor: []string{"T-404"}}})); got != "" {
		t.Fatalf("una dependencia inexistente bloquea: %q", got)
	}
}

func tareaEjemplo() motor.Tarea {
	return motor.Tarea{ID: "T-7", Titulo: "cobrar", Estado: "rojo", ContratoRuta: "contratos/cobro.md",
		TestsBloqueados: []motor.TestBloqueado{{Ruta: "tests/cobro_test.go", SHA256: "x"}}}
}

func TestPaqueteDelImplementadorNoTraeLaVerificacion(t *testing.T) {
	previa := &motor.Tarjeta{ID: "TR-0003", Rol: "tester", Texto: "hecho: tests en rojo por aserción"}
	arr := []motor.Arriendo{{Ruta: "app/cobro.go", Agente: "ag1", Tarea: "T-7"}}
	p, err := Paquete("implementador", tareaEjemplo(), previa, arr)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(strings.TrimSpace(p), "\n")); n > 40 {
		t.Fatalf("el paquete debe tener ≤40 líneas, tiene %d", n)
	}
	for _, debe := range []string{"contratos/cobro.md", "tests/cobro_test.go", "app/cobro.go", "hz tdd verde T-7", "TR-0003", "RG-1"} {
		if !strings.Contains(p, debe) {
			t.Errorf("el paquete del implementador debe incluir %q:\n%s", debe, p)
		}
	}
	for _, prohibido := range []string{"holdout", "rúbrica", "escalera", "V3", "red-team"} {
		if strings.Contains(strings.ToLower(p), strings.ToLower(prohibido)) {
			t.Errorf("el implementador NO recibe %q (es responsabilidad de otro rol):\n%s", prohibido, p)
		}
	}
}

func TestPaquetesPorRolYRolDesconocido(t *testing.T) {
	tester, err := Paquete("tester", tareaEjemplo(), nil, nil)
	if err != nil || !strings.Contains(tester, "hz tdd rojo T-7") {
		t.Fatalf("el tester usa hz tdd rojo: %v\n%s", err, tester)
	}
	verif, err := Paquete("verificador", tareaEjemplo(), nil, nil)
	if err != nil || !strings.Contains(verif, "hz check") {
		t.Fatalf("el verificador corre hz check: %v\n%s", err, verif)
	}
	if _, err := Paquete("astronauta", tareaEjemplo(), nil, nil); err == nil {
		t.Fatal("un rol desconocido debe fallar")
	}
}

// proyecto arma un proyecto con una tarea en VERDE y su test bloqueado.
func armarProyecto(t *testing.T) (*motor.Motor, string) {
	t.Helper()
	raiz := t.TempDir()
	for rel, txt := range map[string]string{
		".harness/harness.yaml": "version: 1\nenabled: true\nnivel: N2\nmodo: genesis\n",
		"AGENTS.md":             "# Mapa\n",
		"contratos/x.md":        "contrato\n",
		"tests/x_test.txt":      "assert\n",
	} {
		ruta := filepath.Join(raiz, rel)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Cerrar() })
	q := tdd.Maquina{M: m, Raiz: raiz}
	if err := q.Contrato("T-1", "x", "contratos/x.md", true, ""); err != nil {
		t.Fatal(err)
	}
	return m, raiz
}

func TestSalidaDeterministaPorRol(t *testing.T) {
	m, raiz := armarProyecto(t)
	q := tdd.Maquina{M: m, Raiz: raiz}
	if err := Salida(m, raiz, "contratista", "T-1"); err != nil {
		t.Fatalf("contratista con contrato registrado debe salir: %v", err)
	}
	if err := Salida(m, raiz, "tester", "T-1"); err == nil {
		t.Fatal("el tester no sale con la tarea aún en CONTRATO")
	}
	if err := q.Rojo("T-1", []string{"tests/x_test.txt"}, "echo 'AssertionError'; exit 1"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "tester", "T-1"); err != nil {
		t.Fatalf("tester con ROJO registrado debe salir: %v", err)
	}
	if err := q.Verde("T-1", "exit 0"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "implementador", "T-1"); err != nil {
		t.Fatalf("implementador con VERDE y tests intactos debe salir: %v", err)
	}
	if err := m.RegistrarMedicion("mutacion:T-1:app/x.go", 1, "20 de 20"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-1"); err != nil {
		t.Fatalf("verificador con evidencia, check en verde y meta-eval de los tests debe salir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "tests", "x_test.txt"), []byte("assert True\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "implementador", "T-1"); err == nil || !strings.Contains(err.Error(), "x_test.txt") {
		t.Fatalf("un test bloqueado modificado impide la salida del implementador: %v", err)
	}
	if err := Salida(m, raiz, "evaluador", "T-1"); err == nil {
		t.Fatal("el evaluador no sale sin su tarjeta de veredicto")
	}
	if _, err := m.NuevaTarjeta(motor.Tarjeta{TareaID: "T-1", Rol: "evaluador", Estado: "listo", Texto: "veredicto: aprobado"}); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "evaluador", "T-1"); err != nil {
		t.Fatalf("con su tarjeta, el evaluador sale: %v", err)
	}
}

func TestPaqueteSinErroresDeFormato(t *testing.T) {
	for _, rol := range Roles() {
		p, err := Paquete(rol, tareaEjemplo(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(p, "%!") {
			t.Errorf("el paquete de %s tiene un error de formato:\n%s", rol, p)
		}
	}
}

func TestSalidaDelVerificadorExigeMutacion(t *testing.T) {
	m, raiz := armarProyecto(t)
	q := tdd.Maquina{M: m, Raiz: raiz}
	if err := q.Rojo("T-1", []string{"tests/x_test.txt"}, "echo 'AssertionError'; exit 1"); err != nil {
		t.Fatal(err)
	}
	if err := q.Verde("T-1", "exit 0"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-1"); err == nil || !strings.Contains(err.Error(), "hz mutar") {
		t.Fatalf("sin meta-eval de los tests el verificador no entrega: %v", err)
	}
	if err := m.RegistrarMedicion("mutacion:T-1:app/x.go", 0.75, "15 de 20"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-1"); err == nil || !strings.Contains(err.Error(), "app/x.go") || !strings.Contains(err.Error(), "75%") {
		t.Fatalf("una suite que detecta 75%% de las fallas sembradas no alcanza el 90%%: %v", err)
	}
	if err := m.RegistrarMedicion("mutacion:T-1:app/x.go", 0.95, "19 de 20"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-1"); err != nil {
		t.Fatalf("con la suite reforzada el verificador entrega: %v", err)
	}
	if err := m.TomarArriendo([]string{"app/y.go", "tests/x_test.txt", "README.md"}, "ag1", "T-1", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-1"); err == nil || !strings.Contains(err.Error(), "app/y.go") || strings.Contains(err.Error(), "README.md") {
		t.Fatalf("todo archivo de código arrendado por la tarea necesita su medición (tests y docs no): %v", err)
	}
	if err := m.RegistrarMedicion("mutacion:T-1:app/y.go", 1, "20 de 20"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-1"); err != nil {
		t.Fatalf("con todos los archivos medidos, entrega: %v", err)
	}
	m2, raiz2 := armarProyecto(t)
	if err := os.WriteFile(filepath.Join(raiz2, ".harness", "harness.yaml"), []byte("version: 1\nenabled: true\nnivel: N2\nmodo: genesis\nverificacion: {mutacion_min: 0}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	q2 := tdd.Maquina{M: m2, Raiz: raiz2}
	if err := q2.Rojo("T-1", []string{"tests/x_test.txt"}, "echo 'AssertionError'; exit 1"); err != nil {
		t.Fatal(err)
	}
	if err := q2.Verde("T-1", "exit 0"); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m2, raiz2, "verificador", "T-1"); err != nil {
		t.Fatalf("con mutacion_min 0 la exigencia queda desactivada: %v", err)
	}
}

func TestEstadosPosterioresAlVerdeNoBloquean(t *testing.T) {
	for _, estado := range []string{"verde", "refactor", "verificada", "cerrada"} {
		m, _ := armarProyecto(t)
		if err := m.RegistrarTarea("tdd.verde", motor.Tarea{ID: "T-1", Titulo: "x", Estado: estado}); err != nil {
			t.Fatal(err)
		}
		if err := m.RegistrarTarea("tdd.contrato", motor.Tarea{ID: "T-2", Titulo: "y", Estado: "contrato", BloqueadaPor: []string{"T-1"}}); err != nil {
			t.Fatal(err)
		}
		tareas, err := m.Tareas()
		if err != nil {
			t.Fatal(err)
		}
		if listas := FrenteListo(tareas); !strings.Contains(ids(listas), "T-2") {
			t.Fatalf("una dependencia en %s ya no bloquea: frente = %s", estado, ids(listas))
		}
	}
}

func TestSalidaSinTestsBloqueadosNiEvidencia(t *testing.T) {
	m, raiz := armarProyecto(t)
	if err := m.RegistrarTarea("tdd.rojo", motor.Tarea{ID: "T-2", Titulo: "x", Estado: "rojo"}); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "tester", "T-2"); err == nil || !strings.Contains(err.Error(), "tests bloqueados") {
		t.Fatalf("en ROJO pero sin tests bloqueados el tester no sale: %v", err)
	}
	if err := m.RegistrarTarea("tdd.verde", motor.Tarea{ID: "T-3", Titulo: "y", Estado: "verde",
		Evidencia: []motor.Evidencia{{Cmd: "go test ./...", Resultado: "exit 1"}}}); err != nil {
		t.Fatal(err)
	}
	if err := Salida(m, raiz, "verificador", "T-3"); err == nil || !strings.Contains(err.Error(), "evidencia") {
		t.Fatalf("sin evidencia ejecutada en verde el verificador no sale: %v", err)
	}
}
