package motor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func errorBase(clase string) Error {
	return Error{Que: "colapsó la barra invertida al escribir un regex por heredoc", Fuente: "verificacion", Severidad: "mayor",
		Causa: "herramienta_mal_usada", Clase: clase, Ambito: "**/*.ts", Patron: "heredoc con barras invertidas"}
}

func TestAbrirReconstruyeUnIndiceConEsquemaViejo(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.NuevaDecision(decisionBase("antes de migrar")); err != nil {
		t.Fatal(err)
	}
	// Simula una base creada por una versión anterior: tabla sin la columna nueva y otra versión de esquema.
	for _, q := range []string{`DROP TABLE lecciones`, `CREATE TABLE lecciones (id TEXT PRIMARY KEY)`, `UPDATE meta SET valor = 'viejo' WHERE clave = 'esquema'`} {
		if _, err := m.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	_ = m.Cerrar()
	m2 := abrir(t, raiz)
	if _, err := m2.Lecciones(""); err != nil {
		t.Fatalf("un índice con esquema viejo debe reconstruirse solo: %v", err)
	}
	if n, _ := m2.ContarDecisiones(); n != 1 {
		t.Fatalf("la reconstrucción conserva lo que está en los logs: %d decisiones", n)
	}
}

func TestErrorNuevoDeduplicaPorClase(t *testing.T) {
	m := abrir(t, proyectoVacio(t))
	id1, rec1, err := m.NuevoError(errorBase("escape-heredoc"))
	if err != nil || id1 != "E-0001" || rec1 != 1 {
		t.Fatalf("primer error: %s %d %v", id1, rec1, err)
	}
	id2, rec2, err := m.NuevoError(errorBase("escape-heredoc"))
	if err != nil || id2 != "E-0002" || rec2 != 2 {
		t.Fatalf("la misma clase suma recurrencia: %s %d %v", id2, rec2, err)
	}
	c, err := m.Clase("escape-heredoc")
	if err != nil || c.Recurrencia != 2 || c.Ambito != "**/*.ts" || c.Causa != "herramienta_mal_usada" {
		t.Fatalf("clase inesperada: %+v %v", c, err)
	}
}

func TestErrorValidaEnumeraciones(t *testing.T) {
	m := abrir(t, proyectoVacio(t))
	e := errorBase("x")
	e.Fuente = "tdd_rojo"
	if _, _, err := m.NuevoError(e); !errors.Is(err, ErrCampos) || !strings.Contains(err.Error(), "fuente") {
		t.Fatalf("el ROJO del TDD no es una fuente de error válida: %v", err)
	}
	e = errorBase("x")
	e.Severidad = "grave"
	if _, _, err := m.NuevoError(e); !errors.Is(err, ErrCampos) {
		t.Fatalf("severidad inválida debe fallar: %v", err)
	}
}

func TestLeccionesEvalsYMedicionesSobrevivenReindex(t *testing.T) {
	raiz := proyectoVacio(t)
	m, err := Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.NuevoError(errorBase("escape-heredoc")); err != nil {
		t.Fatal(err)
	}
	id, err := m.NuevaLeccion(Leccion{Clase: "escape-heredoc", Skill: "hz-leccion-escape", Descripcion: "usa Edit para código con barras invertidas"})
	if err != nil || id != "L-0001" {
		t.Fatalf("nueva lección: %s %v", id, err)
	}
	if err := m.FijarEstadoLeccion("L-0001", "activa", "pasó sus evals"); err != nil {
		t.Fatal(err)
	}
	for _, tipo := range []string{"prevencion", "interferencia", "activacion"} {
		if err := m.RegistrarEval("L-0001", tipo, "pasa", 0.01, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RegistrarMedicion("mutacion:calc.go", 0.5, "1 de 2 detectados"); err != nil {
		t.Fatal(err)
	}
	if err := m.RegistrarRedteam("RD-1", "rt-1-7", "paso", "dlp", "contenido invertido"); err != nil {
		t.Fatal(err)
	}
	_ = m.Cerrar()
	_ = os.Remove(filepath.Join(raiz, ".harness", "harness.db"))
	m2 := abrir(t, raiz)
	l, err := m2.Leccion("L-0001")
	if err != nil || l.Estado != "activa" || l.Ambito != "**/*.ts" {
		t.Fatalf("lección tras reindex: %+v %v", l, err)
	}
	ev, err := m2.UltimosEvals("L-0001")
	if err != nil || len(ev) != 3 || ev["prevencion"] != "pasa" {
		t.Fatalf("evals tras reindex: %v %v", ev, err)
	}
	activas, err := m2.LeccionesActivas()
	if err != nil || len(activas) != 1 {
		t.Fatalf("lecciones activas: %+v %v", activas, err)
	}
	if v, err := m2.Medicion("mutacion:calc.go"); err != nil || v != 0.5 {
		t.Fatalf("medición del verificador: %v %v", v, err)
	}
	if rt, err := m2.EventosRedteam("RD-1"); err != nil || len(rt) != 1 || rt[0].Resultado != "paso" {
		t.Fatalf("red-team tras reindex: %+v %v", rt, err)
	}
}

func TestMedicionesPorPrefijo(t *testing.T) {
	m := abrir(t, t.TempDir())
	for id, v := range map[string]float64{"mutacion:T-1:a.go": 0.95, "mutacion:T-1:b.go": 0.5, "mutacion:T-2:a.go": 1, "mutacion:a.go": 0.95} {
		if err := m.RegistrarMedicion(id, v, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RegistrarMedicion("mutacion:T-1:b.go", 0.92, "reforzado"); err != nil {
		t.Fatal(err)
	}
	ms, err := m.MedicionesConPrefijo("mutacion:T-1:")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].ID != "mutacion:T-1:a.go" || ms[1].Deteccion != 0.92 || ms[1].Detalle != "reforzado" {
		t.Fatalf("solo las mediciones de T-1, la última por archivo y en orden: %+v", ms)
	}
	if ms, err := m.MedicionesConPrefijo("mutacion:T-9:"); err != nil || len(ms) != 0 {
		t.Fatalf("sin mediciones: %+v %v", ms, err)
	}
}

func TestCadaValorValidoSeAceptaYLoDemasNo(t *testing.T) {
	m := abrir(t, t.TempDir())
	base := Error{Que: "x", Fuente: "verificacion", Severidad: "menor", Causa: "otra", Clase: "c"}
	casos := []struct {
		campo   string
		validos []string
		fijar   func(e *Error, v string)
	}{
		{"fuente", []string{"verificacion", "evaluador", "humano", "regla_dura", "revert", "regresion"}, func(e *Error, v string) { e.Fuente = v }},
		{"severidad", []string{"critico", "mayor", "menor"}, func(e *Error, v string) { e.Severidad = v }},
		{"causa", []string{"contrato_ambiguo", "contexto_faltante", "regla_ignorada", "patron_incorrecto", "herramienta_mal_usada", "test_debil", "sobreajuste", "otra"}, func(e *Error, v string) { e.Causa = v }},
	}
	for _, c := range casos {
		for _, v := range append(c.validos, "inventado") {
			e := base
			c.fijar(&e, v)
			_, _, err := m.NuevoError(e)
			if (err == nil) != (v != "inventado") {
				t.Errorf("%s %q: error %v", c.campo, v, err)
			}
		}
	}
	id, err := m.NuevaLeccion(Leccion{Clase: "c", Skill: "hz-leccion-c"})
	if err != nil {
		t.Fatal(err)
	}
	for _, estado := range []string{"activa", "reescrita", "en_prueba", "podada", "eliminada", "inventado"} {
		if err := m.FijarEstadoLeccion(id, estado, "x"); (err == nil) != (estado != "inventado") {
			t.Errorf("estado de lección %q: %v", estado, err)
		}
	}
	for _, tipo := range []string{"prevencion", "interferencia", "activacion"} {
		for _, res := range []string{"pasa", "falla"} {
			if err := m.RegistrarEval(id, tipo, res, 0, ""); err != nil {
				t.Errorf("eval %s/%s: %v", tipo, res, err)
			}
		}
	}
	if err := m.RegistrarEval(id, "otro", "pasa", 0, ""); err == nil {
		t.Error("un tipo de eval inventado se rechaza")
	}
}

func TestErroresYRedteamSeFiltran(t *testing.T) {
	m := abrir(t, t.TempDir())
	for _, clase := range []string{"a", "b", "a"} {
		if _, _, err := m.NuevoError(Error{Que: "x", Fuente: "humano", Severidad: "menor", Causa: "otra", Clase: clase}); err != nil {
			t.Fatal(err)
		}
	}
	if es, err := m.Errores("a"); err != nil || len(es) != 2 {
		t.Fatalf("filtrar por clase: %d %v", len(es), err)
	}
	if es, err := m.Errores(""); err != nil || len(es) != 3 {
		t.Fatalf("sin filtro, todos: %d %v", len(es), err)
	}
	if err := m.RegistrarRedteam("RD-1", "", "paso", "hook", "x"); err == nil {
		t.Fatal("un resultado de red-team sin caso se rechaza")
	}
	for _, r := range []string{"RD-1", "rd-2"} {
		if err := m.RegistrarRedteam(r, "rt-1", "bloqueado", "hook", "x"); err != nil {
			t.Fatal(err)
		}
	}
	if rt, err := m.EventosRedteam("RD-2"); err != nil || len(rt) != 1 || rt[0].Regla != "RD-2" {
		t.Fatalf("filtrar red-team por regla (sin importar mayúsculas): %+v %v", rt, err)
	}
	if rt, err := m.EventosRedteam(""); err != nil || len(rt) != 2 {
		t.Fatalf("sin filtro, todos: %+v %v", rt, err)
	}
}
