// Package tdd implementa la máquina CONTRATO → ROJO → VERDE → REFACTOR (RG-2) con tests bloqueados por hash.
package tdd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"hz528/internal/exportar"
	"hz528/internal/motor"
)

// maxSalida acota la salida capturada de un comando: se conserva el final, donde están los fallos (RG-1.8).
const maxSalida = 64 * 1024

// ErrVerificacion indica que una transición no cumple su criterio; se traduce a código de salida 1.
var ErrVerificacion = errors.New("verificación fallida")

// Resultado de ejecutar un comando de test.
type Resultado struct {
	Codigo   int
	Salida   string
	Duracion time.Duration
}

// Maquina aplica las transiciones sobre el motor del proyecto.
type Maquina struct {
	M       *motor.Motor
	Raiz    string
	CmdTest string // comandos.test de harness.yaml; se usa si la transición no trae --cmd
}

// Patrones de «falla por la razón equivocada», compilados una sola vez (RG-1.4).
var razonEquivocada = []struct {
	re     *regexp.Regexp
	motivo string
}{
	{regexp.MustCompile(`\[build failed\]|undefined: |cannot find package|could not import|syntax error: `), "error de compilación"},
	{regexp.MustCompile(`ModuleNotFoundError|ImportError|SyntaxError|IndentationError|ERROR collecting`), "error de import o sintaxis"},
	{regexp.MustCompile(`Cannot find module|error TS\d{4}|ReferenceError: \S+ is not defined`), "módulo no encontrado o error de tipos"},
}

// ClasificarRojo decide si un fallo es un ROJO válido: falla, y falla por aserción.
func ClasificarRojo(r Resultado) (bool, string) {
	if r.Codigo == 0 {
		return false, "no hay ROJO: los tests pasan sin implementación"
	}
	for _, p := range razonEquivocada {
		if p.re.MatchString(r.Salida) {
			return false, "falla por la razón equivocada (" + p.motivo + "), no por aserción"
		}
	}
	return true, ""
}

// colaAcotada guarda solo los últimos `max` bytes, reutilizando su arreglo (RG-1.6).
type colaAcotada struct {
	buf []byte
	max int
}

func (c *colaAcotada) Write(p []byte) (int, error) {
	c.buf = append(c.buf, p...)
	if len(c.buf) > c.max {
		n := copy(c.buf, c.buf[len(c.buf)-c.max:])
		c.buf = c.buf[:n]
	}
	return len(p), nil
}

// ErrLimite indica que el comando superó su tiempo límite (en mutation testing cuenta como mutante detectado).
var ErrLimite = errors.New("el comando superó su tiempo límite")

// shell elige el intérprete: HZ_SHELL, luego `sh` (en Windows, el de Git y no el lanzador de WSL), luego cmd.
func shell(ctx context.Context, cmd string) *exec.Cmd {
	if s := os.Getenv("HZ_SHELL"); s != "" {
		return exec.CommandContext(ctx, s, "-c", cmd)
	}
	if sh, err := exec.LookPath("sh"); err == nil && !strings.Contains(strings.ToLower(sh), `\windows\system32`) {
		return exec.CommandContext(ctx, sh, "-c", cmd)
	}
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", cmd)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
}

// Ejecutar corre `cmd` en `dir` y captura la salida combinada acotada.
func Ejecutar(dir, cmd string) (Resultado, error) { return EjecutarConLimite(dir, cmd, 0) }

// EjecutarConLimite es Ejecutar con tiempo límite (0 = sin límite); al vencer devuelve ErrLimite.
func EjecutarConLimite(dir, cmd string, limite time.Duration) (Resultado, error) {
	ctx := context.Background()
	if limite > 0 {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithTimeout(ctx, limite)
		defer cancelar()
	}
	c := shell(ctx, cmd)
	c.Dir = dir
	c.WaitDelay = 5 * time.Second // no quedar colgado esperando las tuberías de un hijo que sobrevive
	cola := &colaAcotada{max: maxSalida}
	c.Stdout, c.Stderr = cola, cola
	inicio := time.Now()
	err := c.Run()
	r := Resultado{Salida: string(cola.buf), Duracion: time.Since(inicio)}
	if ctx.Err() == context.DeadlineExceeded {
		return r, fmt.Errorf("%w (%s): %q", ErrLimite, limite, cmd)
	}
	var salida *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &salida):
		r.Codigo = salida.ExitCode()
	default:
		return r, fmt.Errorf("ejecutar %q: %w", cmd, err)
	}
	return r, nil
}

