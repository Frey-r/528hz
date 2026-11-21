// Actividad de los agentes (contratos/plugin.md §7): lógica pura, sin $. La guarda el mod en $.state,
// la dibuja el pane hz-panel y la escribe el snapshot .harness/cache/actividad.json.
import type { Agente, EstadoAgente as Estado } from '../types'
import { relativizar, tareaDeDescripcion, type Hooks } from './nucleo'

export type { Agente }

export const PRINCIPAL = 'principal'

export type Lanzamiento = { id: string; tipo: string; descripcion: string; padre: string }
export type Llamada = { tool: string; destino: string; error: boolean }

const ROLES = new Set(['director', 'lider', 'contratista', 'tester', 'implementador', 'verificador', 'evaluador',
  'registrador', 'integrador', 'curador', 'diagnosticador', 'medidor', 'oraculo'])
const MAX_TERMINADAS = 50
const MAX_DESTINO = 40

export const vivo = (a: Agente) => a.estado === 'lanzado' || a.estado === 'activo'

/** El rol sale del tipo `hz:<rol>`; si no, de la primera palabra de la descripción («tester T-3»); si no, el tipo. */
function rolDe(tipo: string, descripcion: string): string {
  if (tipo.startsWith('hz:')) return tipo.slice(3)
  const primera = descripcion.trim().split(/\s+/)[0]?.toLowerCase() ?? ''
  return ROLES.has(primera) ? primera : tipo || 'agente'
}

function nuevo(id: string, ahora: number, campos: Partial<Agente>): Agente {
  return { id, rol: 'agente', descripcion: '', padre: PRINCIPAL, tarea: '', estado: 'lanzado', tools: 0, errores: 0,
    seguidos: 0, ultima: '', desde: ahora, visto: ahora, ...campos }
}

/** Reemplaza la fila del id (o la agrega al final) y poda las terminadas más viejas. */
function conFila(lista: Agente[], fila: Agente): Agente[] {
  const i = lista.findIndex(a => a.id === fila.id)
  const out = i < 0 ? [...lista, fila] : lista.map((a, j) => (j === i ? fila : a))
  return podar(out)
}

function podar(lista: Agente[]): Agente[] {
  const terminadas = lista.filter(a => !vivo(a))
  if (terminadas.length <= MAX_TERMINADAS) return lista
  const fuera = new Set(terminadas.sort((a, b) => a.visto - b.visto).slice(0, terminadas.length - MAX_TERMINADAS).map(a => a.id))
  return lista.filter(a => !fuera.has(a.id))
}

export function lanzar(lista: Agente[], l: Lanzamiento, ahora: number): Agente[] {
  const previa = lista.find(a => a.id === l.id)
  return conFila(lista, nuevo(l.id, ahora, {
    ...previa,
    rol: rolDe(l.tipo, l.descripcion),
    descripcion: l.descripcion,
    padre: l.padre || PRINCIPAL,
    tarea: tareaDeDescripcion(l.descripcion) ?? '',
    estado: 'lanzado',
    desde: previa?.desde ?? ahora,
    visto: ahora,
  }))
}

/** Cuenta una tool call; sin id es la sesión principal (el director). */
export function registrarTool(lista: Agente[], id: string | undefined, c: Llamada, ahora: number): Agente[] {
  const clave = id || PRINCIPAL
  const previa = lista.find(a => a.id === clave) ??
    nuevo(clave, ahora, clave === PRINCIPAL ? { rol: 'director', padre: '' } : {})
  return conFila(lista, {
    ...previa,
    estado: 'activo',
    tools: previa.tools + 1,
    errores: previa.errores + (c.error ? 1 : 0),
    seguidos: c.error ? previa.seguidos + 1 : 0,
    ultima: c.destino ? `${c.tool} ${c.destino}` : c.tool,
    visto: ahora,
  })
}

const FIN: Record<string, Estado> = { completed: 'terminado', failed: 'fallido', stopped: 'detenido' }

export function reportar(lista: Agente[], id: string, status: string, ahora: number): Agente[] {
  const previa = lista.find(a => a.id === id)
  if (!previa) return lista
  return conFila(lista, { ...previa, estado: FIN[status] ?? 'terminado', visto: ahora })
}

