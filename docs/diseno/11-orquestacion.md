# 11 · Orquestación: roles, relevo supervisado y enjambres (H)

Cada parte operativa del harness la toma un **subagente responsable** que recibe **solo su porción del harness**. Un **supervisor** verifica lo que hace cada subagente, gestiona su flujo y lanza a su **sucesor**, que recibe otra porción. Los agentes **conversan entre sí** para pedir solo el contexto que necesitan. Para construir varias funcionalidades a la vez se lanza un **enjambre** de líderes, cada uno con sus subagentes.

## Primitivas de Claude Code 2.1.329 en que se apoya (verificadas en el spike, `12-spike.md`)
- **`Agent`:** `isolation: "worktree"` y `subagent_type`. ✅ Los roles lanzados sin `isolation` heredan el worktree del líder, y un guardia nativo impide comandos que apunten fuera de él (S10). `model` lo fija el orquestador.
- **Anidamiento:** la profundidad por defecto es 3, suficiente para director → líder → rol → ayudante (S7). **Concurrencia:** 20 por defecto, y 12 células simultáneas funcionaron (S14).
- **`SendMessage`:** ✅ **por `agentId`** entre cualquier par: padre ↔ hijo y entre hermanos. Si el receptor está ocupado, el mensaje llega en su siguiente turno (S8). ❌ Por nombre no: el parámetro `name` no se respeta.
- **Tablero:** ❌ `TaskCreate/TaskList` no están en este build (S9). **El tablero vive en SQLite** (`hz tablero`).
- **Hooks del plugin:**
  - `agent.spawn` reescribe el prompt (inyecta el paquete de rol) y trae `parentAgentId` (S11).
  - `tool.call` trae `agentId` (S12) y puede adjuntar `context` (S2).
  - `$.session.send({ to: { agentId } })` permite que el mod avise a cualquier agente (S13).
  - `agent.report` enruta el informe del agente.
  - `session.send` audita los mensajes.

## 1. Topología

```
                         HUMANO
                           │  gates · aprobaciones · informes
                       ┌───▼────┐
                       │DIRECTOR│  sesión principal: misión, DAG, capacidad, integración final
                       └───┬────┘
        ┌──────────────────┼──────────────────┬───────────────┐
   ┌────▼────┐        ┌────▼────┐        ┌────▼─────┐   ┌─────▼─────┐
   │LÍDER  F1│        │LÍDER  F2│        │INTEGRADOR│   │ CURADOR   │  transversales
   │worktree │        │worktree │        │cola merge│   │ lecciones │  (asíncronos)
   └────┬────┘        └────┬────┘        └──────────┘   └───────────┘
        │ relevo supervisado (por tarea)
   CONTRATISTA ─► TESTER ─► IMPLEMENTADOR ─► VERIFICADOR ─► EVALUADOR
        └────────────── REGISTRADOR (asíncrono) ──────────────┘
```

## 2. Roles y su porción del harness

