// Package estadistica compara rondas de mejora de forma pareada y propone milestones (docs/diseno/08-misiones.md).
// Los números del harness salen de aquí, nunca del conteo del modelo.
package estadistica

import (
	"fmt"
	"math"
	"strings"
)

// Item es la salida por ítem de un sensor: un resultado binario o un puntaje en [0, 1].
type Item struct {
	ID        string   `json:"item_id"`
	Resultado *bool    `json:"resultado,omitempty"`
	Puntaje   *float64 `json:"puntaje,omitempty"`
}

// Comparacion es el resultado pareado de dos rondas para una métrica.
type Comparacion struct {
	Pareados  int     `json:"pareados"`
	Ganados   int     `json:"ganados"`
	Perdidos  int     `json:"perdidos"`
	Empates   int     `json:"empates"`
	ValorA    float64 `json:"valor_a"`
	ValorB    float64 `json:"valor_b"`
	Delta     float64 `json:"delta"`
	P         float64 `json:"p"`
	Veredicto string  `json:"veredicto"`
}

// Metrica es lo que la propuesta de milestone necesita saber de una métrica.
type Metrica struct {
	ID        string
	Direccion string // sube | baja
	Piso      float64
}

// Propuesta de milestone: el harness propone, el humano define (D-0019).
type Propuesta struct {
	Propone bool   `json:"propone"`
	Motivo  string `json:"motivo"`
}

// Alfa es el nivel de significancia del test de signos.
const Alfa = 0.05

func valor(i Item) (float64, bool) {
	switch {
	case i.Resultado != nil:
		if *i.Resultado {
			return 1, true
		}
		return 0, true
	case i.Puntaje != nil:
		return *i.Puntaje, true
	}
	return 0, false
}

// Valor es el promedio de los ítems con valor.
func Valor(items []Item) float64 {
	suma, n := 0.0, 0
	for _, it := range items {
		if v, ok := valor(it); ok {
			suma += v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return suma / float64(n)
}

// PSignos es el p bilateral exacto del test de signos (McNemar exacto en binarios) sobre pares discordantes.
func PSignos(ganados, perdidos int) float64 {
	n := ganados + perdidos
	if n == 0 {
		return 1
	}
	k := min(ganados, perdidos)
	lnN, cola := lgamma(n+1), 0.0
	for i := 0; i <= k; i++ {
		cola += math.Exp(lnN - lgamma(i+1) - lgamma(n-i+1) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*cola)
}

func lgamma(x int) float64 {
	v, _ := math.Lgamma(float64(x))
	return v
}

// Comparar parea por item_id (solo ítems presentes en ambas rondas) y emite un veredicto según la dirección.
func Comparar(a, b []Item, direccion string, piso float64) Comparacion {
	previos := make(map[string]float64, len(a))
	for _, it := range a {
		if v, ok := valor(it); ok {
			previos[it.ID] = v
		}
	}
	var c Comparacion
	sumaA, sumaB := 0.0, 0.0
	for _, it := range b {
		vb, okB := valor(it)
		va, okA := previos[it.ID]
		if !okA || !okB {
			continue
		}
		c.Pareados++
		sumaA, sumaB = sumaA+va, sumaB+vb
		mejor := vb > va
		if direccion == "baja" {
			mejor = vb < va
		}
		switch {
		case va == vb:
			c.Empates++
		case mejor:
			c.Ganados++
		default:
			c.Perdidos++
		}
	}
	if c.Pareados == 0 {
		c.Veredicto, c.P = "sin datos", 1
		return c
	}
	c.ValorA, c.ValorB = sumaA/float64(c.Pareados), sumaB/float64(c.Pareados)
	c.Delta = c.ValorB - c.ValorA
	c.P = PSignos(c.Ganados, c.Perdidos)
	favorable := c.Delta > 0
	if direccion == "baja" {
		favorable = c.Delta < 0
	}
	switch {
	case math.Abs(c.Delta) <= piso || c.P > Alfa:
		c.Veredicto = "ruido"
	case favorable:
		c.Veredicto = "mejora"
	default:
		c.Veredicto = "regresion"
	}
	return c
}

// Proponer sugiere un milestone si alguna métrica mejora más de k × su piso de ruido frente a la base
// (último milestone o línea base) y ninguna empeora más allá de su piso.
func Proponer(base, ronda map[string]float64, metricas []Metrica, k float64) Propuesta {
	var saltos, regresiones []string
	for _, m := range metricas {
		antes, okA := base[m.ID]
		ahora, okB := ronda[m.ID]
		if !okA || !okB {
			continue
		}
		favorable := ahora - antes
		if m.Direccion == "baja" {
			favorable = antes - ahora
		}
		switch {
		case favorable > 0 && favorable > k*m.Piso:
			saltos = append(saltos, fmt.Sprintf("%s %+.3f (umbral %.3f)", m.ID, ahora-antes, k*m.Piso))
		case -favorable > m.Piso:
			regresiones = append(regresiones, fmt.Sprintf("%s %+.3f (piso %.3f)", m.ID, ahora-antes, m.Piso))
		}
	}
	switch {
	case len(regresiones) > 0:
		return Propuesta{Motivo: "hay regresiones fuera de ruido: " + strings.Join(regresiones, ", ")}
	case len(saltos) == 0:
		return Propuesta{Motivo: fmt.Sprintf("ningún salto supera %.0f × el piso de ruido", k)}
	}
	return Propuesta{Propone: true, Motivo: "salto evidente: " + strings.Join(saltos, ", ")}
}
