package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"hz528/internal/aprendizaje"
	"hz528/internal/exportar"
	"hz528/internal/motor"
	"hz528/internal/mutar"
	"hz528/internal/proyecto"
	"hz528/internal/redteam"
)

// ───────────────────────── error ─────────────────────────

func (c contexto) errorCmd(args []string) int {
	if len(args) == 0 || (args[0] != "nuevo" && args[0] != "lista") {
		fmt.Fprintln(c.err, "uso: hz error nuevo --que … --fuente F --severidad S --causa C --clase c [...] | hz error lista [--clase c]")
		return usoInvalido
	}
	fs := c.flagSet("error " + args[0])
	var e motor.Error
	fs.StringVar(&e.Que, "que", "", "qué hizo el agente")
	fs.StringVar(&e.Fuente, "fuente", "", "verificacion|evaluador|humano|regla_dura|revert|regresion")
	fs.StringVar(&e.Severidad, "severidad", "", "critico|mayor|menor")
	fs.StringVar(&e.Causa, "causa", "", "causa raíz de la taxonomía")
	fs.StringVar(&e.Clase, "clase", "", "clase para deduplicar")
	fs.StringVar(&e.Ambito, "ambito", "", "glob de rutas donde aplica")
	fs.StringVar(&e.Patron, "patron", "", "patrón del error")
	fs.StringVar(&e.Tarea, "tarea", "", "tarea T-n")
	fs.StringVar(&e.Agente, "agente", "", "agentId")
	fs.StringVar(&e.Rol, "rol", "", "rol")
	fs.StringVar(&e.DetectadoPor, "detectado-por", "", "verificador que lo detectó")
	fs.StringVar(&e.Leccion, "leccion", "", "lección implicada L-n")
	escapo := fs.Bool("escapo", false, "ningún verificador lo detectó")
	if _, err := parsearIntercalado(fs, args[1:]); err != nil {
		return usoInvalido
	}
	if args[0] == "lista" {
		es, err := c.m.Errores(e.Clase)
		if err != nil {
			return c.codigo(err)
		}
		if c.g.json {
			return c.emitirJSON(es)
		}
		if len(es) == 0 {
			fmt.Fprintln(c.out, "sin errores registrados")
		}
		for _, x := range es {
			fmt.Fprintf(c.out, "%s · %s · %s · clase %s · %s\n", x.ID, x.Severidad, x.Fuente, x.Clase, recortar(x.Que, 70))
		}
		return ok
	}
	if *escapo {
		e.DetectadoPor = "escapó"
	}
	id, rec, accion, err := aprendizaje.RegistrarError(c.m, c.raiz, e)
	if err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "%s · clase %s · recurrencia %d\n", id, e.Clase, rec)
	if accion != "" {
		fmt.Fprintln(c.out, accion)
	}
	if rec >= 2 {
		fmt.Fprintf(c.out, "la clase %s se repite: el curador debe crear o revisar su lección\n", e.Clase)
	}
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

// ───────────────────────── leccion ─────────────────────────

func (c contexto) leccion(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz leccion nueva|estado|eliminar|lista|para …")
		return usoInvalido
	}
	fs := c.flagSet("leccion " + args[0])
	clase := fs.String("clase", "", "clase de error")
	skill := fs.String("skill", "", "nombre de la skill hz-leccion-<x>")
	descripcion := fs.String("descripcion", "", "una línea")
	motivo := fs.String("motivo", "", "por qué")
	estado := fs.String("estado", "", "filtrar por estado")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil {
		return usoInvalido
	}
	switch args[0] {
	case "nueva":
		id, err := aprendizaje.NuevaLeccion(c.m, c.raiz, motor.Leccion{Clase: *clase, Skill: *skill, Descripcion: *descripcion})
		if err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · en_prueba · faltan sus evals (hz eval registrar %s …)\n", id, id)
	case "estado":
		if len(pos) != 2 {
			fmt.Fprintln(c.err, "uso: hz leccion estado L-n <nuevo> --motivo \"…\"")
			return usoInvalido
		}
		if err := aprendizaje.CambiarEstado(c.m, pos[0], pos[1], *motivo); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s → %s\n", strings.ToUpper(pos[0]), pos[1])
	case "eliminar":
		if len(pos) != 1 || *motivo == "" {
			fmt.Fprintln(c.err, "uso: hz leccion eliminar L-n --motivo \"…\"")
			return usoInvalido
		}
		if err := aprendizaje.Eliminar(c.m, c.raiz, pos[0], *motivo); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s eliminada de raíz (skill borrada, decisión registrada)\n", strings.ToUpper(pos[0]))
	case "lista":
		ls, err := c.m.Lecciones(*estado)
		if err != nil {
			return c.codigo(err)
		}
		return c.listarLecciones(ls)
	case "para":
		if len(pos) != 1 {
			fmt.Fprintln(c.err, "uso: hz leccion para <ruta>")
			return usoInvalido
		}
		activas, err := c.m.LeccionesActivas()
		if err != nil {
			return c.codigo(err)
		}
		return c.listarLecciones(aprendizaje.ParaRuta(activas, pos[0], exportar.Coincide))
	default:
		fmt.Fprintf(c.err, "subcomando desconocido: leccion %s\n", args[0])
		return usoInvalido
	}
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

