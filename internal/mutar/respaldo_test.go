package mutar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hz528/internal/tdd"
)

// Dos archivos baratos de medir (sin compilar): el comando falla si cualquiera de los dos pierde su línea original.
const (
	fuenteCalc = "func Sumar(a, b int) int { return a + b }\n"
	fuenteOtro = "func Listo() bool { return true }\n"
	cmdAmbos   = `grep -q "a + b" calc.go && grep -q "return true" otro.go`
	pidMuerto  = 2147483600 // fuera de rango en Linux y sin proceso en Windows
)

func proyectoMutable(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	for rel, txt := range map[string]string{"calc.go": fuenteCalc, "otro.go": fuenteOtro} {
		if err := os.WriteFile(filepath.Join(raiz, rel), []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return raiz
}

func rutaCandado(raiz, rel string) string {
	return strings.TrimSuffix(RutaRespaldo(raiz, rel), ".bak") + ".lock"
}

// candado fabrica el estado que deja una medición en curso (o interrumpida) sobre rel, según contratos/cli.md.
func candado(t *testing.T, raiz, rel, original string, pid int, latido time.Time) {
	t.Helper()
	host, _ := os.Hostname()
	h := sha256.Sum256([]byte(original))
	b, _ := json.Marshal(map[string]any{"ruta": rel, "pid": pid, "host": host, "inicio": latido.Format(time.RFC3339), "sha256": hex.EncodeToString(h[:])})
	lock := rutaCandado(raiz, rel)
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RutaRespaldo(raiz, rel), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(lock, latido, latido); err != nil {
		t.Fatal(err)
	}
}

func leer(t *testing.T, raiz, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(raiz, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func existe(ruta string) bool { _, err := os.Stat(ruta); return err == nil }

func TestRespaldoPorArchivo(t *testing.T) {
	raiz := t.TempDir()
	a, b := RutaRespaldo(raiz, "app/x.go"), RutaRespaldo(raiz, "app/y.go")
	dir := filepath.Join(raiz, ".harness", "cache", "mutar")
	if a == b || filepath.Dir(a) != dir || filepath.Dir(b) != dir || filepath.Ext(a) != ".bak" {
		t.Fatalf("cada archivo tiene su respaldo en .harness/cache/mutar/<hash>.bak: %q %q", a, b)
	}
	if RutaRespaldo(raiz, "./app/../app/x.go") != a {
		t.Fatal("la ruta se normaliza antes de calcular el hash")
	}
}

func TestSegundaMedicionDelMismoArchivoSeRechazaYOtroArchivoSigue(t *testing.T) {
	raiz := proyectoMutable(t)
	candado(t, raiz, "calc.go", fuenteCalc, os.Getpid(), time.Now())
	_, err := Ejecutar(raiz, "calc.go", cmdAmbos, 10, time.Minute)
	if !errors.Is(err, ErrEnCurso) || !errors.Is(err, tdd.ErrVerificacion) || !strings.Contains(err.Error(), "calc.go") {
		t.Fatalf("con una medición viva sobre calc.go, otra sobre el mismo archivo se rechaza nombrándolo: %v", err)
	}
	if leer(t, raiz, "calc.go") != fuenteCalc {
		t.Fatal("el rechazo no toca el archivo")
	}
	r, err := Ejecutar(raiz, "otro.go", cmdAmbos, 10, time.Minute)
	if err != nil || r.Total != 1 || r.Detectados != 1 {
		t.Fatalf("medir otro archivo a la vez no se bloquea: %+v %v", r, err)
	}
	if leer(t, raiz, "otro.go") != fuenteOtro {
		t.Fatal("otro.go queda restaurado")
	}
	if !existe(rutaCandado(raiz, "calc.go")) || !existe(RutaRespaldo(raiz, "calc.go")) {
		t.Fatal("la medición de otro.go no pisa el respaldo ni el candado vivos de calc.go")
	}
	if existe(rutaCandado(raiz, "otro.go")) || existe(RutaRespaldo(raiz, "otro.go")) {
		t.Fatal("al terminar, la medición borra su respaldo y su candado")
	}
	if len(r.Avisos) != 0 {
		t.Fatalf("un candado vivo no es un huérfano: %q", r.Avisos)
	}
}

func TestHuerfanoConMutanteSeRestauraYAvisa(t *testing.T) {
	for nombre, armar := range map[string]func(t *testing.T, raiz string){
		"proceso muerto": func(t *testing.T, raiz string) { candado(t, raiz, "calc.go", fuenteCalc, pidMuerto, time.Now()) },
		"latido vencido": func(t *testing.T, raiz string) {
			candado(t, raiz, "calc.go", fuenteCalc, os.Getpid(), time.Now().Add(-2*time.Minute))
		},
	} {
		t.Run(nombre, func(t *testing.T) {
			raiz := proyectoMutable(t)
			armar(t, raiz)
			mutante := Aplicar(fuenteCalc, Sitios(fuenteCalc, ".go")[0])
			if err := os.WriteFile(filepath.Join(raiz, "calc.go"), []byte(mutante), 0o644); err != nil {
				t.Fatal(err)
			}
			r, err := Ejecutar(raiz, "otro.go", `grep -q "return true" otro.go`, 10, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if leer(t, raiz, "calc.go") != fuenteCalc {
				t.Fatal("el mutante que dejó la corrida interrumpida se restaura al original")
			}
			if len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], "restaurado calc.go") {
				t.Fatalf("la restauración se avisa nombrando el archivo: %q", r.Avisos)
			}
			if existe(rutaCandado(raiz, "calc.go")) || existe(RutaRespaldo(raiz, "calc.go")) {
				t.Fatal("tras restaurar, el respaldo y el candado huérfanos se borran")
			}
		})
	}
}

func TestHuerfanoQueCambioNoSeTocaYBloqueaEseArchivo(t *testing.T) {
	raiz := proyectoMutable(t)
	candado(t, raiz, "calc.go", fuenteCalc, pidMuerto, time.Now())
	editado := "func Sumar(a, b int) int { return b + a } // editado a mano\n"
	if err := os.WriteFile(filepath.Join(raiz, "calc.go"), []byte(editado), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Ejecutar(raiz, "otro.go", `grep -q "return true" otro.go`, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if leer(t, raiz, "calc.go") != editado {
		t.Fatal("un archivo que cambió y no es un mutante no se sobrescribe")
	}
	bak := filepath.ToSlash(RutaRespaldo(raiz, "calc.go"))
	if len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], "calc.go") || !strings.Contains(r.Avisos[0], filepath.Base(bak)) {
		t.Fatalf("se avisa con el archivo y dónde quedó el original: %q", r.Avisos)
	}
	if !existe(rutaCandado(raiz, "calc.go")) || !existe(RutaRespaldo(raiz, "calc.go")) {
		t.Fatal("el respaldo y el candado se conservan para que el humano decida")
	}
	if _, err := Ejecutar(raiz, "calc.go", cmdAmbos, 10, time.Minute); !errors.Is(err, tdd.ErrVerificacion) || !strings.Contains(err.Error(), "calc.go") {
		t.Fatalf("medir ese archivo queda rechazado hasta resolver el huérfano: %v", err)
	}
	if leer(t, raiz, "calc.go") != editado {
		t.Fatal("el rechazo tampoco toca el archivo")
	}
}

func TestRespaldoAntiguoSeAvisa(t *testing.T) {
	raiz := proyectoMutable(t)
	viejo := filepath.Join(raiz, ".harness", "cache", "mutar.bak")
	if err := os.MkdirAll(filepath.Dir(viejo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(viejo, []byte(fuenteCalc), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Ejecutar(raiz, "otro.go", `grep -q "return true" otro.go`, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], "mutar.bak") || !existe(viejo) {
		t.Fatalf("el respaldo único de versiones previas se avisa y no se borra: %q", r.Avisos)
	}
}

// reclamoDe es el archivo con que un proceso reclama la reparación de un huérfano (contratos/cli.md, D-0055).
func reclamoDe(raiz, rel string) string {
	return strings.TrimSuffix(RutaRespaldo(raiz, rel), ".bak") + ".recuperando"
}

func escribir(t *testing.T, ruta, txt string) {
	t.Helper()
	if err := os.WriteFile(ruta, []byte(txt), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHuerfanoEnReparacionNoSeToca(t *testing.T) {
	raiz := proyectoMutable(t)
	candado(t, raiz, "calc.go", fuenteCalc, pidMuerto, time.Now())
	mutante := Aplicar(fuenteCalc, Sitios(fuenteCalc, ".go")[0])
	escribir(t, filepath.Join(raiz, "calc.go"), mutante)
	reclamo := reclamoDe(raiz, "calc.go")
	escribir(t, reclamo, "")
	r, err := Ejecutar(raiz, "otro.go", `grep -q "return true" otro.go`, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if leer(t, raiz, "calc.go") != mutante || !existe(rutaCandado(raiz, "calc.go")) || !existe(RutaRespaldo(raiz, "calc.go")) || !existe(reclamo) {
		t.Fatal("un huérfano que otro proceso está reparando (reclamo vigente) no se toca")
	}
	if len(r.Avisos) != 0 {
		t.Fatalf("un reclamo vigente no se avisa: %q", r.Avisos)
	}
	if _, err := Ejecutar(raiz, "calc.go", cmdAmbos, 10, time.Minute); !errors.Is(err, ErrEnCurso) || !strings.Contains(err.Error(), "calc.go") || !strings.Contains(err.Error(), "reintenta") {
		t.Fatalf("medir el archivo mientras se repara se rechaza con código 1 y pide reintentar: %v", err)
	}
	viejo := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(reclamo, viejo, viejo); err != nil {
		t.Fatal(err)
	}
	r, err = Ejecutar(raiz, "otro.go", `grep -q "return true" otro.go`, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], filepath.Base(reclamo)) || !strings.Contains(r.Avisos[0], "calc.go") {
		t.Fatalf("un reclamo vencido se avisa nombrándolo junto al archivo: %q", r.Avisos)
	}
	if leer(t, raiz, "calc.go") != mutante || !existe(reclamo) {
		t.Fatal("un reclamo vencido no se borra solo ni se repara encima")
	}
}

func TestReparacionConCandadoVistoAntesNoPisaUnaMedicionNueva(t *testing.T) {
	raiz := proyectoMutable(t)
	lock := rutaCandado(raiz, "calc.go")
	candado(t, raiz, "calc.go", fuenteCalc, pidMuerto, time.Now())
	visto, _, err := leerCandado(lock) // lo que leyó un proceso lento antes de que otro reparara el huérfano
	if err != nil {
		t.Fatal(err)
	}
	// Otro proceso ya reparó el huérfano, y una medición nueva y viva tomó calc.go y escribió su mutante.
	candado(t, raiz, "calc.go", fuenteCalc, os.Getpid(), time.Now())
	mutante := Aplicar(fuenteCalc, Sitios(fuenteCalc, ".go")[0])
	escribir(t, filepath.Join(raiz, "calc.go"), mutante)
	if aviso := reparar(raiz, lock, visto); aviso != "" {
		t.Fatalf("con un candado que ya no es el visto, no hay nada que avisar: %q", aviso)
	}
	if leer(t, raiz, "calc.go") != mutante {
		t.Fatal("el proceso lento no restaura encima del mutante de la medición viva")
	}
	if !existe(lock) || !existe(RutaRespaldo(raiz, "calc.go")) {
		t.Fatal("el proceso lento no borra el candado ni el respaldo de la medición viva")
	}
	if existe(reclamoDe(raiz, "calc.go")) {
		t.Fatal("el reclamo se suelta al terminar")
	}
	// Con el candado que sí es el huérfano visto, la reparación procede y no deja el reclamo.
	candado(t, raiz, "calc.go", fuenteCalc, pidMuerto, time.Now())
	visto, _, err = leerCandado(lock)
	if err != nil {
		t.Fatal(err)
	}
	if aviso := reparar(raiz, lock, visto); !strings.Contains(aviso, "restaurado calc.go") {
		t.Fatalf("el huérfano visto se restaura y se avisa: %q", aviso)
	}
	if leer(t, raiz, "calc.go") != fuenteCalc || existe(lock) || existe(RutaRespaldo(raiz, "calc.go")) || existe(reclamoDe(raiz, "calc.go")) {
		t.Fatal("tras reparar quedan el original y ningún candado, respaldo ni reclamo")
	}
}

func TestCandadoIlegibleSeExplicaSinPidInventado(t *testing.T) {
	raiz := proyectoMutable(t)
	lock := rutaCandado(raiz, "calc.go")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	escribir(t, lock, "") // recién creado en exclusiva por otra medición, todavía sin contenido
	_, err := Ejecutar(raiz, "calc.go", cmdAmbos, 10, time.Minute)
	if !errors.Is(err, ErrEnCurso) || !strings.Contains(err.Error(), "calc.go") || !strings.Contains(err.Error(), "reintenta") || strings.Contains(err.Error(), "pid 0") {
		t.Fatalf("un candado que se está escribiendo pide reintentar, sin pid inventado: %v", err)
	}
	escribir(t, lock, "{roto")
	viejo := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(lock, viejo, viejo); err != nil {
		t.Fatal(err)
	}
	_, err = Ejecutar(raiz, "calc.go", cmdAmbos, 10, time.Minute)
	if !errors.Is(err, tdd.ErrVerificacion) || errors.Is(err, ErrEnCurso) || !strings.Contains(err.Error(), filepath.Base(lock)) {
		t.Fatalf("un candado ilegible y vencido se nombra para que el humano lo borre: %v", err)
	}
	if leer(t, raiz, "calc.go") != fuenteCalc {
		t.Fatal("los rechazos no tocan el archivo")
	}
}
