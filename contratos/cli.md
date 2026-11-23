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
- tareas TDD abiertas con su estado (las `verificada`, `cerrada` y `superada` no cuentan como abiertas);
- misión activa, si `misiones/*/estado.json` existe.

### `hz tdd …`: máquina de estados por tarea

| Subcomando | Precondición | Efecto | Fallo |
|---|---|---|---|
| `contrato T-n --titulo "…" --contrato <ruta> [--aprobado] [--mision m]` | La tarea no existe | Crea la tarea en estado `contrato`. `--aprobado` registra la aprobación | La ruta del contrato no existe → código 1 |
| `rojo T-n --tests a,b [--cmd "…"]` | Estado `contrato` | Corre `--cmd` (o `comandos.test` de `harness.yaml`). **Exige** salida ≠ 0 **y** que no aparezca ningún patrón de error de compilación, import o sintaxis. Bloquea los tests (sha256) → estado `rojo` | Los tests pasan → «no hay ROJO». Error de import o sintaxis → «falla por la razón equivocada» |
| `verde T-n [--cmd "…"]` | Estado `rojo` | Verifica que los tests bloqueados no cambiaron y corre el comando, que debe salir con 0 → estado `verde` | Test bloqueado modificado → código 1 nombrando el archivo. El comando falla → código 1 |
| `refactor T-n [--cmd "…"]` | Estado `verde` o `refactor` | Igual que `verde` → estado `refactor` | Igual |
| `superar T-n --por T-m --decision D-NNNN [--motivo "…"] [--quien id]` | T-n en `verde`, `refactor` o `verificada` (pasó por VERDE). T-m existe, es otra tarea y está en `verde` o en un estado posterior, sin estar superada. **Evidencia (D-0054):** al menos un test bloqueado de T-n cambió (su sha256 actual ya no es el bloqueado, o el archivo no existe) y T-m bloquea **cada uno** de los que cambiaron con su contenido actual (misma ruta, limpia y con `/`; en Windows sin distinguir mayúsculas). D-NNNN existe en el log de decisiones y la decidió `humano` o `agente_aprobado` | T-n → estado `superada`. **No toca ni re-bloquea tests**: solo deja constancia con el evento `tdd.superada` (foto de la tarea + `superacion: {por, decision, decidio, motivo, quien, en, estado_previo}`). `motivo` por defecto es el «qué» de la decisión; `quien` sale de `--quien` o de `HZ_SESION`; `decidio` se copia de la decisión | Código 1 con remediación: T-n no existe; T-n en CONTRATO o ROJO («no sirve para saltarse un ROJO»); T-n ya superada (nombra a T-m y la decisión); T-m no existe, es la misma T-n, no llegó a VERDE o está superada (sugiere `--por` la tarea que la superó); ningún test bloqueado de T-n cambió («no hay nada que superar»: sugiere `hz salida`); un test cambiado no está bloqueado por T-m, T-m lo bloqueó con otro contenido o ya no existe (nombra el test y T-m); la decisión no existe; la decisión es `agente` (nombra la decisión y pide una `humano` o `agente_aprobado`). Las comprobaciones van en ese orden y un rechazo no cambia nada. Faltan `--por` o `--decision` → código 2. `--help` imprime el uso y sale con 0 |
| `estado [T-n]` | — | Lista las tareas o la ficha de una | — |

Cada transición agrega un evento `tdd.<estado>` en `tdd.jsonl` (`tdd.superada` para `superar`) y regenera **`.harness/cache/hooks.json`**, que es el contrato con el plugin:
```json
{ "generado": "<RFC3339>",
  "tareas_activas": [{ "id": "T-1", "estado": "rojo", "tests_bloqueados": ["tests/x_test.go"] }],
  "tests_bloqueados": ["tests/x_test.go"],
  "tareas_superadas": [{ "id": "T-7", "por": "T-11", "decision": "D-0042" }] }
```

**Tarea superada** (caso típico: otra tarea regeneró un golden que T-n había bloqueado, con una decisión registrada). Como `hz salida` deja de verificarla, `superar` exige evidencia de que T-m tomó a su cargo lo que cambió y una decisión del humano o aprobada por él: no sirve para saltarse el verificador de T-n (D-0054). Es un estado terminal:
- deja de ser activa: no aparece en `tareas_activas` y sus tests salen de `tests_bloqueados` (si T-m los bloqueó, siguen protegidos por T-m);
- no bloquea a sus dependientes en el tablero (cuenta como posterior al VERDE);
- `hz salida --rol implementador|verificador --tarea T-n` sale con 0 **sin volver a comprobar hashes ni la meta-eval** e informa `superada por T-m (D-NNNN)`;
- `hz tdd estado`, `hz tablero` y `hz estado` la muestran como `SUPERADA` con la tarea y la decisión (`hz estado` no la cuenta como abierta); `hooks.json` la lista en `tareas_superadas` y el resumen del puntero la menciona.

