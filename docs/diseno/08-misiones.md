# 08 · Misiones (E)

Una **misión** es trabajo largo con objetivo, contrato de cierre, presupuesto y estado persistente. Vive en `.harness/misiones/<id>/`, con sus datos en SQLite.

```
.harness/misiones/<id>/
  mision.yaml      objetivo, modo, verticales (Mejora) o hitos (Génesis), presupuesto, gates, aceptación default-FAIL
  estado.json      fase actual, ronda o hito, próximo paso, procesos en background (PID + comando)
  PROGRESO.md      handoff humano legible (lo regenera `hz estado --md`)
```

## Génesis

| Fase | Entregable | Aprobación humana |
|---|---|---|
| 1. Intención | Spec: objetivo, usuarios, non-goals, criterios de aceptación, **RD** | Sí |
| 2. Arquitectura | Dominios, stack, fronteras y **contratos globales**; cada elección es una decisión | Sí |
| 3. Andamiaje | Estructura, AGENTS.md, enforcement de RD, comando único, CI local en verde | Revisión |
| 4. Hitos | Slices verticales; cada tarea CONTRATO → ROJO → VERDE → REFACTOR; células en paralelo | Contratos de features |
| 5. Endurecimiento | Red-team de RD, E2E, meta-evals | Revisión |
| 6. Entrega | Contrato de la misión en verde con evidencia | Sí |

## Mejora continua

### 1. Verticales esenciales (primer paso, obligatorio)
Las verticales son las dimensiones del producto que concentran el valor. Sirven para **enfocar** las mejoras: toda mejora se etiqueta con la o las verticales que ataca.

**Se definen conversando con el usuario** (D-0022). El harness puede sugerir candidatas, pero las decide el usuario. Son las **métricas base** para medir la efectividad del producto y parametrizar bien su funcionamiento. Si el producto hace algo, hay que medir tres cosas de eso, y cada vertical lleva al menos una métrica de cada familia:

| Familia | Pregunta | Ejemplo |
|---|---|---|
| `efectividad` | ¿Qué tan bien lo hace? | % de casos resueltos correctamente |
| `impacto` | ¿Qué cambia gracias a eso? | Horas ahorradas, tiempo de respuesta, conversión |
| `fallos` | ¿Con qué frecuencia falla? | % de errores o acciones incorrectas |

```yaml
verticales:
  - id: V-resolucion
    nombre: "Resolución autónoma"
    peso: 0.4
    metricas:
      - id: tasa_resolucion
        familia: efectividad    # efectividad | impacto | fallos
        sensor: "bun run evals/resolucion.ts --ronda {ronda}"   # determinista, emite JSON por ítem
        direccion: sube
        linea_base: null        # se mide en el paso 1
        meta: 0.25
        piso_ruido: null        # se mide con `hz ruido`
  - id: V-calidad
    nombre: "Calidad de respuesta"
    peso: 0.35
    metricas: [...]
  - id: V-costo
    nombre: "Costo por caso"
    peso: 0.25
    metricas: [...]
```
- **Línea base:** se mide antes de tocar nada.
- **Piso de ruido:** `hz ruido` corre la misma ronda dos veces sobre una muestra y mide la variación natural.
- **Holdout:** un conjunto fuera del repo que solo ve el evaluador y que **rota** en cada milestone.
- En este paso también se definen o revisan las **RD** del proyecto.
- El humano aprueba las verticales.

### 2. Rondas (iteraciones; el proceso interno)
1. **Observar:** analizar la ronda anterior ítem a ítem.
2. **Agrupar** los fallos en clusters.
3. **Diagnosticar** en paralelo, un diagnosticador por cluster.
4. **Planear con costo**, eligiendo el foco por (brecha a la meta × peso) por vertical. Pasa por un gate si supera el presupuesto.
5. **Contrato + test/eval en rojo** de cada mejora.
6. **Implementar** con mini-células por cluster, con archivos disjuntos.
7. **Verificar** (V0 a V3 y V5).
8. **Foto:** `hz snapshot` congela el commit de la ronda en un worktree.
9. **Medir** todas las verticales con su sensor determinista.
10. **Comparar** con `hz compare`: pareo por ítem, ganados y perdidos, banda de ruido, holdout.
11. **Decidir:** mantener o revertir, con decisión registrada.
12. **Registrar** mediciones, errores y lecciones.