func (c contexto) listarLecciones(ls []motor.Leccion) int {
	if c.g.json {
		return c.emitirJSON(ls)
	}
	if len(ls) == 0 {
		fmt.Fprintln(c.out, "sin lecciones")
	}
	for _, l := range ls {
		fmt.Fprintf(c.out, "%s · %s · %s · ámbito %s · %s\n", l.ID, l.Estado, l.Skill, l.Ambito, recortar(l.Descripcion, 60))
	}
	return ok
}

// ───────────────────────── eval ─────────────────────────

func (c contexto) eval(args []string) int {
	if len(args) < 2 || (args[0] != "registrar" && args[0] != "decidir") {
		fmt.Fprintln(c.err, "uso: hz eval registrar L-n --tipo T --resultado pasa|falla [--costo usd] | hz eval decidir L-n [--aplicar]")
		return usoInvalido
	}
	fs := c.flagSet("eval " + args[0])
	tipo := fs.String("tipo", "", "prevencion|interferencia|activacion")
	resultado := fs.String("resultado", "", "pasa|falla")
	costo := fs.Float64("costo", 0, "costo en USD")
	detalle := fs.String("detalle", "", "detalle")
	aplicar := fs.Bool("aplicar", false, "aplicar la decisión")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil || len(pos) != 1 {
		return usoInvalido
	}
	id := strings.ToUpper(pos[0])
	if args[0] == "registrar" {
		if err := c.m.RegistrarEval(id, *tipo, *resultado, *costo, *detalle); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · %s · %s\n", id, *tipo, *resultado)
		return ok
	}
	l, err := c.m.Leccion(id)
	if err != nil {
		return c.codigo(err)
	}
	evals, err := c.m.UltimosEvals(id)
	if err != nil {
		return c.codigo(err)
	}
	criticos, err := c.m.ErroresCriticosDeLeccion(id)
	if err != nil {
		return c.codigo(err)
	}
	d := aprendizaje.Decidir(evals, l.Reescrituras, criticos)
	fmt.Fprintf(c.out, "%s → %s (%s)\n", id, d.Accion, d.Motivo)
	if !*aplicar {
		return ok
	}
	if err := aprendizaje.Aplicar(c.m, c.raiz, id, d); err != nil {
		return c.codigo(err)
	}
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

// ───────────────────────── mutar ─────────────────────────

func (c contexto) mutar(args []string) int {
	fs := c.flagSet("mutar")
	archivo := fs.String("archivo", "", "archivo fuente a mutar")
	tarea := fs.String("tarea", "", "tarea T-n a la que se asocia la medición (la exige hz salida --rol verificador)")
	cmd := fs.String("cmd", c.perfil.Comandos["test"], "comando de la suite")
	max := fs.Int("max", 20, "máximo de mutantes (muestra repartida en todo el archivo)")
	limite := fs.Duration("timeout", 2*time.Minute, "tiempo límite por corrida")
	if _, err := parsearIntercalado(fs, args); err != nil || *archivo == "" {
		fmt.Fprintln(c.err, "uso: hz mutar --archivo <src> [--tarea T-n] [--cmd \"…\"] [--max 20] [--timeout 2m]")
		return usoInvalido
	}
	rel := *archivo
	if filepath.IsAbs(rel) {
		if r, err := filepath.Rel(c.raiz, rel); err == nil {
			rel = r
		}
	}
	rel = path.Clean(filepath.ToSlash(rel))
	idTarea := ""
	if *tarea != "" {
		t, err := c.m.Tarea(*tarea)
		if err != nil {
			return c.codigo(err)
		}
		idTarea = t.ID // el mismo id con que hz salida busca la medición
	}
	r, err := mutar.Ejecutar(c.raiz, rel, *cmd, *max, *limite)
	for _, a := range r.Avisos { // huérfanos de corridas interrumpidas: se avisan aunque la medición falle
		fmt.Fprintln(c.err, "aviso: "+a)
	}
	if err != nil {
		return c.codigo(err)
	}
	var sobrev []string
	for _, s := range r.Sobrevivientes {
		sobrev = append(sobrev, fmt.Sprintf("%d:%d %q→%q", s.Linea, s.Col, strings.TrimSpace(s.Original), strings.TrimSpace(s.Mutante)))
	}
	detalle := fmt.Sprintf("%d de %d mutantes detectados en %s", r.Detectados, r.Total, rel)
	if r.Total == 0 {
		detalle = "sin sitios mutables en " + rel
	}
	ids := []string{"mutacion:" + rel}
	if idTarea != "" {
		ids = append(ids, "mutacion:"+idTarea+":"+rel)
	}
	for _, id := range ids {
		if err := c.m.RegistrarMedicion(id, r.Tasa, detalle); err != nil {
			return c.codigo(err)
		}
	}
	if c.g.json {
		return c.emitirJSON(r)
	}
	fmt.Fprintf(c.out, "detección %.0f%% · %s\n", r.Tasa*100, detalle)
	for _, s := range sobrev {
		fmt.Fprintf(c.out, "  sobrevive %s → candidato a test_debil\n", s)
	}
	return ok
}

// ───────────────────────── redteam ─────────────────────────

func (c contexto) redteam(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz redteam plan [RD-n] | registrar RD-n --caso rt-… --resultado bloqueado|paso | estado")
		return usoInvalido
	}
	reglas, err := proyecto.CargarReglas(c.raiz)
	if err != nil {
		return c.codigo(err)
	}
	fs := c.flagSet("redteam " + args[0])
	caso := fs.String("caso", "", "caso rt-…")
	resultado := fs.String("resultado", "", "bloqueado|paso")
	capa := fs.String("capa", "", "capa que actuó")
	detalle := fs.String("detalle", "", "qué pasó")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil {
		return usoInvalido
	}
	switch args[0] {
	case "plan":
		encontradas := 0
		for _, r := range proyecto.Vigentes(reglas) {
			if len(pos) == 1 && !strings.EqualFold(r.ID, pos[0]) {
				continue
			}
			encontradas++
			fmt.Fprintf(c.out, "%s · %s\n", r.ID, r.Orden)
			for _, k := range redteam.Plan(r) {
				fmt.Fprintf(c.out, "  %s [%s] %s\n", k.ID, k.Herramienta, k.Intento)
			}
		}
		if encontradas == 0 {
			fmt.Fprintln(c.err, "sin reglas duras vigentes que coincidan")
			return falloVerif
		}
		fmt.Fprintln(c.out, "Correr en un worktree aislado y registrar cada caso con `hz redteam registrar`.")
		return ok
	case "registrar":
		if len(pos) != 1 {
			return usoInvalido
		}
		regla := strings.ToUpper(pos[0])
		if err := c.m.RegistrarRedteam(regla, *caso, *resultado, *capa, *detalle); err != nil {
			return c.codigo(err)
		}
		eventos, err := c.m.EventosRedteam(regla)
		if err != nil {
			return c.codigo(err)
		}
		ef := redteam.Estado(eventos)
		fmt.Fprintf(c.out, "%s → %s (%d bloqueados, %d pasaron)\n", regla, ef.Estado, ef.Bloqueados, ef.Pasados)
		if *resultado == "paso" {
			id, _, _, err := aprendizaje.RegistrarError(c.m, c.raiz, motor.Error{
				Que: fmt.Sprintf("%s evadida por el caso %s: %s", regla, *caso, *detalle), Fuente: "regla_dura", Severidad: "critico",
				Causa: "otra", Clase: "rd-inefectiva-" + strings.ToLower(regla), Patron: *caso,
			})
			if err != nil {
				return c.codigo(err)
			}
			fmt.Fprintf(c.out, "ALERTA: %s dejó de ser efectiva; error crítico %s registrado. Refuerza la regla (otra capa o allowlist) y vuelve a correr el red-team.\n", regla, id)
		}
		return ok
	case "estado":
		for _, r := range reglas {
			eventos, err := c.m.EventosRedteam(r.ID)
			if err != nil {
				return c.codigo(err)
			}
			ef := redteam.Estado(eventos)
			linea := fmt.Sprintf("%s · %s · %d casos · %d bloqueados · %d pasaron", r.ID, ef.Estado, ef.Casos, ef.Bloqueados, ef.Pasados)
			if ef.Motivo != "" {
				linea += " · " + ef.Motivo
			}
			fmt.Fprintln(c.out, linea)
		}
		return ok
	}
	fmt.Fprintf(c.err, "subcomando desconocido: redteam %s\n", args[0])
	return usoInvalido
}
