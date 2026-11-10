package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

// FormularioHandler atiende el módulo de formularios y RRHH.
type FormularioHandler struct {
	repo         *repository.FormularioRepository
	templatesDir string
}

func NewFormularioHandler(repo *repository.FormularioRepository, templatesDir string) *FormularioHandler {
	return &FormularioHandler{repo: repo, templatesDir: templatesDir}
}

// ---------- Menú ----------

func (h *FormularioHandler) MenuPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	anho := time.Now().Year()
	data := pageData(r)
	data["Title"] = "Formularios y RRHH"
	data["Anho"] = anho
	setFlash(data, r.URL.Query().Get("ok"))

	idEmpleado, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil {
		data["Error"] = "No se pudo identificar al usuario: " + err.Error()
	} else {
		data["EsPersonal"] = esPersonal
		data["EsAdmin"] = esAdmin
	}
	data["PuedeAdmin"] = esPersonal || esAdmin

	if idEmpleado > 0 {
		if emp, err := h.repo.EmpleadoPorID(idEmpleado, anho); err == nil {
			data["Empleado"] = emp
		}
	}
	if pendientes, err := h.repo.SolicitudesPendientesAprobador(user.UserID); err == nil {
		data["PendientesAprobacion"] = len(pendientes)
	}
	if propias, err := h.repo.Solicitudes(anho, idEmpleado, false); err == nil {
		data["MisSolicitudes"] = len(propias)
		data["Solicitudes"] = propias
	}

	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_menu.html", data)
}

// ---------- Solicitudes propias ----------

func (h *FormularioHandler) FeriadoPage(w http.ResponseWriter, r *http.Request) {
	h.renderSolicitud(w, r, "F", false, "")
}

func (h *FormularioHandler) FeriadoPost(w http.ResponseWriter, r *http.Request) {
	h.crearSolicitud(w, r, "F", false)
}

func (h *FormularioHandler) AdministrativoPage(w http.ResponseWriter, r *http.Request) {
	h.renderSolicitud(w, r, "A", false, "")
}

func (h *FormularioHandler) AdministrativoPost(w http.ResponseWriter, r *http.Request) {
	h.crearSolicitud(w, r, "A", false)
}

// ---------- Registro por Oficina de Personal ----------

func (h *FormularioHandler) PersonalFeriadoPage(w http.ResponseWriter, r *http.Request) {
	h.renderSolicitud(w, r, "F", true, "")
}

func (h *FormularioHandler) PersonalFeriadoPost(w http.ResponseWriter, r *http.Request) {
	h.crearSolicitud(w, r, "F", true)
}

func (h *FormularioHandler) PersonalAdministrativoPage(w http.ResponseWriter, r *http.Request) {
	h.renderSolicitud(w, r, "A", true, "")
}

func (h *FormularioHandler) PersonalAdministrativoPost(w http.ResponseWriter, r *http.Request) {
	h.crearSolicitud(w, r, "A", true)
}

func (h *FormularioHandler) renderSolicitud(w http.ResponseWriter, r *http.Request, tipo string, personal bool, errMsg string) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	anho := time.Now().Year()
	data := pageData(r)
	data["Title"] = "Solicitud de " + tipoNombre(tipo)
	data["Tipo"] = tipo
	data["TipoNombre"] = tipoNombre(tipo)
	data["Personal"] = personal
	data["Error"] = errMsg
	data["Anho"] = anho
	data["Hoy"] = time.Now().Format("2006-01-02")

	_, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil {
		data["Error"] = "No se pudo validar el usuario: " + err.Error()
		renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_solicitud.html", data)
		return
	}
	data["PuedeAdmin"] = esPersonal || esAdmin

	if personal && !(esPersonal || esAdmin) {
		data["Error"] = "Sólo la Oficina de Personal puede registrar solicitudes para otros empleados."
		renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_solicitud.html", data)
		return
	}

	if emp, err := h.repo.EmpleadoDeUsuario(user.UserID, anho); err == nil {
		data["Empleado"] = emp
	} else if !personal {
		if errors.Is(err, sql.ErrNoRows) {
			data["Error"] = "El usuario no tiene un empleado asociado en el directorio."
		} else {
			data["Error"] = "No se pudo cargar el empleado asociado: " + err.Error()
		}
	}

	empleados, err := h.repo.Empleados(anho)
	if err != nil {
		data["Error"] = "No se pudieron cargar los empleados: " + err.Error()
	}
	data["Empleados"] = empleados

	if idStr := r.URL.Query().Get("empleado"); idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			if emp, err := h.repo.EmpleadoPorID(id, anho); err == nil {
				data["Seleccionado"] = emp
			}
		}
	}

	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_solicitud.html", data)
}

