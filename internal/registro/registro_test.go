package registro

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var ahora = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func TestHogarRespetaHZ_HOGAR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HZ_HOGAR", dir)
	h, err := Hogar()
	if err != nil || h != dir {
		t.Fatalf("con HZ_HOGAR el hogar es esa carpeta: %q %v", h, err)
	}
	t.Setenv("HZ_HOGAR", "")
	h, err = Hogar()
	if err != nil || filepath.Base(h) != "hz" {
		t.Fatalf("sin HZ_HOGAR el hogar es <config de usuario>/hz: %q %v", h, err)
	}
}

func TestListaSinArchivoEsVacia(t *testing.T) {
	ps, err := Lista(t.TempDir())
	if err != nil || len(ps) != 0 {
		t.Fatalf("sin proyectos.json el registro está vacío: %v %v", ps, err)
	}
}

func TestConectarEsIdempotenteYNoReescribe(t *testing.T) {
	hogar := t.TempDir()
	raiz := t.TempDir()
	nuevo, err := Conectar(hogar, raiz, ahora)
	if err != nil || !nuevo {
		t.Fatalf("la primera vez se conecta: %v %v", nuevo, err)
	}
	archivo := filepath.Join(hogar, Archivo)
	antes, err := os.Stat(archivo)
	if err != nil {
		t.Fatal(err)
	}
	// otra forma de la misma ruta: con separador final y un tramo «.»
	nuevo, err = Conectar(hogar, raiz+string(filepath.Separator)+"."+string(filepath.Separator), ahora.Add(time.Hour))
	if err != nil || nuevo {
		t.Fatalf("la misma raíz no se conecta dos veces: %v %v", nuevo, err)
	}
	despues, err := os.Stat(archivo)
	if err != nil {
		t.Fatal(err)
	}
	if !despues.ModTime().Equal(antes.ModTime()) || despues.Size() != antes.Size() {
		t.Fatal("si el proyecto ya estaba, el archivo no se reescribe")
	}
	ps, err := Lista(hogar)
	if err != nil || len(ps) != 1 || ps[0].Raiz != filepath.Clean(raiz) || ps[0].Conectado != ahora.Format(time.RFC3339) {
		t.Fatalf("un proyecto con su raíz limpia y cuándo se conectó: %+v %v", ps, err)
	}
	if _, err := os.Stat(archivo + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("la escritura atómica no deja el temporal")
	}
}

func TestConectarExigeRutaAbsoluta(t *testing.T) {
	if _, err := Conectar(t.TempDir(), "relativa/proyecto", ahora); err == nil {
		t.Fatal("una raíz relativa no se registra")
	}
}

func TestEnWindowsLaRaizNoDistingueMayusculas(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("solo Windows compara sin mayúsculas")
	}
	hogar := t.TempDir()
	raiz := t.TempDir()
	if _, err := Conectar(hogar, raiz, ahora); err != nil {
		t.Fatal(err)
	}
	if nuevo, err := Conectar(hogar, strings.ToUpper(raiz), ahora); err != nil || nuevo {
		t.Fatalf("C:\\X y c:\\x son el mismo proyecto en Windows: %v %v", nuevo, err)
	}
	if ID(raiz) != ID(strings.ToUpper(raiz)) {
		t.Fatal("el id tampoco depende de mayúsculas en Windows")
	}
}

func TestDesconectar(t *testing.T) {
	hogar := t.TempDir()
	a, b := t.TempDir(), t.TempDir()
	for _, r := range []string{a, b} {
		if _, err := Conectar(hogar, r, ahora); err != nil {
			t.Fatal(err)
		}
	}
	estaba, err := Desconectar(hogar, a)
	if err != nil || !estaba {
		t.Fatalf("desconectar un proyecto conectado: %v %v", estaba, err)
	}
	estaba, err = Desconectar(hogar, a)
	if err != nil || estaba {
		t.Fatalf("desconectar dos veces no es error, pero informa que no estaba: %v %v", estaba, err)
	}
	ps, _ := Lista(hogar)
	if len(ps) != 1 || ps[0].Raiz != filepath.Clean(b) {
		t.Fatalf("queda solo el otro: %+v", ps)
	}
}

func TestRegistroIlegibleEsError(t *testing.T) {
	hogar := t.TempDir()
	if err := os.WriteFile(filepath.Join(hogar, Archivo), []byte("{roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Lista(hogar); err == nil || !strings.Contains(err.Error(), Archivo) {
		t.Fatalf("un registro corrupto se informa nombrando el archivo: %v", err)
	}
	if nuevo, err := Conectar(hogar, t.TempDir(), ahora); err == nil || nuevo {
		t.Fatalf("no se pisa un registro corrupto: se informa y no se da por conectado: %v %v", nuevo, err)
	}
	if estaba, err := Desconectar(hogar, t.TempDir()); err == nil || estaba {
		t.Fatalf("desconectar sobre un registro corrupto se informa y no se da por hecho: %v %v", estaba, err)
	}
}

func TestEscrituraFallidaInformaYNoDejaTemporal(t *testing.T) {
	hogar := t.TempDir()
	// proyectos.json es una carpeta con contenido: el rename del temporal tiene que fallar
	if err := os.MkdirAll(filepath.Join(hogar, Archivo, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := escribir(hogar, archivo{}); err == nil {
		t.Fatal("si no se puede reemplazar el registro, se informa")
	}
	if _, err := os.Stat(filepath.Join(hogar, Archivo+".tmp")); !os.IsNotExist(err) {
		t.Fatal("el temporal no queda tirado tras un rename fallido")
	}
}

func TestIDEstableYCorto(t *testing.T) {
	raiz := t.TempDir()
	id := ID(raiz)
	if len(id) != 8 || strings.Trim(id, "0123456789abcdef") != "" {
		t.Fatalf("8 hex: %q", id)
	}
	if ID(raiz+string(filepath.Separator)) != id {
		t.Fatal("el id no cambia con un separador final")
	}
	if ID(t.TempDir()) == id {
		t.Fatal("otra raíz, otro id")
	}
}

func TestNombresDesambiguaCarpetasIguales(t *testing.T) {
	sep := string(filepath.Separator)
	base := t.TempDir()
	ps := []Proyecto{
		{Raiz: base + sep + "cliente-a" + sep + "api"},
		{Raiz: base + sep + "cliente-b" + sep + "api"},
		{Raiz: base + sep + "web"},
	}
	n := Nombres(ps)
	if n[ps[0].Raiz] != "cliente-a/api" || n[ps[1].Raiz] != "cliente-b/api" || n[ps[2].Raiz] != "web" {
		t.Fatalf("la carpeta, o padre/carpeta si se repite: %v", n)
	}
}
