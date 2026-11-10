package handlers

import (
	"encoding/json"
	"net/http"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

type LoginHandler struct {
	userRepo *repository.UserRepository
	sessions *session.Manager
}

func NewLoginHandler(userRepo *repository.UserRepository, sessions *session.Manager) *LoginHandler {
	return &LoginHandler{userRepo: userRepo, sessions: sessions}
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
		UserID:      user.IDUsuario,
		Login:       user.Login,
		Nombre:      user.Nombre,
		Tipo:        user.Tipo,
		Etapa:       etapa,
		EtapaNombre: etapaNombre,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.LoginResponse{
			Success: false,
			Message: "No se pudo iniciar la sesión",
		})
		return
	}
	session.SetCookie(w, token)

	user.Password = ""
	writeJSON(w, http.StatusOK, models.LoginResponse{
		Success:  true,
		Message:  "Autenticación exitosa",
		User:     user,
		Redirect: "/dashboard",
	})
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
