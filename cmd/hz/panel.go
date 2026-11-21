package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"hz528/internal/dashboard"
)

// Incrementos 3.6 y 3.7 · observabilidad (contratos/cli.md, contratos/plugin.md §7). La lógica (árbol,
// resumen, lectura del snapshot) vive en internal/dashboard y la comparten el panel y el dashboard web.

const maxFilasPanel = 30

func (c contexto) panel(args []string) int {
	if len(args) > 0 {
		fmt.Fprintln(c.err, "uso: hz panel [--json]")
		return usoInvalido
	}
	d, err := dashboard.Reunir(c.m, c.raiz)
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		salida := map[string]any{"generado": time.Now().Format(time.RFC3339), "actividad_generada": "",
			"agentes": []dashboard.Agente{}, "tareas": d.Tareas, "arriendos": d.Arriendos, "misiones": d.Misiones}
		if d.Actividad != nil {
			salida["actividad_generada"], salida["agentes"] = d.Actividad.Generado, d.Actividad.Agentes
		}
		return c.emitirJSON(salida)
	}
	fmt.Fprintf(c.out, "%s · %s · %s\n", d.Proyecto, d.Nivel, d.Modo)
	for _, m := range d.Misiones {
		linea := fmt.Sprintf("misión %s · %s", m.ID, usdTxt(m.Gasto))
		if m.UsdMax > 0 {
			linea += fmt.Sprintf(" de %s (%.0f%%)", usdTxt(m.UsdMax), m.Gasto/m.UsdMax*100)
		}
		if m.SinPresupuesto {
			linea += " · sin presupuesto: subagentes frenados"
		}
		fmt.Fprintln(c.out, linea)
	}
	tareas := make([]string, 0, len(d.Tareas))
	for _, t := range d.Tareas {
		tareas = append(tareas, t.ID+" "+strings.ToUpper(t.Estado))
	}
	if len(tareas) == 0 {
		tareas = []string{"ninguna"}
	}
	fmt.Fprintf(c.out, "tareas TDD: %s · arriendos: %d\n", recortar(strings.Join(tareas, ", "), 90), d.Arriendos)
	if d.Actividad == nil || len(d.Actividad.Agentes) == 0 {
		fmt.Fprintln(c.out, "sin actividad registrada (el plugin la escribe en "+dashboard.ArchivoActividad+")")
		return ok
	}
	ahora := time.Now()
	fmt.Fprintf(c.out, "agentes (snapshot %s):\n", d.Actividad.Generado)
	nodos := dashboard.Arbol(d.Actividad.Agentes)
	for i, n := range nodos {
		if i == maxFilasPanel {
			fmt.Fprintf(c.out, "… y %d más (hz panel --json)\n", len(nodos)-maxFilasPanel)
			break
		}
		a := n.Agente
		fmt.Fprintf(c.out, "%s%s %s · %s\n", strings.Repeat("  ", n.Nivel), a.Glifo(), strings.TrimSpace(a.Rol+" "+a.Tarea), dashboard.Resumen(a, ahora))
	}
	return ok
}

// dashboard sirve el panel web local en 127.0.0.1 hasta que se interrumpa (contratos/cli.md · 3.7).
func (c contexto) dashboard(args []string) int {
	fs := c.flagSet("dashboard")
	puerto := fs.Int("puerto", 5280, "puerto local (solo 127.0.0.1)")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || *puerto < 1 || *puerto > 65535 {
		fmt.Fprintln(c.err, "uso: hz dashboard [--puerto 5280]")
		return usoInvalido
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*puerto)))
	if err != nil {
		return c.codigo(fmt.Errorf("no se pudo escuchar en 127.0.0.1:%d: %w", *puerto, err))
	}
	srv := &http.Server{Handler: dashboard.Servidor(c.m, c.raiz, *puerto), ReadHeaderTimeout: 5 * time.Second}
	apagar := func() { _ = srv.Shutdown(context.Background()) }
	// Ctrl+C apaga ordenadamente: termina las peticiones en curso y libera el motor con el defer de run.
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	go func() {
		<-ctx.Done()
		apagar()
	}()
	fmt.Fprintf(c.out, "hz dashboard en http://127.0.0.1:%d (solo este equipo, solo lectura; Ctrl+C para salir)\n", *puerto)
	if alEscuchar != nil {
		go alEscuchar(ln.Addr().String(), apagar)
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return c.codigo(err)
	}
	fmt.Fprintln(c.out, "hz dashboard detenido")
	return ok
}

// alEscuchar, si no es nil, recibe la dirección ya escuchando y cómo apagar el servidor (lo usan los tests).
var alEscuchar func(dir string, apagar func())
