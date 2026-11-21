package main

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hz528/internal/estadistica"
	"hz528/internal/exportar"
	"hz528/internal/informe"
	"hz528/internal/mision"
	"hz528/internal/motor"
	"hz528/internal/tdd"
)

// Incremento 3.5 · misiones N3 (contratos/cli.md, docs/diseno/08-misiones.md).

func usdTxt(v float64) string { return "US$" + strconv.FormatFloat(v, 'f', 2, 64) }

func numeroRonda(r string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(r), "R-"))
	return n
}

// fase actualiza estado.json para retomar; una misión sin estado.json (creada a mano) no es un error.
func (c contexto) fase(mid, fase, proximo string) {
	if err := mision.FijarFase(c.raiz, mid, fase, proximo); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		fmt.Fprintf(c.err, "aviso: no se pudo actualizar estado.json de %s: %v\n", mid, err)
	}
}

// definicionLaxa carga mision.yaml aunque sus verticales aún no validen (gasto, aceptación y cierre no las necesitan).
func (c contexto) definicionLaxa(mid string) (mision.Mision, error) {
	def, err := mision.Cargar(c.raiz, mid)
	if def.ID != "" {
		return def, nil
	}
	return def, err
}

func (c contexto) existeRonda(mid, ronda string) error {
	rs, err := c.m.Rondas(mid)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if r.ID == ronda {
			return nil
		}
	}
	return fmt.Errorf("ronda %s de %s: %w", ronda, mid, motor.ErrNoExiste)
}

// metricasDe filtra las métricas de la misión por id (vacío = todas).
func metricasDe(def mision.Mision, id string) ([]mision.Metrica, error) {
	todas := def.Metricas()
	if id == "" {
		if len(todas) == 0 {
			return nil, fmt.Errorf("%w: la misión %s no tiene verticales con métricas (defínelas con el humano en mision.yaml)", tdd.ErrVerificacion, def.ID)
		}
		return todas, nil
	}
	for _, mt := range todas {
		if mt.ID == id {
			return []mision.Metrica{mt}, nil
		}
	}
	return nil, fmt.Errorf("%w: la métrica %s no existe en la misión %s", motor.ErrCampos, id, def.ID)
}

// piso devuelve el piso de ruido conocido: el medido con `hz ruido` o, si no, el declarado en mision.yaml.
func (c contexto) piso(mid string, mt mision.Metrica) (float64, bool, error) {
	p, err := c.m.Piso(mid, mt.ID)
	switch {
	case err == nil:
		return p, true, nil
	case !errors.Is(err, motor.ErrNoExiste):
		return 0, false, err
	case mt.PisoRuido != nil:
		return *mt.PisoRuido, true, nil
	}
	return 0, false, nil
}

var conjuntos = map[string]bool{"trabajo": true, "holdout": true, "ruido": true}

func rutaItems(mid, ronda, metrica, conjunto string) string {
	return filepath.Join(".harness", "misiones", mid, "rondas", ronda, metrica+"."+conjunto+".json")
}

func (c contexto) leerItems(mid, ronda, metrica, conjunto string) ([]estadistica.Item, error) {
	b, err := os.ReadFile(filepath.Join(c.raiz, rutaItems(mid, ronda, metrica, conjunto)))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s no tiene mediciones de %s (%s): hz ronda medir %s %s", tdd.ErrVerificacion, ronda, metrica, conjunto, mid, ronda)
	}
	if err != nil {
		return nil, err
	}
	var items []estadistica.Item
	return items, json.Unmarshal(b, &items)
}

// ───────────────────────── mision ─────────────────────────

