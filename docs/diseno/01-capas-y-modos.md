# 01 · Capas y modos

## Capas

| Capa | Qué es | Dónde vive | Quién la cambia |
|---|---|---|---|
| 1. Núcleo | Plugin: skills, agentes de rol, hooks, CLI `hz` | Repo `528hz/` → instalado a nivel usuario | Mantenedor del harness |
| 2. Reglas | **RG:** generales, en todo proyecto. **RD:** duras, por proyecto | RG en el plugin (`reglas/generales.md`); RD en `<repo>/.harness/reglas.yaml` | RG: mantenedor. RD: solo el humano del proyecto |
| 3. Perfil del proyecto | AGENTS.md, estructura, contratos, política de verificación | Repo del proyecto | Agentes con decisión registrada; humano aprueba |
| 4. Memoria operativa | SQLite: decisiones, reglas, errores, lecciones, evals, verticales, rondas, milestones, traspasos, arriendos | `<repo>/.harness/` (`log/*.jsonl` versionado + `harness.db`) | Solo vía `hz` |
| 5. Misiones | Trabajo largo: Génesis o Mejora | `<repo>/.harness/misiones/<id>/` | Director + humano |
| 5b. Orquestación | Roles, relevo supervisado, conversación entre agentes, enjambres | Plugin (agentes + hooks) + tablero de tareas + SQLite | — |
| 6. Observabilidad | Actividad en el plugin (`$.state`) → status line, pane `hz-panel` y snapshot `.harness/cache/actividad.json` → `hz panel [--json]` → `hz dashboard` (web local, árbol + carriles, D-0050) | Plugin + `<repo>/.harness/cache/` | `contratos/plugin.md` §7 |

## Componentes del plugin

| Componente | Contenido |
|---|---|
| **Skills** | `hz` (activar, descubrir, estado, aprobar, regla, aprender, informe, enjambre) y las lecciones (`hz-leccion-*`, por proyecto o globales). |
| **Agentes de rol** | `hz-director`, `hz-lider`, `hz-contratista`, `hz-tester`, `hz-implementador`, `hz-verificador`, `hz-evaluador`, `hz-registrador`, `hz-integrador`, `hz-curador`, `hz-diagnosticador`, `hz-medidor`, `hz-oraculo`. Cada uno tiene su allowlist de herramientas; el modelo lo decide el orquestador (D-0024, ver `11-orquestacion.md`). |
| **Hooks (mod)** | `tool.call` aplica RD, el orden TDD, los arriendos y la inyección justo a tiempo de AGENTS.md del dominio y de lecciones. `agent.spawn` inyecta el paquete de rol y aplica límites. `agent.report` guarda las tarjetas. `session.send` audita mensajes. `session.start` agrega un puntero breve al estado. |
| **CLI `hz`** (Go, binario único; D-0033) | Sensores deterministas y acceso al motor: `decision`, `regla`, `error`, `leccion`, `tdd`, `salida`, `contexto`, `equipo`, `arriendo`, `estado`, `compare`, `check`, `reindex`, `informe`. |

## Modo Génesis (proyecto desde 0)
```
Intención ─► Arquitectura + Contratos ─► Andamiaje ─► Hitos (TDD) ─► Endurecimiento ─► Entrega
   │humano        │humano aprueba            │CI verde      │células       │red-team RD
```
- **Intención:** spec, usuarios, non-goals, criterios de aceptación y **reglas duras RD**.
- **Arquitectura:** dominios, stack y fronteras. Cada elección queda como decisión. Los **contratos globales** van en `contratos/`.
- **Andamiaje:** estructura, AGENTS.md raíz y por dominio, enforcement de RD, comando único y CI local en verde.
- **Hitos:** slices verticales. Cada tarea sigue CONTRATO → ROJO → VERDE → REFACTOR y en paralelo trabajan células por funcionalidad.
- **Endurecimiento:** red-team de las RD, E2E y meta-evals de los verificadores.

## Modo Mejora continua (proyecto existente)
```
Adopción ─► Verticales + Línea base ─► Ronda ─► Ronda ─► … ─► Milestone ─► … ─► Cierre
 (N1→N2)        │humano aprueba           └ promoción automática por salto ┘
```
- **Adopción:** `hz descubrir` propone el perfil siguiendo la estructura existente, sin mover código.
- **Verticales esenciales:** dimensiones del producto que concentran el valor. Cada una tiene métricas, sensor, línea base, meta, peso y piso de ruido. Aquí se definen o revisan las RD.
- **Rondas:** el proceso interno de iteración.
- **Milestones:** rondas destacadas por su salto. La comparación principal es entre milestones.

Detalle en `08-misiones.md`.

## Lo que el harness no toca
`/goal`, Agent, Workflow, Monitor, worktrees, la memoria por proyecto, los MCP del usuario, `permissions.defaultMode` y el `statusLine` existente. El harness los envuelve con convenciones; no los reemplaza.
