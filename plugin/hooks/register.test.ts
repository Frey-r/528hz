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

type Opciones = { hayHarness?: boolean; salida?: string; hooks?: string }

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
  on('ui.status', () => ({ value: undefined }))
  on('tool.call', () => ({ result: {}, text: o.salida ?? 'salida normal' }))
  on('session.start', (_: unknown, e: { cwd: string }) => ({ cwd: e.cwd }))
}

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
