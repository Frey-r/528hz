package mutar

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSitiosIgnoranStringsYComentarios(t *testing.T) {
	src := "a := b + c // x + y\ns := \"p + q\"\nok := true\n/* m == n */\nd := e == f\n"
	sitios := Sitios(src, ".go")
	var ops []string
	for _, s := range sitios {
		ops = append(ops, s.Original)
	}
	if strings.Join(ops, ",") != " + ,true, == " {
		t.Fatalf("sitios = %q; se esperaban solo los de código", strings.Join(ops, ","))
	}
	if sitios[0].Linea != 1 || sitios[1].Linea != 3 || sitios[2].Linea != 5 {
		t.Fatalf("líneas inesperadas: %+v", sitios)
	}
	py := Sitios("x = a + b  # c + d\n", ".py")
	if len(py) != 1 {
		t.Fatalf("en Python # es comentario: %+v", py)
	}
}

func TestAplicarReemplazaSoloEseSitio(t *testing.T) {
	src := "x := a + b\ny := c + d\n"
	s := Sitios(src, ".go")[1]
	if got := Aplicar(src, s); got != "x := a + b\ny := c - d\n" {
		t.Fatalf("mutante inesperado: %q", got)
	}
}

const calcGo = `package calc

// Sumar devuelve a+b.
func Sumar(a, b int) int { return a + b }

// Mayor indica si a es mayor que b.
func Mayor(a, b int) bool { return a > b }
`

const calcTest = `package calc

import "testing"

func TestSumar(t *testing.T) {
	if Sumar(2, 3) != 5 {
		t.Fatal("suma")
	}
}
`

func TestMutarDetectaElTestDebilYRestauraElOriginal(t *testing.T) {
	raiz := t.TempDir()
	for rel, txt := range map[string]string{"go.mod": "module calc\n\ngo 1.21\n", "calc.go": calcGo, "calc_test.go": calcTest} {
		if err := os.WriteFile(filepath.Join(raiz, rel), []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	antes := sha256.Sum256([]byte(calcGo))
	r, err := Ejecutar(raiz, "calc.go", "go test ./...", 10, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 2 || r.Detectados != 1 || len(r.Sobrevivientes) != 1 {
		t.Fatalf("se esperaban 2 mutantes con 1 detectado: %+v", r)
	}
	if r.Sobrevivientes[0].Linea != 7 || !strings.Contains(r.Sobrevivientes[0].Original, ">") {
		t.Fatalf("el sobreviviente es el > de Mayor (sin test): %+v", r.Sobrevivientes[0])
	}
	despues, _ := os.ReadFile(filepath.Join(raiz, "calc.go"))
	if sha256.Sum256(despues) != antes {
		t.Fatal("el archivo original debe quedar restaurado exactamente")
	}
}

func TestMuestraRepartidaEnTodoElArchivo(t *testing.T) {
	var lineas []string
	for range 40 {
		lineas = append(lineas, "x := a + b")
	}
	sitios := Sitios(strings.Join(lineas, "\n")+"\n", ".go")
	m := Muestra(sitios, 10)
	if len(m) != 10 || m[0].Linea != 1 || m[len(m)-1].Linea < 37 {
		t.Fatalf("la muestra debe cubrir el archivo de punta a punta, no solo el comienzo: %+v", m)
	}
	if got := Muestra(sitios[:5], 10); len(got) != 5 {
		t.Fatalf("con menos sitios que el máximo van todos: %d", len(got))
	}
	if got := Muestra(sitios, 0); len(got) != 40 {
		t.Fatalf("max 0 significa sin tope: %d", len(got))
	}
}

func TestQueArchivosSonCodigoDeProduccion(t *testing.T) {
	for ruta, esperado := range map[string]bool{
		"internal/a/a.go": true, "src/app.ts": true, "web/comp.tsx": true, "pkg/mod.py": true,
		"internal/a/a_test.go": false, "src/app.test.ts": false, "src/app.spec.js": false, "tests/test_mod.py": false,
		"README.md": false, "config.yaml": false, "datos.json": false,
	} {
		if got := EsCodigoDeProduccion(ruta); got != esperado {
			t.Errorf("EsCodigoDeProduccion(%q) = %v, se esperaba %v", ruta, got, esperado)
		}
	}
}

func TestTrueYFalseDentroDeIdentificadoresNoSonSitios(t *testing.T) {
	sitios := Sitios("x_true := 1\nv2false := 2\ntrue_y := 3\nok := true\n", ".go")
	if len(sitios) != 1 || sitios[0].Linea != 4 {
		t.Fatalf("_ y los dígitos son parte del identificador: solo el true suelto de la línea 4 es un sitio: %+v", sitios)
	}
}
