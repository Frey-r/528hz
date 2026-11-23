package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"hz528/internal/dashboard"
	"hz528/internal/proyecto"
	"hz528/internal/registro"
)

// Incremento 3.8 · dashboard central multi-proyecto (contratos/cli.md, D-0053). Un solo servidor por usuario
// lee todos los proyectos del registro; `hz dashboard` conecta el proyecto del cwd y se asegura de que corre.

const (
	// edadSesiones: los snapshots de sesión sin escribir hace más que esto se podan (hz puntero).
	edadSesiones = 24 * time.Hour
	// esperaArranque: cuánto se espera a que el servidor desacoplado responda.
	esperaArranque = 5 * time.Second
	// esperaApagado: cuánto se espera a que un servidor terminado libere el puerto.
	esperaApagado = 3 * time.Second
	archivoLog    = "dashboard.log"
)

// Puntos de inyección para los tests: lanzar y terminar procesos reales no cabe en go test.
var (
	// lanzarDashboard arranca `hz dashboard --primer-plano --puerto p` desacoplado, con cwd en dir y log en log.
	lanzarDashboard = lanzarDesacoplado
	// terminarProceso termina el proceso del dashboard que se identificó en /api/salud.
	terminarProceso = func(pid int) error {
		p, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return p.Kill()
	}
	// alEscuchar, si no es nil, recibe la dirección ya escuchando y cómo apagar el servidor (lo usan los tests).
	alEscuchar func(dir string, apagar func())
)

// binarioPropio identifica este ejecutable (tamaño + fecha de modificación): si se recompila hz, cambia.
func binarioPropio() string {
	exe, err := os.Executable()
	if err != nil {
		return "desconocido"
	}
	info, err := os.Stat(exe)
	if err != nil {
		return "desconocido"
	}
	return fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano())
}

type estadoPuerto int

const (
	puertoVacio estadoPuerto = iota
	puertoHZ
	puertoAjeno
)

type salud struct {
	Servicio string `json:"servicio"`
	Pid      int    `json:"pid"`
	Binario  string `json:"binario"`
}

// consultar mira qué hay en el puerto: nada, un dashboard de hz (con su identidad) u otro servicio.
func consultar(puerto int) (salud, estadoPuerto) {
	dir := net.JoinHostPort("127.0.0.1", strconv.Itoa(puerto))
	conn, err := net.DialTimeout("tcp", dir, time.Second)
	if err != nil {
		return salud{}, puertoVacio
	}
	conn.Close()
	cliente := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	r, err := cliente.Get("http://" + dir + "/api/salud")
	if err != nil {
		return salud{}, puertoAjeno
	}
	defer r.Body.Close()
	var s salud
	if r.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&s) != nil || s.Servicio != dashboard.Servicio {
		return salud{}, puertoAjeno
	}
	return s, puertoHZ
}

