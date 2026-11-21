package exportar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hz528/internal/motor"
)

func escribir(t *testing.T, raiz, rel, txt string) {
	t.Helper()
	ruta := filepath.Join(raiz, rel)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
		t.Fatal(err)
	}
}

func leerHooks(t *testing.T, raiz string) Hooks {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(raiz, ".harness", "cache", "hooks.json"))
	if err != nil {
		t.Fatalf("no se escribió hooks.json: %v", err)
	}
	var h Hooks
	if err := json.Unmarshal(b, &h); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestExportarV2Completo(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, raiz, ".harness/harness.yaml", "version: 1\nenabled: true\nnivel: N2\nmodo: mejora\n")
	escribir(t, raiz, ".harness/reglas.yaml", `version: 1
reglas:
  - {id: RD-1, clave_catalogo: secretos, orden: "No leas secretos.", por_que: "x", origen: genesis, desde: 2026-11-20,
     enforcement: [permisos, hook], patrones: {rutas: [".env", "config/*.env"]}, remediacion: "Usa las variables cargadas.", estado: vigente}
  - {id: RD-2, orden: "No escribas en tickets.", por_que: "y", origen: mejora, desde: 2026-11-20,
     enforcement: [hook, ci], patrones: {herramientas: ["mcp__x__escribir"], bash: ["curl .*produccion"]}, estado: vigente}
  - {id: RD-3, orden: "Vieja.", por_que: "z", origen: genesis, desde: 2026-01-01, enforcement: [hook, ci], estado: retirada}
`)
	escribir(t, raiz, ".env", "TOKEN=valor-secreto-largo\n")
	escribir(t, raiz, "config/prod.env", "CLAVE=otra-clave-larga\n")
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	if err := m.RegistrarTarea("tdd.rojo", motor.Tarea{ID: "T-1", Titulo: "x", Estado: "rojo",
		TestsBloqueados: []motor.TestBloqueado{{Ruta: "tests/a_test.go", SHA256: "h"}}}); err != nil {
		t.Fatal(err)
	}
	if err := Escribir(m, raiz); err != nil {
		t.Fatal(err)
	}
	h := leerHooks(t, raiz)
	if h.Version != 2 || h.Nivel != "N2" || h.Modo != "mejora" {
		t.Fatalf("cabecera inesperada: %+v", h)
	}
	if len(h.ReglasDuras) != 2 || h.ReglasDuras[0].Remediacion == "" || h.ReglasDuras[1].Herramientas[0] != "mcp__x__escribir" {
		t.Fatalf("solo deben exportarse RD vigentes con su remediación: %+v", h.ReglasDuras)
	}
	if strings.Join(h.Secretos, ",") != ".env,config/prod.env" {
		t.Fatalf("secretos debe listar los archivos reales que coinciden: %v", h.Secretos)
	}
	if len(h.TestsBloqueados) != 1 || h.TestsBloqueados[0] != "tests/a_test.go" {
		t.Fatalf("tests bloqueados: %v", h.TestsBloqueados)
	}
	protegidos := strings.Join(h.Protegidos, " ")
	if !strings.Contains(protegidos, ".harness/reglas.yaml") || !strings.Contains(protegidos, ".claude/settings.json") || strings.Contains(protegidos, ".claude/**") {
		t.Fatalf("protegidos inesperados: %v", h.Protegidos)
	}
	if len(h.Resumen) == 0 || len(h.Resumen) > 15 || !strings.Contains(strings.Join(h.Resumen, "\n"), "T-1") {
		t.Fatalf("el resumen debe tener 1–15 líneas y mencionar las tareas activas: %v", h.Resumen)
	}
}

