package repository

import (
	"testing"
	"time"

	"Gobierno2Go/internal/config"
	"Gobierno2Go/internal/models"
)

func TestOperacionAdquisiciones(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewAdquisicionesRepository(db)
	if err := repo.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureSchema(); err != nil {
		t.Fatal("el esquema operacional debe poder aplicarse dos veces:", err)
	}

	var usuario string
	if err := db.QueryRow(`SELECT id_usuario FROM gen_usuario ORDER BY id_usuario LIMIT 1`).Scan(&usuario); err != nil {
		t.Skip("sin usuarios:", err)
	}
	var depto int
	if err := db.QueryRow(`SELECT id_depto FROM gen_departamento WHERE estado = 'V' ORDER BY id_depto LIMIT 1`).Scan(&depto); err != nil {
		t.Skip("sin departamentos:", err)
	}
	var asig int
	if err := db.QueryRow(`SELECT corr_asig FROM adu_asignacion WHERE estado = 'V' ORDER BY corr_asig LIMIT 1`).Scan(&asig); err != nil {
		t.Fatal(err)
	}

	const (
		nroOrden = 990001
		nomMat   = "ZZ Material operacion"
		rut      = "ZZ333333K"
	)
	limpiar := func() {
		db.Exec(`DELETE FROM adu_detalle_facturapr WHERE cod_material IN (SELECT cod_material FROM adu_materialmm WHERE nombre_material = $1)`, nomMat)
		db.Exec(`DELETE FROM adu_facturapr WHERE nro_orden_compra = $1`, nroOrden)
		db.Exec(`
			WITH borradas AS (
				DELETE FROM adu_detalle_sol_mat d
				USING adu_materialmm m
				WHERE d.cod_material = m.cod_material AND m.nombre_material = $1
				RETURNING d.nro_sol_mat
			)
			DELETE FROM adu_solicitud_materiales
			WHERE nro_sol_mat IN (SELECT nro_sol_mat FROM borradas)`, nomMat)
		db.Exec(`DELETE FROM adu_stock WHERE cod_material IN (SELECT cod_material FROM adu_materialmm WHERE nombre_material = $1)`, nomMat)
		db.Exec(`DELETE FROM adu_detalle_ordenco WHERE nro_orden_compra = $1`, nroOrden)
		db.Exec(`DELETE FROM adu_imputacion_orden WHERE nro_orden_compra = $1`, nroOrden)
		db.Exec(`DELETE FROM adu_orden_compra WHERE nro_orden_compra = $1`, nroOrden)
		db.Exec(`DELETE FROM adu_materialmm WHERE nombre_material = $1`, nomMat)
		db.Exec(`DELETE FROM adu_proveedormm WHERE btrim(rut_proveedor) = $1`, rut)
	}
	limpiar()
	t.Cleanup(limpiar)

	if err := repo.CrearMaterial(models.Material{Nombre: nomMat, Tipo: 1, StockCritico: 1}); err != nil {
		t.Fatal(err)
	}
	mats, err := repo.MaterialesCatalogo(1)
	if err != nil {
		t.Fatal(err)
	}
	var cod int
	for _, m := range mats {
		if m.Nombre == nomMat {
			cod = m.Codigo
		}
	}
	if cod == 0 {
		t.Fatal("no apareció el material")
	}
	if err := repo.CrearProveedor(models.Proveedor{Rut: rut, Nombre: "ZZ Proveedor operacion"}); err != nil {
		t.Fatal(err)
	}

	entrada := models.OrdenEntrada{
		Numero: nroOrden, TipoOrden: models.TipoOrdenPedido, RutProveedor: rut, IDUsuario: usuario,
		ModoOrden: models.ModoOrdenMateriales, TipoImpuesto: 21,
		Lineas:       []models.LineaCantidad{{CodMaterial: cod, Cantidad: 4, ValorUnitario: 100}},
		Imputaciones: []models.ImputacionLinea{{CorrAsig: asig, Monto: 400}},
	}
	if err := repo.CrearOrden(entrada); err != nil {
		t.Fatal(err)
	}
	if err := repo.CrearOrden(entrada); err == nil {
		t.Fatal("debía rechazar el número de orden repetido")
	}
	orden, err := repo.Orden(nroOrden)
	if err != nil {
		t.Fatal(err)
	}
	if orden.TotalFinal != 400 || !orden.Editable || !orden.Eliminable || orden.TipoOrden != models.TipoOrdenPedido {
		t.Fatalf("orden = %+v", orden)
	}

	idFact, err := repo.CrearFactura(models.FacturaEntrada{
		NroOrden: nroOrden, NroFactura: 880001, FechaFactura: time.Now().Format("2006-01-02"),
		Modo: models.ModoFacturaMateriales, TipoImpuesto: 21, IDUsuario: usuario,
		Lineas: []models.LineaCantidad{{
			Correlativo: orden.Lineas[0].Correlativo, CodMaterial: cod, Cantidad: 4, ValorUnitario: 100,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idFact == 0 {
		t.Fatal("factura sin correlativo")
	}
	var stock int
	if err := db.QueryRow(`SELECT COALESCE(SUM(stock_actual), 0) FROM adu_stock_actual WHERE cod_material = $1`, cod).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 4 {
		t.Fatalf("stock tras factura = %d", stock)
	}
	orden, err = repo.Orden(nroOrden)
	if err != nil {
		t.Fatal(err)
	}
	if orden.Eliminable || orden.Editable {
		t.Fatal("una orden facturada no se edita ni se elimina")
	}

	nroSol, err := repo.CrearSolicitud(models.SolicitudEntrada{
		IDUsuario: usuario, IDDepto: depto,
		Lineas: []models.LineaCantidad{{CodMaterial: cod, Cantidad: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.AprobarSolicitud(nroSol, true, []models.LineaCantidad{{CodMaterial: cod, Cantidad: 1}}); err != nil {
		t.Fatal(err)
	}
	sol, err := repo.Solicitud(nroSol)
	if err != nil {
		t.Fatal(err)
	}
	if sol.Estado != models.EstadoSolPendienteEntrega || sol.VistoBueno != "S" || len(sol.Lineas) != 1 {
		t.Fatalf("solicitud aprobada = %+v", sol)
	}
	if err := repo.EntregarSolicitud(nroSol, map[int]int{sol.Lineas[0].Correlativo: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COALESCE(SUM(stock_actual), 0) FROM adu_stock_actual WHERE cod_material = $1`, cod).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 3 {
		t.Fatalf("stock tras entrega = %d", stock)
	}
	if _, err := repo.ReporteConsumo(models.FiltroReporte{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReporteStock(models.FiltroReporte{}); err != nil {
		t.Fatal(err)
	}
}
