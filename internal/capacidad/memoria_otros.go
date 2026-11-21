//go:build !windows

package capacidad

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

// memoria lee /proc/meminfo (Linux y WSL2). En otros sistemas devuelve un error explícito.
func memoria() (libreMB, totalMB int, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, errors.New("medición de RAM no soportada en este sistema (falta /proc/meminfo)")
	}
	defer f.Close()
	valores := map[string]int{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		campos := strings.Fields(sc.Text())
		if len(campos) >= 2 {
			if kb, err := strconv.Atoi(campos[1]); err == nil {
				valores[strings.TrimSuffix(campos[0], ":")] = kb
			}
		}
	}
	return valores["MemAvailable"] >> 10, valores["MemTotal"] >> 10, sc.Err()
}
