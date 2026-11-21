// Package proyecto localiza, carga, valida y activa el perfil del harness en un repo.
package proyecto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrSinProyecto indica que ningún ancestro del directorio tiene .harness/harness.yaml.
var ErrSinProyecto = errors.New("sin proyecto activado (falta .harness/harness.yaml)")

// Limites de tamaño que verifica `hz check` (docs/diseno/04-estructura.md).
type Limites struct {
	LineasMaxAgentsRaiz    int `yaml:"lineas_max_agents_raiz"`
	LineasMaxAgentsDominio int `yaml:"lineas_max_agents_dominio"`
	LineasMaxArchivo       int `yaml:"lineas_max_archivo"`
}

// Perfil es el subconjunto de .harness/harness.yaml que usa el CLI.
type Perfil struct {
	Version  int               `yaml:"version"`
	Enabled  bool              `yaml:"enabled"`
	Nivel    string            `yaml:"nivel"`
	Modo     string            `yaml:"modo"`
	Idioma   string            `yaml:"idioma"`
	Comandos map[string]string `yaml:"comandos"`
	Limites  Limites           `yaml:"limites"`
	// Verificacion: solo lo que usa el CLI (schema/harness.schema.json → verificacion).
	Verificacion struct {
		MutacionMin *float64 `yaml:"mutacion_min"`
	} `yaml:"verificacion"`
}

// Accion describe un efecto de `hz activar` sobre un archivo.
type Accion struct{ Ruta, Resultado string }

const archivoPerfil = ".harness/harness.yaml"

// Buscar sube desde `desde` hasta encontrar la raíz que contiene .harness/harness.yaml.
func Buscar(desde string) (string, error) {
	dir, err := filepath.Abs(desde)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, archivoPerfil)); err == nil {
			return dir, nil
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return "", ErrSinProyecto
		}
		dir = padre
	}
}

// Cargar lee el perfil y completa los valores por defecto.
func Cargar(raiz string) (*Perfil, error) {
	b, err := os.ReadFile(filepath.Join(raiz, archivoPerfil))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSinProyecto
		}
		return nil, err
	}
	var p Perfil
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", archivoPerfil, err)
	}
	if p.Limites.LineasMaxAgentsRaiz == 0 {
		p.Limites.LineasMaxAgentsRaiz = 100
	}
	if p.Limites.LineasMaxAgentsDominio == 0 {
		p.Limites.LineasMaxAgentsDominio = 40
	}
	if p.Limites.LineasMaxArchivo == 0 {
		p.Limites.LineasMaxArchivo = 400
	}
	if p.Idioma == "" {
		p.Idioma = "es"
	}
	return &p, nil
}

// Validar devuelve un mensaje por cada campo inválido.
func (p Perfil) Validar() []string {
	var errs []string
	if p.Version != 1 {
		errs = append(errs, fmt.Sprintf("version debe ser 1 (es %d)", p.Version))
	}
	if p.Nivel != "N1" && p.Nivel != "N2" && p.Nivel != "N3" {
		errs = append(errs, fmt.Sprintf("nivel debe ser N1, N2 o N3 (es %q)", p.Nivel))
	}
	if p.Modo != "genesis" && p.Modo != "mejora" {
		errs = append(errs, fmt.Sprintf("modo debe ser genesis o mejora (es %q)", p.Modo))
	}
	return errs
}

// Activar crea o actualiza el perfil del harness en `raiz` sin pisar nada existente.
// `nivel` y `modo` vacíos conservan los del perfil existente (nunca se baja de nivel por omisión).
// Nunca crea CLAUDE.md; si existe sin @AGENTS.md, le agrega la importación (D-0025).
// En N2+ crea reglas.yaml y fusiona la autoprotección y las RD con capa `permisos` en .claude/settings.json.
func Activar(raiz, modo, nivel string) ([]Accion, error) {
	nuevoModo, nuevoNivel := valorOr(modo, "mejora"), valorOr(nivel, "N1")
	if errs := (Perfil{Version: 1, Nivel: nuevoNivel, Modo: nuevoModo}).Validar(); len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	var acciones []Accion
	agregar := func(a Accion, err error) error {
		if err == nil && a.Ruta != "" {
			acciones = append(acciones, a)
		}
		return err
	}
	if err := agregar(crearSiFalta(raiz, archivoPerfil, fmt.Sprintf(
		"# Perfil del harness 528hz (schema/harness.schema.json)\nversion: 1\nenabled: true\nnivel: %s\nmodo: %s\nidioma: es\ncomandos: {}\n",
		nuevoNivel, nuevoModo))); err != nil {
		return acciones, err
	}
	if nivel != "" {
		if err := agregar(fijarCampo(raiz, "nivel", nivel)); err != nil {
			return acciones, err
		}
	}
	for _, paso := range []func() (Accion, error){
		func() (Accion, error) { return crearDirSiFalta(raiz, ".harness/log") },
		func() (Accion, error) { return asegurarLinea(raiz, ".gitignore", ".harness/harness.db*") },
		func() (Accion, error) { return asegurarLinea(raiz, ".gitignore", ".harness/cache/") },
		func() (Accion, error) { return crearSiFalta(raiz, "AGENTS.md", esqueletoAgents(filepath.Base(raiz))) },
		func() (Accion, error) { return importarAgentsEnClaude(raiz) },
	} {
		if err := agregar(paso()); err != nil {
			return acciones, err
		}
	}
	p, err := Cargar(raiz)
	if err != nil {
		return acciones, err
	}
	if p.Nivel == "N1" {
		return acciones, nil
	}
	if err := agregar(crearSiFalta(raiz, archivoReglas, plantillaReglas)); err != nil {
		return acciones, err
	}
	reglas, err := CargarReglas(raiz)
	if err != nil {
		return acciones, err
	}
	return acciones, agregar(FusionarPermisos(raiz, DenyNativos(reglas)))
}

