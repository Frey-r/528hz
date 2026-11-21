Mapa del Ciclo Completo (Dónde encaja cada pieza)
Plaintext

[1. Especificación] ──► [2. Contexto Vivo] ──► [3. Sandboxing]
                                                      │
         ┌────────────── (Lo cubierto en el video) ───┘
         ▼
[Feedback Loop + Agent Evals + Guardrails + (4. Validación Adversarial / 5. Sensores Deterministas)]
         │
         ▼
[6. CI/CD Autoreparable] ──► [7. Despliegue Progresivo] ──► [8. Observabilidad & AI SRE]
                                                                       │
         ┌─────────────────────────────────────────────────────────────┘
         ▼
[9. Garbage Collection (Entropía)]  +  [10. Gobernanza, Seguridad y FinOps]

Fase 1: Antes del código (Intención, Contexto y Entorno)
1. Especificación Ejecutable y Contratos "Default-FAIL" (Spec-Driven Development)

El video asume que el agente ya tiene claro qué construir; en la práctica, el principal fallo en tareas largas es la ambigüedad de intención y el agente declarando "listo" de forma prematura.

    Funcionamiento Abstracto: Separar la especificación de intención (qué debe cumplirse, límites y non-goals, aprobados por humanos) del plan de ejecución (pasos acotados por sesión), obligando a que el estado "terminado" sea estructural y no una opinión del modelo.

    Funcionamiento Técnico:

        Contratos Default-FAIL: Archivos de control (ej. test-results.json o contratos de sprint) donde cada criterio de aceptación arranca en false y un hook bloquea el cierre hasta adjuntar evidencia ejecutada.

        TDD como capa de control (Augmented Coding): Escribir/aprobar las pruebas antes de la implementación (Red-Green-Refactor) junto con mapas de impacto de tests, evitando que el agente tome atajos o modifique/borre pruebas para lograr un build en verde.

        Handoffs persistentes: Para tareas multi-hora, el agente mantiene un registro estructurado de avance (PROGRESS.md + commits atómicos) para que una nueva ventana de contexto retome el trabajo sin pérdida de estado.

    Fuentes y referentes: Anthropic (Effective Harnesses for Long-Running Agents y Harness Design for Long-Running Application Development), Kent Beck (Augmented Coding vs. Vibe Coding), y Thoughtworks.

2. Ingeniería de Contexto Progresiva y Memoria Institucional (Map, Not Manual & Context Lake)

El video menciona el Feature Map y las Skills, pero a escala los archivos gigantes de instrucciones se pudren rápido y saturan la ventana de contexto.

    Funcionamiento Abstracto: Tratar el repositorio como la fuente única de verdad (lo que vive en Slack o en la cabeza de alguien no existe para el agente) con descubrimiento progresivo y trazas de decisiones pasadas.

    Funcionamiento Técnico:

        Mapa corto + Docs versionados: Un AGENTS.md breve (~100 líneas) que actúa solo como tabla de contenidos apuntando a docs/ por dominio, verificados automáticamente por linters en CI (enlaces rotos, vigencia, owners).

        Decision Traces / Context Lake: Registrar qué se intentó antes y por qué se rechazó (además de estado vivo de servicios), evitando que un agente vuelva a abrir un PR con un enfoque que ya fue descartado la semana anterior.

        Compound Engineering (Aprendizaje compuesto): Cada corrección en code review o incidente alimenta permanentemente una regla, un test estructural o una skill del arnés para que el error no ocurra dos veces.

    Fuentes y referentes: OpenAI (Harness engineering: leveraging Codex in an agent-first world), Mitchell Hashimoto (cofundador de HashiCorp y creador de Ghostty), Kieran Klaassen y Dan Shipper (Every — Compound Engineering).

3. Sandboxing y Entornos Efímeros Paralelizables (Warm Devboxes & Worktrees)

