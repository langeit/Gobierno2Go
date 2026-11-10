package handlers

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

type InventarioHandler struct {
	repo         *repository.InventarioRepository
	templatesDir string
}

func NewInventarioHandler(repo *repository.InventarioRepository, templatesDir string) *InventarioHandler {
	return &InventarioHandler{repo: repo, templatesDir: templatesDir}
}

func (h *InventarioHandler) currentUser(w http.ResponseWriter, r *http.Request) (session.Info, bool) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return session.Info{}, false
	}
	return user, true
}

func (h *InventarioHandler) setFlash(data map[string]interface{}, ok string) {
	switch ok {
	case "ingresado":
		data["Success"] = "Bienes ingresados correctamente."
	case "solicitud":
		data["Success"] = "Solicitud registrada correctamente."
	case "resuelto":
		data["Success"] = "Solicitud resuelta correctamente."
	case "ubicacion":
		data["Success"] = "Ubicación del bien actualizada correctamente."
	case "estado":
		data["Success"] = "Estado del bien actualizado correctamente."
	case "cuenta":
		data["Success"] = "Cuenta presupuestaria asignada correctamente."
	case "responsable":
		data["Success"] = "Responsables actualizados correctamente."
	case "porcentaje":
		data["Success"] = "Porcentaje de actualización guardado correctamente."
	case "depreciacion":
		data["Success"] = "Depreciación anual ejecutada correctamente."
	case "catalogo":
		data["Success"] = "Catálogo actualizado correctamente."
	case "sinpermiso":
		data["Error"] = "No tiene permisos para realizar esta acción."
	case "error":
		data["Error"] = "No se pudo completar la operación solicitada."
	}
}

// ---------- Menú e inicio ----------

func (h *InventarioHandler) MenuPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Módulo de Inventario"
	h.setFlash(data, r.URL.Query().Get("ok"))

	idEmpleado, _, _, err := h.repo.UsuarioInfo(user.UserID)
	if err == nil {
		total, mis, pends, errCount := h.repo.Conteos(user.UserID, idEmpleado)
		if errCount == nil {
			data["TotalBienes"] = total
			data["MisBienes"] = mis
			data["MisPendientes"] = pends
		}
	}
	if admin, err := h.repo.EsAdminInventario(user.UserID); err == nil {
		data["EsAdmin"] = admin
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_menu.html", data)
}

// ---------- Bienes e ingreso ----------

func (h *InventarioHandler) BienesPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Listado de Bienes"

	corrTbien, _ := strconv.Atoi(r.URL.Query().Get("tipo"))
	data["TipoFiltro"] = corrTbien

	bienes, err := h.repo.Bienes(corrTbien)
	if err != nil {
		data["Error"] = "Error al cargar bienes: " + err.Error()
	} else {
		data["Bienes"] = bienes
	}

	tipos, err := h.repo.TiposBien()
	if err == nil {
		data["TiposBien"] = tipos
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_bienes.html", data)
}

func (h *InventarioHandler) IngresoPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Ingreso de Bienes"
	data["Fecha"] = time.Now().Format("02-01-2006")
	data["Anho"] = time.Now().Format("2006")
	h.setFlash(data, r.URL.Query().Get("ok"))
	if msg := strings.TrimSpace(r.URL.Query().Get("msg")); msg != "" {
		data["Error"] = msg
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("codigos")); raw != "" {
		codigos := strings.Split(raw, ",")
		data["Codigos"] = codigos
		if len(codigos) == 1 {
			data["Success"] = "El ingreso del bien ha finalizado correctamente. El código asignado es " + codigos[0] + "."
		} else {
			data["Success"] = "El ingreso de los bienes ha finalizado correctamente. Los códigos asignados son: " + strings.Join(codigos, ", ") + "."
		}
	}

	if tipos, err := h.repo.TiposBien(); err == nil {
		data["TiposBien"] = tipos
	}
	if zonas, err := h.repo.Zonas(); err == nil {
		data["Zonas"] = zonas
	}
	if asignaciones, err := h.repo.Asignaciones(); err == nil {
		data["Asignaciones"] = asignaciones
	}
	if proveedores, err := h.repo.Proveedores(); err == nil {
		data["Proveedores"] = proveedores
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_ingreso.html", data)
}

