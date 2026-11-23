// Hooks del harness 528hz (contratos/plugin.md §3–§7). Nunca lanza `hz`: lee .harness/cache/hooks.json (D-0034).
import { atom, read, update } from 'claude-code'
import type { Register } from 'claude-code'

import { anotar, arbol, cerrarSesion, destinoCorto, EV_ERROR, EV_FIN, EV_LANZADO, EV_TOOL, glifo, hace, lanzar, lineaEstado, registrarTool, reportar, rutaSnapshot, snapshot, vivo, PRINCIPAL } from './actividad'
import { evaluarEntrada, firmas, frenoPresupuesto, lineaSupervisor, retener, separador, tareaDeDescripcion, Vigia, avisoLeccion, leccionesPara, relativizar, type Hooks } from './nucleo'

type Estado = { mtime: number; hooks: Hooks; firmas: string[] }

// Caché del proceso: cwd → raíz y raíz → estado. Se invalida en cada session.start.
const raices = new Map<string, string | undefined>()
const estados = new Map<string, Estado>()
let ultimo: Estado | undefined // para el fail-closed del .catch

/** Sube desde `desde` buscando .harness/harness.yaml; un worktree de agente se mapea al repo principal (S10). */
async function buscarRaiz($: any, desde: string): Promise<string | undefined> {
  let dir = separador(desde).replace(/\/+$/, '')
  for (;;) {
    if (await $.fs.exists(`${dir}/.harness/harness.yaml`)) {
      const worktree = /^(.*)\/\.claude\/worktrees\/[^/]+$/.exec(dir)
      if (worktree && (await $.fs.exists(`${worktree[1]}/.harness/harness.yaml`))) return worktree[1]
      return dir
    }
    const padre = dir.replace(/\/[^/]*$/, '')
    if (!padre || padre === dir) return undefined
    dir = padre
  }
}

async function raizActual($: any): Promise<string | undefined> {
  const cwd = separador(await $.session.cwd())
  if (!raices.has(cwd)) raices.set(cwd, await buscarRaiz($, cwd))
  return raices.get(cwd)
}

/** Relee hooks.json solo si cambió su mtime; recalcula las firmas DLP de los secretos declarados. */
async function estadoDe($: any, raiz: string): Promise<Estado | undefined> {
  const ruta = `${raiz}/.harness/cache/hooks.json`
  let mtime: number
  try {
    mtime = (await $.fs.stat(ruta)).mtimeMs
  } catch {
    return undefined
  }
  const previo = estados.get(raiz)
  if (previo && previo.mtime === mtime) return previo
  const hooks = JSON.parse(String(await $.fs.read(ruta))) as Hooks
  const f: string[] = []
  for (const s of hooks.secretos ?? []) {
    try {
      f.push(...firmas(String(await $.fs.read(`${raiz}/${s}`))))
    } catch {
      // un secreto declarado que no existe no aporta firmas
    }
  }
  const nuevo = { mtime, hooks, firmas: f }
  estados.set(raiz, nuevo)
  return nuevo
}

const activo = (h?: Hooks) => h?.nivel === 'N2' || h?.nivel === 'N3'

// Directorio del equipo (agentId → rol y supervisor) y vigía de errores seguidos (contratos/plugin.md §4).
type Miembro = { id: string; tipo: string; descripcion: string; padre: string; visto: string }
const equipo = new Map<string, Miembro>()
const vigia = new Vigia(5)
const avisadas = new Set<string>() // agente|lección ya avisada: una sola vez por agente

/** Contexto justo a tiempo: las lecciones activas del archivo tocado que este agente aún no recibió. */
function avisosPara(h: Hooks, raiz: string, agente: string | undefined, e: any): string[] {
  const ruta = String(e.file_path ?? e.notebook_path ?? e.path ?? '')
  const rel = ruta ? relativizar(ruta, raiz) : undefined
  if (rel === undefined) return []
  const avisos: string[] = []
  for (const l of leccionesPara(h, rel)) {
    const clave = `${agente ?? 'principal'}|${l.id}`
    if (avisadas.has(clave)) continue
    avisadas.add(clave)
    avisos.push(avisoLeccion(l))
  }
  return avisos
}

