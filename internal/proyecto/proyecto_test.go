package proyecto

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribir(t *testing.T, ruta, texto string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
}

func leer(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func existe(ruta string) bool {
	_, err := os.Stat(ruta)
	return err == nil
}

func mismaRuta(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func TestBuscarEncuentraRaizDesdeSubcarpeta(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, ".harness", "harness.yaml"), "version: 1\nenabled: true\nnivel: N1\nmodo: mejora\n")
	sub := filepath.Join(raiz, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Buscar(sub)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !mismaRuta(got, raiz) {
		t.Fatalf("raíz = %q, se esperaba %q", got, raiz)
	}
}

func TestBuscarSinProyecto(t *testing.T) {
	_, err := Buscar(t.TempDir())
	if !errors.Is(err, ErrSinProyecto) {
		t.Fatalf("se esperaba ErrSinProyecto, llegó %v", err)
	}
}

func TestActivarCreaPerfilYNuncaCreaClaudeMd(t *testing.T) {
	raiz := t.TempDir()
	if _, err := Activar(raiz, "genesis", "N1"); err != nil {
		t.Fatalf("activar: %v", err)
	}
	for _, rel := range []string{".harness/harness.yaml", ".harness/log", "AGENTS.md", ".gitignore"} {
		if !existe(filepath.Join(raiz, rel)) {
			t.Errorf("falta %s", rel)
		}
	}
	if existe(filepath.Join(raiz, "CLAUDE.md")) {
		t.Error("activar creó CLAUDE.md: anularía la lectura nativa de AGENTS.md (D-0025)")
	}
	ignorado := leer(t, filepath.Join(raiz, ".gitignore"))
	if !strings.Contains(ignorado, ".harness/harness.db") {
		t.Error(".gitignore no ignora la base del harness")
	}
	if !strings.Contains(ignorado, ".harness/cache/") {
		t.Error(".gitignore no ignora .harness/cache/ (hooks.json es generado; versionarlo daría estado viejo a los worktrees)")
	}
	p, err := Cargar(raiz)
	if err != nil {
		t.Fatalf("cargar: %v", err)
	}
	if p.Nivel != "N1" || p.Modo != "genesis" || !p.Enabled || p.Version != 1 {
		t.Errorf("perfil inesperado: %+v", p)
	}
}

func TestActivarEsIdempotenteYNoPisaAgents(t *testing.T) {
	raiz := t.TempDir()
	if _, err := Activar(raiz, "mejora", "N1"); err != nil {
		t.Fatal(err)
	}
	propio := "# Mi mapa\nno tocar\n"
	escribir(t, filepath.Join(raiz, "AGENTS.md"), propio)
	acciones, err := Activar(raiz, "mejora", "N1")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range acciones {
		if a.Resultado == "creado" || a.Resultado == "modificado" {
			t.Errorf("segunda activación no debía cambiar nada: %+v", a)
		}
	}
	if leer(t, filepath.Join(raiz, "AGENTS.md")) != propio {
		t.Error("activar pisó un AGENTS.md existente")
	}
}

func TestActivarAgregaImportAClaudeMdExistenteUnaSolaVez(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, "CLAUDE.md"), "# Instrucciones previas\n")
	for i := 0; i < 2; i++ {
		if _, err := Activar(raiz, "mejora", "N1"); err != nil {
			t.Fatal(err)
		}
	}
	got := leer(t, filepath.Join(raiz, "CLAUDE.md"))
	if strings.Count(got, "@AGENTS.md") != 1 {
		t.Fatalf("CLAUDE.md debe importar @AGENTS.md exactamente una vez; contenido:\n%s", got)
	}
	if !strings.HasPrefix(got, "# Instrucciones previas") {
		t.Error("se perdió el contenido previo de CLAUDE.md")
	}
}

func TestValidarDetectaValoresInvalidos(t *testing.T) {
	p := Perfil{Version: 1, Enabled: true, Nivel: "N9", Modo: "otro"}
	errs := p.Validar()
	unidos := strings.Join(errs, " | ")
	if !strings.Contains(unidos, "nivel") || !strings.Contains(unidos, "modo") {
		t.Fatalf("se esperaban errores de nivel y modo, llegó: %q", unidos)
	}
}

func TestCargarAplicaLimitesPorDefecto(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, ".harness", "harness.yaml"), "version: 1\nenabled: true\nnivel: N2\nmodo: mejora\ncomandos:\n  test: go test ./...\n")
	p, err := Cargar(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if p.Limites.LineasMaxAgentsRaiz != 100 || p.Limites.LineasMaxAgentsDominio != 40 {
		t.Errorf("límites por defecto inesperados: %+v", p.Limites)
	}
	if p.Comandos["test"] != "go test ./..." {
		t.Errorf("comandos no cargados: %+v", p.Comandos)
	}
}
