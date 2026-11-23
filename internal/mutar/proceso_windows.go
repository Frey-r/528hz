//go:build windows

package mutar

import (
	"errors"

	"golang.org/x/sys/windows"
)

const stillActive = 259 // STILL_ACTIVE de GetExitCodeProcess

// procesoVivo indica si existe un proceso con ese pid en este host (sin acceso a él también cuenta como vivo).
func procesoVivo(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var codigo uint32
	if err := windows.GetExitCodeProcess(h, &codigo); err != nil {
		return true // ante la duda, vivo: decide el latido
	}
	return codigo == stillActive
}
