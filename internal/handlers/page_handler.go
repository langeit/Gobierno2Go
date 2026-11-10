package handlers

import (
	"net/http"

	"Gobierno2Go/internal/session"
)

type PageHandler struct {
	templatesDir string
}

func NewPageHandler(templatesDir string) *PageHandler {
	return &PageHandler{templatesDir: templatesDir}
}

func (h *PageHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.FromContext(r.Context()); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	renderPage(w, h.templatesDir, "login.html", "login.html", "", map[string]interface{}{})
}

func (h *PageHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	if r.URL.Query().Get("ok") == "sinpermiso" {
		data["Error"] = "No tiene acceso a ese módulo."
	}
	renderPage(w, h.templatesDir, "dashboard.html", "dashboard.html", "", data)
}
