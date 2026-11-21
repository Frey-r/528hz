// Package mision maneja la definición (mision.yaml), el estado para retomar (estado.json) y los sensores
// de una misión de trabajo largo (docs/diseno/08-misiones.md).
package mision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"hz528/internal/estadistica"
	"hz528/internal/tdd"
)

// Metrica de una vertical: un sensor determinista con su dirección, meta y piso de ruido.
type Metrica struct {
	ID        string   `yaml:"id" json:"id"`
	Familia   string   `yaml:"familia" json:"familia"`
	Sensor    string   `yaml:"sensor" json:"sensor"`
	Direccion string   `yaml:"direccion" json:"direccion"`
	LineaBase *float64 `yaml:"linea_base" json:"linea_base,omitempty"`
	Meta      float64  `yaml:"meta" json:"meta"`
	PisoRuido *float64 `yaml:"piso_ruido" json:"piso_ruido,omitempty"`
}

// Vertical esencial del producto (D-0022): efectividad, impacto y fallos.
type Vertical struct {
	ID       string    `yaml:"id" json:"id"`
	Nombre   string    `yaml:"nombre" json:"nombre"`
	Peso     float64   `yaml:"peso" json:"peso"`
	Metricas []Metrica `yaml:"metricas" json:"metricas"`
}

// Criterio del contrato de cierre default-FAIL.
type Criterio struct {
	ID       string `yaml:"id" json:"id"`
	Texto    string `yaml:"texto" json:"texto"`
	Cumplido bool   `yaml:"cumplido" json:"cumplido"`
}

// Mision es el subconjunto de mision.yaml que usa el CLI (schema/mision.schema.json).
type Mision struct {
	ID          string     `yaml:"id"`
	Titulo      string     `yaml:"titulo"`
	Modo        string     `yaml:"modo"`
	Objetivo    string     `yaml:"objetivo"`
	Verticales  []Vertical `yaml:"verticales"`
	Presupuesto struct {
		UsdMax float64 `yaml:"usd_max"`
	} `yaml:"presupuesto"`
	Estadistica struct {
		K float64 `yaml:"k_propuesta_milestone"`
	} `yaml:"estadistica"`
	Aceptacion []Criterio `yaml:"aceptacion"`
}

// Estado es lo que se lee para retomar tras un corte (estado.json).
type Estado struct {
	Mision      string `json:"mision"`
	Fase        string `json:"fase"`
	Estado      string `json:"estado"`
	ProximoPaso string `json:"proximo_paso"`
	Actualizado string `json:"actualizado"`
}

// idValido acota los ids de misiones y métricas: se usan como nombres de carpeta y de archivo.
var idValido = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func validarID(id string) error {
	if !idValido.MatchString(id) {
		return fmt.Errorf("%w: id %q inválido (letras, dígitos, - y _; se usa como nombre de carpeta)", tdd.ErrVerificacion, id)
	}
	return nil
}

func dir(raiz, id string) string { return filepath.Join(raiz, ".harness", "misiones", id) }

// Ruta del archivo de una misión.
func Ruta(raiz, id, archivo string) string { return filepath.Join(dir(raiz, id), archivo) }

// Metricas devuelve todas las métricas de la misión.
func (m Mision) Metricas() []Metrica {
	var out []Metrica
	for _, v := range m.Verticales {
		out = append(out, v.Metricas...)
	}
	return out
}

// Crear escribe el esqueleto de la misión y su estado inicial; nunca sobrescribe una existente.
func Crear(raiz, id, modo, objetivo string, usdMax float64) error {
	if err := validarID(id); err != nil {
		return err
	}
	if modo != "genesis" && modo != "mejora" {
		return fmt.Errorf("%w: modo debe ser genesis o mejora", tdd.ErrVerificacion)
	}
	ruta := Ruta(raiz, id, "mision.yaml")
	if _, err := os.Stat(ruta); err == nil {
		return fmt.Errorf("%w: la misión %s ya existe", tdd.ErrVerificacion, id)
	}
	if err := os.MkdirAll(dir(raiz, id), 0o755); err != nil {
		return err
	}
	esqueleto := fmt.Sprintf(`# Misión del harness 528hz (schema/mision.schema.json)
id: %s
titulo: %s
modo: %s
objetivo: %s
non_goals: []
# Mejora: verticales definidas CONVERSANDO con el usuario; cada una con métricas de efectividad, impacto y fallos.
# Sensor: comando determinista que imprime [{"item_id": "…", "resultado": true|false}] o con "puntaje"; {ronda} se reemplaza.
verticales: []
# Génesis: hitos en slices verticales.
hitos: []
estadistica: {k_propuesta_milestone: 2, fraccion_holdout: 0.15}
presupuesto: {usd_max: %s}
aceptacion:
  - {id: A1, texto: "Objetivo cumplido con evidencia ejecutada", cumplido: false}
`, id, id, modo, strconv.Quote(objetivo), strconv.FormatFloat(usdMax, 'f', -1, 64))
	if err := os.WriteFile(ruta, []byte(esqueleto), 0o644); err != nil {
		return err
	}
	fase, proximo := "intencion", "escribir la spec y los criterios de aceptación con el humano"
	if modo == "mejora" {
		fase, proximo = "verticales", "definir las verticales esenciales con el humano (efectividad, impacto, fallos)"
	}
	return escribirEstado(raiz, Estado{Mision: id, Fase: fase, Estado: "activa", ProximoPaso: proximo})
}

