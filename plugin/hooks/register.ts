// Hooks del harness 528hz (contratos/plugin.md §3). Nunca lanza `hz`: lee .harness/cache/hooks.json (D-0034).
import type { Register } from 'claude-code'

import { estadoCorto, evaluarEntrada, firmas, lineaSupervisor, retener, separador, tareaDeDescripcion, Vigia, type Hooks } from './nucleo'

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

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    raices.clear()
    estados.clear()
    equipo.clear()
    vigia.reiniciar()
    const raiz = await buscarRaiz($, e.cwd)
    raices.set(separador(e.cwd), raiz)
    const est = raiz ? await estadoDe($, raiz) : undefined
    if (est) $.ui.status(estadoCorto(est.hooks))
    return next(e)
  })

  // El puntero de inicio de sesión NO va aquí: el plugin interno cc-plugin-sec-default salta prompt.context y
  // classic.SessionStart para los mods. Lo entrega el hook de comando de hooks.json (`hz puntero`), D-0036.

  // Cada subagente sabe a quién escalar desde su primera línea (D-0027).
  on('agent.spawn', async ($, e: any, next) => {
    const raiz = await raizActual($)
    if (!raiz || !(await estadoDe($, raiz))) return next(e)
    return next({ ...e, prompt: `${lineaSupervisor(e.parentAgentId)}\n\n${e.prompt}` })
  })

  on('tool.call', async ($, e: any, next) => {
    const raiz = await raizActual($)
    const est = raiz ? await estadoDe($, raiz) : undefined
    ultimo = est
    if (!raiz || !est) return next(e)
    const agente: string | undefined = e.agentId
    if (agente && !equipo.has(agente)) await registrarEnEquipo($, raiz, agente)
    if (!activo(est.hooks)) return next(e)
    const tarea = agente ? tareaDeDescripcion(equipo.get(agente)?.descripcion ?? '') : undefined
    const motivo = evaluarEntrada(est.hooks, String(e.tool), e, raiz, agente, tarea)
    if (motivo) {
      await vigilar($, agente, true)
      return { deny: motivo }
    }
    const r: any = await next(e)
    await vigilar($, agente, r?.isError === true)
    if (est.firmas.length && r?.deny === undefined && typeof r?.text === 'string' && retener(r.text, est.firmas)) {
      return { deny: RETENIDO }
    }
    return r
  }).catch(($, e, next) => {
    // Fail-closed solo con el harness activo en N2+; sin harness, un fallo nunca bloquea (no romper la autonomía).
    if (!activo(ultimo?.hooks)) return next(e)
    if (next.called && !ultimo?.firmas.length) return next(e)
    return { deny: 'hz: el guardia del harness falló; por seguridad la acción se deniega (fail-closed). Corre `hz exportar` y reintenta.' }
  })
}
