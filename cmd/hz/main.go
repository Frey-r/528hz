// Comando hz: CLI determinista del harness 528hz (contrato en contratos/cli.md).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hz528"
	"hz528/internal/check"
	"hz528/internal/exportar"
	"hz528/internal/motor"
	"hz528/internal/orquesta"
	"hz528/internal/proyecto"
	"hz528/internal/reglas"
	"hz528/internal/tdd"
)

const version = "0.1.0"

// Códigos de salida del contrato.
const (
	ok           = 0
	falloVerif   = 1
	usoInvalido  = 2
	sinProyecto  = 3
	errorInterno = 4
)

type globales struct {
	raiz string
	json bool
}

// separarGlobales extrae --raiz y --json de cualquier posición.
func separarGlobales(args []string) (globales, []string, error) {
	var g globales
	resto := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			g.json = true
		case a == "--raiz":
			if i+1 >= len(args) {
				return g, nil, errors.New("--raiz requiere un directorio")
			}
			g.raiz = args[i+1]
			i++
		case strings.HasPrefix(a, "--raiz="):
			g.raiz = strings.TrimPrefix(a, "--raiz=")
		default:
			resto = append(resto, a)
		}
	}
	if g.raiz == "" {
		g.raiz = os.Getenv("HZ_RAIZ")
	}
	return g, resto, nil
}

func (g globales) directorio() string {
	if g.raiz != "" {
		return g.raiz
	}
	wd, _ := os.Getwd()
	return wd
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

var conProyecto = map[string]bool{"reindex": true, "decision": true, "check": true, "estado": true, "tdd": true, "exportar": true,
	"tablero": true, "arriendo": true, "tarjeta": true, "contexto": true, "salida": true, "equipo": true,
	"error": true, "leccion": true, "eval": true, "mutar": true, "redteam": true,
	"mision": true, "ronda": true, "compare": true, "ruido": true, "milestone": true, "informe": true}

func run(args []string, stdout, stderr io.Writer) int {
	g, args, err := separarGlobales(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return usoInvalido
	}
	if len(args) == 0 {
		uso(stderr)
		return usoInvalido
	}
	cmd, resto := args[0], args[1:]
	switch cmd {
	case "version", "--version":
		fmt.Fprintf(stdout, "hz %s\n", version)
		return ok
	case "help", "-h", "--help":
		uso(stdout)
		return ok
	case "activar":
		return cmdActivar(g, resto, stdout, stderr)
	case "regla":
		return cmdRegla(resto, stdout, stderr)
	case "puntero":
		return cmdPuntero(g, stdout, stderr)
	case "capacidad":
		return cmdCapacidad(g, resto, stdout, stderr)
	}
	if !conProyecto[cmd] {
		fmt.Fprintf(stderr, "comando desconocido %q\n", cmd)
		uso(stderr)
		return usoInvalido
	}
	raiz, err := proyecto.Buscar(g.directorio())
	if err != nil {
		fmt.Fprintf(stderr, "%v; actívalo con `hz activar`\n", err)
		return sinProyecto
	}
	perfil, err := proyecto.Cargar(raiz)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return errorInterno
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return errorInterno
	}
	defer m.Cerrar()
	ctx := contexto{g: g, raiz: raiz, perfil: perfil, m: m, out: stdout, err: stderr}
	switch cmd {
	case "reindex":
		return ctx.reindex()
	case "decision":
		return ctx.decision(resto)
	case "check":
		return ctx.check()
	case "estado":
		return ctx.estado()
	case "exportar":
		return ctx.exportar()
	case "tablero":
		return ctx.tablero(resto)
	case "arriendo":
		return ctx.arriendo(resto)
	case "tarjeta":
		return ctx.tarjeta(resto)
	case "contexto":
		return ctx.contexto(resto)
	case "salida":
		return ctx.salida(resto)
	case "equipo":
		return ctx.equipo()
	case "error":
		return ctx.errorCmd(resto)
	case "leccion":
		return ctx.leccion(resto)
	case "eval":
		return ctx.eval(resto)
	case "mutar":
		return ctx.mutar(resto)
	case "redteam":
		return ctx.redteam(resto)
	case "mision":
		return ctx.misionCmd(resto)
	case "ronda":
		return ctx.ronda(resto)
	case "compare":
		return ctx.compare(resto)
	case "ruido":
		return ctx.ruido(resto)
	case "milestone":
		return ctx.milestone(resto)
	case "informe":
		return ctx.informeCmd(resto)
	default: // tdd
		return ctx.tdd(resto)
	}
}

type contexto struct {
	g      globales
	raiz   string
	perfil *proyecto.Perfil
	m      *motor.Motor
	out    io.Writer
	err    io.Writer
}

// codigo traduce errores de dominio a códigos de salida del contrato.
func (c contexto) codigo(err error) int {
	if err == nil {
		return ok
	}
	fmt.Fprintln(c.err, err)
	switch {
	case errors.Is(err, motor.ErrCampos), errors.Is(err, orquesta.ErrRolDesconocido):
		return usoInvalido
	case errors.Is(err, tdd.ErrVerificacion), errors.Is(err, motor.ErrNoExiste), errors.Is(err, motor.ErrConflicto):
		return falloVerif
	default:
		return errorInterno
	}
}

func (c contexto) emitirJSON(v any) int {
	enc := json.NewEncoder(c.out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return errorInterno
	}
	return ok
}

