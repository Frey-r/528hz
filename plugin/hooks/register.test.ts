// Contrato: contratos/plugin.md §3. Cableado de los hooks con el motor simulado del kit.
import { expect, test } from 'claude-code/testing'

const RAIZ = 'C:/proy/demo'

const HOOKS = JSON.stringify({
  version: 2,
  generado: '',
  nivel: 'N2',
  modo: 'mejora',
  resumen: ['Harness 528hz activo · proyecto demo · nivel N2 · modo mejora.'],
  tareas_activas: [{ id: 'T-1', estado: 'rojo', tests_bloqueados: ['tests/sumar_test.go'] }],
  tests_bloqueados: ['tests/sumar_test.go'],
  protegidos: ['.harness/reglas.yaml', '.harness/log/**'],
  reglas_duras: [],
  secretos: ['.env'],
})

type Opciones = { hayHarness?: boolean; salida?: string; hooks?: string; estados?: string[] }

// motor simula lo que está bajo el plugin: sistema de archivos, sesión y la herramienta misma.
function motor(on: any, o: Opciones = {}) {
  const hay = o.hayHarness ?? true
  on('session.cwd', () => ({ value: RAIZ }))
  on('fs.exists', (_: unknown, e: { path: string }) => ({ value: hay && e.path.replaceAll('\\', '/') === `${RAIZ}/.harness/harness.yaml` }))
  on('fs.stat', () => ({ value: { kind: 'file', size: 10, mtimeMs: 1, isLink: false } }))
  on('fs.read', (_: unknown, e: { path: string }) => {
    const p = e.path.replaceAll('\\', '/')
    if (p.endsWith('.harness/cache/hooks.json')) return { value: o.hooks ?? HOOKS }
    if (p.endsWith('/.env')) return { value: 'TOKEN=canario-SECRETO-4471\n' }
    return { value: '' }
  })
  on('ui.status', (_: unknown, e: any) => {
    o.estados?.push(String(e?.text ?? e ?? ''))
    return { value: undefined }
  })
  on('tool.call', () => ({ result: {}, text: o.salida ?? 'salida normal' }))
  on('session.start', (_: unknown, e: { cwd: string }) => ({ cwd: e.cwd }))
  on('session.id', () => ({ value: SESION }))
  on('session.end', (_: unknown, e: { sessionId: string }) => ({ sessionId: e.sessionId }))
}

const SESION = 'ses-1'

test('deniega editar un test bloqueado', async ($, on) => {
  motor(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.tool.call({ tool: 'Edit', file_path: `${RAIZ}/tests/sumar_test.go`, old_string: 'a', new_string: 'b' })
  expect(r.deny).toContain('RG-2.3')
})

test('retiene el resultado que contiene un secreto (DLP)', async ($, on) => {
  motor(on, { salida: 'TOKEN=canario-SECRETO-4471' })
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.tool.call({ tool: 'Bash', command: 'python leer.py' })
  expect(r.deny).toContain('DLP')
})

test('deja pasar lo permitido sin tocarlo', async ($, on) => {
  motor(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.tool.call({ tool: 'Read', file_path: `${RAIZ}/tests/sumar_test.go` })
  expect(r.deny).toBeUndefined()
  expect(r.text).toBe('salida normal')
})

test('sin harness en el proyecto no interviene', async ($, on) => {
  motor(on, { hayHarness: false, salida: 'TOKEN=canario-SECRETO-4471' })
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.tool.call({ tool: 'Edit', file_path: `${RAIZ}/tests/sumar_test.go`, old_string: 'a', new_string: 'b' })
  expect(r.deny).toBeUndefined()
})

// ── Orquestación ──

const HOOKS_ARRIENDO = JSON.stringify({
  ...JSON.parse(HOOKS),
  arriendos: [{ ruta: 'app/cobro.go', agente: 'ag1', tarea: 'T-7', expira: new Date(Date.now() + 3_600_000).toISOString() }],
})

test('deniega a un subagente escribir un archivo arrendado por otro', async ($, on) => {
  motor(on, { hooks: HOOKS_ARRIENDO })
  on('agent.list', () => ({ value: [] }))
  on('fs.write', () => ({ value: undefined }))
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.tool.call({ tool: 'Edit', file_path: `${RAIZ}/app/cobro.go`, old_string: 'a', new_string: 'b', agentId: 'ag2' })
  expect(r.deny).toContain('ag1')
})

test('al lanzar un subagente le indica su supervisor', async ($, on) => {
  motor(on)
  let prompt = ''
  on('agent.spawn', (_: unknown, e: { prompt: string }) => {
    prompt = e.prompt
    return { model: 'sonnet' }
  })
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'implementa T-7', description: 'implementador T-7', subagentType: 'general-purpose', parentAgentId: 'lider9' } as any)
  expect(prompt).toContain('lider9')
  expect(prompt).toContain('implementa T-7')
})

