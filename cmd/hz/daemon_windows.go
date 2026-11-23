//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// DETACHED_PROCESS: el proceso hijo no hereda la consola, así sobrevive al cerrar la terminal o la sesión.
const procesoDesacoplado = 0x00000008

// lanzarDesacoplado arranca el dashboard central en segundo plano, sin consola y en su propio grupo de
// procesos. El cwd va al hogar de hz para no retener la carpeta de ningún proyecto.
func lanzarDesacoplado(puerto int, dir, rutaLog string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(rutaLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(exe, "dashboard", "--primer-plano", "--puerto", strconv.Itoa(puerto))
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: procesoDesacoplado | syscall.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