func (c contexto) misionCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz mision nueva|fase|gasto|aceptar|cerrar|lista …")
		return usoInvalido
	}
	fs := c.flagSet("mision " + args[0])
	modo := fs.String("modo", "", "genesis|mejora (por defecto, el del proyecto)")
	objetivo := fs.String("objetivo", "", "objetivo de la misión")
	usdMax := fs.Float64("usd-max", 0, "presupuesto en USD (0 = sin tope)")
	usd := fs.Float64("usd", 0, "gasto en USD")
	concepto := fs.String("concepto", "", "en qué se gastó")
	evidencia := fs.String("evidencia", "", "comando → resultado")
	proximo := fs.String("proximo", "", "próximo paso")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil {
		return usoInvalido
	}
	necesita := map[string]int{"nueva": 1, "fase": 2, "gasto": 1, "aceptar": 2, "cerrar": 1, "lista": 0}
	n, valido := necesita[args[0]]
	if !valido || len(pos) != n {
		fmt.Fprintln(c.err, "uso: hz mision nueva <id> --modo m --objetivo \"…\" [--usd-max n] | fase <id> <fase> [--proximo \"…\"] | gasto <id> --usd x | aceptar <id> <criterio> --evidencia \"…\" | cerrar <id> | lista")
		return usoInvalido
	}
	switch args[0] {
	case "nueva":
		return c.misionNueva(pos[0], *modo, *objetivo, *usdMax)
	case "fase":
		if err := mision.FijarFase(c.raiz, pos[0], pos[1], *proximo); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · fase %s\n", pos[0], pos[1])
		return ok
	case "gasto":
		return c.misionGasto(pos[0], *usd, *concepto)
	case "aceptar":
		return c.misionAceptar(pos[0], pos[1], *evidencia)
	case "cerrar":
		return c.misionCerrar(pos[0])
	}
	ms, err := c.m.Misiones()
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(ms)
	}
	if len(ms) == 0 {
		fmt.Fprintln(c.out, "sin misiones (hz mision nueva <id> …)")
	}
	for _, x := range ms {
		g, err := c.m.Gasto(x.ID)
		if err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · %s · %s · gasto %s · %s\n", x.ID, x.Modo, x.Estado, usdTxt(g), recortar(x.Objetivo, 60))
	}
	return ok
}

func (c contexto) misionNueva(id, modo, objetivo string, usdMax float64) int {
	if modo == "" {
		modo = c.perfil.Modo
	}
	if strings.TrimSpace(objetivo) == "" || usdMax < 0 {
		fmt.Fprintln(c.err, "hz mision nueva: --objetivo es obligatorio y --usd-max no puede ser negativo")
		return usoInvalido
	}
	if err := mision.Crear(c.raiz, id, modo, objetivo, usdMax); err != nil {
		return c.codigo(err)
	}
	if err := c.m.NuevaMision(motor.Mision{ID: id, Titulo: id, Modo: modo, Objetivo: objetivo}); err != nil {
		return c.codigo(err)
	}
	e, err := mision.LeerEstado(c.raiz, id)
	if err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "%s · %s · .harness/misiones/%s/mision.yaml\npróximo: %s\n", id, modo, id, e.ProximoPaso)
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

func (c contexto) misionGasto(mid string, usd float64, concepto string) int {
	if err := c.m.RegistrarGasto(mid, usd, concepto); err != nil {
		return c.codigo(err)
	}
	total, err := c.m.Gasto(mid)
	if err != nil {
		return c.codigo(err)
	}
	def, _ := c.definicionLaxa(mid)
	tope := def.Presupuesto.UsdMax
	switch {
	case tope > 0 && total >= tope:
		fmt.Fprintf(c.out, "%s gastó %s de %s · sin presupuesto: el plugin frena nuevos subagentes hasta que el humano amplíe presupuesto.usd_max\n", mid, usdTxt(total), usdTxt(tope))
	case tope > 0:
		fmt.Fprintf(c.out, "%s gastó %s de %s (%.0f%%)\n", mid, usdTxt(total), usdTxt(tope), total/tope*100)
	default:
		fmt.Fprintf(c.out, "%s gastó %s (sin tope)\n", mid, usdTxt(total))
	}
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

func (c contexto) misionAceptar(mid, criterio, evidencia string) int {
	def, err := c.definicionLaxa(mid)
	if err != nil {
		return c.codigo(err)
	}
	criterio = strings.ToUpper(criterio)
	existe := false
	for _, cr := range def.Aceptacion {
		existe = existe || strings.ToUpper(cr.ID) == criterio
	}
	if !existe {
		return c.codigo(fmt.Errorf("%w: el criterio %s no está en el contrato de %s (aceptacion en mision.yaml)", tdd.ErrVerificacion, criterio, mid))
	}
	if err := c.m.Aceptar(mid, criterio, evidencia); err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "%s · %s cumplido con evidencia: %s\n", mid, criterio, recortar(evidencia, 80))
	return ok
}

