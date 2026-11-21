package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hz(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestSinProyectoDevuelve3(t *testing.T) {
	if code, _, _ := hz(t, "--raiz", t.TempDir(), "estado"); code != 3 {
		t.Fatalf("sin proyecto se esperaba código 3, llegó %d", code)
	}
}

func TestUsoIncorrectoDevuelve2(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar"); code != 0 {
		t.Fatalf("activar: %d %s", code, e)
	}
	if code, _, e := hz(t, "--raiz", raiz, "decision", "nueva", "--que", "solo qué"); code != 2 || !strings.Contains(e, "por_que") {
		t.Fatalf("faltan campos: se esperaba código 2 nombrando por_que, llegó %d %q", code, e)
	}
	if code, _, _ := hz(t, "--raiz", raiz, "comando-inexistente"); code != 2 {
		t.Fatalf("comando desconocido debe devolver 2, llegó %d", code)
	}
}

func TestPunteroParaSessionStart(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar"); code != 0 {
		t.Fatalf("activar: %d %s", code, e)
	}
	code, out, e := hz(t, "--raiz", raiz, "puntero")
	if code != 0 {
		t.Fatalf("puntero: %d %s", code, e)
	}
	var salida struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &salida); err != nil {
		t.Fatalf("puntero debe imprimir JSON de hook SessionStart: %v\n%s", err, out)
	}
	if salida.HookSpecificOutput.HookEventName != "SessionStart" || !strings.Contains(salida.HookSpecificOutput.AdditionalContext, "Harness 528hz activo") {
		t.Fatalf("salida inesperada: %+v", salida)
	}
}

func TestPunteroSinHarnessNoFallaNiImprime(t *testing.T) {
	code, out, _ := hz(t, "--raiz", t.TempDir(), "puntero")
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Fatalf("sin harness, puntero nunca debe romper un inicio de sesión: código %d, salida %q", code, out)
	}
}

func TestOrquestacionDesdeElCLI(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N2"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "contratos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "contratos", "x.md"), []byte("contrato\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pasos := []struct {
		args   []string
		codigo int
		debe   string
	}{
		{[]string{"tdd", "contrato", "T-1", "--titulo", "base", "--contrato", "contratos/x.md", "--celula", "pagos"}, 0, "T-1"},
		{[]string{"tdd", "contrato", "T-2", "--titulo", "depende", "--contrato", "contratos/x.md", "--bloqueada-por", "T-1"}, 0, "T-2"},
		{[]string{"tablero", "--listo"}, 0, "T-1"},
		{[]string{"arriendo", "tomar", "app/a.go", "--agente", "ag1", "--tarea", "T-1"}, 0, "app/a.go"},
		{[]string{"arriendo", "tomar", "app/a.go", "--agente", "ag2", "--tarea", "T-2"}, 1, ""},
		{[]string{"arriendo", "lista"}, 0, "ag1"},
		{[]string{"tarjeta", "registrar", "T-1", "--rol", "contratista", "--estado", "listo", "--texto", "hecho: contrato listo"}, 0, "TR-0001"},
		{[]string{"tarjeta", "ultima", "T-1"}, 0, "hecho: contrato listo"},
		{[]string{"contexto", "--rol", "tester", "--tarea", "T-1"}, 0, "hz tdd rojo T-1"},
		{[]string{"contexto", "--rol", "astronauta", "--tarea", "T-1"}, 2, ""},
		{[]string{"salida", "--rol", "contratista", "--tarea", "T-1"}, 0, ""},
		{[]string{"salida", "--rol", "tester", "--tarea", "T-1"}, 1, ""},
		{[]string{"capacidad", "--json"}, 0, "celulas"},
	}
	for _, p := range pasos {
		code, out, e := hz(t, append([]string{"--raiz", raiz}, p.args...)...)
		if code != p.codigo || !strings.Contains(out, p.debe) {
			t.Fatalf("hz %v: código %d (se esperaba %d), salida %q, error %q", p.args, code, p.codigo, out, e)
		}
	}
	if code, out, _ := hz(t, "--raiz", raiz, "tablero", "--listo"); code != 0 || strings.Contains(out, "T-2") {
		t.Fatalf("T-2 está bloqueada por T-1 y no puede estar en el frente listo: %q", out)
	}
}

