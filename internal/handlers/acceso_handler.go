package handlers

import (
	"errors"
	"net"
	"net/http"
	"strconv"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

type AccesoHandler struct {
	repo         *repository.AccesoRepository
	templatesDir string
	sessions     *session.Manager
}

func NewAccesoHandler(repo *repository.AccesoRepository, templatesDir string, sessions *session.Manager) *AccesoHandler {
	return &AccesoHandler{repo: repo, templatesDir: templatesDir, sessions: sessions}
}

func (h *AccesoHandler) setFlash(data map[string]interface{}, ok string) {
	switch ok {
	case "perfil":
		data["Success"] = "Perfil guardado correctamente."
	case "eliminado":
		data["Success"] = "Perfil eliminado correctamente."
	case "usuario":
		data["Success"] = "Usuario creado correctamente."
	case "editado":
		data["Success"] = "Usuario actualizado correctamente."
	case "asignado":
		data["Success"] = "Perfiles del usuario actualizados."
	case "protegido":
		data["Error"] = "El perfil de administrador no se puede eliminar."
	case "ultimoadmin":
		data["Error"] = "Debe quedar al menos un usuario con acceso a administración."
	case "duplicado":
		data["Error"] = "Ya existe un registro con ese nombre o login."
	case "datos":
		data["Error"] = "Revise los datos: el login admite hasta 12 caracteres, la contraseña hasta 72 y el nombre es obligatorio."
	case "password_debil":
		data["Error"] = "La contraseña debe cumplir con la política de seguridad (mínimo 6 caracteres)."
	case "ambito":
		data["Error"] = "El empleado o su centro de responsabilidad queda fuera del ámbito del perfil. Oficina de Personal y Administración no tienen ese límite."
	case "ficha":
		data["Error"] = "Esa ficha ya tiene una cuenta de usuario."
	case "ficha_requerida":
		data["Error"] = "Una cuenta con acceso a formularios debe quedar ligada a un empleado."
	case "empleado":
		data["Error"] = "El empleado no existe o no está vigente."
	case "error":
		data["Error"] = "No se pudo completar la operación."
	}
}

func (h *AccesoHandler) codigoError(err error) string {
	switch {
	case errors.Is(err, repository.ErrPerfilProtegido):
		return "protegido"
	case errors.Is(err, repository.ErrUltimoAdmin):
		return "ultimoadmin"
	case errors.Is(err, repository.ErrDuplicado):
		return "duplicado"
	case errors.Is(err, repository.ErrDatos):
		return "datos"
	case errors.Is(err, repository.ErrPasswordDebil):
		return "password_debil"
	case errors.Is(err, repository.ErrAmbito):
		return "ambito"
	case errors.Is(err, repository.ErrFichaOcupada):
		return "ficha"
	case errors.Is(err, repository.ErrFichaRequerida):
		return "ficha_requerida"
	case errors.Is(err, repository.ErrEmpleado):
		return "empleado"
	default:
		return "error"
	}
}

func (h *AccesoHandler) MenuPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Administración de accesos"
	h.setFlash(data, r.URL.Query().Get("ok"))
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_menu.html", data)
}

func (h *AccesoHandler) PerfilesPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Perfiles de acceso"
	h.setFlash(data, r.URL.Query().Get("ok"))
	if perfiles, err := h.repo.Perfiles(); err == nil {
		data["Perfiles"] = perfiles
	} else {
		data["Error"] = "No se pudieron cargar los perfiles."
	}
	h.cargarPermisos(data, models.Perfil{})
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_perfiles.html", data)
}

func (h *AccesoHandler) PerfilNuevoPost(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/perfiles?ok=error", http.StatusSeeOther)
		return
	}
	user, _ := session.FromContext(r.Context())
	if _, err := h.repo.CrearPerfil(r.FormValue("nombre"), r.FormValue("descripcion"), r.Form["permisos"], enteros(r.Form["ambitos_depto"]), enteros(r.Form["ambitos_etapa"])); err != nil {
		http.Redirect(w, r, "/admin/perfiles?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
	_ = h.repo.Registrar(user.UserID, "perfil_creado", r.FormValue("nombre"), clientIP(r))
	http.Redirect(w, r, "/admin/perfiles?ok=perfil", http.StatusSeeOther)
}

func (h *AccesoHandler) PerfilPage(w http.ResponseWriter, r *http.Request, id int) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Editar perfil"
	h.setFlash(data, r.URL.Query().Get("ok"))
	perfil, err := h.repo.Perfil(id)
	if err != nil {
		http.Redirect(w, r, "/admin/perfiles?ok=error", http.StatusSeeOther)
		return
	}
	data["Perfil"] = perfil
	h.cargarPermisos(data, perfil)
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_perfil.html", data)
}

func (h *AccesoHandler) PerfilPost(w http.ResponseWriter, r *http.Request, id int) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, perfilURL(id, "error"), http.StatusSeeOther)
		return
	}
	user, _ := session.FromContext(r.Context())
	if err := h.repo.ActualizarPerfil(id, r.FormValue("nombre"), r.FormValue("descripcion"), r.Form["permisos"], enteros(r.Form["ambitos_depto"]), enteros(r.Form["ambitos_etapa"])); err != nil {
		http.Redirect(w, r, perfilURL(id, h.codigoError(err)), http.StatusSeeOther)
		return
	}
	_ = h.repo.Registrar(user.UserID, "perfil_actualizado", r.FormValue("nombre"), clientIP(r))
	http.Redirect(w, r, perfilURL(id, "perfil"), http.StatusSeeOther)
}

