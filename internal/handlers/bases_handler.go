package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"

	"github.com/go-chi/chi/v5"
)

// OpcionTabla es un acceso del submenú Tablas.
type OpcionTabla struct {
	Codigo string
	Nombre string
}

// OpcionesTablas devuelve las opciones del menú Bases, submenú Tablas.
func OpcionesTablas() []OpcionTabla {
	var opciones []OpcionTabla
	for _, t := range models.Tablas() {
		opciones = append(opciones, OpcionTabla{Codigo: t.Codigo, Nombre: t.Nombre})
	}
	return opciones
}

type columnaVista struct {
	Etiqueta string
}

type claveVista struct {
	Nombre string
	Valor  string
}

type filaVista struct {
	Celdas      []string
	EditarURL   string
	EliminarURL string
	Claves      []claveVista
	HijosURL    string
}

// BasesHandler muestra el menú Bases y los mantenedores de Tablas.
type BasesHandler struct {
	repo         *repository.TablaRepository
	templatesDir string
}

func NewBasesHandler(repo *repository.TablaRepository, templatesDir string) *BasesHandler {
	return &BasesHandler{repo: repo, templatesDir: templatesDir}
}

func (h *BasesHandler) datos(r *http.Request, title, codigo string) map[string]interface{} {
	data := pageData(r)
	data["Title"] = title
	data["Opciones"] = OpcionesTablas()
	data["Codigo"] = codigo
	return data
}

func (h *BasesHandler) MenuPage(w http.ResponseWriter, r *http.Request) {
	data := h.datos(r, "Tablas", "")
	renderPage(w, h.templatesDir, "bas_layout.html", "bas_layout", "bas_menu.html", data)
}

// ---------- Listado ----------

func (h *BasesHandler) OpcionPage(w http.ResponseWriter, r *http.Request) {
	h.lista(w, r, false)
}

func (h *BasesHandler) HijosPage(w http.ResponseWriter, r *http.Request) {
	h.lista(w, r, true)
}

func (h *BasesHandler) lista(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	def, padre, ok := definicion(codigo, esHija)
	if !ok {
		http.NotFound(w, r)
		return
	}

	padreValor := r.URL.Query().Get("padre")
	padreEtiqueta := ""
	filtroColumna, filtroValor := "", ""
	if esHija {
		if padreValor == "" {
			h.redirigir(w, r, "/bases/tablas/"+codigo, "error", "")
			return
		}
		filtroColumna = def.CampoPadre
		filtroValor = padreValor
		if etiqueta, err := h.repo.EtiquetaRegistro(padre, map[string]string{padre.Clave()[0].Nombre: padreValor}); err == nil {
			padreEtiqueta = etiqueta
		}
	}

	filas, err := h.repo.Listar(def, filtroColumna, filtroValor)
	if err != nil {
		data := h.datos(r, def.Nombre, codigo)
		data["Def"] = def
		data["Error"] = "No se pudieron cargar los registros."
		h.render(w, "bas_lista.html", data)
		return
	}

	base := "/bases/tablas/" + codigo
	if esHija {
		base += "/hijos"
	}
	columnas := make([]columnaVista, 0, len(def.Campos))
	for _, c := range def.Campos {
		columnas = append(columnas, columnaVista{Etiqueta: c.Etiqueta})
	}

	vistas := make([]filaVista, 0, len(filas))
	for _, fila := range filas {
		vista := filaVista{}
		for _, c := range def.Campos {
			vista.Celdas = append(vista.Celdas, textoCelda(c, fila[c.Nombre]))
		}
		consulta := url.Values{}
		for _, c := range def.Clave() {
			vista.Claves = append(vista.Claves, claveVista{Nombre: c.Nombre, Valor: fila[c.Nombre]})
			consulta.Set("k_"+c.Nombre, fila[c.Nombre])
		}
		if def.Editable() {
			vista.EditarURL = base + "/editar?" + consulta.Encode()
		}
		vista.EliminarURL = base + "/eliminar"
		if def.Hija != nil {
			vista.HijosURL = base + "/hijos?padre=" + url.QueryEscape(fila[def.Clave()[0].Nombre])
		}
		vistas = append(vistas, vista)
	}

	data := h.datos(r, def.Nombre, codigo)
	data["Def"] = def
	data["Columnas"] = columnas
	data["Filas"] = vistas
	data["EsHija"] = esHija
	data["Padre"] = padreValor
	data["PadreEtiqueta"] = padreEtiqueta
	h.flash(data, r.URL.Query().Get("ok"), r.URL.Query().Get("detalle"))
	if def.Hija != nil {
		data["HijaNombre"] = def.Hija.Nombre
	}
	if esHija {
		data["NuevoURL"] = base + "/nuevo?padre=" + url.QueryEscape(padreValor)
		data["VolverURL"] = "/bases/tablas/" + codigo
	} else {
		data["NuevoURL"] = base + "/nuevo"
	}
	h.render(w, "bas_lista.html", data)
}