func (h *FormularioHandler) crearSolicitud(w http.ResponseWriter, r *http.Request, tipo string, personal bool) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	anho := time.Now().Year()
	if err := r.ParseForm(); err != nil {
		h.renderSolicitud(w, r, tipo, personal, "Datos del formulario inválidos.")
		return
	}

	_, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil {
		h.renderSolicitud(w, r, tipo, personal, "No se pudo validar el usuario.")
		return
	}
	puedePersonal := esPersonal || esAdmin
	if personal && !puedePersonal {
		h.renderSolicitud(w, r, tipo, personal, "Sólo la Oficina de Personal puede registrar solicitudes para otros empleados.")
		return
	}

	var idEmpleado int
	if personal {
		idEmpleado, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("id_empleado")))
	} else {
		emp, err := h.repo.EmpleadoDeUsuario(user.UserID, anho)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.renderSolicitud(w, r, tipo, personal, "El usuario no tiene un empleado asociado en el directorio.")
			} else {
				h.renderSolicitud(w, r, tipo, personal, "No se pudo cargar el empleado asociado: "+err.Error())
			}
			return
		}
		idEmpleado = emp.IDEmpleado
	}
	if idEmpleado <= 0 {
		h.renderSolicitud(w, r, tipo, personal, "Debe seleccionar un empleado.")
		return
	}

	emp, err := h.repo.EmpleadoPorID(idEmpleado, anho)
	if err != nil {
		h.renderSolicitud(w, r, tipo, personal, "El empleado seleccionado no existe.")
		return
	}

	inicio, err := parseFecha(r.FormValue("fecha_inicio"))
	if err != nil {
		h.renderSolicitud(w, r, tipo, personal, "La fecha de inicio es inválida.")
		return
	}
	fin, err := parseFecha(r.FormValue("fecha_fin"))
	if err != nil {
		h.renderSolicitud(w, r, tipo, personal, "La fecha de término es inválida.")
		return
	}
	if fin.Before(inicio) {
		h.renderSolicitud(w, r, tipo, personal, "La fecha de término no puede ser anterior a la de inicio.")
		return
	}

	feriados, err := h.repo.Feriados(anho)
	if err != nil {
		h.renderSolicitud(w, r, tipo, personal, "No se pudo cargar el calendario de feriados.")
		return
	}
	inicio = ajustarInicioHabil(inicio, feriados)
	fin = ajustarFinHabil(fin, feriados)
	if fin.Before(inicio) {
		h.renderSolicitud(w, r, tipo, personal, "El rango de fechas no contiene días hábiles.")
		return
	}
	dias := calcularDiasHabiles(inicio, fin, feriados)
	if dias <= 0 {
		h.renderSolicitud(w, r, tipo, personal, "El rango de fechas no contiene días hábiles.")
		return
	}

	if tipo == "F" && dias > emp.FeriadosRestantes {
		h.renderSolicitud(w, r, tipo, personal, fmt.Sprintf("No tiene suficientes días de feriado. Disponibles: %.1f, solicitados: %.1f.", emp.FeriadosRestantes, dias))
		return
	}
	if tipo == "A" && dias > emp.AdminRestantes {
		h.renderSolicitud(w, r, tipo, personal, fmt.Sprintf("No tiene suficientes días administrativos. Disponibles: %.1f, solicitados: %.1f.", emp.AdminRestantes, dias))
		return
	}

	vb := "N"
	if personal {
		vb = "S"
	}
	id, err := h.repo.CrearSolicitud(models.CrearSolicitudFormulario{
		IDEmpleado:  idEmpleado,
		DiasTomados: dias,
		FechaInicio: inicio,
		FechaFin:    fin,
		TipoPermiso: tipo,
		VBJefe:      vb,
		MDia:        r.FormValue("mdia"),
	}, anho)
	if err != nil {
		h.renderSolicitud(w, r, tipo, personal, "No se pudo registrar la solicitud: "+err.Error())
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/formularios/listado?ok=creado&id=%d", id), http.StatusSeeOther)
}

