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
- Siguiente: observabilidad (status line, pane y luego dashboard web, con opciones de visualización).

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
| CLI `hz`: paquetes (motor, tdd, check, proyecto, reglas, exportar, orquesta, capacidad, aprendizaje, mutar, redteam, estadistica, mision, informe) | `internal/` · recursos embebidos en `recursos.go` |
| Plugin: guardias puros y cableado de hooks | `plugin/hooks/nucleo.ts` · `plugin/hooks/register.ts` · `plugin/hooks/hooks.json` |
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
- Antes de proponer un enfoque, busca en las decisiones si ya fue descartado.
