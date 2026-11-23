# 528hz: Harness para flujos agénticos largos

Harness de trabajo genérico para Claude Code. Funciona en dos modos:
- **Génesis:** proyecto desde 0.
- **Mejora continua:** proyecto existente.

Este archivo es un **mapa**, no un manual. Cada sección apunta a donde vive el detalle.

## Estado
- **3.1 listo:** CLI `hz` en Go (motor SQLite + JSONL, decisiones, TDD, check, estado).
- **3.2 listo:** plugin `hz` (guardias TDD/RD/autoprotección, DLP, puntero por hook de comando), skill `hz:harness`, `activar --nivel N2`, `exportar`, `puntero`. Validado en sesión real (`docs/diseno/12-spike.md`).
- **3.3 listo:** orquestación: 12 agentes de rol `hz:<rol>`, skill `hz:enjambre`, tablero con DAG, arriendos por tarea, tarjetas, paquetes de rol, `salida`, `capacidad`, `equipo`, supervisor y vigía. Validado en sesión real.
- **3.4 listo:** evals y aprendizaje: `hz error/leccion/eval` (ciclo de vida con eliminación de raíz), lecciones justo a tiempo, `hz mutar` (meta-eval de verificadores), `hz redteam`, skill `hz:redteam`, índice versionado por esquema. Primera lección real evaluada A/B (no activada: prevención no demostrada).
- **3.5 listo:** misiones N3: `hz mision/ronda/compare/ruido/milestone/informe`, sensores deterministas, test de signos exacto, propuesta de milestone por k × piso de ruido, freno de presupuesto en `agent.spawn`, informe C-level (página SVG sin JS) y skills `hz:mision` / `hz:informe`. Validado con una misión demo de 2 verticales y 4 rondas.
- **3.6 listo:** observabilidad: actividad por agente en el plugin, status line, pane `/hz-panel` (árbol por supervisor), snapshot `actividad.json` y `hz panel [--json]`. Además: umbral de mutation testing como criterio de salida del verificador (D-0046), L-0001 activa (D-0048), deny nativos solo `Read`/`Edit` (D-0047). Plugin activo en todas las sesiones (`CLAUDE_CODE_PLUGIN_DIRS`).
- **3.7 listo:** dashboard web local `hz dashboard` (árbol + carriles, elegido por el humano; SVG calculado en Go, solo 127.0.0.1, D-0050) con los eventos por agente que guarda el plugin.
- **3.8 listo:** dashboard central multi-proyecto (D-0053): registro de usuario `proyectos.json` (hogar `%AppData%\hz` o `HZ_HOGAR`), cada sesión conecta su proyecto vía `hz puntero`, snapshot por sesión en `.harness/cache/actividad/` cerrado en `session.end`, `hz dashboard` levanta un único servidor desacoplado (`--detener`, `--desconectar`, `--primer-plano`). Solo lectura.
- **0.2.0 listo (mejoras por uso real):** `hz tdd superar T-n --por T-m --decision D-x`: estado terminal `superada` que exige evidencia (T-m bloquea con su contenido actual cada test de T-n que cambió) y decisión `humano` o `agente_aprobado` (D-0051, D-0054). `hz mutar` con respaldo y candado por archivo en `.harness/cache/mutar/`: mide archivos distintos en paralelo y repara huérfanos reclamándolos (`<h>.recuperando`) solo si es seguro (D-0052, D-0055). Pendiente: meta-eval (`hz mutar`) de `internal/tdd/tdd.go` y `internal/mutar/respaldo.go`.
- Siguiente: acciones desde el dashboard (mensaje, detener, gates) con canal propio; flag `--tecnico` del informe; validar el dashboard con un enjambre real.

## Comandos únicos
- Test: `go test ./...` · Vet: `go vet ./...` · Build: `go build -trimpath -ldflags "-s -w" -o bin/hz.exe ./cmd/hz`
- Plugin: `claude plugin test plugin` · `claude plugin validate plugin`
- Contratos (van antes que tests y código, RG-2): `contratos/cli.md` · `contratos/plugin.md`