// ---------- Listado ----------

func (h *FormularioHandler) ListadoPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	anho := time.Now().Year()
	data := pageData(r)
	data["Title"] = "Listado de Feriados y Facultativos"
	data["Anho"] = anho
	setFlash(data, r.URL.Query().Get("ok"))

	idEmpleado, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil {
		data["Error"] = "No se pudo validar el usuario."
	}
	data["PuedeAdmin"] = esPersonal || esAdmin

	filtroEmpleado := 0
	if !esPersonal && !esAdmin {
		filtroEmpleado = idEmpleado
		if filtroEmpleado == 0 {
			filtroEmpleado = -1
		}
	} else if idStr := r.URL.Query().Get("empleado"); idStr != "" {
		filtroEmpleado, _ = strconv.Atoi(idStr)
	}

	solicitudes, err := h.repo.Solicitudes(anho, filtroEmpleado, false)
	if err != nil {
		data["Error"] = "No se pudo cargar el listado: " + err.Error()
	}
	data["Solicitudes"] = solicitudes

	empleados, _ := h.repo.Empleados(anho)
	data["Empleados"] = empleados
	data["FiltroEmpleado"] = filtroEmpleado
	if filtroEmpleado > 0 {
		data["FiltroEmpleado"] = filtroEmpleado
	}
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_listado.html", data)
}

func (h *FormularioHandler) EliminarPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/listado?ok=error", http.StatusSeeOther)
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	if id <= 0 {
		http.Redirect(w, r, "/formularios/listado?ok=error", http.StatusSeeOther)
		return
	}

	idEmpleado, _, esPersonal, esAdmin, _ := h.repo.UsuarioInfo(user.UserID)
	if !esPersonal && !esAdmin {
		sol, err := h.repo.SolicitudPorID(id)
		if err != nil || sol.IDEmpleado != idEmpleado {
			http.Redirect(w, r, "/formularios/listado?ok=sinpermiso", http.StatusSeeOther)
			return
		}
	}
	if err := h.repo.EliminarSolicitud(id); err != nil {
		http.Redirect(w, r, "/formularios/listado?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/listado?ok=eliminado", http.StatusSeeOther)
}

// ---------- Aprobaciones ----------

func (h *FormularioHandler) AprobacionesPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Aprobaciones"
	setFlash(data, r.URL.Query().Get("ok"))
	if _, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID); err == nil {
		data["PuedeAdmin"] = esPersonal || esAdmin
		data["EsPersonal"] = esPersonal
	}

	solicitudes, err := h.repo.SolicitudesPendientesAprobador(user.UserID)
	if err != nil {
		data["Error"] = "No se pudieron cargar las solicitudes: " + err.Error()
	}
	data["Solicitudes"] = solicitudes

	acumulaciones, err := h.repo.Acumulaciones(0, true)
	if err != nil {
		data["Error"] = "No se pudieron cargar las acumulaciones: " + err.Error()
	}
	if idEmpleado, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID); err == nil {
		if !esPersonal && !esAdmin {
			// Los jefes sólo ven las acumulaciones de su departamento.
			acumulaciones = nil
		}
		data["AprobadorEmpleado"] = idEmpleado
	}
	data["Acumulaciones"] = acumulaciones

	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_aprobaciones.html", data)
}

