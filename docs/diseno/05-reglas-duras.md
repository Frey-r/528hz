# 05 · Reglas duras de seguridad (B)

Las **reglas duras (RD)** son las reglas de seguridad del proyecto. **No se traspasan.** Cada proyecto define las suyas, tanto al nacer (Génesis) como al adoptarse o mejorarse (Mejora). Son guardarraíles fuertes que el agente no puede saltar y cuya efectividad se mide.

## 1. Definición por proyecto
- **Génesis:** se definen en la fase Intención y se ajustan en Arquitectura, cuando ya se conocen los sistemas externos y los datos.
- **Mejora:** se definen o revisan en el paso Verticales + Línea base, partiendo de las candidatas que encontró `hz descubrir`.
- **Catálogo sugerido:** `reglas/catalogo-rd.md` sirve de punto de partida. De ahí se eligen y adaptan las reglas; no se impone una matriz genérica.
- Las aprueba el humano y quedan en `.harness/reglas.yaml` + SQLite.

### Ficha de una RD
```yaml
- id: RD-3
  orden: "No escribas en el sistema de tickets de producción; usa el mock `tests/mocks/tickets.py`."
  por_que: "Una escritura accidental llega a clientes reales."
  origen: genesis            # genesis | mejora | incidente
  desde: 2026-11-20
  enforcement: [permisos, hook, githook, ci]
  patrones:                  # qué se considera violación (alimenta hook y red-team)
    herramientas: ["mcp__tickets__crear", "mcp__tickets__actualizar"]
    bash: ["curl .*tickets\\.ejemplo\\.com/api/.*(POST|PUT|PATCH|DELETE)"]
  allowlist: []              # caminos permitidos (p. ej. `hz git push`)
  red_team: [rt-3a, rt-3b, rt-3c]
  estado: vigente            # vigente | inefectiva | modificada | retirada
```

## 2. Enforcement en capas (defensa en profundidad)

*Revisado con el spike S4 (`12-spike.md`). En el red-team de 10 intentos, 9 quedaron bloqueados y 1 se filtró: una transformación arbitraria del secreto.*

| Capa | Mecanismo | Qué cubre (medido) | Qué no cubre |
|---|---|---|---|
| 1 | **Permisos nativos `deny`** en `.claude/settings.json` del proyecto, generados desde `reglas.yaml` | Herramientas Read, Edit y Write; comandos de archivo reconocidos (`cat`, `head`, `tail`, `sed`, `tee`); **redirecciones de Bash** (`>>`) sobre rutas con `Edit` deny. Aplica en todos los modos | Intérpretes (`python -c`, `node -e`) |
| 2 | **Hook `tool.call` sobre la entrada, fail-closed** | Lógica que un patrón no alcanza: ramas, MCP con argumentos, **allowlist** de acciones críticas (p. ej. solo `hz git push`) | Ofuscación sobre denylist: una regex de entrada no es frontera de seguridad |
| 3 | **DLP de salida (hook `tool.call` sobre el resultado)** | Retiene cualquier resultado que contenga un secreto conocido, en crudo, base64 o hex, venga del comando que venga | Transformaciones arbitrarias (invertir, XOR) |
| 4 | **Git hooks versionados** (`.githooks/` + `core.hooksPath`) | Lo que llega a git por cualquier camino | — |
| 5 | **Servidor / CI**: protección de ramas y checks obligatorios. El harness genera la config y el humano la aplica | Única capa fuera del alcance del agente | — |
| 6 | **Autoprotección** | Que el agente no desactive las capas 1 a 4 | — |
| 7 | **Fuera de alcance físico**, solo en perfil `critico` | Secretos fuera del workspace, inyectados en runtime desde el almacén del SO; o **sandbox del SO** (`sandbox.filesystem.denyRead`), disponible solo en WSL2 | — |

**Riesgo residual (Windows nativo):** un agente deliberadamente adversario puede transformar un secreto legible para que el DLP no lo reconozca. Por eso:
- En los perfiles `estandar` y `sensible` bastan las capas 1 a 6: detienen los caminos habituales y las codificaciones comunes.
- En el perfil `critico` es obligatoria la capa 7.
- El red-team guarda el caso «transformación arbitraria» como evidencia de este límite.

**Principios:**
- Para lo crítico se usa **allowlist** en vez de denylist. Ejemplo: solo se permite `hz git push`, que valida la rama, y cualquier `git push` directo se deniega. Así no hay variantes de sintaxis que evadir.
- Toda denegación trae una **remediación**: qué regla aplica, por qué, y cuál es el camino permitido.

**Autoprotección (capa 6)**, siempre activa en N2+:
- Se deniega editar `.harness/reglas.yaml`, `.claude/settings*.json`, `.githooks/`, los hooks y el plugin. Esto se hace con **deny nativo** (`Edit(./.claude/**)`, `Edit(./.harness/reglas.yaml)`, `Edit(./.githooks/**)`), que también cubre las redirecciones de Bash. No alcanza con que `.claude/` sea un directorio protegido: Claude Code le sugiere al modelo «intentar con otras herramientas».
- Se deniegan `git config core.hooksPath`, `--no-verify`, `git push --force` hacia ramas protegidas y los cambios a variables de entorno `CLAUDE_CODE_*`.
- Las RD **solo cambian con un comando que inicia el humano** (`/hz regla agregar|modificar|retirar`). Hay dos garantías, verificadas en el spike S5:
  - el modelo no ve los comandos del plugin entre sus skills;
  - el handler acepta solo `origin.kind === 'composer'`, es decir, que el humano presionó Enter.

## 3. Efectividad medida
Cada RD acumula eventos en `regla_eventos`:

| Evento | Qué significa |
|---|---|
| `bloqueo` | La regla detuvo una acción. Se registra el agente, la tarea y la capa |
| `intento_evasion` | Se detectó una variante de la acción prohibida, por ejemplo por otra herramienta |
| `escape` | **Ocurrió** lo que la regla debía prevenir: un incidente |
| `red_team` | Resultado de un caso red-team: bloqueado / pasó |
| `falsa_alarma` | El humano aprobó explícitamente algo que la regla bloqueó |

**Red-team por regla.** Cada RD tiene casos de violación: otra sintaxis, otra herramienta, otro camino, codificaciones, scripts intermedios. `hz redteam` los corre en un sandbox:
- en cada milestone;
- al cambiar de modelo;
- al modificar cualquier capa.

**Cuando una regla deja de ser efectiva**, por un `escape` o porque un caso red-team pasó:
1. Queda en estado `inefectiva`, con **cuándo y por qué** (qué camino la evadió).
2. Se abre una alerta al humano: un gate que bloquea el cierre de hito o ronda.
3. Se registra el error y el caso nuevo pasa al red-team permanente.
4. La corrección (otra capa o pasar a allowlist) entra con decisión y vuelve a pasar el red-team.

**Cambios a una RD:** solo los hace el humano. Cada versión queda en SQLite con cuándo, qué, por qué y la decisión asociada. `hz regla historial RD-3` muestra la evolución y su efectividad en cada versión.

## 4. Lo que no está aquí
- **La PII.** Viene de fábrica en las normas de Claude (D-0003).
- **Los gates de gasto** (LLM en volumen, presupuesto). Son controles operativos de la misión (`08-misiones.md`), no reglas de seguridad, aunque usan el mismo mecanismo de gate.
