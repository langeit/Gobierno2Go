package handlers

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
	case "usuario_eliminado":
		data["Success"] = "Usuario eliminado correctamente."
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
	base := "/admin/perfiles"
	busqueda, orden, ascendente, pagina := parametrosLista(r, "nombre", "modulos", "usuarios")
	perfiles, total, pagina, err := h.repo.PerfilesPagina(busqueda, orden, ascendente, pagina, registrosPorPagina)

	data := pageData(r)
	data["Title"] = "Perfiles de acceso"
	h.setFlash(data, r.URL.Query().Get("ok"))
	if err != nil {
		data["Error"] = "No se pudieron cargar los perfiles."
	} else {
		data["Perfiles"] = perfiles
	}
	ordenEfectivo := orden
	if ordenEfectivo == "" {
		ordenEfectivo = "nombre"
	}
	data["Columnas"] = columnasAcceso(base, busqueda, ordenEfectivo, ascendente, []columnaAcceso{
		{"nombre", "Perfil"},
		{"modulos", "Módulos"},
		{"usuarios", "Usuarios"},
	})
	data["Busqueda"] = busqueda
	data["ListaURL"] = base
	data["LimpiarURL"] = base
	data["Paginacion"] = paginacionAcceso(base, busqueda, ordenEfectivo, ascendente, pagina, total, len(perfiles))
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_perfiles.html", data)
}

func (h *AccesoHandler) PerfilNuevoPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Nuevo perfil"
	h.setFlash(data, r.URL.Query().Get("ok"))
	data["Perfil"] = models.Perfil{}
	h.cargarPermisos(data, models.Perfil{})
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_perfil_nuevo.html", data)
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
		http.Redirect(w, r, "/admin/perfiles/nuevo?ok="+h.codigoError(err), http.StatusSeeOther)
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
	base := "/admin/usuarios"
	busqueda, orden, ascendente, pagina := parametrosLista(r, "login", "nombre", "estado", "empleado", "perfiles")
	usuarios, total, pagina, err := h.repo.UsuariosPagina(busqueda, orden, ascendente, pagina, registrosPorPagina)

	data := pageData(r)
	data["Title"] = "Usuarios"
	h.setFlash(data, r.URL.Query().Get("ok"))
	if err != nil {
		data["Error"] = "No se pudieron cargar los usuarios."
	} else {
		data["Usuarios"] = usuarios
	}
	ordenEfectivo := orden
	if ordenEfectivo == "" {
		ordenEfectivo = "nombre"
	}
	data["Columnas"] = columnasAcceso(base, busqueda, ordenEfectivo, ascendente, []columnaAcceso{
		{"login", "Login"},
		{"nombre", "Nombre"},
		{"estado", "Estado"},
		{"empleado", "Empleado"},
		{"perfiles", "Perfiles"},
	})
	data["Busqueda"] = busqueda
	data["ListaURL"] = base
	data["LimpiarURL"] = base
	data["Paginacion"] = paginacionAcceso(base, busqueda, ordenEfectivo, ascendente, pagina, total, len(usuarios))
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_usuarios.html", data)
}

func (h *AccesoHandler) UsuarioNuevoPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	h.renderUsuarioForm(w, r, models.UsuarioAcceso{Estado: "1"}, false, "")
}

func (h *AccesoHandler) UsuarioEditarPage(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	usuario, err := h.repo.Usuario(id)
	if err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok=error", http.StatusSeeOther)
		return
	}
	h.renderUsuarioForm(w, r, usuario, true, "")
}

