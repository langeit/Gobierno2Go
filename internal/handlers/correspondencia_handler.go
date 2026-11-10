package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

type CorrespondenciaHandler struct {
	docRepo      *repository.DocumentRepository
	catalogRepo  *repository.CatalogRepository
	userRepo     *repository.UserRepository
	templatesDir string
}

func NewCorrespondenciaHandler(
	docRepo *repository.DocumentRepository,
	catalogRepo *repository.CatalogRepository,
	userRepo *repository.UserRepository,
	templatesDir string,
) *CorrespondenciaHandler {
	return &CorrespondenciaHandler{
		docRepo:      docRepo,
		catalogRepo:  catalogRepo,
		userRepo:     userRepo,
		templatesDir: templatesDir,
	}
}

// ---------- Páginas ----------

// MenuPage muestra el menú principal del módulo.
func (h *CorrespondenciaHandler) MenuPage(w http.ResponseWriter, r *http.Request) {
	user, _ := session.FromContext(r.Context())
	data := pageData(r)
	data["Success"] = flashMessage(r.URL.Query().Get("ok"))

	stats, err := h.docRepo.GetStats()
	if err != nil {
		data["Error"] = "No se pudieron cargar las estadísticas: " + err.Error()
	}
	data["Stats"] = stats

	if pendientes, err := h.docRepo.PendingForUser(user.UserID, user.Etapa); err == nil {
		data["Pendientes"] = len(pendientes)
	}
	if enMano, err := h.docRepo.HeldByUser(user.UserID, user.Etapa); err == nil {
		data["EnMano"] = len(enMano)
	}

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_menu.html", data)
}

func (h *CorrespondenciaHandler) IngresoPage(w http.ResponseWriter, r *http.Request) {
	h.renderIngreso(w, r, "")
}

func (h *CorrespondenciaHandler) renderIngreso(w http.ResponseWriter, r *http.Request, errMsg string) {
	user, _ := session.FromContext(r.Context())
	data := pageData(r)
	data["Error"] = errMsg

	tipos, err := h.catalogRepo.Tipos()
	if err != nil {
		data["Error"] = "Error cargando tipos: " + err.Error()
	}
	origenes, _ := h.catalogRepo.Origenes()
	acciones, _ := h.catalogRepo.Acciones()
	destinos, _ := h.buildDestinos(user.UserID, user.Etapa)

	data["Tipos"] = tipos
	data["Origenes"] = origenes
	data["Acciones"] = acciones
	data["Destinos"] = destinos

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_ingreso.html", data)
}

func (h *CorrespondenciaHandler) IngresoPost(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderIngreso(w, r, "Datos del formulario inválidos")
		return
	}

	numero, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("numero")))
	destUsers, destEtapas := parseDestinos([]string{r.FormValue("destino")})

	in := models.CreateDocumentInput{
		Tipo:       r.FormValue("tipo"),
		Origen:     r.FormValue("origen"),
		Accion:     r.FormValue("accion"),
		Referencia: r.FormValue("referencia"),
		Origen2:    r.FormValue("origen2"),
		Notas:      r.FormValue("notas"),
		Numero:     numero,
		Fecha:      r.FormValue("fecha"),
	}
	if len(destUsers) > 0 {
		in.EnviarAUsuario = destUsers[0]
	}
	if len(destEtapas) > 0 {
		in.EnviarAEtapa = destEtapas[0]
	}

	barra, err := h.docRepo.Create(in, user.UserID, user.Etapa)
	if err != nil {
		h.renderIngreso(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/correspondencia/ficha?barra="+barra+"&ok=ingresado", http.StatusSeeOther)
}

func (h *CorrespondenciaHandler) RecepcionPage(w http.ResponseWriter, r *http.Request) {
	h.renderRecepcion(w, r, "")
}

func (h *CorrespondenciaHandler) renderRecepcion(w http.ResponseWriter, r *http.Request, errMsg string) {
	user, _ := session.FromContext(r.Context())
	data := pageData(r)
	data["Error"] = errMsg
	data["Success"] = flashMessage(r.URL.Query().Get("ok"))

	pending, err := h.docRepo.PendingForUser(user.UserID, user.Etapa)
	if err != nil {
		data["Error"] = "Error cargando documentos pendientes: " + err.Error()
	}
	data["Pending"] = pending
	data["Total"] = len(pending)

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_recepcion.html", data)
}

func (h *CorrespondenciaHandler) RecepcionPost(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderRecepcion(w, r, "Datos del formulario inválidos")
		return
	}

	barra := strings.TrimSpace(r.FormValue("barra"))
	if err := h.docRepo.Receive(barra, user.UserID, user.Etapa); err != nil {
		h.renderRecepcion(w, r, err.Error())
		return
	}
	http.Redirect(w, r, "/correspondencia/recepcion?ok=recibido", http.StatusSeeOther)
}

