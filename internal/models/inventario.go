package models

import "time"

// TipoBien es un tipo/categoría de bien del inventario.
type TipoBien struct {
	CorrTbien   int
	Descripcion string
	Codigo      string
}

// Zona es una zona física donde se ubican los bienes.
type Zona struct {
	CorrZona    int
	Descripcion string
	Codigo      string
}

// Empleado es una persona del directorio.
type Empleado struct {
	IDEmpleado int
	Nombre     string
	Depto      string
}

// Bien es un bien inventariable con su ubicación y responsables vigentes.
type Bien struct {
	CorrBien         int
	CodigoBien       string
	CorrTbien        int
	TipoBien         string
	Descripcion      string
	Monto            float64
	Estado           string
	Cuenta           string
	NroDoctoCompra   string
	TipoDoctoCompra  string
	RutProveedor     string
	Piso             string
	Oficina          string
	Zona             string
	FechaUbicacion   *time.Time
	FechaAsignacion  *time.Time
	FechaAdquisicion *time.Time
	Responsables     string
}

// EstadoNombre devuelve la etiqueta del estado del bien.
func (b Bien) EstadoNombre() string {
	switch b.Estado {
	case "BU":
		return "Bueno en uso"
	case "BD":
		return "Bueno en desuso"
	case "MR":
		return "Malo recuperable"
	case "MI":
		return "Malo irrecuperable"
	default:
		return b.Estado
	}
}

// Movimiento es un movimiento de bienes (ingreso o traslado).
type Movimiento struct {
	CorrMovi int
	NumMovi  string
	Concepto string
	NumDoc   string
	TipoDoc  string
	Fecha    time.Time
	Hora     string
	Usuario  string
	Bienes   string
}

// ConceptoNombre devuelve la etiqueta del concepto de movimiento.
func (m Movimiento) ConceptoNombre() string {
	switch m.Concepto {
	case "I":
		return "Ingreso"
	case "T":
		return "Traslado"
	case "B":
		return "Baja"
	default:
		return m.Concepto
	}
}

// SolicitudInventario es una solicitud de reparación, devolución o petición de bienes.
type SolicitudInventario struct {
	CorrSolic       int
	FechaSolic      time.Time
	HoraSol         string
	TipoSol         string
	Descripcion     string
	EstadoSol       string
	UsuarioSol      string
	NombreUsuario   string
	Depto           string
	VBJefe          string
	FechaAprobJefe  *time.Time
	VBAdmin         string
	FechaAprobAdmin *time.Time
	Bienes          []Bien
}

// TipoNombre devuelve la etiqueta del tipo de solicitud.
func (s SolicitudInventario) TipoNombre() string {
	switch s.TipoSol {
	case "R":
		return "Reparación de bien"
	case "D":
		return "Devolución por no uso"
	case "P":
		return "Petición de bien"
	default:
		return s.TipoSol
	}
}

// EstadoNombre devuelve la etiqueta del estado de la solicitud.
func (s SolicitudInventario) EstadoNombre() string {
	switch s.EstadoSol {
	case "P":
		return "Pendiente jefe directo"
	case "C":
		return "Pendiente administración"
	case "A":
		return "Aprobada"
	case "R":
		return "Rechazada"
	default:
		return s.EstadoSol
	}
}

// Porcentaje es el porcentaje anual de actualización usado en la depreciación.
type Porcentaje struct {
	CorrPorcent int
	Valor       float64
	Anho        string
	FechaUpdate time.Time
}

// Depreciacion es una línea de depreciación anual de un bien.
type Depreciacion struct {
	CorrDepre    int
	Anho         string
	ValorPorcent float64
	CuotaActua   float64
	BienActua    float64
	AnoVidaUtil  int
	ActuaDepre   float64
	DepreActua   float64
	DepreAnho    float64
	DepreAcum    float64
	ValorLibro   float64
}

// AnhoDepreciacion registra un año en que se ejecutó la depreciación.
type AnhoDepreciacion struct {
	CorrAndep int
	Anho      string
	Fecha     time.Time
	Usuario   string
}

// IngresoItem es una fila del formulario de ingreso de bienes.
type IngresoItem struct {
	CorrTbien   int
	Descripcion string
	Monto       float64
	CorrZona    int
	Piso        string
	Oficina     string
	CorrAsig    int
	Fuente      int
}

// IngresoBien es la entrada completa para registrar una compra de bienes.
type IngresoBien struct {
	TipoDocCompra string
	NumDocCompra  int64
	TipoDocOrigen string
	NumDocOrigen  int64
	RutProveedor  string
	Items         []IngresoItem
}
