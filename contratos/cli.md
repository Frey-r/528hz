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
