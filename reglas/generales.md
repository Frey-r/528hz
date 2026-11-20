# Reglas generales (RG)

Estas reglas aplican a **todo proyecto** que active el harness y van redactadas como órdenes. `hz activar` copia una línea por regla en la sección «Reglas» del `AGENTS.md` del proyecto. El texto completo se consulta con `hz regla ver RG-x`.

Si una RG choca con una **regla dura del proyecto (RD)**, prevalece la RD.

---

## RG-1 · Código de producción y eficiencia de recursos

> Todo código que se genera aquí terminará en producción. Hacer más con menos.

- **RG-1.1** Escribe todo código como si fuera a producción hoy. Fuera de los tests no existe código temporal.
- **RG-1.2** Antes de implementar una función, controlador o servicio, estima su costo en CPU, memoria, I/O, red y conexiones. Elige la alternativa más eficiente que cumpla el contrato.
- **RG-1.3** Reutiliza conexiones con un pool o un cliente compartido por proceso: base de datos, HTTP, colas y caché. Nunca abras una conexión por request ni por llamada.
- **RG-1.4** Crea una sola vez los motores y las instancias costosas, al arrancar o de forma perezosa, y compártelos. Esto incluye clientes de SDK, engines de base de datos, modelos, expresiones regulares y plantillas compiladas.
- **RG-1.5** Antes de instanciar, busca si ya existe una instancia en el flujo o en el contenedor de dependencias y úsala. No dupliques instancias dentro de un flujo.
- **RG-1.6** Reutiliza y muta un objeto existente en vez de crear uno desde cero cuando la mutación respete las reglas duras de seguridad. No lo hagas si el objeto se comparte entre hilos, requests o usuarios. Ante la duda, crea uno nuevo.
- **RG-1.7** Libera todo recurso que adquieras (conexiones, archivos, streams, locks) con el mecanismo del lenguaje: context manager, `try/finally`, `defer` o `using`.
- **RG-1.8** Procesa por lotes o en streaming y pagina las consultas. No cargues colecciones completas en memoria.
- **RG-1.9** No repitas consultas ni cálculos idénticos dentro de un flujo. Cachea con límite de tamaño y expiración cuando corresponda.
- **RG-1.10** Las reglas duras de seguridad prevalecen siempre sobre la optimización.
- **RG-1.11** Registra como decisión, con su porqué, toda excepción a RG-1: una instancia adicional, un recurso sin pool, una carga completa en memoria.
- **RG-1.12** Entrega la solución completa y óptima en la primera implementación. No dejes optimizaciones pendientes ni atajos «por ahora».

**Cómo se verifica:**
- Los contratos de cada componente declaran su **presupuesto de recursos**, por ejemplo conexiones por request, memoria máxima y latencia p95.
- Hay **tests de recursos**: comprueban que el número de conexiones no crece con los requests, que no hay instancias duplicadas y que los recursos se liberan.
- Hay **sensores estáticos por stack** (nivel V0) que detectan:
  - un cliente o conexión instanciado dentro de un handler o un loop;
  - una consulta dentro de un loop (N+1);
  - un recurso abierto sin cierre;
  - una carga completa sin paginar.
- La rúbrica del evaluador escéptico incluye RG-1.

---

## RG-2 · Test-driven siempre

- **RG-2.1** Define primero la estructura general como contratos: interfaces y firmas, esquemas de datos, contratos de API, fronteras de módulos, criterios de aceptación y presupuesto de recursos.
- **RG-2.2** Escribe los tests a partir de los contratos antes de escribir código. Confirma que fallan por aserción, no por import ni por sintaxis.
- **RG-2.3** Construye el código para que pase esos tests. No modifiques tests bloqueados para lograr verde.
- **RG-2.4** Refactoriza solo con los tests intactos y en verde.
- **RG-2.5** En modo Mejora, empieza cada mejora con el test o eval que captura el comportamiento deseado, en rojo.
- **RG-2.6** Declara «listo» solo con evidencia ejecutada: el comando, el resultado y el commit.

Detalle y enforcement en `docs/diseno/06-tdd-y-evals.md`.

---

## RG-3 · Toda decisión queda registrada

- **RG-3.1** Registra cada decisión con **cuándo, qué y por qué**, más las alternativas descartadas, quién decidió y el alcance. Usa `hz decision nueva`.
- **RG-3.2** Antes de proponer un enfoque, ejecuta `hz decision buscar <tema>`. No reabras un enfoque descartado sin evidencia nueva.
- **RG-3.3** Registra siempre:
  - las elecciones entre alternativas;
  - los enfoques descartados;
  - las reversiones;
  - las aprobaciones de gates;
  - las excepciones a RG o RD;
  - los cambios de normas, métricas o contratos.
- **RG-3.4** Enlaza el código con su decisión mediante el trailer de commit `Decision: D-NNNN`.

Detalle en `docs/diseno/07-motor.md`.

---

## RG-4 en adelante
*Las define el usuario.* Cada RG nueva entra con una decisión (RG-3) y su forma de verificarse.
