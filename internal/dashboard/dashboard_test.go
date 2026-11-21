package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hz528/internal/exportar"
	"hz528/internal/motor"
)

var ahora = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func ms(hace time.Duration) int64 { return ahora.Add(-hace).UnixMilli() }

func ejemplo() Datos {
	act := &Actividad{Version: 1, Generado: ahora.Format(time.RFC3339), Agentes: []Agente{
		{ID: "principal", Rol: "director", Estado: "activo", Tools: 12, Ultima: "Edit a.go", Desde: ms(20 * time.Minute), Visto: ms(3 * time.Second)},
		{ID: "L1", Rol: "lider", Padre: "principal", Estado: "activo", Tools: 4, Desde: ms(9 * time.Minute), Visto: ms(10 * time.Second)},
		{ID: "r1", Rol: "tester", Tarea: "T-1", Padre: "L1", Estado: "terminado", Tools: 9, Desde: ms(8 * time.Minute), Visto: ms(6 * time.Minute)},
		{ID: "r2", Rol: "implementador", Tarea: "T-1", Padre: "L1", Estado: "activo", Tools: 5, Errores: 3, Seguidos: 3, Desde: ms(5 * time.Minute), Visto: ms(2 * time.Second)},
	}, Eventos: map[string][][2]int64{
		"principal": {{ms(15 * time.Minute), 0}, {ms(4 * time.Minute), 0}},
		"r2":        {{ms(5 * time.Minute), 2}, {ms(4 * time.Minute), 0}, {ms(3 * time.Minute), 1}, {ms(2 * time.Minute), 1}, {ms(1 * time.Minute), 1}},
		"r1":        {{ms(8 * time.Minute), 2}, {ms(7 * time.Minute), 0}, {ms(6 * time.Minute), 3}},
	}}
	return Datos{Proyecto: "demo", Nivel: "N2", Modo: "mejora", Arriendos: 2, Actividad: act,
		Tareas:   []exportar.Activa{{ID: "T-1", Estado: "rojo", TestsBloqueados: []string{}}},
		Misiones: []exportar.MisionExportada{{ID: "m1", Gasto: 3.2, UsdMax: 5}}}
}

