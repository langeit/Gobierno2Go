package repository

import (
	"database/sql"
	"errors"
	"fmt"
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

func TestListarPaginadoBusquedaYOrden(t *testing.T) {
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
	t.Cleanup(func() { db.Exec(`DELETE FROM tab_region WHERE codigo LIKE 'ZP%'`) })
	db.Exec(`DELETE FROM tab_region WHERE codigo LIKE 'ZP%'`)

	for i, nombre := range []string{"ZZ Prueba C", "ZZ Prueba A", "ZZ Prueba B"} {
		valores := map[string]string{"codigo": fmt.Sprintf("ZP%d", i), "descripcion": nombre}
		if err := repo.Guardar(def, valores, nil, false); err != nil {
			t.Fatal(err)
		}
	}

	filas, total, pagina, err := repo.Listar(def, ListaOpciones{
		Busqueda: "ZZ Prueba", OrdenColumna: "descripcion", Ascendente: true, Pagina: 1, PorPagina: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || pagina != 1 || len(filas) != 2 {
		t.Fatalf("total=%d pagina=%d filas=%d", total, pagina, len(filas))
	}
	if filas[0]["descripcion"] != "ZZ Prueba A" || filas[1]["descripcion"] != "ZZ Prueba B" {
		t.Fatalf("orden inesperado: %q, %q", filas[0]["descripcion"], filas[1]["descripcion"])
	}

	filas, total, pagina, err = repo.Listar(def, ListaOpciones{
		Busqueda: "ZZ Prueba", OrdenColumna: "descripcion", Ascendente: true, Pagina: 2, PorPagina: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || pagina != 2 || len(filas) != 1 || filas[0]["descripcion"] != "ZZ Prueba C" {
		t.Fatalf("segunda página inesperada: total=%d pagina=%d filas=%v", total, pagina, filas)
	}

	filas, _, _, err = repo.Listar(def, ListaOpciones{
		Busqueda: "ZZ Prueba", OrdenColumna: "descripcion", Ascendente: false, Pagina: 1, PorPagina: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filas[0]["descripcion"] != "ZZ Prueba C" || filas[1]["descripcion"] != "ZZ Prueba B" {
		t.Fatalf("orden descendente inesperado: %q, %q", filas[0]["descripcion"], filas[1]["descripcion"])
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