Escalar de 3 a 50 agentes en paralelo en una máquina local colapsa por conflictos de archivos, puertos, bases de datos compartidas y riesgo de comandos destructivos con credenciales personales.

    Funcionamiento Abstracto: Dar a cada agente un entorno aislado, reproducible y desechable ("cattle, not pets"), con acceso estrictamente acotado.

    Funcionamiento Técnico:

        Devboxes precalentados: Pool de instancias/contenedores cloud listos en ~10 segundos con el repo clonado, cachés de compilación/type-check calientes y servicios base levantados.

        Aislamiento por git worktree: La aplicación debe poder levantarse de forma independiente por cada worktree, permitiendo pruebas destructivas en paralelo sin colisiones.

        Capa central de credenciales y herramientas: Exponer integraciones (CI, repos, cloud) mediante servidores MCP con tokens de mínimo privilegio por agente, nunca heredando credenciales personales del desarrollador.

    Fuentes y referentes: Stripe (Alistair Gray — Minions: Stripe's one-shot, end-to-end coding agents), OpenAI, y Gartner (Top Strategic Technology Trends in Software Engineering 2026 — Agent Sandboxing).

Fase 2: Validación Avanzada (Más allá del Feedback Loop básico)
4. Validación Adversarial, Escenarios Holdout y Gemelos Digitales (Anti-Reward-Hacking)

Incluso con Feedback Loops y Agent Evals, los modelos sufren de reward hacking: sobreajustan el código a las pruebas visibles o muestran sesgo de autocomplacencia cuando evalúan su propio trabajo.

    Funcionamiento Abstracto: Aplicar la lógica de entrenamiento/validación de Machine Learning al software: separar físicamente al generador del evaluador y evaluar contra escenarios ocultos en entornos simulados realistas.

    Funcionamiento Técnico:

        Evaluador escéptico de contexto fresco: Un segundo agente sin herramientas de escritura (Write/Edit) y con ventana de contexto limpia audita el resultado usando rúbricas explícitas e interactuando con la app real (ej. Playwright o Chrome DevTools Protocol).

        Escenarios Holdout: Batería de escenarios end-to-end almacenada fuera del repositorio accesible al agente generador, evaluada por métricas de satisfacción probabilística y no solo un pass/fail binario.

        Digital Twin Universe (DTU): Clones conductuales de dependencias externas críticas (Okta, Slack, Jira, Stripe, Google Drive, etc.) construidos contra sus SDKs públicos, permitiendo correr miles de escenarios de fallo por hora sin rate limits, costos de API ni riesgos en entornos reales.

    Fuentes y referentes: Justin McCarthy / StrongDM AI Team (Software Factory, analizado por Simon Willison), y Anthropic (arquitectura Generador vs. Evaluador escéptico).

5. Sensores Computacionales con Remediación Integrada (Feedforward vs. Feedback)

Complementa los Guardrails del video haciendo que los bloqueos enseñen deterministamente cómo arreglar el error sin depender de que el LLM adivine.

    Funcionamiento Abstracto: Distinguir controles computacionales (deterministas, corren en CPU en milisegundos, baratos y 100% fiables) de los inferenciales (LLM-as-a-judge, caros y probabilísticos), maximizando los primeros.

    Funcionamiento Técnico:

        Linters "LLM-friendly": Reglas custom cuyos mensajes de error funcionan como micro-instrucciones de autocorrección (explican la violación y muestran el patrón exacto esperado).

        Architectural Fitness Functions: Tests estructurales (ej. ArchUnit o validadores de grafos de dependencia) que validan fronteras entre módulos en cada commit.

        Revisión por niveles ("Consult the Oracle"): Usar modelos rápidos para iteración y escalar automáticamente a un modelo más capaz ("oráculo") únicamente para auditar diffs de alto riesgo o decisiones de diseño.

    Fuentes y referentes: Birgitta Böckeler en Martin Fowler (Harness engineering for coding agent users), OpenAI, y Mitchell Hashimoto.

Fase 3: Integración, Despliegue y Producción (El cierre del SDLC)
6. CI/CD Autoreparable y Políticas de Merge Basadas en Riesgo

Cuando los agentes producen decenas de PRs diarios, las prácticas tradicionales de revisión humana bloqueante se vuelven el cuello de botella, pero eliminar controles sin criterio dispara las caídas.

    Funcionamiento Abstracto: Tratar la integración continua como un sistema que se repara solo ante fallos rutinarios y gradúa la intervención humana según el impacto del cambio.

    Funcionamiento Técnico:

        Self-Healing CI: Agentes disparados por eventos de pipeline (builds rotos, actualizaciones de dependencias vía Renovate, tests rotos) que leen logs, diagnostican, commitean el parche en la rama y re-ejecutan hasta verde.

        PRs de vida corta y Merge Queues: Cambios atómicos con puertas obligatorias de seguridad automatizada (SAST, escaneo de secretos, SCA de dependencias).

        Human-in-the-Loop por riesgo (Policy Gates): Autonomía total para despliegues en staging o tareas acotadas; checkpoints humanos obligatorios en migraciones destructivas de base de datos, pagos, autenticación o ventanas críticas de producción.

    Fuentes y referentes: OpenAI (Throughput changes the merge philosophy), Elastic (Self-Healing CI en monorepos), Anthropic (2026 Agentic Coding Trends Report), y Port.io (The Agentic SDLC).

7. Entrega Progresiva y Rollback Automático (Progressive Delivery)

Es el cinturón de seguridad indispensable para permitir despliegues de código generado por agentes sin depender de revisión línea por línea.

    Funcionamiento Abstracto: Desacoplar el despliegue (subir código a producción) de la liberación (exponerlo a usuarios), limitando el radio de explosión (blast radius) y automatizando la reversión ante señales de degradación.

    Funcionamiento Técnico:

        Feature Flags y Kill Switches: Subir cambios "en oscuro" (dark launch), probarlos con usuarios internos y poder apagarlos en segundos por configuración sin redeploy.

        Canary Releases y Shadow Traffic con puertas métricas: Herramientas como Argo Rollouts + Prometheus/OpenTelemetry que escalan tráfico gradualmente (1% → 5% → 25% → 100%) verificando automáticamente error rate, latencia P99 y métricas de negocio. Si se rompe un umbral, el sistema hace rollback automático y abre un issue asignado a un agente.

    Fuentes y referentes: Charity Majors (CTO de Honeycomb, coautora de Observability Engineering), LaunchDarkly, y prácticas modernas de Progressive Delivery.

8. Observabilidad Legible por Agentes y Bucle de Auto-Sanación (Agent-Legible Observability & AI SRE)

Conecta la operación en producción de vuelta con los agentes de desarrollo para cerrar el ciclo completo.

    Funcionamiento Abstracto: Exponer métricas, logs estructurados y trazas distribuidas como herramientas nativas del agente, tanto durante el desarrollo como ante incidentes reales en producción.

    Funcionamiento Técnico:

        Telemetría consultable en desarrollo: Dar a cada entorno efímero/worktree su propia pila de observabilidad consultable (ej. vía LogQL / PromQL / OpenTelemetry), haciendo viables tareas como "optimiza hasta que ningún span crítico supere los 500ms".

        Triage y remediación autónoma (AI SRE): Ante una alerta en producción, un agente correlaciona el último despliegue, consulta trazas y logs, reproduce el fallo en un sandbox, genera un test de regresión + PR con el fix y entrega el diagnóstico completo al ingeniero de guardia.

    Fuentes y referentes: OpenAI (pilas desechables con LogQL/PromQL), Charity Majors / Honeycomb, y StrongDM.

Fase 4: Sostenibilidad a Largo Plazo
9. Gestión de Entropía y "Garbage Collection" de Código (Anti-Slop)

Los agentes tienden a imitar los patrones que ya ven en el repositorio—incluidos los malos—, lo que genera una acumulación acelerada de deuda técnica (AI slop) y caída en la estabilidad de entregas (fenómeno documentado en el reporte DORA de desarrollo asistido por IA).

    Funcionamiento Abstracto: Combatir la degradación arquitectónica de forma continua y automatizada, en lugar de esperar a que el sistema colapse.

    Funcionamiento Técnico:

        Agentes de Garbage Collection periódicos: Jobs en segundo plano que escanean diariamente el repositorio contra "principios dorados", detectando desviaciones, código muerto, feature flags caducados, tests flaky y docs obsoletas, abriendo PRs pequeños de refactorización y manteniendo un puntaje de salud por dominio.

        Poda del arnés (Harness Pruning): Cada vez que sale un modelo más capaz, probar a desactivar reglas y piezas del arnés una por una para eliminar restricciones heredadas que ya no aportan valor y solo añaden fricción.

    Fuentes y referentes: OpenAI (Entropy and garbage collection), Kent Beck (Balance Expansion & Contraction / Genie Tarpit), Mitchell Hashimoto (Anti-slop sessions), Anthropic (Re-simplify on model upgrades), y DORA.

10. Gobernanza, Registro de Agentes, Seguridad y FinOps (Agent Registry & Circuit Breakers)

Cuando una organización pasa de experimentos individuales a decenas de agentes operando sobre repositorios y pipelines reales, el riesgo pasa a ser operativo, financiero y de seguridad.

    Funcionamiento Abstracto: Mantener inventario, trazabilidad, presupuestos y frenos de emergencia claros sobre toda la flota de agentes.

    Funcionamiento Técnico:

        Agent Registry y Audit Trails: Catálogo con dueño humano responsable, herramientas permitidas, servicios que toca y registro auditable de qué prompt/agente generó cada cambio.

        Circuit Breakers y FinOps: Límites duros de reintentos, tiempo de ejecución y consumo de tokens por tarea para evitar bucles infinitos costosos, midiendo retorno real (gasto en cómputo vs. tiempo ahorrado / incidentes evitados).

        Controles de procedencia y confianza: Mecanismos (como Vouch en entornos colaborativos/open-source) y políticas claras de accountability humano sobre todo lo que entra a ramas principales.

    Fuentes y referentes: Port.io (Agentic SDLC), Mitchell Hashimoto (Vouch), Simon Willison (Agentic Engineering), Gartner y Veracode (estudios de seguridad en código generado por LLMs).
