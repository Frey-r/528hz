# 07 · Motor de decisiones y memoria operativa (D)

**Criterio:** disponibilidad **sin llenar el contexto**. Una consulta estructurada en SQLite entrega lo justo con menos tokens que leer markdown (D-0007).

## Arquitectura

```
agentes ──► hz <comando> ──► harness.db (SQLite, índice consultable, gitignored)
                │                    ▲
                └──► log/*.jsonl ────┘  hz reindex (reconstruye)
                     (append-only, versionado en git, mergeable)
```
- **`log/*.jsonl` es la fuente de verdad versionada.** Cada escritura agrega un evento. Esto evita conflictos de merge de un binario y permite auditar.
- **`harness.db` es el índice** para consultar rápido. Si se borra o se corrompe, `hz reindex` lo rehace.
- **Vistas markdown bajo demanda** (`hz vista decisiones > docs/decisiones/INDICE.md`) para humanos y revisiones.
- **Salida de `hz`:** breve por defecto, con `--formato breve|json|md`. La forma breve está pensada para agentes: pocas líneas y con IDs.

## Tablas
DDL completo en `schema/harness.sql`.

| Grupo | Tablas |
|---|---|
| Decisiones | `decisiones` |
| Reglas | `reglas`, `regla_versiones`, `regla_eventos`, `redteam_casos` |
| Errores y aprendizaje | `errores`, `clases_error`, `lecciones`, `evals`, `eval_runs` |
| Verificación | `verificadores`, `tareas_tdd` |
| Misiones | `misiones`, `hitos`, `verticales`, `metricas`, `rondas`, `mediciones`, `milestones`, `gates` |
| Orquestación | `traspasos`, `arriendos`, `mensajes`, `agentes` |

## Ficha de una decisión
| Campo | Contenido |
|---|---|
| `id` | `D-NNNN` |
| **`cuando`** | Timestamp ISO + `mision`, `ronda_o_hito`, `sesion`, `commit` |
| **`que`** | Una frase con la decisión |
| **`por_que`** | Evidencia y restricciones: métricas con referencia a mediciones, errores y decisiones previas |
| `alternativas` | Lista de `{opcion, descartada_por}` |
| `decidio` | `humano` \| `agente` \| `agente_aprobado` |
| `alcance` | Módulos o archivos |
| `reversible` | Booleano |
| `estado` | `vigente` \| `reemplazada` (con `reemplazada_por`) \| `revertida` |
| `etiquetas` | Temas para la búsqueda |

### Cuándo es obligatorio registrar (RG-3)
- Al elegir entre alternativas que un agente futuro podría reabrir: arquitectura, dependencias, estructura.
- **Al descartar un enfoque:** registrar lo intentado y por qué se rechazó evita reabrirlo.
- Al revertir, al aprobar un gate o al hacer una excepción a RG o RD.
- Al cambiar normas, métricas, verticales o contratos.

No se registran las ediciones triviales.

## Comandos clave

| Comando | Para qué | Tamaño de la salida |
|---|---|---|
| `hz decision nueva` | Registrar una decisión (exige todos los campos) | 1 línea (ID) |
| `hz decision buscar <tema>` | **Paso fijo antes de proponer un enfoque** | 3 a 5 líneas: `D-id · qué · estado` |
| `hz decision ver D-0007` | Ficha completa | ≤20 líneas |
| `hz estado` | Retomar tras un corte | ≤40 líneas |
| `hz contexto --rol R --tarea T` | Arma el paquete de un rol | Tope de tokens por rol |
| `hz regla ver RG-1` / `hz regla historial RD-3` | Texto y efectividad de una regla | breve |
| `hz error nuevo` / `hz leccion …` | Ciclo de aprendizaje | breve |

## Enlaces
- **Código ↔ decisión:** trailer de commit `Decision: D-0007`. `hz check docs` verifica que los IDs existan.
- **Decisión ↔ evidencia:** `por_que` referencia mediciones (`M-…`), errores (`E-…`) y rondas (`R-…`).
- **Compound engineering:** `/hz aprender` convierte una corrección del humano en regla, test, receta o lección, y la deja registrada como decisión.

## A futuro
Si exponer las consultas como un servidor MCP resulta más barato en tokens que llamar `hz` por Bash, se agrega como segunda interfaz sobre el mismo motor.