var familias = []string{"efectividad", "impacto", "fallos"}

// Cargar lee y valida mision.yaml.
func Cargar(raiz, id string) (Mision, error) {
	if err := validarID(id); err != nil {
		return Mision{}, err
	}
	b, err := os.ReadFile(Ruta(raiz, id, "mision.yaml"))
	if err != nil {
		return Mision{}, fmt.Errorf("%w: la misión %s no existe", tdd.ErrVerificacion, id)
	}
	var m Mision
	if err := yaml.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("mision.yaml de %s: %w", id, err)
	}
	if m.Estadistica.K == 0 {
		m.Estadistica.K = 2
	}
	var errs []string
	if m.Modo != "genesis" && m.Modo != "mejora" {
		errs = append(errs, "modo debe ser genesis o mejora")
	}
	for _, v := range m.Verticales {
		presentes := map[string]bool{}
		for _, mt := range v.Metricas {
			presentes[mt.Familia] = true
			if !idValido.MatchString(mt.ID) || mt.Sensor == "" || (mt.Direccion != "sube" && mt.Direccion != "baja") {
				errs = append(errs, fmt.Sprintf("métrica %q de %s: id (letras, dígitos, - y _), sensor y direccion (sube|baja) son obligatorios", mt.ID, v.ID))
			}
		}
		var faltan []string
		for _, f := range familias {
			if !presentes[f] {
				faltan = append(faltan, f)
			}
		}
		if len(faltan) > 0 {
			errs = append(errs, fmt.Sprintf("la vertical %s no mide %s (cada vertical mide efectividad, impacto y fallos, D-0022)", v.ID, strings.Join(faltan, ", ")))
		}
	}
	if len(errs) > 0 {
		return m, fmt.Errorf("%w: mision.yaml de %s: %s", tdd.ErrVerificacion, id, strings.Join(errs, "; "))
	}
	return m, nil
}

// LeerEstado devuelve el estado para retomar.
func LeerEstado(raiz, id string) (Estado, error) {
	var e Estado
	b, err := os.ReadFile(Ruta(raiz, id, "estado.json"))
	if err != nil {
		return e, err
	}
	return e, json.Unmarshal(b, &e)
}

// FijarFase registra la fase y el próximo paso (en cada transición, para retomar en ≤5 llamadas).
func FijarFase(raiz, id, fase, proximo string) error {
	e, err := LeerEstado(raiz, id)
	if err != nil {
		return err
	}
	e.Fase, e.ProximoPaso = fase, proximo
	if fase == "cerrada" {
		e.Estado = "cerrada"
	}
	return escribirEstado(raiz, e)
}

func escribirEstado(raiz string, e Estado) error {
	e.Actualizado = time.Now().Format(time.RFC3339)
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp := Ruta(raiz, e.Mision, "estado.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, Ruta(raiz, e.Mision, "estado.json"))
}

// Sensor corre un sensor determinista y decodifica su stdout en streaming (RG-1.8): JSON por ítem.
func Sensor(raiz, cmd string) ([]estadistica.Item, error) {
	c := tdd.Shell(context.Background(), cmd)
	c.Dir = raiz
	errores, leerErrores := tdd.ColaAcotada(8 * 1024)
	c.Stderr = errores
	salida, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("sensor %q: %w", cmd, err)
	}
	var items []estadistica.Item
	errJSON := json.NewDecoder(salida).Decode(&items)
	errEsperar := c.Wait()
	if errEsperar != nil {
		return nil, fmt.Errorf("%w: el sensor %q falló: %v %s", tdd.ErrVerificacion, cmd, errEsperar, strings.TrimSpace(leerErrores()))
	}
	if errJSON != nil {
		return nil, fmt.Errorf("%w: el sensor %q no emitió JSON por ítem ([{\"item_id\", \"resultado\"|\"puntaje\"}]): %v", tdd.ErrVerificacion, cmd, errJSON)
	}
	for _, it := range items {
		if it.ID == "" || (it.Resultado == nil) == (it.Puntaje == nil) {
			return nil, fmt.Errorf("%w: cada ítem del sensor necesita item_id y exactamente uno de resultado o puntaje", tdd.ErrVerificacion)
		}
	}
	if len(items) == 0 {
		return nil, errors.New("el sensor no devolvió ítems")
	}
	return items, nil
}