async function registrarEnEquipo($: any, raiz: string, id: string): Promise<void> {
  const info = ((await $.agent.list()) as any[]).find(a => a.id === id)
  equipo.set(id, { id, tipo: info?.type ?? '?', descripcion: info?.description ?? '', padre: info?.parentId ?? '', visto: new Date().toISOString() })
  await $.fs.write(`${raiz}/.harness/cache/equipo.json`, JSON.stringify([...equipo.values()], null, 2))
}

async function vigilar($: any, agente: string | undefined, esError: boolean): Promise<void> {
  if (!agente || !vigia.registrar(agente, esError)) return
  const m = equipo.get(agente)
  const texto =
    `ALERTA (vigía del harness 528hz): el agente ${agente} (${m?.descripcion || 'sin descripción'}) acumuló 5 errores ` +
    'seguidos. Pídele su estado o deténlo y relanza un agente fresco con la tarjeta de falla (D-0015).'
  try {
    if (m?.padre) await $.session.send({ to: { agentId: m.padre }, text: texto })
    else $.ui.toast(texto)
  } catch {
    // avisar nunca debe romper la llamada del agente
  }
}

const RETENIDO =
  'Regla dura de secretos (DLP del harness 528hz): el resultado contenía un valor secreto y fue retenido. ' +
  'No lo imprimas, copies ni transformes; usa las variables que la app ya carga.'

// ───────────────────────── observabilidad (§7) ─────────────────────────
// Nunca bloquea: todo va en try/catch y un fallo solo deja el panel desactualizado.

const PANEL = 'hz-panel'
const TITULO = 'hz · agentes'
const INTERVALO_SNAPSHOT = 2000
// Cambiar el tag cuando cambien los campos de Agente: descarta lo que guardó una versión anterior.
const agentes = atom({ plugin: 'hz', key: 'agentes' } as const, [], { shape: 'agentes-1' })
const eventos = atom({ plugin: 'hz', key: 'eventos' } as const, {}, { shape: 'eventos-1' })

let raizObservada: string | undefined
let sucio = false
let ultimoEstado = ''
let panelAbierto = false

/** Escribe el snapshot de la sesión (un archivo por sesión: dos sesiones del mismo proyecto no se pisan). */
async function escribirSnapshot($: any, terminada = false, sesion?: string): Promise<void> {
  if (!raizObservada) return
  sucio = false
  // el id se pide en cada escritura: tras un /clear la sesión sigue con otro id y sin session.start
  const id = sesion ?? String(await $.session.id())
  const lista = await read($, agentes)
  await $.fs.write(rutaSnapshot(raizObservada, id),
    JSON.stringify(snapshot(id, lista, await read($, eventos), terminada, new Date().toISOString())))
}

async function refrescarEstado($: any, h: Hooks): Promise<void> {
  const linea = lineaEstado(h, await read($, agentes))
  if (linea === ultimoEstado) return
  ultimoEstado = linea
  $.ui.status(linea)
}

async function observar($: any, h: Hooks | undefined, cambio: (l: any[]) => any[], inmediato: boolean,
  evento: { id: string; tipo: number }): Promise<void> {
  try {
    await update($, agentes, cambio)
    await update($, eventos, ev => anotar(ev, evento.id, Date.now(), evento.tipo))
    if (inmediato) await escribirSnapshot($)
    else sucio = true
    if (h) await refrescarEstado($, h)
  } catch {
    // la observabilidad nunca rompe la acción observada
  }
}

/** Línea de un agente en el árbol, recortada al ancho del pane. */
function lineaAgente(a: any, nivel: number, ahora: number, ancho: number): string {
  const partes = [`${'  '.repeat(nivel)}${glifo(a)} ${a.rol}${a.tarea ? ' ' + a.tarea : ''}`, `${a.tools} tools`]
  if (a.errores) partes.push(`${a.errores} err${a.seguidos >= 3 ? ` (${a.seguidos} seguidos)` : ''}`)
  if (a.ultima) partes.push(a.ultima)
  partes.push(vivo(a) ? `hace ${hace(ahora - a.visto)}` : a.estado)
  const s = partes.join(' · ')
  return s.length <= ancho ? s : s.slice(0, Math.max(1, ancho - 1)) + '…'
}