func (h *CorrespondenciaHandler) DespachoPage(w http.ResponseWriter, r *http.Request) {
	h.renderDespacho(w, r, "")
}

func (h *CorrespondenciaHandler) renderDespacho(w http.ResponseWriter, r *http.Request, errMsg string) {
	user, _ := session.FromContext(r.Context())
	data := pageData(r)
	data["Error"] = errMsg
	data["Success"] = flashMessage(r.URL.Query().Get("ok"))

	held, err := h.docRepo.HeldByUser(user.UserID, user.Etapa)
	if err != nil {
		data["Error"] = "Error cargando documentos en su poder: " + err.Error()
	}
	destinos, _ := h.buildDestinos(user.UserID, user.Etapa)

	data["Held"] = held
	data["Total"] = len(held)
	data["Destinos"] = destinos

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_despacho.html", data)
}

func (h *CorrespondenciaHandler) DespachoPost(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderDespacho(w, r, "Datos del formulario inválidos")
		return
	}

	barra := strings.TrimSpace(r.FormValue("barra"))
	destUsers, destEtapas := parseDestinos(r.Form["destinos"])
	if err := h.docRepo.Dispatch(barra, user.UserID, user.Etapa, destUsers, destEtapas); err != nil {
		h.renderDespacho(w, r, err.Error())
		return
	}
	http.Redirect(w, r, "/correspondencia/despacho?ok=despachado", http.StatusSeeOther)
}

func (h *CorrespondenciaHandler) ArchivoPage(w http.ResponseWriter, r *http.Request) {
	h.renderArchivo(w, r, "")
}

func (h *CorrespondenciaHandler) renderArchivo(w http.ResponseWriter, r *http.Request, errMsg string) {
	user, _ := session.FromContext(r.Context())
	data := pageData(r)
	data["Error"] = errMsg
	data["Success"] = flashMessage(r.URL.Query().Get("ok"))

	held, err := h.docRepo.HeldByUser(user.UserID, user.Etapa)
	if err != nil {
		data["Error"] = "Error cargando documentos en su poder: " + err.Error()
	}
	archivados, err := h.docRepo.Archivados(100)
	if err != nil {
		data["Error"] = "Error cargando documentos archivados: " + err.Error()
	}

	data["Held"] = held
	data["Archivados"] = archivados

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_archivo.html", data)
}

func (h *CorrespondenciaHandler) ArchivoPost(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderArchivo(w, r, "Datos del formulario inválidos")
		return
	}

	barra := strings.TrimSpace(r.FormValue("barra"))
	carpeta := strings.TrimSpace(r.FormValue("carpeta"))
	if err := h.docRepo.Archive(barra, user.UserID, user.Etapa, carpeta); err != nil {
		h.renderArchivo(w, r, err.Error())
		return
	}
	http.Redirect(w, r, "/correspondencia/archivo?ok=archivado", http.StatusSeeOther)
}

func (h *CorrespondenciaHandler) ConsultaPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	data["Q"] = q

	docs, err := h.docRepo.Search(q)
	if err != nil {
		data["Error"] = "Error en la búsqueda: " + err.Error()
	}
	data["Docs"] = docs
	data["Total"] = len(docs)

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_consulta.html", data)
}