func (h *FormularioHandler) AprobacionesPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=error", http.StatusSeeOther)
		return
	}
	id, _ := strconv.Atoi(r.FormValue("id"))
	accion := r.FormValue("accion")
	aprobar := accion == "aprobar"

	puede, err := h.repo.PuedeResolver(user.UserID, id)
	if err != nil || !puede {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=sinpermiso", http.StatusSeeOther)
		return
	}
	if err := h.repo.ResolverSolicitud(id, aprobar, r.FormValue("motivo_rechazo")); err != nil {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/aprobaciones?ok=resuelto", http.StatusSeeOther)
}

// ---------- Acumulación ----------

func (h *FormularioHandler) AcumulacionPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	anho := time.Now().Year()
	data := pageData(r)
	data["Title"] = "Acumulación de Feriado"
	data["Anho"] = anho
	setFlash(data, r.URL.Query().Get("ok"))

	idEmpleado, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil {
		data["Error"] = "No se pudo validar el usuario."
	}
	data["PuedeAdmin"] = esPersonal || esAdmin

	solicitudes, err := h.repo.Solicitudes(anho, idEmpleado, false)
	if err != nil {
		data["Error"] = "No se pudieron cargar sus solicitudes."
	}
	if idEmpleado == 0 {
		solicitudes = nil
	}
	// Sólo las solicitudes aprobadas pueden acumularse.
	var aprobadas []models.SolicitudFormulario
	for _, s := range solicitudes {
		if s.VBJefe == "S" {
			aprobadas = append(aprobadas, s)
		}
	}
	data["Solicitudes"] = aprobadas

	acumulaciones, _ := h.repo.Acumulaciones(idEmpleado, false)
	data["Acumulaciones"] = acumulaciones

	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_acumulacion.html", data)
}

func (h *FormularioHandler) AcumulacionPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/acumulacion?ok=error", http.StatusSeeOther)
		return
	}
	idSolicitud, _ := strconv.Atoi(r.FormValue("id_dpermiso"))
	dias, _ := strconv.Atoi(r.FormValue("dias_sol"))
	if idSolicitud <= 0 || dias <= 0 {
		http.Redirect(w, r, "/formularios/acumulacion?ok=error", http.StatusSeeOther)
		return
	}

	sol, err := h.repo.SolicitudPorID(idSolicitud)
	if err != nil || sol.VBJefe != "S" {
		http.Redirect(w, r, "/formularios/acumulacion?ok=error", http.StatusSeeOther)
		return
	}
	idEmpleado, _, esPersonal, esAdmin, _ := h.repo.UsuarioInfo(user.UserID)
	if !esPersonal && !esAdmin && sol.IDEmpleado != idEmpleado {
		http.Redirect(w, r, "/formularios/acumulacion?ok=sinpermiso", http.StatusSeeOther)
		return
	}
	if dias > int(sol.DiasTomados) {
		http.Redirect(w, r, "/formularios/acumulacion?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.CrearAcumulacion(idSolicitud, dias); err != nil {
		http.Redirect(w, r, "/formularios/acumulacion?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/acumulacion?ok=acumulacion", http.StatusSeeOther)
}

func (h *FormularioHandler) AcumulacionAprobarPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	_, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil || (!esPersonal && !esAdmin) {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=sinpermiso", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=error", http.StatusSeeOther)
		return
	}
	correlativo, _ := strconv.Atoi(r.FormValue("correlativo"))
	aprobar := r.FormValue("accion") == "aprobar"
	if correlativo <= 0 {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.ResolverAcumulacion(correlativo, aprobar); err != nil {
		http.Redirect(w, r, "/formularios/aprobaciones?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/aprobaciones?ok=resuelto", http.StatusSeeOther)
}

// ---------- Administración ----------

func (h *FormularioHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Administración de Formularios"
	setFlash(data, r.URL.Query().Get("ok"))
	data["PuedeAdmin"] = true
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_admin.html", data)
}

func (h *FormularioHandler) AsignacionPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	anho := time.Now().Year()
	data := pageData(r)
	data["Title"] = "Asignación de Feriados y Facultativos"
	data["Anho"] = anho
	setFlash(data, r.URL.Query().Get("ok"))
	data["PuedeAdmin"] = true
	empleados, err := h.repo.Empleados(anho)
	if err != nil {
		data["Error"] = "No se pudieron cargar los empleados: " + err.Error()
	}
	data["Empleados"] = empleados
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_asignacion.html", data)
}