| Rol | Responsable de | Recibe | **No** recibe | Escribe en | Entrega |
|---|---|---|---|---|---|
| **Director** | Misión, DAG, gates con humano, capacidad, cierre | `mision.yaml`, verticales, resumen de RD, tablero, tarjetas de líderes | Código, transcripts de roles | Misión, tablero | Informe C-level |
| **Líder de célula** | Supervisar sus roles, validar tarjetas, lanzar sucesores, resolver bloqueos | Contrato de la funcionalidad, DAG local, tarjetas | Código completo, escalera de verificación | Tablero | Tarjeta de célula |
| **Contratista** | Contratos + presupuesto de recursos | Spec, AGENTS.md, estructura, RG-1, decisiones del tema | Tests, holdout | `contratos/` | Tarjeta CONTRATO |
| **Tester** | Tests desde contratos; `hz tdd rojo` | Contrato, recetas de test | Código de producción | Tests | Tarjeta ROJO |
| **Implementador** | Código que pasa los tests | Contrato, tests (lectura, bloqueados), AGENTS.md del dominio, recetas, RG-1, lecciones del ámbito | **Escalera de verificación, red-team, holdout, rúbrica del evaluador** | Solo archivos arrendados | Tarjeta VERDE |
| **Verificador** | V0–V3, sensores RG-1, evidencia en el contrato, registrar errores | Política de verificación, comandos, sensores | Razonamiento del implementador | Solo el motor (`hz`) | Tarjeta VERIFICACIÓN |
| **Evaluador escéptico** | V5: rúbrica (contratos, RG, RD), holdout, contexto fresco | Rúbrica, diff, holdout | Tarjetas y razonamientos previos | Nada | Veredicto |
| **Registrador** | Decisiones, PROGRESO, cola de errores | Formatos del motor | Código | Motor | — |
| **Integrador** | Cola de merge: integra células una a una y corre la suite completa | Ramas, suite, DAG | Detalle interno de las células | Rama de integración (`hz git`) | Tarjeta INTEGRACIÓN |
| **Curador** | Error → lección → eval A/B → ciclo de vida | Taxonomía, errores, casos | Misión en curso | `.claude/skills/hz-leccion-*` | Estado de lecciones |
| **Diagnosticador** (Mejora) | Análisis de un cluster de fallos | Resultados de la ronda (su cluster) | Código de otros clusters | Motor | Diagnóstico |
| **Medidor** (Mejora) | Correr sensores + `hz compare` | Sensores, rondas | Código | Motor | Mediciones |
| **Oráculo** | Revisión de diffs de alto riesgo | Diff, contrato | — | Nada | Veredicto |

**Modelos: los decide el orquestador** (D-0024). El director, o el líder dentro de su célula, elige el modelo de cada agente al lanzarlo (parámetro `model` del tool Agent) según la complejidad, el riesgo y el presupuesto de la tarea. No hay una tabla fija. La elección queda en la tabla `agentes`, así que se puede auditar y comparar el costo y la calidad por rol y modelo.

**Cómo se aplica:**
- Cada rol es un agente del plugin (`agents/hz-<rol>.md`) con una **allowlist de herramientas**. No fija modelo: lo decide el orquestador.
- El hook `agent.spawn` inyecta el **paquete de rol**, que arma `hz contexto --rol R --tarea T` con un tope de tokens por rol. También deniega lanzar un rol fuera de orden: por ejemplo, un implementador sin ROJO registrado.
- El hook `tool.call` aplica el alcance de escritura de cada rol y los arriendos, usando el `agentId`.
- El orquestador fija el modelo en cada lanzamiento. El hook `agent.spawn` lo registra.

## 3. Supervisión y relevo

### 3.1 Cadena por tarea
`Contratista → Tester → Implementador → Verificador → Evaluador`. El Registrador corre en paralelo y de forma asíncrona.

### 3.2 Tarjeta de traspaso (≤30 líneas, se guarda en `traspasos`)
```yaml
tarea: T-12
rol: implementador
estado: listo | fallo | bloqueado
hecho: "…una o dos frases…"
evidencia:
  - cmd: "bun test tests/api/test_pedidos.ts"
    resultado: "12 passed"
    commit: a1b2c3d
archivos: [app/api/pedidos.ts]
decisiones: [D-0042]
dudas: ["¿El descuento aplica antes o después de impuestos? → preguntado a contratista (M-88)"]
para_el_sucesor: "Revisar RG-1.3 en app/api/pedidos.ts:40 (cliente compartido)."
```

### 3.3 Validación determinista (`hz salida --rol R --tarea T`)
| Rol | Criterio de salida |
|---|---|
| Contratista | Contrato válido contra su esquema; presupuesto de recursos presente; aprobado si es feature |
| Tester | `hz tdd rojo` registrado; los tests fallan por aserción |
| Implementador | Tests de la tarea en verde; tests bloqueados intactos (hash); solo tocó archivos arrendados |
| Verificador | Evidencia V0–V3 registrada en el contrato; errores registrados |
| Evaluador | Veredicto con rúbrica completa |

