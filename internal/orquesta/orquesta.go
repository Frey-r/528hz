// Package orquesta implementa la parte determinista de la orquestación por roles (docs/diseno/11-orquestacion.md):
// frente listo del tablero, paquete de rol (la porción del harness de cada rol) y criterio de salida.
package orquesta

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"hz528/internal/check"
	"hz528/internal/motor"
	"hz528/internal/mutar"
	"hz528/internal/proyecto"
	"hz528/internal/tdd"
)

// posteriores al VERDE: una dependencia en estos estados ya no bloquea (una superada pasó por VERDE).
var despuesDeVerde = map[string]bool{"verde": true, "refactor": true, "verificada": true, "cerrada": true, "superada": true}

// FrenteListo devuelve las tareas en CONTRATO cuyas dependencias están en VERDE o después (D-0028).
func FrenteListo(tareas []motor.Tarea) []motor.Tarea {
	estado := make(map[string]string, len(tareas))
	for _, t := range tareas {
		estado[t.ID] = t.Estado
	}
	var listo []motor.Tarea
	for _, t := range tareas {
		if t.Estado != "contrato" {
			continue
		}
		libre := true
		for _, dep := range t.BloqueadaPor {
			if !despuesDeVerde[estado[strings.ToUpper(dep)]] {
				libre = false
				break
			}
		}
		if libre {
			listo = append(listo, t)
		}
	}
	return listo
}

// rol describe la porción del harness que recibe cada rol (D-0014). %[1]s = ID de la tarea.
type rol struct {
	responsable string
	comandos    []string
	noTeToca    string
	salida      string
}

var roles = map[string]rol{
	"contratista": {"Definir el contrato de la tarea: firmas, esquemas, criterios de aceptación y presupuesto de recursos (RG-1).",
		[]string{"hz tdd contrato %[1]s --titulo \"…\" --contrato contratos/<x>.md", "hz decision buscar <tema>", "hz regla ver RG-1"},
		"Tests y código de producción.", "el contrato existe y la tarea está registrada"},
	"tester": {"Escribir los tests desde el contrato y los stubs con las firmas, para que fallen por aserción.",
		[]string{"hz tdd rojo %[1]s --tests <rutas>", "hz tdd estado %[1]s"},
		"Implementar la lógica real (solo stubs con las firmas).", "la tarea está en ROJO con sus tests bloqueados"},
	"implementador": {"Escribir el código que hace pasar los tests bloqueados, eficiente en recursos.",
		[]string{"hz tdd verde %[1]s", "hz tdd refactor %[1]s", "hz regla ver RG-1", "hz decision nueva … (si eliges entre alternativas)"},
		"Tocar tests bloqueados ni archivos fuera de tus arriendos. No verificas ni evalúas: eso lo hacen otros roles.",
		"la tarea está en VERDE o REFACTOR y los tests bloqueados siguen intactos"},
	"verificador": {"Correr la escalera de verificación V0–V3 y los sensores RG-1, medir con mutation testing que los tests detecten fallas (no solo que pasen), dejar la evidencia y registrar errores.",
		[]string{"hz check", "hz tdd estado %[1]s", "hz mutar --archivo <src> --tarea %[1]s (por cada archivo de código que tocó la tarea)", "comandos del proyecto en .harness/harness.yaml"},
		"Editar código o tests.", "la tarea tiene evidencia ejecutada en verde, `hz check` pasa y cada archivo de código tocado supera el umbral de `hz mutar`"},
	"evaluador": {"Evaluar con contexto fresco y solo lectura: contratos, RG-1, reglas duras y holdout.",
		[]string{"hz tdd estado %[1]s", "hz regla ver RG-1"}, "Editar nada (solo lectura).", "registraste tu tarjeta de veredicto"},
	"registrador": {"Registrar decisiones, avance y errores; proponer las decisiones que nadie registró (D-0035).",
		[]string{"hz decision buscar <tema>", "hz decision nueva …"}, "Editar código.", "registraste tu tarjeta"},
	"integrador": {"Integrar las células terminadas una a una y correr la suite completa.",
		[]string{"hz tablero", "hz check"}, "Cambiar el comportamiento de una célula sin hablar con su líder.", "registraste tu tarjeta de integración"},
	"curador": {"Convertir errores en lecciones y evaluarlas (A/B).", []string{"hz decision nueva …"},
		"Activar una lección sin eval.", "registraste el estado de las lecciones"},
	"diagnosticador": {"Analizar un cluster de fallos (solo lectura).", []string{"hz decision buscar <tema>"},
		"Editar código.", "registraste tu diagnóstico"},
	"medidor":  {"Correr los sensores de las verticales y comparar.", []string{"hz tablero"}, "Editar código.", "registraste tus mediciones"},
	"lider":    {"Supervisar la cadena de roles de tu célula: validar cada tarjeta con `hz salida` y lanzar al sucesor solo con su paquete y la tarjeta.", []string{"hz tablero --celula <c>", "hz contexto --rol <r> --tarea %[1]s", "hz salida --rol <r> --tarea %[1]s", "hz tarjeta ultima %[1]s"}, "Implementar o verificar tú mismo.", "registraste la tarjeta de la célula"},
	"director": {"Planificar el enjambre (capacidad, frente listo), pedir el gate humano y supervisar a los líderes.", []string{"hz capacidad", "hz tablero --listo", "hz estado"}, "Editar código.", "registraste la tarjeta de cierre"},
	"oraculo":  {"Revisar un diff de alto riesgo con un modelo más capaz.", []string{"hz tdd estado %[1]s"}, "Editar nada.", "registraste tu veredicto"},
}

