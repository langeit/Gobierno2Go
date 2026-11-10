package repository

import (
	"testing"

	"Gobierno2Go/internal/config"
	"Gobierno2Go/internal/models"
)

func TestNormalizarRut(t *testing.T) {
	casos := map[string]string{
		" 12.345.678-9 ": "123456789",
		"76123456-k":     "76123456K",
		"76123456K":      "76123456K",
	}
	for in, want := range casos {
		if got := NormalizarRut(in); got != want {
			t.Errorf("NormalizarRut(%q) = %q, quiere %q", in, got, want)
		}
	}
}

func TestPrepararIngreso(t *testing.T) {
	base := models.IngresoBien{
		TipoDocCompra: "fa",
		NumDocCompra:  10,
		TipoDocOrigen: "re",
		NumDocOrigen:  20,
		RutProveedor:  "76.123.456-K",
		Items: []models.IngresoItem{{
			CorrTbien: 1, Descripcion: " Silla ", Monto: 1500,
			CorrZona: 1, Piso: "1", Oficina: "12", CorrAsig: 3, Fuente: 1,
		}, {}},
	}
	got, err := prepararIngreso(base)
	if err != nil {
		t.Fatal(err)
	}
	if got.RutProveedor != "76123456K" || got.TipoDocCompra != "FA" || got.TipoDocOrigen != "RE" {
		t.Fatalf("cabecera normalizada inesperada: %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].Descripcion != "Silla" {
		t.Fatalf("items: %+v", got.Items)
	}

	base.Items[0].CorrAsig = 0
	if _, err := prepararIngreso(base); err == nil {
		t.Fatal("la imputación vacía debe rechazarse")
	} else if _, ok := MensajeUsuario(err); !ok {
		t.Fatalf("el rechazo debe poder mostrarse: %v", err)
	}
}

func TestCrearIngresoCiclo(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	var idUsuario, rut string
	var tipo, zona, asig int
	err = db.QueryRow(`SELECT id_usuario FROM gen_usuario ORDER BY id_usuario LIMIT 1`).Scan(&idUsuario)
	if err != nil {
		t.Skip("sin usuario:", err)
	}
	err = db.QueryRow(`SELECT btrim(rut_proveedor) FROM adu_proveedormm WHERE estado = 'V' ORDER BY rut_proveedor LIMIT 1`).Scan(&rut)
	if err != nil {
		t.Skip("sin proveedor:", err)
	}
	err = db.QueryRow(`SELECT corr_tbien FROM inv_tipo_bien ORDER BY corr_tbien LIMIT 1`).Scan(&tipo)
	if err != nil {
		t.Skip("sin tipo de bien:", err)
	}
	err = db.QueryRow(`SELECT corr_zona FROM inv_zona ORDER BY corr_zona LIMIT 1`).Scan(&zona)
	if err != nil {
		t.Skip("sin zona:", err)
	}
	err = db.QueryRow(`SELECT corr_asig FROM adu_asignacion WHERE estado = 'V' ORDER BY corr_asig LIMIT 1`).Scan(&asig)
	if err != nil {
		t.Skip("sin imputación:", err)
	}

	const origen int64 = 880000021
	const factura int64 = 880000022
	limpiar := func() {
		db.Exec(`DELETE FROM inv_imputacion_bien WHERE corr_bien IN (
			SELECT corr_bien FROM inv_bien WHERE nro_docto_compra = $1 AND btrim(tipo_docto_compra) = 'FA' AND btrim(rut_proveedor) = $2)`, factura, rut)
		db.Exec(`DELETE FROM inv_det_movimiento WHERE corr_bien IN (
			SELECT corr_bien FROM inv_bien WHERE nro_docto_compra = $1 AND btrim(tipo_docto_compra) = 'FA' AND btrim(rut_proveedor) = $2)`, factura, rut)
		db.Exec(`DELETE FROM inv_ubicacion WHERE corr_bien IN (
			SELECT corr_bien FROM inv_bien WHERE nro_docto_compra = $1 AND btrim(tipo_docto_compra) = 'FA' AND btrim(rut_proveedor) = $2)`, factura, rut)
		db.Exec(`DELETE FROM inv_bien WHERE nro_docto_compra = $1 AND btrim(tipo_docto_compra) = 'FA' AND btrim(rut_proveedor) = $2`, factura, rut)
		db.Exec(`DELETE FROM inv_movimiento WHERE num_doc = $1 AND btrim(tipo_doc) = 'RE' AND concepto = 'I'`, origen)
	}
	limpiar()
	t.Cleanup(limpiar)

	repo := NewInventarioRepository(db)
	in := models.IngresoBien{
		TipoDocCompra: "FA",
		NumDocCompra:  factura,
		TipoDocOrigen: "RE",
		NumDocOrigen:  origen,
		RutProveedor:  rut[:len(rut)-1] + "-" + rut[len(rut)-1:],
		Items: []models.IngresoItem{{
			CorrTbien: tipo, Descripcion: "Bien de prueba ciclo", Monto: 1000,
			CorrZona: zona, Piso: "1", Oficina: "2", CorrAsig: asig, Fuente: 1,
		}},
	}
	codigos, err := repo.CrearIngreso(in, idUsuario)
	if err != nil {
		t.Fatal(err)
	}
	if len(codigos) != 1 {
		t.Fatalf("códigos: %v", codigos)
	}

	var concepto, tipoDoc string
	var numDoc int64
	err = db.QueryRow(`
		SELECT concepto, num_doc, btrim(tipo_doc)
		FROM inv_movimiento m
		JOIN inv_det_movimiento d ON d.corr_movi = m.corr_movi
		JOIN inv_bien b ON b.corr_bien = d.corr_bien
		WHERE b.codigo_bien = $1`, codigos[0]).Scan(&concepto, &numDoc, &tipoDoc)
	if err != nil {
		t.Fatal(err)
	}
	if concepto != "I" || numDoc != origen || tipoDoc != "RE" {
		t.Fatalf("movimiento = %s %s %d", concepto, tipoDoc, numDoc)
	}

	var imputaciones int
	if err := db.QueryRow(`
		SELECT count(*) FROM inv_imputacion_bien i
		JOIN inv_bien b ON b.corr_bien = i.corr_bien
		WHERE b.codigo_bien = $1 AND i.corr_asig = $2`, codigos[0], asig).Scan(&imputaciones); err != nil {
		t.Fatal(err)
	}
	if imputaciones != 1 {
		t.Fatalf("imputaciones = %d", imputaciones)
	}

	if _, err := repo.CrearIngreso(in, idUsuario); err == nil {
		t.Fatal("el mismo documento de origen debía rechazarse")
	} else if msg, ok := MensajeUsuario(err); !ok || msg == "" {
		t.Fatalf("rechazo sin mensaje de usuario: %v", err)
	}
}
