package models

import "strconv"

// Tipos de campo que sabe dibujar y validar el mantenedor de Bases/Tablas.
const (
	CampoTexto      = "texto"
	CampoEntero     = "entero"
	CampoDecimal    = "decimal"
	CampoFecha      = "fecha"
	CampoBooleano   = "booleano"
	CampoOpcion     = "opcion"
	CampoReferencia = "referencia"
)

// OpcionCampo es un valor fijo de un campo de selección.
type OpcionCampo struct {
	Valor    string
	Etiqueta string
}

// TablaCampo describe una columna y cómo se edita en pantalla.
type TablaCampo struct {
	Nombre      string
	Etiqueta    string
	Tipo        string
	Largo       int
	Requerido   bool
	Clave       bool
	Min         int
	Max         int
	Opciones    []OpcionCampo
	RefTabla    string
	RefValor    string
	RefEtiqueta string
	Ayuda       string
}

// EsClave indica si el campo forma parte de la clave primaria.
func (c TablaCampo) EsClave() bool {
	return c.Clave
}

// Editable indica si el campo se puede modificar después de crear el registro.
func (c TablaCampo) Editable() bool {
	return !c.Clave
}

// TablaUso es una tabla que referencia a otra y que impide su borrado lógico.
type TablaUso struct {
	Tabla   string
	Campo   string
	Detalle string
}

// TablaDef describe un mantenedor completo.
type TablaDef struct {
	Codigo      string
	Nombre      string
	Tabla       string
	Descripcion string
	Campos      []TablaCampo
	Orden       []string
	Etiqueta    string
	UsadaPor    []TablaUso
	Hija        *TablaDef
	CampoPadre  string
	Validar     func(valores map[string]string) string
}

// Clave devuelve los campos que forman la clave primaria.
func (t TablaDef) Clave() []TablaCampo {
	var clave []TablaCampo
	for _, c := range t.Campos {
		if c.Clave {
			clave = append(clave, c)
		}
	}
	return clave
}

// Editable indica si la tabla tiene columnas que se puedan modificar.
func (t TablaDef) Editable() bool {
	for _, c := range t.Campos {
		if !c.Clave {
			return true
		}
	}
	return false
}

// Campo busca una columna por nombre.
func (t TablaDef) Campo(nombre string) (TablaCampo, bool) {
	for _, c := range t.Campos {
		if c.Nombre == nombre {
			return c, true
		}
	}
	return TablaCampo{}, false
}

func campoTexto(nombre, etiqueta string, largo int, requerido bool) TablaCampo {
	return TablaCampo{Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoTexto, Largo: largo, Requerido: requerido}
}

func campoClaveTexto(nombre, etiqueta string, largo int) TablaCampo {
	return TablaCampo{Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoTexto, Largo: largo, Requerido: true, Clave: true}
}

func campoEntero(nombre, etiqueta string, min, max int, requerido, clave bool) TablaCampo {
	return TablaCampo{
		Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoEntero,
		Min: min, Max: max, Requerido: requerido, Clave: clave,
	}
}

func campoDecimal(nombre, etiqueta string, requerido, clave bool) TablaCampo {
	return TablaCampo{Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoDecimal, Requerido: requerido, Clave: clave}
}

func campoFecha(nombre, etiqueta string, requerido, clave bool) TablaCampo {
	return TablaCampo{Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoFecha, Requerido: requerido, Clave: clave}
}

func campoBooleano(nombre, etiqueta string) TablaCampo {
	return TablaCampo{Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoBooleano}
}

func campoOpcion(nombre, etiqueta string, requerido bool, opciones ...OpcionCampo) TablaCampo {
	return TablaCampo{Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoOpcion, Requerido: requerido, Opciones: opciones}
}