/** Árbol por supervisor: raíces (sin padre conocido) en nivel 0; entre hermanos, los vivos primero. */
export function arbol(lista: Agente[]): { agente: Agente; nivel: number }[] {
  const ids = new Set(lista.map(a => a.id))
  const hijos = new Map<string, Agente[]>()
  const raices: Agente[] = []
  for (const a of lista) {
    if (a.padre && ids.has(a.padre) && a.padre !== a.id) {
      hijos.set(a.padre, [...(hijos.get(a.padre) ?? []), a])
    } else {
      raices.push(a)
    }
  }
  const orden = (xs: Agente[]) => [...xs].sort((a, b) => Number(vivo(b)) - Number(vivo(a)) || a.desde - b.desde)
  const out: { agente: Agente; nivel: number }[] = []
  const visitar = (a: Agente, nivel: number) => {
    out.push({ agente: a, nivel })
    for (const h of orden(hijos.get(a.id) ?? [])) visitar(h, nivel + 1)
  }
  for (const r of orden(raices)) visitar(r, 0)
  return out
}

const MAX_ESTADO = 80

/** `hz · N2 · T-1 ROJO · 2 agentes · 1 con errores`: las partes vacías no aparecen. */
export function lineaEstado(h: Hooks, lista: Agente[]): string {
  const subagentes = lista.filter(a => a.id !== PRINCIPAL && vivo(a))
  const conErrores = subagentes.filter(a => a.seguidos > 0).length
  const cola: string[] = []
  if (subagentes.length) cola.push(`${subagentes.length} agente${subagentes.length === 1 ? '' : 's'}`)
  if (conErrores) cola.push(`${conErrores} con errores`)
  const tareas = h.tareas_activas.map(t => `${t.id} ${t.estado.toUpperCase()}`)
  for (let n = Math.min(3, tareas.length); n >= 0; n--) {
    const resto = tareas.length - n
    const parteTareas = n ? tareas.slice(0, n).join(', ') + (resto ? ` +${resto}` : '') : ''
    const linea = ['hz', h.nivel, parteTareas, ...cola].filter(Boolean).join(' · ')
    if (linea.length <= MAX_ESTADO) return linea
  }
  return ['hz', h.nivel, ...cola].join(' · ').slice(0, MAX_ESTADO)
}

const recortar = (s: string, n: number) => (s.length <= n ? s : s.slice(0, n - 1) + '…')

/** Destino corto de una tool call: ruta relativa a la raíz, o el comando/patrón recortado. */
export function destinoCorto(e: any, raiz: string): string {
  const ruta = e?.file_path ?? e?.notebook_path ?? e?.path
  if (typeof ruta === 'string' && ruta) return recortar(relativizar(ruta, raiz) ?? ruta, MAX_DESTINO)
  for (const campo of ['command', 'pattern', 'description', 'url', 'query', 'skill']) {
    const v = e?.[campo]
    if (typeof v === 'string' && v) return recortar(v.replace(/\s+/g, ' ').trim(), MAX_DESTINO)
  }
  return ''
}

export function hace(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000))
  if (s < 60) return `${s} s`
  if (s < 3600) return `${Math.floor(s / 60)} min`
  return `${Math.floor(s / 3600)} h`
}

/** Glifo de estado para el pane (con texto alrededor: nunca solo color). */
export function glifo(a: Agente): string {
  if (a.estado === 'fallido') return '✕'
  if (a.estado === 'detenido') return '■'
  if (a.estado === 'terminado') return '✓'
  if (a.seguidos >= 3) return '!'
  return a.estado === 'activo' ? '●' : '○'
}

// Eventos para los carriles del dashboard (contratos/plugin.md §7, D-0050): [t_ms, tipo] por agente.
export type Eventos = Record<string, [number, number][]>
export const MAX_EVENTOS = 100
export const EV_TOOL = 0
export const EV_ERROR = 1
export const EV_LANZADO = 2
export const EV_FIN = 3

/** Agrega un evento al agente y conserva solo los últimos MAX_EVENTOS. */
export function anotar(ev: Eventos, id: string, t: number, tipo: number): Eventos {
  const lista = [...(ev[id] ?? []), [t, tipo] as [number, number]]
  return { ...ev, [id]: lista.length > MAX_EVENTOS ? lista.slice(lista.length - MAX_EVENTOS) : lista }
}

/** Al podar una fila se van también sus eventos. */
export function podarEventos(ev: Eventos, lista: Agente[]): Eventos {
  const ids = new Set(lista.map(a => a.id))
  return Object.fromEntries(Object.entries(ev).filter(([id]) => ids.has(id)))
}
