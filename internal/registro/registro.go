// Package registro guarda qué proyectos están conectados al dashboard central (contratos/cli.md · 3.8, D-0053).
// Es un archivo de usuario pequeño: se relee entero y se reescribe de forma atómica solo cuando cambia.
package registro

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Archivo es el nombre del registro dentro del hogar de hz.
const Archivo = "proyectos.json"

// Proyecto es una entrada del registro.
type Proyecto struct {
	Raiz      string `json:"raiz"`
	Conectado string `json:"conectado"`
}

type archivo struct {
	Version   int        `json:"version"`
	Proyectos []Proyecto `json:"proyectos"`
}

// Hogar es la carpeta de usuario de hz: HZ_HOGAR o <config de usuario>/hz.
func Hogar() (string, error) {
	if h := os.Getenv("HZ_HOGAR"); h != "" {
		return h, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("hogar de hz: %w", err)
	}
	return filepath.Join(base, "hz"), nil
}

// clave normaliza una raíz para compararla: limpia y, en Windows, sin mayúsculas.
func clave(raiz string) string {
	c := filepath.Clean(raiz)
	if runtime.GOOS == "windows" {
		c = strings.ToLower(c)
	}
	return c
}

// ID es el identificador estable de un proyecto: 8 hex del sha256 de su raíz normalizada.
func ID(raiz string) string {
	s := sha256.Sum256([]byte(filepath.ToSlash(clave(raiz))))
	return hex.EncodeToString(s[:4])
}

func leer(hogar string) (archivo, error) {
	ruta := filepath.Join(hogar, Archivo)
	b, err := os.ReadFile(ruta)
	if errors.Is(err, iofs.ErrNotExist) {
		return archivo{Version: 1}, nil
	}
	if err != nil {
		return archivo{}, err
	}
	var a archivo
	if err := json.Unmarshal(b, &a); err != nil {
		return archivo{}, fmt.Errorf("%s: %w", ruta, err)
	}
	return a, nil
}

// escribir reemplaza el registro de forma atómica (temporal + rename).
func escribir(hogar string, a archivo) error {
	if err := os.MkdirAll(hogar, 0o755); err != nil {
		return err
	}
	a.Version = 1
	if a.Proyectos == nil {
		a.Proyectos = []Proyecto{}
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	ruta := filepath.Join(hogar, Archivo)
	tmp := ruta + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, ruta); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Lista devuelve los proyectos conectados; sin archivo, ninguno.
func Lista(hogar string) ([]Proyecto, error) {
	a, err := leer(hogar)
	return a.Proyectos, err
}

func indice(ps []Proyecto, raiz string) int {
	k := clave(raiz)
	for i, p := range ps {
		if clave(p.Raiz) == k {
			return i
		}
	}
	return -1
}

// Conectar agrega el proyecto si falta; si ya estaba, no reescribe nada y devuelve false.
func Conectar(hogar, raiz string, ahora time.Time) (bool, error) {
	if !filepath.IsAbs(raiz) {
		return false, fmt.Errorf("la raíz %q debe ser absoluta", raiz)
	}
	a, err := leer(hogar)
	if err != nil {
		return false, err
	}
	if indice(a.Proyectos, raiz) >= 0 {
		return false, nil
	}
	a.Proyectos = append(a.Proyectos, Proyecto{Raiz: filepath.Clean(raiz), Conectado: ahora.Format(time.RFC3339)})
	return true, escribir(hogar, a)
}

// Desconectar saca el proyecto; si no estaba, no es error y devuelve false.
func Desconectar(hogar, raiz string) (bool, error) {
	a, err := leer(hogar)
	if err != nil {
		return false, err
	}
	i := indice(a.Proyectos, raiz)
	if i < 0 {
		return false, nil
	}
	a.Proyectos = append(a.Proyectos[:i], a.Proyectos[i+1:]...)
	return true, escribir(hogar, a)
}

// Nombres da el nombre visible de cada raíz: su carpeta, o padre/carpeta si la carpeta se repite.
func Nombres(ps []Proyecto) map[string]string {
	veces := make(map[string]int, len(ps))
	for _, p := range ps {
		veces[clave(filepath.Base(p.Raiz))]++
	}
	out := make(map[string]string, len(ps))
	for _, p := range ps {
		base := filepath.Base(p.Raiz)
		if veces[clave(base)] > 1 {
			base = filepath.Base(filepath.Dir(p.Raiz)) + "/" + base
		}
		out[p.Raiz] = base
	}
	return out
}