func TestAprendizajeDesdeElCLI(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N2"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	archivos := map[string]string{
		".harness/reglas.yaml":                      "version: 1\nreglas:\n  - {id: RD-1, clave_catalogo: secretos, orden: \"No leas secretos.\", por_que: \"x\", origen: genesis, desde: 2026-11-20, enforcement: [permisos, hook], patrones: {rutas: [\".env\"]}, estado: vigente}\n",
		".claude/skills/hz-leccion-escape/SKILL.md": "---\nname: hz-leccion-escape\ndescription: x\n---\n",
	}
	for rel, txt := range archivos {
		ruta := filepath.Join(raiz, rel)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pasos := []struct {
		args   []string
		codigo int
		debe   string
	}{
		{[]string{"error", "nuevo", "--que", "colapsó la barra invertida", "--fuente", "verificacion", "--severidad", "mayor", "--causa", "herramienta_mal_usada", "--clase", "escape", "--ambito", "**/*.ts"}, 0, "E-0001"},
		{[]string{"leccion", "nueva", "--clase", "escape", "--skill", "hz-leccion-escape", "--descripcion", "usa Edit para regex"}, 0, "L-0001"},
		{[]string{"eval", "registrar", "L-0001", "--tipo", "prevencion", "--resultado", "pasa"}, 0, ""},
		{[]string{"eval", "registrar", "L-0001", "--tipo", "interferencia", "--resultado", "pasa"}, 0, ""},
		{[]string{"eval", "registrar", "L-0001", "--tipo", "activacion", "--resultado", "pasa"}, 0, ""},
		{[]string{"eval", "decidir", "L-0001", "--aplicar"}, 0, "activar"},
		{[]string{"leccion", "para", "src/app.ts"}, 0, "L-0001"},
		{[]string{"leccion", "para", "src/app.go"}, 0, "sin lecciones"},
		{[]string{"redteam", "plan", "RD-1"}, 0, "rt-1-1"},
		{[]string{"redteam", "registrar", "RD-1", "--caso", "rt-1-7", "--resultado", "paso", "--detalle", "contenido invertido"}, 0, "inefectiva"},
		{[]string{"redteam", "estado"}, 0, "inefectiva"},
		{[]string{"error", "lista"}, 0, "E-0002"},
	}
	for _, p := range pasos {
		code, out, e := hz(t, append([]string{"--raiz", raiz}, p.args...)...)
		if code != p.codigo || !strings.Contains(out, p.debe) {
			t.Fatalf("hz %v: código %d (se esperaba %d), salida %q, error %q", p.args, code, p.codigo, out, e)
		}
	}
	b, err := os.ReadFile(filepath.Join(raiz, ".harness", "cache", "hooks.json"))
	if err != nil || !strings.Contains(string(b), "hz-leccion-escape") {
		t.Fatalf("la lección activa debe exportarse a hooks.json para la inyección justo a tiempo: %v", err)
	}
}

