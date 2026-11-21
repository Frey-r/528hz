package aprendizaje

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hz528/internal/motor"
)

func TestTransicionesDeLeccion(t *testing.T) {
	validas := [][2]string{{"en_prueba", "activa"}, {"en_prueba", "reescrita"}, {"reescrita", "en_prueba"}, {"activa", "reescrita"}, {"activa", "podada"}}
	for _, v := range validas {
		if err := Transicion(v[0], v[1], 0); err != nil {
			t.Errorf("%s → %s debe ser válida: %v", v[0], v[1], err)
		}
	}
	for _, v := range [][2]string{{"en_prueba", "podada"}, {"activa", "en_prueba"}, {"eliminada", "activa"}} {
		if err := Transicion(v[0], v[1], 0); err == nil {
			t.Errorf("%s → %s debe rechazarse", v[0], v[1])
		}
	}
	if err := Transicion("activa", "reescrita", 2); err == nil || !strings.Contains(err.Error(), "escala al humano") {
		t.Fatalf("la tercera reescritura escala al humano: %v", err)
	}
}

func TestDecidirLeccion(t *testing.T) {
	todosPasan := map[string]string{"prevencion": "pasa", "interferencia": "pasa", "activacion": "pasa"}
	casos := []struct {
		nombre       string
		evals        map[string]string
		reescrituras int
		criticos     int
		accion       string
	}{
		{"error crítico", todosPasan, 0, 1, "eliminar"},
		{"faltan evals", map[string]string{"prevencion": "pasa"}, 0, 0, "faltan evals"},
		{"todo pasa", todosPasan, 0, 0, "activar"},
		{"falla menor", map[string]string{"prevencion": "falla", "interferencia": "pasa", "activacion": "pasa"}, 0, 0, "reescribir"},
		{"falla tras 2 reescrituras", map[string]string{"prevencion": "pasa", "interferencia": "falla", "activacion": "pasa"}, 2, 0, "escalar"},
	}
	for _, c := range casos {
		if got := Decidir(c.evals, c.reescrituras, c.criticos); !strings.HasPrefix(got.Accion, c.accion) {
			t.Errorf("%s: acción %q, se esperaba %q (%s)", c.nombre, got.Accion, c.accion, got.Motivo)
		}
	}
}

func proyectoConLeccion(t *testing.T) (*motor.Motor, string) {
	t.Helper()
	raiz := t.TempDir()
	skill := filepath.Join(raiz, ".claude", "skills", "hz-leccion-escape", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: hz-leccion-escape\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Cerrar() })
	if _, _, err := m.NuevoError(motor.Error{Que: "x", Fuente: "humano", Severidad: "menor", Causa: "herramienta_mal_usada", Clase: "escape", Ambito: "**/*"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NuevaLeccion(m, raiz, motor.Leccion{Clase: "escape", Skill: "hz-leccion-escape", Descripcion: "d"}); err != nil {
		t.Fatal(err)
	}
	return m, raiz
}

func TestNuevaLeccionExigeLaSkillPrimero(t *testing.T) {
	m, raiz := proyectoConLeccion(t)
	if _, err := NuevaLeccion(m, raiz, motor.Leccion{Clase: "escape", Skill: "hz-leccion-inexistente"}); err == nil {
		t.Fatal("la lección se registra después de escribir su skill")
	}
}

func TestEliminarDeRaizBorraLaSkillYRegistraLaDecision(t *testing.T) {
	m, raiz := proyectoConLeccion(t)
	if err := Eliminar(m, raiz, "L-0001", "causó un error crítico"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(raiz, ".claude", "skills", "hz-leccion-escape")); !os.IsNotExist(err) {
		t.Fatal("eliminar de raíz debe borrar la carpeta de la skill")
	}
	if l, _ := m.Leccion("L-0001"); l.Estado != "eliminada" {
		t.Fatalf("estado = %s", l.Estado)
	}
	if ds, _ := m.BuscarDecisiones("L-0001 eliminada", 5); len(ds) == 0 {
		t.Fatal("eliminar una lección debe quedar registrado como decisión (RG-3)")
	}
}

func TestErrorCriticoQueImplicaALaLeccionLaElimina(t *testing.T) {
	m, raiz := proyectoConLeccion(t)
	_, _, accion, err := RegistrarError(m, raiz, motor.Error{Que: "la lección indujo a borrar datos", Fuente: "evaluador", Severidad: "critico",
		Causa: "patron_incorrecto", Clase: "leccion-danina", Leccion: "L-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(accion, "eliminada") {
		t.Fatalf("debe informar la eliminación: %q", accion)
	}
	if l, _ := m.Leccion("L-0001"); l.Estado != "eliminada" {
		t.Fatalf("un error crítico causado por la lección la elimina de raíz: %s", l.Estado)
	}
}
