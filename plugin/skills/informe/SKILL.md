---
name: informe
description: Publica el informe C-level de una misión del harness 528hz como Artifact privado y versionado, con los números del motor. Úsala en cada milestone, al cerrar una misión o cuando el humano pida un informe, un reporte de avance o ver la evolución de las verticales.
---

# Informe C-level de una misión (harness 528hz)

Por defecto el informe es **C-level y visual**: un gráfico reemplaza un párrafo siempre que pueda. Solo es técnico si el humano lo pide.

1. **Datos:** `hz informe <m> --json`. Contiene:
   - KPIs por métrica: `actual`, `linea_base`, `meta`, `cumplida`;
   - `serie` por ronda;
   - `milestones` y `comparacion_milestones`;
   - gasto y tope.

   No inventes ni recalcules números: si falta uno, mídelo (`hz ronda medir`) y vuelve a generar.
2. **Página:** `hz informe <m> --html <scratchpad>/informe-<m>.html` genera una página autocontenida con el design system del harness:
   - glass oscuro con acento teal, en claro y oscuro;
   - KPIs, evolución por ronda con milestones, meta y banda de ruido;
   - la línea base frente al último milestone;
   - la tabla de datos.
3. **Publicación:** publica ese archivo con la herramienta Artifact (privado). Para versionar, **republica en la misma URL** en cada milestone:
   - la primera vez, guarda la URL en `.harness/misiones/<m>/informe-url.txt`;
   - las siguientes, pásala como `url`.
4. **Mensaje al humano:** 2–3 líneas como máximo, con el link.
   - Qué métrica cruzó su meta, cuál retrocede y si se acerca el tope de presupuesto.
   - El resto lo cuenta la página.

**Informe técnico (solo si se pide):** suma a los mismos datos:
- `hz compare <m> R-a R-b --json` por ronda;
- `hz error lista`, `hz leccion lista` y `hz redteam estado`.

Preséntalos como tablas en la misma página o en una sección aparte.