func (h *InventarioHandler) IngresoPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectIngreso(w, r, nil, err)
		return
	}

	ndocCompra, _ := strconv.ParseInt(r.FormValue("ndocto_compra"), 10, 64)
	ndocOrigen, _ := strconv.ParseInt(r.FormValue("ndocto_origen"), 10, 64)

	tipos := r.Form["tipo_bien[]"]
	descs := r.Form["descripcion[]"]
	montos := r.Form["monto[]"]
	zonas := r.Form["zona[]"]
	pisos := r.Form["piso[]"]
	oficinas := r.Form["oficina[]"]
	imputs := r.Form["imputacion[]"]
	fuentes := r.Form["fuente[]"]

	items := make([]models.IngresoItem, 0, len(tipos))
	for i := range tipos {
		monto, _ := strconv.ParseFloat(strings.ReplaceAll(formAt(montos, i), ",", "."), 64)
		corrTbien, _ := strconv.Atoi(formAt(tipos, i))
		corrZona, _ := strconv.Atoi(formAt(zonas, i))
		corrAsig, _ := strconv.Atoi(formAt(imputs, i))
		fuente, _ := strconv.Atoi(formAt(fuentes, i))
		items = append(items, models.IngresoItem{
			CorrTbien:   corrTbien,
			Descripcion: formAt(descs, i),
			Monto:       monto,
			CorrZona:    corrZona,
			Piso:        formAt(pisos, i),
			Oficina:     formAt(oficinas, i),
			CorrAsig:    corrAsig,
			Fuente:      fuente,
		})
	}

	ingreso := models.IngresoBien{
		TipoDocCompra: r.FormValue("tdocto_compra"),
		NumDocCompra:  ndocCompra,
		TipoDocOrigen: r.FormValue("tdocto_origen"),
		NumDocOrigen:  ndocOrigen,
		RutProveedor:  r.FormValue("rut_proveedor"),
		Items:         items,
	}

	codigos, err := h.repo.CrearIngreso(ingreso, user.UserID)
	if err != nil {
		log.Printf("ingreso inventario: %v", err)
		h.redirectIngreso(w, r, nil, err)
		return
	}
	h.redirectIngreso(w, r, codigos, nil)
}

func formAt(values []string, i int) string {
	if i < 0 || i >= len(values) {
		return ""
	}
	return values[i]
}

func (h *InventarioHandler) redirectIngreso(w http.ResponseWriter, r *http.Request, codigos []string, err error) {
	q := url.Values{}
	if err != nil {
		q.Set("ok", "error")
		if msg, ok := repository.MensajeUsuario(err); ok {
			q.Set("msg", msg)
		}
	} else {
		q.Set("ok", "ingresado")
		if len(codigos) > 0 {
			q.Set("codigos", strings.Join(codigos, ","))
		}
	}
	http.Redirect(w, r, "/inventario/ingreso?"+q.Encode(), http.StatusSeeOther)
}

// ---------- Ubicación, Estado, Cuenta, Responsable ----------

func (h *InventarioHandler) UbicacionPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Cambio de Ubicación"
	h.setFlash(data, r.URL.Query().Get("ok"))

	if bienes, err := h.repo.Bienes(0); err == nil {
		data["Bienes"] = bienes
	}
	if zonas, err := h.repo.Zonas(); err == nil {
		data["Zonas"] = zonas
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_ubicacion.html", data)
}

