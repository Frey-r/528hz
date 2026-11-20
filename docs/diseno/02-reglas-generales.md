# 02 · Reglas generales (RG)

El texto vigente vive en `reglas/generales.md`. Este documento explica el **sistema** de reglas generales.

## Qué son
Reglas que aplican a **todo proyecto** que active el harness, sin importar su dominio. Van redactadas como órdenes atómicas, y cada una tiene un ID estable (`RG-n.m`) y una forma de verificarse.

| RG | Tema | Se verifica con |
|---|---|---|
| RG-1 | Código de producción y eficiencia de recursos | Presupuesto de recursos en el contrato, tests de recursos, sensores estáticos V0 y rúbrica del evaluador |
| RG-2 | Test-driven siempre | Máquina de estados TDD aplicada por hook (`06-tdd-y-evals.md`) |
| RG-3 | Decisiones registradas (cuándo, qué, por qué) | `hz decision`, trailer de commit, `hz check docs` |
| RG-4+ | *Por definir por el usuario* | — |

## Precedencia
**RD del proyecto > RG > recetas y convenciones del proyecto > preferencias del agente.**

Ejemplo: RG-1.6 pide mutar objetos para ahorrar memoria, pero si una RD prohíbe compartir estado entre requests, gana la RD.

## Cómo llegan al agente sin llenar el contexto
1. El `AGENTS.md` del proyecto lleva una línea por regla, con su ID y la orden resumida.
2. El texto completo se consulta bajo demanda con `hz regla ver RG-1`.
3. Cada rol recibe solo las RG que le tocan. RG-1 completa va al contratista, al implementador, al verificador y al evaluador; no va al registrador (ver `11-orquestacion.md`).
4. Cuando un sensor detecta una violación, la remediación cita la regla exacta. Ejemplo: «RG-1.3: crea el cliente HTTP una vez en `app/clients.py` y reutilízalo».

## Ciclo de vida de una RG
- **Alta o cambio:** solo el mantenedor del harness, con una decisión registrada (RG-3) y su verificador. Una regla sin verificador no entra.
- **Efectividad:** igual que en las RD, cada RG lleva conteo de violaciones detectadas, escapes y falsas alarmas en SQLite (`regla_eventos`).
- **Poda:** al cambiar de modelo se re-evalúa si cada RG sigue aportando. Una regla que ya no previene nada y solo agrega fricción se retira con una decisión.
