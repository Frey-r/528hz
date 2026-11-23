// Package mutar mide la capacidad de detección de una suite con mutation testing textual
// (meta-eval de verificadores: evals también para lo determinista, docs/diseno/06-tdd-y-evals.md §3.2).
package mutar

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"hz528/internal/tdd"
)

// Sitio es un punto mutable del código fuente.
type Sitio struct {
	Linea    int    `json:"linea"`
	Col      int    `json:"col"`
	Offset   int    `json:"-"`
	Original string `json:"original"`
	Mutante  string `json:"mutante"`
}

// Resultado de una corrida de mutation testing.
type Resultado struct {
	Total          int      `json:"total"`
	Detectados     int      `json:"detectados"`
	Sobrevivientes []Sitio  `json:"sobrevivientes"`
	Tasa           float64  `json:"tasa"`
	Avisos         []string `json:"avisos,omitempty"` // huérfanos reparados o pendientes (respaldo.go)
}

// Operadores binarios rodeados de espacios: evita genéricos, flechas y operadores unarios.
var operadores = [][2]string{
	{" == ", " != "}, {" != ", " == "}, {" <= ", " > "}, {" >= ", " < "},
	{" && ", " || "}, {" || ", " && "}, {" + ", " - "}, {" - ", " + "}, {" < ", " >= "}, {" > ", " <= "},
}

var palabras = map[string]map[string]string{
	".py":  {"True": "False", "False": "True", "and": "or", "or": "and"},
	"otro": {"true": "false", "false": "true"},
}

func esPalabra(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// Sitios recorre el fuente saltando strings y comentarios y devuelve los puntos mutables.
func Sitios(src, ext string) []Sitio {
	py := ext == ".py"
	reemplazos := palabras["otro"]
	if py {
		reemplazos = palabras[".py"]
	}
	var out []Sitio
	linea, col := 1, 1
	var comilla byte
	comentarioLinea, comentarioBloque := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		avanzar := func() {
			if c == '\n' {
				linea, col = linea+1, 1
			} else {
				col++
			}
		}
		switch {
		case comentarioLinea:
			if c == '\n' {
				comentarioLinea = false
			}
		case comentarioBloque:
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				comentarioBloque = false
				i++
				col++
			}
		case comilla != 0:
			if c == '\\' && comilla != '`' {
				i++
				col++
			} else if c == comilla {
				comilla = 0
			}
		case c == '"' || c == '\'' || (c == '`' && !py):
			comilla = c
		case py && c == '#', !py && c == '/' && i+1 < len(src) && src[i+1] == '/':
			comentarioLinea = true
		case !py && c == '/' && i+1 < len(src) && src[i+1] == '*':
			comentarioBloque = true
		default:
			if s, ok := sitioEn(src, i, reemplazos); ok {
				s.Linea, s.Col = linea, col
				out = append(out, s)
			}
		}
		avanzar()
	}
	return out
}

func sitioEn(src string, i int, reemplazos map[string]string) (Sitio, bool) {
	for _, op := range operadores {
		if strings.HasPrefix(src[i:], op[0]) {
			return Sitio{Offset: i, Original: op[0], Mutante: op[1]}, true
		}
	}
	if i > 0 && esPalabra(rune(src[i-1])) {
		return Sitio{}, false
	}
	for original, mutante := range reemplazos {
		fin := i + len(original)
		if strings.HasPrefix(src[i:], original) && (fin == len(src) || !esPalabra(rune(src[fin]))) {
			return Sitio{Offset: i, Original: original, Mutante: mutante}, true
		}
	}
	return Sitio{}, false
}

// Aplicar devuelve el fuente con ese único sitio mutado.
func Aplicar(src string, s Sitio) string {
	return src[:s.Offset] + s.Mutante + src[s.Offset+len(s.Original):]
}

