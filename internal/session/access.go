package session

import (
	"context"
	"net/http"
	"strings"
)

// Puede indica si la sesión puede entrar al módulo indicado.
func (i Info) Puede(codigo string) bool {
	return i.Modulos[codigo]
}

// Enrich completa la sesión con los módulos vigentes del usuario.
func Enrich(load func(userID string) (map[string]bool, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := FromContext(r.Context())
			if ok {
				mods, err := load(info.UserID)
				if err != nil || mods == nil {
					mods = map[string]bool{}
				}
				info.Modulos = mods
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, info))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireModule deja pasar solo a quien tiene el módulo indicado.
func RequireModule(codigo string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := FromContext(r.Context())
			if !ok || !info.Puede(codigo) {
				if strings.HasPrefix(r.URL.Path, "/api/") || wantsJSON(r) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"success":false,"message":"sin acceso al módulo"}`))
					return
				}
				http.Redirect(w, r, "/dashboard?ok=sinpermiso", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