### 3. Milestones (rondas destacadas por su salto)
**Los define el usuario** (D-0019). El harness nunca promueve un milestone por su cuenta:
- **Pregunta constantemente.** Al cerrar cada ronda, el resumen termina con la pregunta «¿Marcamos esta ronda como milestone?» y los datos para decidir: delta por vertical contra el último milestone, ruido y holdout.
- **Propone cuando la mejora es evidente y grande.** Si una ronda mejora al menos una vertical por sobre **k × piso de ruido** (k = 2 por defecto) frente al milestone anterior, sin regresiones fuera de ruido en las demás ni en holdout, la propone de forma explícita con su justificación.
- **La comparación principal es entre milestones.** Las rondas son el proceso interno; los milestones son lo que se informa.

### 4. Generalización
En cada milestone se mide el holdout. Una caída frente al set de trabajo indica **sobreajuste**: se registra como error de clase «sobreajuste» y condiciona la siguiente ronda.

### 5. Cierre
Contrato de la misión en verde: metas de las verticales cumplidas en dos milestones consecutivos y holdout dentro de la tolerancia.

## Monitoreo de producción (opcional, por proyecto)

Es el pilar 8 de `general_description.md`, adaptado (D-0032). **Se activa solo si el proyecto lo declara**, y la herramienta depende del proyecto: el harness no impone ninguna.

```yaml
# .harness/harness.yaml
monitoreo:
  habilitado: true
  skill: grafana-loki            # la skill de monitoreo del proyecto (otra: datadog, sentry, cloudwatch…)
  fuentes:                       # qué consultar, en términos de esa skill
    - app: mi-servicio
      namespace: produccion
  usos: [sensores, triage]       # cualquiera de los dos
  acceso: solo_lectura           # el monitoreo de producción nunca escribe
```

### Usos
1. **Sensores de verticales en producción** (modo Mejora). Las métricas de **impacto** y **fallos** se pueden medir con datos reales de producción además de la batería local. Ejemplos: tasa de errores en logs, latencia, volumen atendido. El sensor llama a la skill declarada y emite JSON por ítem como cualquier otro sensor, así que `hz compare` lo trata igual. Sirve para ver si un milestone se sostiene en producción.
2. **Triage de incidentes** (misión corta de tipo `incidente`):
   1. Llega una alerta, o el humano reporta un fallo.
   2. Con la skill de monitoreo se correlacionan logs y trazas con el último despliegue.
   3. Se reproduce el fallo localmente o en un worktree.
   4. Se escribe el **test en ROJO** que captura el fallo.
   5. Se corrige con el ciclo TDD normal.
   6. Se entrega al humano un diagnóstico C-level más la decisión registrada.
   7. Se registra el error y, si corresponde, su lección.

### Reglas
- El acceso es **siempre de solo lectura**. Ninguna acción sobre producción ocurre sin gate humano y, si el proyecto lo declara, sin una RD.
- Si el proyecto no declara `monitoreo`, nada de esto se carga y no consume contexto.
- El rol que consulta la skill es el **diagnosticador**, que recibe solo la porción de monitoreo y sin código.

## `hz compare`
- **Entrada genérica:** dos `results.json` con `[{item_id, resultado|puntaje, vertical}]`.
- **Salida:** delta por vertical, ganados y perdidos, McNemar (binarios) o bootstrap (continuos), banda de ruido y veredicto: `mejora | ruido | regresion`.
- Los números siempre salen de aquí y nunca del conteo del modelo.

## Estado y retoma
- El estado se escribe en **cada transición de fase**.
- `hz estado` devuelve ≤40 líneas: misión, fase, ronda o hito, último milestone, gates pendientes, procesos en background y próximo paso. **Meta: retomar tras un apagón, una compactación o un reinicio en ≤5 tool calls.**
- **Procesos en background:** el registro de PIDs y comandos permite a `hz estado` decir qué murió y cómo relanzarlo.

## Presupuesto y circuit breakers
`mision.yaml → presupuesto`:
- `usd_max`
- `rondas_por_dia`
- `agentes_paralelos_max`
- `procesos_bg_max` (límite de la máquina)
- `tokens_por_agente`

Al alcanzar un límite, el hook deniega con remediación y se envía una PushNotification. Los **gates de gasto** (LLM en volumen) usan el mismo mecanismo que las RD: no bloqueantes, aprobables con `/hz aprobar G-x`.

## Rol humano
- **Aprueba:** spec, RD, contratos de features, verticales, gates, milestones y cierre.
- **Se le notifica solo en:** gate, bloqueo, milestone o fin. **Nunca** por progreso rutinario.
