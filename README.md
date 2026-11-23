# 528hz

Harness para flujos agénticos largos con [Claude Code](https://claude.com/claude-code).

Reúne convenciones de proyecto, un CLI (`hz`), un plugin con hooks, skills y agentes de rol. Su objetivo es que, en sesiones largas, **lo natural para el agente sea lo correcto**:

- seguir la estructura del proyecto;
- empezar por contratos y tests;
- respetar reglas duras que no puede saltar;
- registrar sus decisiones;
- aprender de sus errores.

Es personal y local: corre en tu máquina, se dimensiona según tu hardware y se activa proyecto por proyecto. Sin activación, todos los hooks son no-op.

## Dos modos

| Modo | Cuándo | Qué obtienes |
|---|---|---|
| **Génesis** | Proyecto desde 0 | Estructura orientada a agentes, contratos, reglas duras y TDD desde el día 1 |
| **Mejora continua** | Proyecto existente | Verticales medidas, rondas de mejora con comparación estadística y milestones |

## Niveles

| Nivel | Incluye | Enforcement |
|---|---|---|
| **N1 Mapa** | `AGENTS.md`, `hz check`, motor de decisiones | Ninguno: solo guía |
| **N2 Normas** | N1 + reglas generales y duras, ciclo TDD, ciclo error → lección | Permisos nativos y hooks fail-closed |
| **N3 Misiones** | N2 + misiones, rondas, milestones, presupuesto, enjambres e informes C-level | Además, gates y circuit breakers |

## Qué trae

- **CLI `hz`** (Go, binario único, SQLite en Go puro sin CGO). Las decisiones y la memoria operativa viven en logs JSONL append-only versionados, más un índice SQLite que se reconstruye desde esos logs.
- **TDD obligatorio.** El ciclo CONTRATO → ROJO → VERDE → REFACTOR bloquea los tests por sha256 y detecta cuando un test falla por la razón equivocada. Además, `hz mutar` hace mutation testing de los verificadores.
- **Reglas duras en capas:** deny nativos, guardias en los hooks, DLP de secretos en la salida y autoprotección. Toda denegación trae su remediación.
- **Aprendizaje.** Los errores generan lecciones (skills), que pasan por un eval A/B y tienen ciclo de vida. Se inyectan justo a tiempo.
- **Orquestación.** Hay 12 agentes de rol `hz:<rol>` y un tablero con dependencias. Los archivos se arriendan por tarea, el relevo entre roles usa tarjetas de traspaso y `hz capacidad` dimensiona el enjambre según la RAM y los núcleos.
- **Misiones N3.** Una misión mide sus verticales con sensores deterministas y compara rondas con un test de signos exacto. Propone milestones según el piso de ruido, pero solo el humano los marca. Tiene un freno de presupuesto.
- **Informes C-level**: páginas autocontenidas con SVG calculado en Go, sin JS.
- **Observabilidad:**
  - status line y pane `/hz-panel` con el árbol de agentes;
  - `hz panel` en la terminal;
  - `hz dashboard`: un dashboard web local (solo `127.0.0.1`) que muestra **todos tus proyectos a la vez**.

## Requisitos

- Go 1.26 o superior.
- Claude Code.

## Instalación

```sh
git clone https://github.com/Frey-r/528hz.git
cd 528hz

# Binario que usa el plugin (lo busca en <repo>/bin/)
go build -trimpath -ldflags "-s -w" -o bin/hz.exe ./cmd/hz

# Opcional: hz en el PATH
go install ./cmd/hz
```

El plugin se carga apuntando Claude Code a la carpeta `plugin/` del repo:

```sh
# Para todas las sesiones
export CLAUDE_CODE_PLUGIN_DIRS="/ruta/a/528hz/plugin"

# O solo para una sesión
claude --plugin-dir /ruta/a/528hz/plugin
```

## Uso rápido

En el proyecto donde quieras usarlo:

```sh
hz activar --modo mejora --nivel N1   # o --modo genesis; sube a N2/N3 cuando quieras
hz check                              # ¿la estructura es legible para agentes?
hz estado                             # resumen para retomar (≤40 líneas)
hz decision nueva …                   # registra cuándo, qué y por qué
hz tdd contrato …                     # arranca el ciclo TDD
```

Desde Claude Code, la skill `hz:harness` guía el uso autónomo del CLI. Para los enjambres y las misiones están `hz:enjambre`, `hz:mision` e `hz:informe`.

### Dashboard de proyectos

```sh
hz dashboard                 # levanta el dashboard central (si no corre) y conecta el proyecto actual
hz dashboard --desconectar   # saca el proyecto actual del dashboard
hz dashboard --detener       # apaga el dashboard central
```

Abre http://127.0.0.1:5280. Hay un solo dashboard por usuario; corre en segundo plano y sobrevive a cerrar la terminal. Por cada proyecto muestra:
- su nivel y modo, las misiones con su presupuesto, las tareas TDD y los arriendos;
- sus sesiones de Claude Code con el árbol de agentes y los carriles de actividad.

Los proyectos se conectan solos al abrir una sesión de Claude Code en un proyecto con harness (o con `hz activar`). Arriba se filtra por proyecto y se elige la ventana de tiempo. Es de solo lectura.

`hz --help` lista todos los subcomandos. El contrato completo está en [`contratos/cli.md`](contratos/cli.md).

## Estructura del repo

| Ruta | Contenido |
|---|---|
| `cmd/hz/` | Entrada y subcomandos del CLI |
| `internal/` | Paquetes: motor, tdd, check, proyecto, reglas, orquesta, aprendizaje, mutar, redteam, estadistica, mision, informe, dashboard… |
| `plugin/` | Plugin `hz`: hooks, skills y agentes de rol |
| `contratos/` | Contratos del CLI y del plugin (van antes que los tests y el código) |
| `docs/diseno/` | Diseño por capacidad (visión, activación, TDD, motor, misiones, orquestación…) |
| `docs/decisiones/` | Índice de decisiones, generado desde `.harness/log/decisiones.jsonl` |
| `reglas/` | Reglas generales y catálogo de reglas duras |
| `schema/` | DDL de SQLite y esquemas de configuración |

[`AGENTS.md`](AGENTS.md) es el mapa del proyecto para los agentes y la mejor puerta de entrada para seguir leyendo.

## Desarrollo

Este repo se construye con su propio harness: test-driven siempre (CONTRATO → ROJO → VERDE → REFACTOR) y cada decisión se registra en `.harness/log/decisiones.jsonl`.

```sh
go test ./...
go vet ./...
claude plugin test plugin
claude plugin validate plugin
```

Docs, decisiones y commits van en español.
