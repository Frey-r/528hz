//go:build !windows && !unix

package mutar

// procesoVivo no puede consultarse en esta plataforma: se asume vivo y decide solo el latido.
func procesoVivo(int) bool { return true }