// Ejecutar aplica cada mutante (uno a la vez), corre la suite y restaura siempre el original.
// Antes repara los huérfanos de corridas interrumpidas (r.Avisos llega aunque haya error) y toma el candado
// del archivo: dos mediciones sobre archivos distintos no se pisan; sobre el mismo, la segunda se rechaza.
func Ejecutar(raiz, archivo, cmd string, max int, limite time.Duration) (r Resultado, err error) {
	if strings.TrimSpace(cmd) == "" {
		return r, fmt.Errorf("%w: sin comando de test (usa --cmd o comandos.test)", tdd.ErrVerificacion)
	}
	rel := normalizar(raiz, archivo)
	r.Avisos = recuperarHuerfanos(raiz)
	resp, err := tomar(raiz, rel)
	if err != nil {
		return r, err
	}
	defer func() {
		if errL := resp.liberar(); errL != nil { // no restaurar es más grave que cualquier otro fallo (código 4)
			if err != nil {
				errL = fmt.Errorf("%w (además: %v)", errL, err)
			}
			err = errL
		}
	}()
	ruta, original := resp.ruta, resp.original
	base, err := tdd.EjecutarConLimite(raiz, cmd, limite)
	if err != nil {
		return r, err
	}
	if base.Codigo != 0 {
		return r, fmt.Errorf("%w: la suite falla sin mutantes; mutation testing necesita una línea base en verde", tdd.ErrVerificacion)
	}
	sitios := Sitios(string(original), strings.ToLower(path.Ext(rel)))
	sitios = Muestra(sitios, max)
	for _, s := range sitios {
		if err := os.WriteFile(ruta, []byte(Aplicar(string(original), s)), 0o644); err != nil {
			return r, err
		}
		res, err := tdd.EjecutarConLimite(raiz, cmd, limite)
		if err != nil && !errors.Is(err, tdd.ErrLimite) {
			return r, err
		}
		r.Total++
		if errors.Is(err, tdd.ErrLimite) || res.Codigo != 0 {
			r.Detectados++
		} else {
			r.Sobrevivientes = append(r.Sobrevivientes, s)
		}
	}
	r.Tasa = 1 // sin sitios mutables no hay fallas que la suite deba detectar
	if r.Total > 0 {
		r.Tasa = float64(r.Detectados) / float64(r.Total)
	}
	return r, nil
}

// Muestra elige hasta max sitios con paso uniforme, del primero al último: así la meta-eval cubre
// todo el archivo y no solo su comienzo. max ≤ 0 significa sin tope.
func Muestra(sitios []Sitio, max int) []Sitio {
	n := len(sitios)
	if max <= 0 || n <= max {
		return sitios
	}
	if max == 1 {
		return sitios[:1]
	}
	out := make([]Sitio, max)
	for i := range out {
		out[i] = sitios[i*(n-1)/(max-1)]
	}
	return out
}

// extensionesDeCodigo son los lenguajes cuyos archivos exige medir el verificador.
// (Listas como datos: así la meta-eval no siembra mutantes en una tabla.)
var (
	extensionesDeCodigo = conjunto(".go .ts .tsx .js .jsx .mjs .cjs .py .rb .java .kt .kts .cs .rs .c .h .cc .cpp .hpp .swift .php .scala .lua .dart .vue .svelte")
	carpetasDeTests     = conjunto("test tests __tests__ spec specs")
)

func conjunto(lista string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, x := range strings.Fields(lista) {
		out[x] = struct{}{}
	}
	return out
}

// EsCodigoDeProduccion indica si una ruta es código fuente que no es test (por convención de nombre o carpeta).
func EsCodigoDeProduccion(ruta string) bool {
	ruta = strings.ToLower(filepath.ToSlash(ruta))
	ext := path.Ext(ruta)
	if _, esCodigo := extensionesDeCodigo[ext]; !esCodigo {
		return false
	}
	base := strings.TrimSuffix(path.Base(ruta), ext)
	if strings.HasSuffix(base, "_test") || strings.HasSuffix(base, "_spec") || strings.HasSuffix(base, ".test") ||
		strings.HasSuffix(base, ".spec") || strings.HasPrefix(base, "test_") {
		return false
	}
	for _, seg := range strings.Split(path.Dir(ruta), "/") {
		if _, esTest := carpetasDeTests[seg]; esTest {
			return false
		}
	}
	return true
}
