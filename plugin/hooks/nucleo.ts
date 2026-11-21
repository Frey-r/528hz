// Lógica pura de los guardias del harness 528hz (contratos/plugin.md §3).
// Sin `$`: se prueba sola y no cuesta nada por tool call. Los datos vienen de hooks.json (lo escribe `hz`).

export type Hooks = {
  version: number
  generado: string
  nivel: string
  modo: string
  resumen: string[]
  tareas_activas: { id: string; estado: string; tests_bloqueados: string[] }[]
  tests_bloqueados: string[]
  protegidos: string[]
  reglas_duras: { id: string; orden: string; remediacion: string; herramientas: string[]; rutas: string[]; bash: string[] }[]
  secretos: string[]
  arriendos?: { ruta: string; agente: string; tarea: string; expira: string }[]
  lecciones?: { id: string; skill: string; ambito: string; descripcion: string }[]
  misiones?: { id: string; gasto: number; usd_max: number; sin_presupuesto: boolean }[]
}

type Regla = Hooks['reglas_duras'][number]

const ESCRIBEN = new Set(['Edit', 'Write', 'MultiEdit', 'NotebookEdit'])
const TOCAN_RUTA = new Set(['Read', 'Edit', 'Write', 'MultiEdit', 'NotebookEdit', 'Grep', 'Glob'])
const SHELLS = new Set(['Bash', 'PowerShell'])

// Operaciones de shell que escriben, mueven o borran (bash y PowerShell). Compilado una vez (RG-1.4).
const ESCRITURA =
  /(^|[^<>&\d])>>?(?![&>])|\btee\b|\b(cp|mv|rm|dd|truncate|install)\s|\bsed\s+(-[a-zA-Z]*i|--in-place)|\b(Set|Add)-Content\b|\bOut-File\b|\b(Remove|Move|Copy|New)-Item\b/i

export const separador = (s: string) => s.replaceAll('\\', '/')
const esAbsoluta = (s: string) => /^[a-zA-Z]:\//.test(s) || s.startsWith('/')
const sinBarraFinal = (s: string) => s.replace(/\/+$/, '')

