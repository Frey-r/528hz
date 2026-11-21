// Package capacidad dimensiona el enjambre según el hardware (D-0023, D-0030).
package capacidad

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

// MBPorAgente es el costo medido de un agente en la máquina (spike S14: ≈560 MB / 12 agentes).
const MBPorAgente = 45

// Entrada describe la máquina y la forma de las células.
type Entrada struct {
	LibreMB, TotalMB, Nucleos int
	AgentesPorCelula, PicoMB  int
	LimiteConcurrencia        int
}

// Resultado es el dimensionamiento recomendado.
type Resultado struct {
	Entrada
	ReservaMB     int    `json:"reserva_mb"`
	CostoCelulaMB int    `json:"costo_celula_mb"`
	PorRAM        int    `json:"por_ram"`
	PorConcurr    int    `json:"por_concurrencia"`
	Celulas       int    `json:"celulas"`
	Recomendacion string `json:"recomendacion,omitempty"`
}

// Calcular aplica: min(concurrencia/agentes, ⌊(libre−reserva)/costo⌋, núcleos), con reserva = max(1 GB, 10 % del total).
func Calcular(e Entrada) Resultado {
	if e.AgentesPorCelula < 1 {
		e.AgentesPorCelula = 1
	}
	r := Resultado{Entrada: e, ReservaMB: max(1024, e.TotalMB/10), CostoCelulaMB: e.AgentesPorCelula*MBPorAgente + e.PicoMB}
	r.PorRAM = max(0, (e.LibreMB-r.ReservaMB)/r.CostoCelulaMB)
	r.PorConcurr = e.LimiteConcurrencia / e.AgentesPorCelula
	r.Celulas = min(r.PorConcurr, r.PorRAM, e.Nucleos)
	if r.Celulas == 0 {
		falta := r.ReservaMB + r.CostoCelulaMB - e.LibreMB
		r.Recomendacion = fmt.Sprintf("Sin RAM para una célula: libera al menos %d MB (navegadores, IDEs) o baja --agentes-por-celula/--pico-mb.", max(falta, 1))
	}
	return r
}

// LimiteConcurrencia lee CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS (20 por defecto).
func LimiteConcurrencia() int {
	if v, err := strconv.Atoi(os.Getenv("CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS")); err == nil && v > 0 {
		return v
	}
	return 20
}

// Medir lee la RAM libre y total y los núcleos de la máquina.
func Medir() (Entrada, error) {
	libre, total, err := memoria()
	if err != nil {
		return Entrada{}, err
	}
	return Entrada{LibreMB: libre, TotalMB: total, Nucleos: runtime.NumCPU(), LimiteConcurrencia: LimiteConcurrencia()}, nil
}