func textoCelda(c models.TablaCampo, valor string) string {
	switch c.Tipo {
	case models.CampoBooleano:
		if valor == "true" || valor == "t" || valor == "1" {
			return "Sí"
		}
		return "No"
	case models.CampoFecha:
		if len(valor) >= 10 {
			return valor[:10]
		}
	}
	return valor
}

// ---------- Formulario ----------

func (h *BasesHandler) NuevoPage(w http.ResponseWriter, r *http.Request) {
	h.nuevo(w, r, false)
}

func (h *BasesHandler) HijoNuevoPage(w http.ResponseWriter, r *http.Request) {
	h.nuevo(w, r, true)
}

func (h *BasesHandler) nuevo(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	def, padre, ok := definicion(codigo, esHija)
	if !ok {
		http.NotFound(w, r)
		return
	}
	valores := map[string]string{}
	if esHija {
		valores[def.CampoPadre] = r.URL.Query().Get("padre")
		if valores[def.CampoPadre] == "" {
			h.redirigir(w, r, "/bases/tablas/"+codigo, "error", "")
			return
		}
	}
	h.renderFormulario(w, r, codigo, def, padre, valores, false, esHija)
}

func (h *BasesHandler) NuevoPost(w http.ResponseWriter, r *http.Request) {
	h.nuevoPost(w, r, false)
}

func (h *BasesHandler) HijoNuevoPost(w http.ResponseWriter, r *http.Request) {
	h.nuevoPost(w, r, true)
}

func (h *BasesHandler) nuevoPost(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	def, padre, ok := definicion(codigo, esHija)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, listaPath(codigo, esHija, ""), "error", "")
		return
	}
	valores, mensaje := validar(def, leerValores(def, r, false))
	if mensaje != "" {
		h.renderFormularioConError(w, r, codigo, def, padre, valores, false, esHija, mensaje)
		return
	}
	if err := h.repo.Guardar(def, valores, nil, false); err != nil {
		h.renderFormularioConError(w, r, codigo, def, padre, valores, false, esHija, mensajeGuardado(err))
		return
	}
	destino := listaPath(codigo, esHija, valores[def.CampoPadre])
	h.redirigir(w, r, destino, "creado", "")
}

func (h *BasesHandler) EditarPage(w http.ResponseWriter, r *http.Request) {
	h.editar(w, r, false)
}

func (h *BasesHandler) HijoEditarPage(w http.ResponseWriter, r *http.Request) {
	h.editar(w, r, true)
}

func (h *BasesHandler) editar(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	def, padre, ok := definicion(codigo, esHija)
	if !ok {
		http.NotFound(w, r)
		return
	}
	clave := claveDesdeQuery(def, r)
	fila, err := h.repo.Fila(def, clave)
	if err != nil {
		h.redirigir(w, r, listaPath(codigo, esHija, clave[def.CampoPadre]), "noencontrado", "")
		return
	}
	h.renderFormulario(w, r, codigo, def, padre, fila, true, esHija)
}

