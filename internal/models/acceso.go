package models

// Modulo es un área del sistema a la que un perfil puede dar acceso.
type Modulo struct {
	Codigo      string
	Nombre      string
	Descripcion string
	Orden       int
}

// Perfil agrupa módulos y se asigna a uno o más usuarios.
type Perfil struct {
	ID          int
	Nombre      string
	Descripcion string
	Protegido   bool
	Modulos     []string
	Permisos    []string
	Deptos      []int
	Etapas      []int
	Nombres     string
	Usuarios    int
}

// Permiso es una acción dentro de un módulo.
type Permiso struct {
	Codigo string
	Modulo string
	Accion string
	Nombre string
	Orden  int
}

// OpcionAmbito es un departamento o un centro de responsabilidad seleccionable.
type OpcionAmbito struct {
	ID     int
	Nombre string
}

// PermisoEfectivo resume lo que puede hacer un usuario.
type PermisoEfectivo struct {
	Usuario  string
	Nombre   string
	Permisos string
}

// PermisoCobertura lista quién puede ejecutar un proceso.
type PermisoCobertura struct {
	Codigo   string
	Modulo   string
	Nombre   string
	Usuarios string
}

// EventoAuditoria es un hecho registrado en la administración de accesos.
type EventoAuditoria struct {
	Fecha   string
	Usuario string
	Accion  string
	Detalle string
	IP      string
}

// Tiene indica si el perfil incluye el módulo indicado.
func (p Perfil) Tiene(codigo string) bool {
	for _, c := range p.Modulos {
		if c == codigo {
			return true
		}
	}
	return false
}

// TienePermiso indica si el perfil incluye el proceso indicado.
func (p Perfil) TienePermiso(codigo string) bool {
	for _, c := range p.Permisos {
		if c == codigo {
			return true
		}
	}
	return false
}

// TieneAmbito indica si el perfil limita ese departamento o centro.
func (p Perfil) TieneAmbito(tipo string, id int) bool {
	lista := p.Deptos
	if tipo == "etapa" {
		lista = p.Etapas
	}
	for _, n := range lista {
		if n == id {
			return true
		}
	}
	return false
}

// UsuarioAcceso es un usuario con los perfiles que tiene asignados.
type UsuarioAcceso struct {
	IDUsuario  string
	Login      string
	Nombre     string
	Estado     string
	IDEmpleado int
	Empleado   string
	PerfilIDs  []int
	Perfiles   string
}

// Activo indica si el usuario puede iniciar sesión.
func (u UsuarioAcceso) Activo() bool {
	return u.Estado == "1" || u.Estado == "t"
}

// TienePerfil indica si el usuario tiene asignado el perfil indicado.
func (u UsuarioAcceso) TienePerfil(id int) bool {
	for _, p := range u.PerfilIDs {
		if p == id {
			return true
		}
	}
	return false
}
