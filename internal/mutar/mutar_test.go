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
