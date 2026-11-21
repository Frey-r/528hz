# Contrato · CLI `hz` (incremento 3.1)

Binario Go (`cmd/hz`). Es la **fuente de verdad determinista** del harness: los agentes y el plugin no calculan nada que `hz` pueda calcular.

## Convenciones generales
- **Raíz del proyecto:** el primer ancestro del cwd que contiene `.harness/harness.yaml`. Se puede forzar con `--raiz <dir>` o con `HZ_RAIZ`. Sin raíz, todo comando (salvo `activar` y `version`) termina con código 3.
- **Salida:** breve por defecto, pensada para agentes (pocas líneas, con IDs). Con `--json`, un objeto JSON por invocación.
- **Códigos de salida:**

  | Código | Significado |
  |---|---|
  | 0 | OK |
  | 1 | Verificación fallida (`check`, `tdd`) |
  | 2 | Uso incorrecto (flags o argumentos) |
  | 3 | Sin proyecto activado |
  | 4 | Error interno (E/S, base de datos) |

- **Fuente versionada:** `.harness/log/<dominio>.jsonl`, append-only. Formato de línea: `{"evento": "<dominio>.<accion>", "ts": "<RFC3339>", "datos": {…}}`.
- **Índice:** `.harness/harness.db` (SQLite, WAL). Si falta o si el tamaño de algún log cambió desde la última indexación (por ejemplo tras un `git pull` o un merge), `hz` hace `reindex` automáticamente antes de responder.
- **Concurrencia** (enjambres): los IDs se asignan dentro de una transacción `BEGIN IMMEDIATE`, con un `busy_timeout` de 5 s. Cada evento es una sola escritura `O_APPEND`.
- **RG-1:**
  - Una sola conexión SQLite por proceso, que se cierra al salir.
  - Sentencias preparadas reutilizadas en el reindex.
  - Los logs se leen en streaming, nunca completos en memoria.

## Comandos

### `hz version`
Imprime `hz <versión>`.

### `hz activar [--modo genesis|mejora] [--nivel N1|N2|N3]`
Crea el perfil en el cwd (o en `--raiz`). Es **idempotente**: nunca sobrescribe lo que ya existe.
- `.harness/harness.yaml` (si no existe), con `version: 1`, `enabled: true`, nivel, modo e idioma `es`.
- `.harness/log/` y `harness.db`.
- Agrega `.harness/harness.db*` a `.gitignore` si falta.
- `AGENTS.md`:
  - si no existe, crea un esqueleto del mapa;
  - si existe, no lo toca.
- `CLAUDE.md`:
  - **nunca lo crea**;
  - si existe y no contiene `@AGENTS.md`, le agrega la línea al final (D-0025).

Salida: una línea por acción (`creado` / `existía` / `modificado`).

### `hz reindex`
Reconstruye `harness.db` desde `.harness/log/*.jsonl`. Ignora eventos desconocidos (los cuenta) y falla con código 4 si hay una línea JSON inválida, indicando archivo y línea. Salida: `reindex: N eventos (D decisiones, T tareas) en X ms`.

### `hz decision nueva --que "…" --por-que "…" --decidio humano|agente|agente_aprobado [--alternativa "opción|motivo"]… [--alcance x]… [--etiqueta y]… [--mision m] [--ronda r] [--commit c] [--no-reversible]`
- Valida que `que`, `por_que` y `decidio` no estén vacíos. Si falta alguno, termina con código 2 y nombra los campos faltantes.
- Asigna `D-NNNN` como siguiente correlativo, agrega el evento `decision.nueva` e indexa.
- Salida: `D-0033`.

### `hz decision buscar <tema…> [--limite 5]`
Hace una búsqueda de texto (FTS5, insensible a tildes) sobre qué, por qué, etiquetas y alternativas. Salida: hasta `--limite` líneas `D-0007 · vigente · <qué recortado a 100 caracteres>`, ordenadas por relevancia. Sin resultados: `sin decisiones sobre "<tema>"` y código 0.

### `hz decision ver D-NNNN`
Ficha completa (≤20 líneas): cuándo, qué, por qué, alternativas, decidió, alcance, reversible y estado. Si el ID no existe, código 1.

### `hz regla ver RG-n`
Imprime la sección `## RG-n` de las reglas generales (embebidas en el binario desde `reglas/generales.md`). Si la regla no existe, código 1.

### `hz check`
Una línea por verificación (`✓` / `✗ <motivo>`). Termina con código 1 si alguna falla.
1. `harness.yaml` legible y con `version`, `enabled`, `nivel` y `modo` válidos.
2. `AGENTS.md` existe y tiene ≤ `limites.lineas_max_agents_raiz` líneas (100 por defecto).
3. Toda ruta citada entre backticks en `AGENTS.md` que termine en una extensión conocida existe. Los globs se expanden.
4. Si existe `CLAUDE.md`, contiene `@AGENTS.md`.
5. Todo `AGENTS.md` de subcarpeta tiene ≤ `limites.lineas_max_agents_dominio` líneas (40 por defecto).

