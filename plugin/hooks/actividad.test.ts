// Contrato: contratos/plugin.md §7. Actividad de los agentes (lógica pura, sin $).
import { expect, test } from 'claude-code/testing'

import { anotar, arbol, cerrarSesion, destinoCorto, glifo, hace, lanzar, lineaEstado, MAX_EVENTOS, podarEventos, PRINCIPAL, registrarTool, reportar, rutaSnapshot, snapshot, type Agente } from './actividad'
import type { Hooks } from './nucleo'

const RAIZ = 'C:/proy/demo'
const H: Hooks = {
  version: 2, generado: '', nivel: 'N2', modo: 'mejora', resumen: [],
  tareas_activas: [{ id: 'T-1', estado: 'rojo', tests_bloqueados: [] }],
  tests_bloqueados: [], protegidos: [], reglas_duras: [], secretos: [],
}

test('un subagente lanzado toma su rol del tipo hz:<rol> o de la descripción', () => {
  let l: Agente[] = []
  l = lanzar(l, { id: 'ag1', tipo: 'hz:implementador', descripcion: 'implementa el parser', padre: '' }, 1000)
  l = lanzar(l, { id: 'ag2', tipo: 'general-purpose', descripcion: 'tester T-3', padre: 'ag1' }, 2000)
  expect(l[0]).toMatchObject({ id: 'ag1', rol: 'implementador', padre: PRINCIPAL, estado: 'lanzado', tools: 0, desde: 1000 })
  expect(l[1]).toMatchObject({ id: 'ag2', rol: 'tester', tarea: 'T-3', padre: 'ag1' })
})

test('cada tool call cuenta; los errores seguidos se reinician con un éxito', () => {
  let l = lanzar([], { id: 'ag1', tipo: 'hz:verificador', descripcion: 'verificador T-2', padre: '' }, 0)
  l = registrarTool(l, 'ag1', { tool: 'Bash', destino: 'go test ./...', error: true }, 10)
  l = registrarTool(l, 'ag1', { tool: 'Bash', destino: 'go test ./...', error: true }, 20)
  expect(l[0]).toMatchObject({ estado: 'activo', tools: 2, errores: 2, seguidos: 2, ultima: 'Bash go test ./...', visto: 20 })
  l = registrarTool(l, 'ag1', { tool: 'Read', destino: 'a.go', error: false }, 30)
  expect(l[0]).toMatchObject({ tools: 3, errores: 2, seguidos: 0 })
  l = registrarTool(l, undefined, { tool: 'Edit', destino: 'b.go', error: false }, 40)
  expect(l.find(a => a.id === PRINCIPAL)).toMatchObject({ rol: 'director', tools: 1 })
})

test('el reporte cierra al agente según cómo terminó', () => {
  let l = lanzar([], { id: 'a', tipo: 'x', descripcion: 'tester T-1', padre: '' }, 0)
  l = lanzar(l, { id: 'b', tipo: 'x', descripcion: 'tester T-2', padre: '' }, 0)
  l = lanzar(l, { id: 'c', tipo: 'x', descripcion: 'tester T-3', padre: '' }, 0)
  l = reportar(l, 'a', 'completed', 5)
  l = reportar(l, 'b', 'failed', 5)
  l = reportar(l, 'c', 'stopped', 5)
  expect(l.map(x => x.estado)).toEqual(['terminado', 'fallido', 'detenido'])
})

test('se conservan las 50 filas terminadas más recientes y todas las vivas', () => {
  let l: Agente[] = []
  for (let i = 0; i < 60; i++) {
    l = lanzar(l, { id: `t${i}`, tipo: 'x', descripcion: 'tester', padre: '' }, i)
    l = reportar(l, `t${i}`, 'completed', i)
  }
  l = lanzar(l, { id: 'vivo', tipo: 'x', descripcion: 'implementador T-9', padre: '' }, 100)
  expect(l.filter(a => a.estado === 'terminado').length).toBe(50)
  expect(l.some(a => a.id === 't0')).toBe(false)
  expect(l.some(a => a.id === 'vivo')).toBe(true)
})