// misionCerrar aplica el contrato default-FAIL: sin evidencia para cada criterio, no hay cierre.
func (c contexto) misionCerrar(mid string) int {
	def, err := c.definicionLaxa(mid)
	if err != nil {
		return c.codigo(err)
	}
	if len(def.Aceptacion) == 0 {
		return c.codigo(fmt.Errorf("%w: el contrato de %s no tiene criterios de aceptación (default-FAIL)", tdd.ErrVerificacion, mid))
	}
	acc, err := c.m.Aceptaciones(mid)
	if err != nil {
		return c.codigo(err)
	}
	var faltan []string
	for _, cr := range def.Aceptacion {
		if acc[strings.ToUpper(cr.ID)] == "" {
			faltan = append(faltan, fmt.Sprintf("%s (%s)", cr.ID, cr.Texto))
		}
	}
	if len(faltan) > 0 {
		return c.codigo(fmt.Errorf("%w: %s no se cierra; faltan criterios con evidencia: %s · hz mision aceptar %s <criterio> --evidencia \"comando → resultado\"",
			tdd.ErrVerificacion, mid, strings.Join(faltan, "; "), mid))
	}
	if err := c.m.CerrarMision(mid); err != nil {
		return c.codigo(err)
	}
	c.fase(mid, "cerrada", "misión cerrada: informe final con hz informe "+mid)
	fmt.Fprintf(c.out, "%s cerrada · %d criterios con evidencia\n", mid, len(def.Aceptacion))
	return c.codigo(exportar.Escribir(c.m, c.raiz))
}

// ───────────────────────── ronda ─────────────────────────

func (c contexto) ronda(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz ronda nueva|medir|cerrar|lista …")
		return usoInvalido
	}
	fs := c.flagSet("ronda " + args[0])
	foco := fs.String("foco", "", "verticales separadas por coma")
	metrica := fs.String("metrica", "", "solo esta métrica")
	conjunto := fs.String("conjunto", "trabajo", "trabajo|holdout|ruido")
	veredicto := fs.String("veredicto", "", "mantener|revertir")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil {
		return usoInvalido
	}
	necesita := map[string]int{"nueva": 1, "medir": 2, "cerrar": 2, "lista": 1}
	if n, valido := necesita[args[0]]; !valido || len(pos) != n || !conjuntos[*conjunto] {
		fmt.Fprintln(c.err, "uso: hz ronda nueva <mision> [--foco V-a,V-b] | medir <mision> R-n [--metrica m] [--conjunto trabajo|holdout|ruido] | cerrar <mision> R-n --veredicto mantener|revertir | lista <mision>")
		return usoInvalido
	}
	mid := pos[0]
	switch args[0] {
	case "nueva":
		id, err := c.m.NuevaRonda(mid, dividir(*foco))
		if err != nil {
			return c.codigo(err)
		}
		c.fase(mid, "ronda "+id, fmt.Sprintf("implementar el foco de %s con TDD y medir: hz ronda medir %s %s", id, mid, id))
		fmt.Fprintf(c.out, "%s · %s", id, mid)
		if *foco != "" {
			fmt.Fprintf(c.out, " · foco %s", *foco)
		}
		fmt.Fprintln(c.out)
		return ok
	case "medir":
		return c.rondaMedir(mid, strings.ToUpper(pos[1]), *metrica, *conjunto)
	case "cerrar":
		return c.rondaCerrar(mid, strings.ToUpper(pos[1]), *veredicto)
	}
	rs, err := c.m.Rondas(mid)
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(rs)
	}
	for _, r := range rs {
		estado := r.Veredicto
		if estado == "" {
			estado = "abierta"
		}
		fmt.Fprintf(c.out, "%s · %s · foco %s\n", r.ID, estado, r.Foco)
	}
	return ok
}

// rondaMedir corre los sensores; los números salen de aquí, nunca del conteo del modelo.
func (c contexto) rondaMedir(mid, ronda, metrica, conjunto string) int {
	def, err := mision.Cargar(c.raiz, mid)
	if err != nil {
		return c.codigo(err)
	}
	if err := c.existeRonda(mid, ronda); err != nil {
		return c.codigo(err)
	}
	metricas, err := metricasDe(def, metrica)
	if err != nil {
		return c.codigo(err)
	}
	if err := os.MkdirAll(filepath.Join(c.raiz, filepath.Dir(rutaItems(mid, ronda, "x", conjunto))), 0o755); err != nil {
		return c.codigo(err)
	}
	for _, mt := range metricas {
		items, err := mision.Sensor(c.raiz, strings.ReplaceAll(mt.Sensor, "{ronda}", ronda))
		if err != nil {
			return c.codigo(fmt.Errorf("métrica %s: %w", mt.ID, err))
		}
		b, err := json.Marshal(items)
		if err != nil {
			return c.codigo(err)
		}
		rel := rutaItems(mid, ronda, mt.ID, conjunto)
		if err := os.WriteFile(filepath.Join(c.raiz, rel), b, 0o644); err != nil {
			return c.codigo(err)
		}
		valor := estadistica.Valor(items)
		if err := c.m.RegistrarMedicionRonda(motor.Medicion{Mision: mid, Ronda: ronda, Metrica: mt.ID, Conjunto: conjunto,
			Valor: valor, N: len(items), Ruta: filepath.ToSlash(rel)}); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · %s · %.3f (n=%d) · %s\n", mt.ID, ronda, valor, len(items), conjunto)
	}
	return ok
}