**Si pasa:** el líder lanza al sucesor con **su porción + la tarjeta**. Nunca le pasa el transcript.

**Si falla:**
1. Registra el error.
2. Lanza un agente **fresco** del mismo rol con la tarjeta de falla.
3. Tras 2 reintentos, escala al director.
4. Después, escala al humano.

### 3.4 Watchdog
El ledger vigila cada agente. Estas señales lo disparan:
- tiempo sin progreso;
- tokens por sobre el presupuesto del rol;
- tool calls fallando en bucle;
- acciones repetidas.

Cuando salta, el mod avisa al líder con un `ALERTA`. El líder pide estado o detiene y relanza.

### 3.5 Nivel superior
El director supervisa a los líderes igual que el líder supervisa a sus roles: tarjetas de célula, criterios de salida de funcionalidad y watchdog.

## 4. Conversación entre agentes: el contexto se pide, no se carga

**Arranque mínimo:** paquete de rol + tarjeta del predecesor.

**Escalera de consulta** (de la más barata a la más cara):
1. `hz` (SQLite): decisiones, contratos, reglas, lecciones.
2. AGENTS.md del dominio.
3. **SendMessage al dueño del conocimiento, por `agentId`.** `hz equipo` muestra el directorio: rol → `agentId` → tarea → archivos arrendados.
   - El mod lo mantiene solo. En `agent.spawn` registra el rol (convención: `description = "<rol> T-<id>"`) y el `parentAgentId`. En el primer `tool.call` de cada agente asocia su `agentId`.
   - El paquete de rol incluye «tu supervisor es `<agentId>`», así que todo agente sabe a quién escalar sin preguntar.
4. Líder.
5. Director.
6. Humano.

**Mensajes tipados:**

| Tipo | Uso | Regla |
|---|---|---|
| `PREGUNTA` | Pedir contexto puntual | Respuesta ≤10 líneas con referencias (`archivo:línea`, `D-id`, `T-id`) |
| `TRASPASO` | Entregar la tarjeta al supervisor | Formato de §3.2 |
| `BLOQUEO` | No se puede avanzar | Al líder; incluye qué se necesita |
| `CAMBIO-CONTRATO` | Un contrato compartido cambió | Lo emite solo el contratista; se difunde a los dependientes y exige decisión |
| `ALERTA` | Watchdog o regla | Del mod al supervisor |

Se mandan **referencias**, nunca transcripts ni archivos completos. Todo mensaje queda en la tabla `mensajes` (hook `session.send`) para auditoría.

## 5. Mapa de paralelismo

| Flujo | ¿Paralelo? | Unidad / restricción |
|---|---|---|
| Génesis · Intención | No | Humano + director |
| Génesis · Arquitectura | Parcial | Investigación por dominio en paralelo; los contratos globales los consolida un solo contratista |
| Génesis · Andamiaje | Sí | Por dominio, tras fijar contratos globales |
| Funcionalidades / hitos | **Sí** | Una célula por funcionalidad en su worktree, si contratos y archivos son disjuntos |
| Dentro de una tarea | No | CONTRATO → ROJO → VERDE → REFACTOR es orden estricto |
| Entre tareas de una célula | Sí (pipeline) | El tester escribe el ROJO de T2 mientras el implementador hace el VERDE de T1; las tareas disjuntas van en paralelo |
| Verificación | Sí | V0–V3 en paralelo entre sí; evaluador y registrador en paralelo |
| Integración | **No** | Cola de merge secuencial (cuello de botella deliberado) |
| Endurecimiento | Sí | Red-team por RD; E2E por flujo |
| Mejora · Verticales | No | Humano |
| Mejora · Observar / diagnosticar | **Sí** | Por cluster, solo lectura |
| Mejora · Plan de ronda | No | Consolidación + gate |
| Mejora · Implementar | Sí | Mini-célula por cluster, archivos disjuntos |
| Mejora · Medir | Sí | Por shards, con tope de máquina y presupuesto |
| Mejora · Comparar / decidir / registrar | No | Determinista (`hz`) |
| Transversales | **Siempre** | Curador, meta-evals / red-team, registrador, informe |

