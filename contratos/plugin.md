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
  "protegidos": [".harness/reglas.yaml", ".harness/log/**", ".claude/settings.json", ".claude/settings.local.json", ".githooks/**"],
  "reglas_duras": [{ "id": "RD-1", "orden": "…", "remediacion": "…",
                     "herramientas": ["mcp__x__escribir"], "rutas": [".env"], "bash": ["regex"] }],
  "secretos": [".env"]
}
```
Las rutas son relativas a la raíz y usan `/`. Las globs admiten `*` (un segmento) y `**` (varios).

## 2. CLI (`contratos/cli.md` → se agrega)
- **`hz exportar`**: regenera `hooks.json` v2. Lo llaman también `tdd …`, `activar` y `regla …`.
- **`hz activar --nivel N2|N3`** sobre un proyecto existente:
  - actualiza el `nivel:` de `harness.yaml`;
  - crea `.harness/reglas.yaml` si no existe (`version: 1`, `reglas: []`, más una guía comentada hacia el catálogo);
  - **fusiona** en `.claude/settings.json → permissions.deny` las reglas de autoprotección y las de las RD con capa `permisos`. Nunca borra reglas ni claves existentes y no duplica;
  - exporta.
- **Autoprotección** (siempre en N2+, D-0026): `Edit` y `Write` sobre `.harness/reglas.yaml`, `.harness/log/**`, `.claude/settings.json`, `.claude/settings.local.json` y `.githooks/**`. **No** se protege todo `.claude/**`, porque las lecciones viven en `.claude/skills/`.
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
