package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
)

type AdquisicionesHandler struct {
	repo         *repository.AdquisicionesRepository
	templatesDir string
}

func NewAdquisicionesHandler(repo *repository.AdquisicionesRepository, templatesDir string) *AdquisicionesHandler {
	return &AdquisicionesHandler{repo: repo, templatesDir: templatesDir}
}

func (h *AdquisicionesHandler) render(w http.ResponseWriter, page string, data map[string]interface{}) {
	renderPage(w, h.templatesDir, "adq_layout.html", "adq_layout", page, data)
}

func (h *AdquisicionesHandler) flash(data map[string]interface{}, ok string) {
	switch ok {
	case "creado":
		data["Success"] = "El registro se guardó correctamente."
	case "actualizado":
		data["Success"] = "El registro se actualizó correctamente."
	case "eliminado":
		data["Success"] = "El registro se eliminó correctamente."
	case "enuso":
		data["Error"] = "No se puede eliminar porque tiene registros asociados."
	case "duplicado":
		data["Error"] = "Ya existe un proveedor con ese RUT."
	case "noencontrado":
		data["Error"] = "El registro no existe."
	case "error":
		data["Error"] = "No se pudo completar la operación."
	case "aprobado":
		data["Success"] = "La solicitud quedó con visto bueno y pendiente de entrega."
	case "rechazado":
		data["Success"] = "La solicitud fue rechazada."
	case "entregado":
		data["Success"] = "La entrega quedó registrada y el stock se actualizó."
	case "facturado":
		data["Success"] = "La factura quedó ingresada y el stock se actualizó."
	}
}

func (h *AdquisicionesHandler) redirigir(w http.ResponseWriter, r *http.Request, path, ok string) {
	http.Redirect(w, r, path+"?ok="+url.QueryEscape(ok), http.StatusSeeOther)
}

func (h *AdquisicionesHandler) mensajeGuardado(err error) string {
	switch {
	case errors.Is(err, repository.ErrDatos):
		return "Revise los datos. Hay un campo obligatorio vacío, un texto demasiado largo o un valor de dominio que no corresponde."
	case errors.Is(err, repository.ErrDuplicado):
		return "Ya existe un proveedor con ese RUT."
	case errors.Is(err, repository.ErrRutReferenciado):
		return "Ese RUT está usado en órdenes de compra o bienes de inventario y no se puede cambiar."
	case errors.Is(err, sql.ErrNoRows):
		return "El registro no existe."
	default:
		return "No se pudo guardar el registro."
	}
}

// ---------- Menú ----------

func (h *AdquisicionesHandler) MenuPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Módulo de Adquisiciones"
	data["Seccion"] = "menu"
	h.flash(data, r.URL.Query().Get("ok"))
	h.render(w, "adq_menu.html", data)
}

func (h *AdquisicionesHandler) MantenedoresPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Mantenedores de Adquisiciones"
	data["Seccion"] = "mantenedores"
	h.flash(data, r.URL.Query().Get("ok"))
	h.render(w, "adq_mantenedores.html", data)
}

// ---------- Tipos de dominio ----------

func (h *AdquisicionesHandler) TiposPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Tipos de dominio"
	data["Seccion"] = "tipos"
	h.flash(data, r.URL.Query().Get("ok"))
	tipos, err := h.repo.TiposDominio()
	if err != nil {
		data["Error"] = "No se pudieron cargar los tipos de dominio."
	}
	data["Tipos"] = tipos
	h.render(w, "adq_tipos.html", data)
}

func (h *AdquisicionesHandler) TipoNuevoPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Nuevo tipo de dominio"
	data["Seccion"] = "tipos"
	data["Tipo"] = models.TipoDominio{}
	h.render(w, "adq_tipo_form.html", data)
}

