package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

type LoginHandler struct {
	userRepo *repository.UserRepository
	acceso   *repository.AccesoRepository
	sessions *session.Manager
}

func NewLoginHandler(userRepo *repository.UserRepository, acceso *repository.AccesoRepository, sessions *session.Manager) *LoginHandler {
	return &LoginHandler{userRepo: userRepo, acceso: acceso, sessions: sessions}
}

func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.LoginResponse{
			Success: false,
			Message: "Solicitud inválida",
		})
		return
	}

	user, err := h.userRepo.Login(req.Login, req.Password)
	if err != nil {
		if h.acceso != nil {
			_ = h.acceso.Registrar(req.Login, "ingreso_rechazado", err.Error(), clientIP(r))
		}
		writeJSON(w, http.StatusUnauthorized, models.LoginResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	etapa, etapaNombre, err := h.userRepo.GetCorrespondenciaInfo(user.IDUsuario)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.LoginResponse{
			Success: false,
			Message: "Error obteniendo el centro del usuario",
		})
		return
	}

	token, err := h.sessions.Create(session.Info{
		UserID:           user.IDUsuario,
		Login:            user.Login,
		Nombre:           user.Nombre,
		Tipo:             user.Tipo,
		Etapa:            etapa,
		EtapaNombre:      etapaNombre,
		DebeCambiarClave: user.DebeCambiar,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.LoginResponse{
			Success: false,
			Message: "No se pudo iniciar la sesión",
		})
		return
	}
	session.SetCookie(w, token)
	if h.acceso != nil {
		_ = h.acceso.Registrar(user.IDUsuario, "ingreso", "", clientIP(r))
	}

	destino := "/dashboard"
	if user.DebeCambiar {
		destino = "/cuenta/clave"
	}
	user.Password = ""
	writeJSON(w, http.StatusOK, models.LoginResponse{
		Success:  true,
		Message:  "Autenticación exitosa",
		User:     user,
		Redirect: destino,
	})
}

func (h *LoginHandler) ClavePage(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.FromContext(r.Context()); !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	data := pageData(r)
	data["Title"] = "Cambiar contraseña"
	switch r.URL.Query().Get("ok") {
	case "debil":
		data["Error"] = "La contraseña debe tener al menos 6 caracteres."
	case "distinta":
		data["Error"] = "La contraseña nueva debe ser distinta de la actual."
	case "actual":
		data["Error"] = "La contraseña actual no coincide."
	case "confirma":
		data["Error"] = "La confirmación no coincide con la contraseña nueva."
	case "bloqueado":
		data["Error"] = "La cuenta está bloqueada por intentos fallidos. Intente más tarde."
	case "error":
		data["Error"] = "No se pudo cambiar la contraseña."
	}
	renderPage(w, "internal/templates", "cuenta_clave.html", "cuenta_clave.html", "", data)
}

func (h *LoginHandler) ClavePost(w http.ResponseWriter, r *http.Request) {
	info, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/cuenta/clave?ok=error", http.StatusSeeOther)
		return
	}
	nueva := r.FormValue("nueva")
	if nueva != r.FormValue("confirma") {
		http.Redirect(w, r, "/cuenta/clave?ok=confirma", http.StatusSeeOther)
		return
	}
	err := h.userRepo.CambiarClave(info.UserID, r.FormValue("actual"), nueva)
	switch {
	case errors.Is(err, repository.ErrPasswordDebil):
		http.Redirect(w, r, "/cuenta/clave?ok=debil", http.StatusSeeOther)
		return
	case errors.Is(err, repository.ErrClaveIgual):
		http.Redirect(w, r, "/cuenta/clave?ok=distinta", http.StatusSeeOther)
		return
	case errors.Is(err, repository.ErrCredenciales):
		http.Redirect(w, r, "/cuenta/clave?ok=actual", http.StatusSeeOther)
		return
	case errors.Is(err, repository.ErrUsuarioBloqueado):
		http.Redirect(w, r, "/cuenta/clave?ok=bloqueado", http.StatusSeeOther)
		return
	case err != nil:
		http.Redirect(w, r, "/cuenta/clave?ok=error", http.StatusSeeOther)
		return
	}
	info.DebeCambiarClave = false
	if c, err := r.Cookie(session.CookieName); err == nil {
		h.sessions.Update(c.Value, info)
	}
	if h.acceso != nil {
		_ = h.acceso.Registrar(info.UserID, "clave_cambiada", "cambio del propio usuario", clientIP(r))
	}
	http.Redirect(w, r, "/dashboard?ok=clave", http.StatusSeeOther)
}

func (h *LoginHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(session.CookieName); err == nil {
		h.sessions.Delete(c.Value)
	}
	session.ClearCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
