//go:build windows

package capacidad

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// memoryStatusEx replica MEMORYSTATUSEX de la API de Windows (x/sys no la expone en v0.49).
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// Se resuelve una sola vez por proceso (RG-1.4).
var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func memoria() (libreMB, totalMB int, err error) {
	ms := memoryStatusEx{}
	ms.Length = uint32(unsafe.Sizeof(ms))
	if r, _, e := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); r == 0 {
		return 0, 0, e
	}
	return int(ms.AvailPhys >> 20), int(ms.TotalPhys >> 20), nil
}
