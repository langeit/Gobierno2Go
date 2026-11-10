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
	Nombres     string
	Usuarios    int
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

// UsuarioAcceso es un usuario con los perfiles que tiene asignados.
type UsuarioAcceso struct {
	IDUsuario  string
	Login      string
	Nombre     string
	Estado     string
	IDEmpleado int
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