func (h *BasesHandler) EditarPost(w http.ResponseWriter, r *http.Request) {
	h.editarPost(w, r, false)
}

func (h *BasesHandler) HijoEditarPost(w http.ResponseWriter, r *http.Request) {
	h.editarPost(w, r, true)
}

func (h *BasesHandler) editarPost(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	def, padre, ok := definicion(codigo, esHija)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, listaPath(codigo, esHija, ""), "error", "")
		return
	}
	clave := claveDesdeForm(def, r)
	valores, mensaje := validar(def, leerValores(def, r, true))
	if mensaje != "" {
		h.renderFormularioConError(w, r, codigo, def, padre, valores, true, esHija, mensaje)
		return
	}
	if err := h.repo.Guardar(def, valores, clave, true); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, listaPath(codigo, esHija, clave[def.CampoPadre]), "noencontrado", "")
			return
		}
		h.renderFormularioConError(w, r, codigo, def, padre, valores, true, esHija, mensajeGuardado(err))
		return
	}
	h.redirigir(w, r, listaPath(codigo, esHija, valores[def.CampoPadre]), "actualizado", "")
}

func (h *BasesHandler) renderFormulario(w http.ResponseWriter, r *http.Request, codigo string, def, padre models.TablaDef, valores map[string]string, editar, esHija bool) {
	h.renderFormularioConError(w, r, codigo, def, padre, valores, editar, esHija, "")
}

func (h *BasesHandler) renderFormularioConError(w http.ResponseWriter, r *http.Request, codigo string, def, padre models.TablaDef, valores map[string]string, editar, esHija bool, mensaje string) {
	base := "/bases/tablas/" + codigo
	accion := base + "/nuevo"
	volver := base
	padreValor := valores[def.CampoPadre]
	padreEtiqueta := ""
	if esHija {
		accion = base + "/hijos/nuevo"
		volver = base + "/hijos?padre=" + url.QueryEscape(padreValor)
		if etiqueta, err := h.repo.EtiquetaRegistro(padre, map[string]string{padre.Clave()[0].Nombre: padreValor}); err == nil {
			padreEtiqueta = etiqueta
		}
	}
	if editar {
		accion = base + "/editar"
		if esHija {
			accion = base + "/hijos/editar"
		}
	}

	opciones := map[string][]models.OpcionCampo{}
	for _, c := range def.Campos {
		if c.Tipo != models.CampoReferencia {
			continue
		}
		lista, err := h.repo.OpcionesReferencia(c)
		if err != nil {
			mensaje = "No se pudieron cargar las opciones de " + c.Etiqueta + "."
			break
		}
		opciones[c.Nombre] = lista
	}

	title := def.Nombre
	if editar {
		title = "Modificar " + strings.ToLower(def.Nombre)
	} else {
		title = "Nuevo registro de " + strings.ToLower(def.Nombre)
	}

	data := h.datos(r, title, codigo)
	data["Def"] = def
	data["Valores"] = valores
	data["OpcionesRef"] = opciones
	data["Editar"] = editar
	data["EsHija"] = esHija
	data["Accion"] = accion
	data["VolverURL"] = volver
	data["Padre"] = padreValor
	data["PadreEtiqueta"] = padreEtiqueta
	if mensaje != "" {
		data["Error"] = mensaje
	}
	h.render(w, "bas_form.html", data)
}

// ---------- Eliminar ----------

func (h *BasesHandler) EliminarPost(w http.ResponseWriter, r *http.Request) {
	h.eliminar(w, r, false)
}

func (h *BasesHandler) HijoEliminarPost(w http.ResponseWriter, r *http.Request) {
	h.eliminar(w, r, true)
}

