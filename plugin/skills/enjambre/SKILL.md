---
name: enjambre
description: Procedimiento del director del harness 528hz para construir varias funcionalidades (o atacar varios clusters de una ronda de mejora) a la vez con células de agentes en paralelo. Úsala cuando el usuario pida un enjambre o cuando el frente listo tenga más de una tarea independiente.
---

# Enjambre: el director y sus células

Tú eres el **director** (la sesión principal). No editas código: planificas, pides el gate humano, lanzas líderes y supervisas tarjetas. Detalle en `docs/diseno/11-orquestacion.md` del harness.

## 1. Plan (sin gastar en volumen)
1. `hz estado`, `hz tablero --listo` y `hz capacidad`. No lances más células de las que caben.
2. Agrupa el frente listo en **células**, una por funcionalidad o cluster, con archivos disjuntos. Si dos células tocan el mismo archivo, sepáralas en el tiempo o asígnalo a una sola.
3. Elige el modelo de cada rol según riesgo y complejidad (D-0024); anótalo en el plan.
4. Presenta al humano: células, tareas por célula, modelos, costo estimado y capacidad. **Espera su OK explícito**: es gasto en volumen.

## 2. Lanzamiento
Por cada célula, en un solo mensaje y en paralelo:
- `subagent_type: "hz:lider"`, `isolation: "worktree"`, `run_in_background: true`
- `description: "lider <celula> <T-n primera>"`
- `prompt`: la célula, sus tareas en orden (`hz tablero --celula <c>`) y la salida de `hz contexto --rol lider --tarea <T-n>`.

## 3. Supervisión
- Espera las notificaciones; no hagas polling.
- Por cada líder que termina: `hz tarjeta ultima <T-n> --rol lider` y `hz salida --rol lider --tarea <T-n>`.
- Si llega una ALERTA del vigía, o un líder escala: pide estado por SendMessage (al `agentId` que da `hz equipo`) o detenlo y relanza uno fresco con la tarjeta de falla.
- Al humano solo se le avisa por gate, bloqueo, milestone o fin.

## 4. Integración y cierre
1. Lanza `hz:integrador` con las ramas de las células en orden de dependencias. Integra una por una y corre la suite completa entre cada merge.
2. `hz estado` y `hz check` en verde.
3. Informe C-level al humano: qué se entregó por célula, evidencia, decisiones (`hz decision buscar`) y costo.
