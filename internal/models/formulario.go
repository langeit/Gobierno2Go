package models

import "time"

// EmpleadoFormulario contiene los datos de un empleado y sus días disponibles.
type EmpleadoFormulario struct {
	IDEmpleado        int
	Nombre            string
	Depto             int
	DeptoNombre       string
	Email             string
	IDDias            int
	FeriadosAnuales   float64
	Administrativos   float64
	FeriadosUsados    float64
	AdminUsados       float64
	FeriadosRestantes float64
	AdminRestantes    float64
	Acumulados        float64
}

// SolicitudFormulario es una solicitud de feriado o permiso administrativo.
type SolicitudFormulario struct {
	IDDpermiso      int
	IDEmpleado      int
	Empleado        string
	Depto           string
	TipoPermiso     string
	DiasTomados     float64
	FechaInicio     time.Time
	FechaFin        time.Time
	Anho            int
	VBJefe          string
	FechaSolicitud  time.Time
	FechaResolucion *time.Time
	FechaPosterga   *time.Time
	MDia            string
	MotivoRechazo   string
	Acumulado       bool
}

// Estado devuelve la etiqueta de estado de una solicitud.
func (s SolicitudFormulario) Estado() string {
	if s.VBJefe == "S" {
		return "Aprobada"
	}
	if s.FechaResolucion != nil || s.MotivoRechazo != "" {
		return "Rechazada"
	}
	return "Pendiente"
}

// TipoNombre devuelve el nombre legible del tipo de permiso.
func (s SolicitudFormulario) TipoNombre() string {
	if s.TipoPermiso == "A" {
		return "Administrativo"
	}
	return "Feriado legal"
}

// AcumulacionFormulario es una solicitud de acumulación de feriado.
type AcumulacionFormulario struct {
	Correlativo int
	IDDpermiso  int
	Empleado    string
	Depto       string
	DiasSol     int
	VBPersonal  bool
	FechaSol    time.Time
	FechaVB     *time.Time
	TipoPermiso string
	FechaInicio time.Time
	FechaFin    time.Time
}

// JefeDepto es la configuración de jefe directo y aprobador de un departamento.
type JefeDepto struct {
	IDDepto     int
	DeptoNombre string
	IDEmpleado  int
	Empleado    string
	IDAprueba   int
	Aprueba     string
}

// CrearSolicitudFormulario es la entrada para registrar una solicitud.
type CrearSolicitudFormulario struct {
	IDEmpleado  int
	DiasTomados float64
	FechaInicio time.Time
	FechaFin    time.Time
	TipoPermiso string
	VBJefe      string
	MDia        string
}

// Departamento representa una unidad o departamento organizativo.
type Departamento struct {
	IDDepto     int
	Descripcion string
}

// EmpleadoCRUD representa los datos de un trabajador para el mantenedor CRUD.
type EmpleadoCRUD struct {
	IDEmpleado  int
	Nombre      string
	Apellido    string
	Email       string
	Depto       int
	DeptoNombre string
}
