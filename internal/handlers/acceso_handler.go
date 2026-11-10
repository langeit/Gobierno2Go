package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

type AccesoHandler struct {
	repo         *repository.AccesoRepository
	templatesDir string
}

func NewAccesoHandler(repo *repository.AccesoRepository, templatesDir string) *AccesoHandler {
	return &AccesoHandler{repo: repo, templatesDir: templatesDir}
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
	if mods, err := h.repo.Modulos(); err == nil {
		data["Modulos"] = mods
	}
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
	if _, err := h.repo.CrearPerfil(r.FormValue("nombre"), r.FormValue("descripcion"), r.Form["modulos"]); err != nil {
		http.Redirect(w, r, "/admin/perfiles?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
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
	if mods, err := h.repo.Modulos(); err == nil {
		data["Modulos"] = mods
	}
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
	if err := h.repo.ActualizarPerfil(id, r.FormValue("nombre"), r.FormValue("descripcion"), r.Form["modulos"]); err != nil {
		http.Redirect(w, r, perfilURL(id, h.codigoError(err)), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, perfilURL(id, "perfil"), http.StatusSeeOther)
}

func (h *AccesoHandler) PerfilEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	if err := h.repo.EliminarPerfil(id); err != nil {
		http.Redirect(w, r, "/admin/perfiles?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
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
	ids := enteros(r.Form["perfiles"])
	if err := h.repo.CrearUsuario(r.FormValue("login"), r.FormValue("password"), r.FormValue("nombre"), ids); err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
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
	idEmpleado, _ := strconv.Atoi(r.FormValue("id_empleado"))
	err := h.repo.ActualizarUsuario(
		id,
		r.FormValue("nombre"),
		r.FormValue("estado"),
		r.FormValue("password"),
		idEmpleado,
		enteros(r.Form["perfiles"]),
	)
	if err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/usuarios?ok=editado", http.StatusSeeOther)
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
