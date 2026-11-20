package proyecto

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const reglasValidas = `version: 1
reglas:
  - id: RD-1
    clave_catalogo: secretos
    orden: "No leas ni imprimas secretos."
    por_que: "Filtración de credenciales."
    origen: genesis
    desde: 2026-11-20
    enforcement: [permisos, hook]
    patrones:
      rutas: [".env"]
    remediacion: "Usa las variables ya cargadas por la app."
    estado: vigente
  - id: RD-2
    orden: "No escribas en el sistema de tickets."
    por_que: "Llega a clientes reales."
    origen: mejora
    desde: 2026-11-20
    enforcement: [permisos, hook]
    patrones:
      herramientas: ["mcp__tickets__crear"]
      bash: ["curl .*tickets\\.ejemplo/api"]
    estado: vigente
  - id: RD-3
    orden: "Regla vieja."
    por_que: "Ya no aplica."
    origen: genesis
    desde: 2026-01-01
    enforcement: [hook, ci]
    estado: retirada
`

func TestCargarReglasSinArchivoDevuelveVacio(t *testing.T) {
	rs, err := CargarReglas(t.TempDir())
	if err != nil || len(rs) != 0 {
		t.Fatalf("sin reglas.yaml se esperaba lista vacía sin error: %v %v", rs, err)
	}
}

func TestCargarReglasValidasYVigentes(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, ".harness", "reglas.yaml"), reglasValidas)
	rs, err := CargarReglas(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 3 {
		t.Fatalf("se esperaban 3 reglas, hay %d", len(rs))
	}
	if rs[0].ClaveCatalogo != "secretos" || rs[0].Patrones.Rutas[0] != ".env" || rs[1].Patrones.Herramientas[0] != "mcp__tickets__crear" {
		t.Fatalf("reglas mal cargadas: %+v", rs)
	}
	if v := Vigentes(rs); len(v) != 2 {
		t.Fatalf("solo RD-1 y RD-2 están vigentes, hay %d", len(v))
	}
}

func TestCargarReglasRechazaReglaInvalida(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, ".harness", "reglas.yaml"), "version: 1\nreglas:\n  - id: X-1\n    orden: \"x\"\n    por_que: \"y\"\n    origen: genesis\n    desde: 2026-11-20\n    enforcement: [hook]\n    estado: vigente\n")
	_, err := CargarReglas(raiz)
	if err == nil || !strings.Contains(err.Error(), "X-1") || !strings.Contains(err.Error(), "capas") {
		t.Fatalf("se esperaba un error que nombre la regla y las capas mínimas, llegó %v", err)
	}
}

func leerSettings(t *testing.T, raiz string) map[string]any {
	t.Helper()
	var s map[string]any
	if err := json.Unmarshal([]byte(leer(t, filepath.Join(raiz, ".claude", "settings.json"))), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func denies(s map[string]any) []string {
	p, _ := s["permissions"].(map[string]any)
	lista, _ := p["deny"].([]any)
	out := make([]string, 0, len(lista))
	for _, v := range lista {
		out = append(out, v.(string))
	}
	return out
}

func contar(lista []string, v string) int {
	n := 0
	for _, x := range lista {
		if x == v {
			n++
		}
	}
	return n
}

func TestActivarN2CreaReglasYFusionaPermisosSinPisar(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, ".claude", "settings.json"),
		`{"outputStyle":"Concise","permissions":{"allow":["Bash(go test *)"],"deny":["Read(./secreto)"]}}`)
	for i := 0; i < 2; i++ {
		if _, err := Activar(raiz, "mejora", "N2"); err != nil {
			t.Fatal(err)
		}
	}
	if !existe(filepath.Join(raiz, ".harness", "reglas.yaml")) {
		t.Fatal("N2 debe crear .harness/reglas.yaml")
	}
	s := leerSettings(t, raiz)
	if s["outputStyle"] != "Concise" {
		t.Error("se perdió una clave existente de settings.json")
	}
	if p := s["permissions"].(map[string]any); len(p["allow"].([]any)) != 1 {
		t.Error("se perdieron las reglas allow existentes")
	}
	d := denies(s)
	for _, regla := range []string{"Read(./secreto)", "Edit(./.harness/reglas.yaml)", "Write(./.harness/log/**)", "Edit(./.claude/settings.json)", "Edit(./.githooks/**)"} {
		if contar(d, regla) != 1 {
			t.Errorf("deny %q debe aparecer exactamente una vez; deny = %v", regla, d)
		}
	}
	for _, r := range d {
		if strings.Contains(r, ".claude/**") {
			t.Errorf("no se protege todo .claude/**: las lecciones viven en .claude/skills (%s)", r)
		}
	}
	p, err := Cargar(raiz)
	if err != nil || p.Nivel != "N2" {
		t.Fatalf("harness.yaml debe quedar en N2: %+v %v", p, err)
	}
}

func TestActivarN2AgregaDenyDeReglasConCapaPermisos(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, filepath.Join(raiz, ".harness", "reglas.yaml"), reglasValidas)
	if _, err := Activar(raiz, "mejora", "N2"); err != nil {
		t.Fatal(err)
	}
	d := denies(leerSettings(t, raiz))
	for _, regla := range []string{"Read(./.env)", "mcp__tickets__crear"} {
		if contar(d, regla) != 1 {
			t.Errorf("falta deny %q derivado de una RD con capa permisos; deny = %v", regla, d)
		}
	}
}

func TestActivarSinNivelExplicitoNoDegrada(t *testing.T) {
	raiz := t.TempDir()
	if _, err := Activar(raiz, "mejora", "N2"); err != nil {
		t.Fatal(err)
	}
	if _, err := Activar(raiz, "", ""); err != nil {
		t.Fatal(err)
	}
	if p, _ := Cargar(raiz); p.Nivel != "N2" {
		t.Fatalf("activar sin --nivel no debe bajar el nivel: quedó %s", p.Nivel)
	}
}