### `hz estado [--json]`
Resumen de ≤40 líneas para retomar:
- proyecto, nivel y modo;
- conteo de decisiones y las 3 últimas;
- tareas TDD abiertas con su estado;
- misión activa, si `misiones/*/estado.json` existe.

### `hz tdd …`: máquina de estados por tarea

| Subcomando | Precondición | Efecto | Fallo |
|---|---|---|---|
| `contrato T-n --titulo "…" --contrato <ruta> [--aprobado] [--mision m]` | La tarea no existe | Crea la tarea en estado `contrato`. `--aprobado` registra la aprobación | La ruta del contrato no existe → código 1 |
| `rojo T-n --tests a,b [--cmd "…"]` | Estado `contrato` | Corre `--cmd` (o `comandos.test` de `harness.yaml`). **Exige** salida ≠ 0 **y** que no aparezca ningún patrón de error de compilación, import o sintaxis. Bloquea los tests (sha256) → estado `rojo` | Los tests pasan → «no hay ROJO». Error de import o sintaxis → «falla por la razón equivocada» |
| `verde T-n [--cmd "…"]` | Estado `rojo` | Verifica que los tests bloqueados no cambiaron y corre el comando, que debe salir con 0 → estado `verde` | Test bloqueado modificado → código 1 nombrando el archivo. El comando falla → código 1 |
| `refactor T-n [--cmd "…"]` | Estado `verde` o `refactor` | Igual que `verde` → estado `refactor` | Igual |
| `estado [T-n]` | — | Lista las tareas o la ficha de una | — |

Cada transición agrega un evento `tdd.<estado>` en `tdd.jsonl` y regenera **`.harness/cache/hooks.json`**, que es el contrato con el plugin:
```json
{ "generado": "<RFC3339>",
  "tareas_activas": [{ "id": "T-1", "estado": "rojo", "tests_bloqueados": ["tests/x_test.go"] }],
  "tests_bloqueados": ["tests/x_test.go"] }
```

---

# Incremento 3.3 · Orquestación (docs/diseno/11-orquestacion.md)

### Tablero (DAG en SQLite, D-0028)
- `hz tdd contrato T-n … [--celula c] [--bloqueada-por T-1,T-2]`: la tarea nace con su célula y sus dependencias.
- `hz tablero [--listo] [--celula c]`: lista las tareas con estado, célula, dueño y bloqueos. Con `--listo` muestra solo el **frente listo**: tareas en CONTRATO cuyas dependencias ya están en VERDE o en un estado posterior.

### Arriendos de archivos (D-0017)
- `hz arriendo tomar <ruta>… --agente <id> --tarea T-n [--ttl 30m]`: toma rutas en exclusiva.
  - Si alguna ruta tiene un arriendo vigente de **otro** agente: código 1, nombrando a su titular para que se le pida por SendMessage.
  - Renovar las rutas propias está permitido.
- `hz arriendo soltar --agente <id> [<ruta>…]`: suelta todas las rutas del agente, o solo las indicadas.
- `hz arriendo lista`: muestra los arriendos vigentes; los vencidos no aparecen.
- Se registran como eventos en `arriendos.jsonl`. Sobreviven al reindex, pero vencen por tiempo. Se exportan en `hooks.json → arriendos`.

### Tarjetas de traspaso (D-0015)
- `hz tarjeta registrar T-n --rol R --estado listo|fallo|bloqueado --texto "<yaml>"`: el texto tiene **≤30 líneas**; si es más largo, código 2. Se guarda en `traspasos.jsonl` y devuelve `TR-NNNN`.
- `hz tarjeta ultima T-n [--rol R]`: muestra la última tarjeta. Es lo único que recibe el sucesor, nunca el transcript.

### Equipo
- `hz equipo`: lista el directorio rol ↔ `agentId` ↔ supervisor que mantiene el plugin en `.harness/cache/equipo.json`.

### Paquete de rol (D-0014)
- `hz contexto --rol R --tarea T-n`: devuelve ≤40 líneas con la **porción del harness** de ese rol:
  - qué le toca y qué NO le toca;
  - la ficha de la tarea (contrato, tests bloqueados, arriendos);
  - sus comandos `hz`;
  - su criterio de salida;
  - la tarjeta del predecesor, si existe.
- Roles: `contratista`, `tester`, `implementador`, `verificador`, `evaluador`, `registrador`, `integrador`, `curador`, `diagnosticador`, `medidor`, `lider`, `director`, `oraculo`. Un rol desconocido devuelve código 2.
- El paquete del implementador **no** menciona la escalera de verificación, el holdout ni la rúbrica del evaluador.

