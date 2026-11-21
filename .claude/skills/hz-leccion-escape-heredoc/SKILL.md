---
name: hz-leccion-escape-heredoc
description: Úsala antes de escribir o modificar con Bash (heredoc, python -c, python - <<EOF, sed) un archivo cuyo contenido lleve barras invertidas: regex (\b, \d), escapes ("\n", "\\"), rutas de Windows.
---

# Lección: las barras invertidas no sobreviven a un heredoc de Bash

## Qué salió mal (clase `escape-heredoc-bash`, 4 veces en una sesión)
Generé código con scripts de Python pasados por un heredoc de Bash. Al llegar al archivo:
- `'\\'` se colapsó en `'\'`, y el string quedó sin terminar;
- `'\n'` se volvió un salto de línea real dentro de un literal;
- `\b` de un regex se volvió el carácter de control retroceso (0x08), invisible al leer el archivo.

## Por qué
El texto pasa por varias capas de escape antes de llegar al archivo: la herramienta, la shell y el string de Python. Cada capa puede consumir una barra. Lo que se ve en el comando no es lo que se escribe.

## Patrón correcto
1. Si el contenido nuevo lleva `\`, escríbelo con las herramientas **Write o Edit**. Esas no pasan por ninguna shell.
2. Si de verdad necesitas Python (por ejemplo, reemplazos masivos), no escribas barras en el script: créalas con `chr(92)`. Ejemplo: `b = chr(92); linea = "/" + b + "bT-(" + b + "d+)" + b + "b/"`.
3. Nunca uses `sed` con barras dentro del reemplazo.

## Cómo verificarlo (siempre, después de escribir)
- No deben quedar caracteres de control: `grep -nP "[\x00-\x08\x0B\x0C\x0E-\x1F]" <archivo>` no debe imprimir nada.
- Compila o carga los tests del archivo antes de seguir.
- Si la herramienta Edit no encuentra un texto que «se ve igual», sospecha de caracteres invisibles y reemplaza la línea completa con Write.