func campoReferencia(nombre, etiqueta, tabla string, requerido, clave bool) TablaCampo {
	return TablaCampo{
		Nombre: nombre, Etiqueta: etiqueta, Tipo: CampoReferencia,
		RefTabla: tabla, RefValor: "codigo", RefEtiqueta: "descripcion",
		Requerido: requerido, Clave: clave,
	}
}

var tablaRegion = TablaDef{
	Codigo:      "region",
	Nombre:      "Región",
	Tabla:       "tab_region",
	Descripcion: "Regiones del país.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("descripcion", "Descripción", 40, true),
	},
	Orden: []string{"codigo"},
	UsadaPor: []TablaUso{
		{Tabla: "tab_ciudad", Campo: "codigo_region", Detalle: "ciudades"},
		{Tabla: "tab_comuna", Campo: "codigo_region", Detalle: "comunas"},
		{Tabla: "tab_provincia", Campo: "codigo_region", Detalle: "provincias"},
	},
}

var tablaCiudad = TablaDef{
	Codigo:      "ciudad",
	Nombre:      "Ciudad",
	Tabla:       "tab_ciudad",
	Descripcion: "Ciudades agrupadas por región.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("descripcion", "Descripción", 20, true),
		campoReferencia("codigo_region", "Región", "tab_region", true, false),
	},
	Orden: []string{"descripcion"},
	UsadaPor: []TablaUso{
		{Tabla: "tab_afp", Campo: "codigo_ciudad", Detalle: "AFP"},
		{Tabla: "tab_isapre", Campo: "codigo_ciudad", Detalle: "ISAPRE"},
	},
}

var tablaComuna = TablaDef{
	Codigo:      "comuna",
	Nombre:      "Comuna",
	Tabla:       "tab_comuna",
	Descripcion: "Comunas agrupadas por región.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("descripcion", "Descripción", 20, true),
		campoReferencia("codigo_region", "Región", "tab_region", true, false),
	},
	Orden: []string{"descripcion"},
	UsadaPor: []TablaUso{
		{Tabla: "tab_afp", Campo: "codigo_comuna", Detalle: "AFP"},
		{Tabla: "tab_isapre", Campo: "codigo_comuna", Detalle: "ISAPRE"},
	},
}

var tablaProvincia = TablaDef{
	Codigo:      "provincia",
	Nombre:      "Provincia",
	Tabla:       "tab_provincia",
	Descripcion: "Provincias agrupadas por región.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 5),
		campoTexto("descripcion", "Descripción", 50, true),
		campoReferencia("codigo_region", "Región", "tab_region", true, false),
	},
	Orden: []string{"descripcion"},
}

var tablaCentrosCosto = TablaDef{
	Codigo:      "centros-de-costo",
	Nombre:      "Centros de costo",
	Tabla:       "tab_centros_costo",
	Descripcion: "Centros de costo usados por los procesos internos.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 8),
		campoTexto("descripcion", "Descripción", 60, true),
		campoEntero("nivel", "Nivel", 0, 99, false, false),
	},
	Orden: []string{"codigo"},
}

var tablaAreasNegocio = TablaDef{
	Codigo:      "areas-de-negocio",
	Nombre:      "Áreas de negocio",
	Tabla:       "tab_areas_negocio",
	Descripcion: "Áreas de negocio de la institución.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden: []string{"codigo"},
}

