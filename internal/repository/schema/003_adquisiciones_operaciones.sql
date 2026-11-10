-- Operación de adquisiciones: solicitudes, stock, órdenes, facturas y vistas.
-- Idempotente: se puede ejecutar más de una vez sobre una base ya creada.

ALTER TABLE adu_solicitud_materiales
    ADD COLUMN IF NOT EXISTS hora_sol_mat TIME,
    ADD COLUMN IF NOT EXISTS fecha_aprueba_rechaza DATE;

ALTER TABLE adu_orden_compra
    ADD COLUMN IF NOT EXISTS total_orden DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS iva_orden DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS desc_servicio TEXT,
    ADD COLUMN IF NOT EXISTS cant_servicio DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS valorunit_servicio DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS tipo_impuesto INTEGER,
    ADD COLUMN IF NOT EXISTS modo_orden INTEGER;

CREATE TABLE IF NOT EXISTS adu_detalle_sol_mat (
    nro_corr_solmat SERIAL PRIMARY KEY,
    cod_material INTEGER NOT NULL REFERENCES adu_materialmm(cod_material),
    nro_sol_mat INTEGER NOT NULL REFERENCES adu_solicitud_materiales(nro_sol_mat),
    cantidad_solicitud INTEGER NOT NULL DEFAULT 0,
    obs_mat TEXT,
    cantidad_autorizada INTEGER,
    fecha_despacho DATE,
    hora_despacho TIME,
    cantidad_despachada INTEGER
);

CREATE TABLE IF NOT EXISTS adu_stock (
    correlativo SERIAL PRIMARY KEY,
    cod_material INTEGER NOT NULL REFERENCES adu_materialmm(cod_material),
    fecha DATE,
    cantidad INTEGER,
    valor_unidad INTEGER,
    hora TIME
);

CREATE TABLE IF NOT EXISTS adu_detalle_ordenco (
    corr_det_ordenco SERIAL PRIMARY KEY,
    nro_orden_compra INTEGER REFERENCES adu_orden_compra(nro_orden_compra),
    cod_material INTEGER REFERENCES adu_materialmm(cod_material),
    cantidad_detalleord INTEGER NOT NULL DEFAULT 0,
    valor_unitario_ord INTEGER NOT NULL DEFAULT 0,
    valor_totalmat INTEGER,
    facturado CHAR(1)
);

CREATE TABLE IF NOT EXISTS adu_facturapr (
    nro_corr_fact SERIAL PRIMARY KEY,
    nro_orden_compra INTEGER REFERENCES adu_orden_compra(nro_orden_compra),
    rut_proveedor VARCHAR(12) REFERENCES adu_proveedormm(rut_proveedor),
    nro_factura INTEGER NOT NULL DEFAULT 0,
    fecha_factura DATE NOT NULL,
    total_factura DOUBLE PRECISION NOT NULL DEFAULT 0,
    iva_factura DOUBLE PRECISION NOT NULL,
    total_final_factura DOUBLE PRECISION NOT NULL,
    fecha_ingreso DATE NOT NULL,
    id_usuario VARCHAR(12) NOT NULL REFERENCES gen_usuario(id_usuario),
    modo_factura INTEGER NOT NULL,
    desc_servicio TEXT,
    cant_servicio DOUBLE PRECISION,
    valorunit_servicio DOUBLE PRECISION,
    tipo_impuesto INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS adu_detalle_facturapr (
    nro_det_fact SERIAL PRIMARY KEY,
    cod_material INTEGER REFERENCES adu_materialmm(cod_material),
    nro_corr_fact INTEGER REFERENCES adu_facturapr(nro_corr_fact),
    cantidad_detallefact INTEGER NOT NULL DEFAULT 0,
    valor_unitariofactura INTEGER NOT NULL DEFAULT 0,
    valor_totalfact INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE VIEW adu_consumo_valorizado AS
SELECT sol.nro_sol_mat,
       det.nro_corr_solmat,
       sol.id_depto,
       det.fecha_despacho,
       dom.desc_dominio AS tipo_material,
       mat.tipo_material AS cod_tipo_mat,
       mat.cod_material,
       mat.nombre_material,
       det.cantidad_autorizada,
       sto.valor_unidad,
       (det.cantidad_autorizada * sto.valor_unidad) AS valor_total
FROM adu_solicitud_materiales sol
JOIN adu_detalle_sol_mat det ON sol.nro_sol_mat = det.nro_sol_mat
JOIN adu_materialmm mat ON det.cod_material = mat.cod_material
JOIN adu_dominio dom ON mat.tipo_material = dom.cod_dominio
JOIN adu_stock sto ON det.cod_material = sto.cod_material
    AND det.fecha_despacho = sto.fecha
    AND det.hora_despacho = sto.hora
WHERE sto.correlativo IN (
    SELECT max(s.correlativo)
    FROM adu_stock s
    WHERE s.fecha = det.fecha_despacho
      AND s.hora = det.hora_despacho
    GROUP BY s.cod_material
);

CREATE OR REPLACE VIEW adu_stock_actual AS
SELECT mat.tipo_material,
       dom.desc_dominio,
       sto.cod_material,
       mat.nombre_material,
       sum(sto.cantidad) AS stock_actual,
       mat.stock_critico
FROM adu_stock sto
JOIN adu_materialmm mat ON sto.cod_material = mat.cod_material
JOIN adu_dominio dom ON mat.tipo_material = dom.cod_dominio
WHERE sto.correlativo IN (
    SELECT max(s.correlativo)
    FROM adu_stock s
    GROUP BY s.cod_material, s.valor_unidad
)
GROUP BY mat.tipo_material, dom.desc_dominio, sto.cod_material, mat.nombre_material, mat.stock_critico;
