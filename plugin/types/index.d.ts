// Contrato de estado del plugin hz (contratos/plugin.md §7): lo que guarda en $.state. Autocontenido.

export type EstadoAgente = 'lanzado' | 'activo' | 'terminado' | 'fallido' | 'detenido'

/** Una fila de actividad: la sesión principal (`principal`) o un subagente. */
export type Agente = {
  id: string
  rol: string
  descripcion: string
  padre: string
  tarea: string
  estado: EstadoAgente
  tools: number
  errores: number
  seguidos: number
  ultima: string
  desde: number
  visto: number
}

/** Últimos eventos por agente para los carriles del dashboard: [t_ms, tipo] (0 tool, 1 error, 2 lanzado, 3 fin). */
export type Eventos = Record<string, [number, number][]>

declare module 'claude-code' {
  interface PluginState {
    hz: { agentes: Shaped<Agente[]>; eventos: Shaped<Eventos> }
  }
}
