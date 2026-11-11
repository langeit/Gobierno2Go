package repository

import (
	"database/sql"
	"errors"
	"testing"

	"Gobierno2Go/internal/config"
	"Gobierno2Go/internal/models"
)

func TestCRUDTablaRegion(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewTablaRepository(db)
	def, ok := models.TablaPorCodigo("region")
	if !ok {
		t.Fatal("no existe la definición de región")
	}
	const codigo = "ZZ99"
	t.Cleanup(func() { db.Exec(`DELETE FROM tab_region WHERE codigo = $1`, codigo) })
	db.Exec(`DELETE FROM tab_region WHERE codigo = $1`, codigo)

	valores := map[string]string{"codigo": codigo, "descripcion": "Región de prueba"}
	if err := repo.Guardar(def, valores, nil, false); err != nil {
		t.Fatal(err)
	}
	fila, err := repo.Fila(def, map[string]string{"codigo": codigo})
	if err != nil {
		t.Fatal(err)
	}
	if fila["descripcion"] != "Región de prueba" {
		t.Fatalf("descripción = %q", fila["descripcion"])
	}

	if err := repo.Guardar(def, valores, nil, false); !errors.Is(err, ErrDuplicado) {
		t.Fatalf("se esperaba duplicado, se obtuvo %v", err)
	}

	valores["descripcion"] = "Región editada"
	if err := repo.Guardar(def, valores, map[string]string{"codigo": codigo}, true); err != nil {
		t.Fatal(err)
	}
	fila, err = repo.Fila(def, map[string]string{"codigo": codigo})
	if err != nil {
		t.Fatal(err)
	}
	if fila["descripcion"] != "Región editada" {
		t.Fatalf("descripción = %q", fila["descripcion"])
	}

	if _, err := repo.Eliminar(def, map[string]string{"codigo": codigo}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Fila(def, map[string]string{"codigo": codigo}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("se esperaba sql.ErrNoRows, se obtuvo %v", err)
	}
}

func TestCRUDTablaConClaveCompuesta(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewTablaRepository(db)
	def, ok := models.TablaPorCodigo("impuesto-unico")
	if !ok {
		t.Fatal("no existe la definición de impuesto único")
	}
	clave := map[string]string{"anio": "2099", "mes": "12", "tramo": "9"}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM tab_impuesto_unico WHERE anio = 2099 AND mes = 12 AND tramo = 9`)
	})
	db.Exec(`DELETE FROM tab_impuesto_unico WHERE anio = 2099 AND mes = 12 AND tramo = 9`)

	valores := map[string]string{
		"anio": "2099", "mes": "12", "tramo": "9",
		"tope_minimo": "1", "tope_maximo": "2",
		"porcentaje": "3", "cantidad_rebajar": "4",
	}
	if err := repo.Guardar(def, valores, nil, false); err != nil {
		t.Fatal(err)
	}
	fila, err := repo.Fila(def, clave)
	if err != nil {
		t.Fatal(err)
	}
	if fila["porcentaje"] != "3" {
		t.Fatalf("porcentaje = %q", fila["porcentaje"])
	}
	valores["porcentaje"] = "5"
	if err := repo.Guardar(def, valores, clave, true); err != nil {
		t.Fatal(err)
	}
	fila, err = repo.Fila(def, clave)
	if err != nil {
		t.Fatal(err)
	}
	if fila["porcentaje"] != "5" {
		t.Fatalf("porcentaje = %q", fila["porcentaje"])
	}
	if _, err := repo.Eliminar(def, clave); err != nil {
		t.Fatal(err)
	}
}
