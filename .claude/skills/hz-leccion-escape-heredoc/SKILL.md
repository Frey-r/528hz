---
name: hz-leccion-escape-heredoc
description: Úsala antes de escribir o modificar con Bash (heredoc, python -c, python - <<EOF, sed) un archivo cuyo contenido lleve barras invertidas: regex (\b, \d), escapes ("\n", "\\"), rutas de Windows. También antes de un reemplazo en lote sobre varios archivos.
---

# Lección: las barras invertidas no sobreviven a un heredoc de Bash

## Qué salió mal (clase `escape-heredoc-bash`, 6 veces)
Generé o reemplacé código con scripts de Python pasados por un heredoc de Bash. Al llegar al archivo o al script:
- `'\\'` se colapsó en `'\'`, y el string quedó sin terminar;
- `'\\n'` (barra + n literal, p. ej. el texto a buscar en un `.go`) llegó como `'\n'`, un salto de línea real, y el reemplazo no encontró nada o escribió otra cosa;
- `\b` de un regex se volvió el carácter de control retroceso (0x08), invisible al leer el archivo.

**Pasa incluso con el heredoc entre comillas (`<<'EOF'`)**: no es la shell, es el transporte del comando. Lo que se ve en el comando no es lo que llega.

## Patrón correcto
1. Si el contenido nuevo **o el texto a buscar** lleva `\`, usa las herramientas **Edit o Write**: no pasan por ninguna shell. Para varios archivos, una llamada a Edit por archivo.
2. Si de verdad necesitas Python (reemplazos masivos), no escribas barras en el script: créalas con `chr(92)`. Ejemplo: `b = chr(92); viejo = '"version: 1' + b + 'n"'`.
3. Nunca uses `sed` con barras en el patrón ni en el reemplazo.

## Cómo verificarlo (siempre, después de escribir)
- No deben quedar caracteres de control: `grep -nP "[\x00-\x08\x0B\x0C\x0E-\x1F]" <archivo>` no debe imprimir nada.
- Relee la línea cambiada y compara las secuencias de escape una por una.
- Compila o carga los tests del archivo antes de seguir.
- Si Edit no encuentra un texto que «se ve igual», sospecha de caracteres invisibles y reemplaza la línea completa con Write.