// HashArchivo calcula el sha256 en streaming.
func HashArchivo(ruta string) (string, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ahora() string { return time.Now().Format(time.RFC3339) }

func verificacion(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrVerificacion}, args...)...)
}

// final devuelve las últimas `n` líneas de un texto, para mensajes breves.
func final(s string, n int) string {
	lineas := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lineas) > n {
		lineas = lineas[len(lineas)-n:]
	}
	return strings.Join(lineas, "\n")
}

func (q Maquina) tareaEn(id string, estados ...string) (motor.Tarea, error) {
	t, err := q.M.Tarea(id)
	if errors.Is(err, motor.ErrNoExiste) {
		return t, verificacion("la tarea %s no existe; créala con `hz tdd contrato`", id)
	}
	if err != nil {
		return t, err
	}
	for _, e := range estados {
		if t.Estado == e {
			return t, nil
		}
	}
	return t, verificacion("la tarea %s está en %s; esta transición exige %s (orden CONTRATO → ROJO → VERDE → REFACTOR)",
		id, strings.ToUpper(t.Estado), strings.ToUpper(strings.Join(estados, " o ")))
}

func (q Maquina) comando(cmd string) (string, error) {
	if cmd != "" {
		return cmd, nil
	}
	if q.CmdTest != "" {
		return q.CmdTest, nil
	}
	return "", verificacion("sin comando de test: usa --cmd o declara comandos.test en .harness/harness.yaml")
}

// Contrato crea la tarea; el contrato debe existir antes que cualquier test o código (RG-2.1).
func (q Maquina) Contrato(id, titulo, contrato string, aprobado bool, mision string) error {
	return q.ContratoConDAG(id, titulo, contrato, aprobado, mision, "", nil)
}

// ContratoConDAG crea la tarea con su célula y sus dependencias en el tablero (D-0028).
func (q Maquina) ContratoConDAG(id, titulo, contrato string, aprobado bool, mision, celula string, bloqueadaPor []string) error {
	if _, err := q.M.Tarea(id); err == nil {
		return verificacion("la tarea %s ya existe", id)
	} else if !errors.Is(err, motor.ErrNoExiste) {
		return err
	}
	if _, err := os.Stat(filepath.Join(q.Raiz, contrato)); err != nil {
		return verificacion("el contrato %s no existe: el contrato va antes que los tests (RG-2.1)", contrato)
	}
	for i, dep := range bloqueadaPor {
		bloqueadaPor[i] = strings.ToUpper(strings.TrimSpace(dep))
	}
	t := motor.Tarea{ID: strings.ToUpper(id), Titulo: titulo, Estado: "contrato", ContratoRuta: filepath.ToSlash(contrato),
		MisionID: mision, Celula: celula, BloqueadaPor: bloqueadaPor}
	if aprobado {
		t.ContratoOKEn = ahora()
	}
	if err := q.M.RegistrarTarea("tdd.contrato", t); err != nil {
		return err
	}
	return ExportarHooks(q.M, q.Raiz)
}

// Rojo exige que los tests fallen por aserción y los bloquea por hash.
func (q Maquina) Rojo(id string, tests []string, cmd string) error {
	t, err := q.tareaEn(id, "contrato")
	if err != nil {
		return err
	}
	if len(tests) == 0 {
		return verificacion("--tests es obligatorio: indica qué archivos de test quedan bloqueados")
	}
	c, err := q.comando(cmd)
	if err != nil {
		return err
	}
	bloqueados := make([]motor.TestBloqueado, 0, len(tests))
	for _, rel := range tests {
		h, err := HashArchivo(filepath.Join(q.Raiz, rel))
		if err != nil {
			return verificacion("el test %s no existe o no se puede leer", rel)
		}
		bloqueados = append(bloqueados, motor.TestBloqueado{Ruta: filepath.ToSlash(rel), SHA256: h})
	}
	r, err := Ejecutar(q.Raiz, c)
	if err != nil {
		return err
	}
	if ok, motivo := ClasificarRojo(r); !ok {
		return verificacion("%s\n%s", motivo, final(r.Salida, 12))
	}
	t.Estado, t.RojoEn, t.TestsBloqueados = "rojo", ahora(), bloqueados
	t.Evidencia = append(t.Evidencia, motor.Evidencia{Cmd: c, Resultado: fmt.Sprintf("exit %d: rojo por aserción", r.Codigo), Nivel: "V1", En: t.RojoEn})
	if err := q.M.RegistrarTarea("tdd.rojo", t); err != nil {
		return err
	}
	return ExportarHooks(q.M, q.Raiz)
}