func (h *AdquisicionesHandler) TipoNuevoPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "error")
		return
	}
	descripcion := strings.TrimSpace(r.FormValue("descripcion"))
	if err := h.repo.CrearTipoDominio(descripcion); err != nil {
		data := pageData(r)
		data["Title"] = "Nuevo tipo de dominio"
		data["Seccion"] = "tipos"
		data["Tipo"] = models.TipoDominio{}
		data["Descripcion"] = descripcion
		data["Error"] = h.mensajeGuardado(err)
		h.render(w, "adq_tipo_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "creado")
}

func (h *AdquisicionesHandler) TipoVerPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Tipo de dominio"
	data["Seccion"] = "tipos"
	tipo, err := h.repo.TipoDominio(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "noencontrado")
		return
	}
	data["Tipo"] = tipo
	h.render(w, "adq_tipo_ver.html", data)
}

func (h *AdquisicionesHandler) TipoEditarPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Modificar tipo de dominio"
	data["Seccion"] = "tipos"
	tipo, err := h.repo.TipoDominio(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "noencontrado")
		return
	}
	data["Tipo"] = tipo
	data["Descripcion"] = tipo.Descripcion
	h.render(w, "adq_tipo_form.html", data)
}

func (h *AdquisicionesHandler) TipoEditarPost(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "error")
		return
	}
	descripcion := strings.TrimSpace(r.FormValue("descripcion"))
	if err := h.repo.ActualizarTipoDominio(id, descripcion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "noencontrado")
			return
		}
		tipo, _ := h.repo.TipoDominio(id)
		data := pageData(r)
		data["Title"] = "Modificar tipo de dominio"
		data["Seccion"] = "tipos"
		data["Tipo"] = tipo
		data["Descripcion"] = descripcion
		data["Error"] = h.mensajeGuardado(err)
		h.render(w, "adq_tipo_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "actualizado")
}

func (h *AdquisicionesHandler) TipoEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	err := h.repo.EliminarTipoDominio(id)
	switch {
	case errors.Is(err, repository.ErrRegistroEnUso):
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "enuso")
	case errors.Is(err, sql.ErrNoRows):
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "noencontrado")
	case err != nil:
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "error")
	default:
		h.redirigir(w, r, "/adquisiciones/mantenedores/tipos-dominio", "eliminado")
	}
}

// ---------- Dominios ----------

func (h *AdquisicionesHandler) DominiosPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Dominios"
	data["Seccion"] = "dominios"
	h.flash(data, r.URL.Query().Get("ok"))
	dominios, err := h.repo.Dominios()
	if err != nil {
		data["Error"] = "No se pudieron cargar los dominios."
	}
	data["Dominios"] = dominios
	h.render(w, "adq_dominios.html", data)
}

func (h *AdquisicionesHandler) cargarTipos(data map[string]interface{}) {
	if tipos, err := h.repo.TiposDominio(); err == nil {
		data["Tipos"] = tipos
	}
}

func (h *AdquisicionesHandler) DominioNuevoPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Nuevo dominio"
	data["Seccion"] = "dominios"
	data["Dominio"] = models.Dominio{}
	data["CodTipo"] = 0
	h.cargarTipos(data)
	h.render(w, "adq_dominio_form.html", data)
}

func (h *AdquisicionesHandler) DominioNuevoPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "error")
		return
	}
	codTipo, _ := strconv.Atoi(r.FormValue("cod_tipo"))
	descripcion := strings.TrimSpace(r.FormValue("descripcion"))
	if err := h.repo.CrearDominio(codTipo, descripcion); err != nil {
		data := pageData(r)
		data["Title"] = "Nuevo dominio"
		data["Seccion"] = "dominios"
		data["Dominio"] = models.Dominio{}
		data["CodTipo"] = codTipo
		data["Descripcion"] = descripcion
		data["Error"] = h.mensajeGuardado(err)
		h.cargarTipos(data)
		h.render(w, "adq_dominio_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "creado")
}

func (h *AdquisicionesHandler) DominioVerPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Dominio"
	data["Seccion"] = "dominios"
	dom, err := h.repo.Dominio(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "noencontrado")
		return
	}
	data["Dominio"] = dom
	h.render(w, "adq_dominio_ver.html", data)
}

func (h *AdquisicionesHandler) DominioEditarPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Modificar dominio"
	data["Seccion"] = "dominios"
	dom, err := h.repo.Dominio(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "noencontrado")
		return
	}
	data["Dominio"] = dom
	data["Descripcion"] = dom.Descripcion
	h.render(w, "adq_dominio_form.html", data)
}

