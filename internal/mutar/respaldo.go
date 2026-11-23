package mutar

// Respaldo por archivo de hz mutar (contratos/cli.md): un candado exclusivo con latido y un respaldo por ruta,
// para que dos mediciones sobre archivos distintos no se pisen y una corrida interrumpida se repare en la siguiente.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"hz528/internal/tdd"
)

const (
	cadaLatido    = 10 * time.Second // el dueño renueva el mtime del candado con este periodo
	latidoVencido = time.Minute      // sin latido por más que esto, el candado es huérfano
)

// ErrEnCurso indica que ya hay una medición viva sobre el mismo archivo (código 1).
var ErrEnCurso = fmt.Errorf("%w: medición en curso", tdd.ErrVerificacion)

// infoCandado es el contenido de <h>.lock: identifica al dueño y al original que respalda.
type infoCandado struct {
	Ruta   string `json:"ruta"`
	PID    int    `json:"pid"`
	Host   string `json:"host"`
	Inicio string `json:"inicio"`
	SHA256 string `json:"sha256"`
}

func dirRespaldos(raiz string) string { return filepath.Join(raiz, ".harness", "cache", "mutar") }

// normalizar devuelve la ruta relativa a la raíz, con / y limpia.
func normalizar(raiz, archivo string) string {
	if filepath.IsAbs(archivo) {
		if r, err := filepath.Rel(raiz, archivo); err == nil {
			archivo = r
		}
	}
	return path.Clean(filepath.ToSlash(archivo))
}

// clave son los primeros 16 hex del sha256 de la ruta normalizada (en Windows, sin distinguir mayúsculas).
func clave(rel string) string {
	if runtime.GOOS == "windows" {
		rel = strings.ToLower(rel)
	}
	h := sha256.Sum256([]byte(rel))
	return hex.EncodeToString(h[:8])
}

// RutaRespaldo devuelve dónde se respalda `rel` durante una medición: .harness/cache/mutar/<h>.bak.
func RutaRespaldo(raiz, rel string) string {
	return filepath.Join(dirRespaldos(raiz), clave(normalizar(raiz, rel))+".bak")
}