test('registra al subagente en el equipo en su primer tool call', async ($, on) => {
  motor(on)
  const escrito: string[] = []
  // AgentInfo parcial: el hook solo lee id, type, description y parentId.
  on('agent.list', () => ({ value: [{ id: 'ag2', type: 'general-purpose', description: 'implementador T-7', parentId: 'lider9' }] as any }))
  on('fs.write', (_: unknown, e: { path: string; text: string }) => {
    if (e.path.replaceAll('\\', '/').endsWith('.harness/cache/equipo.json')) escrito.push(e.text)
    return { value: undefined }
  })
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.tool.call({ tool: 'Read', file_path: `${RAIZ}/README.md`, agentId: 'ag2' })
  expect(escrito.at(-1)).toContain('ag2')
  expect(escrito.at(-1)).toContain('lider9')
})

test('avisa una sola vez por agente la lección que aplica al archivo que toca', async ($, on) => {
  const hooks = JSON.stringify({ ...JSON.parse(HOOKS), lecciones: [{ id: 'L-1', skill: 'hz-leccion-escape', ambito: '**/*.ts', descripcion: 'usa Edit para regex' }] })
  motor(on, { hooks })
  on('agent.list', () => ({ value: [] }))
  on('fs.write', () => ({ value: undefined }))
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const primera: any = await $.tool.call({ tool: 'Read', file_path: `${RAIZ}/src/a.ts`, agentId: 'ag1' })
  expect((primera.context ?? []).join(' ')).toContain('hz-leccion-escape')
  const segunda: any = await $.tool.call({ tool: 'Read', file_path: `${RAIZ}/src/b.ts`, agentId: 'ag1' })
  expect((segunda.context ?? []).join(' ')).not.toContain('hz-leccion-escape')
  const otraRuta: any = await $.tool.call({ tool: 'Read', file_path: `${RAIZ}/src/c.go`, agentId: 'ag2' })
  expect((otraRuta.context ?? []).join(' ')).not.toContain('hz-leccion-escape')
})

test('frena el lanzamiento de subagentes cuando una misión activa se quedó sin presupuesto', async ($, on) => {
  const hooks = JSON.stringify({ ...JSON.parse(HOOKS), misiones: [{ id: 'm1', gasto: 1.5, usd_max: 1, sin_presupuesto: true }] })
  motor(on, { hooks })
  let lanzado = false
  on('agent.spawn', () => {
    lanzado = true
    return { model: 'sonnet' }
  })
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: 'implementador T-1', subagentType: 'general-purpose' } as any)
  expect(r.deny).toContain('presupuesto')
  expect(lanzado).toBe(false)
})

// ───────────── Observabilidad (contratos/plugin.md §7) ─────────────

const PANEL = {
  plugin: 'hz',
  component: 'Pane',
  requestId: 'hz-panel',
  props: { title: 'hz', isFocused: false, bodyColumns: 60, placement: 'dock', scroll: { offset: 0, bodyRows: 30 }, view: {} },
} as const

