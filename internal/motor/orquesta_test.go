package motor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTareaConservaCelulaYDependenciasTrasReindex(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RegistrarTarea("tdd.contrato", Tarea{ID: "T-2", Titulo: "x", Estado: "contrato", Celula: "pagos", BloqueadaPor: []string{"T-1"}}); err != nil {
		t.Fatal(err)
	}
	_ = m.Cerrar()
	_ = os.Remove(filepath.Join(raiz, ".harness", "harness.db"))
	ta, err := abrir(t, raiz).Tarea("T-2")
	if err != nil || ta.Celula != "pagos" || len(ta.BloqueadaPor) != 1 || ta.BloqueadaPor[0] != "T-1" {
		t.Fatalf("célula y dependencias deben sobrevivir al reindex: %+v %v", ta, err)
	}
}

func TestArriendosExclusivosRenovablesYConVencimiento(t *testing.T) {
	raiz := proyectoVacio(t)
	m := abrir(t, raiz)
	en := time.Now().Add(30 * time.Minute)
	if err := m.TomarArriendo([]string{"app/a.go", "app/b.go"}, "ag1", "T-1", en); err != nil {
		t.Fatal(err)
	}
	err := m.TomarArriendo([]string{"app/b.go"}, "ag2", "T-2", en)
	if !errors.Is(err, ErrConflicto) || !strings.Contains(err.Error(), "ag1") {
		t.Fatalf("un arriendo ajeno vigente debe dar ErrConflicto nombrando al titular, llegó %v", err)
	}
	if err := m.TomarArriendo([]string{"app/b.go"}, "ag1", "T-1", en.Add(time.Hour)); err != nil {
		t.Fatalf("el titular puede renovar: %v", err)
	}
	if err := m.SoltarArriendos("ag1", []string{"app/b.go"}); err != nil {
		t.Fatal(err)
	}
	if err := m.TomarArriendo([]string{"app/b.go"}, "ag2", "T-2", en); err != nil {
		t.Fatalf("tras soltar, otro agente puede tomarlo: %v", err)
	}
	if err := m.TomarArriendo([]string{"app/c.go"}, "ag3", "T-3", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := m.TomarArriendo([]string{"app/c.go"}, "ag4", "T-4", en); err != nil {
		t.Fatalf("un arriendo vencido no bloquea: %v", err)
	}
	vig, err := m.Arriendos()
	if err != nil {
		t.Fatal(err)
	}
	if len(vig) != 3 {
		t.Fatalf("se esperaban 3 arriendos vigentes (a de ag1, b de ag2, c de ag4), hay %d: %+v", len(vig), vig)
	}
	_ = m.Cerrar()
	_ = os.Remove(filepath.Join(raiz, ".harness", "harness.db"))
	if vig2, _ := abrir(t, raiz).Arriendos(); len(vig2) != 3 {
		t.Fatalf("los arriendos vigentes deben sobrevivir al reindex: %+v", vig2)
	}
}

func TestTarjetasCorrelativasYUltimaPorRol(t *testing.T) {
	m := abrir(t, proyectoVacio(t))
	id1, err := m.NuevaTarjeta(Tarjeta{TareaID: "T-1", Rol: "tester", Estado: "listo", Texto: "hecho: tests en rojo"})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := m.NuevaTarjeta(Tarjeta{TareaID: "T-1", Rol: "implementador", Estado: "listo", Texto: "hecho: verde"})
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "TR-0001" || id2 != "TR-0002" {
		t.Fatalf("IDs = %s, %s", id1, id2)
	}
	if u, err := m.UltimaTarjeta("T-1", ""); err != nil || u.Rol != "implementador" {
		t.Fatalf("la última de la tarea debe ser la del implementador: %+v %v", u, err)
	}
	if u, err := m.UltimaTarjeta("T-1", "tester"); err != nil || u.ID != "TR-0001" {
		t.Fatalf("la última del tester debe ser TR-0001: %+v %v", u, err)
	}
	if _, err := m.UltimaTarjeta("T-9", ""); !errors.Is(err, ErrNoExiste) {
		t.Fatalf("sin tarjetas debe dar ErrNoExiste, llegó %v", err)
	}
	if _, err := m.NuevaTarjeta(Tarjeta{TareaID: "T-1", Rol: "tester", Estado: "listo", Texto: strings.Repeat("l\n", 31)}); !errors.Is(err, ErrCampos) {
		t.Fatalf("una tarjeta de más de 30 líneas debe rechazarse, llegó %v", err)
	}
}