func (c contexto) rondaCerrar(mid, ronda, veredicto string) int {
	if err := c.existeRonda(mid, ronda); err != nil {
		return c.codigo(err)
	}
	if err := c.m.CerrarRonda(mid, ronda, veredicto); err != nil {
		return c.codigo(err)
	}
	fmt.Fprintf(c.out, "%s cerrada · %s\n", ronda, veredicto)
	if veredicto == "revertir" {
		c.fase(mid, "ronda "+ronda+" revertida", "abrir la siguiente ronda: hz ronda nueva "+mid)
		fmt.Fprintln(c.out, "ronda revertida: no es candidata a milestone")
		return ok
	}
	c.fase(mid, "ronda "+ronda+" cerrada", fmt.Sprintf("preguntar al humano si %s es milestone; luego hz ronda nueva %s", ronda, mid))
	// D-0019: el harness siempre pregunta y solo propone cuando el salto es evidente y grande.
	fmt.Fprintf(c.out, "¿Marcamos %s como milestone? Lo decide el humano: hz milestone marcar %s %s --resumen \"qué cambió\"\n", ronda, mid, ronda)
	p, _, err := c.proponer(mid, ronda)
	if err != nil {
		fmt.Fprintf(c.err, "sin propuesta automática: %v\n", err)
		return ok
	}
	if p.Propone {
		fmt.Fprintf(c.out, "Propuesta del harness: %s\n", p.Motivo)
	}
	return ok
}

// proponer evalúa si `ronda` (o la última ronda medida, si viene vacía) merece un milestone.
func (c contexto) proponer(mid, ronda string) (estadistica.Propuesta, string, error) {
	var p estadistica.Propuesta
	def, err := mision.Cargar(c.raiz, mid)
	if err != nil {
		return p, ronda, err
	}
	meds, err := c.m.Mediciones(mid, "trabajo")
	if err != nil {
		return p, ronda, err
	}
	porRonda := map[string]map[string]float64{}
	for _, md := range meds {
		if porRonda[md.Ronda] == nil {
			porRonda[md.Ronda] = map[string]float64{}
		}
		porRonda[md.Ronda][md.Metrica] = md.Valor
	}
	medidas := make([]string, 0, len(porRonda))
	for r := range porRonda {
		medidas = append(medidas, r)
	}
	sort.Slice(medidas, func(i, j int) bool { return numeroRonda(medidas[i]) < numeroRonda(medidas[j]) })
	if ronda == "" && len(medidas) > 0 {
		ronda = medidas[len(medidas)-1]
	}
	actual := porRonda[ronda]
	if len(actual) == 0 {
		return p, ronda, fmt.Errorf("%w: %s no tiene mediciones: hz ronda medir %s %s", tdd.ErrVerificacion, ronda, mid, ronda)
	}
	// Base: el último milestone anterior; si no hay, la línea base declarada o la primera ronda medida.
	base, frente := map[string]float64{}, ""
	mss, err := c.m.Milestones(mid)
	if err != nil {
		return p, ronda, err
	}
	for i := len(mss) - 1; i >= 0; i-- {
		if numeroRonda(mss[i].Ronda) < numeroRonda(ronda) {
			base, frente = porRonda[mss[i].Ronda], mss[i].ID
			break
		}
	}
	var metricas []estadistica.Metrica
	var sinPiso []string
	for _, mt := range def.Metricas() {
		if frente == "" {
			switch {
			case mt.LineaBase != nil:
				base[mt.ID] = *mt.LineaBase
			case medidas[0] != ronda:
				if v, ok := porRonda[medidas[0]][mt.ID]; ok {
					base[mt.ID] = v
				}
			}
		}
		piso, conocido, err := c.piso(mid, mt)
		if err != nil {
			return p, ronda, err
		}
		if !conocido {
			sinPiso = append(sinPiso, mt.ID)
			continue
		}
		metricas = append(metricas, estadistica.Metrica{ID: mt.ID, Direccion: mt.Direccion, Piso: piso})
	}
	if frente == "" {
		frente = "la línea base"
	}
	if len(metricas) == 0 {
		p.Motivo = fmt.Sprintf("ninguna métrica tiene piso de ruido conocido: mídelo con hz ruido %s R-a R-b", mid)
		return p, ronda, nil
	}
	p = estadistica.Proponer(base, actual, metricas, def.Estadistica.K)
	p.Motivo += " (frente a " + frente + ")"
	if len(sinPiso) > 0 {
		p.Motivo += "; sin piso de ruido, no cuentan: " + strings.Join(sinPiso, ", ")
	}
	return p, ronda, nil
}

