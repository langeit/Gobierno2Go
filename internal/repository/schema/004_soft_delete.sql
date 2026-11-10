-- Borrado lógico. estado 'V' = vigente, 'E' = eliminado.
-- Idempotente: se puede ejecutar más de una vez sobre una base ya creada.

ALTER TABLE adu_tipo_dominio ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_dominio ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_materialmm ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_proveedormm ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_asignacion ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_orden_compra ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_detalle_ordenco ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_imputacion_orden ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE adu_detalle_sol_mat ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE for_dias_permiso ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE for_solic_acumferiado ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE gen_departamento ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE seg_perfil ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE seg_perfil_modulo ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE seg_usuario_perfil ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE inv_responsable ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE cor_doctoetapa ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';

ALTER TABLE seg_perfil DROP CONSTRAINT IF EXISTS seg_perfil_nombre_key;
CREATE UNIQUE INDEX IF NOT EXISTS seg_perfil_nombre_vigente_idx ON seg_perfil (nombre) WHERE estado = 'V';

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
WHERE det.estado = 'V'
  AND sto.correlativo IN (
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
WHERE mat.estado = 'V'
  AND sto.correlativo IN (
    SELECT max(s.correlativo)
    FROM adu_stock s
    GROUP BY s.cod_material, s.valor_unidad
)
GROUP BY mat.tipo_material, dom.desc_dominio, sto.cod_material, mat.nombre_material, mat.stock_critico;
