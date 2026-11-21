# 06 · Test-driven, verificación, evals y lecciones (C)

## 1. Ciclo obligatorio por tarea (RG-2)

```
CONTRATO ──► ROJO ──► VERDE ──► REFACTOR ──► (verificación)
   │           │         │          │
 contratos/  tests     código    tests intactos
 aprobados   bloqueados  pasa      y en verde
```

| Estado | Qué ocurre | Comando que lo registra | Qué permite el hook |
|---|---|---|---|
| **CONTRATO** | Se define la estructura como contrato: interfaces y firmas, esquemas, contratos de API, fronteras, criterios de aceptación y **presupuesto de recursos (RG-1)**. En features y en Génesis, el humano los aprueba. | `hz tdd contrato T-12 --aprobado` | Editar `contratos/` y tests. **Denegado: código de producción** |
| **ROJO** | Los tests (aceptación, unitarios, de contrato, de recursos) se escriben desde el contrato y se corren. | `hz tdd rojo T-12`: verifica que **fallan por aserción**, no por import o sintaxis, y bloquea esos tests (hash) | Editar el código de producción de los archivos arrendados |
| **VERDE** | El código se construye para pasar los tests. | `hz tdd verde T-12`: pasan los tests de la tarea y la suite afectada | **Denegado: editar tests bloqueados** (salvo decisión + aprobación del evaluador o del humano) |
| **REFACTOR** | Se mejora el código sin tocar el comportamiento. | `hz tdd refactor T-12`: los tests siguen intactos y en verde | Igual que VERDE |

- **Exenciones** (declaradas en `harness.yaml → tdd.exenciones`): docs, configuración trivial y renombres mecánicos. Toda exención fuera de la lista exige una decisión.
- **Modo Mejora:** cada mejora empieza con el test o eval que captura el comportamiento deseado, en rojo, y nunca con el cambio.

## 2. Escalera de verificación

| Nivel | Qué | Tipo | Costo |
|---|---|---|---|
| V0 | Formato, lint, tipos, sensores RG-1, fronteras (`hz check`) | Determinista | segundos |
| V1 | Unitarios afectados (mapa de impacto) | Determinista | segundos a minutos |
| V2 | Integración y contrato | Determinista | minutos |
| V3 | E2E sobre la app real (Playwright/Chrome) | Determinista | minutos |
| V4 | Evals de comportamiento (LLM-judge, comparación con línea base) | Probabilístico | $$; gate por presupuesto |
| V5 | Evaluador escéptico: contexto fresco, solo lectura, rúbrica (contratos, RG, RD), holdout. En perfil crítico, también el humano | Probabilístico | $ |

- **«Listo»** significa evidencia ejecutada (comando + resultado + commit) en el contrato de la tarea, que es **default-FAIL**: cada criterio arranca en `false`.
- **Holdout:** escenarios fuera del repo que solo ve el evaluador.
- **Flaky:** van a cuarentena registrada; un test en cuarentena no cuenta como verde.

## 3. Evals para todo, también lo determinista

### 3.1 Evals del producto
- **Con IA:** V4, pareado contra línea base, con piso de ruido.
- **Sin IA:** suites de escenarios **con puntaje**, no solo pass/fail: corrección sobre datasets, rendimiento (p95, throughput) y consumo de recursos (RG-1).

### 3.2 Meta-evals de verificadores y reglas
Cada verificador (test, lint, sensor, gate, RD) se evalúa a sí mismo:

| Métrica | Cómo se mide |
|---|---|
| **Detección (recall)** | Se siembran fallas: mutation testing y errores reales del registro reproducidos como casos. Se mide el % que atrapa |
| **Falsas alarmas** | Fallos sin bug real; bloqueos que el humano aprobó |
| **Validez del sensor** | El script de métrica contra un set de oro etiquetado a mano |
| **Costo / tiempo** | Duración y gasto por corrida |
| **Red-team (RD)** | Casos de evasión bloqueados / pasados (`05-reglas-duras.md`) |

**Regla del escape:** si un error llegó más allá de V3, se responde obligatoriamente «¿por qué no lo detectó?». La respuesta es un test, una regla o un sensor nuevo, y queda como decisión.

**La detección es criterio de salida (D-0046).** El verificador corre `hz mutar --archivo <src> --tarea T-n` sobre cada archivo de código que tocó la tarea. `hz salida --rol verificador` exige una detección ≥ `verificacion.mutacion_min` (0,9 por defecto; 0 la desactiva) en cada archivo medido y en cada archivo de código arrendado por la tarea. `hz mutar` muestrea los sitios repartidos en todo el archivo: tomar solo los primeros sesgaba la medición hacia el comienzo, y al corregirlo bajaron de 100 % a 40–85 % varios archivos del propio harness.

### 3.3 Evals del agente y de sus lecciones
- Tasa de error por cambio y por rol.
- Recurrencia por clase de error.
- A/B de cada lección (§4.5).

