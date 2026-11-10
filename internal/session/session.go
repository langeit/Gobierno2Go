package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

const CookieName = "g2g_session"

const sessionTTL = 8 * time.Hour

// Info contiene los datos del usuario autenticado que viajan en la sesión.
type Info struct {
	UserID      string
	Login       string
	Nombre      string
	Tipo        string
	Etapa       int
	EtapaNombre string
	Modulos     map[string]bool
}

type sessionData struct {
	Info    Info
	Expires time.Time
}

// Manager mantiene las sesiones en memoria. Para un despliegue con varias
// instancias conviene sustituirlo por un almacén compartido (Redis, DB, etc.).
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]sessionData
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]sessionData)}
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create registra una sesión y devuelve su token.
func (m *Manager) Create(info Info) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.sessions[token] = sessionData{Info: info, Expires: time.Now().Add(sessionTTL)}
	m.mu.Unlock()
	return token, nil
}

// Get recupera la información de una sesión válida.
func (m *Manager) Get(token string) (Info, bool) {
	m.mu.RLock()
	data, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return Info{}, false
	}
	if time.Now().After(data.Expires) {
		m.Delete(token)
		return Info{}, false
	}
	return data.Info, true
}

// Delete elimina una sesión.
func (m *Manager) Delete(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

type ctxKey struct{}

// FromContext devuelve la sesión activa, si existe.
func FromContext(ctx context.Context) (Info, bool) {
	info, ok := ctx.Value(ctxKey{}).(Info)
	return info, ok
}

// Middleware carga la sesión desde la cookie y la deja disponible en el contexto.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(CookieName); err == nil {
			if info, ok := m.Get(c.Value); ok {
				ctx := context.WithValue(r.Context(), ctxKey{}, info)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireLogin rechaza las peticiones sin sesión. Las peticiones de API reciben
// 401 JSON; las de navegación se redirigen al login.
func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") || wantsJSON(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success":false,"message":"no autenticado"}`))
				return
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// SetCookie escribe la cookie de sesión.
func SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// ClearCookie invalida la cookie de sesión en el navegador.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
