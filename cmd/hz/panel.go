package main

import (
	"fmt"
	"strings"
	"time"

	"hz528/internal/dashboard"
)

// Incrementos 3.6 y 3.8 · observabilidad (contratos/cli.md, contratos/plugin.md §7). La lógica (árbol,
// resumen, lectura de las sesiones) vive en internal/dashboard y la comparten el panel y el dashboard web.

const maxFilasPanel = 30

// agenteDeSesion es un agente del panel --json con la sesión a la que pertenece.
type agenteDeSesion struct {
	dashboard.Agente
	Sesion string `json:"sesion"`
}

func (c contexto) panel(args []string) int {
	if len(args) > 0 {
		fmt.Fprintln(c.err, "uso: hz panel [--json]")
		return usoInvalido
	}
	ahora := time.Now()
	p, err := dashboard.Reunir(c.m, c.raiz, ahora)
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.panelJSON(p, ahora)
	}
	fmt.Fprintf(c.out, "%s · %s · %s\n", p.Nombre, p.Nivel, p.Modo)
	for _, m := range p.Misiones {
		linea := fmt.Sprintf("misión %s · %s", m.ID, usdTxt(m.Gasto))
		if m.UsdMax > 0 {
			linea += fmt.Sprintf(" de %s (%.0f%%)", usdTxt(m.UsdMax), m.Gasto/m.UsdMax*100)
		}
		if m.SinPresupuesto {
			linea += " · sin presupuesto: subagentes frenados"
		}
		fmt.Fprintln(c.out, linea)
	}
	tareas := make([]string, 0, len(p.Tareas))
	for _, t := range p.Tareas {
		tareas = append(tareas, t.ID+" "+strings.ToUpper(t.Estado))
	}
	if len(tareas) == 0 {
		tareas = []string{"ninguna"}
	}
	fmt.Fprintf(c.out, "tareas TDD: %s · arriendos: %d\n", recortar(strings.Join(tareas, ", "), 90), p.Arriendos)
	if len(p.Sesiones) == 0 {
		fmt.Fprintln(c.out, "sin actividad registrada (el plugin la escribe en "+dashboard.DirSesiones+"/)")
		return ok
	}
	filas := 0
	for i, s := range p.Sesiones {
		nodos := dashboard.Arbol(s.Agentes)
		if filas+1+len(nodos) > maxFilasPanel && filas > 0 {
			fmt.Fprintf(c.out, "… y %d sesiones más (hz panel --json)\n", len(p.Sesiones)-i)
			break
		}
		fmt.Fprintf(c.out, "sesión %s · %s\n", s.Corto(), s.EstadoTexto())
		filas++
		for j, n := range nodos {
			if filas == maxFilasPanel {
				fmt.Fprintf(c.out, "  … y %d más (hz panel --json)\n", len(nodos)-j)
				break
			}
			a := n.Agente
			fmt.Fprintf(c.out, "  %s%s %s · %s\n", strings.Repeat("  ", n.Nivel), a.Glifo(), strings.TrimSpace(a.Rol+" "+a.Tarea), dashboard.Resumen(a, ahora))
			filas++
		}
	}
	return ok
}

func (c contexto) panelJSON(p dashboard.Proyecto, ahora time.Time) int {
	sesiones := make([]map[string]any, 0, len(p.Sesiones))
	agentes := []agenteDeSesion{}
	ultima := ""
	for _, s := range p.Sesiones {
		sesiones = append(sesiones, map[string]any{"id": s.ID, "generado": s.Generado, "terminada": s.Terminada, "inactiva": s.Inactiva, "agentes": s.Agentes})
		for _, a := range s.Agentes {
			agentes = append(agentes, agenteDeSesion{Agente: a, Sesion: s.ID})
		}
		if s.Generado > ultima {
			ultima = s.Generado
		}
	}
	return c.emitirJSON(map[string]any{"generado": ahora.Format(time.RFC3339), "actividad_generada": ultima,
		"sesiones": sesiones, "agentes": agentes, "tareas": p.Tareas, "arriendos": p.Arriendos, "misiones": p.Misiones})
}
