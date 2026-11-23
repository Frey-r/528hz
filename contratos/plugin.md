# Contrato · Plugin `hz` + exportación v2 (incremento 3.2)

## Principio
`hz` (Go) calcula y el plugin (hooks TS) aplica. Los hooks **nunca lanzan `hz`** en un tool call, porque cada proceso cuesta 250–370 ms en esta máquina (D-0034). Leen `.harness/cache/hooks.json`, que `hz` regenera después de cada mutación y con `hz exportar`.

## 1. `hooks.json` v2 (lo escribe `hz`, lo lee el plugin)
```json
{
  "version": 2,
  "generado": "RFC3339",
  "nivel": "N1|N2|N3",
  "modo": "genesis|mejora",
  "resumen": ["≤15 líneas para el puntero de inicio de sesión"],
  "tareas_activas": [{ "id": "T-1", "estado": "rojo", "tests_bloqueados": ["tests/x_test.go"] }],
  "tests_bloqueados": ["tests/x_test.go"],
  "tareas_superadas": [{ "id": "T-7", "por": "T-11", "decision": "D-0042" }],
  "protegidos": [".harness/reglas.yaml", ".harness/log/**", ".claude/settings.json", ".claude/settings.local.json", ".githooks/**"],
  "reglas_duras": [{ "id": "RD-1", "orden": "…", "remediacion": "…",
                     "herramientas": ["mcp__x__escribir"], "rutas": [".env"], "bash": ["regex"] }],
  "secretos": [".env"]
}
```
Las rutas son relativas a la raíz y usan `/`. Las globs admiten `*` (un segmento) y `**` (varios).
`tareas_superadas` es informativo y aditivo (`hz tdd superar`): esas tareas no están en `tareas_activas` y sus tests no están en `tests_bloqueados`, salvo que otra tarea activa los bloquee.

## 2. CLI (`contratos/cli.md` → se agrega)
- **`hz exportar`**: regenera `hooks.json` v2. Lo llaman también `tdd …`, `activar` y `regla …`.
- **`hz activar --nivel N2|N3`** sobre un proyecto existente:
  - actualiza el `nivel:` de `harness.yaml`;
  - crea `.harness/reglas.yaml` si no existe (`version: 1`, `reglas: []`, más una guía comentada hacia el catálogo);
  - **fusiona** en `.claude/settings.json → permissions.deny` las reglas de autoprotección y las de las RD con capa `permisos`. Nunca borra reglas ni claves existentes y no duplica;
  - genera solo reglas `Read(ruta)` y `Edit(ruta)`: Claude Code no aplica `Write(ruta)` en sus verificaciones de archivos (avisa en cada inicio) y `Edit(ruta)` ya cubre todas las herramientas que escriben (D-0047). La única regla que retira son las `Write(ruta)` que generaron versiones anteriores del harness cuando la misma ruta ya tiene su `Edit(ruta)`;
  - exporta.
- **Autoprotección** (siempre en N2+, D-0026): toda escritura (deny nativo `Edit(ruta)` y el hook para `Edit`, `Write`, `MultiEdit`, `NotebookEdit` y Bash) sobre `.harness/reglas.yaml`, `.harness/log/**`, `.claude/settings.json`, `.claude/settings.local.json` y `.githooks/**`. **No** se protege todo `.claude/**`, porque las lecciones viven en `.claude/skills/`.
- **RD vigentes:** salen de `.harness/reglas.yaml`, validado contra `schema/reglas.schema.json` (id `RD-n`, al menos 2 capas de enforcement). `secretos` son los archivos reales de la raíz que coinciden con `patrones.rutas` de las RD con `clave_catalogo: secretos`.

## 3. Hooks del plugin

| Evento | Nivel | Comportamiento |
|---|---|---|
| `SessionStart` (**hook de comando** en `hooks.json`: `hz puntero`) | N1+ | Entrega el `resumen` (≤15 líneas) como `additionalContext` al iniciar, al retomar, al hacer `/clear` y **tras una compactación**. Un solo proceso por evento, no por tool call. Los mods no sirven aquí: el plugin interno `cc-plugin-sec-default` salta `prompt.context` y `classic.SessionStart` para ellos (D-0036) |
| `tool.call` (entrada) | N2+ | Deniega, con remediación: **(a)** `Edit/Write/NotebookEdit` sobre `tests_bloqueados` («RG-2.3: test bloqueado en T-n…»); **(b)** `Edit/Write` sobre `protegidos`; **(c)** herramientas de una RD; **(d)** rutas de una RD vía `Read/Edit/Write/Grep/Glob`; **(e)** `Bash` que coincide con el regex de una RD; **(f)** `Bash` que **escribe** (`>`, `>>`, `tee`, `cp`, `mv`, `rm`, `sed -i`) sobre un protegido o un test bloqueado |
| `tool.call` (salida) | N2+ con `secretos` | **DLP:** si el resultado contiene un valor de secreto (crudo, base64 o hex), se retiene con remediación |
| `tool.call` (catch) | N2+ | **Fail-closed:** si el guardia falla, deniega |
| `session.start` | N1+ | Status line `hz · <nivel> · <tareas activas>` |