// ───────────────────────── compare y ruido ─────────────────────────

type comparacionMetrica struct {
	Metrica string `json:"metrica"`
	estadistica.Comparacion
}

func (c contexto) comparar(mid, a, b, metrica, conjunto string, conPiso bool) ([]comparacionMetrica, error) {
	def, err := mision.Cargar(c.raiz, mid)
	if err != nil {
		return nil, err
	}
	metricas, err := metricasDe(def, metrica)
	if err != nil {
		return nil, err
	}
	out := make([]comparacionMetrica, 0, len(metricas))
	for _, mt := range metricas {
		ia, err := c.leerItems(mid, a, mt.ID, conjunto)
		if err != nil {
			return nil, err
		}
		ib, err := c.leerItems(mid, b, mt.ID, conjunto)
		if err != nil {
			return nil, err
		}
		piso := 0.0
		if conPiso {
			if piso, _, err = c.piso(mid, mt); err != nil {
				return nil, err
			}
		}
		out = append(out, comparacionMetrica{Metrica: mt.ID, Comparacion: estadistica.Comparar(ia, ib, mt.Direccion, piso)})
	}
	return out, nil
}

func (c contexto) compare(args []string) int {
	fs := c.flagSet("compare")
	metrica := fs.String("metrica", "", "solo esta métrica")
	conjunto := fs.String("conjunto", "trabajo", "trabajo|holdout|ruido")
	pos, err := parsearIntercalado(fs, args)
	if err != nil {
		return usoInvalido
	}
	if len(pos) != 3 || !conjuntos[*conjunto] {
		fmt.Fprintln(c.err, "uso: hz compare <mision> R-a R-b [--metrica m] [--conjunto trabajo|holdout|ruido]")
		return usoInvalido
	}
	a, b := strings.ToUpper(pos[1]), strings.ToUpper(pos[2])
	cs, err := c.comparar(pos[0], a, b, *metrica, *conjunto, true)
	if err != nil {
		return c.codigo(err)
	}
	if c.g.json {
		return c.emitirJSON(cs)
	}
	for _, x := range cs {
		fmt.Fprintf(c.out, "%s · %s → %s · %.3f → %.3f (%+.3f) · %d ganados / %d perdidos / %d empates · p=%.4f · %s\n",
			x.Metrica, a, b, x.ValorA, x.ValorB, x.Delta, x.Ganados, x.Perdidos, x.Empates, x.P, x.Veredicto)
	}
	return ok
}

// ruido mide el piso de ruido: dos rondas medidas sobre el mismo código.
func (c contexto) ruido(args []string) int {
	fs := c.flagSet("ruido")
	conjunto := fs.String("conjunto", "trabajo", "trabajo|holdout|ruido")
	pos, err := parsearIntercalado(fs, args)
	if err != nil {
		return usoInvalido
	}
	if len(pos) != 3 || !conjuntos[*conjunto] {
		fmt.Fprintln(c.err, "uso: hz ruido <mision> R-a R-b (dos mediciones del mismo código)")
		return usoInvalido
	}
	mid, a, b := pos[0], strings.ToUpper(pos[1]), strings.ToUpper(pos[2])
	cs, err := c.comparar(mid, a, b, "", *conjunto, false)
	if err != nil {
		return c.codigo(err)
	}
	for _, x := range cs {
		piso := math.Round(math.Abs(x.Delta)*1e6) / 1e6 // sin residuos de coma flotante (0.024999…)
		if err := c.m.FijarPiso(mid, x.Metrica, piso); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · piso de ruido %.3f (%s vs %s, n=%d)\n", x.Metrica, piso, a, b, x.Pareados)
	}
	return ok
}