func (h *AccesoHandler) PerfilEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	user, _ := session.FromContext(r.Context())
	if err := h.repo.EliminarPerfil(id); err != nil {
		http.Redirect(w, r, "/admin/perfiles?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
	_ = h.repo.Registrar(user.UserID, "perfil_eliminado", strconv.Itoa(id), clientIP(r))
	http.Redirect(w, r, "/admin/perfiles?ok=eliminado", http.StatusSeeOther)
}

func (h *AccesoHandler) UsuariosPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Usuarios y perfiles"
	h.setFlash(data, r.URL.Query().Get("ok"))
	if usuarios, err := h.repo.Usuarios(); err == nil {
		data["Usuarios"] = usuarios
	} else {
		data["Error"] = "No se pudieron cargar los usuarios."
	}
	if perfiles, err := h.repo.Perfiles(); err == nil {
		data["Perfiles"] = perfiles
	}
	if empleados, err := h.repo.EmpleadosDirectorio(); err == nil {
		data["Empleados"] = empleados
	}
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_usuarios.html", data)
}

func (h *AccesoHandler) UsuarioNuevoPost(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok=error", http.StatusSeeOther)
		return
	}
	user, _ := session.FromContext(r.Context())
	ids := enteros(r.Form["perfiles"])
	idEmpleado, _ := strconv.Atoi(r.FormValue("id_empleado"))
	login := r.FormValue("login")
	if err := h.repo.CrearUsuario(login, r.FormValue("password"), r.FormValue("nombre"), idEmpleado, ids); err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
	_ = h.repo.Registrar(user.UserID, "usuario_creado", login, clientIP(r))
	http.Redirect(w, r, "/admin/usuarios?ok=usuario", http.StatusSeeOther)
}

func (h *AccesoHandler) UsuarioPost(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok=error", http.StatusSeeOther)
		return
	}
	user, _ := session.FromContext(r.Context())
	idEmpleado, _ := strconv.Atoi(r.FormValue("id_empleado"))
	estado := r.FormValue("estado")
	err := h.repo.ActualizarUsuario(
		id,
		r.FormValue("nombre"),
		estado,
		r.FormValue("password"),
		idEmpleado,
		enteros(r.Form["perfiles"]),
	)
	if err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
	if estado == "0" && h.sessions != nil {
		h.sessions.DeleteByUser(id)
		_ = h.repo.Registrar(user.UserID, "usuario_desactivado", id, clientIP(r))
	} else {
		accion := "usuario_actualizado"
		if r.FormValue("password") != "" {
			accion = "clave_cambiada"
		}
		_ = h.repo.Registrar(user.UserID, accion, id, clientIP(r))
	}
	http.Redirect(w, r, "/admin/usuarios?ok=editado", http.StatusSeeOther)
}

func (h *AccesoHandler) PermisosPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Permisos efectivos"
	if lista, err := h.repo.PermisosEfectivos(); err == nil {
		data["Efectivos"] = lista
	} else {
		data["Error"] = "No se pudieron cargar los permisos."
	}
	if lista, err := h.repo.QuienPuede(); err == nil {
		data["Cobertura"] = lista
	}
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_permisos.html", data)
}

func (h *AccesoHandler) AuditoriaPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Auditoría de accesos"
	if lista, err := h.repo.AuditoriaReciente(); err == nil {
		data["Eventos"] = lista
	} else {
		data["Error"] = "No se pudo cargar la auditoría."
	}
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_auditoria.html", data)
}

func (h *AccesoHandler) cargarPermisos(data map[string]interface{}, perfil models.Perfil) {
	data["Perfil"] = perfil
	permisos, err := h.repo.Permisos()
	if err != nil {
		data["Error"] = "No se pudieron cargar los permisos."
		return
	}
	mods, err := h.repo.Modulos()
	if err != nil {
		data["Error"] = "No se pudieron cargar los módulos."
		return
	}
	type grupo struct {
		Codigo   string
		Nombre   string
		Permisos []models.Permiso
	}
	var grupos []grupo
	for _, m := range mods {
		g := grupo{Codigo: m.Codigo, Nombre: m.Nombre}
		for _, p := range permisos {
			if p.Modulo == m.Codigo {
				g.Permisos = append(g.Permisos, p)
			}
		}
		grupos = append(grupos, g)
	}
	data["Grupos"] = grupos
	if deptos, err := h.repo.Departamentos(); err == nil {
		data["Departamentos"] = deptos
	}
	if etapas, err := h.repo.Etapas(); err == nil {
		data["Etapas"] = etapas
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *AccesoHandler) usuario(w http.ResponseWriter, r *http.Request) (session.Info, bool) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return session.Info{}, false
	}
	return user, true
}

func perfilURL(id int, ok string) string {
	return "/admin/perfiles/" + strconv.Itoa(id) + "?ok=" + ok
}

func enteros(vals []string) []int {
	var ids []int
	for _, v := range vals {
		n, err := strconv.Atoi(v)
		if err == nil {
			ids = append(ids, n)
		}
	}
	return ids
}
