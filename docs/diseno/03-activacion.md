# 03 · Activación por proyecto y niveles

El harness es **opt-in por proyecto**: sin `.harness/harness.yaml` (o con `enabled: false`) todos los hooks son no-op.

## `/hz activar`
Detecta el caso y ofrece el camino.

| Caso | Detección | Camino |
|---|---|---|
| **Génesis** | Directorio vacío o sin código | Elegir plantilla (servicio/API, frontend, librería, agente LLM, pipeline de datos) → misión Génesis |
| **Adopción** | Repo con código existente | `hz descubrir` → propuesta → aprobación humana → nivel elegido |

### Adopción (`hz descubrir`)
Lanza agentes Explore en paralelo, de solo lectura, que infieren:
- **Estructura:** dominios y módulos, fronteras de facto (grafo de imports), archivos ejemplares por tipo (endpoint, modelo, test…), zonas generadas, vendor y datos.
- **Comandos:** test, lint, build, run y CI (desde `package.json`, `Makefile`, `pyproject`, `.gitlab-ci.yml`, etc.).
- **Convenciones:** nombres, ubicación de los tests, estilo de commits e idioma.
- **Contexto existente:** `AGENTS.md`, `CLAUDE.md`, `docs/`, specs (OpenSpec u otras) y ADRs.
- **Zonas sensibles:** candidatas a reglas duras (secretos, integraciones externas, migraciones).

Escribe una **propuesta** en `.harness/propuesta/`:
- `AGENTS.md` raíz y por dominio;
- `harness.yaml`;
- candidatas a RD;
- recetas con sus ejemplares.

El humano la revisa y la aprueba total o parcialmente. Reglas de la Adopción:
- **No mueve ni renombra código.** La estructura existente pasa a ser la norma (D-0005).
- Si ya existe un `AGENTS.md`, se respeta: la propuesta es un diff sobre él. Si mide más de 100 líneas, se propone dividirlo en el mapa raíz más archivos por dominio.
- Reestructurar solo ocurre dentro de una misión explícita y con una decisión registrada.

## Niveles (graduales)

| Nivel | Incluye | Enforcement |
|---|---|---|
| **N1 Mapa** | AGENTS.md raíz y por dominio, recetas, `hz check` de estructura, motor de decisiones | Ninguno: solo guía |
| **N2 Normas** | N1 + RG + RD + ciclo TDD + escalera de verificación + ciclo error → lección | Permisos nativos, hooks fail-closed, git hooks |
| **N3 Misiones** | N2 + misiones, verticales, rondas, milestones, presupuesto, orquestación por roles, informes | + gates y circuit breakers |

Se sube de nivel con `/hz activar --nivel N2`, que propone las RD y exige que el humano las apruebe.

## Archivos que agrega a un proyecto
```
<repo>/
  AGENTS.md                 FUENTE PRINCIPAL ≤100 líneas: mapa, comandos únicos, reglas (RG+RD, 1 línea c/u), cómo consultar `hz`
  (nunca CLAUDE.md: anula la lectura de AGENTS.md; si ya existe, se le agrega `@AGENTS.md`)
  <dominio>/AGENTS.md       ≤40 líneas: contexto local, inyectado por hook al trabajar en esa carpeta
  contratos/                interfaces, esquemas, contratos de API, fronteras, presupuestos de recursos (N2+)
  .claude/settings.json     permisos nativos deny generados desde las RD (N2+)
  .claude/skills/hz-leccion-*/SKILL.md   lecciones activas (N2+)
  .githooks/                pre-commit / pre-push (N2+)
  .harness/
    harness.yaml            nivel, modo, comandos, rutas, verticales, idioma, modelos por rol
    reglas.yaml             reglas duras (N2+, protegido: solo el humano lo modifica)
    log/*.jsonl             registro append-only versionado
    harness.db              SQLite (gitignored, reconstruible con `hz reindex`)
    misiones/<id>/          (N3)
```
Las rutas son configurables. En Adopción se respetan las carpetas existentes: por ejemplo, si el proyecto ya tiene `docs/adr/`, las vistas de decisiones van ahí.

## Desactivar
`enabled: false` en `harness.yaml` apaga hooks, gates e inyecciones. Los archivos de contexto (AGENTS.md, contratos) se quedan porque son útiles igual. `hz desactivar --limpiar` propone eliminar lo que agregó el harness.

## Puntero al iniciar sesión
Con un perfil activo, el hook `session.start` agrega como máximo 15 líneas:
- nivel y modo;
- misión activa y fase;
- gates pendientes;
- «retoma con `hz estado`».

No agrega nada más al prompt.
