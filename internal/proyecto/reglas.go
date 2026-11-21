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

// Patrones describen qué se considera violación de una regla dura (alimentan hook, permisos y red-team).
type Patrones struct {
	Herramientas []string `yaml:"herramientas"`
	Bash         []string `yaml:"bash"`
	Rutas        []string `yaml:"rutas"`
	Dominios     []string `yaml:"dominios"`
}

// ReglaDura es una ficha de .harness/reglas.yaml (schema/reglas.schema.json).
type ReglaDura struct {
	ID            string   `yaml:"id"`
	ClaveCatalogo string   `yaml:"clave_catalogo"`
	Orden         string   `yaml:"orden"`
	PorQue        string   `yaml:"por_que"`
	Origen        string   `yaml:"origen"`
	Desde         string   `yaml:"desde"`
	Enforcement   []string `yaml:"enforcement"`
	Patrones      Patrones `yaml:"patrones"`
	Allowlist     []string `yaml:"allowlist"`
	Remediacion   string   `yaml:"remediacion"`
	RedTeam       []string `yaml:"red_team"`
	Estado        string   `yaml:"estado"`
}

const archivoReglas = ".harness/reglas.yaml"

var (
	idRegla      = regexp.MustCompile(`^RD-[0-9]+$`)
	origenValido = map[string]bool{"genesis": true, "mejora": true, "incidente": true}
	capaValida   = map[string]bool{"permisos": true, "hook": true, "githook": true, "ci": true, "sensor": true, "rubrica": true}
	estadoValido = map[string]bool{"vigente": true, "inefectiva": true, "modificada": true, "retirada": true}
)

// CargarReglas lee y valida las reglas duras. Sin archivo devuelve una lista vacía.
func CargarReglas(raiz string) ([]ReglaDura, error) {
	b, err := os.ReadFile(filepath.Join(raiz, archivoReglas))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Version int         `yaml:"version"`
		Reglas  []ReglaDura `yaml:"reglas"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", archivoReglas, err)
	}
	var errs []string
	vistos := map[string]bool{}
	for _, r := range doc.Reglas {
		if e := validarRegla(r); e != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", r.ID, e))
		}
		if vistos[r.ID] {
			errs = append(errs, fmt.Sprintf("%s: id duplicado", r.ID))
		}
		vistos[r.ID] = true
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s inválido: %s", archivoReglas, strings.Join(errs, "; "))
	}
	return doc.Reglas, nil
}

func validarRegla(r ReglaDura) string {
	var p []string
	if !idRegla.MatchString(r.ID) {
		p = append(p, "id inválido (se espera RD-n)")
	}
	if strings.TrimSpace(r.Orden) == "" || strings.TrimSpace(r.PorQue) == "" {
		p = append(p, "orden y por_que son obligatorios")
	}
	if !origenValido[r.Origen] {
		p = append(p, "origen debe ser genesis, mejora o incidente")
	}
	capas := 0
	for _, c := range r.Enforcement {
		if capaValida[c] {
			capas++
		}
	}
	if capas < 2 {
		p = append(p, fmt.Sprintf("capas de enforcement válidas: %d (mínimo 2, defensa en profundidad)", capas))
	}
	if !estadoValido[r.Estado] {
		p = append(p, "estado inválido")
	}
	return strings.Join(p, ", ")
}

// Vigentes filtra las reglas en estado vigente.
func Vigentes(rs []ReglaDura) []ReglaDura {
	out := make([]ReglaDura, 0, len(rs))
	for _, r := range rs {
		if r.Estado == "vigente" {
			out = append(out, r)
		}
	}
	return out
}

// TieneCapa indica si la regla declara una capa de enforcement.
func (r ReglaDura) TieneCapa(capa string) bool {
	for _, c := range r.Enforcement {
		if c == capa {
			return true
		}
	}
	return false
}