// Verde exige tests bloqueados intactos y el comando en verde.
func (q Maquina) Verde(id, cmd string) error { return q.avanzar(id, cmd, "verde", "rojo") }

// Refactor exige lo mismo que Verde, desde VERDE o REFACTOR.
func (q Maquina) Refactor(id, cmd string) error {
	return q.avanzar(id, cmd, "refactor", "verde", "refactor")
}

func (q Maquina) avanzar(id, cmd, nuevo string, desde ...string) error {
	t, err := q.tareaEn(id, desde...)
	if err != nil {
		return err
	}
	for _, tb := range t.TestsBloqueados {
		h, err := HashArchivo(filepath.Join(q.Raiz, tb.Ruta))
		if err != nil || h != tb.SHA256 {
			return verificacion("el test bloqueado %s fue modificado o borrado; cambiarlo exige una decisión registrada y aprobación (RG-2.3)", tb.Ruta)
		}
	}
	c, err := q.comando(cmd)
	if err != nil {
		return err
	}
	r, err := Ejecutar(q.Raiz, c)
	if err != nil {
		return err
	}
	if r.Codigo != 0 {
		return verificacion("los tests fallan (exit %d)\n%s", r.Codigo, final(r.Salida, 12))
	}
	en := ahora()
	t.Estado = nuevo
	if nuevo == "verde" {
		t.VerdeEn = en
	}
	t.Evidencia = append(t.Evidencia, motor.Evidencia{Cmd: c, Resultado: "exit 0", Nivel: "V1", En: en})
	if err := q.M.RegistrarTarea("tdd."+nuevo, t); err != nil {
		return err
	}
	return ExportarHooks(q.M, q.Raiz)
}

// superable: solo una tarea que pasó por VERDE puede superarse (no sirve para saltarse un ROJO).
var superable = map[string]bool{"verde": true, "refactor": true, "verificada": true}

// supera: la tarea que reemplaza tiene que estar en VERDE o después (una superada se rechaza aparte).
var supera = map[string]bool{"verde": true, "refactor": true, "verificada": true, "cerrada": true}

// Superar deja constancia de que la tarea `id` quedó reemplazada por `por`, con la decisión registrada `decision`.
// No toca ni re-bloquea tests: es el caso de un golden que otra tarea regeneró con aprobación (RG-2.3).
func (q Maquina) Superar(id, por, decision, motivo, quien string) error {
	id, por, decision = strings.ToUpper(strings.TrimSpace(id)), strings.ToUpper(strings.TrimSpace(por)), strings.ToUpper(strings.TrimSpace(decision))
	t, err := q.M.Tarea(id)
	if errors.Is(err, motor.ErrNoExiste) {
		return verificacion("la tarea %s no existe; revisa el id con `hz tdd estado`", id)
	}
	if err != nil {
		return err
	}
	if s := t.Superacion; t.Estado == "superada" && s != nil {
		return verificacion("la tarea %s ya fue superada por %s (%s) el %s; no hay nada que hacer", id, s.Por, s.Decision, s.En)
	}
	if !superable[t.Estado] {
		return verificacion("la tarea %s está en %s: superar exige que haya pasado por VERDE (no sirve para saltarse un ROJO); termina su ciclo con `hz tdd verde %s`",
			id, strings.ToUpper(t.Estado), id)
	}
	if por == id {
		return verificacion("una tarea no puede superarse a sí misma: --por debe ser la tarea que la reemplaza")
	}
	tm, err := q.M.Tarea(por)
	if errors.Is(err, motor.ErrNoExiste) {
		return verificacion("la tarea que supera, %s, no existe; regístrala con `hz tdd contrato %s` y llévala a VERDE", por, por)
	}
	if err != nil {
		return err
	}
	if tm.Estado == "superada" && tm.Superacion != nil {
		return verificacion("la tarea %s también fue superada; usa --por %s", por, tm.Superacion.Por)
	}
	if !supera[tm.Estado] {
		return verificacion("la tarea que supera, %s, está en %s; tiene que estar en VERDE o después: `hz tdd verde %s`", por, strings.ToUpper(tm.Estado), por)
	}
	if err := respalda(q.Raiz, t, tm); err != nil {
		return err
	}
	d, err := q.M.VerDecision(decision)
	if errors.Is(err, motor.ErrNoExiste) {
		return verificacion("la decisión %s no existe; regístrala antes con `hz decision nueva --que … --por-que … --decidio humano|agente_aprobado`", decision)
	}
	if err != nil {
		return err
	}
	if d.Decidio != "humano" && d.Decidio != "agente_aprobado" {
		return verificacion("la decisión %s la decidió %s: superar deja de verificar los tests bloqueados de %s, así que exige una decisión humano o agente_aprobado; pide la aprobación del humano y registra esa decisión",
			d.ID, d.Decidio, id)
	}
	if strings.TrimSpace(motivo) == "" {
		motivo = d.Que
	}
	t.Superacion = &motor.Superacion{Por: por, Decision: d.ID, Decidio: d.Decidio, Motivo: motivo, Quien: quien, En: ahora(), EstadoPrevio: t.Estado}
	t.Estado = "superada"
	if err := q.M.RegistrarTarea("tdd.superada", t); err != nil {
		return err
	}
	return ExportarHooks(q.M, q.Raiz)
}