function observar(on: any, o: Opciones = {}) {
  const estados: string[] = []
  motor(on, { ...o, estados })
  const snapshots: string[] = []
  const rutas: string[] = []
  on('fs.write', (_: unknown, e: { path: string; text: string }) => {
    if (e.path.replaceAll('\\', '/').includes('.harness/cache/actividad/')) {
      snapshots.push(e.text)
      rutas.push(e.path.replaceAll('\\', '/'))
    }
    return { value: undefined }
  })
  const abiertos: string[] = []
  on('command.register', () => ({ value: { command: 'hz-panel' } }))
  on('ui.open', (_: unknown, e: { id: string }) => {
    abiertos.push(e.id)
    return { value: { isPlaced: true } }
  })
  on('agent.list', () => ({ value: [] }))
  on('agent.spawn', () => ({ model: 'sonnet', agentId: 'ag7' }))
  on('agent.report', (_: unknown, e: { text: string }) => ({ text: e.text }))
  return { snapshots, rutas, estados, abiertos }
}

test('lanzar y terminar un subagente queda en el snapshot de su sesión', async ($, on) => {
  const { snapshots, rutas } = observar(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: 'implementador T-1', subagentType: 'hz:implementador' } as any)
  const lanzado = JSON.parse(snapshots.at(-1) ?? '{}')
  expect(rutas.at(-1)).toBe(`${RAIZ}/.harness/cache/actividad/${SESION}.json`)
  expect(lanzado).toMatchObject({ version: 2, sesion: SESION, terminada: false })
  expect(lanzado.agentes.find((a: any) => a.id === 'ag7')).toMatchObject({ rol: 'implementador', tarea: 'T-1', estado: 'lanzado' })
  await $.agent.report({ agentId: 'ag7', subagentType: 'hz:implementador', status: 'completed', text: 'listo' } as any)
  const terminado = JSON.parse(snapshots.at(-1) ?? '{}')
  expect(terminado.agentes.find((a: any) => a.id === 'ag7').estado).toBe('terminado')
})

test('la línea de estado cuenta los agentes vivos', async ($, on) => {
  const { estados } = observar(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: 'tester T-1', subagentType: 'hz:tester' } as any)
  expect(estados.at(-1)).toContain('1 agente')
})

test('el pane hz-panel muestra el árbol de agentes con su rol y su tarea', async ($, on) => {
  observar(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: 'implementador T-1', subagentType: 'hz:implementador' } as any)
  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount({ ...PANEL, surface })
    expect((await ui.find({ type: 'Text', text: /implementador/ }))?.text).toContain('T-1')
    expect(await ui.find({ type: 'Text', text: /T-1 ROJO/ })).toBeTruthy()
    await ui.unmount()
  }
})

test('el comando /hz-panel abre el pane', async ($, on) => {
  const { abiertos } = observar(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  const r: any = await $.command.run({ command: 'hz-panel', args: '' } as any)
  expect(abiertos).toContain('hz-panel')
  expect(String(r?.text ?? '')).toContain('hz-panel')
})

test('el snapshot trae los eventos de cada agente para los carriles del dashboard', async ($, on) => {
  const { snapshots } = observar(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: 'tester T-1', subagentType: 'hz:tester' } as any)
  await $.agent.report({ agentId: 'ag7', subagentType: 'hz:tester', status: 'completed', text: 'listo' } as any)
  const snap = JSON.parse(snapshots.at(-1) ?? '{}')
  expect(snap.eventos.ag7.map((e: number[]) => e[1])).toEqual([2, 3])
})

test('al terminar la sesión su snapshot queda cerrado y los subagentes vivos, detenidos', async ($, on) => {
  const { snapshots } = observar(on)
  await $.session.start({ cwd: RAIZ, surface: 'terminal', isInteractive: true })
  await $.tool.call({ tool: 'Read', file_path: `${RAIZ}/README.md` })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: 'tester T-1', subagentType: 'hz:tester' } as any)
  await $.session.end({ reason: 'prompt_input_exit', sessionId: SESION, resume: {} } as any)
  const fin = JSON.parse(snapshots.at(-1) ?? '{}')
  expect(fin).toMatchObject({ sesion: SESION, terminada: true })
  expect(fin.agentes.find((a: any) => a.id === 'principal').estado).toBe('terminado')
  expect(fin.agentes.find((a: any) => a.id === 'ag7').estado).toBe('detenido')
  expect(fin.eventos.ag7.map((e: number[]) => e[1])).toEqual([2, 3])
})
