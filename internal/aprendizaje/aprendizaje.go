// Package aprendizaje implementa el ciclo error → lección → eval (docs/diseno/06-tdd-y-evals.md §4, D-0008):
// transiciones de las lecciones, la decisión determinista tras sus evals y la eliminación de raíz.
package aprendizaje

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hz528/internal/motor"
	"hz528/internal/tdd"
)

// Las reescrituras máximas antes de escalar al humano.
const maxReescrituras = 2

var transiciones = map[string]map[string]bool{
	"en_prueba": {"activa": true, "reescrita": true},
	"reescrita": {"en_prueba": true},
	"activa":    {"reescrita": true, "podada": true},
}

// Transicion valida un cambio de estado de lección. Eliminar de raíz no pasa por aquí (ver Eliminar).
func Transicion(actual, nuevo string, reescrituras int) error {
	if !transiciones[actual][nuevo] {
		return fmt.Errorf("%w: una lección no pasa de %s a %s", tdd.ErrVerificacion, actual, nuevo)
	}
	if nuevo == "reescrita" && reescrituras >= maxReescrituras {
		return fmt.Errorf("%w: la lección ya se reescribió %d veces; escala al humano", tdd.ErrVerificacion, reescrituras)
	}
	return nil
}

// Decision es la recomendación determinista sobre una lección.
type Decision struct {
	Accion string `json:"accion"`
	Motivo string `json:"motivo"`
}

var tiposEval = []string{"prevencion", "interferencia", "activacion"}

// Decidir aplica, en orden: crítico → eliminar; faltan evals; todo pasa → activar; falla → reescribir o escalar.
func Decidir(evals map[string]string, reescrituras, criticos int) Decision {
	if criticos > 0 {
		return Decision{"eliminar", fmt.Sprintf("%d error(es) crítico(s) causados por la lección: se elimina de raíz", criticos)}
	}
	var faltan, fallan []string
	for _, t := range tiposEval {
		switch evals[t] {
		case "":
			faltan = append(faltan, t)
		case "falla":
			fallan = append(fallan, t)
		}
	}
	if len(faltan) > 0 {
		return Decision{"faltan evals: " + strings.Join(faltan, ", "), "una lección no se activa sin sus tres evals"}
	}
	if len(fallan) > 0 {
		if reescrituras >= maxReescrituras {
			return Decision{"escalar al humano", fmt.Sprintf("falla en %s tras %d reescrituras", strings.Join(fallan, ", "), reescrituras)}
		}
		return Decision{"reescribir", "falla en " + strings.Join(fallan, ", ") + " (falla menor de interpretación)"}
	}
	return Decision{"activar", "previene el error, no interfiere y se activa cuando corresponde"}
}

func rutaSkill(raiz, skill string) string { return filepath.Join(raiz, ".claude", "skills", skill) }

// NuevaLeccion exige que la skill ya esté escrita y que la clase exista antes de registrar la lección.
func NuevaLeccion(m *motor.Motor, raiz string, l motor.Leccion) (string, error) {
	if _, err := os.Stat(filepath.Join(rutaSkill(raiz, l.Skill), "SKILL.md")); err != nil {
		return "", fmt.Errorf("%w: escribe primero .claude/skills/%s/SKILL.md", tdd.ErrVerificacion, l.Skill)
	}
	if _, err := m.Clase(l.Clase); err != nil {
		return "", fmt.Errorf("%w: la clase %s no existe; registra antes el error con `hz error nuevo`", tdd.ErrVerificacion, l.Clase)
	}
	return m.NuevaLeccion(l)
}

// CambiarEstado aplica una transición validada.
func CambiarEstado(m *motor.Motor, id, nuevo, motivo string) error {
	l, err := m.Leccion(id)
	if err != nil {
		return err
	}
	if err := Transicion(l.Estado, nuevo, l.Reescrituras); err != nil {
		return err
	}
	return m.FijarEstadoLeccion(id, nuevo, motivo)
}

// Eliminar borra de raíz la skill de la lección, la marca eliminada y registra la decisión (RG-3).
func Eliminar(m *motor.Motor, raiz, id, motivo string) error {
	l, err := m.Leccion(id)
	if err != nil {
		return err
	}
	// Solo se borran skills del harness: nunca una skill ajena por un nombre mal escrito.
	if !strings.HasPrefix(l.Skill, "hz-leccion-") || strings.ContainsAny(l.Skill, `/\.`) {
		return fmt.Errorf("%w: %q no es una skill de lección del harness; no se borra", tdd.ErrVerificacion, l.Skill)
	}
	if err := os.RemoveAll(rutaSkill(raiz, l.Skill)); err != nil {
		return err
	}
	if err := m.FijarEstadoLeccion(l.ID, "eliminada", motivo); err != nil {
		return err
	}
	_, err = m.NuevaDecision(motor.Decision{
		Que:       fmt.Sprintf("Lección %s (%s) eliminada de raíz", l.ID, l.Skill),
		PorQue:    motivo,
		Decidio:   "agente",
		Alcance:   []string{".claude/skills/" + l.Skill},
		Etiquetas: []string{"lecciones", "eliminada", l.ID, l.Clase},
	})
	return err
}

// RegistrarError registra el error y aplica la regla: un error crítico causado por una lección la elimina de raíz.
func RegistrarError(m *motor.Motor, raiz string, e motor.Error) (id string, recurrencia int, accion string, err error) {
	id, recurrencia, err = m.NuevoError(e)
	if err != nil || e.Leccion == "" || e.Severidad != "critico" {
		return id, recurrencia, "", err
	}
	if err := Eliminar(m, raiz, e.Leccion, fmt.Sprintf("Error crítico %s causado por la lección: %s", id, e.Que)); err != nil {
		return id, recurrencia, "", err
	}
	return id, recurrencia, fmt.Sprintf("lección %s eliminada de raíz", strings.ToUpper(e.Leccion)), nil
}

// Aplicar ejecuta la acción de Decidir sobre la lección.
func Aplicar(m *motor.Motor, raiz, id string, d Decision) error {
	switch {
	case d.Accion == "eliminar":
		return Eliminar(m, raiz, id, d.Motivo)
	case d.Accion == "activar":
		return CambiarEstado(m, id, "activa", d.Motivo)
	case d.Accion == "reescribir":
		return CambiarEstado(m, id, "reescrita", d.Motivo)
	default:
		return nil // faltan evals o escalar al humano: no hay transición automática
	}
}

// ParaRuta devuelve, ordenadas, las lecciones cuyo ámbito coincide con la ruta (modo índice, D-0021).
func ParaRuta(lecciones []motor.Leccion, rel string, coincide func(patron, rel string) bool) []motor.Leccion {
	var out []motor.Leccion
	for _, l := range lecciones {
		if coincide(strings.TrimPrefix(filepath.ToSlash(l.Ambito), "./"), filepath.ToSlash(rel)) {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
