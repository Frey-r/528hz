package orquesta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if err := Salida(m, raiz, "verificador", "T-1"); err != nil {
		t.Fatalf("verificador con evidencia y check en verde debe salir: %v", err)
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