func (h *InventarioHandler) UbicacionPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/ubicacion?ok=error", http.StatusSeeOther)
		return
	}
	corrBien, _ := strconv.Atoi(r.FormValue("corr_bien"))
	corrZona, _ := strconv.Atoi(r.FormValue("corr_zona"))
	piso := r.FormValue("piso")
	oficina := r.FormValue("oficina")
	tipoDoc := r.FormValue("tipo_doc")
	numDoc, _ := strconv.ParseInt(r.FormValue("num_doc"), 10, 64)

	if corrBien <= 0 || corrZona <= 0 {
		http.Redirect(w, r, "/inventario/ubicacion?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.CambiarUbicacion(corrBien, corrZona, piso, oficina, tipoDoc, numDoc, user.UserID); err != nil {
		http.Redirect(w, r, "/inventario/ubicacion?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/ubicacion?ok=ubicacion", http.StatusSeeOther)
}

func (h *InventarioHandler) EstadoPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Cambio de Estado"
	h.setFlash(data, r.URL.Query().Get("ok"))

	if bienes, err := h.repo.Bienes(0); err == nil {
		data["Bienes"] = bienes
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_estado.html", data)
}

func (h *InventarioHandler) EstadoPost(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/estado?ok=error", http.StatusSeeOther)
		return
	}
	corrBien, _ := strconv.Atoi(r.FormValue("corr_bien"))
	estado := r.FormValue("estado")

	if corrBien <= 0 || estado == "" {
		http.Redirect(w, r, "/inventario/estado?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.CambiarEstado(corrBien, estado); err != nil {
		http.Redirect(w, r, "/inventario/estado?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/estado?ok=estado", http.StatusSeeOther)
}

func (h *InventarioHandler) CuentaPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Asignación de Cuenta"
	h.setFlash(data, r.URL.Query().Get("ok"))

	if bienes, err := h.repo.Bienes(0); err == nil {
		data["Bienes"] = bienes
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_cuenta.html", data)
}

func (h *InventarioHandler) CuentaPost(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/cuenta?ok=error", http.StatusSeeOther)
		return
	}
	corrBien, _ := strconv.Atoi(r.FormValue("corr_bien"))
	cuenta := r.FormValue("cuenta")

	if corrBien <= 0 {
		http.Redirect(w, r, "/inventario/cuenta?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.AsignarCuenta(corrBien, cuenta); err != nil {
		http.Redirect(w, r, "/inventario/cuenta?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/cuenta?ok=cuenta", http.StatusSeeOther)
}

func (h *InventarioHandler) ResponsablePage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Asignación de Responsable"
	h.setFlash(data, r.URL.Query().Get("ok"))

	if bienes, err := h.repo.Bienes(0); err == nil {
		data["Bienes"] = bienes
	}
	if empleados, err := h.repo.Empleados(); err == nil {
		data["Empleados"] = empleados
	}

	if idStr := r.URL.Query().Get("bien"); idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			if bien, err := h.repo.BienPorID(id); err == nil {
				data["Seleccionado"] = bien
			}
			if asig, err := h.repo.ResponsablesDe(id); err == nil {
				data["ResponsablesAsignados"] = asig
			}
		}
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_responsable.html", data)
}

func (h *InventarioHandler) ResponsablePost(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/responsable?ok=error", http.StatusSeeOther)
		return
	}
	corrBien, _ := strconv.Atoi(r.FormValue("corr_bien"))
	empleadosStrs := r.Form["id_empleado[]"]

	var ids []int
	for _, s := range empleadosStrs {
		if id, err := strconv.Atoi(s); err == nil {
			ids = append(ids, id)
		}
	}

	if corrBien <= 0 {
		http.Redirect(w, r, "/inventario/responsable?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.AsignarResponsables(corrBien, ids); err != nil {
		http.Redirect(w, r, "/inventario/responsable?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/inventario/responsable?bien=%d&ok=responsable", corrBien), http.StatusSeeOther)
}

// ---------- Solicitudes ----------

func (h *InventarioHandler) SolicitudesPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Solicitudes de Inventario"
	h.setFlash(data, r.URL.Query().Get("ok"))

	isAdmin, _ := h.repo.EsAdminInventario(user.UserID)
	data["EsAdmin"] = isAdmin

	solicitudes, err := h.repo.Solicitudes(user.UserID, false)
	if err == nil {
		data["Solicitudes"] = solicitudes
	}

	if isAdmin {
		if adminPends, err := h.repo.SolicitudesPendientesAdmin(); err == nil {
			data["PendientesAdmin"] = adminPends
		}
	}
	if jefePends, err := h.repo.SolicitudesPendientesJefe(user.UserID); err == nil {
		data["PendientesJefe"] = jefePends
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_solicitudes.html", data)
}

func (h *InventarioHandler) SolicitudNuevaPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Nueva Solicitud de Inventario"
	data["Fecha"] = time.Now().Format("02-01-2006")

	idEmpleado, _, _, err := h.repo.UsuarioInfo(user.UserID)
	if err == nil && idEmpleado > 0 {
		if bienes, err := h.repo.BienesDeEmpleado(idEmpleado); err == nil {
			data["BienesEmpleado"] = bienes
		}
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_solicitud_nueva.html", data)
}

func (h *InventarioHandler) SolicitudNuevaPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/solicitudes/nueva?ok=error", http.StatusSeeOther)
		return
	}

	tipo := r.FormValue("tipo_sol")
	desc := r.FormValue("descripcion")
	bienesStrs := r.Form["bienes[]"]

	var bienes []int
	for _, s := range bienesStrs {
		if id, err := strconv.Atoi(s); err == nil {
			bienes = append(bienes, id)
		}
	}

	idEmpleado, _, _, err := h.repo.UsuarioInfo(user.UserID)
	if err != nil {
		http.Redirect(w, r, "/inventario/solicitudes/nueva?ok=error", http.StatusSeeOther)
		return
	}

	if _, err := h.repo.CrearSolicitud(user.UserID, tipo, desc, idEmpleado, bienes); err != nil {
		http.Redirect(w, r, "/inventario/solicitudes/nueva?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/solicitudes?ok=solicitud", http.StatusSeeOther)
}

