-- Perfiles de acceso a los módulos de Gobierno2Go.
-- El script es idempotente: se puede ejecutar más de una vez.

CREATE TABLE IF NOT EXISTS seg_modulo (
    codigo VARCHAR(20) PRIMARY KEY,
    nombre VARCHAR(80) NOT NULL,
    descripcion VARCHAR(200) NOT NULL DEFAULT '',
    orden SMALLINT NOT NULL
);

CREATE TABLE IF NOT EXISTS seg_perfil (
    id_perfil SERIAL PRIMARY KEY,
    nombre VARCHAR(80) NOT NULL UNIQUE,
    descripcion VARCHAR(200) NOT NULL DEFAULT '',
    protegido BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS seg_perfil_modulo (
    id_perfil INTEGER NOT NULL REFERENCES seg_perfil(id_perfil) ON DELETE CASCADE,
    codigo_modulo VARCHAR(20) NOT NULL REFERENCES seg_modulo(codigo),
    PRIMARY KEY (id_perfil, codigo_modulo)
);

CREATE TABLE IF NOT EXISTS seg_usuario_perfil (
    id_usuario VARCHAR(12) NOT NULL REFERENCES gen_usuario(id_usuario) ON DELETE CASCADE,
    id_perfil INTEGER NOT NULL REFERENCES seg_perfil(id_perfil) ON DELETE CASCADE,
    PRIMARY KEY (id_usuario, id_perfil)
);

CREATE UNIQUE INDEX IF NOT EXISTS gen_usuario_login_key ON gen_usuario (login);

INSERT INTO seg_modulo (codigo, nombre, descripcion, orden) VALUES
    ('adquisiciones', 'Adquisiciones', 'Órdenes de compra, proveedores y stock.', 1),
    ('inventario', 'Inventario', 'Bienes, ubicaciones, responsables y depreciación.', 2),
    ('correspondencia', 'Correspondencia', 'Documentos oficiales y estadísticas.', 3),
    ('formularios', 'Formularios y RRHH', 'Feriados, días administrativos y acumulación.', 4),
    ('administracion', 'Administración', 'Perfiles de acceso y usuarios.', 5)
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion,
    orden = EXCLUDED.orden;

-- La clave inicial queda en claro solo en este alta. Al arrancar, MigrarContrasenas la guarda como bcrypt.
INSERT INTO gen_usuario (id_usuario, login, password, estado, nombre, tipo)
VALUES ('admin', 'admin', 'secret', '1', 'Administrador', '0')
ON CONFLICT (id_usuario) DO NOTHING;

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Administrador', 'Acceso a todos los módulos y a la administración de perfiles.', TRUE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Administrador');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Acceso general', 'Operación de adquisiciones, inventario, correspondencia y formularios.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Acceso general');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Adquisiciones', 'Solo el módulo de adquisiciones.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Adquisiciones');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Inventario', 'Solo el módulo de inventario.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Inventario');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Correspondencia', 'Solo el módulo de correspondencia.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Correspondencia');

INSERT INTO seg_perfil (nombre, descripcion, protegido)
SELECT 'Formularios y RRHH', 'Solo el módulo de formularios y recursos humanos.', FALSE
WHERE NOT EXISTS (SELECT 1 FROM seg_perfil WHERE nombre = 'Formularios y RRHH');

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo)
SELECT p.id_perfil, m.codigo
FROM seg_perfil p
CROSS JOIN seg_modulo m
WHERE p.nombre = 'Administrador'
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo)
SELECT p.id_perfil, m.codigo
FROM seg_perfil p
JOIN seg_modulo m ON m.codigo <> 'administracion'
WHERE p.nombre = 'Acceso general'
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo)
SELECT p.id_perfil, 'adquisiciones'
FROM seg_perfil p
WHERE p.nombre = 'Adquisiciones'
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo)
SELECT p.id_perfil, 'inventario'
FROM seg_perfil p
WHERE p.nombre = 'Inventario'
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo)
SELECT p.id_perfil, 'correspondencia'
FROM seg_perfil p
WHERE p.nombre = 'Correspondencia'
ON CONFLICT DO NOTHING;

INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo)
SELECT p.id_perfil, 'formularios'
FROM seg_perfil p
WHERE p.nombre = 'Formularios y RRHH'
ON CONFLICT DO NOTHING;

INSERT INTO seg_usuario_perfil (id_usuario, id_perfil)
SELECT 'admin', p.id_perfil
FROM seg_perfil p
WHERE p.nombre = 'Administrador'
ON CONFLICT DO NOTHING;

INSERT INTO seg_usuario_perfil (id_usuario, id_perfil)
SELECT u.id_usuario, p.id_perfil
FROM gen_usuario u
JOIN seg_perfil p ON p.nombre = 'Acceso general'
WHERE u.id_usuario <> 'admin'
  AND NOT EXISTS (
      SELECT 1 FROM seg_usuario_perfil up WHERE up.id_usuario = u.id_usuario
  )
ON CONFLICT DO NOTHING;
