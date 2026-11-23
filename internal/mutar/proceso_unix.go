//go:build unix

package mutar

import (
	"errors"
	"syscall"
)

// procesoVivo indica si existe un proceso con ese pid en este host (EPERM: existe pero es de otro usuario).
func procesoVivo(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