var tablaAtributos = TablaDef{
	Codigo:      "atributos",
	Nombre:      "Atributos",
	Tabla:       "tab_atributos",
	Descripcion: "Atributos y su formato de captura.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("descripcion", "Descripción", 60, true),
		campoOpcion("formato", "Formato", true,
			OpcionCampo{Valor: "Numérico", Etiqueta: "Numérico"},
			OpcionCampo{Valor: "Fecha", Etiqueta: "Fecha"},
			OpcionCampo{Valor: "Texto", Etiqueta: "Texto"},
		),
		func() TablaCampo {
			c := campoEntero("largo_texto", "Largo del texto", 1, 32767, false, false)
			c.Ayuda = "Solo para formato Texto."
			return c
		}(),
		func() TablaCampo {
			c := campoEntero("digitos_entero", "Dígitos enteros", 0, 32767, false, false)
			c.Ayuda = "Solo para formato Numérico."
			return c
		}(),
		func() TablaCampo {
			c := campoEntero("digitos_decimal", "Dígitos decimales", 0, 32767, false, false)
			c.Ayuda = "Solo para formato Numérico."
			return c
		}(),
	},
	Orden: []string{"codigo"},
	Validar: func(v map[string]string) string {
		switch v["formato"] {
		case "Numérico":
			if v["digitos_entero"] == "" || v["digitos_decimal"] == "" {
				return "Para el formato Numérico indique los dígitos enteros y decimales."
			}
			if v["largo_texto"] != "" {
				return "El formato Numérico no usa largo del texto."
			}
		case "Fecha":
			if v["largo_texto"] != "" || v["digitos_entero"] != "" || v["digitos_decimal"] != "" {
				return "El formato Fecha no usa largo del texto ni dígitos."
			}
		case "Texto":
			if v["largo_texto"] == "" {
				return "Para el formato Texto indique el largo del texto."
			}
			if v["digitos_entero"] != "" || v["digitos_decimal"] != "" {
				return "El formato Texto no usa dígitos enteros ni decimales."
			}
		}
		return ""
	},
}

var tablaINE = TablaDef{
	Codigo:      "ine",
	Nombre:      "INE",
	Tabla:       "tab_ine",
	Descripcion: "Códigos del Instituto Nacional de Estadísticas.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("descripcion", "Descripción", 30, true),
	},
	Orden: []string{"codigo"},
}

var tablaTiposEventos = TablaDef{
	Codigo:      "tipos-de-eventos",
	Nombre:      "Tipos de eventos",
	Tabla:       "tab_tipos_eventos",
	Descripcion: "Tipos de eventos registrados por el sistema.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("descripcion", "Descripción", 40, true),
	},
	Orden: []string{"codigo"},
}

var tablaEstudiosSuperiores = TablaDef{
	Codigo:      "estudios-superiores",
	Nombre:      "Estudios superiores",
	Tabla:       "tab_estudios_superiores",
	Descripcion: "Niveles de estudios superiores.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("descripcion", "Descripción", 40, true),
	},
	Orden: []string{"codigo"},
}

var tablaCargos = TablaDef{
	Codigo:      "cargos",
	Nombre:      "Cargos",
	Tabla:       "tab_cargos",
	Descripcion: "Cargos disponibles en la institución.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden: []string{"codigo"},
}

var tablaAFP = TablaDef{
	Codigo:      "afp",
	Nombre:      "AFP",
	Tabla:       "tab_afp",
	Descripcion: "Administradoras de fondos de pensiones.",
	Etiqueta:    "nombre",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("nombre", "Nombre", 40, true),
		campoTexto("rut", "RUT", 20, true),
		campoTexto("direccion", "Dirección", 40, true),
		campoReferencia("codigo_comuna", "Comuna", "tab_comuna", true, false),
		campoReferencia("codigo_ciudad", "Ciudad", "tab_ciudad", true, false),
		campoTexto("telefono", "Teléfono", 20, true),
		campoTexto("fax", "Fax", 20, false),
		campoDecimal("pct_fondo_pensiones", "Porcentaje fondo de pensiones", true, false),
		campoDecimal("pct_adicional_seguro", "Porcentaje adicional del seguro", true, false),
		campoTexto("contacto_apellido_paterno", "Contacto: apellido paterno", 20, false),
		campoTexto("contacto_apellido_materno", "Contacto: apellido materno", 20, false),
		campoTexto("contacto_nombres", "Contacto: nombres", 20, false),
		campoTexto("contacto_rut", "Contacto: RUT", 20, false),
		campoTexto("contacto_telefono", "Contacto: teléfono", 20, false),
	},
	Orden: []string{"nombre"},
}