**Rutas:**
- Toda ruta absoluta se relativiza a la raíz del proyecto.
- Si cae dentro de `.claude/worktrees/<agente>/`, se relativiza a ese worktree (S10).
- La comparación es insensible a mayúsculas en Windows y normaliza `\` → `/`.

**Caché:** el plugin relee `hooks.json` solo si cambió su `mtimeMs` (un `stat` por tool call, en proceso). Sin `hooks.json`, o con `enabled: false`, todo es no-op.

---

## 4. Orquestación (incremento 3.3)
- **`hooks.json` v2 suma** `arriendos: [{ "ruta", "agente", "tarea", "expira" }]` (solo los vigentes).
- **`tool.call` (arriendos, N2+):** si un agente intenta `Edit/Write/MultiEdit/NotebookEdit` sobre una ruta arrendada, se deniega con «pídeselo a `<agente>` (tarea T-n) por SendMessage», **salvo** que sea su titular o que su descripción declare la misma tarea («<rol> T-n», D-0037). La sesión principal (sin `agentId`) también respeta los arriendos. Al vencer, el arriendo deja de aplicar.
- **Bash:** los segmentos que invocan `hz` no se evalúan como escritura, y los operadores entre comillas son datos (D-0038).
- **Equipo:** en el primer `tool.call` de cada `agentId`, el mod lo registra en `.harness/cache/equipo.json` (`{ id, tipo, descripcion, padre, visto }`) con `$.agent.list()`. Con eso `hz equipo` muestra quién es quién.
- **Supervisor (`agent.spawn`):** agrega al prompt del subagente la línea «Tu supervisor es `<parentAgentId>` (o la sesión principal): escálale por SendMessage con ese id». Así cada agente sabe a quién escalar sin preguntar (D-0027).
- **Watchdog:** si un agente acumula **5 tool calls seguidas con error**, el mod envía un único `ALERTA` a su supervisor (`$.session.send({ to: { agentId } })`). Si el supervisor es la sesión principal, muestra un toast.

## 5. Lecciones justo a tiempo (incremento 3.4)
- `hooks.json` v2 suma `lecciones: [{ id, skill, ambito, descripcion }]`, solo las **activas**. Una lección en prueba, reescrita o eliminada nunca se inyecta.
- **`tool.call`** (N1+): la primera vez que un agente toca (`Read/Edit/Write/…`) un archivo dentro del ámbito de una lección activa, se adjunta como `context` el aviso «Lección L-n aplica… carga la skill …». Va **una sola vez por agente y lección**, para no llenar el contexto.

## 6. Presupuesto de misión (incremento 3.5)
- `hooks.json` v2 suma `misiones: [{ id, gasto, usd_max, sin_presupuesto }]`, solo las **activas** (las cerradas no frenan nada). `usd_max` sale de `mision.yaml → presupuesto.usd_max`; `gasto` es la suma de `hz mision gasto`. `sin_presupuesto` es `true` cuando `usd_max > 0` y `gasto ≥ usd_max`.
- **`agent.spawn`** (en cualquier nivel: solo hay misiones si el humano las creó): si alguna misión activa está `sin_presupuesto`, se deniega el lanzamiento con la remediación «la misión <id> gastó US$x de US$y: pide al humano ampliar `presupuesto.usd_max` en mision.yaml (y `hz exportar`) o cierra la misión». Es un circuit breaker: el humano decide si se gasta más.

## 7. Observabilidad (incremento 3.6)
Solo con harness en el proyecto. Un fallo de observabilidad **nunca** deniega una herramienta: se atrapa y se ignora.
- **Actividad** (en `$.state` del mod, sobrevive a una recarga): una fila por agente, la sesión principal (`principal`) y cada subagente:
  `{ id, rol, descripcion, padre, tarea, estado, tools, errores, seguidos, ultima, desde, visto }`.
  - `agent.spawn` (tras `next`, con el `agentId` que devuelve): fila `lanzado`. El rol sale del tipo `hz:<rol>` o, si no, de la descripción «<rol> T-n»; la tarea, de la descripción.
  - `tool.call`: `activo`; suma `tools` y, si el resultado es error o denegación, `errores` y `seguidos` (un éxito los vuelve a 0). `ultima` = herramienta + destino corto (ruta relativa o comando recortado a 40 caracteres).
  - `agent.report`: `terminado` (completed), `fallido` (failed) o `detenido` (stopped).
  - Se conservan hasta 50 filas terminadas (las más recientes); las vivas, todas.
- **Snapshot** `.harness/cache/actividad.json` `{ version: 1, generado, agentes: [...] }`: se escribe al lanzar y al terminar un agente y, entre medio, como máximo una vez cada 2 s (RG-1). Lo leen `hz panel` y el dashboard.
- **Status line** (`$.ui.status`, solo cuando cambia): `hz · N2 · T-1 ROJO · 3 agentes · 1 con errores` (≤80 caracteres; las partes vacías no aparecen).
- **Pane `hz-panel`**: el comando `/hz-panel` lo abre; se abre solo al lanzarse el primer subagente, donde la terminal tiene lugar.
  - cabecera: proyecto · nivel · modo, y cada misión activa con su barra de presupuesto;
  - árbol de agentes por supervisor (principal → líderes → roles): glifo de estado, rol, tarea, tools, errores, última herramienta y hace cuánto; `seguidos ≥ 3` se marca;
  - tareas TDD activas con su estado y cantidad de arriendos vigentes.
- **Eventos para los carriles del dashboard (3.7, D-0050):** además de la fila, el mod guarda por agente sus últimos **100** eventos `[t_ms, tipo]` (`tipo` 0 = tool call, 1 = error o denegación, 2 = lanzado, 3 = terminó). Van en `$.state` (`hz.eventos`) y en el snapshot como `eventos: { "<id>": [[t, tipo], …] }`. Al podar una fila, se van también sus eventos.
