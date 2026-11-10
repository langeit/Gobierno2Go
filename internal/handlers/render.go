package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"Gobierno2Go/internal/session"
)

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"money": func(v float64) string {
			entero := fmt.Sprintf("%.0f", v)
			neg := strings.HasPrefix(entero, "-")
			if neg {
				entero = entero[1:]
			}
			var partes []string
			for len(entero) > 3 {
				partes = append([]string{entero[len(entero)-3:]}, partes...)
				entero = entero[:len(entero)-3]
			}
			partes = append([]string{entero}, partes...)
			res := "$" + strings.Join(partes, ".")
			if neg {
				res = "-" + res
			}
			return res
		},
		"pct": func(v float64) string {
			return fmt.Sprintf("%.2f%%", v)
		},
	}
}

// pageData construye el mapa base que reciben todas las plantillas.
func pageData(r *http.Request) map[string]interface{} {
	data := map[string]interface{}{}
	if info, ok := session.FromContext(r.Context()); ok {
		data["User"] = info
	}
	return data
}

// renderPage parsea el layout indicado junto con la página y los ejecuta.
func renderPage(w http.ResponseWriter, templatesDir, layoutFile, layoutName, pageFile string, data map[string]interface{}) {
	files := []string{filepath.Join(templatesDir, layoutFile)}
	if pageFile != "" {
		files = append(files, filepath.Join(templatesDir, pageFile))
	}

	tmpl, err := template.New(layoutName).Funcs(templateFuncs()).ParseFiles(files...)
	if err != nil {
		http.Error(w, "Error cargando plantilla: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, layoutName, data); err != nil {
		http.Error(w, "Error renderizando plantilla: "+err.Error(), http.StatusInternalServerError)
	}
}

func flashMessage(ok string) string {
	switch ok {
	case "ingresado":
		return "Documento ingresado correctamente."
	case "recibido":
		return "Documento recepcionado correctamente."
	case "despachado":
		return "Documento despachado correctamente."
	case "archivado":
		return "Documento archivado correctamente."
	default:
		return ""
	}
}
