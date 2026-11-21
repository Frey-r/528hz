package reglas

import (
	"strings"
	"testing"

	"hz528"
)

func TestSeccionDevuelveSoloLaReglaPedida(t *testing.T) {
	txt, ok := Seccion(hz528.ReglasGenerales, "RG-1")
	if !ok {
		t.Fatal("RG-1 debe existir")
	}
	if !strings.Contains(txt, "RG-1.3") || strings.Contains(txt, "RG-2.1") {
		t.Fatalf("la sección debe contener RG-1.x y no RG-2.x:\n%s", txt)
	}
	if _, ok := Seccion(hz528.ReglasGenerales, "RG-9"); ok {
		t.Fatal("RG-9 no existe")
	}
	if _, ok := Seccion(hz528.ReglasGenerales, "rg-2"); !ok {
		t.Fatal("el ID debe ser insensible a mayúsculas")
	}
}