// Roles devuelve los roles conocidos, ordenados.
func Roles() []string {
	out := make([]string, 0, len(roles))
	for r := range roles {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// ErrRolDesconocido indica un rol que no existe.
var ErrRolDesconocido = errors.New("rol desconocido")

// Paquete arma la porción del harness para un rol y una tarea (≤40 líneas).
func Paquete(nombre string, t motor.Tarea, previa *motor.Tarjeta, arriendos []motor.Arriendo) (string, error) {
	r, ok := roles[nombre]
	if !ok {
		return "", fmt.Errorf("%w %q; roles: %s", ErrRolDesconocido, nombre, strings.Join(Roles(), ", "))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "PAQUETE DE ROL · %s · tarea %s «%s» (estado %s)\n", nombre, t.ID, t.Titulo, strings.ToUpper(t.Estado))
	fmt.Fprintf(&b, "Te toca: %s\n", r.responsable)
	if t.ContratoRuta != "" {
		fmt.Fprintf(&b, "Contrato: %s\n", t.ContratoRuta)
	}
	if len(t.TestsBloqueados) > 0 {
		rutas := make([]string, 0, len(t.TestsBloqueados))
		for _, tb := range t.TestsBloqueados {
			rutas = append(rutas, tb.Ruta)
		}
		fmt.Fprintf(&b, "Tests bloqueados (solo lectura): %s\n", strings.Join(rutas, ", "))
	}
	var propios []string
	for _, a := range arriendos {
		if strings.EqualFold(a.Tarea, t.ID) {
			propios = append(propios, a.Ruta)
		}
	}
	if len(propios) > 0 {
		fmt.Fprintf(&b, "Tus archivos (arriendos): %s\n", strings.Join(propios, ", "))
	}
	if previa != nil {
		fmt.Fprintf(&b, "Tarjeta del predecesor (%s, %s):\n", previa.ID, previa.Rol)
		lineas := strings.Split(strings.TrimRight(previa.Texto, "\n"), "\n")
		if len(lineas) > 12 {
			lineas = append(lineas[:12], "… (completa: hz tarjeta ultima "+t.ID+")")
		}
		for _, l := range lineas {
			fmt.Fprintf(&b, "  %s\n", l)
		}
	}
	comandos := make([]string, 0, len(r.comandos))
	for _, c := range r.comandos {
		if strings.Contains(c, "%[1]s") { // solo los comandos que llevan la tarea se formatean
			c = fmt.Sprintf(c, t.ID)
		}
		comandos = append(comandos, c)
	}
	fmt.Fprintf(&b, "Tus comandos: %s\n", strings.Join(comandos, " · "))
	fmt.Fprintf(&b, "No te toca: %s\n", r.noTeToca)
	fmt.Fprintf(&b, "Para entregar: %s. Compruébalo con `hz salida --rol %s --tarea %s` y registra tu tarjeta (≤30 líneas): "+
		"`hz tarjeta registrar %s --rol %s --estado listo --texto \"…\"`.\n", r.salida, nombre, t.ID, t.ID, nombre)
	b.WriteString("Si te falta contexto, pídelo en este orden: hz → AGENTS.md del dominio → SendMessage a tu supervisor. No cargues archivos enteros por si acaso.\n")
	return b.String(), nil
}

func noSale(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{tdd.ErrVerificacion}, args...)...)
}

// Salida verifica de forma determinista si un rol cumplió su criterio de salida (D-0015).
func Salida(m *motor.Motor, raiz, nombre, tarea string) error {
	_, err := SalidaConNota(m, raiz, nombre, tarea)
	return err
}

// SalidaConNota es Salida más una nota para quien la pide; hoy solo la lleva una tarea superada, cuyos
// tests bloqueados ya no se verifican (los reemplazó otra tarea con una decisión registrada).
func SalidaConNota(m *motor.Motor, raiz, nombre, tarea string) (string, error) {
	if _, ok := roles[nombre]; !ok {
		return "", fmt.Errorf("%w %q", ErrRolDesconocido, nombre)
	}
	t, err := m.Tarea(tarea)
	if err != nil {
		return "", noSale("la tarea %s no existe", tarea)
	}
	if s := t.Superacion; t.Estado == "superada" && s != nil && (nombre == "implementador" || nombre == "verificador") {
		return fmt.Sprintf("superada por %s (%s): sus tests bloqueados ya no se verifican; la verificación vigente es la de %s", s.Por, s.Decision, s.Por), nil
	}
	return "", salidaPorRol(m, raiz, nombre, t)
}