func TestFragmentoArbolYCarriles(t *testing.T) {
	var b bytes.Buffer
	if err := Fragmento(&b, ejemplo(), ahora, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	h := b.String()
	for _, debe := range []string{"demo", "T-1 ROJO", "m1", "<svg", "3 seguidos", "hace 2 s", "−10 min", "ahora"} {
		if !strings.Contains(h, debe) {
			t.Errorf("el fragmento debe mostrar %q", debe)
		}
	}
	orden := []string{"director", "lider", "implementador", "tester"}
	ultimo := -1
	for _, rol := range orden {
		i := strings.Index(h, ">"+rol)
		if i < ultimo {
			t.Fatalf("orden de árbol con los vivos primero: %v (falló en %s)", orden, rol)
		}
		ultimo = i
	}
	if n := strings.Count(h, `class="ev err"`); n != 3 {
		t.Errorf("tres errores dentro de la ventana, se dibujaron %d", n)
	}
	if n := strings.Count(h, `class="ev ok"`); n != 3 {
		t.Errorf("tool calls dentro de la ventana: principal 1 (el de hace 15 min queda fuera), r2 1, r1 1; se dibujaron %d", n)
	}
	if strings.Contains(strings.ToLower(h), "<script") || strings.Contains(h, "ZgotmplZ") {
		t.Error("el fragmento es HTML seguro sin scripts")
	}
}

func TestPosicionDeUnEvento(t *testing.T) {
	if x := xEvento(ms(5*time.Minute), ahora, 10*time.Minute); x != carrilIzq+carrilAncho/2 {
		t.Fatalf("un evento a mitad de la ventana va a mitad del carril: %v", x)
	}
	if x := xEvento(ms(0), ahora, 10*time.Minute); x != carrilIzq+carrilAncho {
		t.Fatalf("ahora es el borde derecho: %v", x)
	}
	if x := xEvento(ms(20*time.Minute), ahora, 10*time.Minute); x >= 0 {
		t.Fatalf("un evento fuera de la ventana no se dibuja (x < 0): %v", x)
	}
}

func TestFragmentoSinActividad(t *testing.T) {
	d := ejemplo()
	d.Actividad = nil
	var b bytes.Buffer
	if err := Fragmento(&b, d, ahora, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "sin actividad") {
		t.Fatalf("sin snapshot del plugin, el estado vacío lo explica:\n%s", b.String())
	}
}

func TestServidorSoloLocalYDeLectura(t *testing.T) {
	raiz := t.TempDir()
	for rel, txt := range map[string]string{
		".harness/harness.yaml":         "version: 1\nenabled: true\nnivel: N2\nmodo: mejora\n",
		".harness/cache/actividad.json": `{"version":1,"generado":"x","agentes":[{"id":"principal","rol":"director","estado":"activo","tools":1,"desde":1,"visto":1}],"eventos":{"principal":[[1,0]]}}`,
	} {
		ruta := filepath.Join(raiz, rel)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	srv := Servidor(m, raiz, 5280)
	pedir := func(metodo, host, ruta string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(metodo, ruta, nil)
		r.Host = host
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		return w
	}
	if w := pedir("GET", "evil.example:5280", "/api/panel"); w.Code != http.StatusForbidden {
		t.Fatalf("un Host ajeno (DNS rebinding) se rechaza: %d", w.Code)
	}
	if w := pedir("GET", "127.0.0.1:9999", "/"); w.Code != http.StatusForbidden {
		t.Fatalf("otro puerto en el Host también se rechaza: %d", w.Code)
	}
	if w := pedir("POST", "127.0.0.1:5280", "/api/panel"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("solo lectura: %d", w.Code)
	}
	for _, host := range []string{"127.0.0.1:5280", "localhost:5280", "[::1]:5280"} {
		if w := pedir("GET", host, "/"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<title>") || !strings.Contains(w.Body.String(), "/app.js") {
			t.Fatalf("la página se sirve en %s: %d", host, w.Code)
		}
	}
	if w := pedir("GET", "127.0.0.1:5280", "/app.js"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/fragmento") {
		t.Fatalf("el script refresca el fragmento: %d", w.Code)
	}
	if w := pedir("GET", "127.0.0.1:5280", "/fragmento?ventana=30m"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "director") {
		t.Fatalf("fragmento: %d %s", w.Code, w.Body.String())
	}
	if w := pedir("GET", "127.0.0.1:5280", "/fragmento?ventana=3h"); w.Code != http.StatusBadRequest {
		t.Fatalf("solo ventanas de 5m, 10m o 30m: %d", w.Code)
	}
	w := pedir("GET", "127.0.0.1:5280", "/api/panel")
	var p struct {
		Agentes []map[string]any   `json:"agentes"`
		Eventos map[string][][]int `json:"eventos"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &p) != nil || len(p.Agentes) != 1 || len(p.Eventos["principal"]) != 1 {
		t.Fatalf("api/panel: %d %s", w.Code, w.Body.String())
	}
}

func TestGlifosArbolRaroYResumen(t *testing.T) {
	for estado, g := range map[string]string{"lanzado": "○", "activo": "●", "terminado": "✓", "fallido": "✕", "detenido": "■"} {
		if got := (Agente{Estado: estado}).Glifo(); got != g {
			t.Errorf("glifo de %s: %q, se esperaba %q", estado, got, g)
		}
	}
	if (Agente{Estado: "activo", Seguidos: 3}).Glifo() != "!" {
		t.Error("3 errores seguidos se marcan con !")
	}
	nodos := Arbol([]Agente{{ID: "a", Padre: "a", Estado: "activo"}, {ID: "b", Padre: "fantasma", Estado: "activo"}, {ID: "c", Padre: "a", Estado: "activo"}})
	niveles := map[string]int{}
	for _, n := range nodos {
		niveles[n.Agente.ID] = n.Nivel
	}
	if len(nodos) != 3 || niveles["a"] != 0 || niveles["b"] != 0 || niveles["c"] != 1 {
		t.Fatalf("un agente que es su propio padre o cuyo padre no existe es raíz: %+v", nodos)
	}
	if got := Resumen(Agente{Estado: "activo", Tools: 1, Visto: ahora.UnixMilli()}, ahora); got != "1 tools · hace 0 s" {
		t.Fatalf("sin última acción no queda un hueco en el resumen: %q", got)
	}
}

func TestFragmentoMarcaFallidosYBarraDePresupuesto(t *testing.T) {
	d := ejemplo()
	d.Actividad.Agentes = append(d.Actividad.Agentes, Agente{ID: "r3", Rol: "verificador", Padre: "L1", Estado: "fallido", Desde: ms(time.Minute), Visto: ms(time.Second)})
	d.Misiones = append(d.Misiones, exportar.MisionExportada{ID: "m2", Gasto: 1})
	var b bytes.Buffer
	if err := Fragmento(&b, d, ahora, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	h := b.String()
	if !strings.Contains(h, `<div class="fila alerta" role="row">`+"\n      "+`<span class="nombre n2" role="cell" title=""><span class="glifo" aria-hidden="true">✕</span>verificador`) {
		t.Fatalf("un agente fallido se marca como alerta aunque no tenga errores seguidos:\n%s", h)
	}
	if n := strings.Count(h, `class="barrita"`); n != 1 {
		t.Fatalf("solo la misión con tope lleva barra de presupuesto (m1 sí, m2 no): %d", n)
	}
}

func TestAPINoEscapaElTexto(t *testing.T) {
	raiz := t.TempDir()
	for rel, txt := range map[string]string{
		".harness/harness.yaml":         "version: 1\nenabled: true\nnivel: N2\nmodo: mejora\n",
		".harness/cache/actividad.json": `{"version":1,"generado":"x","agentes":[{"id":"principal","rol":"director","estado":"activo","ultima":"Bash a && b","desde":1,"visto":1}]}`,
	} {
		ruta := filepath.Join(raiz, rel)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	r := httptest.NewRequest("GET", "/api/panel", nil)
	r.Host = "127.0.0.1:5280"
	w := httptest.NewRecorder()
	Servidor(m, raiz, 5280).ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "Bash a && b") || !strings.Contains(w.Body.String(), `"eventos": {}`) && !strings.Contains(w.Body.String(), `"eventos":{}`) {
		t.Fatalf("el JSON conserva el texto tal cual y trae eventos vacíos sin snapshot de eventos: %s", w.Body.String())
	}
}