func hexSHA(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hora(rfc string) string {
	if t, err := time.Parse(time.RFC3339, rfc); err == nil {
		return t.Local().Format("15:04:05")
	}
	return rfc
}

// respaldo es la medición en curso de un archivo: su candado, su respaldo y el latido.
type respaldo struct {
	ruta, bak, lock string
	original        []byte
	parar           chan struct{}
	latiendo        sync.WaitGroup
}

// tomar crea el candado en exclusiva, lee el original ya con el candado tomado (nadie lo está mutando),
// escribe el respaldo y arranca el latido. Si el candado existe, rechaza sin tocar el archivo.
func tomar(raiz, rel string) (*respaldo, error) {
	dir := dirRespaldos(raiz)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	k := clave(rel)
	r := &respaldo{ruta: filepath.Join(raiz, filepath.FromSlash(rel)), bak: filepath.Join(dir, k+".bak"), lock: filepath.Join(dir, k+".lock")}
	f, err := os.OpenFile(r.lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil, ocupado(raiz, rel, r)
	}
	if err != nil {
		return nil, err
	}
	soltar := func(e error) (*respaldo, error) {
		_ = f.Close()
		_ = os.Remove(r.lock)
		return nil, e
	}
	if r.original, err = os.ReadFile(r.ruta); err != nil {
		return soltar(err)
	}
	host, _ := os.Hostname()
	meta, _ := json.Marshal(infoCandado{Ruta: rel, PID: os.Getpid(), Host: host, Inicio: time.Now().Format(time.RFC3339), SHA256: hexSHA(r.original)})
	if _, err := f.Write(meta); err != nil {
		return soltar(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(r.lock)
		return nil, err
	}
	if err := os.WriteFile(r.bak, r.original, 0o644); err != nil {
		_ = os.Remove(r.bak)
		_ = os.Remove(r.lock)
		return nil, err
	}
	r.parar = make(chan struct{})
	r.latiendo.Add(1)
	go r.latir()
	return r, nil
}

// latir renueva el mtime del candado hasta que la medición termina (es lo que distingue a un dueño vivo).
func (r *respaldo) latir() {
	defer r.latiendo.Done()
	t := time.NewTicker(cadaLatido)
	defer t.Stop()
	for {
		select {
		case <-r.parar:
			return
		case ahora := <-t.C:
			_ = os.Chtimes(r.lock, ahora, ahora)
		}
	}
}

// liberar restaura el original y lo verifica; solo entonces borra el respaldo y, después, el candado.
// Si no puede verificar, conserva ambos: la próxima corrida lo repara.
func (r *respaldo) liberar() error {
	close(r.parar)
	r.latiendo.Wait()
	errW := os.WriteFile(r.ruta, r.original, 0o644)
	actual, errR := os.ReadFile(r.ruta)
	if errW != nil || errR != nil || !bytes.Equal(actual, r.original) {
		causa := "el contenido no coincide"
		if e := errors.Join(errW, errR); e != nil {
			causa = e.Error()
		}
		return fmt.Errorf("no se pudo restaurar %s (%s): el original quedó en %s", r.ruta, causa, r.bak)
	}
	limpiar(r.bak, r.lock)
	return nil
}

// limpiar borra el respaldo y, solo si ya no existe, el candado: nunca queda un respaldo sin su candado.
func limpiar(bak, lock string) {
	if err := os.Remove(bak); err == nil || errors.Is(err, fs.ErrNotExist) {
		_ = os.Remove(lock)
	}
}

func leerCandado(lock string) (infoCandado, time.Time, error) {
	var c infoCandado
	fi, err := os.Stat(lock)
	if err != nil {
		return c, time.Time{}, err
	}
	b, err := os.ReadFile(lock)
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	return c, fi.ModTime(), err
}

// huerfano: el latido venció, o el dueño era de este host y su proceso ya no existe.
func huerfano(c infoCandado, latido time.Time, host string) bool {
	if time.Since(latido) > latidoVencido {
		return true
	}
	return c.Host == host && c.PID > 0 && !procesoVivo(c.PID)
}

// ocupado explica por qué no se pudo tomar el candado de rel.
func ocupado(raiz, rel string, r *respaldo) error {
	c, latido, err := leerCandado(r.lock)
	host, _ := os.Hostname()
	if err == nil && huerfano(c, latido, host) {
		return fmt.Errorf("%w: %s tiene un respaldo huérfano de una corrida interrumpida de hz mutar que no se restauró porque el archivo cambió; compáralo con %s y borra ese archivo y su .lock antes de medirlo",
			tdd.ErrVerificacion, rel, relativa(raiz, r.bak))
	}
	return fmt.Errorf("%w: sobre %s ya hay otra medición viva (pid %d, desde %s); espera a que termine o mide otro archivo. Si esa corrida murió, la próxima hz mutar restaura el original sola (cuando su proceso ya no exista o tras %s sin latido)",
		ErrEnCurso, rel, c.PID, hora(c.Inicio), latidoVencido)
}

func relativa(raiz, ruta string) string {
	if r, err := filepath.Rel(raiz, ruta); err == nil {
		return filepath.ToSlash(r)
	}
	return ruta
}

// recuperarHuerfanos revisa todos los candados y repara los de corridas interrumpidas; devuelve los avisos.
func recuperarHuerfanos(raiz string) []string {
	var avisos []string
	if _, err := os.Stat(filepath.Join(raiz, ".harness", "cache", "mutar.bak")); err == nil {
		avisos = append(avisos, "queda .harness/cache/mutar.bak de una versión anterior de hz mutar y no dice de qué archivo es: compáralo a mano (git diff) y bórralo")
	}
	locks, _ := filepath.Glob(filepath.Join(dirRespaldos(raiz), "*.lock"))
	if len(locks) == 0 {
		return avisos
	}
	host, _ := os.Hostname()
	for _, lock := range locks {
		if aviso := recuperar(raiz, lock, host); aviso != "" {
			avisos = append(avisos, aviso)
		}
	}
	return avisos
}

// recuperar repara un candado huérfano solo si es seguro: el archivo está igual al original (limpia) o es un
// mutante de él (restaura). Si cambió de otra forma, no lo toca y conserva el respaldo para el humano.
func recuperar(raiz, lock, host string) string {
	c, latido, err := leerCandado(lock)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || time.Since(latido) <= latidoVencido {
			return "" // ya se liberó, o su dueño lo está escribiendo
		}
		return fmt.Sprintf("candado ilegible %s (%v): revísalo y bórralo junto a su .bak", relativa(raiz, lock), err)
	}
	if !huerfano(c, latido, host) {
		return ""
	}
	bak := strings.TrimSuffix(lock, ".lock") + ".bak"
	ruta := filepath.Join(raiz, filepath.FromSlash(c.Ruta))
	actual, errA := os.ReadFile(ruta)
	original, errB := os.ReadFile(bak)
	if c.SHA256 == "" && errB == nil {
		c.SHA256 = hexSHA(original)
	}
	if errA == nil && hexSHA(actual) == c.SHA256 {
		limpiar(bak, lock) // el archivo ya está como el original
		return ""
	}
	if errB != nil || hexSHA(original) != c.SHA256 {
		// sin respaldo completo la corrida murió antes del primer mutante: no hay nada que restaurar
		limpiar(bak, lock)
		return ""
	}
	desde := fmt.Sprintf("pid %d, desde %s", c.PID, hora(c.Inicio))
	if errA == nil && esMutante(original, actual, strings.ToLower(path.Ext(c.Ruta))) {
		if err := os.WriteFile(ruta, original, 0o644); err != nil {
			return fmt.Sprintf("no se pudo restaurar %s desde %s (%v): restáuralo a mano", c.Ruta, relativa(raiz, bak), err)
		}
		if b, err := os.ReadFile(ruta); err != nil || !bytes.Equal(b, original) {
			return fmt.Sprintf("no se pudo verificar la restauración de %s: el original sigue en %s", c.Ruta, relativa(raiz, bak))
		}
		limpiar(bak, lock)
		return fmt.Sprintf("restaurado %s: quedó un mutante de una corrida de hz mutar interrumpida (%s)", c.Ruta, desde)
	}
	return fmt.Sprintf("%s cambió desde una corrida de hz mutar interrumpida (%s) y no es un mutante: no se restauró. El original está en %s; compáralo y borra ese archivo y su .lock para volver a medirlo",
		c.Ruta, desde, relativa(raiz, bak))
}

// esMutante indica si `actual` es `original` con exactamente un sitio mutado (lo único que escribe hz mutar).
func esMutante(original, actual []byte, ext string) bool {
	p := 0 // primer byte distinto: el sitio mutado tiene que empezar en o antes de p
	for p < len(original) && p < len(actual) && original[p] == actual[p] {
		p++
	}
	if p == len(original) && p == len(actual) {
		return false
	}
	for _, s := range Sitios(string(original), ext) {
		if s.Offset > p {
			break // los sitios vienen en orden de aparición
		}
		fin, finMut := s.Offset+len(s.Original), s.Offset+len(s.Mutante)
		if len(actual)-finMut == len(original)-fin && string(actual[s.Offset:finMut]) == s.Mutante && bytes.Equal(actual[finMut:], original[fin:]) {
			return true
		}
	}
	return false
}