func (h *FormularioHandler) AsignacionPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/admin/asignacion?ok=error", http.StatusSeeOther)
		return
	}
	anho := time.Now().Year()
	empleados, err := h.repo.Empleados(anho)
	if err != nil {
		http.Redirect(w, r, "/formularios/admin/asignacion?ok=error", http.StatusSeeOther)
		return
	}
	for _, emp := range empleados {
		id := strconv.Itoa(emp.IDEmpleado)
		feriados := parseFloat(r.FormValue("feriados_" + id))
		admin := parseFloat(r.FormValue("administrativos_" + id))
		if err := h.repo.ActualizarDiasEmpleado(emp.IDEmpleado, feriados, admin); err != nil {
			http.Redirect(w, r, "/formularios/admin/asignacion?ok=error", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/formularios/admin/asignacion?ok=asignado", http.StatusSeeOther)
}

func (h *FormularioHandler) JefesPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	anho := time.Now().Year()
	data := pageData(r)
	data["Title"] = "Configuración de Jefes de Departamento"
	setFlash(data, r.URL.Query().Get("ok"))
	data["PuedeAdmin"] = true
	jefes, err := h.repo.Jefes()
	if err != nil {
		data["Error"] = "No se pudo cargar la configuración: " + err.Error()
	}
	data["Jefes"] = jefes
	empleados, _ := h.repo.Empleados(anho)
	data["Empleados"] = empleados
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_jefes.html", data)
}

func (h *FormularioHandler) JefesPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/admin/jefes?ok=error", http.StatusSeeOther)
		return
	}
	config := make(map[int][2]int)
	for key, values := range r.Form {
		if !strings.HasPrefix(key, "jefe_") || len(values) == 0 {
			continue
		}
		idDepto, err := strconv.Atoi(strings.TrimPrefix(key, "jefe_"))
		if err != nil {
			continue
		}
		idJefe, _ := strconv.Atoi(values[0])
		idAprueba, _ := strconv.Atoi(r.FormValue("aprueba_" + strconv.Itoa(idDepto)))
		config[idDepto] = [2]int{idJefe, idAprueba}
	}
	if err := h.repo.GuardarJefes(config); err != nil {
		http.Redirect(w, r, "/formularios/admin/jefes?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/jefes?ok=jefes", http.StatusSeeOther)
}

// ---------- Utilidades ----------

func (h *FormularioHandler) currentUser(w http.ResponseWriter, r *http.Request) (session.Info, bool) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return session.Info{}, false
	}
	return user, true
}

func (h *FormularioHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, ok := h.currentUser(w, r)
	if !ok {
		return false
	}
	if user.Puede("administracion") {
		return true
	}
	_, _, esPersonal, esAdmin, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil || (!esPersonal && !esAdmin) {
		http.Redirect(w, r, "/formularios?ok=sinpermiso", http.StatusSeeOther)
		return false
	}
	return true
}

func parseFecha(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

func parseFloat(value string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	return f
}

func tipoNombre(tipo string) string {
	if tipo == "A" {
		return "Permiso Administrativo"
	}
	return "Feriado Legal"
}

func esDiaHabil(d time.Time, feriados map[string]bool) bool {
	if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		return false
	}
	return !feriados[d.Format("2006-01-02")]
}