## Mapa
| Tema | Dónde |
|---|---|
| Visión y alcance | `docs/diseno/00-vision.md` |
| Capas y modos (Génesis / Mejora) | `docs/diseno/01-capas-y-modos.md` |
| Reglas generales (RG) | `docs/diseno/02-reglas-generales.md` → texto en `reglas/generales.md` |
| Activación por proyecto y niveles N1–N3 | `docs/diseno/03-activacion.md` |
| Estructura y granularidad orientada a agentes | `docs/diseno/04-estructura.md` |
| Reglas duras de seguridad (RD, por proyecto) | `docs/diseno/05-reglas-duras.md` → catálogo en `reglas/catalogo-rd.md` |
| TDD, verificación, evals y lecciones | `docs/diseno/06-tdd-y-evals.md` |
| Motor SQLite (decisiones y memoria operativa) | `docs/diseno/07-motor.md` → DDL en `schema/harness.sql` |
| Misiones (hitos, rondas, verticales, milestones) y monitoreo de producción opcional | `docs/diseno/08-misiones.md` |
| Informes C-level | `docs/diseno/09-informes.md` |
| Brechas abiertas y spike | `docs/diseno/10-brechas.md` |
| Orquestación (roles, relevo, enjambres) | `docs/diseno/11-orquestacion.md` |
| Resultados del spike técnico (S1–S14) | `docs/diseno/12-spike.md` |
| Esquemas de configuración | `schema/*.schema.json` |
| CLI `hz`: entrada y subcomandos | `cmd/hz/main.go` · `cmd/hz/subcomandos.go` |
| CLI `hz`: paquetes (motor, tdd, check, proyecto, reglas, exportar, orquesta, capacidad, aprendizaje, mutar, redteam, estadistica, mision, informe, dashboard, registro) | `internal/` · recursos embebidos en `recursos.go` |
| Dashboard central: comando, servidor desacoplado y registro de proyectos | `cmd/hz/dashboard.go` · `internal/dashboard/dashboard.go` · `internal/registro/registro.go` |
| Plugin: guardias puros, actividad y cableado de hooks (incluido el pane) | `plugin/hooks/nucleo.ts` · `plugin/hooks/actividad.ts` · `plugin/hooks/register.tsx` · `plugin/hooks/hooks.json` · estado en `plugin/types/index.d.ts` |
| Skill del procedimiento (uso autónomo de `hz`) | `plugin/skills/harness/SKILL.md` |
| Skill del director (enjambres) y agentes de rol | `plugin/skills/enjambre/SKILL.md` · `plugin/agents/*.md` |
| Skill de red-team y lecciones del propio repo | `plugin/skills/redteam/SKILL.md` · `.claude/skills/` |
| Skills de misión e informe C-level (N3) | `plugin/skills/mision/SKILL.md` · `plugin/skills/informe/SKILL.md` · plantilla `internal/informe/plantilla.html` |
| Decisiones | `.harness/log/decisiones.jsonl` (fuente) → vista `docs/decisiones/INDICE.md` |
| Insumo original (lluvia de ideas) | `general_description.md` |

## Reglas que aplican a este repo
- **RG-1** Código de producción y eficiente en recursos (`reglas/generales.md`).
- **RG-2** Test-driven siempre: CONTRATO → ROJO → VERDE → REFACTOR.
- **RG-3** Toda decisión se registra con cuándo, qué y por qué en `.harness/log/decisiones.jsonl`.
- El harness es genérico: ningún proyecto concreto es su referencia (D-0001).

## Convenciones
- Idioma: español para docs, decisiones y commits.
- Commits solo cuando el humano lo pide.
- `AGENTS.md` siempre al día (D-0056): todo cambio que se integra actualiza en el mismo commit su línea de **Estado** (qué quedó listo, decisiones y pendientes) y, si agrega o mueve módulos, comandos o documentos, el **Mapa**.
- Antes de proponer un enfoque, busca en las decisiones si ya fue descartado.