### Criterio de salida (D-0015)
- `hz salida --rol R --tarea T-n`: verificación determinista; código 0 si el rol puede entregar y 1 con el motivo si no.

| Rol | Criterio |
|---|---|
| contratista | El contrato existe y la tarea está registrada |
| tester | Tarea en ROJO o posterior, con tests bloqueados |
| implementador | Tarea en VERDE o REFACTOR **y** tests bloqueados intactos (hash) |
| verificador | Tarea en VERDE o posterior, con evidencia ejecutada, y `check` en verde |
| evaluador / otros | Existe una tarjeta del rol para la tarea |

### Capacidad (D-0023, D-0030)
- `hz capacidad [--agentes-por-celula 3] [--pico-mb 300]`: mide la RAM libre y total y los núcleos, y calcula las células simultáneas:

  ```
  células = min(límite_concurrencia/agentes_por_célula,
                ⌊(RAM_libre − reserva) / (agentes_por_célula × 45 MB + pico)⌋,
                núcleos)
  ```

  donde `reserva = max(1 GB, 10 % de la RAM total)` y `límite_concurrencia = CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS` (20 si no está definido). Si el resultado es 0, recomienda liberar RAM. `--json` da la salida en JSON.

---

# Incremento 3.4 · Evals y aprendizaje (docs/diseno/06-tdd-y-evals.md §3–4)

### Errores (solo en puntos de entrega; el ROJO del TDD no es error)
`hz error nuevo --que "…" --fuente F --severidad S --causa C --clase <slug> [--ambito "<glob>"] [--patron "…"] [--tarea T-n] [--agente id] [--rol r] [--detectado-por v | --escapo] [--leccion L-n]`
- `fuente`: verificacion | evaluador | humano | regla_dura | revert | regresion.
- `severidad`: critico | mayor | menor.
- `causa`: contrato_ambiguo | contexto_faltante | regla_ignorada | patron_incorrecto | herramienta_mal_usada | test_debil | sobreajuste | otra.
- **Deduplicación por `--clase`:** si la clase existe, suma recurrencia; si es nueva, nace con su causa, ámbito y patrón. Devuelve `E-NNNN` y la recurrencia de la clase.
- **Con `--leccion L-n` y severidad `critico`:** la lección se **elimina de raíz** (se borra la carpeta de su skill y queda una decisión registrada).
- `hz error lista [--clase c]`.

### Lecciones (la skill va antes que el registro)
- `hz leccion nueva --clase c --skill hz-leccion-<x> [--descripcion "…"]`: exige que `.claude/skills/<skill>/SKILL.md` exista; nace `en_prueba` y devuelve `L-NNNN`.
- `hz leccion estado L-n <nuevo> --motivo "…"`, con transiciones válidas:
  - `en_prueba → activa | reescrita`
  - `reescrita → en_prueba`
  - `activa → reescrita | podada | eliminada`

  Cada `reescrita` suma 1 a las reescrituras. La tercera reescritura se rechaza con «escala al humano».
- `hz leccion eliminar L-n --motivo "…"`: borra de raíz la carpeta de la skill y registra la decisión.
- `hz leccion lista [--estado e]` y `hz leccion para <ruta>`: devuelve las lecciones **activas** cuyo ámbito coincide con la ruta (modo índice, D-0021/D-0029).
- Se exportan a `hooks.json → lecciones: [{ id, skill, ambito, descripcion }]` (solo las activas), para la inyección justo a tiempo.

### Evals de lecciones
- `hz eval registrar L-n --tipo prevencion|interferencia|activacion --resultado pasa|falla [--costo usd] [--detalle "…"]`.
- `hz eval decidir L-n [--aplicar]`, determinista y en este orden:
  1. Un error crítico que implica a la lección → `eliminar`.
  2. Faltan tipos por evaluar → `faltan evals: …`.
  3. Los tres tipos pasan (último resultado de cada uno) → `activar`.
  4. Alguno falla → `reescribir`, o `escalar al humano` si ya tiene 2 reescrituras.

### Meta-eval de verificadores (mutation testing, evals para lo determinista)
`hz mutar --archivo <src> [--cmd "…"] [--max 20] [--timeout 120s]`
- Aplica mutaciones textuales, **una a la vez**, fuera de strings y comentarios: ` + `↔` - `, ` == `↔` != `, ` < `↔` >= `, ` > `↔` <= `, ` && `↔` || ` y `true`↔`false`.
- Para cada mutante corre el comando (`comandos.test` por defecto): un mutante es **detectado** si el comando falla.
- **Siempre restaura el original** (respaldo en `.harness/cache/mutar.bak` y verificación por hash).
- Informa la tasa de detección y los mutantes sobrevivientes (`línea:col operador`), que son candidatos a `test_debil`.
- Registra la medición en `verificadores.jsonl`.

