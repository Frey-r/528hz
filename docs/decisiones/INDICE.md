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
| D-0037 | 2026-11-20 20:15 | Los arriendos se toman por tarea: el líder arrienda los archivos a nombre de la tarea y puede escribirlos todo agente cuya descripción declare esa tarea («<rol> T-n») | agente | vigente |
| D-0038 | 2026-11-20 20:15 | En el guardia de bash, los segmentos que invocan hz no se evalúan y los operadores entre comillas son datos | agente | vigente |
| D-0039 | 2026-11-20 20:40 | El índice SQLite se versiona por hash del DDL: con otro esquema se descarta y se reconstruye desde los logs | agente | vigente |
| D-0040 | 2026-11-20 20:40 | La primera lección real (L-0001, escapes por heredoc) no se activó: su eval de prevención falló porque sin la lección el error tampoco ocurrió; queda reescrita para mejorar el caso de reproducción | agente | vigente |
| D-0041 | 2026-11-20 20:40 | El mutation testing (hz mutar) es la meta-eval estándar de los verificadores deterministas; sus sobrevivientes se registran como error test_debil | agente | vigente |
| D-0042 | 2026-11-20 21:23 | Una propuesta de milestone solo cuenta métricas con piso de ruido conocido (medido con hz ruido o declarado en mision.yaml); al cerrar una ronda con mantener siempre se pregunta, y una ronda revertida no es candidata | agente | vigente |
| D-0043 | 2026-11-20 21:23 | El presupuesto de misión es un circuit breaker en agent.spawn: hz exporta misiones activas con sin_presupuesto a hooks.json y el plugin deniega lanzar subagentes con remediación; usd_max se lee directo de mision.yaml | agente | vigente |
| D-0044 | 2026-11-20 21:23 | El informe C-level es una página autocontenida que genera hz en Go (html/template, SVG calculado en Go, sin JS) y se publica como Artifact privado en la misma URL por milestone; hz informe --json es el contrato de datos | agente | vigente |
| D-0045 | 2026-11-21 11:11 | El informe C-level es solo del modo Mejora continua; Génesis no tiene informe | humano | vigente |
| D-0046 | 2026-11-21 11:19 | El verificador solo entrega una tarea si sus tests superan un umbral de mutation testing (verificacion.mutacion_min, 0.9 por defecto) en cada archivo de código tocado; hz mutar muestrea todo el archivo | humano | vigente |
| D-0047 | 2026-11-21 11:26 | hz activar genera solo reglas deny nativas Read(ruta) y Edit(ruta); retira las Write(ruta) que generaron versiones anteriores cuando la ruta ya tiene su Edit(ruta) | agente | vigente |
| D-0048 | 2026-11-21 11:26 | L-0001 (escapes por heredoc) se activó: el A/B que mide el proceso demostró la prevención; los evals de lecciones miden intentos fallidos y costo, no solo el resultado final | agente | vigente |
| D-0049 | 2026-11-21 11:42 | Observabilidad: el plugin lleva la actividad por agente en $.state (sobrevive recargas), la muestra en la status line y en el pane /hz-panel (árbol por supervisor), y escribe un snapshot .harness/cache/actividad.json (al lanzar/terminar y como máximo cada 2 s) que lee hz panel [--json], contrato de datos del dashboard | agente | vigente |
| D-0050 | 2026-11-21 11:44 | Dashboard web local: hz dashboard sirve en 127.0.0.1 una vista árbol + carriles (elegida por el humano) calculada en Go como SVG; la página solo refresca el fragmento cada 2 s; el plugin guarda los últimos 100 eventos por agente para los carriles | humano | vigente |
| D-0051 | 2026-11-22 22:26 | Las tareas TDD se pueden marcar como superadas con hz tdd superar T-n --por T-m --decision D-x: estado nuevo y terminal 'superada', sin tocar ni re-bloquear tests | agente_aprobado | vigente |
| D-0052 | 2026-11-22 22:26 | hz mutar usa un respaldo por archivo (.harness/cache/mutar/<sha256 de la ruta, 16 hex>.bak) con un candado exclusivo <h>.lock (pid, host, inicio, sha256) y latido por mtime cada 10 s; una segunda medición del mismo archivo se rechaza con código 1 y los huérfanos se restauran solo si el archivo es igual al original o un mutante de él | agente_aprobado | vigente |
| D-0054 | 2026-11-23 11:05 | hz tdd superar exige evidencia: al menos un test bloqueado de T-n cambió y T-m bloquea cada uno de los que cambiaron con su contenido actual; la decisión que lo justifica debe ser humano o agente_aprobado | agente_aprobado | vigente |
| D-0055 | 2026-11-23 11:05 | La reparación de huérfanos de hz mutar se reclama creando en exclusiva <h>.recuperando y relee el candado antes de tocar nada; un reclamo vencido se avisa y no se borra solo | agente_aprobado | vigente |
