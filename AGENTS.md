# 528hz: Harness para flujos agénticos largos

Harness de trabajo genérico para Claude Code. Funciona en dos modos:
- **Génesis:** proyecto desde 0.
- **Mejora continua:** proyecto existente.

Este archivo es un **mapa**, no un manual. Cada sección apunta a donde vive el detalle.

## Estado
- **3.1 listo:** CLI `hz` en Go (motor SQLite + JSONL, decisiones, TDD, check, estado).
- **3.2 listo:** plugin `hz` (guardias TDD/RD/autoprotección, DLP, puntero por hook de comando), skill `hz:harness`, `activar --nivel N2`, `exportar`, `puntero`. Validado en sesión real (`docs/diseno/12-spike.md`).
- Siguiente: 3.3, orquestación (agentes de rol, tarjetas, arriendos, `hz capacidad`, `/hz enjambre`).

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
| CLI `hz`: paquetes (motor, tdd, check, proyecto, reglas, exportar) | `internal/` · recursos embebidos en `recursos.go` |
| Plugin: guardias puros y cableado de hooks | `plugin/hooks/nucleo.ts` · `plugin/hooks/register.ts` · `plugin/hooks/hooks.json` |
| Skill del procedimiento (uso autónomo de `hz`) | `plugin/skills/harness/SKILL.md` |
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
