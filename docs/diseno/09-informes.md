# 09 · Informes (F)

## Norma
- **Todo informe es C-level salvo que se pida uno técnico** (`--tecnico`).
- **Se comunica de forma visual:** si un gráfico representa mejor la información y ahorra un párrafo, va el gráfico.
- **Los números salen del motor SQLite**, nunca del modelo.

## `/hz informe`
Publica un **Artifact privado y versionado**: la misma URL se actualiza en cada milestone. Se construye con las skills `dataviz` y `artifact-design` y con el design system de `dash_example.jpg`:
- fondo oscuro verde-azulado;
- paneles glass;
- acento menta/teal;
- KPI grandes;
- chips de estado.

### Contenido C-level (modo Mejora)
| # | Bloque | Forma visual |
|---|---|---|
| 1 | KPIs por vertical: actual vs. línea base vs. meta, y delta desde el último milestone | Tiles con número grande + barra a la meta |
| 2 | Evolución de cada vertical | Small multiples, una por vertical: rondas como puntos tenues, milestones como marcadores etiquetados, línea de meta y banda de ruido |
| 3 | Comparación entre milestones | Dumbbell o slope chart por vertical (M1 → M2 → M3) |
| 4 | Qué cambió en cada milestone | Una línea por milestone, con link a su decisión |
| 5 | Esfuerzo y costo | Rondas, USD y tiempo por milestone (barras) |

### Modo Génesis: sin informe (D-0045)
El informe C-level es de **Mejora continua**: mide la evolución de verticales por rondas. En Génesis no hay verticales que medir; el avance se sigue con `hz estado`, el tablero y el contrato de cierre (`hz mision cerrar`).

### Versión técnica (`--tecnico`)
Agrega:
- tablas por ronda;
- taxonomía de errores;
- estado de las lecciones (activas, reescritas, eliminadas);
- meta-evals de los verificadores (detección y falsas alarmas);
- efectividad de las RD;
- detalle del holdout.

## Notificaciones
Una PushNotification breve por milestone, con el link al informe.

## Estado de implementación (3.5, D-0044)
- `hz informe <m> --json` es el contrato de datos; `hz informe <m> --html <archivo>` genera la página autocontenida (Go `html/template` + SVG calculado en Go, sin JS) que la skill `hz:informe` publica como Artifact en la misma URL.
- La página cubre el contenido C-level de **Mejora**: KPIs, small multiples con milestones, meta y banda de ruido, línea base → último milestone, milestones, gasto y tabla.
- **Pendiente:** el flag `--tecnico` en el CLI. Mientras tanto, la skill arma la versión técnica con `hz compare --json`, `hz error lista`, `hz leccion lista` y `hz redteam estado`.
