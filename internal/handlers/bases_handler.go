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
	"Gobierno2Go/internal/session"

	"github.com/go-chi/chi/v5"
)

// OpcionTabla es un acceso del submenú Tablas.
type OpcionTabla struct {
	Codigo string
	Nombre string
}

// OpcionesTablasPara devuelve solo las tablas autorizadas para la sesión.
func OpcionesTablasPara(info session.Info) []OpcionTabla {
	var opciones []OpcionTabla
	for _, t := range models.Tablas() {
		if info.PuedeProceso(models.PermisoTabla(t.Codigo)) {
			opciones = append(opciones, OpcionTabla{Codigo: t.Codigo, Nombre: t.Nombre})
		}
	}
	return opciones
}

type columnaVista struct {
	Etiqueta   string
	OrdenURL   string
	Activa     bool
	Ascendente bool
}

type paginaVista struct {
	Numero int
	URL    string
	Activa bool
}

type paginacionVista struct {
	Pagina       int
	TotalPaginas int
	Total        int
	Desde        int
	Hasta        int
	AnteriorURL  string
	SiguienteURL string
	Paginas      []paginaVista
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
	if info, ok := session.FromContext(r.Context()); ok {
		data["Opciones"] = OpcionesTablasPara(info)
	}
	data["Codigo"] = codigo
	return data
}

func (h *BasesHandler) autorizar(w http.ResponseWriter, r *http.Request, codigo string) bool {
	info, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return false
	}
	if info.PuedeProceso(models.PermisoTabla(codigo)) {
		return true
	}
	http.Redirect(w, r, "/dashboard?ok=sinpermiso", http.StatusSeeOther)
	return false
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

const registrosPorPagina = 15

func (h *BasesHandler) lista(w http.ResponseWriter, r *http.Request, esHija bool) {
	codigo := chi.URLParam(r, "codigo")
	if !h.autorizar(w, r, codigo) {
		return
	}
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

	base := "/bases/tablas/" + codigo
	if esHija {
		base += "/hijos"
	}

	busqueda := strings.TrimSpace(r.URL.Query().Get("q"))
	orden := r.URL.Query().Get("orden")
	if orden != "" {
		if _, existe := def.Campo(orden); !existe {
			orden = ""
		}
	}
	ascendente := r.URL.Query().Get("dir") != "desc"
	pagina := 1
	if n, err := strconv.Atoi(r.URL.Query().Get("pagina")); err == nil && n > 0 {
		pagina = n
	}
	ordenEfectivo := orden
	if ordenEfectivo == "" && len(def.Orden) == 1 {
		ordenEfectivo = def.Orden[0]
	}

	filas, total, pagina, err := h.repo.Listar(def, repository.ListaOpciones{
		FiltroColumna: filtroColumna,
		FiltroValor:   filtroValor,
		Busqueda:      busqueda,
		OrdenColumna:  orden,
		Ascendente:    ascendente,
		Pagina:        pagina,
		PorPagina:     registrosPorPagina,
	})
	if err != nil {
		data := h.datos(r, def.Nombre, codigo)
		data["Def"] = def
		data["Error"] = "No se pudieron cargar los registros."
		h.render(w, "bas_lista.html", data)
		return
	}

	columnas := make([]columnaVista, 0, len(def.Campos))
	for _, c := range def.Campos {
		activa := c.Nombre == ordenEfectivo
		nuevaAscendente := true
		if activa && ascendente {
			nuevaAscendente = false
		}
		columnas = append(columnas, columnaVista{
			Etiqueta:   c.Etiqueta,
			OrdenURL:   urlLista(base, busqueda, esHija, padreValor, c.Nombre, nuevaAscendente, 1),
			Activa:     activa,
			Ascendente: ascendente,
		})
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

	totalPaginas := (total + registrosPorPagina - 1) / registrosPorPagina
	if totalPaginas < 1 {
		totalPaginas = 1
	}
	desde, hasta := 0, 0
	if total > 0 {
		desde = (pagina-1)*registrosPorPagina + 1
		hasta = desde + len(vistas) - 1
	}
	paginacion := paginacionVista{
		Pagina:       pagina,
		TotalPaginas: totalPaginas,
		Total:        total,
		Desde:        desde,
		Hasta:        hasta,
	}
	if pagina > 1 {
		paginacion.AnteriorURL = urlLista(base, busqueda, esHija, padreValor, ordenEfectivo, ascendente, pagina-1)
	}
	if pagina < totalPaginas {
		paginacion.SiguienteURL = urlLista(base, busqueda, esHija, padreValor, ordenEfectivo, ascendente, pagina+1)
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
			URL:    urlLista(base, busqueda, esHija, padreValor, ordenEfectivo, ascendente, i),
			Activa: i == pagina,
		})
	}

	data := h.datos(r, def.Nombre, codigo)
	data["Def"] = def
	data["Columnas"] = columnas
	data["Filas"] = vistas
	data["EsHija"] = esHija
	data["Padre"] = padreValor
	data["PadreEtiqueta"] = padreEtiqueta
	data["ListaURL"] = base
	data["Busqueda"] = busqueda
	data["Orden"] = orden
	data["LimpiarURL"] = urlLista(base, "", esHija, padreValor, "", true, 1)
	data["Paginacion"] = paginacion
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
	if !h.autorizar(w, r, codigo) {
		return
	}
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
	if !h.autorizar(w, r, codigo) {
		return
	}
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
	if !h.autorizar(w, r, codigo) {
		return
	}
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
	if !h.autorizar(w, r, codigo) {
		return
	}
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

	var title string
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
	if !h.autorizar(w, r, codigo) {
		return
	}
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

// urlLista arma los enlaces del listado conservando búsqueda, orden y página.
func urlLista(base, busqueda string, esHija bool, padre, orden string, ascendente bool, pagina int) string {
	valores := url.Values{}
	if busqueda != "" {
		valores.Set("q", busqueda)
	}
	if esHija {
		valores.Set("padre", padre)
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
