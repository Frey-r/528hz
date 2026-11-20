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