### Red-team de reglas duras (efectividad medida, D-0011/D-0026)
- `hz redteam plan [RD-n]`: genera los casos de la regla a partir de sus patrones, con variantes de evasión (`rt-<n>-<k>`). Se corren en un **worktree aislado**.
- `hz redteam registrar RD-n --caso rt-… --resultado bloqueado|paso [--capa c] [--detalle "…"]`. Un `paso` marca la RD como **inefectiva**, con cuándo y por qué, y registra automáticamente un error `critico` de fuente `regla_dura`.
- `hz redteam estado`: por cada RD muestra casos, bloqueados, pasados y estado (`efectiva | inefectiva | sin red-team`), con el caso que la evadió.

---

# Incremento 3.5 · Misiones N3 (docs/diseno/08-misiones.md)

### Misión
- `hz mision nueva <id> --modo genesis|mejora --objetivo "…" [--usd-max n]`: crea `.harness/misiones/<id>/mision.yaml` (esqueleto válido contra `schema/mision.schema.json`) y `estado.json`. Registra el evento `mision.nueva`.
- `hz mision fase <id> <fase> [--proximo "…"]`: actualiza `estado.json` (fase y próximo paso) en cada transición. Es lo que lee `hz estado` para retomar.
- `hz mision gasto <id> --usd x [--concepto "…"]`: acumula gasto. Si llega a `presupuesto.usd_max`, la misión queda **sin presupuesto** y el plugin deniega lanzar subagentes, con remediación.
- `hz mision aceptar <id> <criterio> --evidencia "comando → resultado"`: marca un criterio del contrato default-FAIL, siempre con evidencia.
- `hz mision cerrar <id>`: código 1 si algún criterio de `aceptacion` no está cumplido con evidencia. Si todos lo están, la misión queda cerrada.

### Verticales y sensores (modo mejora)
- `mision.yaml → verticales[].metricas[]`: `{ id, familia: efectividad|impacto|fallos, sensor, direccion: sube|baja, meta, linea_base?, piso_ruido? }`. Cada vertical lleva al menos una métrica de cada familia (D-0022).
- **Formato del sensor:** comando determinista que imprime JSON por ítem en stdout: `[{"item_id": "…", "resultado": true|false}]` o `[{"item_id": "…", "puntaje": 0.0–1.0}]`. `{ronda}` se reemplaza por el ID de la ronda. El valor de la métrica es el promedio.

### Rondas
- `hz ronda nueva <mision> [--foco V-a,V-b]` → `R-n`.
- `hz ronda medir <mision> R-n [--metrica m] [--conjunto trabajo|holdout|ruido]`: corre los sensores, guarda los resultados por ítem en `.harness/misiones/<id>/rondas/R-n/<metrica>.<conjunto>.json` y registra la medición. Los números siempre salen de aquí; el modelo nunca los cuenta.
- `hz ronda cerrar <mision> R-n --veredicto mantener|revertir`: cierra la ronda. Con `mantener`, **pregunta siempre** si se marca como milestone y, si el salto es evidente y grande, lo **propone** con su justificación (D-0019). Una ronda revertida no es candidata.

### Comparación y ruido
- `hz compare <mision> R-a R-b [--metrica m]`: comparación pareada por `item_id`, con ganados, perdidos, empates, valor A → B, delta y p de un **test de signos exacto** sobre los pares discordantes (McNemar exacto si es binario).
  - `veredicto`: `ruido` si |delta| ≤ piso de ruido o p > 0,05; si no, `mejora` o `regresion` según la dirección de la métrica.
- `hz ruido <mision> R-a R-b`: dos mediciones del **mismo código** → `piso_ruido = |delta|` por métrica (registrado).

### Milestones (los define el usuario)
- `hz milestone proponer <mision> [R-n]`: propone solo si alguna métrica mejora más de **k × piso_ruido** (k = 2 por defecto) frente al último milestone (o a la línea base, o a la primera ronda), sin regresiones fuera de ruido. Solo cuentan las métricas con piso conocido (`hz ruido` o `piso_ruido` en mision.yaml): sin piso no hay forma de saber si el salto es grande. Nunca marca nada.
- `hz milestone marcar <mision> R-n --resumen "…" [--propuesta]`: lo ejecuta el humano. Registra el milestone (`MS-n`) con su resumen de una línea.

### Informe C-level (D-0009)
- `hz informe <mision> --json`: datos deterministas para el informe:
  - KPIs por métrica: actual vs. línea base vs. meta;
  - serie por ronda;
  - milestones;
  - comparación entre milestones;
  - gasto.
- `hz informe <mision> --html <archivo>`: página autocontenida con el design system de `dash_example.jpg`. Los números salen del motor. Se publica como Artifact con la skill `hz:informe`.
