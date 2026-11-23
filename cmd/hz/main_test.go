package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hz528/internal/mutar"
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

func TestPanelJuntaActividadYMotor(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N2"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "panel"); code != 0 || !strings.Contains(out, "sin actividad registrada") {
		t.Fatalf("sin snapshot del plugin: código %d, salida %q", code, out)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "contratos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "contratos", "x.md"), []byte("contrato\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, e := hz(t, "--raiz", raiz, "tdd", "contrato", "T-1", "--titulo", "parser", "--contrato", "contratos/x.md"); code != 0 {
		t.Fatalf("contrato: %s", e)
	}
	ahora := time.Now().UnixMilli()
	actividad := fmt.Sprintf(`{"version":1,"generado":"2026-11-21T10:00:00Z","agentes":[
 {"id":"principal","rol":"director","descripcion":"","padre":"","tarea":"","estado":"activo","tools":12,"errores":0,"seguidos":0,"ultima":"Edit a.go","desde":%[1]d,"visto":%[1]d},
 {"id":"L1","rol":"lider","descripcion":"lider pagos","padre":"principal","tarea":"","estado":"activo","tools":4,"errores":0,"seguidos":0,"ultima":"Agent","desde":%[1]d,"visto":%[1]d},
 {"id":"r1","rol":"tester","descripcion":"tester T-1","padre":"L1","tarea":"T-1","estado":"terminado","tools":9,"errores":1,"seguidos":0,"ultima":"Bash go test","desde":%[1]d,"visto":%[1]d},
 {"id":"r2","rol":"implementador","descripcion":"implementador T-1","padre":"L1","tarea":"T-1","estado":"activo","tools":3,"errores":3,"seguidos":3,"ultima":"Bash go test","desde":%[1]d,"visto":%[1]d}]}`, ahora)
	if err := os.WriteFile(filepath.Join(raiz, ".harness", "cache", "actividad.json"), []byte(actividad), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, e := hz(t, "--raiz", raiz, "panel")
	if code != 0 {
		t.Fatalf("panel: %d %s", code, e)
	}
	lineas := strings.Split(strings.TrimSpace(out), "\n")
	indice := func(s string) int {
		for i, l := range lineas {
			if strings.Contains(l, s) {
				return i
			}
		}
		return -1
	}
	if indice("T-1 CONTRATO") < 0 || indice("director") < 0 || indice("3 seguidos") < 0 || len(lineas) > 40 {
		t.Fatalf("el panel muestra tareas, el árbol y los errores seguidos en ≤40 líneas:\n%s", out)
	}
	if !(indice("director") < indice("lider") && indice("lider") < indice("implementador") && indice("implementador") < indice("tester")) {
		t.Fatalf("árbol por supervisor con los vivos primero (implementador vivo antes que tester terminado):\n%s", out)
	}
	if !strings.HasPrefix(lineas[indice("implementador")], "    ") {
		t.Fatalf("un rol de un líder va dos niveles adentro:\n%s", out)
	}
	code, out, _ = hz(t, "--raiz", raiz, "panel", "--json")
	var p struct {
		Agentes  []map[string]any `json:"agentes"`
		Tareas   []map[string]any `json:"tareas"`
		Misiones []map[string]any `json:"misiones"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &p) != nil || len(p.Agentes) != 4 || len(p.Tareas) != 1 || p.Misiones == nil {
		t.Fatalf("panel --json es el contrato del dashboard: %d %s", code, out)
	}
}

func TestDashboardValidaPuertoYPuertoOcupado(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N3"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	for _, args := range [][]string{{"--puerto", "0"}, {"--puerto", "70000"}, {"--puerto", "abc"}, {"extra"}} {
		if code, _, _ := hz(t, append([]string{"--raiz", raiz, "dashboard"}, args...)...); code != 2 {
			t.Errorf("hz dashboard %v debe ser uso inválido, llegó %d", args, code)
		}
	}
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupado.Close()
	puerto := strconv.Itoa(ocupado.Addr().(*net.TCPAddr).Port)
	if code, _, e := hz(t, "--raiz", raiz, "dashboard", "--puerto", puerto); code != 4 || !strings.Contains(e, "no se pudo escuchar") {
		t.Fatalf("con el puerto ocupado, hz dashboard falla sin quedarse colgado: %d %q", code, e)
	}
	if code, _, e := hz(t, "--raiz", raiz, "mision", "nueva", "m1", "--objetivo", "x", "--usd-max", "5"); code != 0 {
		t.Fatalf("mision: %s", e)
	}
	if code, _, e := hz(t, "--raiz", raiz, "mision", "gasto", "m1", "--usd", "6"); code != 0 {
		t.Fatalf("gasto: %s", e)
	}
	if code, out, _ := hz(t, "--raiz", raiz, "panel"); code != 0 || !strings.Contains(out, "de US$5.00 (120%)") || !strings.Contains(out, "sin presupuesto") {
		t.Fatalf("el panel muestra la misión con su tope y si se quedó sin presupuesto:\n%s", out)
	}
}

func TestDashboardSirveYSeApagaOrdenadamente(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N2"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	libre, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	puerto := strconv.Itoa(libre.Addr().(*net.TCPAddr).Port)
	libre.Close()
	estado := make(chan int, 1)
	alEscuchar = func(dir string, apagar func()) {
		defer apagar()
		r, err := http.Get("http://" + dir + "/api/panel")
		if err != nil {
			estado <- -1
			return
		}
		r.Body.Close()
		estado <- r.StatusCode
	}
	defer func() { alEscuchar = nil }()
	code, out, e := hz(t, "--raiz", raiz, "dashboard", "--puerto", puerto)
	if code != 0 || !strings.Contains(out, "detenido") {
		t.Fatalf("al apagarse, hz dashboard sale con 0: %d %q %q", code, out, e)
	}
	if s := <-estado; s != http.StatusOK {
		t.Fatalf("mientras corría, /api/panel respondió %d", s)
	}
}

func TestSuperarDesdeElCLI(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar", "--nivel", "N2"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	for rel, txt := range map[string]string{"contratos/x.md": "contrato\n", "golden/sprites.json": "{\"v\":1}\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(raiz, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(raiz, rel), []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ciclo := func(id string) [][]string {
		return [][]string{
			{"tdd", "contrato", id, "--titulo", "golden " + id, "--contrato", "contratos/x.md"},
			{"tdd", "rojo", id, "--tests", "golden/sprites.json", "--cmd", "echo 'AssertionError'; exit 1"},
			{"tdd", "verde", id, "--cmd", "exit 0"},
		}
	}
	for _, args := range ciclo("T-7") {
		if code, _, e := hz(t, append([]string{"--raiz", raiz}, args...)...); code != 0 {
			t.Fatalf("hz %v: %s", args, e)
		}
	}
	if err := os.WriteFile(filepath.Join(raiz, "golden", "sprites.json"), []byte("{\"v\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range append(ciclo("T-11"), []string{"decision", "nueva", "--que", "regenerar el golden de sprites", "--por-que", "aprobación visual del humano", "--decidio", "humano"}) {
		if code, _, e := hz(t, append([]string{"--raiz", raiz}, args...)...); code != 0 {
			t.Fatalf("hz %v: %s", args, e)
		}
	}
	if code, _, e := hz(t, "--raiz", raiz, "salida", "--rol", "verificador", "--tarea", "T-7"); code != 1 || !strings.Contains(e, "sprites.json") {
		t.Fatalf("antes de superarla, T-7 falla por el golden regenerado: %d %q", code, e)
	}
	pasos := []struct {
		args       []string
		codigo     int
		out, errSt string
	}{
		{[]string{"tdd", "superar", "--help"}, 0, "--por", ""},
		{[]string{"tdd", "superar", "T-7", "--por", "T-11"}, 2, "", "--decision"},
		{[]string{"tdd", "superar", "T-7", "--por", "T-11", "--decision", "D-0404"}, 1, "", "D-0404"},
		{[]string{"tdd", "superar", "T-7", "--por", "T-11", "--decision", "D-0001", "--motivo", "golden regenerado tras aprobación"}, 0, "T-7 → SUPERADA por T-11 (D-0001)", ""},
		{[]string{"tdd", "superar", "T-7", "--por", "T-11", "--decision", "D-0001"}, 1, "", "ya fue superada"},
		{[]string{"tdd", "estado"}, 0, "T-7 · SUPERADA", ""},
		{[]string{"tdd", "estado", "T-7"}, 0, "superada por T-11 (D-0001)", ""},
		{[]string{"tdd", "estado", "T-7"}, 0, "golden regenerado tras aprobación", ""},
		{[]string{"tablero"}, 0, "superada por T-11 (D-0001)", ""},
		{[]string{"salida", "--rol", "verificador", "--tarea", "T-7"}, 0, "superada por T-11 (D-0001)", ""},
	}
	for _, p := range pasos {
		code, out, e := hz(t, append([]string{"--raiz", raiz}, p.args...)...)
		if code != p.codigo || !strings.Contains(out, p.out) || !strings.Contains(e, p.errSt) {
			t.Fatalf("hz %v: código %d (se esperaba %d), salida %q, error %q", p.args, code, p.codigo, out, e)
		}
	}
	if _, out, _ := hz(t, "--raiz", raiz, "estado"); strings.Contains(out, "T-7 ·") || !strings.Contains(out, "T-11 ·") {
		t.Fatalf("una tarea superada no cuenta como abierta en hz estado:\n%s", out)
	}
	b, err := os.ReadFile(filepath.Join(raiz, ".harness", "cache", "hooks.json"))
	if err != nil || !strings.Contains(string(b), `"tareas_superadas"`) || !strings.Contains(string(b), `"por": "T-11"`) {
		t.Fatalf("exportar lista la tarea superada en hooks.json: %v\n%s", err, b)
	}
}

// candadoDeMutar fabrica el candado y el respaldo de una medición de hz mutar (formato de contratos/cli.md).
func candadoDeMutar(t *testing.T, raiz, rel, original string, pid int) {
	t.Helper()
	host, _ := os.Hostname()
	bak := mutar.RutaRespaldo(raiz, rel)
	h := sha256.Sum256([]byte(original))
	b, _ := json.Marshal(map[string]any{"ruta": rel, "pid": pid, "host": host, "inicio": time.Now().Format(time.RFC3339), "sha256": hex.EncodeToString(h[:])})
	if err := os.MkdirAll(filepath.Dir(bak), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(bak, ".bak")+".lock", b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bak, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMutarAvisaHuerfanosYRechazaElMismoArchivo(t *testing.T) {
	raiz := t.TempDir()
	if code, _, e := hz(t, "--raiz", raiz, "activar"); code != 0 {
		t.Fatalf("activar: %s", e)
	}
	const calc, otro = "func Sumar(a, b int) int { return a + b }\n", "func Listo() bool { return true }\n"
	for rel, txt := range map[string]string{"calc.go": "func Sumar(a, b int) int { return a - b }\n", "otro.go": otro} {
		if err := os.WriteFile(filepath.Join(raiz, rel), []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	candadoDeMutar(t, raiz, "calc.go", calc, 2147483600)
	cmd := `grep -q "return true" otro.go`
	code, out, e := hz(t, "--raiz", raiz, "mutar", "--archivo", "otro.go", "--cmd", cmd)
	if code != 0 || !strings.Contains(out, "detección 100%") || !strings.Contains(e, "aviso: restaurado calc.go") {
		t.Fatalf("hz mutar restaura el mutante huérfano y lo avisa por stderr: %d %q %q", code, out, e)
	}
	if b, _ := os.ReadFile(filepath.Join(raiz, "calc.go")); string(b) != calc {
		t.Fatalf("calc.go vuelve a su original: %q", b)
	}
	candadoDeMutar(t, raiz, "otro.go", otro, os.Getpid())
	if code, _, e := hz(t, "--raiz", raiz, "mutar", "--archivo", "otro.go", "--cmd", cmd); code != 1 || !strings.Contains(e, "en curso") || !strings.Contains(e, "otro.go") {
		t.Fatalf("una segunda medición sobre el mismo archivo se rechaza con código 1 y un mensaje claro: %d %q", code, e)
	}
}