func (h *AdquisicionesHandler) DominioEditarPost(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "error")
		return
	}
	descripcion := strings.TrimSpace(r.FormValue("descripcion"))
	if err := h.repo.ActualizarDominio(id, descripcion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "noencontrado")
			return
		}
		dom, _ := h.repo.Dominio(id)
		data := pageData(r)
		data["Title"] = "Modificar dominio"
		data["Seccion"] = "dominios"
		data["Dominio"] = dom
		data["Descripcion"] = descripcion
		data["Error"] = h.mensajeGuardado(err)
		h.render(w, "adq_dominio_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "actualizado")
}

func (h *AdquisicionesHandler) DominioEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	err := h.repo.EliminarDominio(id)
	switch {
	case errors.Is(err, repository.ErrRegistroEnUso):
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "enuso")
	case errors.Is(err, sql.ErrNoRows):
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "noencontrado")
	case err != nil:
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "error")
	default:
		h.redirigir(w, r, "/adquisiciones/mantenedores/dominios", "eliminado")
	}
}

// ---------- Materiales ----------

func (h *AdquisicionesHandler) cargarTiposMaterial(data map[string]interface{}) {
	if tipos, err := h.repo.DominiosPorTipo(models.CodTipoMaterial); err == nil {
		data["TiposMaterial"] = tipos
	}
}

func (h *AdquisicionesHandler) MaterialesPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Materiales"
	data["Seccion"] = "materiales"
	h.flash(data, r.URL.Query().Get("ok"))
	pagina, _ := strconv.Atoi(r.URL.Query().Get("pagina"))
	total, err := h.repo.ContarMateriales()
	if err != nil {
		data["Error"] = "No se pudieron cargar los materiales."
		data["Pagina"] = models.Pagina{Actual: 1, Total: 1}
		data["Total"] = 0
		h.render(w, "adq_materiales.html", data)
		return
	}
	meta := repository.PaginaMateriales(total, pagina)
	materiales, err := h.repo.Materiales(meta.Actual)
	if err != nil {
		data["Error"] = "No se pudieron cargar los materiales."
	}
	data["Materiales"] = materiales
	data["Pagina"] = meta
	data["Total"] = total
	h.render(w, "adq_materiales.html", data)
}

func (h *AdquisicionesHandler) MaterialNuevoPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Nuevo material"
	data["Seccion"] = "materiales"
	data["Material"] = models.Material{StockCritico: 1}
	h.cargarTiposMaterial(data)
	h.render(w, "adq_material_form.html", data)
}

func materialDesdeForm(r *http.Request) models.Material {
	tipo, _ := strconv.Atoi(r.FormValue("tipo"))
	stock, err := strconv.Atoi(strings.TrimSpace(r.FormValue("stock")))
	if err != nil {
		stock = -1
	}
	return models.Material{
		Nombre:       strings.TrimSpace(r.FormValue("nombre")),
		Tipo:         tipo,
		StockCritico: stock,
	}
}

func (h *AdquisicionesHandler) MaterialNuevoPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "error")
		return
	}
	m := materialDesdeForm(r)
	if err := h.repo.CrearMaterial(m); err != nil {
		data := pageData(r)
		data["Title"] = "Nuevo material"
		data["Seccion"] = "materiales"
		data["Material"] = m
		data["Error"] = h.mensajeGuardado(err)
		h.cargarTiposMaterial(data)
		h.render(w, "adq_material_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "creado")
}

func (h *AdquisicionesHandler) MaterialVerPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Material"
	data["Seccion"] = "materiales"
	m, err := h.repo.Material(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "noencontrado")
		return
	}
	data["Material"] = m
	h.render(w, "adq_material_ver.html", data)
}

func (h *AdquisicionesHandler) MaterialEditarPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Modificar material"
	data["Seccion"] = "materiales"
	m, err := h.repo.Material(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "noencontrado")
		return
	}
	data["Material"] = m
	h.cargarTiposMaterial(data)
	h.render(w, "adq_material_form.html", data)
}