## 4. Ciclo error → lección → eval

### 4.1 Qué es un error
Solo se registra lo que ocurre en un **punto de entrega**. El rojo del TDD **no** es error.

| Fuente | Ejemplo |
|---|---|
| Verificación fallida al entregar | El implementador entrega y el verificador encuentra V1 en rojo |
| Rechazo del evaluador | Viola un contrato o RG-1 |
| Corrección del humano | «Eso está mal, debía ir en dev» |
| Intento contra una RD | Bloqueo de la capa 1 o 2 |
| Revert | El commit se revierte |
| Regresión de ronda | `hz compare` marca una regresión fuera de ruido |

### 4.2 Registro (`hz error nuevo`)
Campos:
- **cuándo:** misión, ronda o hito, sesión, commit;
- **quién:** agente y rol;
- tarea;
- qué hizo;
- qué verificación lo detectó, o `escapó`;
- **causa raíz** según taxonomía: contrato ambiguo, contexto faltante, regla ignorada, patrón incorrecto, herramienta mal usada, test débil, otra;
- **severidad:**
  - `crítico`: seguridad, pérdida de datos, producción, RD;
  - `mayor`: rompe build/tests o comportamiento entregado;
  - `menor`: interpretación, estilo, re-trabajo;
- situación reproducible: commit + tarea + archivos.

### 4.3 Deduplicación por clase
Cada error se asigna a una **clase**: causa raíz + ámbito (dominio/archivo) + patrón. Si la clase ya existe, suma recurrencia a su lección en vez de crear otra.

### 4.4 La lección (skill)
`.claude/skills/hz-leccion-<clase>/SKILL.md`:
```markdown
---
name: hz-leccion-cliente-http-por-request
description: Úsala al crear o editar handlers en app/api/ que llamen servicios externos.
---
## Qué salió mal
## Por qué
## Patrón correcto (con ejemplo del repo)
## Cómo verificarlo (comando)
```
- La descripción es el **disparador**: tiene que ser precisa, porque se carga siempre en el contexto.
- Si la clase se puede detectar con un sensor determinista (lint, test, hook), el curador crea **también** el sensor. La skill explica y el sensor obliga.

### 4.5 Eval de la lección (antes de activarla)
Corre en un worktree, en el commit original, con un subagente **fresco** (rol `hz-curador` → `evaluador-lecciones`):

| Eval | Pregunta | Criterio |
|---|---|---|
| **Prevención** | ¿Evita el error? Caso de reproducción A/B, con y sin la skill | Error en «sin» y no en «con». **Se mide en el proceso** (intentos fallidos, llamadas, costo), no solo en el resultado final: un agente que verifica se recupera del error y termina bien igual, y el costo del error queda en los reintentos (D-0048) |
| **Interferencia** | ¿Rompe algo en tareas vecinas de control? | 0 errores nuevos |
| **Activación** | ¿Se carga cuando corresponde, y solo entonces? | Dispara en el caso; no dispara en los controles ajenos |

Consume LLM, así que corre por lotes (en cada milestone o de noche con `/schedule`) y dentro del presupuesto.

### 4.6 Ciclo de vida

```
en_prueba ──(pasa eval)──► activa ──(error menor de interpretación)──► reescrita ──► en_prueba
    │                        │
    └─(no pasa)─► reescrita  └─(causa error crítico)──► ELIMINADA de raíz
```
- **Error crítico** causado por la lección: se **elimina de raíz** (se borran sus archivos, se registra la decisión) y la clase queda marcada como «requiere sensor determinista».
- **Falla de interpretación menor:** se **reescribe** y se re-evalúa. Tras 2 reescrituras fallidas, escala al humano.
- **Poda:** se retira si no previene nada o si no se dispara en N sesiones.
- **Re-evaluación:** todas las lecciones se re-evalúan al cambiar de modelo.
- **Sin tope de lecciones** (D-0021). Para que el número no llene el contexto, el modo de carga cambia según cuántas haya:
  - **Pocas** (hasta `lecciones.indice_desde`, 30 por defecto, ≈2k tokens; cada skill cuesta ≈69 tokens por request, medido en S3): cada lección es una skill propia con su disparador.
  - **Muchas:** pasan a **modo índice**. Las lecciones se mapean en SQLite por ámbito (dominio, glob de rutas, herramienta, rol). Una sola skill `hz-lecciones` funciona como índice: su descripción es general y su cuerpo explica cómo consultar `hz leccion para <ruta|tarea>`. El hook justo a tiempo (§4.7) entrega solo las lecciones del ámbito que se está tocando, así que el contexto se mantiene constante.
- **Promoción a global** (en el plugin) cuando la misma clase aparece en ≥2 proyectos.

### 4.7 Justo a tiempo
Cuando un agente va a editar dentro del ámbito de una lección activa, el hook `tool.call` agrega una línea: «Aplica L-12 (hz-leccion-x): cárgala antes de editar». Así no hace falta precargar todas las lecciones.
