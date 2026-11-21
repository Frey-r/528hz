// Package reglas extrae secciones de las reglas generales embebidas.
package reglas

import "strings"

// Seccion devuelve el texto de la sección `## <id> …` hasta la siguiente sección de nivel 2.
func Seccion(md, id string) (string, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	lineas := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	inicio := -1
	for i, l := range lineas {
		if !strings.HasPrefix(l, "## ") {
			continue
		}
		if inicio >= 0 {
			return limpiar(lineas[inicio:i]), true
		}
		titulo := strings.ToUpper(strings.TrimPrefix(l, "## "))
		if titulo == id || strings.HasPrefix(titulo, id+" ") {
			inicio = i
		}
	}
	if inicio >= 0 {
		return limpiar(lineas[inicio:]), true
	}
	return "", false
}

func limpiar(lineas []string) string {
	texto := strings.TrimSpace(strings.Join(lineas, "\n"))
	return strings.TrimSpace(strings.TrimSuffix(texto, "---"))
}
