package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"hz528/internal/exportar"
	"hz528/internal/motor"
	"hz528/internal/tdd"
)

// lista es un flag repetible.
type lista []string

func (l *lista) String() string     { return strings.Join(*l, ",") }
func (l *lista) Set(v string) error { *l = append(*l, v); return nil }

// parsearIntercalado permite flags antes y después de los argumentos posicionales.
func parsearIntercalado(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func (c contexto) flagSet(nombre string) *flag.FlagSet {
	fs := flag.NewFlagSet(nombre, flag.ContinueOnError)
	fs.SetOutput(c.err)
	return fs
}

func recortar(s string, n int) string {
	r := []rune(strings.ReplaceAll(s, "\n", " "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}

// ───────────────────────── decision ─────────────────────────

func (c contexto) decision(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz decision nueva|buscar|ver …")
		return usoInvalido
	}
	switch args[0] {
	case "nueva":
		return c.decisionNueva(args[1:])
	case "buscar":
		return c.decisionBuscar(args[1:])
	case "ver":
		return c.decisionVer(args[1:])
	}
	fmt.Fprintf(c.err, "subcomando desconocido: decision %s\n", args[0])
	return usoInvalido
}

func (c contexto) decisionNueva(args []string) int {
	fs := c.flagSet("decision nueva")
	var d motor.Decision
	var alternativas, alcance, etiquetas lista
	fs.StringVar(&d.Que, "que", "", "la decisión en una frase")
	fs.StringVar(&d.PorQue, "por-que", "", "evidencia y restricciones")
	fs.StringVar(&d.Decidio, "decidio", "", "humano | agente | agente_aprobado")
	fs.StringVar(&d.MisionID, "mision", "", "misión")
	fs.StringVar(&d.RondaOHito, "ronda", "", "ronda o hito")
	fs.StringVar(&d.CommitRef, "commit", "", "commit")
	fs.StringVar(&d.Sesion, "sesion", os.Getenv("HZ_SESION"), "sesión")
	noReversible := fs.Bool("no-reversible", false, "la decisión no es reversible")
	fs.Var(&alternativas, "alternativa", `"opción|motivo del descarte" (repetible)`)
	fs.Var(&alcance, "alcance", "módulo o archivo (repetible)")
	fs.Var(&etiquetas, "etiqueta", "tema para la búsqueda (repetible)")
	if _, err := parsearIntercalado(fs, args); err != nil {
		return usoInvalido
	}
	for _, a := range alternativas {
		opcion, motivo, _ := strings.Cut(a, "|")
		d.Alternativas = append(d.Alternativas, motor.Alternativa{Opcion: strings.TrimSpace(opcion), DescartadaPor: strings.TrimSpace(motivo)})
	}
	d.Alcance, d.Etiquetas, d.Reversible = alcance, etiquetas, !*noReversible
	id, err := c.m.NuevaDecision(d)
	if err != nil {
		return c.codigo(err)
	}
	if err := exportar.Escribir(c.m, c.raiz); err != nil { // el resumen del puntero cuenta decisiones
		return c.codigo(err)
	}
	fmt.Fprintln(c.out, id)
	return ok
}

func (c contexto) decisionBuscar(args []string) int {
	fs := c.flagSet("decision buscar")
	limite := fs.Int("limite", 5, "máximo de resultados")
	pos, err := parsearIntercalado(fs, args)
	if err != nil || len(pos) == 0 {
		fmt.Fprintln(c.err, "uso: hz decision buscar <tema…> [--limite n]")
		return usoInvalido
	}
	tema := strings.Join(pos, " ")
	ds, err := c.m.BuscarDecisiones(tema, *limite)
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(ds)
	}
	if len(ds) == 0 {
		fmt.Fprintf(c.out, "sin decisiones sobre %q\n", tema)
	}
	for _, d := range ds {
		fmt.Fprintf(c.out, "%s · %s · %s\n", d.ID, d.Estado, recortar(d.Que, 100))
	}
	return ok
}

func (c contexto) decisionVer(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(c.err, "uso: hz decision ver D-NNNN")
		return usoInvalido
	}
	d, err := c.m.VerDecision(args[0])
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(d)
	}
	fmt.Fprintf(c.out, "%s · %s · decidió: %s\n", d.ID, d.Estado, d.Decidio)
	ubicacion := []string{}
	for _, par := range [][2]string{{"misión", d.MisionID}, {"ronda/hito", d.RondaOHito}, {"commit", d.CommitRef}} {
		if par[1] != "" {
			ubicacion = append(ubicacion, par[0]+" "+par[1])
		}
	}
	fmt.Fprintf(c.out, "cuándo: %s %s\n", d.Cuando, strings.Join(ubicacion, ", "))
	fmt.Fprintf(c.out, "qué: %s\npor qué: %s\n", d.Que, d.PorQue)
	if len(d.Alternativas) > 0 {
		fmt.Fprintln(c.out, "alternativas descartadas:")
		for _, a := range d.Alternativas {
			fmt.Fprintf(c.out, "  - %s → %s\n", a.Opcion, a.DescartadaPor)
		}
	}
	reversible := "sí"
	if !d.Reversible {
		reversible = "no"
	}
	fmt.Fprintf(c.out, "alcance: %s · etiquetas: %s · reversible: %s\n", strings.Join(d.Alcance, ", "), strings.Join(d.Etiquetas, ", "), reversible)
	if d.ReemplazadaPor != "" {
		fmt.Fprintf(c.out, "reemplazada por: %s\n", d.ReemplazadaPor)
	}
	return ok
}

