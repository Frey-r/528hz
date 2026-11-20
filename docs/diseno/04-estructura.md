# 04 · Estructura y granularidad orientada a agentes (A)

**Objetivo:** que lo natural para el agente sea seguir la estructura del proyecto. Si el camino estructurado es el más corto, el agente lo toma.

## 1. AGENTS.md raíz: el mapa (≤100 líneas)
Es una tabla de contenidos, no un manual. Secciones fijas:
1. Qué es el proyecto (3 líneas) y su estado.
2. **Mapa de dominios:** dominio → carpeta → una línea de responsabilidad.
3. **Comandos únicos:** `test`, `lint`, `build`, `run` y `verificar`, uno por acción.
4. **Reglas:** una línea por RG y por RD, con su ID.
5. **Cómo consultar:** `hz estado`, `hz decision buscar`, `hz regla ver` y `hz contexto`.
6. **Zonas no editables:** generado, vendor y datos.

`hz check` falla si el archivo supera 100 líneas o si un link apunta a un archivo inexistente.

**Carga nativa (verificado, S1):** Claude Code lee `AGENTS.md` de forma nativa. Regla crítica: **si existe un `CLAUDE.md`, Claude Code lee solo ese**. Por eso el harness:
- **nunca crea `CLAUDE.md`**;
- si el proyecto ya tiene uno, le agrega la línea `@AGENTS.md`, y `hz check` verifica que esté (D-0025).

## 2. AGENTS.md por dominio (≤40 líneas): contexto progresivo
Contiene:
- **responsabilidad**;
- **fronteras**: qué puede importar y qué no;
- **archivo ejemplar** de cada tipo;
- **recetas locales**;
- **trampas conocidas**: enlaces a las lecciones del ámbito.

**Carga:** Claude Code carga de forma nativa el AGENTS.md de una subcarpeta cuando el agente lee un archivo ahí, salvo que esa subcarpeta tenga su propio CLAUDE.md. En sesiones sin carga nativa (headless o directorios sin confianza, S1), el hook `tool.call` lo inyecta como `context` una sola vez por agente (S2). En ambos casos el contexto llega solo cuando se trabaja ahí.

## 3. Recetas y ejemplares
Una receta es el camino exacto para un cambio frecuente:

```
Receta: agregar un endpoint
1. Contrato: contratos/api/<recurso>.yaml (copia el ejemplar contratos/api/usuarios.yaml)
2. Tests: tests/api/test_<recurso>.py (ejemplar: tests/api/test_usuarios.py)
3. Código: app/api/<recurso>.py (ejemplar: app/api/usuarios.py) — usa el cliente compartido de app/clients.py (RG-1.3)
4. Registro: app/api/__init__.py
5. Verificar: `hz verificar --tarea T-…`
```
- En **Adopción** las recetas se extraen del código existente: `hz descubrir` busca el archivo más representativo de cada tipo.
- En **Génesis** las prescribe la plantilla.
- `hz check` detecta recetas que apuntan a archivos que ya no existen (anti-deriva).

## 4. Comando único por acción
Los comandos se declaran en `harness.yaml → comandos` o se delegan al make, npm o just del proyecto. El agente no improvisa comandos: `hz verificar` y los roles usan los declarados.

## 5. Fronteras verificables (fitness functions)
`hz check` valida en V0:
- **dependencias entre módulos**: reglas `desde → no puede importar` en `harness.yaml → fronteras`;
- **tamaño máximo de archivo**: aviso sobre N líneas, configurable, para que los agentes no lean archivos gigantes;
- **convenciones de nombres** y **zonas no editables**. Editar una zona no editable se deniega, con la remediación correspondiente (por ejemplo, «regenera con `make gen`»).

## 6. Granularidad del trabajo

```
Misión ─► Hito (Génesis) | Ronda (Mejora) ─► Tarea ─► Paso
```

| Unidad | Definición | Dueño |
|---|---|---|
| **Misión** | Objetivo largo con contrato de cierre | Director |
| **Hito / Ronda** | Entregable verificable / una iteración de mejora | Director → líderes |
| **Tarea** | Lo que recibe una célula o un rol | Líder |
| **Paso** | Una acción dentro del ciclo TDD | Rol |

**Condiciones de una tarea bien cortada** (las valida `hz tarea crear`):
- Cabe en una ventana de contexto: estimación por los archivos involucrados.
- Tiene **archivos exclusivos**, con arriendo (ver `11-orquestacion.md`).
- Tiene **contrato y tests propios**.
- Lleva un **brief común** (misión, RG/RD aplicables, decisiones del tema) más uno **específico**.
- Tiene un criterio de salida verificable.

## 7. Plantillas de Génesis

| Plantilla | Estructura base |
|---|---|
| servicio/API | `contratos/api`, `app/<dominio>/`, `app/clients.py` (clientes compartidos, RG-1), `tests/` espejo |
| frontend | `contratos/ui` (props/estados), `src/features/<feature>/` (slice vertical), tests colocados |
| librería | `contratos/` (API pública), `src/`, `tests/`, benchmarks |
| agente LLM | `contratos/` (herramientas, estados), `prompts/` versionados, `evals/` (V4), `app/` |
| pipeline de datos | `contratos/esquemas`, `etapas/<etapa>/`, `tests/` con fixtures sintéticos |

Todas traen desde el día 1:
- AGENTS.md raíz y por dominio;
- comando único;
- CI local en verde;
- carpeta `contratos/`;
- un ejemplar por tipo de archivo.