test('el árbol cuelga cada agente de su supervisor, con los vivos primero', () => {
  let l = registrarTool([], undefined, { tool: 'Read', destino: 'x', error: false }, 0)
  l = lanzar(l, { id: 'L1', tipo: 'hz:lider', descripcion: 'lider pagos', padre: '' }, 1)
  l = lanzar(l, { id: 'r1', tipo: 'hz:tester', descripcion: 'tester T-1', padre: 'L1' }, 2)
  l = lanzar(l, { id: 'r2', tipo: 'hz:implementador', descripcion: 'implementador T-1', padre: 'L1' }, 3)
  l = reportar(l, 'r1', 'completed', 4)
  const filas = arbol(l).map(f => `${f.nivel}:${f.agente.id}`)
  expect(filas).toEqual(['0:principal', '1:L1', '2:r2', '2:r1'])
})

test('la línea de estado resume tareas y agentes en ≤80 caracteres', () => {
  expect(lineaEstado(H, [])).toBe('hz · N2 · T-1 ROJO')
  let l = lanzar([], { id: 'a', tipo: 'hz:tester', descripcion: 'tester T-1', padre: '' }, 0)
  l = lanzar(l, { id: 'b', tipo: 'hz:implementador', descripcion: 'implementador T-1', padre: '' }, 0)
  l = registrarTool(l, 'b', { tool: 'Bash', destino: 'x', error: true }, 1)
  expect(lineaEstado(H, l)).toBe('hz · N2 · T-1 ROJO · 2 agentes · 1 con errores')
  const muchas: Hooks = { ...H, tareas_activas: Array.from({ length: 9 }, (_, i) => ({ id: `T-${i + 10}`, estado: 'contrato', tests_bloqueados: [] })) }
  expect(lineaEstado(muchas, l).length).toBeLessThanOrEqual(80)
})

test('el destino corto relativiza rutas y recorta comandos', () => {
  expect(destinoCorto({ file_path: `${RAIZ}/internal/a.go` }, RAIZ)).toBe('internal/a.go')
  expect(destinoCorto({ command: 'go test ./... && go vet ./... && go build -o bin/hz.exe ./cmd/hz' }, RAIZ).length).toBeLessThanOrEqual(40)
  expect(destinoCorto({ pattern: 'TODO' }, RAIZ)).toBe('TODO')
})

test('hace() dice hace cuánto en la unidad legible', () => {
  expect(hace(4_000)).toBe('4 s')
  expect(hace(180_000)).toBe('3 min')
  expect(hace(7_200_000)).toBe('2 h')
})

test('sin rol reconocible, el rol es el tipo del agente', () => {
  expect(lanzar([], { id: 'x', tipo: 'Explore', descripcion: 'buscar archivos', padre: '' }, 0)[0].rol).toBe('Explore')
  expect(lanzar([], { id: 'y', tipo: '', descripcion: 'algo', padre: '' }, 0)[0].rol).toBe('agente')
})

test('la poda descarta las terminadas más viejas por última actividad, no por orden de llegada', () => {
  let l: Agente[] = []
  for (let i = 0; i <= 50; i++) {
    l = lanzar(l, { id: `t${i}`, tipo: 'x', descripcion: 'tester', padre: '' }, 0)
    l = reportar(l, `t${i}`, 'completed', i === 0 ? 1000 : i) // t0 llegó primero pero es la más reciente
  }
  expect(l.some(a => a.id === 't0')).toBe(true)
  expect(l.some(a => a.id === 't1')).toBe(false)
})