func (h *AdquisicionesHandler) MaterialEditarPost(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "error")
		return
	}
	m := materialDesdeForm(r)
	m.Codigo = id
	if err := h.repo.ActualizarMaterial(m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "noencontrado")
			return
		}
		data := pageData(r)
		data["Title"] = "Modificar material"
		data["Seccion"] = "materiales"
		data["Material"] = m
		data["Error"] = h.mensajeGuardado(err)
		h.cargarTiposMaterial(data)
		h.render(w, "adq_material_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "actualizado")
}

func (h *AdquisicionesHandler) MaterialEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	err := h.repo.EliminarMaterial(id)
	switch {
	case errors.Is(err, repository.ErrRegistroEnUso):
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "enuso")
	case errors.Is(err, sql.ErrNoRows):
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "noencontrado")
	case err != nil:
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "error")
	default:
		h.redirigir(w, r, "/adquisiciones/mantenedores/materiales", "eliminado")
	}
}

// ---------- Proveedores ----------

func (h *AdquisicionesHandler) cargarRegiones(data map[string]interface{}) {
	if regiones, err := h.repo.DominiosPorTipo(models.CodTipoRegion); err == nil {
		data["Regiones"] = regiones
	}
}

func proveedorDesdeForm(r *http.Request) models.Proveedor {
	region, _ := strconv.Atoi(r.FormValue("region"))
	return models.Proveedor{
		Rut:            repository.NormalizarRut(r.FormValue("rut")),
		Nombre:         strings.TrimSpace(r.FormValue("nombre")),
		NombreFantasia: strings.TrimSpace(r.FormValue("fantasia")),
		Direccion:      strings.TrimSpace(r.FormValue("direccion")),
		Ciudad:         strings.TrimSpace(r.FormValue("ciudad")),
		Region:         region,
		Fono:           strings.TrimSpace(r.FormValue("fono")),
		Contacto:       strings.TrimSpace(r.FormValue("contacto")),
		Fax:            strings.TrimSpace(r.FormValue("fax")),
		Mail:           strings.TrimSpace(r.FormValue("mail")),
	}
}

func (h *AdquisicionesHandler) ProveedoresPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Proveedores"
	data["Seccion"] = "proveedores"
	h.flash(data, r.URL.Query().Get("ok"))
	proveedores, err := h.repo.Proveedores()
	if err != nil {
		data["Error"] = "No se pudieron cargar los proveedores."
	}
	data["Proveedores"] = proveedores
	h.render(w, "adq_proveedores.html", data)
}

func (h *AdquisicionesHandler) ProveedorNuevoPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Nuevo proveedor"
	data["Seccion"] = "proveedores"
	data["Proveedor"] = models.Proveedor{}
	h.cargarRegiones(data)
	h.render(w, "adq_proveedor_form.html", data)
}

func (h *AdquisicionesHandler) ProveedorNuevoPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "error")
		return
	}
	p := proveedorDesdeForm(r)
	if err := h.repo.CrearProveedor(p); err != nil {
		data := pageData(r)
		data["Title"] = "Nuevo proveedor"
		data["Seccion"] = "proveedores"
		data["Proveedor"] = p
		data["Error"] = h.mensajeGuardado(err)
		h.cargarRegiones(data)
		h.render(w, "adq_proveedor_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "creado")
}

func (h *AdquisicionesHandler) ProveedorVerPage(w http.ResponseWriter, r *http.Request, rut string) {
	data := pageData(r)
	data["Title"] = "Proveedor"
	data["Seccion"] = "proveedores"
	p, err := h.repo.Proveedor(rut)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "noencontrado")
		return
	}
	data["Proveedor"] = p
	h.render(w, "adq_proveedor_ver.html", data)
}

func (h *AdquisicionesHandler) ProveedorEditarPage(w http.ResponseWriter, r *http.Request, rut string) {
	data := pageData(r)
	data["Title"] = "Modificar proveedor"
	data["Seccion"] = "proveedores"
	p, err := h.repo.Proveedor(rut)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "noencontrado")
		return
	}
	data["Proveedor"] = p
	data["RutActual"] = p.Rut
	h.cargarRegiones(data)
	h.render(w, "adq_proveedor_form.html", data)
}

