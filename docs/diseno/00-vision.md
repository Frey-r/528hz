# 00 · Visión

## Qué es 528hz
Un **harness de trabajo** para Claude Code: convenciones de proyecto, skills, agentes de rol, hooks y un CLI (`hz`). Se instala una vez como plugin a nivel usuario y se activa proyecto por proyecto.

Es **100 % personal y local** (D-0018): corre en la máquina del usuario y se dimensiona según su hardware. No está pensado para equipos ni para infraestructura compartida.

Su función es que en flujos agénticos largos **lo natural para el agente sea lo correcto**: seguir la estructura del proyecto, empezar por contratos y tests, respetar reglas duras que no puede saltar, registrar sus decisiones y aprender de sus errores.

## Dos modos de uso
| Modo | Cuándo | Resultado |
|---|---|---|
| **Génesis** | Proyecto desde 0 | Estructura orientada a agentes, contratos, reglas duras y CI en verde desde el día 1; hitos construidos con TDD |
| **Mejora continua** | Proyecto existente | Verticales esenciales medidas, rondas de mejora con comparación estadística y milestones que muestran los saltos |

## Qué debe poder hacer (capacidades A–H)
- **A. Estructura y granularidad orientada a agentes:** AGENTS.md como fuente principal. Se activa por proyecto y en proyectos existentes sigue su estructura → `04-estructura.md`
- **B. Reglas duras de seguridad:** se definen por proyecto, son inquebrantables y su efectividad se mide → `05-reglas-duras.md`
- **C. Test-driven siempre:** contratos → tests → código. Todo lleva evals, también lo determinista, y hay un ciclo error → lección → eval → `06-tdd-y-evals.md`
- **D. Decisiones con cuándo, qué y por qué:** van a un motor SQLite que entrega lo justo sin llenar el contexto → `07-motor.md`
- **E. Misiones largas:** hitos (Génesis); verticales, rondas y milestones (Mejora) → `08-misiones.md`
- **F. Informes C-level y visuales por defecto** → `09-informes.md`
- **G. Reglas generales para todo proyecto:** la primera es RG-1, eficiencia de recursos → `02-reglas-generales.md`
- **H. Orquestación por roles:**
  - cada parte operativa la toma un subagente que recibe solo su porción del harness;
  - un supervisor verifica y gestiona el relevo;
  - los agentes conversan para pedir contexto;
  - hay enjambres para varias funcionalidades a la vez.

  Detalle en `11-orquestacion.md`.

## Principios
1. **Camino natural.** El agente toma el camino de menor resistencia, así que el harness hace que el camino estructurado sea el más corto. Para eso usa un mapa corto, contexto por dominio, recetas con ejemplares, un comando único por acción, denegaciones que enseñan la corrección y lecciones justo a tiempo.
2. **Determinista antes que probabilístico.** Lo que puede verificar un script no lo juzga un LLM, y los números nunca los cuenta el modelo.
3. **Contexto por demanda.** La información se consulta (SQLite, AGENTS.md del dominio, preguntas a otros agentes) en vez de precargarse.
4. **No romper la autonomía.**
   - Sin perfil activo, el harness es no-op.
   - Los hooks bloquean solo las reglas duras y el orden TDD.
   - Toda denegación trae su remediación.
   - `/goal`, Agent, Workflow, Monitor, worktrees, memoria y MCPs siguen funcionando igual.
5. **Todo se mide, también el harness.** Verificadores, reglas, lecciones y el propio harness tienen evals.

## Fuera de alcance
- **Protección de PII.** Ya viene de fábrica en las normas de Claude y no se re-especifica (D-0003).
- **CI/CD autoreparable y entrega progresiva** (pilares 6 y 7 de `general_description.md`): se omiten. Venían de la investigación previa y no aplican a un harness personal y local (D-0031).
- **El monitoreo de producción** (pilar 8) **no** queda fuera: es un módulo **opcional por proyecto**, con la skill de monitoreo que use ese proyecto (`08-misiones.md` § Monitoreo de producción, D-0032).
- **El dashboard web.** Es secundario y va al final (D-0002).

## Cómo se distribuye
Es un plugin de Claude Code a nivel usuario, servido desde un marketplace local en este repo (D-0004). Durante el desarrollo se carga con `CLAUDE_CODE_PLUGIN_DIRS`.