---

# Incremento 3.3 · Orquestación (docs/diseno/11-orquestacion.md)

### Tablero (DAG en SQLite, D-0028)
- `hz tdd contrato T-n … [--celula c] [--bloqueada-por T-1,T-2]`: la tarea nace con su célula y sus dependencias.
- `hz tablero [--listo] [--celula c]`: lista las tareas con estado, célula, dueño y bloqueos. Con `--listo` muestra solo el **frente listo**: tareas en CONTRATO cuyas dependencias ya están en VERDE o en un estado posterior (`verde`, `refactor`, `verificada`, `cerrada`, `superada`). Una tarea superada se muestra `SUPERADA · superada por T-m (D-NNNN)`.

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
| implementador | Tarea en VERDE o REFACTOR **y** tests bloqueados intactos (hash). Tarea `superada`: sale con 0 e informa por quién y con qué decisión |
| verificador | Tarea en VERDE o posterior, con evidencia ejecutada, `check` en verde **y meta-eval de sus tests** (D-0046): al menos una medición `hz mutar --tarea T-n`; cada archivo medido con detección ≥ `verificacion.mutacion_min` (0,9 por defecto, 0 la desactiva); y cada archivo de código arrendado por la tarea (que no sea test) medido. Tarea `superada`: sale con 0 sin estas comprobaciones y la salida dice `superada por T-m (D-NNNN)` |
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
`hz mutar --archivo <src> [--tarea T-n] [--cmd "…"] [--max 20] [--timeout 120s]`
- Aplica mutaciones textuales, **una a la vez**, fuera de strings y comentarios: ` + `↔` - `, ` == `↔` != `, ` < `↔` >= `, ` > `↔` <= `, ` && `↔` || ` y `true`↔`false`.
- Para cada mutante corre el comando (`comandos.test` por defecto): un mutante es **detectado** si el comando falla.
- **Siempre restaura el original**, con un respaldo **por archivo** y verificación por hash:
  - El respaldo vive en `.harness/cache/mutar/<h>.bak`, donde `<h>` son los primeros 16 hex del sha256 de la ruta relativa normalizada (con `/`, limpia y, en Windows, en minúsculas).
  - A su lado vive el candado `<h>.lock`: se crea en exclusiva (`O_EXCL`) **antes** del respaldo, guarda `{ruta, pid, host, inicio, sha256}` del original y su mtime se renueva cada 10 s (latido) mientras dura la medición.
  - Al terminar, restaura, verifica el hash y borra primero el respaldo y después el candado. Si la verificación falla, conserva ambos y termina con código 4 nombrando el respaldo.
- **Concurrencia:** dos mediciones sobre **archivos distintos** corren a la vez sin pisarse. Una segunda medición sobre el **mismo archivo** mientras la primera vive → código 1, sin tocar el archivo: «ya hay una medición de mutación en curso sobre `<ruta>` (pid N, desde HH:MM:SS)…». Si el candado todavía no se puede leer (la otra medición lo está escribiendo o acaba de borrarlo), el rechazo dice que reintente, sin pid inventado; si es ilegible y su latido venció, nombra el candado para que el humano lo borre.
- **Huérfanos** (corrida interrumpida): al empezar, `hz mutar` revisa **todos** los candados. Un candado es huérfano si su proceso ya no existe en este host o si su latido tiene más de 1 min. Cada huérfano se **reclama** antes de tocarlo (D-0055): se crea en exclusiva `<h>.recuperando` y se **vuelve a leer** el candado; si ya no es el mismo que se vio huérfano (otro proceso lo reparó y una medición nueva tomó el archivo), no se toca nada. Si el reclamo ya existe, otro proceso lo está reparando y se salta; si además tiene más de 1 min, se avisa para que el humano lo resuelva (no se borra solo). El reclamo se borra al terminar, después del candado; medir ese archivo mientras se repara → código 1 («reintenta»). Por cada huérfano reclamado:
  - si el archivo es igual al original, borra respaldo y candado;
  - si el archivo es **un mutante** del original (difiere en un único sitio mutado), restaura el original, lo verifica, borra respaldo y candado y avisa: `aviso: restaurado <ruta> …`;
  - si el archivo cambió de otra forma, **no lo toca**: avisa con la ruta del respaldo y conserva respaldo y candado. Medir ese archivo queda rechazado (código 1) hasta que el humano lo resuelva borrando `<h>.lock` y `<h>.bak`.
  - Un respaldo antiguo `.harness/cache/mutar.bak` (versiones previas) solo se avisa, porque no dice de qué archivo es.
