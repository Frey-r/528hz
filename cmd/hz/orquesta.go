package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hz528/internal/capacidad"
	"hz528/internal/exportar"
	"hz528/internal/motor"
	"hz528/internal/orquesta"
)

// ───────────────────────── tablero ─────────────────────────

func (c contexto) tablero(args []string) int {
	fs := c.flagSet("tablero")
	listo := fs.Bool("listo", false, "solo el frente listo")
	celula := fs.String("celula", "", "filtrar por célula")
	if _, err := parsearIntercalado(fs, args); err != nil {
		return usoInvalido
	}
	tareas, err := c.m.Tareas()
	if err != nil {
		return c.codigo(err)
	}
	if *listo {
		tareas = orquesta.FrenteListo(tareas)
	}
	filtradas := tareas[:0]
	for _, t := range tareas {
		if *celula == "" || strings.EqualFold(t.Celula, *celula) {
			filtradas = append(filtradas, t)
		}
	}
	if c.g.json {
		return c.emitirJSON(filtradas)
	}
	if len(filtradas) == 0 {
		fmt.Fprintln(c.out, "sin tareas")
	}
	for _, t := range filtradas {
		linea := fmt.Sprintf("%s · %s · %s", t.ID, strings.ToUpper(t.Estado), recortar(t.Titulo, 60))
		if t.Celula != "" {
			linea += " · célula " + t.Celula
		}
		if len(t.BloqueadaPor) > 0 {
			linea += " · depende de " + strings.Join(t.BloqueadaPor, ",")
		}
		fmt.Fprintln(c.out, linea)
	}
	return ok
}

// ───────────────────────── arriendo ─────────────────────────

func (c contexto) arriendo(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz arriendo tomar|soltar|lista …")
		return usoInvalido
	}
	fs := c.flagSet("arriendo " + args[0])
	agente := fs.String("agente", "", "agentId del titular")
	tarea := fs.String("tarea", "", "tarea T-n")
	ttl := fs.Duration("ttl", 30*time.Minute, "duración del arriendo")
	rutas, err := parsearIntercalado(fs, args[1:])
	if err != nil {
		return usoInvalido
	}
	for i, r := range rutas {
		rutas[i] = filepath.ToSlash(r)
	}
	switch args[0] {
	case "tomar":
		if len(rutas) == 0 || *agente == "" {
			fmt.Fprintln(c.err, "uso: hz arriendo tomar <ruta>… --agente <id> --tarea T-n [--ttl 30m]")
			return usoInvalido
		}
		if err := c.m.TomarArriendo(rutas, *agente, strings.ToUpper(*tarea), time.Now().Add(*ttl)); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "arrendado a %s por %s: %s\n", *agente, *ttl, strings.Join(rutas, ", "))
	case "soltar":
		if *agente == "" {
			fmt.Fprintln(c.err, "uso: hz arriendo soltar --agente <id> [<ruta>…]")
			return usoInvalido
		}
		if err := c.m.SoltarArriendos(*agente, rutas); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "liberado (%s)\n", *agente)
	case "lista":
		arr, err := c.m.Arriendos()
		if err != nil {
			return c.codigo(err)
		}
		if c.g.json {
			return c.emitirJSON(arr)
		}
		if len(arr) == 0 {
			fmt.Fprintln(c.out, "sin arriendos vigentes")
		}
		for _, a := range arr {
			fmt.Fprintf(c.out, "%s · %s · %s · vence %s\n", a.Ruta, a.Agente, a.Tarea, a.Expira.Local().Format("15:04"))
		}
		return ok
	default:
		fmt.Fprintf(c.err, "subcomando desconocido: arriendo %s\n", args[0])
		return usoInvalido
	}
	// los hooks aplican los arriendos desde hooks.json
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

// ───────────────────────── tarjeta ─────────────────────────

func (c contexto) tarjeta(args []string) int {
	if len(args) < 2 || (args[0] != "registrar" && args[0] != "ultima") {
		fmt.Fprintln(c.err, "uso: hz tarjeta registrar T-n --rol R --estado listo|fallo|bloqueado --texto \"…\" | hz tarjeta ultima T-n [--rol R]")
		return usoInvalido
	}
	fs := c.flagSet("tarjeta " + args[0])
	rol := fs.String("rol", "", "rol que entrega")
	estado := fs.String("estado", "listo", "listo | fallo | bloqueado")
	texto := fs.String("texto", "", "tarjeta YAML ≤30 líneas")
	agente := fs.String("agente", "", "agentId (opcional)")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil || len(pos) != 1 {
		return usoInvalido
	}
	if args[0] == "ultima" {
		t, err := c.m.UltimaTarjeta(pos[0], *rol)
		if err != nil {
			return c.codigo(err)
		}
		if c.g.json {
			return c.emitirJSON(t)
		}
		fmt.Fprintf(c.out, "%s · %s · %s · %s\n%s\n", t.ID, t.TareaID, t.Rol, t.Estado, t.Texto)
		return ok
	}
	id, err := c.m.NuevaTarjeta(motor.Tarjeta{TareaID: pos[0], Rol: *rol, Estado: *estado, Texto: *texto, Agente: *agente})
	if err != nil {
		return c.codigo(err)
	}
	fmt.Fprintln(c.out, id)
	return ok
}

