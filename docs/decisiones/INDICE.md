# Índice de decisiones

> Vista generada desde `.harness/log/decisiones.jsonl` (fuente, RG-3). No editar a mano. Ficha completa: `hz decision ver D-NNNN`.

| ID | Cuándo | Qué | Decidió | Estado |
|---|---|---|---|---|
| D-0001 | 2026-11-20 15:55 | El harness es 100 % genérico; ningún proyecto concreto es su referencia. | humano | vigente |
| D-0002 | 2026-11-20 15:56 | El dashboard web es secundario; se prioriza el harness. | humano | vigente |
| D-0003 | 2026-11-20 15:57 | La protección de PII queda fuera del harness. | humano | vigente |
| D-0004 | 2026-11-20 16:00 | El harness se distribuye como plugin de Claude Code a nivel usuario, desde un marketplace local en 528hz; en desarrollo se carga con CLAUDE_CODE_PLUGIN_DIRS. | agente_aprobado | vigente |
| D-0005 | 2026-11-20 16:02 | La adopción de un proyecto existente no reestructura: documenta la estructura existente como norma. | humano | vigente |
| D-0006 | 2026-11-20 16:04 | Los gates humanos son no bloqueantes: el hook deniega, deja el gate pendiente y se aprueba por chat, pane o panel. | agente_aprobado | vigente |
| D-0007 | 2026-11-20 16:06 | Las decisiones y la memoria operativa viven en SQLite (índice consultable) con un log JSONL append-only versionado como fuente. | agente_aprobado | vigente |
| D-0008 | 2026-11-20 16:08 | Los errores de los agentes se registran y generan una skill-lección; la lección pasa por eval y tiene ciclo de vida (crítico → eliminar, menor → reescribir). | humano | vigente |
| D-0009 | 2026-11-20 16:09 | Los informes son C-level y visuales por defecto; técnico solo a pedido. | humano | vigente |
| D-0010 | 2026-11-20 16:12 | AGENTS.md es la fuente principal de contexto del proyecto; CLAUDE.md solo como puente de una línea si hace falta. | humano | vigente |
| D-0011 | 2026-11-20 16:13 | Las reglas duras de seguridad se definen por proyecto, son inquebrantables (5 capas de enforcement) y se mide su efectividad. | humano | vigente |
| D-0012 | 2026-11-20 16:14 | Se trabaja siempre test-driven: CONTRATO → ROJO → VERDE → REFACTOR, aplicado por hook. | humano | vigente |
| D-0013 | 2026-11-20 16:15 | RG-1: todo código es de producción y eficiente en recursos (reutilizar conexiones, motores e instancias; mutar cuando sea seguro; liberar; lotes). | humano | vigente |
| D-0014 | 2026-11-20 16:20 | Cada parte operativa del harness la toma un rol que recibe solo su porción (p. ej. el implementador no recibe la verificación). | humano | vigente |
| D-0015 | 2026-11-20 16:21 | El relevo entre roles es supervisado: tarjeta de traspaso validada de forma determinista; el sucesor recibe solo su porción y la tarjeta. | agente_aprobado | vigente |
| D-0016 | 2026-11-20 16:22 | El enjambre jerárquico (director → líderes → roles) es el motor por defecto; Workflow se usa para pipelines repetitivos y masivos. | agente_aprobado | vigente |
| D-0017 | 2026-11-20 16:23 | Los agentes paralelos usan arriendos de archivos aplicados por hook; los archivos compartidos solo los cambia el contratista con aviso. | agente_aprobado | vigente |
| D-0018 | 2026-11-20 17:20 | El harness es 100 % personal y local; las RG-4 en adelante las define el usuario. | humano | vigente |
| D-0019 | 2026-11-20 17:21 | Los milestones los define el usuario: el harness pregunta en cada cierre de ronda y propone uno cuando la mejora es evidente y grande (salto > k × ruido); nunca promueve solo. | humano | vigente |
| D-0020 | 2026-11-20 17:22 | Se adoptan las propuestas para resiliencia y límites de máquina, rol humano, coordinación multi-sesión, métricas del harness, idioma y exenciones TDD. | humano | vigente |
| D-0021 | 2026-11-20 17:23 | Las lecciones no tienen tope; cuando son muchas pasan a modo índice (mapeo por ámbito en SQLite + inyección justo a tiempo). | humano | vigente |
| D-0022 | 2026-11-20 17:24 | Las verticales se definen conversando con el usuario; cada una mide efectividad (qué tan bien), impacto y tasa de fallos. | humano | vigente |
| D-0023 | 2026-11-20 17:25 | El tamaño del enjambre es dinámico según hardware y viabilidad (`hz capacidad`); si el hardware lo permite se montan, por ejemplo, 12 células simultáneas. | humano | vigente |
| D-0024 | 2026-11-20 17:26 | El orquestador decide qué modelo usa cada agente al lanzarlo; la elección se registra. | humano | vigente |
| D-0025 | 2026-11-20 17:40 | El harness nunca crea CLAUDE.md; si el proyecto ya tiene uno, le agrega @AGENTS.md. Los AGENTS.md de dominio se complementan con inyección por hook donde no hay carga nativa. | agente_aprobado | vigente |
| D-0026 | 2026-11-20 17:41 | Las reglas duras se aplican en capas: deny nativo + hook de entrada (allowlist para lo crítico) + DLP de salida + git/CI + autoprotección; en perfil crítico, los secretos quedan fuera de alcance o se usa sandbox WSL2. | agente_aprobado | vigente |
| D-0027 | 2026-11-20 17:42 | La mensajería entre agentes se dirige por agentId; el harness mantiene el directorio rol↔agentId e inyecta el supervisor en el paquete de rol. | agente_aprobado | vigente |
| D-0028 | 2026-11-20 17:43 | El tablero de tareas (DAG) vive en SQLite (`hz tablero`). | agente_aprobado | vigente |
| D-0029 | 2026-11-20 17:44 | Las lecciones pasan a modo índice desde 30 (≈69 tokens por skill por request). | agente_aprobado | vigente |
| D-0030 | 2026-11-20 17:45 | `hz capacidad` dimensiona el enjambre con RAM libre, núcleos y la carga medida por célula (≈45 MB por agente + pico de la célula) y frena lanzamientos si la RAM baja de la reserva. | agente_aprobado | vigente |
| D-0031 | 2026-11-20 17:55 | Se omiten los pilares 6 (CI/CD autoreparable) y 7 (entrega progresiva) de general_description.md. | humano | vigente |
| D-0032 | 2026-11-20 17:56 | El monitoreo de producción (pilar 8) es un módulo opcional por proyecto: la skill de monitoreo la declara cada proyecto (p. ej. grafana-loki), con acceso de solo lectura, para sensores de verticales y triage de incidentes. | humano | vigente |
| D-0033 | 2026-11-20 18:15 | El CLI hz se escribe en Go (binario único), no en Bun + TypeScript | humano | vigente |
| D-0034 | 2026-11-20 18:15 | Los hooks del plugin leen .harness/cache/hooks.json en vez de lanzar hz en cada tool call | agente | vigente |
| D-0035 | 2026-11-20 18:43 | El registro de decisiones no depende solo del agente: el rol registrador revisa cada tarea cerrada y propone decisiones no registradas; la rúbrica del evaluador pregunta si hubo decisiones sin registrar | humano | vigente |
| D-0036 | 2026-11-20 19:06 | El puntero de inicio de sesión lo entrega un hook de comando SessionStart (hz puntero) declarado en el hooks.json del plugin, no un hook de mod | agente | vigente |