func ajustarInicioHabil(d time.Time, feriados map[string]bool) time.Time {
	for !esDiaHabil(d, feriados) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func ajustarFinHabil(d time.Time, feriados map[string]bool) time.Time {
	for !esDiaHabil(d, feriados) {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

func calcularDiasHabiles(inicio, fin time.Time, feriados map[string]bool) float64 {
	if fin.Before(inicio) {
		return 0
	}
	dias := 0
	for d := inicio; !d.After(fin); d = d.AddDate(0, 0, 1) {
		if esDiaHabil(d, feriados) {
			dias++
		}
	}
	return float64(dias)
}

// setFlash coloca el mensaje del parámetro ok en Success o Error según corresponda.
func setFlash(data map[string]interface{}, ok string) {
	msg, esError := formularioFlash(ok)
	if msg == "" {
		return
	}
	if esError {
		data["Error"] = msg
	} else {
		data["Success"] = msg
	}
}

func formularioFlash(ok string) (string, bool) {
	switch ok {
	case "creado":
		return "Solicitud registrada correctamente.", false
	case "eliminado":
		return "Solicitud eliminada correctamente.", false
	case "resuelto":
		return "La solicitud fue resuelta correctamente.", false
	case "acumulacion":
		return "Solicitud de acumulación registrada correctamente.", false
	case "asignado":
		return "Días asignados actualizados correctamente.", false
	case "jefes":
		return "Configuración de jefes actualizada correctamente.", false
	case "trabajador_creado":
		return "Trabajador registrado correctamente.", false
	case "trabajador_actualizado":
		return "Trabajador actualizado correctamente.", false
	case "trabajador_eliminado":
		return "Trabajador eliminado correctamente.", false
	case "departamento_creado":
		return "Departamento registrado correctamente.", false
	case "departamento_actualizado":
		return "Departamento actualizado correctamente.", false
	case "departamento_eliminado":
		return "Departamento eliminado correctamente.", false
	case "departamento_enuso":
		return "El departamento tiene trabajadores o registros asociados y no se puede eliminar.", true
	case "invalido":
		return "Debe completar los campos obligatorios (nombre y apellido).", true
	case "noencontrado":
		return "El trabajador no fue encontrado.", true
	case "sinpermiso":
		return "No tiene permisos para realizar esa acción.", true
	case "error":
		return "No se pudo completar la operación.", true
	default:
		return "", false
	}
}

func (h *FormularioHandler) TrabajadoresAdminPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Mantenedor de Trabajadores"
	data["PuedeAdmin"] = true
	setFlash(data, r.URL.Query().Get("ok"))
	empleados, err := h.repo.TrabajadoresCRUD()
	if err != nil {
		data["Error"] = "No se pudieron cargar los trabajadores: " + err.Error()
	}
	data["Trabajadores"] = empleados
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_trabajadores.html", data)
}

func (h *FormularioHandler) TrabajadorNuevoPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Nuevo Trabajador"
	data["PuedeAdmin"] = true
	deps, err := h.repo.Departamentos()
	if err == nil {
		data["Departamentos"] = deps
	}
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_trabajador_form.html", data)
}

func (h *FormularioHandler) TrabajadorNuevoPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/admin/trabajadores?ok=error", http.StatusSeeOther)
		return
	}
	nombre := strings.TrimSpace(r.FormValue("nombre"))
	apellido := strings.TrimSpace(r.FormValue("apellido"))
	email := strings.TrimSpace(r.FormValue("email"))
	depto, _ := strconv.Atoi(r.FormValue("depto"))

	if nombre == "" || apellido == "" {
		http.Redirect(w, r, "/formularios/admin/trabajadores/nuevo?ok=invalido", http.StatusSeeOther)
		return
	}

	if err := h.repo.CrearTrabajador(nombre, apellido, email, depto); err != nil {
		http.Redirect(w, r, "/formularios/admin/trabajadores?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/trabajadores?ok=trabajador_creado", http.StatusSeeOther)
}

func (h *FormularioHandler) TrabajadorEditarPage(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Editar Trabajador"
	data["PuedeAdmin"] = true
	emp, err := h.repo.TrabajadorCRUDPorID(id)
	if err != nil {
		http.Redirect(w, r, "/formularios/admin/trabajadores?ok=noencontrado", http.StatusSeeOther)
		return
	}
	data["Trabajador"] = emp
	deps, err := h.repo.Departamentos()
	if err == nil {
		data["Departamentos"] = deps
	}
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_trabajador_form.html", data)
}