- Los avisos salen por stderr, aunque la medición falle después; con `--json` van además en `avisos`.
- Si hay más sitios que `--max`, toma una **muestra determinista repartida en todo el archivo** (paso uniforme), no solo los primeros.
- Un archivo sin sitios mutables registra detección 1: no hay fallas que la suite deba detectar.
- Informa la tasa de detección y los mutantes sobrevivientes (`línea:col operador`), que son candidatos a `test_debil`.
- Con `--tarea T-n`, la medición queda además asociada a la tarea (`mutacion:T-n:<archivo>`): es lo que exige `hz salida --rol verificador`.
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

---

# Incremento 3.6 · Observabilidad (contratos/plugin.md §7)

### `hz panel [--json]`
- Junta lo que el plugin vio (los snapshots de sesión de `.harness/cache/actividad/`, ver 3.8) con lo que sabe el motor: tareas TDD abiertas, arriendos vigentes y misiones activas con su gasto.
- Sin `--json`: por cada sesión, una línea `sesión <8 primeros del id> · activa|inactiva|terminada` y debajo su árbol de agentes por supervisor, con los vivos primero; todo en ≤40 líneas. Las sesiones activas van primero. Sin snapshots: «sin actividad registrada (el plugin la escribe en .harness/cache/actividad/)».
- `--json`: `{ generado, actividad_generada, sesiones: [{ id, generado, terminada, inactiva, agentes }], agentes: [...], tareas: [...], arriendos, misiones: [{ id, gasto, usd_max, sin_presupuesto }] }`. `agentes` junta los de todas las sesiones y cada uno lleva `sesion`; `actividad_generada` es el snapshot más reciente.

# Incremento 3.7 · Dashboard web local (D-0050)

### `hz dashboard [--puerto 5280]`
- Sirve **solo en `127.0.0.1`** y es de **solo lectura**. Rechaza con 403 toda petición cuyo `Host` no sea `127.0.0.1`, `localhost` o `[::1]` con ese puerto: así otra página no puede leer los datos por DNS rebinding.
- `GET /`: la página con el design system del harness (glass oscuro, acento teal, claro y oscuro). Cada 2 s pide el fragmento y lo reemplaza; no usa librerías externas.
- `GET /fragmento?ventana=5m|10m|30m` (10m por defecto): HTML calculado en Go con:
  - cabecera: proyecto, nivel, modo, misiones con su presupuesto, tareas TDD y arriendos;
  - **árbol + carriles**: una fila por agente en orden de árbol (vivos primero), con su nombre sangrado y, a la derecha, un carril SVG de la ventana (marca por tool call, ✕ por error, inicio y fin del agente, hueco = sin actividad) y su resumen (tools, errores, hace cuánto);
  - sin actividad: un estado vacío que explica de dónde salen los datos.
- `GET /api/panel`: el mismo JSON de `hz panel --json` más `eventos`.
- Lee el motor y `actividad.json` en cada petición; reutiliza una sola conexión al motor (RG-1).

> **Reemplazado en 3.8:** el dashboard pasa a ser central y multi-proyecto; la vista, las rutas y la protección se mantienen y se extienden abajo.

# Incremento 3.8 · Dashboard central multi-proyecto (D-0053)

### Registro de proyectos
- Archivo de usuario `proyectos.json` en el **hogar de hz**: `os.UserConfigDir()/hz` (en Windows `%AppData%\hz`), o la carpeta de `HZ_HOGAR` si está definida. Forma: `{ "version": 1, "proyectos": [{ "raiz": "<ruta absoluta>", "conectado": "<RFC3339>" }] }`.
- Las raíces se guardan absolutas y limpias; en Windows se comparan sin distinguir mayúsculas. Conectar es **idempotente**: si el proyecto ya está, no reescribe el archivo. La escritura es atómica (temporal + rename). Un archivo que falta es un registro vacío.
- Cada proyecto tiene un `id` estable: los 8 primeros hex del sha256 de su raíz normalizada. Su nombre visible es la carpeta; si dos proyectos tienen la misma carpeta, `padre/carpeta`.
- **Quién conecta:**
  - `hz puntero` (corre en cada SessionStart): conecta el proyecto y poda los snapshots de sesión con más de 24 h sin escribir. Cualquier error se ignora: el puntero nunca falla.
  - `hz activar`: conecta el proyecto al activarlo.
  - `hz dashboard` dentro de un proyecto.