func salidaPorRol(m *motor.Motor, raiz, nombre string, t motor.Tarea) error {
	switch nombre {
	case "contratista":
		if _, err := os.Stat(filepath.Join(raiz, t.ContratoRuta)); err != nil {
			return noSale("el contrato %s no existe", t.ContratoRuta)
		}
		return nil
	case "tester":
		if t.Estado == "contrato" || len(t.TestsBloqueados) == 0 {
			return noSale("la tarea %s está en %s sin tests bloqueados: falta `hz tdd rojo`", t.ID, strings.ToUpper(t.Estado))
		}
		return nil
	case "implementador":
		if !despuesDeVerde[t.Estado] {
			return noSale("la tarea %s está en %s: falta `hz tdd verde`", t.ID, strings.ToUpper(t.Estado))
		}
		return testsIntactos(raiz, t)
	case "verificador":
		if !despuesDeVerde[t.Estado] {
			return noSale("la tarea %s aún no está en VERDE", t.ID)
		}
		verde := false
		for _, e := range t.Evidencia {
			if e.Resultado == "exit 0" {
				verde = true
			}
		}
		if !verde {
			return noSale("la tarea %s no tiene evidencia ejecutada en verde", t.ID)
		}
		perfil, err := proyecto.Cargar(raiz)
		if err != nil {
			return err
		}
		if !check.TodoOK(check.Ejecutar(raiz, perfil)) {
			return noSale("`hz check` falla: corrígelo antes de entregar")
		}
		if err := testsIntactos(raiz, t); err != nil {
			return err
		}
		return mutacionSuficiente(m, perfil.UmbralMutacion(), t)
	default:
		tj, err := m.UltimaTarjeta(t.ID, nombre)
		if err != nil || tj.Estado != "listo" {
			return noSale("falta la tarjeta del rol %s en estado listo para %s", nombre, t.ID)
		}
		return nil
	}
}

// mutacionSuficiente es la meta-eval de los tests de la tarea (D-0046): que pasen no basta, tienen que detectar
// fallas sembradas en el código que la tarea tocó.
func mutacionSuficiente(m *motor.Motor, umbral float64, t motor.Tarea) error {
	if umbral <= 0 {
		return nil
	}
	prefijo := "mutacion:" + t.ID + ":"
	meds, err := m.MedicionesConPrefijo(prefijo)
	if err != nil {
		return err
	}
	if len(meds) == 0 {
		return noSale("falta la meta-eval de los tests de %s: corre `hz mutar --archivo <src> --tarea %s` por cada archivo de código que tocó (detección mínima %.0f%%)",
			t.ID, t.ID, umbral*100)
	}
	medidos := make(map[string]bool, len(meds))
	var bajos []string
	for _, x := range meds {
		archivo := strings.TrimPrefix(x.ID, prefijo)
		medidos[path.Clean(archivo)] = true
		if x.Deteccion < umbral-1e-9 {
			bajos = append(bajos, fmt.Sprintf("%s %.0f%%", archivo, x.Deteccion*100))
		}
	}
	if len(bajos) > 0 {
		return noSale("la suite de %s no detecta suficientes fallas sembradas (mínimo %.0f%%): %s. Refuerza los tests con los sobrevivientes de `hz mutar` y registra el error test_debil",
			t.ID, umbral*100, strings.Join(bajos, ", "))
	}
	arriendos, err := m.Arriendos()
	if err != nil {
		return err
	}
	bloqueados := make(map[string]bool, len(t.TestsBloqueados))
	for _, tb := range t.TestsBloqueados {
		bloqueados[path.Clean(filepath.ToSlash(tb.Ruta))] = true
	}
	var faltan []string
	for _, a := range arriendos {
		ruta := path.Clean(filepath.ToSlash(a.Ruta))
		if a.Tarea == t.ID && !bloqueados[ruta] && !medidos[ruta] && mutar.EsCodigoDeProduccion(ruta) {
			faltan = append(faltan, ruta)
		}
	}
	if len(faltan) > 0 {
		return noSale("archivos de código arrendados por %s sin meta-eval: %s (`hz mutar --archivo <src> --tarea %s`)",
			t.ID, strings.Join(faltan, ", "), t.ID)
	}
	return nil
}

func testsIntactos(raiz string, t motor.Tarea) error {
	for _, tb := range t.TestsBloqueados {
		h, err := tdd.HashArchivo(filepath.Join(raiz, tb.Ruta))
		if err != nil || h != tb.SHA256 {
			return noSale("el test bloqueado %s fue modificado o borrado (RG-2.3)", tb.Ruta)
		}
	}
	return nil
}
