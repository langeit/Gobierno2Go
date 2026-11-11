package models

import "testing"

func TestValidarAtributos(t *testing.T) {
	def, ok := TablaPorCodigo("atributos")
	if !ok {
		t.Fatal("no existe la definición de atributos")
	}
	casos := []struct {
		nombre  string
		valores map[string]string
		error   bool
	}{
		{
			nombre:  "numérico completo",
			valores: map[string]string{"formato": "Numérico", "digitos_entero": "8", "digitos_decimal": "2"},
		},
		{
			nombre:  "numérico sin decimales",
			valores: map[string]string{"formato": "Numérico", "digitos_entero": "8"},
			error:   true,
		},
		{
			nombre:  "fecha limpia",
			valores: map[string]string{"formato": "Fecha"},
		},
		{
			nombre:  "fecha con largo",
			valores: map[string]string{"formato": "Fecha", "largo_texto": "10"},
			error:   true,
		},
		{
			nombre:  "texto completo",
			valores: map[string]string{"formato": "Texto", "largo_texto": "40"},
		},
		{
			nombre:  "texto sin largo",
			valores: map[string]string{"formato": "Texto"},
			error:   true,
		},
	}
	for _, caso := range casos {
		mensaje := def.Validar(caso.valores)
		if caso.error && mensaje == "" {
			t.Errorf("%s: se esperaba error", caso.nombre)
		}
		if !caso.error && mensaje != "" {
			t.Errorf("%s: error inesperado: %s", caso.nombre, mensaje)
		}
	}
}

func TestValidarTiposLicencias(t *testing.T) {
	def, ok := TablaPorCodigo("tipos-de-licencias")
	if !ok {
		t.Fatal("no existe la definición de tipos de licencias")
	}
	if mensaje := def.Validar(map[string]string{"dias_descontar": "5", "descuenta_bajo_dias": "4"}); mensaje == "" {
		t.Error("se esperaba error cuando el bajo n es menor que los días a descontar")
	}
	if mensaje := def.Validar(map[string]string{"dias_descontar": "5", "descuenta_bajo_dias": "5"}); mensaje != "" {
		t.Errorf("error inesperado: %s", mensaje)
	}
}
