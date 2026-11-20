-- 528hz · Motor de memoria operativa (SQLite)
-- Índice consultable. La fuente versionada es .harness/log/*.jsonl; `hz reindex` reconstruye esta base.
-- Convención: IDs legibles con prefijo (D-, RD-, E-, L-, T-, R-, M-, G-, TR-, MS-).

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- ───────────────────────── Misiones ─────────────────────────
CREATE TABLE IF NOT EXISTS misiones (
  id            TEXT PRIMARY KEY,
  titulo        TEXT NOT NULL,
  modo          TEXT NOT NULL CHECK (modo IN ('genesis','mejora')),
  objetivo      TEXT NOT NULL,
  estado        TEXT NOT NULL DEFAULT 'activa' CHECK (estado IN ('activa','pausada','cerrada','abortada')),
  fase          TEXT,
  creada_en     TEXT NOT NULL,
  cerrada_en    TEXT
);

CREATE TABLE IF NOT EXISTS hitos (               -- Génesis
  id            TEXT PRIMARY KEY,
  mision_id     TEXT NOT NULL REFERENCES misiones(id),
  titulo        TEXT NOT NULL,
  estado        TEXT NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente','en_curso','verde','cerrado')),
  creado_en     TEXT NOT NULL,
  cerrado_en    TEXT
);

CREATE TABLE IF NOT EXISTS verticales (          -- Mejora
  id            TEXT PRIMARY KEY,
  mision_id     TEXT NOT NULL REFERENCES misiones(id),
  nombre        TEXT NOT NULL,
  peso          REAL NOT NULL CHECK (peso > 0 AND peso <= 1),
  aprobada_en   TEXT
);

CREATE TABLE IF NOT EXISTS metricas (
  id            TEXT PRIMARY KEY,
  vertical_id   TEXT NOT NULL REFERENCES verticales(id),
  nombre        TEXT NOT NULL,
  sensor        TEXT NOT NULL,                   -- comando determinista que emite JSON por ítem
  direccion     TEXT NOT NULL CHECK (direccion IN ('sube','baja')),
  linea_base    REAL,
  meta          REAL,
  piso_ruido    REAL
);

CREATE TABLE IF NOT EXISTS rondas (
  id            TEXT PRIMARY KEY,                -- R-…
  mision_id     TEXT NOT NULL REFERENCES misiones(id),
  numero        INTEGER NOT NULL,
  foco          TEXT,                            -- verticales atacadas (JSON array)
  commit_foto   TEXT,
  veredicto     TEXT CHECK (veredicto IN ('mejora','ruido','regresion')),
  decision_id   TEXT,                            -- mantener / revertir
  costo_usd     REAL,
  iniciada_en   TEXT NOT NULL,
  cerrada_en    TEXT,
  UNIQUE (mision_id, numero)
);

CREATE TABLE IF NOT EXISTS mediciones (
  id            TEXT PRIMARY KEY,                -- M-…
  ronda_id      TEXT NOT NULL REFERENCES rondas(id),
  metrica_id    TEXT NOT NULL REFERENCES metricas(id),
  conjunto      TEXT NOT NULL DEFAULT 'trabajo' CHECK (conjunto IN ('trabajo','holdout','ruido')),
  valor         REAL NOT NULL,
  n_items       INTEGER NOT NULL,
  ic_bajo       REAL,
  ic_alto       REAL,
  resultados    TEXT,                            -- ruta a results.json por ítem
  medido_en     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS milestones (
  id            TEXT PRIMARY KEY,                -- MS-…
  mision_id     TEXT NOT NULL REFERENCES misiones(id),
  ronda_id      TEXT NOT NULL REFERENCES rondas(id),
  promocion     TEXT NOT NULL CHECK (promocion IN ('propuesta_aceptada','manual')),  -- siempre la define el usuario (D-0019)
  resumen       TEXT NOT NULL,                   -- una línea: qué cambió
  decision_id   TEXT,
  creado_en     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS gates (
  id            TEXT PRIMARY KEY,                -- G-…
  mision_id     TEXT REFERENCES misiones(id),
  tipo          TEXT NOT NULL CHECK (tipo IN ('regla_dura','gasto','contrato','enjambre','cierre','otro')),
  motivo        TEXT NOT NULL,
  estado        TEXT NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente','aprobado','rechazado','expirado')),
  solicitado_en TEXT NOT NULL,
  resuelto_en   TEXT,
  resuelto_por  TEXT,
  decision_id   TEXT
);

-- ───────────────────────── Decisiones (RG-3) ─────────────────────────
CREATE TABLE IF NOT EXISTS decisiones (
  id              TEXT PRIMARY KEY,              -- D-NNNN
  cuando          TEXT NOT NULL,                 -- ISO 8601 con zona
  mision_id       TEXT,
  ronda_o_hito    TEXT,
  sesion          TEXT,
  commit_ref      TEXT,
  que             TEXT NOT NULL,
  por_que         TEXT NOT NULL,
  alternativas    TEXT NOT NULL DEFAULT '[]',    -- JSON [{opcion, descartada_por}]
  decidio         TEXT NOT NULL CHECK (decidio IN ('humano','agente','agente_aprobado')),
  alcance         TEXT NOT NULL DEFAULT '[]',    -- JSON array
  reversible      INTEGER NOT NULL DEFAULT 1,
  estado          TEXT NOT NULL DEFAULT 'vigente' CHECK (estado IN ('vigente','reemplazada','revertida')),
  reemplazada_por TEXT REFERENCES decisiones(id),
  etiquetas       TEXT NOT NULL DEFAULT '[]'     -- JSON array, para `hz decision buscar`
);
CREATE VIRTUAL TABLE IF NOT EXISTS decisiones_fts USING fts5(id UNINDEXED, que, por_que, etiquetas, alternativas);

-- ───────────────────────── Reglas (RG y RD) ─────────────────────────
CREATE TABLE IF NOT EXISTS reglas (
  id            TEXT PRIMARY KEY,                -- RG-1.3 | RD-3
  tipo          TEXT NOT NULL CHECK (tipo IN ('general','dura')),
  orden         TEXT NOT NULL,                   -- texto imperativo
  por_que       TEXT NOT NULL,
  origen        TEXT CHECK (origen IN ('genesis','mejora','incidente','harness')),
  desde         TEXT NOT NULL,
  enforcement   TEXT NOT NULL DEFAULT '[]',      -- JSON: permisos|hook|githook|ci|sensor|rubrica
  version       INTEGER NOT NULL DEFAULT 1,
  estado        TEXT NOT NULL DEFAULT 'vigente' CHECK (estado IN ('vigente','inefectiva','modificada','retirada'))
);

CREATE TABLE IF NOT EXISTS regla_versiones (
  regla_id      TEXT NOT NULL REFERENCES reglas(id),
  version       INTEGER NOT NULL,
  orden         TEXT NOT NULL,
  por_que       TEXT NOT NULL,
  cambiada_en   TEXT NOT NULL,
  decision_id   TEXT NOT NULL,                   -- todo cambio de regla tiene decisión
  PRIMARY KEY (regla_id, version)
);

CREATE TABLE IF NOT EXISTS redteam_casos (
  id            TEXT PRIMARY KEY,                -- rt-…
  regla_id      TEXT NOT NULL REFERENCES reglas(id),
  descripcion   TEXT NOT NULL,
  intento       TEXT NOT NULL,                   -- acción a intentar en sandbox
  activo        INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS regla_eventos (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  regla_id      TEXT NOT NULL REFERENCES reglas(id),
  tipo          TEXT NOT NULL CHECK (tipo IN ('bloqueo','intento_evasion','escape','red_team','falsa_alarma','violacion_detectada')),
  resultado     TEXT,                            -- red_team: bloqueado|paso
  capa          TEXT,                            -- permisos|hook|githook|ci|sensor
  agente_id     TEXT,
  tarea_id      TEXT,
  caso_id       TEXT REFERENCES redteam_casos(id),
  detalle       TEXT,
  ocurrido_en   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_regla_eventos ON regla_eventos (regla_id, tipo, ocurrido_en);

-- ───────────────────────── Verificación y TDD ─────────────────────────
CREATE TABLE IF NOT EXISTS tareas_tdd (
  id              TEXT PRIMARY KEY,              -- T-…
  mision_id       TEXT REFERENCES misiones(id),
  titulo          TEXT NOT NULL,
  estado          TEXT NOT NULL DEFAULT 'contrato' CHECK (estado IN ('contrato','rojo','verde','refactor','verificada','cerrada')),
  contrato_ruta   TEXT,
  contrato_ok_en  TEXT,
  tests_bloqueados TEXT NOT NULL DEFAULT '[]',   -- JSON [{ruta, sha256}]
  rojo_en         TEXT,
  verde_en        TEXT,
  evidencia       TEXT NOT NULL DEFAULT '[]',    -- JSON [{cmd, resultado, commit, nivel}]
  exencion        TEXT,                          -- motivo si aplica una exención declarada
  -- Tablero (DAG). TaskList no existe en este build: el tablero vive aquí (spike S9)
  celula          TEXT,
  owner_agente_id TEXT,
  bloqueada_por   TEXT NOT NULL DEFAULT '[]'     -- JSON [T-…]
);

CREATE TABLE IF NOT EXISTS verificadores (
  id            TEXT PRIMARY KEY,                -- p. ej. V1:pytest, V0:sensor-rg1-cliente-en-loop
  nivel         TEXT NOT NULL CHECK (nivel IN ('V0','V1','V2','V3','V4','V5')),
  comando       TEXT NOT NULL,
  deteccion     REAL,                            -- recall sobre fallas sembradas (meta-eval)
  falsas_alarmas REAL,
  costo_ms      INTEGER,
  evaluado_en   TEXT
);

-- ───────────────────────── Errores, lecciones y evals ─────────────────────────
CREATE TABLE IF NOT EXISTS clases_error (
  id              TEXT PRIMARY KEY,
  causa_raiz      TEXT NOT NULL CHECK (causa_raiz IN ('contrato_ambiguo','contexto_faltante','regla_ignorada','patron_incorrecto','herramienta_mal_usada','test_debil','sobreajuste','otra')),
  ambito          TEXT NOT NULL,                 -- dominio o glob de rutas
  patron          TEXT NOT NULL,
  requiere_sensor INTEGER NOT NULL DEFAULT 0,
  recurrencia     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS errores (
  id            TEXT PRIMARY KEY,                -- E-…
  cuando        TEXT NOT NULL,
  mision_id     TEXT,
  ronda_o_hito  TEXT,
  sesion        TEXT,
  commit_ref    TEXT,
  agente_id     TEXT,
  rol           TEXT,
  tarea_id      TEXT,
  que_hizo      TEXT NOT NULL,
  fuente        TEXT NOT NULL CHECK (fuente IN ('verificacion','evaluador','humano','regla_dura','revert','regresion')),
  detectado_por TEXT,                            -- verificador o 'escapo'
  clase_id      TEXT REFERENCES clases_error(id),
  severidad     TEXT NOT NULL CHECK (severidad IN ('critico','mayor','menor')),
  reproducible  TEXT,                            -- JSON {commit, tarea, archivos}
  leccion_id    TEXT                             -- lección implicada, si la hubo
);

CREATE TABLE IF NOT EXISTS lecciones (
  id            TEXT PRIMARY KEY,                -- L-…
  clase_id      TEXT NOT NULL REFERENCES clases_error(id),
  skill_ruta    TEXT NOT NULL,                   -- .claude/skills/hz-leccion-<clase>/SKILL.md
  alcance       TEXT NOT NULL DEFAULT 'proyecto' CHECK (alcance IN ('proyecto','global')),
  estado        TEXT NOT NULL DEFAULT 'en_prueba' CHECK (estado IN ('en_prueba','activa','reescrita','eliminada','podada')),
  reescrituras  INTEGER NOT NULL DEFAULT 0,
  disparos      INTEGER NOT NULL DEFAULT 0,
  creada_en     TEXT NOT NULL,
  actualizada_en TEXT NOT NULL,
  motivo_estado TEXT
);

CREATE TABLE IF NOT EXISTS evals (
  id            TEXT PRIMARY KEY,
  objeto_tipo   TEXT NOT NULL CHECK (objeto_tipo IN ('producto','verificador','regla','leccion','agente','harness')),
  objeto_id     TEXT NOT NULL,
  tipo          TEXT NOT NULL,                   -- prevencion|interferencia|activacion|deteccion|red_team|escenarios|…
  descripcion   TEXT
);

CREATE TABLE IF NOT EXISTS eval_runs (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  eval_id       TEXT NOT NULL REFERENCES evals(id),
  resultado     TEXT NOT NULL,                   -- pasa|falla|puntaje
  puntaje       REAL,
  costo_usd     REAL,
  detalle       TEXT,
  corrido_en    TEXT NOT NULL
);

-- ───────────────────────── Orquestación ─────────────────────────
CREATE TABLE IF NOT EXISTS agentes (
  id            TEXT PRIMARY KEY,                -- agentId de Claude Code
  nombre        TEXT,
  rol           TEXT NOT NULL,
  padre_id      TEXT,
  tarea_id      TEXT,
  modelo        TEXT,
  estado        TEXT NOT NULL DEFAULT 'corriendo',
  iniciado_en   TEXT NOT NULL,
  terminado_en  TEXT,
  tokens        INTEGER
);

CREATE TABLE IF NOT EXISTS traspasos (
  id            TEXT PRIMARY KEY,                -- TR-…
  tarea_id      TEXT NOT NULL,
  rol           TEXT NOT NULL,
  agente_id     TEXT,
  estado        TEXT NOT NULL CHECK (estado IN ('listo','fallo','bloqueado')),
  tarjeta       TEXT NOT NULL,                   -- YAML ≤30 líneas
  salida_ok     INTEGER,                         -- resultado de `hz salida`
  creado_en     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS arriendos (
  ruta          TEXT PRIMARY KEY,
  agente_id     TEXT NOT NULL,
  tarea_id      TEXT,
  expira_en     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS mensajes (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  de            TEXT NOT NULL,
  para          TEXT NOT NULL,
  tipo          TEXT NOT NULL CHECK (tipo IN ('PREGUNTA','RESPUESTA','TRASPASO','BLOQUEO','CAMBIO-CONTRATO','ALERTA')),
  resumen       TEXT NOT NULL,                   -- ≤200 caracteres; el cuerpo queda en el transcript
  referencias   TEXT NOT NULL DEFAULT '[]',
  enviado_en    TEXT NOT NULL
);
