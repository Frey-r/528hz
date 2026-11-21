---
name: mision
description: Procedimiento de una misión larga del harness 528hz (nivel N3) en modo Génesis o Mejora continua - verticales, rondas medidas con sensores, comparación con ruido, milestones y presupuesto. Úsala cuando el humano pida mejorar un producto por rondas, iniciar una misión, medir una ronda, comparar rondas, proponer un milestone o cerrar una misión.
---

# Misión del harness 528hz

Los números de una misión salen de `hz` (sensores deterministas y estadística), nunca de contar a ojo. Tú conduces; el humano define y aprueba.

## Arranque
1. `hz estado`: si ya hay una misión activa, retómala desde su fase y su próximo paso.
2. `hz mision nueva <id> --modo genesis|mejora --objetivo "…" [--usd-max n]` crea `.harness/misiones/<id>/mision.yaml`.
3. **Mejora:** define las verticales esenciales **conversando con el humano**. Puedes sugerir, pero las aprueba él.
   - Cada vertical mide tres familias: **efectividad** (qué tan bien lo hace), **impacto** (cuánto importa) y **fallos** (tasa de fallos).
   - Cada métrica tiene `sensor` (comando determinista que imprime `[{"item_id", "resultado"|"puntaje"}]`; `{ronda}` se reemplaza), `direccion` (sube|baja), `meta` y, si se conoce, `linea_base` y `piso_ruido`.
   - Escribe las verticales en `mision.yaml` y revisa con el humano las reglas duras del proyecto.
4. **Génesis:** spec, non-goals, criterios de aceptación y reglas duras con el humano; después, hitos en slices verticales con TDD.

## Ronda de mejora
1. `hz ronda nueva <m> --foco V-a`.
2. Implementa el foco con el ciclo TDD (`hz tdd …`). Para varios frentes, usa la skill `hz:enjambre`.
3. `hz ronda medir <m> R-n`: corre los sensores y registra la medición.
4. `hz compare <m> R-a R-b`: comparación pareada con test de signos exacto. Resultado: `mejora`, `regresion` o `ruido`.
   - Sin piso de ruido conocido, mide dos veces el mismo código (dos rondas sin cambios) y corre `hz ruido <m> R-a R-b`.
5. `hz ronda cerrar <m> R-n --veredicto mantener|revertir`: con regresión fuera de ruido, revierte el código y cierra con `revertir`.
6. **Milestones (D-0019):** al cerrar con `mantener`, el harness **siempre pregunta** y, si el salto es evidente y grande, adjunta su propuesta (`hz milestone proponer`).
   - Pregúntale al humano con esa justificación.
   - **Solo él marca:** `hz milestone marcar <m> R-n --resumen "qué cambió" [--propuesta]`.
7. Registra las decisiones de la ronda (`hz decision nueva …`) y el gasto (`hz mision gasto <m> --usd x --concepto "…"`).

## Presupuesto
- Si el gasto llega a `presupuesto.usd_max`, el plugin **deniega lanzar subagentes**.
- No lo rodees: pide al humano ampliar el tope (en `mision.yaml`, seguido de `hz exportar`) o cierra la misión.

## Cierre
- El contrato es default-FAIL: cada criterio de `aceptacion` necesita evidencia ejecutada.
  - Márcala con `hz mision aceptar <m> <criterio> --evidencia "comando → resultado"`.
- `hz mision cerrar <m>` devuelve código 1 mientras falte evidencia.
- Al cerrar, entrega el informe C-level con la skill `hz:informe`.
