# 10 · Brechas de `general_description.md`

## Lo que el md no cubría y quedó resuelto

| Tema | Resolución | Doc |
|---|---|---|
| Los dos modos de uso (Génesis / Mejora) | Eje del harness | `01` |
| Fuente de contexto | AGENTS.md (nativo). Nunca se crea CLAUDE.md | `04` |
| Seguridad a nivel proyecto | RD por proyecto, inquebrantables, en capas, con efectividad medida y red-team | `05` |
| Definición de «terminado» | TDD siempre + escalera V0–V5 + contrato default-FAIL | `06` |
| Evals más allá de lo LLM | Evals de producto, meta-evals de verificadores y evals del agente y sus lecciones | `06` |
| Aprender de los errores | Error → lección (skill) → eval → ciclo de vida | `06` |
| Formato de decisiones | Cuándo, qué y por qué en SQLite + JSONL | `07` |
| Mejora enfocada | Verticales (efectividad, impacto, fallos) → rondas → milestones | `08` |
| Comunicación | Informes C-level visuales | `09` |
| Paralelismo y roles | Porción del harness por rol, relevo supervisado, conversación, enjambres | `11` |
| Reglas generales | RG-1 recursos, RG-2 TDD, RG-3 decisiones | `02` |
| PII | Fuera del harness: viene de fábrica | `00` |

## Brechas abiertas: resueltas por el usuario (2026-11-20)

| # | Brecha | Resolución | Decisión |
|---|---|---|---|
| 1 | Público y escala | **100 % personal y local** | D-0018 |
| 2 | Reglas generales RG-4+ | Las define el usuario | D-0018 |
| 3 | Estadística de la mejora / milestones | **Los define el usuario.** El harness pregunta en cada cierre de ronda y propone un milestone cuando la mejora es evidente y grande (> k × ruido, k = 2) | D-0019 |
| 4 | Resiliencia y límites de máquina | Propuesta aceptada: estado en cada transición, registro de PIDs, límites según RAM | D-0020 |
| 5 | Matriz del rol humano | Propuesta aceptada: interrumpir solo por gate, bloqueo, milestone o fin | D-0020 |
| 6 | Coordinación multi-sesión | Propuesta aceptada: arriendos + worktree por sesión de misión + lock de misión | D-0020 |
| 7 | Gobernanza de lecciones | **Sin tope.** Sobre 30 lecciones pasan a modo índice (mapa por ámbito + inyección justo a tiempo) | D-0021, D-0029 |
| 8 | Métricas del propio harness | Propuesta aceptada: tiempo de retoma, intervenciones, re-trabajo, recurrencia, escapes y costo por milestone | D-0020 |
| 9 | Pilares 6–8 de `general_description.md` | **Los pilares 6 (CI/CD) y 7 (entrega progresiva) se omiten.** **El pilar 8 (monitoreo de producción) queda como módulo opcional por proyecto**, con la skill de monitoreo que declare cada uno (p. ej. `grafana-loki`) | D-0031, D-0032 |
| 10 | Idioma | Propuesta aceptada: campo `idioma`, español por defecto | D-0020 |
| 11 | Exenciones al TDD | Propuesta aceptada: docs, configuración trivial y renombres mecánicos | D-0020 |
| 12 | Tamaño del enjambre | **Dinámico según hardware y viabilidad** (p. ej. 12 células si el hardware lo permite). Lo calcula `hz capacidad` | D-0023, D-0030 |
| 13 | Modelos por rol | **Los decide el orquestador** en cada lanzamiento; la elección se registra | D-0024 |

## Spike técnico
Los resultados (S1–S14) están en `12-spike.md`: 11 confirmados, 2 con alternativa (mensajería por nombre y TaskList) y 1 con riesgo residual documentado (S4, Windows sin sandbox).