var tablaIsapre = TablaDef{
	Codigo:      "isapre",
	Nombre:      "ISAPRE",
	Tabla:       "tab_isapre",
	Descripcion: "Instituciones de salud previsional.",
	Etiqueta:    "nombre",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("nombre", "Nombre", 40, true),
		campoTexto("rut", "RUT", 20, true),
		campoTexto("direccion", "Dirección", 40, true),
		campoReferencia("codigo_comuna", "Comuna", "tab_comuna", true, false),
		campoReferencia("codigo_ciudad", "Ciudad", "tab_ciudad", true, false),
		campoTexto("telefono", "Teléfono", 20, true),
		campoTexto("fax", "Fax", 20, false),
		campoTexto("contacto_apellido_paterno", "Contacto: apellido paterno", 20, false),
		campoTexto("contacto_apellido_materno", "Contacto: apellido materno", 20, false),
		campoTexto("contacto_nombres", "Contacto: nombres", 20, false),
		campoTexto("contacto_rut", "Contacto: RUT", 20, false),
		campoTexto("contacto_telefono", "Contacto: teléfono", 20, false),
	},
	Orden: []string{"nombre"},
}

var tablaCajaCompensacion = TablaDef{
	Codigo:      "caja-de-compensacion",
	Nombre:      "Caja de compensación",
	Tabla:       "tab_caja_compensacion",
	Descripcion: "Cajas de compensación de asignación familiar.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 3),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden: []string{"codigo"},
}

var tablaBancosSucursales = &TablaDef{
	Codigo:      "bancos-sucursales-hijas",
	Nombre:      "Sucursales",
	Tabla:       "tab_bancos_sucursales",
	Descripcion: "Sucursales del banco seleccionado.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		func() TablaCampo {
			c := campoReferencia("codigo_banco", "Banco", "tab_bancos", true, true)
			c.RefEtiqueta = "nombre"
			return c
		}(),
		campoClaveTexto("codigo", "Código de sucursal", 10),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden:      []string{"codigo"},
	CampoPadre: "codigo_banco",
}

var tablaBancos = TablaDef{
	Codigo:      "bancos-sucursales",
	Nombre:      "Bancos/Sucursales",
	Tabla:       "tab_bancos",
	Descripcion: "Bancos y sus sucursales.",
	Etiqueta:    "nombre",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 2),
		campoTexto("rut", "RUT", 20, true),
		campoTexto("nombre", "Nombre", 40, true),
		campoTexto("identificacion_superintendencia", "Identificación Superintendencia", 10, true),
		campoTexto("casilla", "Casilla", 10, false),
	},
	Orden:    []string{"codigo"},
	UsadaPor: []TablaUso{{Tabla: "tab_bancos_sucursales", Campo: "codigo_banco", Detalle: "sucursales"}},
	Hija:     tablaBancosSucursales,
}

var tablaImpuestoUnico = TablaDef{
	Codigo:      "impuesto-unico",
	Nombre:      "Impuesto único",
	Tabla:       "tab_impuesto_unico",
	Descripcion: "Tramos mensuales del impuesto único de segunda categoría.",
	Etiqueta:    "tramo",
	Campos: []TablaCampo{
		campoEntero("anio", "Año", 1900, 9999, true, true),
		campoEntero("mes", "Mes", 1, 12, true, true),
		campoEntero("tramo", "Tramo", 1, 999, true, true),
		campoDecimal("tope_minimo", "Tope mínimo", true, false),
		campoDecimal("tope_maximo", "Tope máximo", true, false),
		campoDecimal("porcentaje", "Porcentaje", true, false),
		campoDecimal("cantidad_rebajar", "Cantidad a rebajar", true, false),
	},
	Orden: []string{"anio", "mes", "tramo"},
}