func cmdActivar(g globales, args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("activar", flag.ContinueOnError)
	fs.SetOutput(errw)
	modo := fs.String("modo", "mejora", "genesis | mejora")
	nivel := fs.String("nivel", "N1", "N1 | N2 | N3")
	if err := fs.Parse(args); err != nil {
		return usoInvalido
	}
	raiz, err := filepath.Abs(g.directorio())
	if err != nil {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	acciones, err := proyecto.Activar(raiz, *modo, *nivel)
	if err != nil {
		fmt.Fprintln(errw, err)
		return usoInvalido
	}
	for _, a := range acciones {
		fmt.Fprintf(out, "%-10s %s\n", a.Resultado, a.Ruta)
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	defer m.Cerrar()
	fmt.Fprintln(out, "listo      .harness/harness.db")
	if err := exportar.Escribir(m, raiz); err != nil {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	fmt.Fprintln(out, "exportado  .harness/cache/hooks.json")
	return ok
}

// cmdPuntero imprime el contexto de inicio de sesión para el hook clásico SessionStart (D-0036).
// Nunca falla: sin harness o ante cualquier error, no imprime nada y sale con 0.
func cmdPuntero(g globales, out, errw io.Writer) int {
	raiz, err := proyecto.Buscar(g.directorio())
	if err != nil {
		return ok
	}
	if p, err := proyecto.Cargar(raiz); err != nil || !p.Enabled {
		return ok
	}
	m, err := motor.Abrir(raiz)
	if err != nil {
		fmt.Fprintln(errw, err)
		return ok
	}
	defer m.Cerrar()
	h, err := exportar.Construir(m, raiz)
	if err != nil {
		fmt.Fprintln(errw, err)
		return ok
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":     "SessionStart",
		"additionalContext": strings.Join(h.Resumen, "\n"),
	}})
	return ok
}

func cmdRegla(args []string, out, errw io.Writer) int {
	if len(args) != 2 || args[0] != "ver" {
		fmt.Fprintln(errw, "uso: hz regla ver RG-n")
		return usoInvalido
	}
	txt, encontrada := reglas.Seccion(hz528.ReglasGenerales, args[1])
	if !encontrada {
		fmt.Fprintf(errw, "la regla %s no existe\n", args[1])
		return falloVerif
	}
	fmt.Fprintln(out, txt)
	return ok
}

func (c contexto) exportar() int {
	if err := exportar.Escribir(c.m, c.raiz); err != nil {
		return c.codigo(err)
	}
	fmt.Fprintln(c.out, "exportado .harness/cache/hooks.json")
	return ok
}

func (c contexto) reindex() int {
	r, err := c.m.Reindex()
	if err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "reindex: %d eventos (%d decisiones, %d de tareas, %d desconocidos) en %d ms\n",
		r.Eventos, r.Decisiones, r.Tareas, r.Desconocidos, r.Duracion.Milliseconds())
	return ok
}

func (c contexto) check() int {
	rs := check.Ejecutar(c.raiz, c.perfil)
	if c.g.json {
		c.emitirJSON(rs)
	} else {
		for _, r := range rs {
			marca := "✓"
			if !r.OK {
				marca = "✗"
			}
			fmt.Fprintf(c.out, "%s %s — %s\n", marca, r.Nombre, r.Motivo)
		}
	}
	if !check.TodoOK(rs) {
		return falloVerif
	}
	return ok
}

func uso(w io.Writer) {
	fmt.Fprint(w, `hz — CLI del harness 528hz (contratos/cli.md)

  hz activar [--modo genesis|mejora] [--nivel N1|N2|N3]
  hz estado [--json]                     resumen para retomar (≤40 líneas)
  hz check [--json]                      estructura orientada a agentes
  hz decision nueva|buscar|ver …         registro cuándo/qué/por qué (RG-3)
  hz tdd contrato|rojo|verde|refactor|estado …   ciclo TDD (RG-2)
  hz tablero [--listo] [--celula c]      tareas, dependencias y frente listo
  hz arriendo tomar|soltar|lista …       exclusividad de archivos entre agentes
  hz tarjeta registrar|ultima …          tarjetas de traspaso entre roles (≤30 líneas)
  hz contexto --rol R --tarea T-n        paquete de rol (la porción del harness de ese rol)
  hz salida --rol R --tarea T-n          criterio de salida determinista del rol
  hz capacidad [--json]                  cuántas células caben en esta máquina
  hz equipo                              directorio rol ↔ agentId (lo mantiene el plugin)
  hz error nuevo|lista …                 errores en puntos de entrega (el ROJO del TDD no es error)
  hz leccion nueva|estado|eliminar|lista|para …   lecciones (skills) con ciclo de vida
  hz eval registrar|decidir L-n …        evals de lecciones y decisión determinista
  hz mutar --archivo <src> [--cmd …]     mutation testing: ¿la suite detecta fallas sembradas?
  hz redteam plan|registrar|estado …     efectividad de las reglas duras
  hz mision nueva|fase|gasto|aceptar|cerrar|lista …   misiones N3 con presupuesto y contrato default-FAIL
  hz ronda nueva|medir|cerrar|lista …    rondas de mejora; medir corre los sensores deterministas
  hz compare <mision> R-a R-b [--metrica m]   comparación pareada con test de signos exacto
  hz ruido <mision> R-a R-b              piso de ruido (dos mediciones del mismo código)
  hz milestone proponer|marcar|lista …   el harness propone, el humano marca
  hz informe <mision> [--json] [--html f]   informe C-level (datos o página para Artifact)
  hz regla ver RG-n                      texto de una regla general
  hz reindex                             reconstruye el índice desde los logs
  hz exportar                            regenera .harness/cache/hooks.json para el plugin
  hz puntero                             contexto para el hook SessionStart (nunca falla)
  hz version

Globales: --raiz <dir> (o HZ_RAIZ), --json
`)
}