// sensorArchivo escribe la salida determinista de un sensor: 20 ítems, los primeros `verdaderos` en true.
func sensorArchivo(t *testing.T, raiz, ronda, metrica string, verdaderos int) {
	t.Helper()
	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf(`{"item_id":"t%02d","resultado":%t}`, i, i < verdaderos)
	}
	ruta := filepath.Join(raiz, "sensores", ronda+"-"+metrica+".json")
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte("["+strings.Join(items, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

const misionMejora = `id: m1
titulo: Resolución autónoma
modo: mejora
objetivo: subir la resolución sin aumentar los fallos
presupuesto: {usd_max: 1}
estadistica: {k_propuesta_milestone: 2}
aceptacion:
  - {id: A1, texto: "tasa de resolución ≥ 0.7 con evidencia", cumplido: false}
verticales:
  - id: V-resolucion
    nombre: Resolución autónoma
    peso: 1
    metricas:
      - {id: tasa, familia: efectividad, sensor: "cat sensores/{ronda}-tasa.json", direccion: sube, meta: 0.7, piso_ruido: 0.02}
      - {id: rapidez, familia: impacto, sensor: "cat sensores/{ronda}-rapidez.json", direccion: sube, meta: 0.5}
      - {id: errores, familia: fallos, sensor: "cat sensores/{ronda}-errores.json", direccion: baja, meta: 0.1}
`

func TestMisionDeMejoraDeExtremoAExtremo(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N3", "--modo", "mejora"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	if code, out, e := hz(t, "--raiz", raiz, "mision", "nueva", "m1", "--modo", "mejora", "--objetivo", "x", "--usd-max", "1"); code != 0 || !strings.Contains(out, "m1") {
		t.Fatalf("mision nueva: %d %q %q", code, out, e)
	}
	if err := os.WriteFile(filepath.Join(raiz, ".harness", "misiones", "m1", "mision.yaml"), []byte(misionMejora), 0o644); err != nil {
		t.Fatal(err)
	}
	// errores 8 → 2: 6 pares discordantes, p exacto ≈ 0,031 (con 4 pares el mínimo es 0,125 y sería ruido)
	for ronda, v := range map[string][3]int{"R-1": {4, 5, 8}, "R-2": {15, 12, 2}} {
		sensorArchivo(t, raiz, ronda, "tasa", v[0])
		sensorArchivo(t, raiz, ronda, "rapidez", v[1])
		sensorArchivo(t, raiz, ronda, "errores", v[2])
	}
	informe := filepath.Join(raiz, "informe.html")
	pasos := []struct {
		args   []string
		codigo int
		debe   string
	}{
		{[]string{"ronda", "nueva", "m1", "--foco", "V-resolucion"}, 0, "R-1"},
		{[]string{"ronda", "medir", "m1", "R-1"}, 0, "tasa"},
		{[]string{"ronda", "nueva", "m1"}, 0, "R-2"},
		{[]string{"ronda", "medir", "m1", "R-2"}, 0, "tasa"},
		{[]string{"compare", "m1", "R-1", "R-2", "--metrica", "tasa"}, 0, "mejora"},
		{[]string{"compare", "m1", "R-1", "R-2", "--metrica", "errores"}, 0, "mejora"},
		{[]string{"ronda", "cerrar", "m1", "R-2", "--veredicto", "mantener"}, 0, "¿Marcamos R-2 como milestone?"},
		{[]string{"milestone", "proponer", "m1", "R-2"}, 0, "Propuesta"},
		{[]string{"milestone", "marcar", "m1", "R-2", "--resumen", "la tasa subió a 75%", "--propuesta"}, 0, "MS-1"},
		{[]string{"informe", "m1", "--json"}, 0, "MS-1"},
		{[]string{"informe", "m1", "--html", informe}, 0, "informe.html"},
		{[]string{"mision", "gasto", "m1", "--usd", "1.5", "--concepto", "rondas"}, 0, "sin presupuesto"},
		{[]string{"mision", "cerrar", "m1"}, 1, ""},
		{[]string{"mision", "aceptar", "m1", "A1", "--evidencia", "hz ronda medir m1 R-2 → tasa 0.75"}, 0, "A1"},
		{[]string{"mision", "cerrar", "m1"}, 0, "cerrada"},
	}
	for _, p := range pasos {
		if p.args[0] == "mision" && p.args[1] == "cerrar" && p.codigo == 1 {
			b, _ := os.ReadFile(filepath.Join(raiz, ".harness", "cache", "hooks.json"))
			if !strings.Contains(string(b), `"sin_presupuesto": true`) {
				t.Fatalf("una misión sin presupuesto debe exportarse para que el plugin frene los lanzamientos:\n%s", b)
			}
		}
		code, out, e := hz(t, append([]string{"--raiz", raiz}, p.args...)...)
		if code != p.codigo || !strings.Contains(out, p.debe) {
			t.Fatalf("hz %v: código %d (se esperaba %d), salida %q, error %q", p.args, code, p.codigo, out, e)
		}
	}
	html, err := os.ReadFile(informe)
	if err != nil || !strings.Contains(string(html), "<svg") || !strings.Contains(string(html), "MS-1") {
		t.Fatalf("el informe HTML debe traer gráficos SVG y los milestones: %v", err)
	}
}

func TestActivarN2YExportar(t *testing.T) {
	raiz := t.TempDir()
	if code, out, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N2"); code != 0 || !strings.Contains(out, ".claude/settings.json") || !strings.Contains(out, ".harness/reglas.yaml") {
		t.Fatalf("activar N2: %d %q %q", code, out, e)
	}
	if code, out, e := hz(t, "--raiz", raiz, "exportar"); code != 0 || !strings.Contains(out, "hooks.json") {
		t.Fatalf("exportar: %d %q %q", code, out, e)
	}
}

func TestFlujoActivarDecidirBuscarVerEstadoCheck(t *testing.T) {
	raiz := t.TempDir()
	if code, out, e := hz(t, "--raiz", raiz, "activar", "--modo", "genesis"); code != 0 || !strings.Contains(out, "creado") {
		t.Fatalf("activar: %d %q %q", code, out, e)
	}
	code, out, e := hz(t, "--raiz", raiz, "decision", "nueva",
		"--que", "Usar Go para el CLI", "--por-que", "preferencia del usuario y binario único",
		"--decidio", "humano", "--alternativa", "Bun + TS|el usuario prefiere Go", "--etiqueta", "cli")
	if code != 0 || strings.TrimSpace(out) != "D-0001" {
		t.Fatalf("decision nueva: %d %q %q", code, out, e)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "decision", "buscar", "cli", "go"); code != 0 || !strings.Contains(out, "D-0001") {
		t.Fatalf("buscar: %d %q", code, out)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "decision", "ver", "D-0001"); code != 0 || !strings.Contains(out, "Bun + TS") {
		t.Fatalf("ver: %d %q", code, out)
	}
	if code, _, _ := hz(t, "--raiz", raiz, "decision", "ver", "D-0404"); code != 1 {
		t.Fatalf("ver inexistente debe devolver 1, llegó %d", code)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "estado"); code != 0 || !strings.Contains(out, "D-0001") || len(strings.Split(strings.TrimSpace(out), "\n")) > 40 {
		t.Fatalf("estado: %d %q", code, out)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "check"); code != 0 {
		t.Fatalf("el esqueleto creado por activar debe pasar check: %d\n%s", code, out)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "regla", "ver", "RG-1"); code != 0 || !strings.Contains(out, "RG-1.3") {
		t.Fatalf("regla ver: %d %q", code, out)
	}
	if code, out, _ := hz(t, "version"); code != 0 || !strings.HasPrefix(out, "hz ") {
		t.Fatalf("version: %d %q", code, out)
	}
}
