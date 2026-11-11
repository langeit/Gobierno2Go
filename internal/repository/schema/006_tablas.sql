-- Tablas del menú Bases, submenú Tablas.
-- Idempotente: se puede ejecutar más de una vez.
-- estado 'V' = vigente, 'E' = eliminado.
--
-- El largo que el manual indica se usa tal cual.
-- Donde el manual no indica largo, el comentario de esa tabla lo dice.
-- Bancos/Sucursales y Monedas y Equivalencia traen una grilla hija, así que
-- cada una queda en dos tablas. Impuesto único guarda un tramo por año y mes.
-- I.P.C. mensual guarda un año, el factor de apertura y un factor por mes.

CREATE TABLE IF NOT EXISTS tab_region (
    codigo VARCHAR(4) PRIMARY KEY,
    descripcion VARCHAR(40) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El largo del código depende de Bases/Parámetros y el manual no da un número.
-- Los ejemplos del informe usan 3 caracteres. Se deja el código en 8.
CREATE TABLE IF NOT EXISTS tab_centros_costo (
    codigo VARCHAR(8) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    nivel SMALLINT,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_areas_negocio (
    codigo VARCHAR(3) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El informe de ejemplo usa códigos de 4 dígitos. La descripción no trae largo: se usa 60.
-- Numérico pide dígitos del entero y de los decimales. Texto pide su largo. Fecha no pide más datos.
CREATE TABLE IF NOT EXISTS tab_atributos (
    codigo VARCHAR(4) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    formato VARCHAR(10) NOT NULL CHECK (formato IN ('Numérico', 'Fecha', 'Texto')),
    largo_texto SMALLINT,
    digitos_entero SMALLINT,
    digitos_decimal SMALLINT,
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    CHECK (
        (formato = 'Numérico' AND digitos_entero IS NOT NULL AND digitos_decimal IS NOT NULL AND largo_texto IS NULL)
        OR (formato = 'Fecha' AND digitos_entero IS NULL AND digitos_decimal IS NULL AND largo_texto IS NULL)
        OR (formato = 'Texto' AND largo_texto IS NOT NULL AND largo_texto > 0 AND digitos_entero IS NULL AND digitos_decimal IS NULL)
    )
);

CREATE TABLE IF NOT EXISTS tab_ine (
    codigo VARCHAR(4) PRIMARY KEY,
    descripcion VARCHAR(30) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_ciudad (
    codigo VARCHAR(3) PRIMARY KEY,
    descripcion VARCHAR(20) NOT NULL,
    codigo_region VARCHAR(4) NOT NULL REFERENCES tab_region(codigo),
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_comuna (
    codigo VARCHAR(4) PRIMARY KEY,
    descripcion VARCHAR(20) NOT NULL,
    codigo_region VARCHAR(4) NOT NULL REFERENCES tab_region(codigo),
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El párrafo de provincia copia el de ciudades: el código queda en 5 y la descripción en 50.
CREATE TABLE IF NOT EXISTS tab_provincia (
    codigo VARCHAR(5) PRIMARY KEY,
    descripcion VARCHAR(50) NOT NULL,
    codigo_region VARCHAR(4) NOT NULL REFERENCES tab_region(codigo),
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_tipos_eventos (
    codigo VARCHAR(3) PRIMARY KEY,
    descripcion VARCHAR(40) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_estudios_superiores (
    codigo VARCHAR(3) PRIMARY KEY,
    descripcion VARCHAR(40) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_cargos (
    codigo VARCHAR(3) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El manual no indica largos de la AFP. Se usan los que sí indica para la ISAPRE.
CREATE TABLE IF NOT EXISTS tab_afp (
    codigo VARCHAR(3) PRIMARY KEY,
    nombre VARCHAR(40) NOT NULL,
    rut VARCHAR(20) NOT NULL,
    direccion VARCHAR(40) NOT NULL,
    codigo_comuna VARCHAR(4) NOT NULL REFERENCES tab_comuna(codigo),
    codigo_ciudad VARCHAR(3) NOT NULL REFERENCES tab_ciudad(codigo),
    telefono VARCHAR(20) NOT NULL,
    fax VARCHAR(20) NOT NULL,
    pct_fondo_pensiones NUMERIC NOT NULL,
    pct_adicional_seguro NUMERIC NOT NULL,
    contacto_apellido_paterno VARCHAR(20) NOT NULL,
    contacto_apellido_materno VARCHAR(20) NOT NULL,
    contacto_nombres VARCHAR(20) NOT NULL,
    contacto_rut VARCHAR(20) NOT NULL,
    contacto_telefono VARCHAR(20) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_isapre (
    codigo VARCHAR(3) PRIMARY KEY,
    nombre VARCHAR(40) NOT NULL,
    rut VARCHAR(20) NOT NULL,
    direccion VARCHAR(40) NOT NULL,
    codigo_comuna VARCHAR(4) NOT NULL REFERENCES tab_comuna(codigo),
    codigo_ciudad VARCHAR(3) NOT NULL REFERENCES tab_ciudad(codigo),
    telefono VARCHAR(20) NOT NULL,
    fax VARCHAR(20) NOT NULL,
    contacto_apellido_paterno VARCHAR(20) NOT NULL,
    contacto_apellido_materno VARCHAR(20) NOT NULL,
    contacto_nombres VARCHAR(20) NOT NULL,
    contacto_rut VARCHAR(20) NOT NULL,
    contacto_telefono VARCHAR(20) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_caja_compensacion (
    codigo VARCHAR(3) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_bancos (
    codigo VARCHAR(2) PRIMARY KEY,
    rut VARCHAR(20) NOT NULL,
    nombre VARCHAR(40) NOT NULL,
    identificacion_superintendencia VARCHAR(10) NOT NULL,
    casilla VARCHAR(10) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El manual no indica el largo de la sucursal. El informe muestra códigos de 3 caracteres.
-- El código queda en 10 y la descripción en 60.
CREATE TABLE IF NOT EXISTS tab_bancos_sucursales (
    codigo_banco VARCHAR(2) NOT NULL REFERENCES tab_bancos(codigo),
    codigo VARCHAR(10) NOT NULL,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    PRIMARY KEY (codigo_banco, codigo)
);

CREATE TABLE IF NOT EXISTS tab_impuesto_unico (
    anio SMALLINT NOT NULL,
    mes SMALLINT NOT NULL CHECK (mes BETWEEN 1 AND 12),
    tramo SMALLINT NOT NULL CHECK (tramo >= 1),
    tope_minimo NUMERIC NOT NULL,
    tope_maximo NUMERIC NOT NULL,
    porcentaje NUMERIC NOT NULL,
    cantidad_rebajar NUMERIC NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    PRIMARY KEY (anio, mes, tramo)
);

CREATE TABLE IF NOT EXISTS tab_billetes_monedas (
    valor NUMERIC PRIMARY KEY,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_ipc (
    anio SMALLINT PRIMARY KEY,
    factor_apertura NUMERIC,
    factor_enero NUMERIC,
    factor_febrero NUMERIC,
    factor_marzo NUMERIC,
    factor_abril NUMERIC,
    factor_mayo NUMERIC,
    factor_junio NUMERIC,
    factor_julio NUMERIC,
    factor_agosto NUMERIC,
    factor_septiembre NUMERIC,
    factor_octubre NUMERIC,
    factor_noviembre NUMERIC,
    factor_diciembre NUMERIC,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El símbolo y el código del Banco Central no traen largo. Se dejan en 10.
-- Decimales de informe: máximo 2. Decimales de venta: máximo 5.
CREATE TABLE IF NOT EXISTS tab_monedas (
    codigo VARCHAR(2) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    simbolo VARCHAR(10) NOT NULL,
    decimales_informe SMALLINT NOT NULL CHECK (decimales_informe BETWEEN 0 AND 2),
    decimales_venta SMALLINT NOT NULL CHECK (decimales_venta BETWEEN 0 AND 5),
    moneda_banco_central VARCHAR(10) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_monedas_equivalencia (
    codigo_moneda VARCHAR(2) NOT NULL REFERENCES tab_monedas(codigo),
    fecha DATE NOT NULL,
    valor NUMERIC NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    PRIMARY KEY (codigo_moneda, fecha)
);

-- El manual no indica largos. El informe muestra códigos de un carácter y puerta COM1 o COM2.
-- El año del reloj se maneja con 2 o con 4 dígitos.
CREATE TABLE IF NOT EXISTS tab_relojes (
    codigo VARCHAR(4) PRIMARY KEY,
    ubicacion VARCHAR(60) NOT NULL,
    puerta VARCHAR(10) NOT NULL,
    digitos_anio SMALLINT NOT NULL CHECK (digitos_anio IN (2, 4)),
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_festivos (
    fecha DATE PRIMARY KEY,
    motivo VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_tipos_ausentismos (
    codigo VARCHAR(4) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

-- El manual no indica el largo del código ni de la descripción. El informe usa códigos de un dígito.
-- Días a descontar van de 0 a 100. Los meses de base imponible van de 0 a 23.
-- Bajo n días no puede ser menor que los días a descontar.
CREATE TABLE IF NOT EXISTS tab_tipos_licencias (
    codigo VARCHAR(4) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    descuenta_dias BOOLEAN NOT NULL,
    dias_descontar SMALLINT NOT NULL CHECK (dias_descontar BETWEEN 0 AND 100),
    descuenta_bajo_dias SMALLINT NOT NULL,
    meses_base_imponible SMALLINT NOT NULL CHECK (meses_base_imponible BETWEEN 0 AND 23),
    estado VARCHAR(1) NOT NULL DEFAULT 'V',
    CHECK (descuenta_bajo_dias >= dias_descontar)
);

CREATE TABLE IF NOT EXISTS tab_especialidades_medicas (
    codigo VARCHAR(15) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

CREATE TABLE IF NOT EXISTS tab_diagnosticos (
    codigo VARCHAR(15) PRIMARY KEY,
    descripcion VARCHAR(60) NOT NULL,
    estado VARCHAR(1) NOT NULL DEFAULT 'V'
);

ALTER TABLE tab_region ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_centros_costo ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_areas_negocio ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_atributos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_ine ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_ciudad ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_comuna ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_provincia ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_tipos_eventos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_estudios_superiores ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_cargos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_afp ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_isapre ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_caja_compensacion ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_bancos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_bancos_sucursales ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_impuesto_unico ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_billetes_monedas ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_ipc ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_monedas ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_monedas_equivalencia ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_relojes ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_festivos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_tipos_ausentismos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_tipos_licencias ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_especialidades_medicas ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
ALTER TABLE tab_diagnosticos ADD COLUMN IF NOT EXISTS estado VARCHAR(1) NOT NULL DEFAULT 'V';
