// Package check verifica la estructura orientada a agentes (anti-deriva, docs/diseno/04-estructura.md).
package check

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"hz528/internal/proyecto"
)

// Resultado de una verificación.
type Resultado struct {
	Nombre string
	OK     bool
	Motivo string
}

// rutaCitada detecta rutas entre backticks con extensión conocida (compilado una vez, RG-1.4).
var rutaCitada = regexp.MustCompile("`([^`\\s]+\\.(?:md|json|jsonl|sql|yaml|yml|go|ts|tsx|py|js|txt|toml))`")

// directorios que no son dominios del proyecto.
var ignorar = map[string]bool{".git": true, "node_modules": true, ".harness": true, ".claude": true, "vendor": true,
	"dist": true, "venv": true, ".venv": true, "__pycache__": true, "build": true}

// Ejecutar corre todas las verificaciones.
func Ejecutar(raiz string, p *proyecto.Perfil) []Resultado {
	agents := filepath.Join(raiz, "AGENTS.md")
	n, err := contarLineas(agents)
	return []Resultado{
		perfil(p),
		agentsRaiz(n, err, p.Limites.LineasMaxAgentsRaiz),
		links(raiz, agents),
		claudeMd(raiz),
		agentsDominio(raiz, p.Limites.LineasMaxAgentsDominio),
	}
}

// TodoOK indica si todas las verificaciones pasaron.
func TodoOK(rs []Resultado) bool {
	for _, r := range rs {
		if !r.OK {
			return false
		}
	}
	return len(rs) > 0
}

func perfil(p *proyecto.Perfil) Resultado {
	if errs := p.Validar(); len(errs) > 0 {
		return Resultado{"perfil .harness/harness.yaml", false, strings.Join(errs, "; ")}
	}
	return Resultado{"perfil .harness/harness.yaml", true, fmt.Sprintf("nivel %s, modo %s", p.Nivel, p.Modo)}
}

func agentsRaiz(n int, err error, max int) Resultado {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Resultado{"AGENTS.md", false, "no existe: es la fuente principal de contexto (D-0010)"}
	case err != nil:
		return Resultado{"AGENTS.md", false, err.Error()}
	case n > max:
		return Resultado{"AGENTS.md", false, fmt.Sprintf("tiene %d líneas (máx %d): es un mapa, no un manual", n, max)}
	}
	return Resultado{"AGENTS.md", true, fmt.Sprintf("%d líneas", n)}
}

func links(raiz, agents string) Resultado {
	b, err := os.ReadFile(agents)
	if err != nil {
		return Resultado{"links de AGENTS.md", false, "AGENTS.md ilegible"}
	}
	var rotos []string
	vistos := map[string]bool{}
	for _, m := range rutaCitada.FindAllStringSubmatch(string(b), -1) {
		ruta := m[1]
		// Solo se verifican rutas: un nombre sin carpeta (`hooks.json`) es una mención, no un link.
		if vistos[ruta] || !strings.Contains(ruta, "/") || strings.ContainsAny(ruta, "<>{}") || strings.Contains(ruta, "://") {
			continue
		}
		vistos[ruta] = true
		abs := filepath.Join(raiz, filepath.FromSlash(ruta))
		if strings.ContainsAny(ruta, "*?[") {
			if coincide, _ := filepath.Glob(abs); len(coincide) == 0 {
				rotos = append(rotos, ruta)
			}
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			rotos = append(rotos, ruta)
		}
	}
	if len(rotos) > 0 {
		return Resultado{"links de AGENTS.md", false, "rotos: " + strings.Join(rotos, ", ")}
	}
	return Resultado{"links de AGENTS.md", true, fmt.Sprintf("%d rutas citadas, todas existen", len(vistos))}
}

func claudeMd(raiz string) Resultado {
	b, err := os.ReadFile(filepath.Join(raiz, "CLAUDE.md"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Resultado{"CLAUDE.md", true, "no existe (correcto: AGENTS.md se lee de forma nativa)"}
	case err != nil:
		return Resultado{"CLAUDE.md", false, err.Error()}
	case !strings.Contains(string(b), "@AGENTS.md"):
		return Resultado{"CLAUDE.md", false, "existe sin @AGENTS.md: anula la lectura nativa de AGENTS.md (D-0025); agrégale la línea @AGENTS.md"}
	}
	return Resultado{"CLAUDE.md", true, "importa @AGENTS.md"}
}

func agentsDominio(raiz string, max int) Resultado {
	var largos []string
	revisados := 0
	_ = filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ruta != raiz && ignorar[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "AGENTS.md" || filepath.Dir(ruta) == filepath.Clean(raiz) {
			return nil
		}
		revisados++
		if n, err := contarLineas(ruta); err == nil && n > max {
			rel, _ := filepath.Rel(raiz, ruta)
			largos = append(largos, fmt.Sprintf("%s (%d)", filepath.ToSlash(rel), n))
		}
		return nil
	})
	if len(largos) > 0 {
		return Resultado{"AGENTS.md de dominio", false, fmt.Sprintf("superan %d líneas: %s", max, strings.Join(largos, ", "))}
	}
	return Resultado{"AGENTS.md de dominio", true, fmt.Sprintf("%d revisados", revisados)}
}

// contarLineas cuenta en streaming, sin cargar el archivo (RG-1.8).
func contarLineas(ruta string) (int, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}