var tablaBilletesMonedas = TablaDef{
	Codigo:      "billetes-monedas",
	Nombre:      "Detalle de billetes y/o monedas",
	Tabla:       "tab_billetes_monedas",
	Descripcion: "Denominaciones de billetes y monedas.",
	Etiqueta:    "valor",
	Campos: []TablaCampo{
		campoDecimal("valor", "Valor", true, true),
	},
	Orden: []string{"valor"},
}

var tablaIPC = TablaDef{
	Codigo:      "ipc",
	Nombre:      "I.P.C. mensual",
	Tabla:       "tab_ipc",
	Descripcion: "Factores mensuales del Índice de Precios al Consumidor.",
	Etiqueta:    "anio",
	Campos: []TablaCampo{
		campoEntero("anio", "Año", 1900, 9999, true, true),
		campoDecimal("factor_apertura", "Factor de apertura", false, false),
		campoDecimal("factor_enero", "Factor enero", false, false),
		campoDecimal("factor_febrero", "Factor febrero", false, false),
		campoDecimal("factor_marzo", "Factor marzo", false, false),
		campoDecimal("factor_abril", "Factor abril", false, false),
		campoDecimal("factor_mayo", "Factor mayo", false, false),
		campoDecimal("factor_junio", "Factor junio", false, false),
		campoDecimal("factor_julio", "Factor julio", false, false),
		campoDecimal("factor_agosto", "Factor agosto", false, false),
		campoDecimal("factor_septiembre", "Factor septiembre", false, false),
		campoDecimal("factor_octubre", "Factor octubre", false, false),
		campoDecimal("factor_noviembre", "Factor noviembre", false, false),
		campoDecimal("factor_diciembre", "Factor diciembre", false, false),
	},
	Orden: []string{"anio"},
}

var tablaMonedasEquivalencia = &TablaDef{
	Codigo:      "monedas-equivalencia-hijas",
	Nombre:      "Equivalencias",
	Tabla:       "tab_monedas_equivalencia",
	Descripcion: "Valores diarios de la moneda seleccionada.",
	Etiqueta:    "fecha",
	Campos: []TablaCampo{
		campoReferencia("codigo_moneda", "Moneda", "tab_monedas", true, true),
		campoFecha("fecha", "Fecha", true, true),
		campoDecimal("valor", "Valor", true, false),
	},
	Orden:      []string{"fecha"},
	CampoPadre: "codigo_moneda",
}

var tablaMonedas = TablaDef{
	Codigo:      "monedas",
	Nombre:      "Monedas y equivalencia",
	Tabla:       "tab_monedas",
	Descripcion: "Monedas y sus equivalencias diarias.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 2),
		campoTexto("descripcion", "Descripción", 60, true),
		campoTexto("simbolo", "Símbolo", 10, true),
		campoEntero("decimales_informe", "Decimales de informe", 0, 2, true, false),
		campoEntero("decimales_venta", "Decimales de venta", 0, 5, true, false),
		campoTexto("moneda_banco_central", "Moneda del Banco Central", 10, true),
	},
	Orden:    []string{"codigo"},
	UsadaPor: []TablaUso{{Tabla: "tab_monedas_equivalencia", Campo: "codigo_moneda", Detalle: "equivalencias"}},
	Hija:     tablaMonedasEquivalencia,
}

var tablaRelojes = TablaDef{
	Codigo:      "relojes",
	Nombre:      "Relojes",
	Tabla:       "tab_relojes",
	Descripcion: "Relojes controladores de asistencia.",
	Etiqueta:    "ubicacion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("ubicacion", "Ubicación", 60, true),
		campoTexto("puerta", "Puerta", 10, true),
		campoOpcion("digitos_anio", "Dígitos del año", true,
			OpcionCampo{Valor: "2", Etiqueta: "2 dígitos"},
			OpcionCampo{Valor: "4", Etiqueta: "4 dígitos"},
		),
	},
	Orden: []string{"codigo"},
}

