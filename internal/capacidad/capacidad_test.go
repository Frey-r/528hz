package capacidad

import "testing"

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

func TestMedirDevuelveValoresDelSistema(t *testing.T) {
	e, err := Medir()
	if err != nil {
		t.Fatalf("medir: %v", err)
	}
	if e.TotalMB <= 0 || e.LibreMB <= 0 || e.Nucleos <= 0 || e.LibreMB > e.TotalMB {
		t.Fatalf("medición inválida: %+v", e)
	}
}