// ───────────────────────── milestone ─────────────────────────

func (c contexto) milestone(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.err, "uso: hz milestone proponer|marcar|lista …")
		return usoInvalido
	}
	fs := c.flagSet("milestone " + args[0])
	resumen := fs.String("resumen", "", "qué cambió, en una línea")
	propuesta := fs.Bool("propuesta", false, "el harness lo había propuesto")
	pos, err := parsearIntercalado(fs, args[1:])
	if err != nil {
		return usoInvalido
	}
	switch {
	case args[0] == "proponer" && (len(pos) == 1 || len(pos) == 2):
		ronda := ""
		if len(pos) == 2 {
			ronda = strings.ToUpper(pos[1])
		}
		p, ronda, err := c.proponer(pos[0], ronda)
		if err != nil {
			return c.codigo(err)
		}
		if c.g.json {
			return c.emitirJSON(map[string]any{"ronda": ronda, "propone": p.Propone, "motivo": p.Motivo})
		}
		if p.Propone {
			fmt.Fprintf(c.out, "Propuesta: marcar %s como milestone · %s\nLo decide el humano: hz milestone marcar %s %s --resumen \"qué cambió\" --propuesta\n", ronda, p.Motivo, pos[0], ronda)
		} else {
			fmt.Fprintf(c.out, "Sin propuesta para %s: %s\n", ronda, p.Motivo)
		}
		return ok
	case args[0] == "marcar" && len(pos) == 2:
		id, err := c.m.MarcarMilestone(pos[0], pos[1], *resumen, *propuesta)
		if err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "%s · %s · %s\n", id, strings.ToUpper(pos[1]), *resumen)
		return ok
	case args[0] == "lista" && len(pos) == 1:
		ms, err := c.m.Milestones(pos[0])
		if err != nil {
			return c.codigo(err)
		}
		if c.g.json {
			return c.emitirJSON(ms)
		}
		if len(ms) == 0 {
			fmt.Fprintln(c.out, "sin milestones (los define el humano)")
		}
		for _, x := range ms {
			fmt.Fprintf(c.out, "%s · %s · %s · %s\n", x.ID, x.Ronda, x.Promocion, x.Resumen)
		}
		return ok
	}
	fmt.Fprintln(c.err, "uso: hz milestone proponer <mision> [R-n] | marcar <mision> R-n --resumen \"…\" [--propuesta] | lista <mision>")
	return usoInvalido
}

// ───────────────────────── informe ─────────────────────────

func (c contexto) informeCmd(args []string) int {
	fs := c.flagSet("informe")
	html := fs.String("html", "", "archivo HTML autocontenido para publicar como Artifact")
	pos, err := parsearIntercalado(fs, args)
	if err != nil {
		return usoInvalido
	}
	if len(pos) != 1 {
		fmt.Fprintln(c.err, "uso: hz informe <mision> [--json] [--html archivo]")
		return usoInvalido
	}
	def, err := mision.Cargar(c.raiz, pos[0])
	if err != nil {
		return c.codigo(err)
	}
	inf, err := informe.Construir(c.m, def)
	if err != nil {
		return c.codigo(err)
	}
	if *html != "" {
		if err := escribirHTML(*html, inf); err != nil {
			return c.codigo(err)
		}
		fmt.Fprintf(c.out, "informe → %s · publícalo como Artifact con la skill hz:informe\n", *html)
	}
	if c.g.json {
		return c.emitirJSON(inf)
	}
	if *html != "" {
		return ok
	}
	fmt.Fprintf(c.out, "%s · %s · %d rondas · %d milestones · gasto %s\n", inf.Mision, inf.Estado, inf.Rondas, len(inf.Milestones), usdTxt(inf.Gasto))
	for _, m := range inf.Metricas {
		actual := "sin datos"
		if m.Actual != nil {
			actual = strconv.FormatFloat(*m.Actual, 'f', 3, 64)
		}
		fmt.Fprintf(c.out, "  %s · %s (base %.3f, meta %.3f) · cumple: %s\n", m.ID, actual, m.Base, m.Meta, siNo(m.Cumplida))
	}
	return ok
}

func escribirHTML(ruta string, inf informe.Informe) (err error) {
	f, err := os.Create(ruta)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return informe.HTML(f, inf)
}