// respalda exige la evidencia de D-0054: algún test bloqueado de t cambió y tm bloquea cada uno de los que
// cambiaron con su contenido actual. Como hz salida deja de verificar una tarea superada, sin esto superar
// serviría para saltarse a su verificador.
func respalda(raiz string, t, tm motor.Tarea) error {
	nuevos := make(map[string]string, len(tm.TestsBloqueados))
	for _, tb := range tm.TestsBloqueados {
		nuevos[rutaTest(tb.Ruta)] = tb.SHA256
	}
	cambiados := 0
	for _, tb := range t.TestsBloqueados {
		h, err := HashArchivo(filepath.Join(raiz, filepath.FromSlash(tb.Ruta)))
		if err == nil && h == tb.SHA256 {
			continue
		}
		cambiados++
		bloqueado, ok := nuevos[rutaTest(tb.Ruta)]
		switch {
		case err != nil:
			return verificacion("el test %s de %s ya no existe o no se puede leer: %s no puede respaldarlo con su contenido actual", tb.Ruta, t.ID, tm.ID)
		case !ok:
			return verificacion("el test %s de %s cambió y %s no lo bloquea: supera %s con la tarea que lo regeneró y lo bloqueó", tb.Ruta, t.ID, tm.ID, t.ID)
		case bloqueado != h:
			return verificacion("el test %s cambió después de que %s lo bloqueara: su contenido actual no es el que %s respalda", tb.Ruta, tm.ID, tm.ID)
		}
	}
	if cambiados == 0 {
		return verificacion("ningún test bloqueado de %s cambió: no hay nada que superar; si la tarea terminó, entrégala con `hz salida --rol verificador --tarea %s`", t.ID, t.ID)
	}
	return nil
}

// rutaTest normaliza la ruta de un test bloqueado para compararla: limpia, con / y, en Windows, sin mayúsculas.
func rutaTest(r string) string {
	r = path.Clean(filepath.ToSlash(r))
	if runtime.GOOS == "windows" {
		r = strings.ToLower(r)
	}
	return r
}

// ExportarHooks regenera .harness/cache/hooks.json v2 (contratos/plugin.md); vive en el paquete exportar.
func ExportarHooks(m *motor.Motor, raiz string) error { return exportar.Escribir(m, raiz) }

// Shell expone la selección de intérprete para otros paquetes (p. ej. los sensores de las misiones).
func Shell(ctx context.Context, cmd string) *exec.Cmd { return shell(ctx, cmd) }

// ColaAcotada devuelve un writer que conserva solo los últimos `max` bytes, y una función para leerlos.
func ColaAcotada(max int) (io.Writer, func() string) {
	c := &colaAcotada{max: max}
	return c, func() string { return string(c.buf) }
}