var tablaFestivos = TablaDef{
	Codigo:      "festivos",
	Nombre:      "Festivos",
	Tabla:       "tab_festivos",
	Descripcion: "Días festivos del calendario.",
	Etiqueta:    "motivo",
	Campos: []TablaCampo{
		campoFecha("fecha", "Fecha", true, true),
		campoTexto("motivo", "Motivo", 60, true),
	},
	Orden: []string{"fecha"},
}

var tablaTiposAusentismos = TablaDef{
	Codigo:      "tipos-de-ausentismos",
	Nombre:      "Tipos de ausentismos justificados",
	Tabla:       "tab_tipos_ausentismos",
	Descripcion: "Motivos de ausentismo justificado.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden: []string{"codigo"},
}

var tablaTiposLicencias = TablaDef{
	Codigo:      "tipos-de-licencias",
	Nombre:      "Tipos de licencias médicas",
	Tabla:       "tab_tipos_licencias",
	Descripcion: "Tipos de licencia y su efecto en el cálculo.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 4),
		campoTexto("descripcion", "Descripción", 60, true),
		campoBooleano("descuenta_dias", "Descuenta días"),
		campoEntero("dias_descontar", "Días a descontar", 0, 100, true, false),
		campoEntero("descuenta_bajo_dias", "Descuenta bajo n días", 0, 1000, true, false),
		campoEntero("meses_base_imponible", "Meses de base imponible", 0, 23, true, false),
	},
	Orden: []string{"codigo"},
	Validar: func(v map[string]string) string {
		dias, errDias := atoiValidado(v["dias_descontar"])
		bajo, errBajo := atoiValidado(v["descuenta_bajo_dias"])
		if errDias == nil && errBajo == nil && bajo < dias {
			return "Descuenta bajo n días no puede ser menor que los días a descontar."
		}
		return ""
	},
}

var tablaEspecialidadesMedicas = TablaDef{
	Codigo:      "especialidades-medicas",
	Nombre:      "Especialidades médicas",
	Tabla:       "tab_especialidades_medicas",
	Descripcion: "Especialidades médicas reconocidas.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 15),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden: []string{"codigo"},
}

var tablaDiagnosticos = TablaDef{
	Codigo:      "diagnosticos",
	Nombre:      "Diagnósticos",
	Tabla:       "tab_diagnosticos",
	Descripcion: "Diagnósticos médicos.",
	Etiqueta:    "descripcion",
	Campos: []TablaCampo{
		campoClaveTexto("codigo", "Código", 15),
		campoTexto("descripcion", "Descripción", 60, true),
	},
	Orden: []string{"codigo"},
}

var tablasBases = []TablaDef{
	tablaCentrosCosto,
	tablaAreasNegocio,
	tablaAtributos,
	tablaINE,
	tablaCiudad,
	tablaComuna,
	tablaRegion,
	tablaProvincia,
	tablaTiposEventos,
	tablaEstudiosSuperiores,
	tablaCargos,
	tablaAFP,
	tablaIsapre,
	tablaCajaCompensacion,
	tablaBancos,
	tablaImpuestoUnico,
	tablaBilletesMonedas,
	tablaIPC,
	tablaMonedas,
	tablaRelojes,
	tablaFestivos,
	tablaTiposAusentismos,
	tablaTiposLicencias,
	tablaEspecialidadesMedicas,
	tablaDiagnosticos,
}

// Tablas devuelve los mantenedores del menú Bases, submenú Tablas.
func Tablas() []TablaDef {
	return tablasBases
}

// TablaPorCodigo busca un mantenedor por el código que aparece en la URL.
func TablaPorCodigo(codigo string) (TablaDef, bool) {
	for _, t := range tablasBases {
		if t.Codigo == codigo {
			return t, true
		}
	}
	return TablaDef{}, false
}

func atoiValidado(s string) (int, error) {
	return strconv.Atoi(s)
}