func (h *AccesoHandler) renderUsuarioForm(w http.ResponseWriter, r *http.Request, usuario models.UsuarioAcceso, editar bool, ok string) {
	data := pageData(r)
	if editar {
		data["Title"] = "Editar usuario"
		data["Accion"] = "/admin/usuarios/" + url.PathEscape(usuario.IDUsuario)
	} else {
		data["Title"] = "Nuevo usuario"
		data["Accion"] = "/admin/usuarios"
	}
	data["Usuario"] = usuario
	data["Editar"] = editar
	data["VolverURL"] = "/admin/usuarios"
	h.setFlash(data, ok)
	if perfiles, err := h.repo.Perfiles(); err == nil {
		data["Perfiles"] = perfiles
	} else if _, existe := data["Error"]; !existe {
		data["Error"] = "No se pudieron cargar los perfiles."
	}
	if empleados, err := h.repo.EmpleadosDirectorio(); err == nil {
		data["Empleados"] = empleados
	}
	renderPage(w, h.templatesDir, "adm_layout.html", "adm_layout", "adm_usuario_form.html", data)
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
	idEmpleado, _ := strconv.Atoi(r.FormValue("id_empleado"))
	usuario := models.UsuarioAcceso{
		Login:      strings.TrimSpace(r.FormValue("login")),
		Nombre:     strings.TrimSpace(r.FormValue("nombre")),
		Estado:     "1",
		IDEmpleado: idEmpleado,
		PerfilIDs:  enteros(r.Form["perfiles"]),
	}
	if err := h.repo.CrearUsuario(usuario.Login, r.FormValue("password"), usuario.Nombre, idEmpleado, usuario.PerfilIDs); err != nil {
		h.renderUsuarioForm(w, r, usuario, false, h.codigoError(err))
		return
	}
	_ = h.repo.Registrar(user.UserID, "usuario_creado", usuario.Login, clientIP(r))
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
	usuario := models.UsuarioAcceso{
		IDUsuario:  id,
		Login:      strings.TrimSpace(r.FormValue("login")),
		Nombre:     strings.TrimSpace(r.FormValue("nombre")),
		Estado:     strings.TrimSpace(r.FormValue("estado")),
		IDEmpleado: idEmpleado,
		PerfilIDs:  enteros(r.Form["perfiles"]),
	}
	if usuario.Login == "" {
		if actual, err := h.repo.Usuario(id); err == nil {
			usuario.Login = actual.Login
		}
	}
	err := h.repo.ActualizarUsuario(
		id,
		usuario.Nombre,
		usuario.Estado,
		r.FormValue("password"),
		idEmpleado,
		usuario.PerfilIDs,
	)
	if err != nil {
		h.renderUsuarioForm(w, r, usuario, true, h.codigoError(err))
		return
	}
	if usuario.Estado == "0" && h.sessions != nil {
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

func (h *AccesoHandler) UsuarioEliminarPost(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := h.usuario(w, r); !ok {
		return
	}
	user, _ := session.FromContext(r.Context())
	if err := h.repo.EliminarUsuario(id); err != nil {
		http.Redirect(w, r, "/admin/usuarios?ok="+h.codigoError(err), http.StatusSeeOther)
		return
	}
	if h.sessions != nil {
		h.sessions.DeleteByUser(id)
	}
	_ = h.repo.Registrar(user.UserID, "usuario_eliminado", id, clientIP(r))
	http.Redirect(w, r, "/admin/usuarios?ok=usuario_eliminado", http.StatusSeeOther)
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

// columnaAcceso describe una columna ordenable de un listado de acceso.
type columnaAcceso struct {
	Clave    string
	Etiqueta string
}

// parametrosLista lee la búsqueda, el orden y la página de la URL, descartando
// claves de orden que no estén permitidas para el listado.
func parametrosLista(r *http.Request, claves ...string) (string, string, bool, int) {
	busqueda := strings.TrimSpace(r.URL.Query().Get("q"))
	orden := r.URL.Query().Get("orden")
	valido := false
	for _, c := range claves {
		if c == orden {
			valido = true
			break
		}
	}
	if !valido {
		orden = ""
	}
	ascendente := r.URL.Query().Get("dir") != "desc"
	pagina := 1
	if n, err := strconv.Atoi(r.URL.Query().Get("pagina")); err == nil && n > 0 {
		pagina = n
	}
	return busqueda, orden, ascendente, pagina
}

// urlAcceso arma un enlace del listado conservando búsqueda, orden y página.
func urlAcceso(base, busqueda, orden string, ascendente bool, pagina int) string {
	valores := url.Values{}
	if busqueda != "" {
		valores.Set("q", busqueda)
	}
	if orden != "" {
		valores.Set("orden", orden)
		if ascendente {
			valores.Set("dir", "asc")
		} else {
			valores.Set("dir", "desc")
		}
	}
	if pagina > 1 {
		valores.Set("pagina", strconv.Itoa(pagina))
	}
	if len(valores) == 0 {
		return base
	}
	return base + "?" + valores.Encode()
}

// columnasAcceso genera los encabezados ordenables de un listado de acceso.
func columnasAcceso(base, busqueda, ordenEfectivo string, ascendente bool, columnas []columnaAcceso) map[string]columnaVista {
	vistas := make(map[string]columnaVista, len(columnas))
	for _, c := range columnas {
		activa := c.Clave == ordenEfectivo
		nuevaAscendente := true
		if activa && ascendente {
			nuevaAscendente = false
		}
		vistas[c.Clave] = columnaVista{
			Etiqueta:   c.Etiqueta,
			OrdenURL:   urlAcceso(base, busqueda, c.Clave, nuevaAscendente, 1),
			Activa:     activa,
			Ascendente: ascendente,
		}
	}
	return vistas
}

// paginacionAcceso genera los enlaces y contadores del paginador.
func paginacionAcceso(base, busqueda, orden string, ascendente bool, pagina, total, mostrados int) paginacionVista {
	totalPaginas := (total + registrosPorPagina - 1) / registrosPorPagina
	if totalPaginas < 1 {
		totalPaginas = 1
	}
	desde, hasta := 0, 0
	if total > 0 {
		desde = (pagina-1)*registrosPorPagina + 1
		hasta = desde + mostrados - 1
	}
	paginacion := paginacionVista{
		Pagina:       pagina,
		TotalPaginas: totalPaginas,
		Total:        total,
		Desde:        desde,
		Hasta:        hasta,
	}
	if pagina > 1 {
		paginacion.AnteriorURL = urlAcceso(base, busqueda, orden, ascendente, pagina-1)
	}
	if pagina < totalPaginas {
		paginacion.SiguienteURL = urlAcceso(base, busqueda, orden, ascendente, pagina+1)
	}
	inicio := pagina - 2
	if inicio < 1 {
		inicio = 1
	}
	fin := inicio + 4
	if fin > totalPaginas {
		fin = totalPaginas
	}
	if inicio = fin - 4; inicio < 1 {
		inicio = 1
	}
	for i := inicio; i <= fin; i++ {
		paginacion.Paginas = append(paginacion.Paginas, paginaVista{
			Numero: i,
			URL:    urlAcceso(base, busqueda, orden, ascendente, i),
			Activa: i == pagina,
		})
	}
	return paginacion
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