// ───────────────────────── contexto y salida ─────────────────────────

func (c contexto) rolYTarea(nombre string, args []string) (string, string, bool) {
	fs := c.flagSet(nombre)
	rol := fs.String("rol", "", "rol")
	tarea := fs.String("tarea", "", "tarea T-n")
	if _, err := parsearIntercalado(fs, args); err != nil || *rol == "" || *tarea == "" {
		fmt.Fprintf(c.err, "uso: hz %s --rol <rol> --tarea T-n (roles: %s)\n", nombre, strings.Join(orquesta.Roles(), ", "))
		return "", "", false
	}
	return *rol, strings.ToUpper(*tarea), true
}

func (c contexto) contexto(args []string) int {
	rol, tarea, valido := c.rolYTarea("contexto", args)
	if !valido {
		return usoInvalido
	}
	t, err := c.m.Tarea(tarea)
	if err != nil {
		return c.codigo(err)
	}
	var previa *motor.Tarjeta
	if p, err := c.m.UltimaTarjeta(tarea, ""); err == nil {
		previa = &p
	} else if !errors.Is(err, motor.ErrNoExiste) {
		return c.codigo(err)
	}
	arr, err := c.m.Arriendos()
	if err != nil {
		return c.codigo(err)
	}
	paquete, err := orquesta.Paquete(rol, t, previa, arr)
	if err != nil {
		return c.codigo(err)
	}
	fmt.Fprint(c.out, paquete)
	return ok
}

func (c contexto) salida(args []string) int {
	rol, tarea, valido := c.rolYTarea("salida", args)
	if !valido {
		return usoInvalido
	}
	if err := orquesta.Salida(c.m, c.raiz, rol, tarea); err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "%s puede entregar %s\n", rol, tarea)
	return ok
}

// ───────────────────────── equipo ─────────────────────────

func (c contexto) equipo() int {
	b, err := os.ReadFile(filepath.Join(c.raiz, ".harness", "cache", "equipo.json"))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(c.out, "sin agentes registrados (el plugin los anota en su primer tool call)")
		return ok
	}
	if err != nil {
		return c.codigo(err)
	}
	var agentes []struct {
		ID, Tipo, Descripcion, Padre, Visto string
	}
	if err := json.Unmarshal(b, &agentes); err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(agentes)
	}
	for _, a := range agentes {
		padre := a.Padre
		if padre == "" {
			padre = "sesión principal"
		}
		fmt.Fprintf(c.out, "%s · %s · %s · supervisor %s\n", a.ID, a.Tipo, recortar(a.Descripcion, 50), padre)
	}
	return ok
}

// ───────────────────────── capacidad ─────────────────────────

func cmdCapacidad(g globales, args []string, out, errw io.Writer) int {
	fs := flagSetGlobal("capacidad", errw)
	agentes := fs.Int("agentes-por-celula", 3, "agentes activos simultáneos por célula")
	pico := fs.Int("pico-mb", 300, "pico de RAM de lo que corre la célula (tests, builds)")
	if err := fs.Parse(args); err != nil {
		return usoInvalido
	}
	e, err := capacidad.Medir()
	if err != nil {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	e.AgentesPorCelula, e.PicoMB = *agentes, *pico
	r := capacidad.Calcular(e)
	if g.json {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"libre_mb": r.LibreMB, "total_mb": r.TotalMB, "nucleos": r.Nucleos,
			"agentes_por_celula": r.AgentesPorCelula, "pico_mb": r.PicoMB, "limite_concurrencia": r.LimiteConcurrencia,
			"reserva_mb": r.ReservaMB, "costo_celula_mb": r.CostoCelulaMB, "por_ram": r.PorRAM, "por_concurrencia": r.PorConcurr,
			"celulas": r.Celulas, "recomendacion": r.Recomendacion})
		return ok
	}
	fmt.Fprintf(out, "RAM libre %d MB de %d MB · reserva %d MB · núcleos %d · concurrencia máx %d\n",
		r.LibreMB, r.TotalMB, r.ReservaMB, r.Nucleos, r.LimiteConcurrencia)
	fmt.Fprintf(out, "célula = %d agentes × %d MB + pico %d MB = %d MB\n", r.AgentesPorCelula, capacidad.MBPorAgente, r.PicoMB, r.CostoCelulaMB)
	fmt.Fprintf(out, "células simultáneas: %d (por RAM %d · por concurrencia %d · por núcleos %d)\n", r.Celulas, r.PorRAM, r.PorConcurr, r.Nucleos)
	if r.Recomendacion != "" {
		fmt.Fprintln(out, r.Recomendacion)
	}
	return ok
}
