package proyecto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Protegidos son las rutas de autoprotección (D-0026). No incluye todo .claude/**:
// las lecciones viven en .claude/skills/ y el curador debe poder escribirlas.
var Protegidos = []string{
	".harness/reglas.yaml",
	".harness/log/**",
	".claude/settings.json",
	".claude/settings.local.json",
	".githooks/**",
}

// DenyNativos calcula las reglas deny de Claude Code: autoprotección + RD vigentes con capa `permisos`.
func DenyNativos(reglas []ReglaDura) []string {
	var out []string
	for _, p := range Protegidos {
		out = append(out, "Edit(./"+p+")", "Write(./"+p+")")
	}
	for _, r := range Vigentes(reglas) {
		if !r.TieneCapa("permisos") {
			continue
		}
		for _, ruta := range r.Patrones.Rutas {
			ruta = "./" + strings.TrimPrefix(filepathSlash(ruta), "./")
			out = append(out, "Read("+ruta+")", "Edit("+ruta+")", "Write("+ruta+")")
		}
		out = append(out, r.Patrones.Herramientas...)
	}
	return out
}

func filepathSlash(s string) string { return strings.ReplaceAll(s, `\`, "/") }

// par es una clave JSON con su valor crudo, para conservar el orden del archivo del usuario.
type par struct {
	clave string
	valor json.RawMessage
}

func decodificarObjeto(b []byte) ([]par, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("se esperaba un objeto JSON")
	}
	var pares []par
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		pares = append(pares, par{t.(string), v})
	}
	return pares, nil
}

func codificarObjeto(pares []par) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range pares {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, _ := json.Marshal(p.clave)
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(p.valor)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

func fijar(pares []par, clave string, valor json.RawMessage) []par {
	for i := range pares {
		if pares[i].clave == clave {
			pares[i].valor = valor
			return pares
		}
	}
	return append(pares, par{clave, valor})
}

func obtener(pares []par, clave string) json.RawMessage {
	for _, p := range pares {
		if p.clave == clave {
			return p.valor
		}
	}
	return nil
}

// FusionarPermisos agrega reglas a .claude/settings.json → permissions.deny sin borrar ni duplicar,
// conservando el orden y las demás claves del archivo.
func FusionarPermisos(raiz string, reglas []string) (Accion, error) {
	const rel = ".claude/settings.json"
	ruta := filepath.Join(raiz, rel)
	b, err := os.ReadFile(ruta)
	nuevo := errors.Is(err, os.ErrNotExist)
	if err != nil && !nuevo {
		return Accion{}, err
	}
	if nuevo {
		b = []byte("{}")
	}
	raizObj, err := decodificarObjeto(b)
	if err != nil {
		return Accion{}, fmt.Errorf("%s: %w", rel, err)
	}
	permisos := []par{}
	if crudo := obtener(raizObj, "permissions"); crudo != nil {
		if permisos, err = decodificarObjeto(crudo); err != nil {
			return Accion{}, fmt.Errorf("%s permissions: %w", rel, err)
		}
	}
	var deny []string
	if crudo := obtener(permisos, "deny"); crudo != nil {
		if err := json.Unmarshal(crudo, &deny); err != nil {
			return Accion{}, fmt.Errorf("%s permissions.deny: %w", rel, err)
		}
	}
	presentes := make(map[string]bool, len(deny))
	for _, d := range deny {
		presentes[d] = true
	}
	agregadas := 0
	for _, r := range reglas {
		if !presentes[r] {
			deny = append(deny, r)
			presentes[r] = true
			agregadas++
		}
	}
	if agregadas == 0 && !nuevo {
		return Accion{rel, "existía"}, nil
	}
	denyJSON, _ := json.Marshal(deny)
	permisos = fijar(permisos, "deny", denyJSON)
	raizObj = fijar(raizObj, "permissions", codificarObjeto(permisos))
	var salida bytes.Buffer
	if err := json.Indent(&salida, codificarObjeto(raizObj), "", "  "); err != nil {
		return Accion{}, err
	}
	salida.WriteByte('\n')
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return Accion{}, err
	}
	if err := os.WriteFile(ruta, salida.Bytes(), 0o644); err != nil {
		return Accion{}, err
	}
	if nuevo {
		return Accion{rel, "creado"}, nil
	}
	return Accion{rel, "modificado"}, nil
}