### Snapshots por sesión (los escribe el plugin, contratos/plugin.md §7)
- `.harness/cache/actividad/<sessionId>.json`: `{ version: 2, sesion, generado, terminada, agentes: [...], eventos: {...} }`.
- Una sesión es **inactiva** si no está terminada y su `generado` tiene más de 30 min. Los agentes de una sesión terminada o inactiva no cuentan como vivos.
- Un archivo ilegible (por ejemplo, leído a medio escribir) se omite: nunca rompe el panel. El `actividad.json` anterior a 3.8 se ignora.

### `hz dashboard [--puerto 5280] [--primer-plano | --detener | --desconectar]`
Funciona dentro o fuera de un proyecto. Las opciones son excluyentes; un argumento suelto, un puerto fuera de 1–65535 o dos opciones juntas → código 2.
- **Sin opciones:** conecta el proyecto del cwd (si hay uno) y se asegura de que el dashboard central corre en `127.0.0.1:<puerto>`:
  - `GET /api/salud` responde `{ servicio: "hz-dashboard", version, pid, binario }` con el mismo `binario` que este `hz` (tamaño + fecha de modificación del ejecutable) → no lanza nada;
  - responde con otro `binario` (se recompiló `hz`) → termina ese `pid` y lanza el nuevo;
  - no responde → lanza `hz dashboard --primer-plano --puerto <n>` **desacoplado de la terminal** (sobrevive a la sesión), con stdout y stderr en `dashboard.log` del hogar, y espera hasta 5 s a que `/api/salud` responda; si no, código 4 nombrando el log;
  - responde otra cosa (el puerto lo ocupa otro servicio) → código 4: «el puerto <n> lo ocupa otro servicio: usa --puerto».
  - Salida: `hz dashboard en http://127.0.0.1:<puerto>` y, si conectó un proyecto, `proyecto <nombre> conectado`.
- `--primer-plano`: sirve en primer plano hasta Ctrl+C (es lo que corre el proceso desacoplado). Puerto ocupado → código 4 «no se pudo escuchar».
- `--detener`: si `/api/salud` confirma `hz-dashboard`, termina su `pid` → `hz dashboard detenido`. Si no hay dashboard: `no hay dashboard corriendo` y código 0.
- `--desconectar`: saca el proyecto del cwd del registro → `proyecto <nombre> desconectado`. Fuera de un proyecto → código 3.

### Rutas (solo GET/HEAD; mismo chequeo de `Host` y cabeceras que 3.7)
- `GET /api/salud`: la identidad del servidor (arriba).
- `GET /`: la página. Cada 2 s pide el fragmento; recuerda la ventana, el filtro y los proyectos que el humano plegó o desplegó (almacenamiento local, opcional).
- `GET /fragmento?ventana=5m|10m|30m&proyecto=<id>`:
  - **cabecera global:** `N proyectos · S sesiones activas · X agentes vivos · Y alertas`, y chips para filtrar (`todos` y uno por proyecto, con `aria-pressed`). Un `proyecto` desconocido muestra todos;
  - **un bloque por proyecto**, primero los que tienen alertas, luego los que tienen agentes vivos, luego por actividad más reciente y por nombre:
    - cabecera: nombre · nivel · modo, misiones con su barra de presupuesto, tareas TDD y arriendos;
    - si tiene agentes vivos o eventos dentro de la ventana, se despliega el **árbol + carriles** de 3.7 con una raíz por sesión (`sesión <8 primeros> · activa|inactiva|terminada`, con los carriles de la sesión principal) y debajo sus subagentes por supervisor. Solo se muestran las sesiones activas y las que tuvieron eventos en la ventana;
    - si no, queda plegado en una línea con «sin actividad»;
    - si no se puede leer (carpeta borrada, sin harness o desactivado, motor ilegible): una línea «no disponible: <motivo>».
  - **alertas:** agentes con `seguidos ≥ 3` en sesiones activas, agentes `fallido` con eventos en la ventana y misiones sin presupuesto.
  - Sin proyectos conectados: un estado vacío que explica cómo conectar uno.
- `GET /api/panel`: `{ generado, proyectos: [{ id, nombre, raiz, disponible, error, nivel, modo, tareas, arriendos, misiones, sesiones: [{ id, generado, terminada, inactiva, agentes, eventos }] }] }`.
- **RG-1:** relee el registro en cada petición (es un archivo pequeño); abre un motor por proyecto al primer uso y lo reutiliza; lo cierra cuando el proyecto sale del registro o falla (se reabre en la siguiente petición).