func (h *InventarioHandler) SolicitudDetallePage(w http.ResponseWriter, r *http.Request, id int) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Detalle de Solicitud"
	h.setFlash(data, r.URL.Query().Get("ok"))

	sol, err := h.repo.SolicitudPorID(id)
	if err != nil {
		data["Error"] = "La solicitud no existe."
	} else {
		data["Solicitud"] = sol
		if okJefe, _ := h.repo.PuedeAprobarJefe(user.UserID, id); okJefe && sol.EstadoSol == "P" {
			data["PuedeResolverJefe"] = true
		}
		if okAdmin, _ := h.repo.PuedeAprobarAdmin(user.UserID); okAdmin && sol.EstadoSol == "C" {
			data["PuedeResolverAdmin"] = true
		}
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_solicitud_detalle.html", data)
}

func (h *InventarioHandler) SolicitudResolverPost(w http.ResponseWriter, r *http.Request, id int) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/inventario/solicitudes/%d?ok=error", id), http.StatusSeeOther)
		return
	}
	accion := r.FormValue("accion")
	aprobar := accion == "aprobar"

	sol, err := h.repo.SolicitudPorID(id)
	if err != nil {
		http.Redirect(w, r, "/inventario/solicitudes?ok=error", http.StatusSeeOther)
		return
	}

	if sol.EstadoSol == "P" {
		if okJefe, _ := h.repo.PuedeAprobarJefe(user.UserID, id); !okJefe {
			http.Redirect(w, r, fmt.Sprintf("/inventario/solicitudes/%d?ok=sinpermiso", id), http.StatusSeeOther)
			return
		}
		if err := h.repo.ResolverJefe(id, aprobar); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/inventario/solicitudes/%d?ok=error", id), http.StatusSeeOther)
			return
		}
	} else if sol.EstadoSol == "C" {
		if okAdmin, _ := h.repo.PuedeAprobarAdmin(user.UserID); !okAdmin {
			http.Redirect(w, r, fmt.Sprintf("/inventario/solicitudes/%d?ok=sinpermiso", id), http.StatusSeeOther)
			return
		}
		if err := h.repo.ResolverAdmin(id, aprobar); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/inventario/solicitudes/%d?ok=error", id), http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/inventario/solicitudes/%d?ok=resuelto", id), http.StatusSeeOther)
}

// ---------- Porcentajes y Depreciación ----------