func valorOr(v, defecto string) string {
	if v == "" {
		return defecto
	}
	return v
}

// lineaCampo ubica `clave: valor` de primer nivel en harness.yaml.
var lineaCampo = map[string]*regexp.Regexp{
	"nivel": regexp.MustCompile(`(?m)^nivel:[ \t]*\S*`),
	"modo":  regexp.MustCompile(`(?m)^modo:[ \t]*\S*`),
}

// fijarCampo cambia el valor de un campo de primer nivel conservando el resto del archivo.
func fijarCampo(raiz, clave, valor string) (Accion, error) {
	ruta := filepath.Join(raiz, archivoPerfil)
	b, err := os.ReadFile(ruta)
	if err != nil {
		return Accion{}, err
	}
	nueva := clave + ": " + valor
	re := lineaCampo[clave]
	actual := re.Find(b)
	if string(actual) == nueva {
		return Accion{}, nil
	}
	var salida []byte
	if actual == nil {
		salida = append(append(b, '\n'), []byte(nueva+"\n")...)
	} else {
		salida = re.ReplaceAll(b, []byte(nueva))
	}
	if err := os.WriteFile(ruta, salida, 0o644); err != nil {
		return Accion{}, err
	}
	return Accion{archivoPerfil + " (" + nueva + ")", "modificado"}, nil
}

const plantillaReglas = `# Reglas duras del proyecto (RD). Solo el humano las modifica (/hz regla …); el agente tiene denegada su edición.
# Formato: schema/reglas.schema.json del harness. Punto de partida: reglas/catalogo-rd.md (elegir y adaptar, no imponer).
# Cada regla: id RD-n, orden imperativa, por_que, origen, desde, ≥2 capas de enforcement, patrones, remediacion, estado.
version: 1
reglas: []
`

func crearSiFalta(raiz, rel, contenido string) (Accion, error) {
	ruta := filepath.Join(raiz, rel)
	if _, err := os.Stat(ruta); err == nil {
		return Accion{rel, "existía"}, nil
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return Accion{}, err
	}
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		return Accion{}, err
	}
	return Accion{rel, "creado"}, nil
}

func crearDirSiFalta(raiz, rel string) (Accion, error) {
	ruta := filepath.Join(raiz, rel)
	if st, err := os.Stat(ruta); err == nil && st.IsDir() {
		return Accion{rel, "existía"}, nil
	}
	if err := os.MkdirAll(ruta, 0o755); err != nil {
		return Accion{}, err
	}
	return Accion{rel, "creado"}, nil
}

// asegurarLinea agrega `linea` al final de `rel` si no está; crea el archivo si no existe.
func asegurarLinea(raiz, rel, linea string) (Accion, error) {
	ruta := filepath.Join(raiz, rel)
	b, err := os.ReadFile(ruta)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Accion{}, err
	}
	actual := string(b)
	if strings.Contains(actual, linea) {
		return Accion{rel, "existía"}, nil
	}
	resultado := "modificado"
	if errors.Is(err, os.ErrNotExist) {
		resultado = "creado"
	}
	if actual != "" && !strings.HasSuffix(actual, "\n") {
		actual += "\n"
	}
	if err := os.WriteFile(ruta, []byte(actual+linea+"\n"), 0o644); err != nil {
		return Accion{}, err
	}
	return Accion{rel, resultado}, nil
}

func importarAgentsEnClaude(raiz string) (Accion, error) {
	if _, err := os.Stat(filepath.Join(raiz, "CLAUDE.md")); errors.Is(err, os.ErrNotExist) {
		return Accion{}, nil // nunca se crea (D-0025)
	}
	return asegurarLinea(raiz, "CLAUDE.md", "@AGENTS.md")
}

func esqueletoAgents(nombre string) string {
	return fmt.Sprintf(`# %s

Mapa del proyecto para agentes (≤100 líneas). Apunta a dónde vive el detalle; no es un manual.

## Estado
- Recién activado con el harness 528hz. Completar el mapa con `+"`hz descubrir`"+` o a mano.

## Mapa de dominios
| Dominio | Carpeta | Responsabilidad |
|---|---|---|

## Comandos únicos
Declarados en `+"`.harness/harness.yaml → comandos`"+` (test, lint, build, run).

## Reglas
- RG-1 Código de producción y eficiente en recursos (`+"`hz regla ver RG-1`"+`).
- RG-2 Test-driven siempre: CONTRATO → ROJO → VERDE → REFACTOR.
- RG-3 Toda decisión se registra: `+"`hz decision nueva`"+`; antes de proponer, `+"`hz decision buscar <tema>`"+`.

## Cómo consultar
`+"`hz estado`"+` para retomar · `+"`hz decision buscar <tema>`"+` · `+"`hz check`"+`
`, nombre)
}

// umbralMutacionPorDefecto: con ≤20 mutantes admite hasta 2 sobrevivientes (algunos son equivalentes).
const umbralMutacionPorDefecto = 0.9

// UmbralMutacion es la detección mínima de hz mutar para que el verificador entregue una tarea (D-0046).
// 0 desactiva la exigencia.
func (p *Perfil) UmbralMutacion() float64 {
	if p.Verificacion.MutacionMin == nil {
		return umbralMutacionPorDefecto
	}
	return *p.Verificacion.MutacionMin
}
