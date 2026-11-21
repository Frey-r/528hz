package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hz528/internal/proyecto"
)

func armar(t *testing.T, archivos map[string]string) string {
	t.Helper()
	raiz := t.TempDir()
	base := map[string]string{
		".harness/harness.yaml": "version: 1\nenabled: true\nnivel: N1\nmodo: mejora\n",
		"AGENTS.md":             "# Mapa\nVer `docs/guia.md`.\n",
		"docs/guia.md":          "guía\n",
	}
	for k, v := range archivos {
		base[k] = v
	}
	for rel, txt := range base {
		if txt == "<borrar>" {
			continue
		}
		ruta := filepath.Join(raiz, rel)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return raiz
}

func correr(t *testing.T, raiz string) []Resultado {
	t.Helper()
	p, err := proyecto.Cargar(raiz)
	if err != nil {
		t.Fatal(err)
	}
	return Ejecutar(raiz, p)
}

func fallo(rs []Resultado, contiene string) bool {
	for _, r := range rs {
		if !r.OK && strings.Contains(r.Nombre+" "+r.Motivo, contiene) {
			return true
		}
	}
	return false
}

func TestCheckProyectoSano(t *testing.T) {
	rs := correr(t, armar(t, nil))
	if !TodoOK(rs) {
		t.Fatalf("un proyecto sano no debe fallar: %+v", rs)
	}
	if len(rs) < 5 {
		t.Fatalf("se esperaban al menos 5 verificaciones, hay %d", len(rs))
	}
}

func TestCheckAgentsDemasiadoLargo(t *testing.T) {
	rs := correr(t, armar(t, map[string]string{"AGENTS.md": strings.Repeat("línea\n", 101)}))
	if !fallo(rs, "AGENTS.md") {
		t.Fatalf("un AGENTS.md de 101 líneas debe fallar: %+v", rs)
	}
}

func TestCheckLinkRotoYPlaceholdersIgnorados(t *testing.T) {
	rs := correr(t, armar(t, map[string]string{"AGENTS.md": "# Mapa\n`docs/falta.md` y `<dominio>/AGENTS.md` y `docs/*.md`\n"}))
	if !fallo(rs, "docs/falta.md") {
		t.Fatalf("debe reportar el link roto: %+v", rs)
	}
	if fallo(rs, "<dominio>") {
		t.Fatalf("los placeholders no son links: %+v", rs)
	}
	if fallo(rs, "docs/*.md") {
		t.Fatalf("un glob con coincidencias es válido: %+v", rs)
	}
}

func TestCheckNombresSueltosNoSonLinks(t *testing.T) {
	rs := correr(t, armar(t, map[string]string{"AGENTS.md": "# Mapa\nEl plugin lee el `hooks.json` que exporta hz; ver `docs/guia.md`.\n"}))
	if fallo(rs, "hooks.json") {
		t.Fatalf("un nombre sin carpeta es una mención, no una ruta verificable: %+v", rs)
	}
	rs = correr(t, armar(t, map[string]string{"AGENTS.md": "# Mapa\n`docs/hooks.json`\n"}))
	if !fallo(rs, "docs/hooks.json") {
		t.Fatalf("una ruta con carpeta sí debe verificarse: %+v", rs)
	}
}

func TestCheckClaudeMdSinImport(t *testing.T) {
	rs := correr(t, armar(t, map[string]string{"CLAUDE.md": "# reglas viejas\n"}))
	if !fallo(rs, "CLAUDE.md") {
		t.Fatalf("CLAUDE.md sin @AGENTS.md debe fallar (anula AGENTS.md): %+v", rs)
	}
	rs = correr(t, armar(t, map[string]string{"CLAUDE.md": "@AGENTS.md\n"}))
	if fallo(rs, "CLAUDE.md") {
		t.Fatalf("CLAUDE.md con @AGENTS.md es válido: %+v", rs)
	}
}

func TestCheckAgentsDeDominioLargo(t *testing.T) {
	rs := correr(t, armar(t, map[string]string{"app/pagos/AGENTS.md": strings.Repeat("x\n", 41)}))
	if !fallo(rs, "pagos") {
		t.Fatalf("un AGENTS.md de dominio de 41 líneas debe fallar: %+v", rs)
	}
}

func TestCheckPerfilInvalido(t *testing.T) {
	rs := correr(t, armar(t, map[string]string{".harness/harness.yaml": "version: 1\nenabled: true\nnivel: N7\nmodo: mejora\n"}))
	if !fallo(rs, "nivel") {
		t.Fatalf("un nivel inválido debe fallar: %+v", rs)
	}
}
