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
	"hz528/internal/registro"
)

var ahora = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func ms(hace time.Duration) int64 { return ahora.Add(-hace).UnixMilli() }

func iso(hace time.Duration) string { return ahora.Add(-hace).Format(time.RFC3339) }

// sesionDemo: director → líder → (tester terminado, implementador con 3 errores seguidos).
func sesionDemo() Sesion {
	return Sesion{ID: "s1abcdef-0000", Generado: iso(time.Second), Agentes: []Agente{
		{ID: "principal", Rol: "director", Estado: "activo", Tools: 12, Ultima: "Edit a.go", Desde: ms(20 * time.Minute), Visto: ms(3 * time.Second)},
		{ID: "L1", Rol: "lider", Padre: "principal", Estado: "activo", Tools: 4, Desde: ms(9 * time.Minute), Visto: ms(10 * time.Second)},
		{ID: "r1", Rol: "tester", Tarea: "T-1", Padre: "L1", Estado: "terminado", Tools: 9, Desde: ms(8 * time.Minute), Visto: ms(6 * time.Minute)},
		{ID: "r2", Rol: "implementador", Tarea: "T-1", Padre: "L1", Estado: "activo", Tools: 5, Errores: 3, Seguidos: 3, Desde: ms(5 * time.Minute), Visto: ms(2 * time.Second)},
	}, Eventos: map[string][][2]int64{
		"principal": {{ms(15 * time.Minute), 0}, {ms(4 * time.Minute), 0}},
		"r2":        {{ms(5 * time.Minute), 2}, {ms(4 * time.Minute), 0}, {ms(3 * time.Minute), 1}, {ms(2 * time.Minute), 1}, {ms(1 * time.Minute), 1}},
		"r1":        {{ms(8 * time.Minute), 2}, {ms(7 * time.Minute), 0}, {ms(6 * time.Minute), 3}},
	}}
}

func demo() Proyecto {
	return Proyecto{ID: "aaaa0001", Nombre: "demo", Raiz: "/x/demo", Disponible: true, Nivel: "N2", Modo: "mejora", Arriendos: 2,
		Tareas:   []exportar.Activa{{ID: "T-1", Estado: "rojo", TestsBloqueados: []string{}}},
		Misiones: []exportar.MisionExportada{{ID: "m1", Gasto: 3.2, UsdMax: 5}},
		Sesiones: []Sesion{sesionDemo()}}
}

