package capacidad

import (
	"strings"
	"testing"
)

func TestCalcularSegunRamNucleosYConcurrencia(t *testing.T) {
	base := Entrada{TotalMB: 16000, Nucleos: 12, AgentesPorCelula: 3, PicoMB: 300, LimiteConcurrencia: 20}
	casos := []struct {
		nombre  string
		libreMB int
		celulas int
	}{
		{"poca ram (caso real de la máquina)", 2048, 1},
		{"ram holgada: manda la concurrencia", 16000, 6},
		{"sin ram disponible", 1000, 0},
	}
	for _, c := range casos {
		e := base
		e.LibreMB = c.libreMB
		r := Calcular(e)
		if r.Celulas != c.celulas {
			t.Errorf("%s: células = %d, se esperaban %d (%+v)", c.nombre, r.Celulas, c.celulas, r)
		}
		if r.ReservaMB != 1600 {
			t.Errorf("%s: la reserva es max(1 GB, 10%% del total) = 1600 MB, fue %d", c.nombre, r.ReservaMB)
		}
		if c.celulas == 0 && r.Recomendacion == "" {
			t.Errorf("%s: con 0 células debe recomendar liberar RAM", c.nombre)
		}
	}
	pocosNucleos := base
	pocosNucleos.LibreMB, pocosNucleos.Nucleos = 16000, 2
	if r := Calcular(pocosNucleos); r.Celulas != 2 {
		t.Errorf("los núcleos también limitan: %+v", r)
	}
}

func TestRecomendacionIndicaCuantaRamFalta(t *testing.T) {
	r := Calcular(Entrada{LibreMB: 1000, TotalMB: 16000, Nucleos: 12, AgentesPorCelula: 3, PicoMB: 300, LimiteConcurrencia: 20})
	// falta = reserva 1600 + costo de una célula 435 − libre 1000 = 1035 MB
	if !strings.Contains(r.Recomendacion, "1035 MB") {
		t.Fatalf("la recomendación debe decir exactamente cuánta RAM liberar: %q", r.Recomendacion)
	}
}

func TestLimiteConcurrenciaDesdeElEntorno(t *testing.T) {
	casos := map[string]int{"8": 8, "0": 20, "abc": 20, "": 20}
	for valor, esperado := range casos {
		t.Setenv("CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS", valor)
		if got := LimiteConcurrencia(); got != esperado {
			t.Errorf("CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS=%q → %d, se esperaba %d", valor, got, esperado)
		}
	}
}

func TestMedirDevuelveValoresDelSistema(t *testing.T) {
	e, err := Medir()
	if err != nil {
		t.Fatalf("medir: %v", err)
	}
	if e.TotalMB <= 0 || e.LibreMB <= 0 || e.Nucleos <= 0 || e.LibreMB > e.TotalMB {
		t.Fatalf("medición inválida: %+v", e)
	}
}