test('entre hermanos del mismo estado, primero el que empezó antes', () => {
  let l = lanzar([], { id: 'tarde', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 50)
  l = lanzar(l, { id: 'temprano', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 10)
  expect(arbol(l).map(f => f.agente.id)).toEqual(['temprano', 'tarde'])
})

test('la línea de estado no cuenta a la sesión principal ni a los terminados', () => {
  let l = registrarTool([], undefined, { tool: 'Read', destino: 'x', error: true }, 0)
  l = lanzar(l, { id: 'a', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 0)
  l = reportar(l, 'a', 'completed', 1)
  expect(lineaEstado({ ...H, tareas_activas: [] }, l)).toBe('hz · N2')
  let m = lanzar([], { id: 'b', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 0)
  m = lanzar(m, { id: 'c', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 0)
  m = lanzar(m, { id: 'd', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 0)
  m = registrarTool(m, 'b', { tool: 'Bash', destino: 'x', error: true }, 1)
  expect(lineaEstado({ ...H, tareas_activas: [] }, m)).toBe('hz · N2 · 3 agentes · 1 con errores')
})

test('una ruta vacía no tapa al comando como destino', () => {
  expect(destinoCorto({ file_path: '', command: 'ls -la' }, RAIZ)).toBe('ls -la')
})

test('glifos de estado', () => {
  const base = lanzar([], { id: 'a', tipo: 'hz:tester', descripcion: 'tester', padre: '' }, 0)[0]
  expect(glifo(base)).toBe('○')
  expect(glifo({ ...base, estado: 'activo' })).toBe('●')
  expect(glifo({ ...base, estado: 'activo', seguidos: 2 })).toBe('●')
  expect(glifo({ ...base, estado: 'activo', seguidos: 3 })).toBe('!')
  expect(glifo({ ...base, estado: 'terminado' })).toBe('✓')
  expect(glifo({ ...base, estado: 'fallido' })).toBe('✕')
  expect(glifo({ ...base, estado: 'detenido' })).toBe('■')
})

test('los eventos de cada agente se acotan a los últimos 100 y se podan con su fila', () => {
  let ev = {} as Record<string, [number, number][]>
  for (let i = 0; i < MAX_EVENTOS + 20; i++) ev = anotar(ev, 'a', i, i % 2)
  ev = anotar(ev, 'b', 5, 2)
  expect(ev.a.length).toBe(MAX_EVENTOS)
  expect(ev.a[0]).toEqual([20, 0])
  expect(ev.a.at(-1)).toEqual([MAX_EVENTOS + 19, 1])
  const vivos = lanzar([], { id: 'b', tipo: 'x', descripcion: 'tester', padre: '' }, 0)
  expect(Object.keys(podarEventos(ev, vivos))).toEqual(['b'])
})

// ── Snapshot por sesión (3.8, D-0053) ──

test('al cerrar la sesión la principal termina y los subagentes vivos quedan detenidos', () => {
  let l: Agente[] = []
  l = registrarTool(l, undefined, { tool: 'Read', destino: 'a.go', error: false }, 1000)
  l = lanzar(l, { id: 'ag1', tipo: 'hz:tester', descripcion: 'tester T-1', padre: '' }, 1100)
  l = lanzar(l, { id: 'ag2', tipo: 'hz:implementador', descripcion: 'implementador T-1', padre: '' }, 1200)
  l = reportar(l, 'ag2', 'completed', 1300)
  const c = cerrarSesion(l, 5000)
  expect(c.find(a => a.id === PRINCIPAL)).toMatchObject({ estado: 'terminado', visto: 5000 })
  expect(c.find(a => a.id === 'ag1')).toMatchObject({ estado: 'detenido', visto: 5000 })
  expect(c.find(a => a.id === 'ag2')).toMatchObject({ estado: 'terminado', visto: 1300 })
})

test('el snapshot de una sesión va a su propio archivo y lleva su id', () => {
  expect(rutaSnapshot(RAIZ, 'abc-123')).toBe(`${RAIZ}/.harness/cache/actividad/abc-123.json`)
  expect(rutaSnapshot(RAIZ, '../../fuera')).toBe(`${RAIZ}/.harness/cache/actividad/______fuera.json`)
  const l = lanzar([], { id: 'ag1', tipo: 'hz:tester', descripcion: '', padre: '' }, 1000)
  const s = snapshot('abc-123', l, { ag1: [[1000, 2]], fantasma: [[1, 0]] }, true, '2026-11-21T12:00:00.000Z')
  expect(s).toEqual({ version: 2, sesion: 'abc-123', generado: '2026-11-21T12:00:00.000Z', terminada: true, agentes: l, eventos: { ag1: [[1000, 2]] } })
})
