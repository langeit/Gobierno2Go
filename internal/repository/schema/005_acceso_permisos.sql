-- Permisos por proceso, ámbito del perfil, auditoría y política de clave.
-- Idempotente: se puede ejecutar más de una vez.

ALTER TABLE seg_perfil ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE seg_perfil_modulo ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE seg_usuario_perfil ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';

CREATE TABLE IF NOT EXISTS seg_permiso (
    codigo VARCHAR(40) PRIMARY KEY,
    codigo_modulo VARCHAR(20) NOT NULL REFERENCES seg_modulo(codigo),
    accion VARCHAR(20) NOT NULL,
    nombre VARCHAR(80) NOT NULL,
    orden SMALLINT NOT NULL
);

CREATE TABLE IF NOT EXISTS seg_perfil_permiso (
    id_perfil INTEGER NOT NULL REFERENCES seg_perfil(id_perfil) ON DELETE CASCADE,
    codigo_permiso VARCHAR(40) NOT NULL REFERENCES seg_permiso(codigo),
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    PRIMARY KEY (id_perfil, codigo_permiso)
);

CREATE TABLE IF NOT EXISTS seg_perfil_ambito (
    id_perfil INTEGER NOT NULL REFERENCES seg_perfil(id_perfil) ON DELETE CASCADE,
    tipo VARCHAR(20) NOT NULL,
    referencia INTEGER NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    PRIMARY KEY (id_perfil, tipo, referencia)
);