func (h *BasesHandler) eliminar(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	def, _, ok := definicion(codigo, esHija)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirigir(w, r, listaPath(codigo, esHija, ""), "error", "")
		return
	}
	clave := claveDesdeForm(def, r)
	detalle, err := h.repo.Eliminar(def, clave)
	switch {
	case errors.Is(err, repository.ErrRegistroEnUso):
		h.redirigir(w, r, listaPath(codigo, esHija, clave[def.CampoPadre]), "enuso", detalle)
	case errors.Is(err, sql.ErrNoRows):
		h.redirigir(w, r, listaPath(codigo, esHija, clave[def.CampoPadre]), "noencontrado", "")
	case err != nil:
		h.redirigir(w, r, listaPath(codigo, esHija, clave[def.CampoPadre]), "error", "")
	default:
		h.redirigir(w, r, listaPath(codigo, esHija, clave[def.CampoPadre]), "eliminado", "")
	}
}

// ---------- Utilidades ----------

func definicion(codigo string, esHija bool) (models.TablaDef, models.TablaDef, bool) {
	parent, ok := models.TablaPorCodigo(codigo)
	if !ok {
		return models.TablaDef{}, models.TablaDef{}, false
	}
	if !esHija {
		return parent, models.TablaDef{}, true
	}
	if parent.Hija == nil {
		return models.TablaDef{}, models.TablaDef{}, false
	}
	return *parent.Hija, parent, true
}

func listaPath(codigo string, esHija bool, padre string) string {
	base := "/bases/tablas/" + codigo
	if !esHija {
		return base
	}
	return base + "/hijos?padre=" + url.QueryEscape(padre)
}

func claveDesdeQuery(def models.TablaDef, r *http.Request) map[string]string {
	clave := map[string]string{}
	for _, c := range def.Clave() {
		clave[c.Nombre] = r.URL.Query().Get("k_" + c.Nombre)
	}
	return clave
}

func claveDesdeForm(def models.TablaDef, r *http.Request) map[string]string {
	clave := map[string]string{}
	for _, c := range def.Clave() {
		clave[c.Nombre] = strings.TrimSpace(r.FormValue("k_" + c.Nombre))
	}
	return clave
}

func leerValores(def models.TablaDef, r *http.Request, editar bool) map[string]string {
	valores := map[string]string{}
	for _, c := range def.Campos {
		if c.Clave && (editar || c.Nombre == def.CampoPadre) {
			valores[c.Nombre] = strings.TrimSpace(r.FormValue("k_" + c.Nombre))
			continue
		}
		valores[c.Nombre] = strings.TrimSpace(r.FormValue("f_" + c.Nombre))
	}
	return valores
}

