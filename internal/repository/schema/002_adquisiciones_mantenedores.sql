-- Mantenedores de adquisiciones.
-- Idempotente: se puede ejecutar más de una vez sobre una base ya creada.

CREATE TABLE IF NOT EXISTS adu_tipo_dominio (
    cod_tipo_dom SERIAL PRIMARY KEY,
    desc_tipo_dom VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS adu_dominio (
    cod_dominio SERIAL PRIMARY KEY,
    cod_tipo_dom INTEGER REFERENCES adu_tipo_dominio(cod_tipo_dom),
    desc_dominio VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS adu_materialmm (
    cod_material SERIAL PRIMARY KEY,
    nombre_material VARCHAR(100) NOT NULL,
    tipo_material INTEGER NOT NULL REFERENCES adu_dominio(cod_dominio),
    stock_critico INTEGER NOT NULL DEFAULT 1
);

ALTER TABLE adu_proveedormm
    ADD COLUMN IF NOT EXISTS region_proveedor INTEGER,
    ADD COLUMN IF NOT EXISTS contacto_proveedor VARCHAR(100),
    ADD COLUMN IF NOT EXISTS fax_proveedor CHAR(15);

INSERT INTO adu_tipo_dominio (cod_tipo_dom, desc_tipo_dom) VALUES
    (1, 'Tipo Material'),
    (2, 'Estado Solicitud'),
    (3, 'Tipo Orden'),
    (4, 'Región'),
    (5, 'Otros'),
    (6, 'Modo Factura'),
    (7, 'Modo Orden'),
    (8, 'Tipo Impuesto')
ON CONFLICT (cod_tipo_dom) DO NOTHING;

INSERT INTO adu_dominio (cod_dominio, cod_tipo_dom, desc_dominio) VALUES
    (1, 1, 'Aseo'),
    (2, 1, 'Computación'),
    (3, 1, 'Escritorio'),
    (4, 1, 'Eléctrico'),
    (5, 2, 'Pendiente'),
    (6, 2, 'Resuelta'),
    (9, 4, 'Región Metropolitana'),
    (10, 3, 'Orden de Pedido'),
    (12, 3, 'Res. Admin. Ex. de Compra'),
    (13, 3, 'Res. Admin. de Compra'),
    (14, 6, 'Factura de Materiales'),
    (15, 6, 'Factura de Servicios'),
    (16, 4, 'Quinta región'),
    (17, 7, 'Orden de Materiales'),
    (18, 7, 'Orden de Servicios'),
    (19, 8, '19'),
    (20, 8, '10'),
    (21, 8, '0'),
    (25, 2, 'Aprobación')
ON CONFLICT (cod_dominio) DO NOTHING;

SELECT setval(
    pg_get_serial_sequence('adu_tipo_dominio', 'cod_tipo_dom'),
    (SELECT COALESCE(MAX(cod_tipo_dom), 1) FROM adu_tipo_dominio)
);

SELECT setval(
    pg_get_serial_sequence('adu_dominio', 'cod_dominio'),
    (SELECT COALESCE(MAX(cod_dominio), 1) FROM adu_dominio)
);