func (h *InventarioHandler) PorcentajePage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Ingreso Porcentaje de Actualización"
	h.setFlash(data, r.URL.Query().Get("ok"))

	anho := r.URL.Query().Get("anho")
	if anho == "" {
		anho = time.Now().Format("2006")
	}
	data["Anho"] = anho

	if p, encontrado, err := h.repo.PorcentajeAnio(anho); err == nil && encontrado {
		data["Porcentaje"] = p
	}
	if ejecutada, err := h.repo.DepreciacionEjecutada(anho); err == nil {
		data["DepreciacionEjecutada"] = ejecutada
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_porcentaje.html", data)
}

func (h *InventarioHandler) PorcentajePost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/porcentaje?ok=error", http.StatusSeeOther)
		return
	}
	anho := r.FormValue("anho")
	valor, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("valor"), ",", "."), 64)

	if anho == "" || valor <= 0 {
		http.Redirect(w, r, "/inventario/porcentaje?ok=error", http.StatusSeeOther)
		return
	}
	if err := h.repo.GuardarPorcentaje(anho, valor, user.UserID); err != nil {
		http.Redirect(w, r, "/inventario/porcentaje?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/inventario/porcentaje?anho=%s&ok=porcentaje", anho), http.StatusSeeOther)
}

func (h *InventarioHandler) DepreciacionPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Depreciación de Bienes"
	h.setFlash(data, r.URL.Query().Get("ok"))

	anho := time.Now().Format("2006")
	data["Anho"] = anho

	if ejecutada, err := h.repo.DepreciacionEjecutada(anho); err == nil {
		data["DepreciacionEjecutada"] = ejecutada
	}
	if p, encontrado, err := h.repo.PorcentajeAnio(anho); err == nil && encontrado {
		data["Porcentaje"] = p
	}
	if anos, err := h.repo.AnosDepreciados(); err == nil {
		data["AnosDepreciados"] = anos
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_depreciacion.html", data)
}

func (h *InventarioHandler) DepreciacionPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/depreciacion?ok=error", http.StatusSeeOther)
		return
	}
	anho := r.FormValue("anho")
	if anho == "" {
		anho = time.Now().Format("2006")
	}

	if _, err := h.repo.EjecutarDepreciacion(anho, user.UserID); err != nil {
		http.Redirect(w, r, "/inventario/depreciacion?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/depreciacion?ok=depreciacion", http.StatusSeeOther)
}

// ---------- Reportes ----------

func (h *InventarioHandler) ReportesPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Reportes de Inventario"

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reportes.html", data)
}

func (h *InventarioHandler) ReporteResponsablePage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Listado de Bienes por Responsable"

	if empleados, err := h.repo.Empleados(); err == nil {
		data["Empleados"] = empleados
	}

	idEmp, _ := strconv.Atoi(r.URL.Query().Get("empleado"))
	desde := r.URL.Query().Get("desde")
	hasta := r.URL.Query().Get("hasta")
	data["IdEmpleado"] = idEmp
	data["Desde"] = desde
	data["Hasta"] = hasta

	if idEmp > 0 {
		if bienes, err := h.repo.BienesPorResponsable(idEmp, desde, hasta); err == nil {
			data["Bienes"] = bienes
		}
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reporte_responsable.html", data)
}

func (h *InventarioHandler) ReporteUbicacionPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Listado de Bienes por Ubicación"

	if zonas, err := h.repo.Zonas(); err == nil {
		data["Zonas"] = zonas
	}
	if tipos, err := h.repo.TiposBien(); err == nil {
		data["TiposBien"] = tipos
	}

	zonaID, _ := strconv.Atoi(r.URL.Query().Get("zona"))
	tipoID, _ := strconv.Atoi(r.URL.Query().Get("tipo"))
	piso := r.URL.Query().Get("piso")
	oficina := r.URL.Query().Get("oficina")

	data["ZonaID"] = zonaID
	data["TipoID"] = tipoID
	data["Piso"] = piso
	data["Oficina"] = oficina

	if bienes, err := h.repo.BienesPorUbicacion(zonaID, tipoID, piso, oficina); err == nil {
		data["Bienes"] = bienes
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reporte_ubicacion.html", data)
}