func TestExportarIncluyeSoloArriendosVigentes(t *testing.T) {
	raiz := t.TempDir()
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	if err := m.TomarArriendo([]string{"app/a.go"}, "ag1", "T-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := m.TomarArriendo([]string{"app/viejo.go"}, "ag2", "T-2", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Escribir(m, raiz); err != nil {
		t.Fatal(err)
	}
	h := leerHooks(t, raiz)
	if len(h.Arriendos) != 1 || h.Arriendos[0].Ruta != "app/a.go" || h.Arriendos[0].Agente != "ag1" || h.Arriendos[0].Tarea != "T-1" {
		t.Fatalf("solo el arriendo vigente debe exportarse: %+v", h.Arriendos)
	}
}

func TestExportarIncluyeSoloLeccionesActivasConSuAmbito(t *testing.T) {
	raiz := t.TempDir()
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	if _, _, err := m.NuevoError(motor.Error{Que: "x", Fuente: "humano", Severidad: "menor", Causa: "otra", Clase: "c1", Ambito: "app/**"}); err != nil {
		t.Fatal(err)
	}
	for _, skill := range []string{"hz-leccion-a", "hz-leccion-b"} {
		if _, err := m.NuevaLeccion(motor.Leccion{Clase: "c1", Skill: skill, Descripcion: "d"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.FijarEstadoLeccion("L-0001", "activa", "pasó"); err != nil {
		t.Fatal(err)
	}
	if err := Escribir(m, raiz); err != nil {
		t.Fatal(err)
	}
	h := leerHooks(t, raiz)
	if len(h.Lecciones) != 1 || h.Lecciones[0].Skill != "hz-leccion-a" || h.Lecciones[0].Ambito != "app/**" {
		t.Fatalf("solo la lección activa, con su ámbito: %+v", h.Lecciones)
	}
}

func TestExportarSinPerfilEsN1SinReglas(t *testing.T) {
	raiz := t.TempDir()
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	if err := Escribir(m, raiz); err != nil {
		t.Fatal(err)
	}
	h := leerHooks(t, raiz)
	if h.Nivel != "N1" || len(h.ReglasDuras) != 0 || len(h.Secretos) != 0 {
		t.Fatalf("sin perfil se exporta N1 sin enforcement: %+v", h)
	}
}

func TestExportarMisionesActivasConPresupuesto(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, raiz, ".harness/harness.yaml", "version: 1\nenabled: true\nnivel: N3\nmodo: mejora\n")
	escribir(t, raiz, ".harness/misiones/m1/mision.yaml", "id: m1\nmodo: mejora\npresupuesto: {usd_max: 1}\n")
	escribir(t, raiz, ".harness/misiones/m2/mision.yaml", "id: m2\nmodo: mejora\npresupuesto: {usd_max: 5}\n")
	escribir(t, raiz, ".harness/misiones/m3/mision.yaml", "id: m3\nmodo: mejora\npresupuesto: {usd_max: 1}\n")
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	for _, id := range []string{"m1", "m2", "m3"} {
		if err := m.NuevaMision(motor.Mision{ID: id, Modo: "mejora"}); err != nil {
			t.Fatal(err)
		}
	}
	for id, usd := range map[string]float64{"m1": 1.5, "m2": 1, "m3": 3} {
		if err := m.RegistrarGasto(id, usd, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.CerrarMision("m3"); err != nil {
		t.Fatal(err)
	}
	if err := Escribir(m, raiz); err != nil {
		t.Fatal(err)
	}
	h := leerHooks(t, raiz)
	if len(h.Misiones) != 2 {
		t.Fatalf("solo las misiones activas se exportan (m3 está cerrada): %+v", h.Misiones)
	}
	por := map[string]MisionExportada{}
	for _, x := range h.Misiones {
		por[x.ID] = x
	}
	if x := por["m1"]; !x.SinPresupuesto || x.Gasto != 1.5 || x.UsdMax != 1 {
		t.Fatalf("m1 gastó 1.5 de 1: debe quedar sin presupuesto: %+v", x)
	}
	if x := por["m2"]; x.SinPresupuesto || x.UsdMax != 5 {
		t.Fatalf("m2 gastó 1 de 5: tiene presupuesto: %+v", x)
	}
}