func fragmento(t *testing.T, ps []Proyecto, filtro string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Fragmento(&b, ps, filtro, ahora, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	h := b.String()
	if strings.Contains(strings.ToLower(h), "<script") || strings.Contains(h, "ZgotmplZ") {
		t.Fatal("el fragmento es HTML seguro sin scripts")
	}
	return h
}

func enOrden(t *testing.T, h string, partes ...string) {
	t.Helper()
	ultimo := -1
	for _, p := range partes {
		i := strings.Index(h, p)
		if i < 0 || i < ultimo {
			t.Fatalf("se esperaba este orden %q (falló en %q):\n%s", partes, p, h)
		}
		ultimo = i
	}
}

func TestFragmentoArbolYCarriles(t *testing.T) {
	h := fragmento(t, []Proyecto{demo()}, "")
	for _, debe := range []string{"demo", "T-1 ROJO", "m1", "<svg", "3 seguidos", "hace 2 s", "−10 min", "ahora", "sesión s1abcdef · activa"} {
		if !strings.Contains(h, debe) {
			t.Errorf("el fragmento debe mostrar %q", debe)
		}
	}
	enOrden(t, h, "sesión s1abcdef", ">lider", ">implementador", ">tester")
	if n := strings.Count(h, `class="ev err"`); n != 3 {
		t.Errorf("tres errores dentro de la ventana, se dibujaron %d", n)
	}
	if n := strings.Count(h, `class="ev ok"`); n != 3 {
		t.Errorf("tool calls dentro de la ventana: principal 1 (el de hace 15 min queda fuera), r2 1, r1 1; se dibujaron %d", n)
	}
	if strings.Contains(h, ">director") {
		t.Error("la sesión principal se dibuja como la raíz «sesión …», no como una fila «director» aparte")
	}
	if !strings.Contains(h, `<div class="fila sesion" role="row">`) {
		t.Error("una sesión activa sin errores seguidos en la principal es una fila de sesión normal")
	}
	if c := claseDeFila(h, ">tester"); c != "fila fin" {
		t.Errorf("un subagente terminado de una sesión activa se atenúa: %q", c)
	}
	if c := claseDeFila(h, ">lider"); c != "fila " {
		t.Errorf("un subagente vivo de una sesión activa no se atenúa: %q", c)
	}
}

// claseDeFila devuelve el atributo class de la fila que contiene marca.
func claseDeFila(h, marca string) string {
	i := strings.Index(h, marca)
	if i < 0 {
		return "no está"
	}
	fila := h[strings.LastIndex(h[:i], `<div class="`)+len(`<div class="`):]
	return fila[:strings.Index(fila, `"`)]
}

func TestSesionConAlertaYSesionCerradaConEventosRecientes(t *testing.T) {
	alerta := Sesion{ID: "s-alerta", Generado: iso(time.Second), Agentes: []Agente{
		{ID: "principal", Rol: "director", Estado: "activo", Seguidos: 3, Errores: 3, Desde: ms(time.Minute), Visto: ms(time.Second)},
	}, Eventos: map[string][][2]int64{}}
	cerrada := Sesion{ID: "s-cerrada", Generado: iso(2 * time.Minute), Terminada: true, Agentes: []Agente{
		{ID: "principal", Rol: "director", Estado: "terminado", Desde: ms(9 * time.Minute), Visto: ms(2 * time.Minute)},
		{ID: "k1", Rol: "tester", Padre: "principal", Estado: "activo", Desde: ms(8 * time.Minute), Visto: ms(3 * time.Minute)},
	}, Eventos: map[string][][2]int64{"k1": {{ms(3 * time.Minute), 0}}}}
	vieja := Sesion{ID: "s-vieja", Generado: iso(50 * time.Minute), Terminada: true, Agentes: []Agente{
		{ID: "principal", Rol: "director", Estado: "terminado", Desde: ms(60 * time.Minute), Visto: ms(50 * time.Minute)},
	}, Eventos: map[string][][2]int64{"principal": {{ms(50 * time.Minute), 0}}}}
	p := Proyecto{ID: "acac0008", Nombre: "p", Disponible: true, Nivel: "N2", Modo: "mejora", Sesiones: []Sesion{alerta, cerrada, vieja}}
	h := fragmento(t, []Proyecto{p}, "")
	if c := claseDeFila(h, "sesión s-alerta"); c != "fila sesion alerta" {
		t.Errorf("la principal con 3 errores seguidos marca la sesión: %q", c)
	}
	if c := claseDeFila(h, "sesión s-cerrad"); c != "fila sesion fin" {
		t.Errorf("una sesión terminada con eventos en la ventana se muestra atenuada: %q", c)
	}
	if c := claseDeFila(h, ">tester"); c != "fila fin" {
		t.Errorf("un agente que quedó activo en una sesión terminada se atenúa: %q", c)
	}
	if strings.Contains(h, "s-vieja") {
		t.Error("una sesión terminada sin eventos en la ventana no se muestra")
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

func variosProyectos() []Proyecto {
	api := Proyecto{ID: "bbbb0002", Nombre: "api", Raiz: "/x/api", Disponible: true, Nivel: "N3", Modo: "genesis",
		Sesiones: []Sesion{{ID: "s2", Generado: iso(5 * time.Minute), Agentes: []Agente{
			{ID: "principal", Rol: "director", Estado: "activo", Tools: 2, Desde: ms(3 * time.Minute), Visto: ms(time.Minute)},
			{ID: "a1", Rol: "tester", Tarea: "T-4", Padre: "principal", Estado: "activo", Tools: 1, Desde: ms(2 * time.Minute), Visto: ms(time.Minute)},
		}, Eventos: map[string][][2]int64{"a1": {{ms(time.Minute), 0}}}}}}
	web := Proyecto{ID: "cccc0003", Nombre: "web", Raiz: "/x/web", Disponible: true, Nivel: "N1", Modo: "mejora"}
	roto := Proyecto{ID: "dddd0004", Nombre: "roto", Raiz: "/x/roto", Error: "la carpeta no existe"}
	// sin agentes vivos pero con la sesión más reciente que api: va después de api igual
	reciente := Proyecto{ID: "abcd0009", Nombre: "reciente", Raiz: "/x/reciente", Disponible: true, Nivel: "N1", Modo: "mejora",
		Sesiones: []Sesion{{ID: "s3", Generado: iso(30 * time.Second), Terminada: true, Agentes: []Agente{}, Eventos: map[string][][2]int64{}}}}
	return []Proyecto{web, roto, reciente, api, demo()}
}

func TestFragmentoMultiProyecto(t *testing.T) {
	h := fragmento(t, variosProyectos(), "")
	if !strings.Contains(h, "5 proyectos · 2 sesiones activas · 3 agentes vivos · 1 alerta") {
		t.Fatalf("cabecera global: proyectos, sesiones activas, subagentes vivos (L1, r2, a1) y alertas (solo r2 con 3 seguidos; m1 tiene presupuesto):\n%s", h)
	}
	enOrden(t, h, `data-id="aaaa0001"`, `data-id="bbbb0002"`, `data-id="abcd0009"`, `data-id="cccc0003"`)
	if !strings.Contains(h, `data-proyecto="aaaa0001" aria-pressed="false" class="con-alerta"`) || strings.Contains(h, `data-proyecto="bbbb0002" aria-pressed="false" class="con-alerta"`) {
		t.Error("solo el chip de un proyecto con alertas se marca")
	}
	if !strings.Contains(h, "no disponible: la carpeta no existe") {
		t.Error("un proyecto que no se puede leer queda en una línea con el motivo")
	}
	web := h[strings.Index(h, `data-id="cccc0003"`):]
	web = web[:strings.Index(web, "</section>")]
	if !strings.Contains(web, "sin actividad") || strings.Contains(web, " open") || strings.Contains(web, "<svg") {
		t.Errorf("un proyecto sin actividad queda plegado en una línea:\n%s", web)
	}
	dem := h[strings.Index(h, `data-id="aaaa0001"`):]
	dem = dem[:strings.Index(dem, "</section>")]
	if !strings.Contains(dem, " open") {
		t.Errorf("un proyecto con actividad se despliega:\n%s", dem)
	}
	for _, id := range []string{"aaaa0001", "bbbb0002", "cccc0003", "dddd0004"} {
		if !strings.Contains(h, `data-proyecto="`+id+`"`) {
			t.Errorf("chip de filtro para %s", id)
		}
	}
	if !strings.Contains(h, `data-proyecto="" aria-pressed="true"`) {
		t.Error("sin filtro, el chip «todos» queda marcado")
	}
}

func TestAlertaPorMisionSinPresupuesto(t *testing.T) {
	p := Proyecto{ID: "eeee0005", Nombre: "caro", Disponible: true, Nivel: "N3", Modo: "mejora",
		Misiones: []exportar.MisionExportada{{ID: "m9", Gasto: 6, UsdMax: 5, SinPresupuesto: true}}}
	h := fragmento(t, []Proyecto{p}, "")
	if !strings.Contains(h, "1 alerta") || !strings.Contains(h, "sin presupuesto") {
		t.Fatalf("una misión sin presupuesto es una alerta:\n%s", h)
	}
}

func TestFiltroPorProyecto(t *testing.T) {
	h := fragmento(t, variosProyectos(), "bbbb0002")
	if !strings.Contains(h, `data-id="bbbb0002"`) || strings.Contains(h, `data-id="aaaa0001"`) || strings.Contains(h, `data-id="cccc0003"`) {
		t.Fatalf("con filtro solo va el bloque de ese proyecto:\n%s", h)
	}
	if !strings.Contains(h, `data-proyecto="bbbb0002" aria-pressed="true"`) || !strings.Contains(h, "5 proyectos") {
		t.Fatalf("el chip del filtro queda marcado y la cabecera sigue contando todo:\n%s", h)
	}
	if h := fragmento(t, variosProyectos(), "ffff9999"); !strings.Contains(h, `data-id="aaaa0001"`) || !strings.Contains(h, `data-id="cccc0003"`) {
		t.Fatal("un filtro desconocido muestra todos")
	}
}

func TestSinProyectosConectados(t *testing.T) {
	if h := fragmento(t, nil, ""); !strings.Contains(h, "hz dashboard") || !strings.Contains(h, "0 proyectos") {
		t.Fatalf("el estado vacío explica cómo conectar un proyecto:\n%s", h)
	}
}

func escribir(t *testing.T, raiz string, archivos map[string]string) {
	t.Helper()
	for rel, txt := range archivos {
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLeerSesionesActivasInactivasYTerminadas(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, raiz, map[string]string{
		DirSesiones + "/vieja.json":     `{"version":2,"sesion":"vieja","generado":"` + iso(45*time.Minute) + `","terminada":false,"agentes":[{"id":"principal","estado":"activo"},{"id":"z1","padre":"principal","estado":"activo"}]}`,
		DirSesiones + "/cerrada.json":   `{"version":2,"sesion":"cerrada","generado":"` + iso(5*time.Minute) + `","terminada":true,"agentes":[]}`,
		DirSesiones + "/viva.json":      `{"version":2,"sesion":"viva","generado":"` + iso(time.Minute) + `","terminada":false,"agentes":[]}`,
		DirSesiones + "/sin-id.json":    `{"version":2,"generado":"` + iso(2*time.Minute) + `","agentes":[]}`,
		DirSesiones + "/rota.json":      `{"version":2,"ses`,
		DirSesiones + "/notas.txt":      `{"version":2,"sesion":"notas","generado":"` + iso(time.Minute) + `","agentes":[]}`,
		".harness/cache/actividad.json": `{"version":1,"agentes":[{"id":"principal","estado":"activo"}]}`,
	})
	ss, err := LeerSesiones(raiz, ahora)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, s := range ss {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "viva,sin-id,cerrada,vieja" {
		t.Fatalf("las activas primero y luego por la más reciente; el archivo roto, los que no son .json y el actividad.json viejo se omiten; sin id, el nombre del archivo: %v", ids)
	}
	if !ss[0].Viva() || ss[2].Viva() || !ss[2].Terminada || ss[3].Viva() || !ss[3].Inactiva {
		t.Fatalf("activa, terminada e inactiva (>30 min sin escribir): %+v", ss)
	}
	if ss, err := LeerSesiones(t.TempDir(), ahora); err != nil || len(ss) != 0 {
		t.Fatalf("sin carpeta de sesiones no hay sesiones ni error: %v %v", ss, err)
	}
	p := Proyecto{ID: "ffff0006", Nombre: "x", Disponible: true, Nivel: "N1", Modo: "mejora", Sesiones: ss}
	if h := fragmento(t, []Proyecto{p}, ""); !strings.Contains(h, "1 proyecto · 2 sesiones activas · 0 agentes vivos") {
		t.Fatalf("los agentes de una sesión inactiva no cuentan como vivos:\n%s", h)
	}
}

func TestPodarSesionesViejas(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, raiz, map[string]string{DirSesiones + "/vieja.json": "{}", DirSesiones + "/nueva.json": "{}", DirSesiones + "/otra.txt": "x"})
	viejo := ahora.Add(-25 * time.Hour)
	for _, n := range []string{"vieja.json", "otra.txt"} {
		if err := os.Chtimes(filepath.Join(raiz, DirSesiones, n), viejo, viejo); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(filepath.Join(raiz, DirSesiones, "nueva.json"), ahora, ahora); err != nil {
		t.Fatal(err)
	}
	if n := PodarSesiones(raiz, ahora, 24*time.Hour); n != 1 {
		t.Fatalf("se poda solo el snapshot viejo: %d", n)
	}
	for n, debe := range map[string]bool{"vieja.json": false, "nueva.json": true, "otra.txt": true} {
		if _, err := os.Stat(filepath.Join(raiz, DirSesiones, n)); (err == nil) != debe {
			t.Errorf("%s: existe=%v, se esperaba %v", n, err == nil, debe)
		}
	}
	if n := PodarSesiones(t.TempDir(), ahora, 24*time.Hour); n != 0 {
		t.Fatal("sin carpeta no hay nada que podar")
	}
}

const perfilN2 = "version: 1\nenabled: true\nnivel: N2\nmodo: mejora\n"

func TestCentralLeeVariosProyectosYReusaMotores(t *testing.T) {
	hogar := t.TempDir()
	a, b := t.TempDir(), t.TempDir()
	escribir(t, a, map[string]string{".harness/harness.yaml": perfilN2,
		DirSesiones + "/s1.json": `{"version":2,"sesion":"s1","generado":"` + time.Now().Format(time.RFC3339) + `","agentes":[{"id":"principal","rol":"director","estado":"activo","tools":1,"desde":1,"visto":1}],"eventos":{"principal":[[1,0]]}}`})
	escribir(t, b, map[string]string{".harness/harness.yaml": "version: 1\nenabled: true\nnivel: N3\nmodo: genesis\n"})
	apagado := t.TempDir()
	escribir(t, apagado, map[string]string{".harness/harness.yaml": "version: 1\nenabled: false\nnivel: N1\nmodo: mejora\n"})
	sinHarness := t.TempDir()
	borrado := filepath.Join(t.TempDir(), "ya-no-existe")
	for _, r := range []string{a, b, apagado, sinHarness, borrado} {
		if _, err := registro.Conectar(hogar, r, ahora); err != nil {
			t.Fatal(err)
		}
	}
	c := NuevaCentral(hogar)
	defer c.Cerrar()
	ps, err := c.Proyectos(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	por := map[string]Proyecto{}
	for _, p := range ps {
		por[p.Raiz] = p
	}
	if len(ps) != 5 || !por[a].Disponible || por[a].Nivel != "N2" || len(por[a].Sesiones) != 1 || por[b].Modo != "genesis" || por[a].ID != registro.ID(a) {
		t.Fatalf("lee cada proyecto conectado con su nivel, modo y sesiones: %+v", ps)
	}
	for r, motivo := range map[string]string{apagado: "harness desactivado", sinHarness: "sin harness", borrado: "la carpeta no existe"} {
		if por[r].Disponible || por[r].Error != motivo {
			t.Errorf("%s: no disponible con el motivo %q, llegó %+v", r, motivo, por[r])
		}
	}
	if len(c.motores) != 2 {
		t.Fatalf("un motor por proyecto disponible: %d", len(c.motores))
	}
	ma := c.motores[registro.ID(a)]
	if _, err := c.Proyectos(time.Now()); err != nil || c.motores[registro.ID(a)] != ma {
		t.Fatal("el motor se reutiliza entre peticiones (RG-1)")
	}
	if _, err := registro.Desconectar(hogar, b); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Proyectos(time.Now()); err != nil || len(c.motores) != 1 {
		t.Fatalf("al desconectar un proyecto se cierra su motor: %d", len(c.motores))
	}
}

func servidorDePrueba(t *testing.T) (http.Handler, string) {
	t.Helper()
	hogar := t.TempDir()
	raiz := t.TempDir()
	escribir(t, raiz, map[string]string{".harness/harness.yaml": perfilN2,
		DirSesiones + "/s1.json": `{"version":2,"sesion":"s1","generado":"` + time.Now().Format(time.RFC3339) + `","agentes":[{"id":"principal","rol":"director","estado":"activo","tools":1,"ultima":"Bash a && b","desde":1,"visto":1}],"eventos":{"principal":[[1,0]]}}`})
	if _, err := registro.Conectar(hogar, raiz, ahora); err != nil {
		t.Fatal(err)
	}
	c := NuevaCentral(hogar)
	t.Cleanup(c.Cerrar)
	return Servidor(c, 5280, Salud{Version: "0.9.0", Pid: 4242, Binario: "123-456"}), raiz
}

func pedir(srv http.Handler, metodo, host, ruta string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, ruta, nil)
	r.Host = host
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

func TestServidorSoloLocalYDeLectura(t *testing.T) {
	srv, raiz := servidorDePrueba(t)
	if w := pedir(srv, "GET", "evil.example:5280", "/api/panel"); w.Code != http.StatusForbidden {
		t.Fatalf("un Host ajeno (DNS rebinding) se rechaza: %d", w.Code)
	}
	if w := pedir(srv, "GET", "127.0.0.1:9999", "/"); w.Code != http.StatusForbidden {
		t.Fatalf("otro puerto en el Host también se rechaza: %d", w.Code)
	}
	if w := pedir(srv, "POST", "127.0.0.1:5280", "/api/panel"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("solo lectura: %d", w.Code)
	}
	for _, host := range []string{"127.0.0.1:5280", "localhost:5280", "[::1]:5280"} {
		if w := pedir(srv, "GET", host, "/"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<title>") || !strings.Contains(w.Body.String(), "/app.js") {
			t.Fatalf("la página se sirve en %s: %d", host, w.Code)
		}
	}
	if w := pedir(srv, "GET", "127.0.0.1:5280", "/app.js"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/fragmento") || !strings.Contains(w.Body.String(), "proyecto=") {
		t.Fatalf("el script refresca el fragmento con el filtro de proyecto: %d", w.Code)
	}
	if w := pedir(srv, "GET", "127.0.0.1:5280", "/fragmento?ventana=30m"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), filepath.Base(raiz)) || !strings.Contains(w.Body.String(), "sesión s1") {
		t.Fatalf("fragmento: %d %s", w.Code, w.Body.String())
	}
	if w := pedir(srv, "GET", "127.0.0.1:5280", "/fragmento?ventana=3h"); w.Code != http.StatusBadRequest {
		t.Fatalf("solo ventanas de 5m, 10m o 30m: %d", w.Code)
	}
	w := pedir(srv, "GET", "127.0.0.1:5280", "/api/salud")
	var s map[string]any
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &s) != nil || s["servicio"] != "hz-dashboard" || s["pid"] != float64(4242) || s["binario"] != "123-456" || s["version"] != "0.9.0" {
		t.Fatalf("api/salud identifica al servidor: %d %s", w.Code, w.Body.String())
	}
	w = pedir(srv, "GET", "127.0.0.1:5280", "/api/panel")
	var p struct {
		Proyectos []struct {
			ID         string `json:"id"`
			Disponible bool   `json:"disponible"`
			Sesiones   []struct {
				Agentes []map[string]any   `json:"agentes"`
				Eventos map[string][][]int `json:"eventos"`
			} `json:"sesiones"`
		} `json:"proyectos"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &p) != nil || len(p.Proyectos) != 1 || !p.Proyectos[0].Disponible ||
		p.Proyectos[0].ID != registro.ID(raiz) || len(p.Proyectos[0].Sesiones[0].Agentes) != 1 || len(p.Proyectos[0].Sesiones[0].Eventos["principal"]) != 1 {
		t.Fatalf("api/panel: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Bash a && b") {
		t.Fatalf("el JSON conserva el texto tal cual: %s", w.Body.String())
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

func TestSubagenteSinPrincipalCuelgaDeLaSesion(t *testing.T) {
	p := Proyecto{ID: "abab0007", Nombre: "solo-sub", Disponible: true, Nivel: "N1", Modo: "mejora", Sesiones: []Sesion{{ID: "s9", Generado: iso(time.Second),
		Agentes: []Agente{{ID: "q1", Rol: "tester", Padre: "principal", Estado: "activo", Desde: ms(time.Minute), Visto: ms(time.Second)},
			{ID: "q2", Rol: "implementador", Padre: "q1", Estado: "activo", Desde: ms(time.Minute), Visto: ms(time.Second)}}}}}
	h := fragmento(t, []Proyecto{p}, "")
	enOrden(t, h, "sesión s9", `n1" role="cell" title=""><span class="glifo" aria-hidden="true">●</span>tester`, `n2" role="cell" title=""><span class="glifo" aria-hidden="true">●</span>implementador`)
}

func TestFragmentoMarcaFallidosYBarraDePresupuesto(t *testing.T) {
	p := demo()
	s := &p.Sesiones[0]
	s.Agentes = append(s.Agentes, Agente{ID: "r3", Rol: "verificador", Padre: "L1", Estado: "fallido", Desde: ms(time.Minute), Visto: ms(time.Second)})
	s.Eventos["r3"] = [][2]int64{{ms(time.Minute), 2}, {ms(time.Second), 3}}
	p.Misiones = append(p.Misiones, exportar.MisionExportada{ID: "m2", Gasto: 1})
	h := fragmento(t, []Proyecto{p}, "")
	i := strings.Index(h, ">verificador")
	if i < 0 {
		t.Fatalf("falta el verificador:\n%s", h)
	}
	fila := h[strings.LastIndex(h[:i], `<div class="fila`):i]
	if !strings.HasPrefix(fila, `<div class="fila alerta"`) || !strings.Contains(fila, `nombre n2`) || !strings.Contains(fila, "✕") {
		t.Fatalf("un agente fallido se marca como alerta aunque no tenga errores seguidos:\n%s", fila)
	}
	if n := strings.Count(h, `class="barrita"`); n != 1 {
		t.Fatalf("solo la misión con tope lleva barra de presupuesto (m1 sí, m2 no): %d", n)
	}
	if !strings.Contains(h, "2 alertas") {
		t.Fatalf("r2 (3 seguidos) y r3 (fallido en la ventana) son alertas:\n%s", h)
	}
}
