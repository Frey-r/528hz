package main

import (
	"bytes"
	"encoding/json"
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