func (h *CorrespondenciaHandler) FichaPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Success"] = flashMessage(r.URL.Query().Get("ok"))
	barra := strings.TrimSpace(r.URL.Query().Get("barra"))
	data["Barra"] = barra

	if barra == "" {
		data["Error"] = "Debe indicar un código de barra"
		renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_ficha.html", data)
		return
	}

	ficha, err := h.docRepo.GetFicha(barra)
	if err != nil {
		data["Error"] = err.Error()
	} else {
		data["Ficha"] = ficha
	}

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_ficha.html", data)
}

func (h *CorrespondenciaHandler) EstadisticasPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)

	stats, err := h.docRepo.GetStats()
	if err != nil {
		data["Error"] = "Error cargando estadísticas: " + err.Error()
	}
	porTipo, _ := h.docRepo.StatsPorTipo()
	porEtapa, _ := h.docRepo.StatsPorEtapa()

	data["Stats"] = stats
	data["PorTipo"] = porTipo
	data["PorEtapa"] = porEtapa

	renderPage(w, h.templatesDir, "cor_layout.html", "cor_layout", "cor_estadisticas.html", data)
}

// ---------- API JSON ----------

func (h *CorrespondenciaHandler) ApiGetDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := h.docRepo.Search(r.URL.Query().Get("q"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (h *CorrespondenciaHandler) ApiCreateDocument(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "no autenticado"})
		return
	}

	var in models.CreateDocumentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "datos inválidos"})
		return
	}

	barra, err := h.docRepo.Create(in, user.UserID, user.Etapa)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "barra": barra})
}

func (h *CorrespondenciaHandler) ApiGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.docRepo.GetStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *CorrespondenciaHandler) ApiPending(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "no autenticado"})
		return
	}
	pending, err := h.docRepo.PendingForUser(user.UserID, user.Etapa)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pending)
}

func (h *CorrespondenciaHandler) ApiReceive(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "no autenticado"})
		return
	}
	var body struct {
		Barra string `json:"barra"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "datos inválidos"})
		return
	}
	if err := h.docRepo.Receive(body.Barra, user.UserID, user.Etapa); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "documento recepcionado"})
}

func (h *CorrespondenciaHandler) ApiDispatch(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "no autenticado"})
		return
	}
	var body struct {
		Barra    string   `json:"barra"`
		Destinos []string `json:"destinos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "datos inválidos"})
		return
	}
	destUsers, destEtapas := parseDestinos(body.Destinos)
	if err := h.docRepo.Dispatch(body.Barra, user.UserID, user.Etapa, destUsers, destEtapas); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "documento despachado"})
}

func (h *CorrespondenciaHandler) ApiGetFicha(w http.ResponseWriter, r *http.Request) {
	barra := strings.TrimSpace(r.URL.Query().Get("barra"))
	if barra == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "falta barra"})
		return
	}
	ficha, err := h.docRepo.GetFicha(barra)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ficha)
}

// ---------- utilidades ----------

func (h *CorrespondenciaHandler) buildDestinos(userID string, etapa int) ([]models.DestinationOption, error) {
	opts := []models.DestinationOption{}
	if etapa == 0 {
		return opts, nil
	}

	users, err := h.catalogRepo.UsuariosPorEtapa(etapa, userID)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		opts = append(opts, models.DestinationOption{
			Value: "u:" + u.ID,
			Label: u.Nombre,
			Group: "Usuario (mi centro)",
		})
	}

	etapas, err := h.catalogRepo.EtapasDistintas(etapa)
	if err != nil {
		return nil, err
	}
	for _, e := range etapas {
		opts = append(opts, models.DestinationOption{
			Value: "e:" + e.Codigo,
			Label: e.Descripcion,
			Group: "Etapa",
		})
	}

	return opts, nil
}

func parseDestinos(values []string) ([]string, []int) {
	var users []string
	var etapas []int
	for _, v := range values {
		v = strings.TrimSpace(v)
		switch {
		case strings.HasPrefix(v, "u:") && len(v) > 2:
			users = append(users, strings.TrimPrefix(v, "u:"))
		case strings.HasPrefix(v, "e:") && len(v) > 2:
			if n, err := strconv.Atoi(strings.TrimPrefix(v, "e:")); err == nil {
				etapas = append(etapas, n)
			}
		}
	}
	return users, etapas
}