func (h *FormularioHandler) TrabajadorEditarPost(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/admin/trabajadores?ok=error", http.StatusSeeOther)
		return
	}
	nombre := strings.TrimSpace(r.FormValue("nombre"))
	apellido := strings.TrimSpace(r.FormValue("apellido"))
	email := strings.TrimSpace(r.FormValue("email"))
	depto, _ := strconv.Atoi(r.FormValue("depto"))

	if nombre == "" || apellido == "" {
		http.Redirect(w, r, fmt.Sprintf("/formularios/admin/trabajadores/%d?ok=invalido", id), http.StatusSeeOther)
		return
	}

	if err := h.repo.ActualizarTrabajador(id, nombre, apellido, email, depto); err != nil {
		http.Redirect(w, r, "/formularios/admin/trabajadores?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/trabajadores?ok=trabajador_actualizado", http.StatusSeeOther)
}

func (h *FormularioHandler) TrabajadorEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := h.repo.EliminarTrabajador(id); err != nil {
		http.Redirect(w, r, "/formularios/admin/trabajadores?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/trabajadores?ok=trabajador_eliminado", http.StatusSeeOther)
}

// ---------- Mantenedor de departamentos ----------

func (h *FormularioHandler) DepartamentosAdminPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Mantenedor de Departamentos"
	data["PuedeAdmin"] = true
	setFlash(data, r.URL.Query().Get("ok"))
	deps, err := h.repo.Departamentos()
	if err != nil {
		data["Error"] = "No se pudieron cargar los departamentos: " + err.Error()
	}
	data["Departamentos"] = deps
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_departamentos.html", data)
}

func (h *FormularioHandler) DepartamentoNuevoPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Nuevo Departamento"
	data["PuedeAdmin"] = true
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_departamento_form.html", data)
}

func (h *FormularioHandler) DepartamentoNuevoPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/admin/departamentos?ok=error", http.StatusSeeOther)
		return
	}
	descripcion := strings.TrimSpace(r.FormValue("descripcion"))
	if descripcion == "" {
		http.Redirect(w, r, "/formularios/admin/departamentos/nuevo?ok=invalido", http.StatusSeeOther)
		return
	}
	if err := h.repo.CrearDepartamento(descripcion); err != nil {
		http.Redirect(w, r, "/formularios/admin/departamentos?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/departamentos?ok=departamento_creado", http.StatusSeeOther)
}

func (h *FormularioHandler) DepartamentoEditarPage(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireAdmin(w, r) {
		return
	}
	data := pageData(r)
	data["Title"] = "Editar Departamento"
	data["PuedeAdmin"] = true
	dep, err := h.repo.DepartamentoPorID(id)
	if err != nil {
		http.Redirect(w, r, "/formularios/admin/departamentos?ok=noencontrado", http.StatusSeeOther)
		return
	}
	data["Departamento"] = dep
	renderPage(w, h.templatesDir, "for_layout.html", "for_layout", "for_departamento_form.html", data)
}

func (h *FormularioHandler) DepartamentoEditarPost(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/formularios/admin/departamentos?ok=error", http.StatusSeeOther)
		return
	}
	descripcion := strings.TrimSpace(r.FormValue("descripcion"))
	if descripcion == "" {
		http.Redirect(w, r, fmt.Sprintf("/formularios/admin/departamentos/%d?ok=invalido", id), http.StatusSeeOther)
		return
	}
	if err := h.repo.ActualizarDepartamento(id, descripcion); err != nil {
		http.Redirect(w, r, "/formularios/admin/departamentos?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/departamentos?ok=departamento_actualizado", http.StatusSeeOther)
}

func (h *FormularioHandler) DepartamentoEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireAdmin(w, r) {
		return
	}
	if err := h.repo.EliminarDepartamento(id); err != nil {
		if errors.Is(err, repository.ErrDepartamentoEnUso) {
			http.Redirect(w, r, "/formularios/admin/departamentos?ok=departamento_enuso", http.StatusSeeOther)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.Redirect(w, r, "/formularios/admin/departamentos?ok=noencontrado", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/formularios/admin/departamentos?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/formularios/admin/departamentos?ok=departamento_eliminado", http.StatusSeeOther)
}
