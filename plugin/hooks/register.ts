// Hooks del harness 528hz (contratos/plugin.md §3). Nunca lanza `hz`: lee .harness/cache/hooks.json (D-0034).
import type { Register } from 'claude-code'

import { estadoCorto, evaluarEntrada, firmas, retener, separador, type Hooks } from './nucleo'

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

const RETENIDO =
  'Regla dura de secretos (DLP del harness 528hz): el resultado contenía un valor secreto y fue retenido. ' +
  'No lo imprimas, copies ni transformes; usa las variables que la app ya carga.'

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    raices.clear()
    estados.clear()
    const raiz = await buscarRaiz($, e.cwd)
    raices.set(separador(e.cwd), raiz)
    const est = raiz ? await estadoDe($, raiz) : undefined
    if (est) $.ui.status(estadoCorto(est.hooks))
    return next(e)
  })

  // El puntero de inicio de sesión NO va aquí: el plugin interno cc-plugin-sec-default salta prompt.context y
  // classic.SessionStart para los mods. Lo entrega el hook de comando de hooks.json (`hz puntero`), D-0036.

  on('tool.call', async ($, e: any, next) => {
    const raiz = await raizActual($)
    const est = raiz ? await estadoDe($, raiz) : undefined
    ultimo = est
    if (!raiz || !est || !activo(est.hooks)) return next(e)
    const motivo = evaluarEntrada(est.hooks, String(e.tool), e, raiz)
    if (motivo) return { deny: motivo }
    const r: any = await next(e)
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
