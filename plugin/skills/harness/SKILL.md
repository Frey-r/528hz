---
name: harness
description: Procedimiento del harness 528hz. Úsala en todo proyecto con .harness/ (el contexto dice «Harness 528hz activo») antes de escribir código, al elegir entre alternativas, al retomar tras un corte y antes de entregar una tarea.
---

# Cómo se trabaja con el harness 528hz

`hz` es el CLI determinista del harness: tú decides el *qué* (contenido, criterio); `hz` garantiza el *cómo* (IDs, formato, hashes, orden). Úsalo tú mismo; el humano no debería tener que pedírtelo.

## Al empezar o retomar
1. `hz estado` → tareas TDD abiertas, últimas decisiones, misión activa. No reconstruyas el estado leyendo transcripts.
2. Antes de proponer un enfoque: `hz decision buscar <tema>`. No reabras un enfoque descartado sin evidencia nueva.

## Ciclo obligatorio por tarea (RG-2: test-driven siempre)
1. **CONTRATO** — escribe el contrato en `contratos/` (firmas, esquemas, criterios de aceptación y presupuesto de recursos RG-1). Registra: `hz tdd contrato T-n --titulo "…" --contrato <ruta>`. En features nuevas o Génesis, pide al humano que apruebe el contrato y agrega `--aprobado`. IDs correlativos: mira `hz tdd estado`.
2. **ROJO** — escribe los tests desde el contrato y *stubs* con las firmas, para que fallen por aserción y no por compilación o import. Registra: `hz tdd rojo T-n --tests ruta1,ruta2`. Desde aquí esos tests quedan **bloqueados**.
3. **VERDE** — implementa hasta que `hz tdd verde T-n` pase.
4. **REFACTOR** — mejora sin tocar los tests: `hz tdd refactor T-n`.

Si `hz` o un hook rechazan algo, el mensaje trae la remediación: síguela. Nunca edites un test bloqueado para lograr verde; si el test está mal, registra la decisión y pide aprobación.

## Decisiones (RG-3)
Registra con `hz decision nueva --que "…" --por-que "…" --decidio agente [--alternativa "opción|motivo del descarte"]… [--alcance x]… [--etiqueta y]…` cuando:
- eliges entre alternativas que alguien podría reabrir (arquitectura, dependencias, estructura);
- **descartas un enfoque** (lo intentado y por qué no);
- reviertes, haces una excepción a una RG o RD, o cambias contratos, métricas o normas.

No registres ediciones triviales. En commits, agrega el trailer `Decision: D-NNNN`.

## Reglas
- **RG-1** (código de producción, eficiente en recursos): `hz regla ver RG-1` antes de crear conexiones, clientes o instancias.
- **Reglas duras (RD)** del proyecto: `.harness/reglas.yaml`. Son inquebrantables. Si un hook te deniega algo, no busques un rodeo: intentar evadir una RD es un error crítico.

## Antes de entregar
`hz check` y la suite del proyecto (comandos en `.harness/harness.yaml`). Entrega con evidencia ejecutada (comando + resultado). Que los tests pasen no basta: `hz mutar --archivo <src> --tarea T-n` mide si detectan fallas (umbral `verificacion.mutacion_min`).

## Ver qué está pasando
`/hz-panel` abre el panel de agentes junto al chat (se abre solo al lanzar el primer subagente). `hz panel` muestra lo mismo en la terminal. `hz dashboard` abre la vista web local (árbol + carriles de actividad) en http://127.0.0.1:5280.

## Lo que no haces
- No editas `.harness/reglas.yaml`, `.harness/log/`, `.claude/settings*.json` ni `.githooks/` (solo el humano o `hz`).
- No creas `CLAUDE.md`: anula la lectura de `AGENTS.md`.
- No imprimes, copias ni transformas secretos.
