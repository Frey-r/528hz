package mision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrearYCargarMisionValida(t *testing.T) {
	raiz := t.TempDir()
	if err := Crear(raiz, "m1", "mejora", "subir la resolución autónoma", 25); err != nil {
		t.Fatal(err)
	}
	m, err := Cargar(raiz, "m1")
	if err != nil {
		t.Fatalf("la misión recién creada debe ser válida: %v", err)
	}
	if m.Modo != "mejora" || m.Presupuesto.UsdMax != 25 || len(m.Aceptacion) == 0 || m.Estadistica.K != 2 {
		t.Fatalf("misión inesperada: %+v", m)
	}
	if err := Crear(raiz, "m1", "mejora", "otra", 0); err == nil {
		t.Fatal("no se sobrescribe una misión existente")
	}
	if e, err := LeerEstado(raiz, "m1"); err != nil || e.Estado != "activa" || e.Fase == "" {
		t.Fatalf("estado inicial: %+v %v", e, err)
	}
}

func TestVerticalSinLasTresFamiliasEsInvalida(t *testing.T) {
	raiz := t.TempDir()
	if err := Crear(raiz, "m1", "mejora", "x", 0); err != nil {
		t.Fatal(err)
	}
	yaml := `id: m1
titulo: m1
modo: mejora
objetivo: x
presupuesto: {usd_max: 0}
aceptacion: [{id: A1, texto: "x", cumplido: false}]
verticales:
  - id: V-resolucion
    nombre: Resolución
    peso: 1
    metricas:
      - {id: tasa, familia: efectividad, sensor: "echo []", direccion: sube, meta: 0.25}
`
	if err := os.WriteFile(filepath.Join(raiz, ".harness", "misiones", "m1", "mision.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Cargar(raiz, "m1")
	if err == nil || !strings.Contains(err.Error(), "impacto") || !strings.Contains(err.Error(), "fallos") {
		t.Fatalf("una vertical mide efectividad, impacto y fallos (D-0022): %v", err)
	}
}

func TestFaseYProximoPasoParaRetomar(t *testing.T) {
	raiz := t.TempDir()
	if err := Crear(raiz, "m1", "genesis", "x", 0); err != nil {
		t.Fatal(err)
	}
	if err := FijarFase(raiz, "m1", "hitos", "implementar el hito 2"); err != nil {
		t.Fatal(err)
	}
	e, err := LeerEstado(raiz, "m1")
	if err != nil || e.Fase != "hitos" || e.ProximoPaso != "implementar el hito 2" || e.Mision != "m1" {
		t.Fatalf("estado: %+v %v", e, err)
	}
}

func TestSensorDevuelveItems(t *testing.T) {
	items, err := Sensor(t.TempDir(), `echo '[{"item_id":"a","resultado":true},{"item_id":"b","resultado":false},{"item_id":"c","puntaje":0.5}]'`)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != "a" || *items[0].Resultado != true || *items[2].Puntaje != 0.5 {
		t.Fatalf("ítems inesperados: %+v", items)
	}
	if _, err := Sensor(t.TempDir(), "echo esto no es json"); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("un sensor que no emite JSON por ítem debe fallar claro: %v", err)
	}
	if _, err := Sensor(t.TempDir(), "exit 3"); err == nil {
		t.Fatal("un sensor que falla no produce medición")
	}
}

func TestIDsQueEscapanDeLaCarpetaSeRechazan(t *testing.T) {
	raiz := t.TempDir()
	for _, id := range []string{"../fuera", `..\fuera`, "a/b", "", ".oculta"} {
		if err := Crear(raiz, id, "mejora", "x", 0); err == nil {
			t.Errorf("el id %q debe rechazarse: la misión vive en .harness/misiones/<id>", id)
		}
	}
	if _, err := os.Stat(filepath.Join(raiz, "fuera")); err == nil {
		t.Fatal("se creó una carpeta fuera de .harness/misiones")
	}
	escribir := func(id, metrica string) {
		ruta := Ruta(raiz, id, "mision.yaml")
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		y := "id: " + id + "\nmodo: mejora\nverticales:\n  - id: V\n    metricas:\n" +
			"      - {id: " + metrica + ", familia: efectividad, sensor: x, direccion: sube}\n" +
			"      - {id: b, familia: impacto, sensor: x, direccion: sube}\n" +
			"      - {id: c, familia: fallos, sensor: x, direccion: baja}\n"
		if err := os.WriteFile(ruta, []byte(y), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escribir("m9", `"../x"`)
	if _, err := Cargar(raiz, "m9"); err == nil || !strings.Contains(err.Error(), "../x") {
		t.Fatalf("una métrica con id de ruta debe rechazarse (sus ítems se guardan como <metrica>.<conjunto>.json): %v", err)
	}
}