func validar(def models.TablaDef, valores map[string]string) (map[string]string, string) {
	for _, c := range def.Campos {
		valor := strings.TrimSpace(valores[c.Nombre])
		switch c.Tipo {
		case models.CampoTexto:
			if c.Requerido && valor == "" {
				return valores, "El campo " + c.Etiqueta + " es obligatorio."
			}
			if c.Largo > 0 && utf8.RuneCountInString(valor) > c.Largo {
				return valores, fmt.Sprintf("El campo %s admite hasta %d caracteres.", c.Etiqueta, c.Largo)
			}
		case models.CampoEntero:
			if valor == "" {
				if c.Requerido {
					return valores, "El campo " + c.Etiqueta + " es obligatorio."
				}
				break
			}
			n, err := strconv.Atoi(valor)
			if err != nil {
				return valores, "El campo " + c.Etiqueta + " debe ser un número entero."
			}
			if n < c.Min || (c.Max > 0 && n > c.Max) {
				if c.Max > 0 {
					return valores, fmt.Sprintf("El campo %s debe estar entre %d y %d.", c.Etiqueta, c.Min, c.Max)
				}
				return valores, fmt.Sprintf("El campo %s no puede ser menor que %d.", c.Etiqueta, c.Min)
			}
			valores[c.Nombre] = strconv.Itoa(n)
		case models.CampoDecimal:
			if valor == "" {
				if c.Requerido {
					return valores, "El campo " + c.Etiqueta + " es obligatorio."
				}
				break
			}
			n := strings.ReplaceAll(valor, ",", ".")
			if _, err := strconv.ParseFloat(n, 64); err != nil {
				return valores, "El campo " + c.Etiqueta + " debe ser un número."
			}
			valores[c.Nombre] = n
		case models.CampoFecha:
			if valor == "" {
				if c.Requerido {
					return valores, "El campo " + c.Etiqueta + " es obligatorio."
				}
				break
			}
			if len(valor) != 10 || valor[4] != '-' || valor[7] != '-' {
				return valores, "El campo " + c.Etiqueta + " debe tener el formato AAAA-MM-DD."
			}
		case models.CampoBooleano:
			if valor == "true" || valor == "1" || valor == "on" {
				valores[c.Nombre] = "true"
			} else {
				valores[c.Nombre] = "false"
			}
		case models.CampoOpcion:
			if valor == "" {
				if c.Requerido {
					return valores, "El campo " + c.Etiqueta + " es obligatorio."
				}
				break
			}
			encontrada := false
			for _, o := range c.Opciones {
				if o.Valor == valor {
					encontrada = true
					break
				}
			}
			if !encontrada {
				return valores, "El valor de " + c.Etiqueta + " no es válido."
			}
		case models.CampoReferencia:
			if c.Requerido && valor == "" {
				return valores, "Debe seleccionar " + c.Etiqueta + "."
			}
		}
	}
	if def.Validar != nil {
		if mensaje := def.Validar(valores); mensaje != "" {
			return valores, mensaje
		}
	}
	return valores, ""
}

func mensajeGuardado(err error) string {
	switch {
	case errors.Is(err, repository.ErrDuplicado):
		return "Ya existe un registro con esa clave."
	case errors.Is(err, repository.ErrReferencia):
		return "La referencia seleccionada no existe o fue eliminada."
	case errors.Is(err, repository.ErrDatos):
		return "Revise los datos: hay un valor fuera de rango o un campo obligatorio vacío."
	default:
		return "No se pudo guardar el registro."
	}
}

func (h *BasesHandler) flash(data map[string]interface{}, ok, detalle string) {
	switch ok {
	case "creado":
		data["Success"] = "El registro se guardó correctamente."
	case "actualizado":
		data["Success"] = "El registro se actualizó correctamente."
	case "eliminado":
		data["Success"] = "El registro se eliminó correctamente."
	case "duplicado":
		data["Error"] = "Ya existe un registro con esa clave."
	case "enuso":
		if detalle != "" {
			data["Error"] = "No se puede eliminar porque tiene " + detalle + " asociadas."
		} else {
			data["Error"] = "No se puede eliminar porque tiene registros asociados."
		}
	case "referencia":
		data["Error"] = "La referencia seleccionada no existe o fue eliminada."
	case "datos":
		data["Error"] = "Revise los datos ingresados."
	case "noencontrado":
		data["Error"] = "El registro no existe o fue eliminado."
	case "error":
		data["Error"] = "No se pudo completar la operación."
	}
}

func (h *BasesHandler) redirigir(w http.ResponseWriter, r *http.Request, path, ok, detalle string) {
	separador := "?"
	if strings.Contains(path, "?") {
		separador = "&"
	}
	destino := path + separador + "ok=" + url.QueryEscape(ok)
	if detalle != "" {
		destino += "&detalle=" + url.QueryEscape(detalle)
	}
	http.Redirect(w, r, destino, http.StatusSeeOther)
}

func (h *BasesHandler) render(w http.ResponseWriter, page string, data map[string]interface{}) {
	renderPage(w, h.templatesDir, "bas_layout.html", "bas_layout", page, data)
}