CREATE TABLE IF NOT EXISTS seg_auditoria (
    id_auditoria SERIAL PRIMARY KEY,
    fecha TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    id_usuario VARCHAR(12) NOT NULL DEFAULT '',
    accion VARCHAR(40) NOT NULL,
    detalle VARCHAR(400) NOT NULL DEFAULT '',
    ip VARCHAR(64) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS seg_migracion (
    codigo VARCHAR(40) PRIMARY KEY,
    aplicada_en TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE gen_usuario ADD COLUMN IF NOT EXISTS debe_cambiar_password BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE gen_usuario ADD COLUMN IF NOT EXISTS password_expira TIMESTAMP;
ALTER TABLE gen_usuario ADD COLUMN IF NOT EXISTS intentos_fallidos SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE gen_usuario ADD COLUMN IF NOT EXISTS bloqueado_hasta TIMESTAMP;
ALTER TABLE gen_usuario ADD COLUMN IF NOT EXISTS password_cambiada_en TIMESTAMP;

CREATE UNIQUE INDEX IF NOT EXISTS gen_usuario_empleado_key
    ON gen_usuario (id_empleado)
    WHERE id_empleado IS NOT NULL;

UPDATE gen_usuario
SET password_expira = CURRENT_TIMESTAMP + INTERVAL '90 days'
WHERE password_expira IS NULL;

INSERT INTO seg_permiso (codigo, codigo_modulo, accion, nombre, orden) VALUES
    ('adquisiciones.consultar', 'adquisiciones', 'consultar', 'Consultar', 11),
    ('adquisiciones.ingresar', 'adquisiciones', 'ingresar', 'Ingresar', 12),
    ('adquisiciones.aprobar', 'adquisiciones', 'aprobar', 'Aprobar', 13),
    ('adquisiciones.mantener', 'adquisiciones', 'mantener', 'Mantener', 14),
    ('inventario.consultar', 'inventario', 'consultar', 'Consultar', 21),
    ('inventario.ingresar', 'inventario', 'ingresar', 'Ingresar', 22),
    ('inventario.aprobar', 'inventario', 'aprobar', 'Aprobar', 23),
    ('inventario.mantener', 'inventario', 'mantener', 'Mantener', 24),
    ('correspondencia.consultar', 'correspondencia', 'consultar', 'Consultar', 31),
    ('correspondencia.ingresar', 'correspondencia', 'ingresar', 'Ingresar', 32),
    ('correspondencia.aprobar', 'correspondencia', 'aprobar', 'Aprobar', 33),
    ('correspondencia.mantener', 'correspondencia', 'mantener', 'Mantener', 34),
    ('formularios.consultar', 'formularios', 'consultar', 'Consultar', 41),
    ('formularios.ingresar', 'formularios', 'ingresar', 'Ingresar', 42),
    ('formularios.aprobar', 'formularios', 'aprobar', 'Aprobar', 43),
    ('formularios.mantener', 'formularios', 'mantener', 'Mantener', 44),
    ('administracion.consultar', 'administracion', 'consultar', 'Consultar', 51),
    ('administracion.ingresar', 'administracion', 'ingresar', 'Ingresar', 52),
    ('administracion.aprobar', 'administracion', 'aprobar', 'Aprobar', 53),
    ('administracion.mantener', 'administracion', 'mantener', 'Mantener', 54)
ON CONFLICT (codigo) DO UPDATE
SET codigo_modulo = EXCLUDED.codigo_modulo,
    accion = EXCLUDED.accion,
    nombre = EXCLUDED.nombre,
    orden = EXCLUDED.orden;

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Funcionario', 'Consulta e ingreso de sus feriados y permisos.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Funcionario' AND estado = 'V');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Jefatura', 'Consulta, ingreso y aprobación de feriados de su ámbito.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Jefatura' AND estado = 'V');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Oficina de Personal', 'Mantiene fichas, días y aprobaciones de todo el servicio.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Oficina de Personal' AND estado = 'V');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Operador de inventario', 'Consulta, ingreso y aprobación de inventario, sin mantenedores.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Operador de inventario' AND estado = 'V');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Encargado de inventario', 'Opera inventario y mantiene sus catálogos.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Encargado de inventario' AND estado = 'V');

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT p.id_perfil, pe.codigo, 'V'
FROM seg_perfil p
CROSS JOIN seg_permiso pe
WHERE p.protegido = TRUE
  AND p.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = p.id_perfil)
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT pm.id_perfil, pe.codigo, 'V'
FROM seg_perfil_modulo pm
JOIN seg_perfil p ON p.id_perfil = pm.id_perfil AND p.protegido = FALSE AND p.estado = 'V'
JOIN seg_permiso pe ON pe.codigo_modulo = pm.codigo_modulo
WHERE pm.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = pm.id_perfil)
  AND (
        pe.codigo_modulo IN ('adquisiciones', 'correspondencia', 'administracion')
        OR (pe.codigo_modulo = 'formularios' AND pe.accion IN ('consultar', 'ingresar', 'aprobar'))
        OR (pe.codigo_modulo = 'inventario' AND pe.accion IN ('consultar', 'ingresar'))
      )
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT p.id_perfil, pe.codigo, 'V'
FROM seg_perfil p
JOIN seg_permiso pe ON pe.codigo IN ('formularios.consultar', 'formularios.ingresar')
WHERE p.nombre = 'Funcionario' AND p.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = p.id_perfil)
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT p.id_perfil, pe.codigo, 'V'
FROM seg_perfil p
JOIN seg_permiso pe ON pe.codigo IN ('formularios.consultar', 'formularios.ingresar', 'formularios.aprobar')
WHERE p.nombre = 'Jefatura' AND p.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = p.id_perfil)
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT p.id_perfil, pe.codigo, 'V'
FROM seg_perfil p
JOIN seg_permiso pe ON pe.codigo_modulo = 'formularios'
WHERE p.nombre = 'Oficina de Personal' AND p.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = p.id_perfil)
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT p.id_perfil, pe.codigo, 'V'
FROM seg_perfil p
JOIN seg_permiso pe ON pe.codigo IN ('inventario.consultar', 'inventario.ingresar', 'inventario.aprobar')
WHERE p.nombre = 'Operador de inventario' AND p.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = p.id_perfil)
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
SELECT p.id_perfil, pe.codigo, 'V'
FROM seg_perfil p
JOIN seg_permiso pe ON pe.codigo_modulo = 'inventario'
WHERE p.nombre = 'Encargado de inventario' AND p.estado = 'V'
  AND NOT EXISTS (SELECT 1 FROM seg_perfil_permiso x WHERE x.id_perfil = p.id_perfil)
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo, estado)
SELECT DISTINCT pp.id_perfil, pe.codigo_modulo, 'V'
FROM seg_perfil_permiso pp
JOIN seg_permiso pe ON pe.codigo = pp.codigo_permiso
WHERE pp.estado = 'V'
ON CONFLICT (id_perfil, codigo_modulo) DO UPDATE SET estado = 'V';