func (h *AdquisicionesHandler) ProveedorEditarPost(w http.ResponseWriter, r *http.Request, rut string) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "error")
		return
	}
	p := proveedorDesdeForm(r)
	if err := h.repo.ActualizarProveedor(rut, p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "noencontrado")
			return
		}
		data := pageData(r)
		data["Title"] = "Modificar proveedor"
		data["Seccion"] = "proveedores"
		data["Proveedor"] = p
		data["RutActual"] = repository.NormalizarRut(rut)
		data["Error"] = h.mensajeGuardado(err)
		h.cargarRegiones(data)
		h.render(w, "adq_proveedor_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "actualizado")
}

func (h *AdquisicionesHandler) ProveedorEliminarPost(w http.ResponseWriter, r *http.Request, rut string) {
	err := h.repo.EliminarProveedor(rut)
	switch {
	case errors.Is(err, repository.ErrRegistroEnUso):
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "enuso")
	case errors.Is(err, sql.ErrNoRows):
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "noencontrado")
	case err != nil:
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "error")
	default:
		h.redirigir(w, r, "/adquisiciones/mantenedores/proveedores", "eliminado")
	}
}

// ---------- Asignaciones ----------

func (h *AdquisicionesHandler) AsignacionesPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Asignaciones"
	data["Seccion"] = "asignaciones"
	h.flash(data, r.URL.Query().Get("ok"))
	asignaciones, err := h.repo.AsignacionesMantenedor()
	if err != nil {
		data["Error"] = "No se pudieron cargar las asignaciones."
	}
	data["Asignaciones"] = asignaciones
	h.render(w, "adq_asignaciones.html", data)
}

func (h *AdquisicionesHandler) AsignacionNuevaPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Nueva asignación"
	data["Seccion"] = "asignaciones"
	data["Asignacion"] = models.Asignacion{}
	h.render(w, "adq_asignacion_form.html", data)
}

func asignacionDesdeForm(r *http.Request) models.Asignacion {
	return models.Asignacion{
		Codigo:      strings.TrimSpace(r.FormValue("codigo")),
		Descripcion: strings.TrimSpace(r.FormValue("descripcion")),
	}
}

func (h *AdquisicionesHandler) AsignacionNuevaPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "error")
		return
	}
	a := asignacionDesdeForm(r)
	if err := h.repo.CrearAsignacion(a); err != nil {
		data := pageData(r)
		data["Title"] = "Nueva asignación"
		data["Seccion"] = "asignaciones"
		data["Asignacion"] = a
		data["Error"] = h.mensajeGuardado(err)
		h.render(w, "adq_asignacion_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "creado")
}

func (h *AdquisicionesHandler) AsignacionVerPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Asignación"
	data["Seccion"] = "asignaciones"
	a, err := h.repo.Asignacion(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "noencontrado")
		return
	}
	data["Asignacion"] = a
	h.render(w, "adq_asignacion_ver.html", data)
}

func (h *AdquisicionesHandler) AsignacionEditarPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Modificar asignación"
	data["Seccion"] = "asignaciones"
	a, err := h.repo.Asignacion(id)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "noencontrado")
		return
	}
	data["Asignacion"] = a
	h.render(w, "adq_asignacion_form.html", data)
}

func (h *AdquisicionesHandler) AsignacionEditarPost(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "error")
		return
	}
	a := asignacionDesdeForm(r)
	a.Correlativo = id
	if err := h.repo.ActualizarAsignacion(a); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "noencontrado")
			return
		}
		data := pageData(r)
		data["Title"] = "Modificar asignación"
		data["Seccion"] = "asignaciones"
		data["Asignacion"] = a
		data["Error"] = h.mensajeGuardado(err)
		h.render(w, "adq_asignacion_form.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "actualizado")
}

func (h *AdquisicionesHandler) AsignacionEliminarPost(w http.ResponseWriter, r *http.Request, id int) {
	err := h.repo.EliminarAsignacion(id)
	switch {
	case errors.Is(err, repository.ErrRegistroEnUso):
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "enuso")
	case errors.Is(err, sql.ErrNoRows):
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "noencontrado")
	case err != nil:
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "error")
	default:
		h.redirigir(w, r, "/adquisiciones/mantenedores/asignaciones", "eliminado")
	}
}
