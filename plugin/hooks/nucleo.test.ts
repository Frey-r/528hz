// Contrato: contratos/plugin.md §3. Lógica pura de los guardias (sin $).
import { expect, test } from 'claude-code/testing'

import { coincide, escribeEnBash, evaluarEntrada, firmas, puntero, relativizar, retener, type Hooks } from './nucleo'

const RAIZ = 'C:\\proy\\demo'

const H: Hooks = {
  version: 2,
  generado: '',
  nivel: 'N2',
  modo: 'mejora',
  resumen: ['Harness 528hz activo · proyecto demo · nivel N2 · modo mejora.', 'Trabajo test-driven.'],
  tareas_activas: [{ id: 'T-1', estado: 'rojo', tests_bloqueados: ['tests/sumar_test.go'] }],
  tests_bloqueados: ['tests/sumar_test.go'],
  protegidos: ['.harness/reglas.yaml', '.harness/log/**', '.claude/settings.json', '.claude/settings.local.json', '.githooks/**'],
  reglas_duras: [
    { id: 'RD-1', orden: 'No leas secretos.', remediacion: 'Usa las variables cargadas.', herramientas: [], rutas: ['.env', 'config/*.env'], bash: [] },
    { id: 'RD-2', orden: 'No escribas en tickets.', remediacion: 'Usa el mock.', herramientas: ['mcp__tickets__crear'], rutas: [], bash: ['curl .*tickets\\.ejemplo/api'] },
  ],
  secretos: ['.env'],
}

const ruta = (rel: string) => `${RAIZ}\\${rel.replaceAll('/', '\\')}`

test('relativizar normaliza separadores, mayúsculas de la raíz y worktrees', () => {
  expect(relativizar(ruta('tests/sumar_test.go'), RAIZ)).toBe('tests/sumar_test.go')
  expect(relativizar('c:/PROY/demo/tests/sumar_test.go', RAIZ)).toBe('tests/sumar_test.go')
  expect(relativizar(ruta('.claude/worktrees/agent-1/tests/sumar_test.go'), RAIZ)).toBe('tests/sumar_test.go')
  expect(relativizar('tests/sumar_test.go', RAIZ)).toBe('tests/sumar_test.go')
  expect(relativizar('D:\\otro\\x.go', RAIZ)).toBeUndefined()
})

test('coincide con * dentro de un segmento y ** entre segmentos', () => {
  expect(coincide('.harness/log/**', '.harness/log/decisiones.jsonl')).toBe(true)
  expect(coincide('config/*.env', 'config/prod.env')).toBe(true)
  expect(coincide('config/*.env', 'config/sub/prod.env')).toBe(false)
  expect(coincide('**/.env', 'a/b/.env')).toBe(true)
  expect(coincide('.env', 'a/.env')).toBe(false)
})

test('en N1 no hay enforcement', () => {
  expect(evaluarEntrada({ ...H, nivel: 'N1' }, 'Edit', { file_path: ruta('tests/sumar_test.go') }, RAIZ)).toBeUndefined()
})

test('deniega modificar un test bloqueado nombrando la tarea; leerlo está permitido', () => {
  for (const [tool, input] of [
    ['Edit', { file_path: ruta('tests/sumar_test.go') }],
    ['Write', { file_path: ruta('tests/sumar_test.go') }],
    ['MultiEdit', { file_path: ruta('tests/sumar_test.go') }],
  ] as const) {
    const r = evaluarEntrada(H, tool, input, RAIZ)
    expect(r).toContain('RG-2.3')
    expect(r).toContain('T-1')
  }
  expect(evaluarEntrada(H, 'Read', { file_path: ruta('tests/sumar_test.go') }, RAIZ)).toBeUndefined()
})

test('autoprotección: protegidos sí; las lecciones en .claude/skills no', () => {
  expect(evaluarEntrada(H, 'Edit', { file_path: ruta('.harness/reglas.yaml') }, RAIZ)).toMatch(/autoprotecci/i)
  expect(evaluarEntrada(H, 'Write', { file_path: ruta('.harness/log/decisiones.jsonl') }, RAIZ)).toMatch(/autoprotecci/i)
  expect(evaluarEntrada(H, 'Edit', { file_path: ruta('.claude/settings.json') }, RAIZ)).toMatch(/autoprotecci/i)
  expect(evaluarEntrada(H, 'Write', { file_path: ruta('.claude/skills/hz-leccion-x/SKILL.md') }, RAIZ)).toBeUndefined()
})

test('reglas duras por herramienta, por ruta y por bash, con su remediación', () => {
  const herramienta = evaluarEntrada(H, 'mcp__tickets__crear', {}, RAIZ)
  expect(herramienta).toContain('RD-2')
  expect(herramienta).toContain('Usa el mock.')
  expect(evaluarEntrada(H, 'Read', { file_path: ruta('.env') }, RAIZ)).toContain('RD-1')
  expect(evaluarEntrada(H, 'Grep', { pattern: 'x', path: ruta('config/prod.env') }, RAIZ)).toContain('RD-1')
  expect(evaluarEntrada(H, 'Read', { file_path: ruta('src/app.go') }, RAIZ)).toBeUndefined()
  expect(evaluarEntrada(H, 'Bash', { command: 'curl -X POST https://tickets.ejemplo/api/v1' }, RAIZ)).toContain('RD-2')
  expect(evaluarEntrada(H, 'Bash', { command: 'go test ./...' }, RAIZ)).toBeUndefined()
})

test('bash o powershell que escriben sobre protegidos o tests bloqueados', () => {
  expect(escribeEnBash(H, 'echo x >> .harness/log/decisiones.jsonl')).toMatch(/autoprotecci/i)
  expect(escribeEnBash(H, 'sed -i s/5/0/ tests/sumar_test.go')).toContain('RG-2.3')
  expect(escribeEnBash(H, 'Set-Content .harness\\reglas.yaml "x"')).toMatch(/autoprotecci/i)
  expect(escribeEnBash(H, 'cat .harness/reglas.yaml')).toBeUndefined()
  expect(escribeEnBash(H, 'go test ./tests/...')).toBeUndefined()
  expect(evaluarEntrada(H, 'Bash', { command: 'echo y > .githooks/pre-commit' }, RAIZ)).toMatch(/autoprotecci/i)
})

test('DLP: retiene el secreto en crudo, base64 y hex; ignora valores cortos', () => {
  const f = firmas('TOKEN=canario-SECRETO-4471\nCORTO=abc\n')
  expect(retener('la salida dice canario-SECRETO-4471', f)).toBe(true)
  expect(retener('VE9LRU49Y2FuYXJpby1TRUNSRVRPLTQ0NzEK', f)).toBe(true)
  expect(retener('Y2FuYXJpby1TRUNSRVRPLTQ0NzE=', f)).toBe(true)
  expect(retener('63616e6172696f2d5345435245544f2d34343731', f)).toBe(true)
  expect(retener('abc', f)).toBe(false)
  expect(retener('salida normal', f)).toBe(false)
})

test('el puntero resume el estado en a lo más 15 líneas', () => {
  const p = puntero(H)
  expect(p).toContain('Harness 528hz activo')
  expect(p.split('\n').length).toBeLessThanOrEqual(15)
})