### Coordinación
- **Tablero = DAG en SQLite** (`hz tablero`; tabla `tareas_tdd` con `bloqueada_por`, `owner_agente_id` y `celula`). El director calcula el **frente listo** (tareas sin bloqueos) con `hz tablero listo` y lo asigna según la capacidad.
- **Arriendos de archivos** (tabla `arriendos`: agente, tarea, ruta, expiración):
  - Editar un archivo arrendado por otro se deniega con la remediación «pídelo a `<agente>` por SendMessage».
  - El arriendo se renueva con la actividad y expira si el agente muere.
- **Archivos compartidos** (contratos globales, configuración común): solo los cambia el contratista, con decisión y aviso `CAMBIO-CONTRATO`.

## 6. Enjambre: `/hz enjambre <funcionalidades | clusters>`
1. **Plan.** El director valida los contratos globales y el DAG, corre `hz capacidad` y propone:
   - las células (una por funcionalidad, tantas como permita el hardware);
   - los roles y los modelos que eligió;
   - el costo estimado;
   - los límites.

   El plan pasa por un **gate**: el OK del humano para el gasto en volumen.
2. **Lanzamiento.** Lanza N líderes en background, con `isolation: "worktree"` y `name: lider-<funcionalidad>`. Cada líder lanza sus roles con su relevo supervisado.
3. **Integración.** El integrador consume las células terminadas en orden de dependencias. Ante un conflicto, conversa con los líderes involucrados.
4. **Seguimiento.** El director ve solo las tarjetas de los líderes. Notifica al humano en gate, bloqueo, milestone o fin, y cierra con un informe C-level.

**Tamaño: dinámico, según hardware y viabilidad** (D-0023). No hay tope fijo: si un proyecto grande ataca 12 frentes a la vez y el hardware lo permite, se montan 12 células simultáneas.
- `hz capacidad` mide la RAM libre, los núcleos, el disco y la carga local de cada célula. La inferencia corre en la API; lo que consume la máquina son el proceso de cada agente (**≈45 MB por agente**, medido en S14) y lo que cada célula ejecuta (tests, builds, servidores).
- La fórmula es: `células = min(límite_concurrencia, ⌊(RAM_libre − reserva) / (agentes_por_célula × 45 MB + pico_de_carga_célula)⌋, núcleos_lógicos si la carga es de CPU)`. El `pico_de_carga_célula` se mide en la primera corrida de la suite del proyecto (D-0030).
- Durante el enjambre, `hz capacidad` vuelve a medir. Si la RAM libre baja de la reserva, el director deja de lanzar células nuevas, para evitar que el sistema mate procesos por falta de memoria.
- El límite de concurrencia de Claude Code (`CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`, 20 por defecto) se ajusta cuando el hardware da para más.
- `profundidad`: 3 (director → líder → rol → ayudante). Requiere `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH`; se valida en el spike.
- El presupuesto se reparte por célula.

**Dos motores:**

| Motor | Cuándo | Por qué |
|---|---|---|
| **Enjambre jerárquico** (agéntico) | Por defecto para construir funcionalidades | Flexible, conversa por SendMessage, usa el tablero |
| **Workflow** (determinista) | Pipelines repetitivos y masivos: rondas de Mejora a escala, lotes A/B de lecciones, red-team | El script codifica la cadena de roles y las fases; resultados reproducibles |
