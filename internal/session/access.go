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

// PuedeProceso indica si la sesión puede ejecutar el proceso indicado.
// El módulo de administración autoriza el resto de los procesos.
func (i Info) PuedeProceso(codigo string) bool {
	if i.Modulos["administracion"] {
		return true
	}
	return i.Permisos[codigo]
}

// Enrich completa la sesión con los módulos vigentes del usuario.
func Enrich(load func(userID string) (map[string]bool, error)) func(http.Handler) http.Handler {
	return enriquecer(load, func(info *Info, valores map[string]bool) {
		info.Modulos = valores
	})
}

// EnrichPermisos completa la sesión con los procesos vigentes del usuario.
func EnrichPermisos(load func(userID string) (map[string]bool, error)) func(http.Handler) http.Handler {
	return enriquecer(load, func(info *Info, valores map[string]bool) {
		info.Permisos = valores
	})
}

func enriquecer(load func(userID string) (map[string]bool, error), aplicar func(*Info, map[string]bool)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := FromContext(r.Context())
			if ok {
				valores, err := load(info.UserID)
				if err != nil || valores == nil {
					valores = map[string]bool{}
				}
				aplicar(&info, valores)
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, info))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireClaveVigente envía a cambiar la contraseña cuando venció o es la primera.
func RequireClaveVigente(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := FromContext(r.Context())
		if ok && info.DebeCambiarClave && r.URL.Path != "/cuenta/clave" && r.URL.Path != "/logout" {
			if strings.HasPrefix(r.URL.Path, "/api/") || wantsJSON(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"success":false,"message":"debe cambiar la contraseña"}`))
				return
			}
			http.Redirect(w, r, "/cuenta/clave", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAlgunProceso deja pasar si la sesión tiene alguno de los procesos.
func RequireAlgunProceso(codigos ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := FromContext(r.Context())
			permitido := false
			if ok {
				for _, codigo := range codigos {
					if info.PuedeProceso(codigo) {
						permitido = true
						break
					}
				}
			}
			if !permitido {
				if strings.HasPrefix(r.URL.Path, "/api/") || wantsJSON(r) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"success":false,"message":"sin acceso al proceso"}`))
					return
				}
				http.Redirect(w, r, "/dashboard?ok=sinpermiso", http.StatusSeeOther)
				return
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