// esperar repite la condición cada 100 ms hasta que se cumpla o se acabe el plazo.
func esperar(plazo time.Duration, cumple func() bool) bool {
	limite := time.Now().Add(plazo)
	for {
		if cumple() {
			return true
		}
		if time.Now().After(limite) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func cmdDashboard(g globales, args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(errw)
	puerto := fs.Int("puerto", 5280, "puerto local (solo 127.0.0.1)")
	primerPlano := fs.Bool("primer-plano", false, "sirve en primer plano hasta Ctrl+C")
	detener := fs.Bool("detener", false, "detiene el dashboard central")
	desconectar := fs.Bool("desconectar", false, "saca el proyecto del cwd del dashboard")
	opciones := 0
	err := fs.Parse(args)
	for _, o := range []bool{*primerPlano, *detener, *desconectar} {
		if o {
			opciones++
		}
	}
	if err != nil || fs.NArg() > 0 || *puerto < 1 || *puerto > 65535 || opciones > 1 {
		fmt.Fprintln(errw, "uso: hz dashboard [--puerto 5280] [--primer-plano | --detener | --desconectar]")
		return usoInvalido
	}
	hogar, err := registro.Hogar()
	if err != nil {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	switch {
	case *primerPlano:
		return servirPrimerPlano(hogar, *puerto, out, errw)
	case *detener:
		return detenerDashboard(*puerto, out, errw)
	case *desconectar:
		raiz, err := proyecto.Buscar(g.directorio())
		if err != nil {
			fmt.Fprintf(errw, "%v; no hay proyecto que desconectar\n", err)
			return sinProyecto
		}
		if _, err := registro.Desconectar(hogar, raiz); err != nil {
			fmt.Fprintln(errw, err)
			return errorInterno
		}
		fmt.Fprintf(out, "proyecto %s desconectado\n", filepath.Base(raiz))
		return ok
	}
	conectado := ""
	if raiz, err := proyecto.Buscar(g.directorio()); err == nil {
		if _, err := registro.Conectar(hogar, raiz, time.Now()); err != nil {
			fmt.Fprintln(errw, err)
			return errorInterno
		}
		conectado = filepath.Base(raiz)
	}
	if code := asegurarServidor(hogar, *puerto, errw); code != ok {
		return code
	}
	fmt.Fprintf(out, "hz dashboard en http://127.0.0.1:%d\n", *puerto)
	if conectado != "" {
		fmt.Fprintf(out, "proyecto %s conectado\n", conectado)
	}
	return ok
}

// asegurarServidor deja corriendo un dashboard de este binario en el puerto: reutiliza el que hay, reemplaza
// uno de otro binario o lanza uno nuevo desacoplado de la terminal.
func asegurarServidor(hogar string, puerto int, errw io.Writer) int {
	propio := binarioPropio()
	s, estado := consultar(puerto)
	switch estado {
	case puertoAjeno:
		fmt.Fprintf(errw, "el puerto %d lo ocupa otro servicio: usa --puerto\n", puerto)
		return errorInterno
	case puertoHZ:
		if s.Binario == propio {
			return ok
		}
		if err := terminarProceso(s.Pid); err != nil {
			fmt.Fprintf(errw, "no se pudo detener el dashboard anterior (pid %d): %v\n", s.Pid, err)
			return errorInterno
		}
		if !esperar(esperaApagado, func() bool { _, e := consultar(puerto); return e == puertoVacio }) {
			fmt.Fprintf(errw, "el dashboard anterior (pid %d) no liberó el puerto %d\n", s.Pid, puerto)
			return errorInterno
		}
	}
	if err := os.MkdirAll(hogar, 0o755); err != nil {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	log := filepath.Join(hogar, archivoLog)
	if err := lanzarDashboard(puerto, hogar, log); err != nil {
		fmt.Fprintf(errw, "no se pudo lanzar el dashboard: %v\n", err)
		return errorInterno
	}
	listo := esperar(esperaArranque, func() bool {
		s, e := consultar(puerto)
		return e == puertoHZ && s.Binario == propio
	})
	if !listo {
		fmt.Fprintf(errw, "el dashboard no respondió en %s; revisa %s\n", esperaArranque, log)
		return errorInterno
	}
	return ok
}

func detenerDashboard(puerto int, out, errw io.Writer) int {
	s, estado := consultar(puerto)
	switch estado {
	case puertoVacio:
		fmt.Fprintln(out, "no hay dashboard corriendo")
		return ok
	case puertoAjeno:
		fmt.Fprintf(errw, "el puerto %d lo ocupa otro servicio, no un dashboard de hz\n", puerto)
		return errorInterno
	}
	if err := terminarProceso(s.Pid); err != nil {
		fmt.Fprintf(errw, "no se pudo detener el dashboard (pid %d): %v\n", s.Pid, err)
		return errorInterno
	}
	esperar(esperaApagado, func() bool { _, e := consultar(puerto); return e == puertoVacio })
	fmt.Fprintln(out, "hz dashboard detenido")
	return ok
}

// servirPrimerPlano sirve el dashboard central hasta Ctrl+C (es lo que corre el proceso desacoplado).
func servirPrimerPlano(hogar string, puerto int, out, errw io.Writer) int {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(puerto)))
	if err != nil {
		fmt.Fprintf(errw, "no se pudo escuchar en 127.0.0.1:%d: %v\n", puerto, err)
		return errorInterno
	}
	c := dashboard.NuevaCentral(hogar)
	defer c.Cerrar()
	identidad := dashboard.Salud{Version: version, Pid: os.Getpid(), Binario: binarioPropio()}
	srv := &http.Server{Handler: dashboard.Servidor(c, puerto, identidad), ReadHeaderTimeout: 5 * time.Second}
	apagar := func() { _ = srv.Shutdown(context.Background()) }
	// Ctrl+C apaga ordenadamente: termina las peticiones en curso y libera los motores.
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	go func() {
		<-ctx.Done()
		apagar()
	}()
	fmt.Fprintf(out, "hz dashboard en http://127.0.0.1:%d (solo este equipo, solo lectura; Ctrl+C para salir)\n", puerto)
	if alEscuchar != nil {
		go alEscuchar(ln.Addr().String(), apagar)
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(errw, err)
		return errorInterno
	}
	fmt.Fprintln(out, "hz dashboard detenido")
	return ok
}