func (h *InventarioHandler) ReporteAnioPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Listado de Bienes Adquiridos en el Año"

	if tipos, err := h.repo.TiposBien(); err == nil {
		data["TiposBien"] = tipos
	}

	anho := r.URL.Query().Get("anho")
	if anho == "" {
		anho = time.Now().Format("2006")
	}
	tipoID, _ := strconv.Atoi(r.URL.Query().Get("tipo"))

	data["Anho"] = anho
	data["TipoID"] = tipoID

	if bienes, err := h.repo.BienesAdquiridosAnio(anho, tipoID); err == nil {
		data["Bienes"] = bienes
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reporte_anio.html", data)
}

func (h *InventarioHandler) ReporteMovimientosPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Historial de Movimientos"

	concepto := r.URL.Query().Get("concepto")
	desde := r.URL.Query().Get("desde")
	hasta := r.URL.Query().Get("hasta")

	data["Concepto"] = concepto
	data["Desde"] = desde
	data["Hasta"] = hasta

	if movs, err := h.repo.Movimientos(concepto, desde, hasta); err == nil {
		data["Movimientos"] = movs
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reporte_movimientos.html", data)
}

func (h *InventarioHandler) ReporteDepreciacionPage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Listado de Bienes Depreciados"

	if tipos, err := h.repo.TiposBien(); err == nil {
		data["TiposBien"] = tipos
	}

	tipoID, _ := strconv.Atoi(r.URL.Query().Get("tipo"))
	codigo := r.URL.Query().Get("codigo")

	data["TipoID"] = tipoID
	data["Codigo"] = codigo

	if bienes, err := h.repo.BienesDepreciados(tipoID, codigo); err == nil {
		data["Bienes"] = bienes
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reporte_depreciacion.html", data)
}

func (h *InventarioHandler) ReporteDepreciacionDetallePage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Detalle de Depreciación"

	bienID, _ := strconv.Atoi(r.URL.Query().Get("bien"))
	if bienID > 0 {
		if bien, err := h.repo.BienPorID(bienID); err == nil {
			data["Bien"] = bien
		}
		if filas, err := h.repo.DepreciacionDetalle(bienID); err == nil {
			data["Depreciaciones"] = filas
		}
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_reporte_depreciacion_detalle.html", data)
}

// ---------- Administración ----------

func (h *InventarioHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	data := pageData(r)
	data["Title"] = "Administración de Inventario"
	h.setFlash(data, r.URL.Query().Get("ok"))

	admin, err := h.repo.EsAdminInventario(user.UserID)
	if err != nil || !admin {
		http.Redirect(w, r, "/inventario?ok=sinpermiso", http.StatusSeeOther)
		return
	}

	if tipos, err := h.repo.TiposBien(); err == nil {
		data["TiposBien"] = tipos
	}
	if zonas, err := h.repo.Zonas(); err == nil {
		data["Zonas"] = zonas
	}

	renderPage(w, h.templatesDir, "inv_layout.html", "inv_layout", "inv_admin.html", data)
}

func (h *InventarioHandler) AdminTipoBienPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	admin, _ := h.repo.EsAdminInventario(user.UserID)
	if !admin {
		http.Redirect(w, r, "/inventario?ok=sinpermiso", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/admin?ok=error", http.StatusSeeOther)
		return
	}
	desc := r.FormValue("descripcion")
	codigo := r.FormValue("codigo")
	if err := h.repo.CrearTipoBien(desc, codigo); err != nil {
		http.Redirect(w, r, "/inventario/admin?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/admin?ok=catalogo", http.StatusSeeOther)
}

func (h *InventarioHandler) AdminZonaPost(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	admin, _ := h.repo.EsAdminInventario(user.UserID)
	if !admin {
		http.Redirect(w, r, "/inventario/admin?ok=sinpermiso", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/inventario/admin?ok=error", http.StatusSeeOther)
		return
	}
	desc := r.FormValue("descripcion")
	codigo := r.FormValue("codigo")
	if err := h.repo.CrearZona(desc, codigo); err != nil {
		http.Redirect(w, r, "/inventario/admin?ok=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/inventario/admin?ok=catalogo", http.StatusSeeOther)
}