/** Ruta relativa a la raíz con `/`; dentro de `.claude/worktrees/<x>/` se relativiza al worktree (S10). */
export function relativizar(ruta: string, raiz: string): string | undefined {
  const r = sinBarraFinal(separador(ruta))
  let rel: string
  if (!esAbsoluta(r)) {
    rel = r.replace(/^\.\//, '')
  } else {
    const base = sinBarraFinal(separador(raiz))
    const ci = /^[a-zA-Z]:\//.test(base)
    const [a, b] = ci ? [r.toLowerCase(), base.toLowerCase()] : [r, base]
    if (a === b) return ''
    if (!a.startsWith(`${b}/`)) return undefined
    rel = r.slice(base.length + 1)
  }
  const worktree = /^\.claude\/worktrees\/[^/]+\/(.*)$/.exec(rel)
  return worktree ? worktree[1] : rel
}

const segmentos = new Map<string, RegExp>()
function segmento(glob: string): RegExp {
  let re = segmentos.get(glob)
  if (!re) {
    const cuerpo = glob.replace(/[.+^${}()|[\]\\]/g, '\\$&').replaceAll('*', '[^/]*').replaceAll('?', '[^/]')
    re = new RegExp(`^${cuerpo}$`)
    segmentos.set(glob, re)
  }
  return re
}

function coincideSegmentos(p: string[], r: string[]): boolean {
  const [cabeza, ...resto] = p
  if (cabeza === undefined) return r.length === 0
  if (cabeza === '**') {
    for (let i = 0; i <= r.length; i++) if (coincideSegmentos(resto, r.slice(i))) return true
    return false
  }
  const [actual, ...demas] = r
  return actual !== undefined && segmento(cabeza).test(actual) && coincideSegmentos(resto, demas)
}

/** Glob con `*` (dentro de un segmento) y `**` (cero o más segmentos); misma semántica que `hz`. */
export function coincide(patron: string, rel: string): boolean {
  return coincideSegmentos(separador(patron).replace(/^\.\//, '').split('/'), rel.split('/'))
}

const activo = (h: Hooks) => h.nivel === 'N2' || h.nivel === 'N3'

const regexes = new Map<string, RegExp | null>()
function regexDe(fuente: string): RegExp | null {
  if (!regexes.has(fuente)) {
    try {
      regexes.set(fuente, new RegExp(fuente))
    } catch {
      regexes.set(fuente, null) // una regla mal escrita no rompe el guardia; `hz` la valida al exportar
    }
  }
  return regexes.get(fuente) ?? null
}

const mensajeRegla = (r: Regla) => `${r.id} (regla dura del proyecto, inquebrantable): ${r.orden} ${r.remediacion}`.trim()

function mensajeTest(rel: string, tarea?: string) {
  return (
    `RG-2.3 (harness 528hz): ${rel} es un test bloqueado${tarea ? ` por la tarea ${tarea}` : ''} desde su ROJO. ` +
    'No se modifica para lograr verde: implementa el código para que pase. Si el test está mal, registra la decisión ' +
    '(`hz decision nueva`) y pide aprobación antes de tocarlo.'
  )
}

const mensajeProtegido = (rel: string) =>
  `Autoprotección del harness 528hz: ${rel} solo lo modifica el humano o el CLI \`hz\` ` +
  '(reglas duras: /hz regla; decisiones: `hz decision nueva`; TDD: `hz tdd …`).'

function tareaDe(h: Hooks, relN: string): string | undefined {
  return h.tareas_activas.find(t => t.tests_bloqueados.some(x => x.toLowerCase() === relN))?.id
}

/** Quita las comillas y neutraliza los operadores que estaban dentro: entre comillas son datos, no redirecciones. */
function sinOperadoresEntreComillas(cmd: string): string {
  let salida = ''
  let comilla: string | undefined
  for (const ch of cmd) {
    if (comilla) {
      if (ch === comilla) comilla = undefined
      else salida += '<>|;&'.includes(ch) ? ' ' : ch
    } else if (ch === '"' || ch === "'") {
      comilla = ch
    } else {
      salida += ch
    }
  }
  return salida
}

/** `hz` es el camino sancionado: nunca escribe tests bloqueados ni protegidos por su cuenta. */
const esHz = (segmento: string) => /^(\S*[\\/])?hz(\.exe)?(\s|$)/i.test(segmento.trim())

/** Comando de shell que escribe sobre un test bloqueado o un protegido (capa complementaria al deny nativo). */
export function escribeEnBash(h: Hooks, cmd: string): string | undefined {
  for (const segmento of sinOperadoresEntreComillas(cmd).split(/&&|\|\||;|\|/)) {
    if (esHz(segmento) || !ESCRITURA.test(segmento)) continue
    const c = separador(segmento).toLowerCase()
    for (const t of h.tests_bloqueados) {
      if (c.includes(t.toLowerCase())) return mensajeTest(t, tareaDe(h, t.toLowerCase()))
    }
    for (const p of h.protegidos) {
      const literal = (separador(p).split('*')[0] ?? '').toLowerCase()
      if (literal && c.includes(literal)) return mensajeProtegido(literal)
    }
  }
  return undefined
}

/** Decide si una llamada se deniega antes de ejecutarse; devuelve el motivo con su remediación. */
export function evaluarEntrada(
  h: Hooks,
  tool: string,
  input: Record<string, unknown>,
  raiz: string,
  agente?: string,
  tarea?: string,
): string | undefined {
  if (!activo(h)) return undefined
  for (const r of h.reglas_duras) if (r.herramientas.includes(tool)) return mensajeRegla(r)
  if (SHELLS.has(tool)) {
    const cmd = String(input.command ?? '')
    for (const r of h.reglas_duras) if (r.bash.some(b => regexDe(b)?.test(cmd))) return mensajeRegla(r)
    return escribeEnBash(h, cmd)
  }
  const ruta = String(input.file_path ?? input.notebook_path ?? input.path ?? '')
  if (!ruta || !TOCAN_RUTA.has(tool)) return undefined
  const rel = relativizar(ruta, raiz)
  if (rel === undefined) return undefined
  const relN = rel.toLowerCase()
  if (ESCRIBEN.has(tool)) {
    if (h.tests_bloqueados.some(t => t.toLowerCase() === relN)) return mensajeTest(rel, tareaDe(h, relN))
    if (h.protegidos.some(p => coincide(p.toLowerCase(), relN))) return mensajeProtegido(rel)
    const ahora = Date.now()
    const ajeno = (h.arriendos ?? []).find(
      a =>
        a.ruta.toLowerCase() === relN &&
        Date.parse(a.expira) > ahora &&
        a.agente !== agente &&
        a.tarea.toLowerCase() !== (tarea ?? '').toLowerCase(),
    )
    if (ajeno) return mensajeArriendo(rel, ajeno)
  }
  for (const r of h.reglas_duras) if (r.rutas.some(p => coincide(p.toLowerCase(), relN))) return mensajeRegla(r)
  return undefined
}

const B64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
function base64(bytes: Uint8Array): string {
  let o = ''
  for (let i = 0; i < bytes.length; i += 3) {
    const n = ((bytes[i] ?? 0) << 16) | ((bytes[i + 1] ?? 0) << 8) | (bytes[i + 2] ?? 0)
    o += B64.charAt((n >> 18) & 63) + B64.charAt((n >> 12) & 63) + (i + 1 < bytes.length ? B64.charAt((n >> 6) & 63) : '=') + (i + 2 < bytes.length ? B64.charAt(n & 63) : '=')
  }
  return o
}

const codificador = new TextEncoder()

/** Firmas de los valores secretos (≥8 caracteres): crudo, hex y base64 en sus 3 alineaciones. */
export function firmas(texto: string): string[] {
  const salida = new Set<string>()
  for (const linea of texto.split(/\r?\n/)) {
    const l = linea.trim()
    if (!l || l.startsWith('#')) continue
    const i = l.search(/[=:]/)
    const valor = (i >= 0 ? l.slice(i + 1) : l).trim().replace(/^["']|["']$/g, '')
    if (valor.length < 8) continue
    const bytes = codificador.encode(valor)
    const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
    salida.add(valor).add(hex).add(hex.toUpperCase())
    for (let desfase = 0; desfase < 3; desfase++) {
      const conPrefijo = new Uint8Array(desfase + bytes.length)
      conPrefijo.set(bytes, desfase)
      const grupos = Math.floor((desfase + bytes.length) / 3)
      const firma = base64(conPrefijo).slice(desfase === 0 ? 0 : 4, grupos * 4)
      if (firma.length >= 12) salida.add(firma)
    }
  }
  return [...salida]
}

export const retener = (texto: string, firmasSecretas: string[]) => firmasSecretas.some(f => texto.includes(f))

/** Bloque de contexto para el inicio de la conversación (≤15 líneas). */
export const puntero = (h: Hooks) => h.resumen.slice(0, 15).join('\n')

/** Texto de la status line. */
export function estadoCorto(h: Hooks): string {
  const tareas = h.tareas_activas.map(t => `${t.id} ${t.estado.toUpperCase()}`)
  return `hz · ${h.nivel} · ${tareas.length ? tareas.slice(0, 3).join(', ') : 'sin tareas TDD'}`
}

// ── Orquestación (contratos/plugin.md §4) ──

type Arriendo = NonNullable<Hooks['arriendos']>[number]

function mensajeArriendo(rel: string, a: Arriendo): string {
  const vence = new Date(a.expira).toLocaleTimeString('es-CL', { hour: '2-digit', minute: '2-digit' })
  return (
    `Arriendo del harness 528hz: ${rel} lo tiene el agente ${a.agente} (tarea ${a.tarea}) hasta las ${vence}. ` +
    `No lo edites: pídeselo por SendMessage (to: "${a.agente}") o coordina con tu líder.`
  )
}

/** Línea que se antepone al prompt de cada subagente: a quién escalar (D-0027). */
export function lineaSupervisor(padre?: string): string {
  return padre
    ? `Tu supervisor es el agente ${padre}: escálale dudas y bloqueos por SendMessage (to: "${padre}").`
    : 'Tu supervisor es la sesión principal (director): escálale dudas y bloqueos en tu respuesta final.'
}

/** Vigía de errores seguidos por agente: alerta una sola vez al llegar al umbral; un éxito lo rearma. */
export class Vigia {
  private readonly seguidos = new Map<string, number>()
  constructor(private readonly umbral: number) {}

  registrar(agente: string, esError: boolean): boolean {
    if (!esError) {
      this.seguidos.set(agente, 0)
      return false
    }
    const n = (this.seguidos.get(agente) ?? 0) + 1
    this.seguidos.set(agente, n)
    return n === this.umbral
  }

  reiniciar(): void {
    this.seguidos.clear()
  }
}

/** Tarea declarada en la descripción del agente (convención «<rol> T-n …»). */
export function tareaDeDescripcion(descripcion: string): string | undefined {
  const numero = /\bT-(\d+)\b/i.exec(descripcion)?.[1]
  return numero ? `T-${numero}` : undefined
}

// ── Lecciones justo a tiempo (D-0008, D-0021) ──

type LeccionActiva = NonNullable<Hooks['lecciones']>[number]

/** Lecciones activas cuyo ámbito coincide con la ruta (misma semántica de glob que `hz leccion para`). */
export function leccionesPara(h: Hooks, rel: string): LeccionActiva[] {
  const relN = rel.toLowerCase()
  return (h.lecciones ?? []).filter(l => coincide(l.ambito.toLowerCase(), relN))
}

/** Aviso breve: la lección completa vive en su skill; aquí solo se indica que aplica. */
export const avisoLeccion = (l: LeccionActiva) =>
  `Lección del harness ${l.id} aplica a este archivo: ${l.descripcion}. Carga la skill ${l.skill} antes de seguir editando.`

const usd = (v: number) => `US$${v.toFixed(2)}`

/**
 * Circuit breaker de presupuesto (contratos/plugin.md §6): con una misión activa sin presupuesto no se lanzan
 * subagentes; el humano decide si se gasta más.
 */
export function frenoPresupuesto(h: Hooks): string | undefined {
  const m = h.misiones?.find(x => x.sin_presupuesto)
  if (!m) return undefined
  return `Sin presupuesto: la misión ${m.id} gastó ${usd(m.gasto)} de ${usd(m.usd_max)}. ` +
    'Pide al humano ampliar presupuesto.usd_max en su mision.yaml (y luego `hz exportar`) o cierra la misión.'
}