// ───────────────────────── estado ─────────────────────────

type estadoMision struct {
	Mision      string `json:"mision"`
	Fase        string `json:"fase"`
	Estado      string `json:"estado"`
	ProximoPaso string `json:"proximo_paso"`
}

func (c contexto) estado() int {
	total, err := c.m.ContarDecisiones()
	if err != nil {
		return c.codigo(err)
	}
	ultimas, err := c.m.UltimasDecisiones(3)
	if err != nil {
		return c.codigo(err)
	}
	tareas, err := c.m.Tareas()
	if err != nil {
		return c.codigo(err)
	}
	abiertas := make([]motor.Tarea, 0, len(tareas))
	for _, t := range tareas {
		if t.Estado != "cerrada" && t.Estado != "verificada" {
			abiertas = append(abiertas, t)
		}
	}
	misiones := misionesActivas(c.raiz)
	if c.g.json {
		return c.emitirJSON(map[string]any{"proyecto": filepath.Base(c.raiz), "nivel": c.perfil.Nivel, "modo": c.perfil.Modo,
			"decisiones": total, "ultimas_decisiones": ultimas, "tareas_abiertas": abiertas, "misiones_activas": misiones})
	}
	fmt.Fprintf(c.out, "proyecto: %s · nivel %s · modo %s\n", filepath.Base(c.raiz), c.perfil.Nivel, c.perfil.Modo)
	for _, ms := range misiones {
		fmt.Fprintf(c.out, "misión activa: %s · fase %s · próximo: %s\n", ms.Mision, ms.Fase, recortar(ms.ProximoPaso, 90))
	}
	fmt.Fprintf(c.out, "decisiones: %d", total)
	if len(ultimas) > 0 {
		fmt.Fprint(c.out, " · últimas:")
	}
	fmt.Fprintln(c.out)
	for _, d := range ultimas {
		fmt.Fprintf(c.out, "  %s · %s\n", d.ID, recortar(d.Que, 90))
	}
	fmt.Fprintf(c.out, "tareas TDD abiertas: %d\n", len(abiertas))
	for i, t := range abiertas {
		if i == 20 {
			fmt.Fprintf(c.out, "  … y %d más (hz tdd estado)\n", len(abiertas)-20)
			break
		}
		fmt.Fprintf(c.out, "  %s · %s · %s\n", t.ID, strings.ToUpper(t.Estado), recortar(t.Titulo, 70))
	}
	fmt.Fprintln(c.out, "más: hz decision buscar <tema> · hz tdd estado · hz check")
	return ok
}

func misionesActivas(raiz string) []estadoMision {
	archivos, _ := filepath.Glob(filepath.Join(raiz, ".harness", "misiones", "*", "estado.json"))
	var res []estadoMision
	for _, a := range archivos {
		b, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		var e estadoMision
		if json.Unmarshal(b, &e) == nil && (e.Estado == "" || e.Estado == "activa") {
			if e.Mision == "" {
				e.Mision = filepath.Base(filepath.Dir(a))
			}
			res = append(res, e)
		}
	}
	return res
}

