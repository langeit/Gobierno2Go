package models

// Códigos de tipo de dominio que el mantenedor PHP usa como catálogo fijo.
const (
	CodTipoMaterial    = 1
	CodTipoEstadoSol   = 2
	CodTipoOrden       = 3
	CodTipoRegion      = 4
	CodTipoModoFactura = 6
	CodTipoModoOrden   = 7
	CodTipoImpuesto    = 8
)

// Estados de solicitud_materiales y modos de orden o factura del PHP.
const (
	EstadoSolPendienteEntrega = 5
	EstadoSolResuelta         = 6
	EstadoSolAprobacion       = 25
	TipoOrdenPedido           = 10
	TipoOrdenResExenta        = 12
	TipoOrdenResCompra        = 13
	ModoFacturaMateriales     = 14
	ModoFacturaServicios      = 15
	ModoOrdenMateriales       = 17
	ModoOrdenServicios        = 18
)

// TipoDominio agrupa valores de un catálogo (material, región, estado, etc.).
type TipoDominio struct {
	Codigo      int
	Descripcion string
}

// Dominio es un valor de un tipo de dominio.
type Dominio struct {
	Codigo          int
	CodTipo         int
	TipoDescripcion string
	Descripcion     string
}

// Material es un insumo del mantenedor de materiales.
type Material struct {
	Codigo       int
	Nombre       string
	Tipo         int
	TipoNombre   string
	StockCritico int
}

// Proveedor es un proveedor de adquisiciones.
type Proveedor struct {
	Rut            string
	Nombre         string
	NombreFantasia string
	Direccion      string
	Ciudad         string
	Region         int
	RegionNombre   string
	Fono           string
	Contacto       string
	Fax            string
	Mail           string
}

// Asignacion es una imputación presupuestaria.
type Asignacion struct {
	Correlativo int
	Codigo      string
	Descripcion string
}

// Pagina describe el paginado de un listado.
type Pagina struct {
	Actual  int
	Total   int
	Paginas []int
}

// LineaCantidad es un material con cantidad y, si corresponde, valor unitario.
type LineaCantidad struct {
	Correlativo        int
	CodMaterial        int
	Nombre             string
	Cantidad           int
	CantidadAutorizada int
	CantidadDespachada int
	ValorUnitario      int
	ValorTotal         int
	Facturado          bool
	Observacion        string
}

// ImputacionLinea es un monto cargado a una asignación presupuestaria.
type ImputacionLinea struct {
	Correlativo int
	CorrAsig    int
	Codigo      string
	Descripcion string
	Monto       float64
}

// OrdenEntrada son los datos que se guardan al crear o modificar una orden.
type OrdenEntrada struct {
	Numero            int
	TipoOrden         int
	RutProveedor      string
	IDUsuario         string
	ModoOrden         int
	TipoImpuesto      int
	DescServicio      string
	CantServicio      float64
	ValorUnitServicio float64
	Lineas            []LineaCantidad
	Imputaciones      []ImputacionLinea
}

// OrdenCompra es el encabezado y el detalle de una orden.
type OrdenCompra struct {
	Numero            int
	TipoOrden         int
	TipoNombre        string
	Fecha             string
	RutProveedor      string
	Proveedor         string
	IDUsuario         string
	Usuario           string
	Total             float64
	IVA               float64
	TotalFinal        float64
	DescServicio      string
	CantServicio      float64
	ValorUnitServicio float64
	TipoImpuesto      int
	ImpuestoNombre    string
	Porcentaje        float64
	ModoOrden         int
	ModoNombre        string
	TienePendientes   bool
	TieneFactura      bool
	ServicioFacturado bool
	Editable          bool
	Eliminable        bool
	Lineas            []LineaCantidad
	Imputaciones      []ImputacionLinea
}

// FacturaEntrada son los datos de un ingreso de factura.
type FacturaEntrada struct {
	NroOrden          int
	NroFactura        int
	FechaFactura      string
	Modo              int
	TipoImpuesto      int
	IDUsuario         string
	DescServicio      string
	CantServicio      float64
	ValorUnitServicio float64
	Lineas            []LineaCantidad
}

// Factura es una factura de proveedor asociada a una orden.
type Factura struct {
	Correlativo       int
	NroOrden          int
	RutProveedor      string
	Proveedor         string
	NroFactura        int
	FechaFactura      string
	FechaIngreso      string
	Total             float64
	IVA               float64
	TotalFinal        float64
	Modo              int
	ModoNombre        string
	DescServicio      string
	CantServicio      float64
	ValorUnitServicio float64
	TipoImpuesto      int
	ImpuestoNombre    string
	IDUsuario         string
	Lineas            []LineaCantidad
}

// SolicitudEntrada crea o reescribe las líneas de una solicitud.
type SolicitudEntrada struct {
	IDUsuario string
	IDDepto   int
	Lineas    []LineaCantidad
}

// SolicitudMaterial es el encabezado de un pedido interno.
type SolicitudMaterial struct {
	Numero       int
	Fecha        string
	Hora         string
	Estado       int
	EstadoNombre string
	IDUsuario    string
	Usuario      string
	IDDepto      int
	Departamento string
	VistoBueno   string
	FechaAprueba string
	Lineas       []LineaCantidad
}

// EntregaLinea es la cantidad despachada de una línea, con su contexto de stock.
type EntregaLinea struct {
	Correlativo   int
	CodMaterial   int
	Nombre        string
	Solicitada    int
	Stock         int
	Promedio      int
	EntregaAnio   int
	EntregaMesAnt int
}

// FiltroReporte agrupa los criterios de los listados del menú de reportes.
type FiltroReporte struct {
	Depto      int
	Tipo       int
	Material   int
	Fecha1     string
	Fecha2     string
	FechaCorte string
	Anio       int
	Mes        int
	Umbral     int
	UsaCritico bool
	Numero     int
	TipoOrden  int
}