/** Pane hz-panel: cabecera, misiones con su presupuesto, tareas TDD y el árbol de agentes. */
async function dibujarPanel($: any, e: any) {
  const { Box, Text, Progress } = $.ui.resolve(e)
  const ancho = Math.max(20, Number(e.props?.bodyColumns ?? e.viewport?.columns ?? 60))
  const raiz = raizObservada ?? (await raizActual($))
  const est = raiz ? await estadoDe($, raiz) : undefined
  if (!raiz || !est) return <Text dimColor>Sin harness en este proyecto.</Text>
  const hk = est.hooks
  const lista = await read($, agentes)
  const ahora = Date.now()
  const proyecto = raiz.split('/').pop() ?? raiz
  const tareas = hk.tareas_activas.map(t => `${t.id} ${t.estado.toUpperCase()}`)
  const filas = arbol(lista)
  return (
    <Box flexDirection="column">
      <Text bold>{`${proyecto} · ${hk.nivel} · ${hk.modo}`}</Text>
      {(hk.misiones ?? []).map(m => (
        <Box flexDirection="column">
          <Text color={m.sin_presupuesto ? 'error' : 'text'}>
            {`misión ${m.id} · US$${m.gasto.toFixed(2)}${m.usd_max > 0 ? ` de US$${m.usd_max.toFixed(2)}` : ''}${m.sin_presupuesto ? ' · sin presupuesto' : ''}`}
          </Text>
          {m.usd_max > 0 && <Progress label={`presupuesto ${m.id}`} value={Math.min(1, m.gasto / m.usd_max)} />}
        </Box>
      ))}
      <Text dimColor>{`Tareas TDD: ${tareas.length ? tareas.join(', ') : 'ninguna'} · arriendos: ${(hk.arriendos ?? []).length}`}</Text>
      <Text dimColor>{`Agentes (${lista.filter(a => a.id !== PRINCIPAL && vivo(a)).length} vivos)`}</Text>
      {filas.length === 0 && <Text dimColor>Sin actividad todavía.</Text>}
      {filas.map(({ agente: a, nivel }) => (
        <Text color={a.seguidos >= 3 || a.estado === 'fallido' ? 'error' : 'text'} dimColor={a.estado === 'terminado'}>
          {lineaAgente(a, nivel, ahora, ancho)}
        </Text>
      ))}
    </Box>
  )
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    raices.clear()
    estados.clear()
    equipo.clear()
    vigia.reiniciar()
    avisadas.clear()
    ultimoEstado = ''
    const raiz = await buscarRaiz($, e.cwd)
    raices.set(separador(e.cwd), raiz)
    raizObservada = raiz
    const est = raiz ? await estadoDe($, raiz) : undefined
    if (est) {
      try {
        await refrescarEstado($, est.hooks)
        await $.command.register({ name: PANEL, description: 'Panel del harness 528hz: agentes en vivo, tareas TDD y presupuesto' })
        $.clock.every(INTERVALO_SNAPSHOT, () => {
          if (sucio) void escribirSnapshot($).catch(() => {})
        })
      } catch {
        // sin panel ni snapshot, el harness sigue funcionando
      }
    }
    return next(e)
  })

  // Al terminar la sesión su snapshot queda cerrado: el dashboard deja de contar sus agentes como vivos.
  on('session.end', async ($, e: any, next) => {
    try {
      if (raizObservada) {
        const t = Date.now()
        const vivos = (await read($, agentes)).filter(vivo)
        await update($, eventos, ev => vivos.reduce((acc, a) => anotar(acc, a.id, t, EV_FIN), ev))
        await update($, agentes, lista => cerrarSesion(lista, t))
        await escribirSnapshot($, true, e.sessionId ? String(e.sessionId) : undefined)
      }
    } catch {
      // si falla, el dashboard la marca inactiva a los 30 min
    }
    return next(e)
  })

  // El puntero de inicio de sesión NO va aquí: el plugin interno cc-plugin-sec-default salta prompt.context y
  // classic.SessionStart para los mods. Lo entrega el hook de comando de hooks.json (`hz puntero`), D-0036.

  // Cada subagente sabe a quién escalar desde su primera línea (D-0027); el lanzamiento queda en la actividad.
  on('agent.spawn', async ($, e: any, next) => {
    const raiz = await raizActual($)
    const est = raiz ? await estadoDe($, raiz) : undefined
    if (!raiz || !est) return next(e)
    const freno = frenoPresupuesto(est.hooks)
    if (freno) return { deny: freno }
    const r: any = await next({ ...e, prompt: `${lineaSupervisor(e.parentAgentId)}\n\n${e.prompt}` })
    if (r?.agentId && r?.deny === undefined) {
      const l = { id: String(r.agentId), tipo: String(e.subagentType ?? ''), descripcion: String(e.description ?? ''), padre: String(e.parentAgentId ?? '') }
      await observar($, est.hooks, lista => lanzar(lista, l, Date.now()), true, { id: l.id, tipo: EV_LANZADO })
      if (!panelAbierto) {
        panelAbierto = true
        void Promise.resolve($.ui.open({ id: PANEL, title: TITULO })).catch(() => {})
      }
    }
    return r
  }).catch(($, e, next) => next(e)) // el freno de presupuesto es un circuit breaker, no una regla dura: falla abierto

  on('agent.report', async ($, e: any, next) => {
    const raiz = await raizActual($)
    const est = raiz ? await estadoDe($, raiz) : undefined
    if (raiz && est) {
      await observar($, est.hooks, lista => reportar(lista, String(e.agentId), String(e.status), Date.now()), true,
        { id: String(e.agentId), tipo: EV_FIN })
    }
    return next(e)
  }).catch(($, e, next) => next(e)) // un reporte nunca se pierde por la observabilidad

  on('command.run', { command: PANEL }, async $ => {
    try {
      await $.ui.open({ id: PANEL, title: TITULO })
      panelAbierto = true
      return { text: `Panel ${PANEL} abierto.` }
    } catch (err) {
      return { text: `No se pudo abrir ${PANEL}: ${String(err)}` }
    }
  })

  on('ui.render', { component: 'Pane', requestId: PANEL }, async ($, e: any) => {
    try {
      return await dibujarPanel($, e)
    } catch (err) {
      const { Text } = $.ui.resolve(e)
      return <Text dimColor>{`Panel no disponible: ${String((err as any)?.message ?? err)}`}</Text>
    }
  })

  on('tool.call', async ($, e: any, next) => {
    const raiz = await raizActual($)
    const est = raiz ? await estadoDe($, raiz) : undefined
    ultimo = est
    if (!raiz || !est) return next(e)
    const agente: string | undefined = e.agentId
    if (agente && !equipo.has(agente)) await registrarEnEquipo($, raiz, agente)
    const avisos = avisosPara(est.hooks, raiz, agente, e)
    const contar = (r: any) => {
      const error = r?.isError === true || r?.deny !== undefined
      return observar($, est.hooks, lista => registrarTool(lista, agente, { tool: String(e.tool), destino: destinoCorto(e, raiz), error }, Date.now()),
        false, { id: agente || PRINCIPAL, tipo: error ? EV_ERROR : EV_TOOL })
    }
    if (!activo(est.hooks)) {
      const r: any = await next(e)
      await contar(r)
      return avisos.length && r?.deny === undefined ? { ...r, context: [...(r?.context ?? []), ...avisos] } : r
    }
    const tarea = agente ? tareaDeDescripcion(equipo.get(agente)?.descripcion ?? '') : undefined
    const motivo = evaluarEntrada(est.hooks, String(e.tool), e, raiz, agente, tarea)
    if (motivo) {
      await vigilar($, agente, true)
      await contar({ deny: motivo })
      return { deny: motivo }
    }
    const r: any = await next(e)
    await vigilar($, agente, r?.isError === true)
    await contar(r)
    if (est.firmas.length && r?.deny === undefined && typeof r?.text === 'string' && retener(r.text, est.firmas)) {
      return { deny: RETENIDO }
    }
    return avisos.length && r?.deny === undefined ? { ...r, context: [...(r?.context ?? []), ...avisos] } : r
  }).catch(($, e, next) => {
    // Fail-closed solo con el harness activo en N2+; sin harness, un fallo nunca bloquea (no romper la autonomía).
    if (!activo(ultimo?.hooks)) return next(e)
    if (next.called && !ultimo?.firmas.length) return next(e)
    return { deny: 'hz: el guardia del harness falló; por seguridad la acción se deniega (fail-closed). Corre `hz exportar` y reintenta.' }
  })
}