// ───────────────────────── tdd ─────────────────────────

func (c contexto) tdd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz tdd contrato|rojo|verde|refactor|estado …")
		return usoInvalido
	}
	q := tdd.Maquina{M: c.m, Raiz: c.raiz, CmdTest: c.perfil.Comandos["test"]}
	sub, resto := args[0], args[1:]
	fs := c.flagSet("tdd " + sub)
	switch sub {
	case "contrato":
		titulo := fs.String("titulo", "", "título de la tarea")
		contrato := fs.String("contrato", "", "ruta del contrato")
		aprobado := fs.Bool("aprobado", false, "el humano aprobó el contrato")
		mision := fs.String("mision", "", "misión")
		pos, err := parsearIntercalado(fs, resto)
		if err != nil || len(pos) != 1 || *titulo == "" || *contrato == "" {
			fmt.Fprintln(c.err, "uso: hz tdd contrato T-n --titulo \"…\" --contrato <ruta> [--aprobado] [--mision m]")
			return usoInvalido
		}
		return c.transicion(q.Contrato(pos[0], *titulo, *contrato, *aprobado, *mision), pos[0], "CONTRATO")
	case "rojo":
		tests := fs.String("tests", "", "tests a bloquear, separados por coma")
		cmd := fs.String("cmd", "", "comando de test")
		pos, err := parsearIntercalado(fs, resto)
		if err != nil || len(pos) != 1 {
			fmt.Fprintln(c.err, "uso: hz tdd rojo T-n --tests a,b [--cmd \"…\"]")
			return usoInvalido
		}
		return c.transicion(q.Rojo(pos[0], dividir(*tests), *cmd), pos[0], "ROJO")
	case "verde", "refactor":
		cmd := fs.String("cmd", "", "comando de test")
		pos, err := parsearIntercalado(fs, resto)
		if err != nil || len(pos) != 1 {
			fmt.Fprintf(c.err, "uso: hz tdd %s T-n [--cmd \"…\"]\n", sub)
			return usoInvalido
		}
		paso := q.Verde
		if sub == "refactor" {
			paso = q.Refactor
		}
		return c.transicion(paso(pos[0], *cmd), pos[0], strings.ToUpper(sub))
	case "estado":
		return c.tddEstado(resto)
	}
	fmt.Fprintf(c.err, "subcomando desconocido: tdd %s\n", sub)
	return usoInvalido
}

func (c contexto) transicion(err error, id, estado string) int {
	if err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "%s → %s\n", strings.ToUpper(id), estado)
	return ok
}

func (c contexto) tddEstado(args []string) int {
	if len(args) == 1 {
		t, err := c.m.Tarea(args[0])
		if err != nil {
			return c.codigo(err)
		}
		if c.g.json {
			return c.emitirJSON(t)
		}
		fmt.Fprintf(c.out, "%s · %s · %s\ncontrato: %s (aprobado: %s)\n", t.ID, strings.ToUpper(t.Estado), t.Titulo, t.ContratoRuta, siNo(t.ContratoOKEn != ""))
		for _, tb := range t.TestsBloqueados {
			fmt.Fprintf(c.out, "bloqueado: %s (%s…)\n", tb.Ruta, tb.SHA256[:12])
		}
		for _, e := range t.Evidencia {
			fmt.Fprintf(c.out, "evidencia %s: %s → %s\n", e.En, recortar(e.Cmd, 60), e.Resultado)
		}
		return ok
	}
	ts, err := c.m.Tareas()
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(ts)
	}
	if len(ts) == 0 {
		fmt.Fprintln(c.out, "sin tareas TDD")
	}
	for _, t := range ts {
		fmt.Fprintf(c.out, "%s · %s · %s\n", t.ID, strings.ToUpper(t.Estado), recortar(t.Titulo, 80))
	}
	return ok
}

func dividir(s string) []string {
	var r []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			r = append(r, p)
		}
	}
	return r
}

func siNo(b bool) string {
	if b {
		return "sí"
	}
	return "no"
}
