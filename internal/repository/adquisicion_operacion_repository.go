package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
)

func datosf(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrDatos}, args...)...)
}

func fechaONil(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return nil, datosf("La fecha %q no es válida", s)
	}
	return s, nil
}

func fmtFecha(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("02-01-2006")
}

func fmtHora(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	h := strings.TrimSpace(s.String)
	if len(h) >= 8 {
		return h[:8]
	}
	return h
}

// ---------- Catálogos de la operación ----------

func (r *AdquisicionesRepository) Departamentos() ([]models.Departamento, error) {
	rows, err := r.db.Query(`SELECT id_depto, descripcion FROM gen_departamento WHERE estado = 'V' ORDER BY descripcion`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Departamento
	for rows.Next() {
		var d models.Departamento
		if err := rows.Scan(&d.IDDepto, &d.Descripcion); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) DepartamentoUsuario(idUsuario string) (int, string, error) {
	var id int
	var nombre string
	err := r.db.QueryRow(`
		SELECT CASE WHEN dep.id_depto IS NULL THEN 0 ELSE COALESCE(d.depto, 0) END,
		       COALESCE(dep.descripcion, '')
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto AND dep.estado = 'V'
		WHERE u.id_usuario = $1`, idUsuario).Scan(&id, &nombre)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	return id, nombre, err
}

func (r *AdquisicionesRepository) MaterialesCatalogo(tipo int) ([]models.Material, error) {
	rows, err := r.db.Query(`
		SELECT m.cod_material, m.nombre_material, m.tipo_material,
		       COALESCE(d.desc_dominio, ''), m.stock_critico
		FROM adu_materialmm m
		LEFT JOIN adu_dominio d ON d.cod_dominio = m.tipo_material
		WHERE m.estado = 'V' AND ($1 = 0 OR m.tipo_material = $1)
		ORDER BY m.nombre_material`, tipo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Material
	for rows.Next() {
		var m models.Material
		if err := rows.Scan(&m.Codigo, &m.Nombre, &m.Tipo, &m.TipoNombre, &m.StockCritico); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) SiguienteOrden() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COALESCE(MAX(nro_orden_compra), 0) + 1 FROM adu_orden_compra`).Scan(&n)
	return n, err
}

func (r *AdquisicionesRepository) porcentajeImpuesto(cod int) (float64, error) {
	var desc string
	err := r.db.QueryRow(`
		SELECT btrim(desc_dominio) FROM adu_dominio
		WHERE cod_dominio = $1 AND cod_tipo_dom = $2 AND estado = 'V'`, cod, models.CodTipoImpuesto).Scan(&desc)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, datosf("Seleccione un tipo de impuesto")
	}
	if err != nil {
		return 0, err
	}
	desc = strings.TrimSpace(strings.TrimSuffix(desc, "%"))
	v, err := strconv.ParseFloat(desc, 64)
	if err != nil || v < 0 {
		return 0, datosf("El impuesto %q no es un porcentaje", desc)
	}
	return v, nil
}

func (r *AdquisicionesRepository) validarLineasOrden(lineas []models.LineaCantidad) error {
	if len(lineas) == 0 {
		return nil
	}
	vistos := map[int]bool{}
	for _, l := range lineas {
		if l.Cantidad <= 0 || l.ValorUnitario < 0 || l.CodMaterial <= 0 {
			return datosf("Cada línea de material necesita un material, una cantidad y un valor unitario")
		}
		if vistos[l.CodMaterial] {
			return datosf("El material está repetido en la orden")
		}
		vistos[l.CodMaterial] = true
		ok, err := r.materialExiste(l.CodMaterial)
		if err != nil {
			return err
		}
		if !ok {
			return datosf("Hay un material que no existe")
		}
	}
	return nil
}

func (r *AdquisicionesRepository) materialExiste(id int) (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT 1 FROM adu_materialmm WHERE cod_material = $1 AND estado = 'V'`, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *AdquisicionesRepository) asignacionExiste(id int) (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT 1 FROM adu_asignacion WHERE corr_asig = $1 AND estado = 'V'`, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *AdquisicionesRepository) validarOrden(in models.OrdenEntrada, exigirLineas bool) error {
	if in.Numero <= 0 {
		return datosf("Indique el número de la orden")
	}
	if strings.TrimSpace(in.IDUsuario) == "" {
		return datosf("No hay un usuario de sesión")
	}
	if in.ModoOrden != models.ModoOrdenMateriales && in.ModoOrden != models.ModoOrdenServicios {
		return datosf("Seleccione si la orden es de materiales o de servicios")
	}
	ok, err := r.dominioEsTipo(in.TipoOrden, models.CodTipoOrden)
	if err != nil {
		return err
	}
	if !ok {
		return datosf("Seleccione el tipo de orden")
	}
	prov, err := r.Proveedor(in.RutProveedor)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return datosf("Seleccione un proveedor")
		}
		return err
	}
	in.RutProveedor = prov.Rut
	if _, err := r.porcentajeImpuesto(in.TipoImpuesto); err != nil {
		return err
	}
	if err := r.validarLineasOrden(in.Lineas); err != nil {
		return err
	}
	if in.ModoOrden == models.ModoOrdenMateriales {
		if exigirLineas && len(in.Lineas) == 0 {
			return datosf("Agregue al menos un material")
		}
	} else if strings.TrimSpace(in.DescServicio) == "" || in.CantServicio <= 0 || in.ValorUnitServicio < 0 {
		return datosf("El servicio necesita descripción, cantidad y valor unitario")
	}
	if len(in.Imputaciones) == 0 {
		return datosf("Agregue al menos una imputación presupuestaria")
	}
	for _, im := range in.Imputaciones {
		if im.Monto <= 0 || im.CorrAsig <= 0 {
			return datosf("Cada imputación necesita una asignación y un monto")
		}
		ok, err := r.asignacionExiste(im.CorrAsig)
		if err != nil {
			return err
		}
		if !ok {
			return datosf("Hay una asignación que no existe")
		}
	}
	return nil
}

func totalesOrden(in models.OrdenEntrada, porcentaje float64) (sub, iva, total float64) {
	for _, l := range in.Lineas {
		sub += float64(l.Cantidad * l.ValorUnitario)
	}
	if in.ModoOrden == models.ModoOrdenServicios {
		sub += in.CantServicio * in.ValorUnitServicio
	}
	iva = sub * porcentaje / 100
	return sub, iva, sub + iva
}

// ---------- Órdenes de compra ----------

func (r *AdquisicionesRepository) CrearOrden(in models.OrdenEntrada) error {
	if err := r.validarOrden(in, true); err != nil {
		return err
	}
	if prov, errProv := r.Proveedor(in.RutProveedor); errProv == nil {
		in.RutProveedor = prov.Rut
	}
	pct, err := r.porcentajeImpuesto(in.TipoImpuesto)
	if err != nil {
		return err
	}
	sub, iva, total := totalesOrden(in, pct)
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var ya int
	err = tx.QueryRow(`SELECT 1 FROM adu_orden_compra WHERE nro_orden_compra = $1`, in.Numero).Scan(&ya)
	if err == nil {
		return datosf("Ese número de orden ya está registrado")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var desc, cant, valor any
	if in.ModoOrden == models.ModoOrdenServicios {
		desc = strings.TrimSpace(in.DescServicio)
		cant = in.CantServicio
		valor = in.ValorUnitServicio
	}
	_, err = tx.Exec(`
		INSERT INTO adu_orden_compra (
			nro_orden_compra, tipo_orden, fecha_orden_compra, rut_proveedor, id_usuario,
			total_orden, iva_orden, total_final_orden, desc_servicio, cant_servicio,
			valorunit_servicio, tipo_impuesto, modo_orden)
		VALUES ($1, $2, CURRENT_DATE, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		in.Numero, in.TipoOrden, in.RutProveedor, in.IDUsuario,
		sub, iva, total, desc, cant, valor, in.TipoImpuesto, in.ModoOrden)
	if err != nil {
		return err
	}
	if err := insertarDetalleOrden(tx, in.Numero, in.Lineas); err != nil {
		return err
	}
	if err := insertarImputaciones(tx, in.Numero, in.Imputaciones); err != nil {
		return err
	}
	return tx.Commit()
}

func insertarDetalleOrden(tx *sql.Tx, nro int, lineas []models.LineaCantidad) error {
	for _, l := range lineas {
		_, err := tx.Exec(`
			INSERT INTO adu_detalle_ordenco (
				nro_orden_compra, cod_material, cantidad_detalleord, valor_unitario_ord, valor_totalmat, facturado)
			VALUES ($1, $2, $3, $4, $5, 'N')`,
			nro, l.CodMaterial, l.Cantidad, l.ValorUnitario, l.Cantidad*l.ValorUnitario)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertarImputaciones(tx *sql.Tx, nro int, lineas []models.ImputacionLinea) error {
	for _, im := range lineas {
		_, err := tx.Exec(`
			INSERT INTO adu_imputacion_orden (corr_asig, nro_orden_compra, monto)
			VALUES ($1, $2, $3)`, im.CorrAsig, nro, im.Monto)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *AdquisicionesRepository) ActualizarOrden(in models.OrdenEntrada) error {
	actual, err := r.Orden(in.Numero)
	if err != nil {
		return err
	}
	if !actual.Editable {
		return datosf("La orden ya está facturada y no se puede modificar")
	}
	in.ModoOrden = actual.ModoOrden
	if actual.ServicioFacturado {
		in.DescServicio = actual.DescServicio
		in.CantServicio = actual.CantServicio
		in.ValorUnitServicio = actual.ValorUnitServicio
	}
	tieneFacturadas := false
	for _, l := range actual.Lineas {
		if l.Facturado {
			tieneFacturadas = true
			break
		}
	}
	if err := r.validarOrden(in, !tieneFacturadas); err != nil {
		return err
	}
	if prov, errProv := r.Proveedor(in.RutProveedor); errProv == nil {
		in.RutProveedor = prov.Rut
	}
	pct, err := r.porcentajeImpuesto(in.TipoImpuesto)
	if err != nil {
		return err
	}
	sub, _, _ := totalesOrden(in, pct)
	for _, l := range actual.Lineas {
		if l.Facturado {
			sub += float64(l.ValorTotal)
		}
	}
	iva := sub * pct / 100
	total := sub + iva

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var desc, cant, valor any
	if actual.ModoOrden == models.ModoOrdenServicios {
		desc = strings.TrimSpace(in.DescServicio)
		cant = in.CantServicio
		valor = in.ValorUnitServicio
	}
	_, err = tx.Exec(`
		UPDATE adu_orden_compra
		SET tipo_orden = $2, rut_proveedor = $3, id_usuario = $4,
		    total_orden = $5, iva_orden = $6, total_final_orden = $7,
		    desc_servicio = $8, cant_servicio = $9, valorunit_servicio = $10, tipo_impuesto = $11
		WHERE nro_orden_compra = $1 AND estado = 'V'`,
		in.Numero, in.TipoOrden, in.RutProveedor, in.IDUsuario,
		sub, iva, total, desc, cant, valor, in.TipoImpuesto)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE adu_detalle_ordenco SET estado = 'E'
		WHERE nro_orden_compra = $1 AND btrim(facturado) = 'N' AND estado = 'V'`, in.Numero); err != nil {
		return err
	}
	if err := insertarDetalleOrden(tx, in.Numero, in.Lineas); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE adu_imputacion_orden SET estado = 'E'
		WHERE nro_orden_compra = $1 AND estado = 'V'`, in.Numero); err != nil {
		return err
	}
	if err := insertarImputaciones(tx, in.Numero, in.Imputaciones); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AdquisicionesRepository) EliminarOrden(nro int) error {
	actual, err := r.Orden(nro)
	if err != nil {
		return err
	}
	if !actual.Eliminable {
		return datosf("No se puede eliminar una orden con líneas facturadas")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE adu_imputacion_orden SET estado = 'E' WHERE nro_orden_compra = $1 AND estado = 'V'`, nro); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE adu_detalle_ordenco SET estado = 'E' WHERE nro_orden_compra = $1 AND estado = 'V'`, nro); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE adu_orden_compra SET estado = 'E' WHERE nro_orden_compra = $1 AND estado = 'V'`, nro); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AdquisicionesRepository) Ordenes() ([]models.OrdenCompra, error) {
	rows, err := r.db.Query(`
		SELECT o.nro_orden_compra, o.tipo_orden, COALESCE(td.desc_dominio, ''),
		       o.fecha_orden_compra, btrim(o.rut_proveedor), COALESCE(p.nombre_proveedor, ''),
		       o.total_final_orden, COALESCE(o.modo_orden, 0), COALESCE(md.desc_dominio, ''),
		       EXISTS (
		           SELECT 1 FROM adu_detalle_ordenco d
		           WHERE d.nro_orden_compra = o.nro_orden_compra AND d.estado = 'V' AND btrim(d.facturado) = 'N')
		FROM adu_orden_compra o
		LEFT JOIN adu_dominio td ON td.cod_dominio = o.tipo_orden
		LEFT JOIN adu_dominio md ON md.cod_dominio = o.modo_orden
		LEFT JOIN adu_proveedormm p ON p.rut_proveedor = o.rut_proveedor
		WHERE o.estado = 'V'
		ORDER BY o.nro_orden_compra DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OrdenCompra
	for rows.Next() {
		var o models.OrdenCompra
		var fecha time.Time
		if err := rows.Scan(&o.Numero, &o.TipoOrden, &o.TipoNombre, &fecha, &o.RutProveedor, &o.Proveedor,
			&o.TotalFinal, &o.ModoOrden, &o.ModoNombre, &o.TienePendientes); err != nil {
			return nil, err
		}
		o.Fecha = fecha.Format("02-01-2006")
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) Orden(nro int) (models.OrdenCompra, error) {
	var o models.OrdenCompra
	var fecha time.Time
	var desc, impuesto sql.NullString
	var cant, valor sql.NullFloat64
	var tipoImp, modo sql.NullInt64
	err := r.db.QueryRow(`
		SELECT o.nro_orden_compra, o.tipo_orden, COALESCE(td.desc_dominio, ''),
		       o.fecha_orden_compra, btrim(o.rut_proveedor), COALESCE(p.nombre_proveedor, ''),
		       btrim(o.id_usuario), COALESCE(u.nombre, ''),
		       o.total_orden, o.iva_orden, o.total_final_orden,
		       o.desc_servicio, o.cant_servicio, o.valorunit_servicio,
		       o.tipo_impuesto, COALESCE(imp.desc_dominio, ''),
		       o.modo_orden, COALESCE(md.desc_dominio, '')
		FROM adu_orden_compra o
		LEFT JOIN adu_dominio td ON td.cod_dominio = o.tipo_orden
		LEFT JOIN adu_dominio imp ON imp.cod_dominio = o.tipo_impuesto
		LEFT JOIN adu_dominio md ON md.cod_dominio = o.modo_orden
		LEFT JOIN adu_proveedormm p ON p.rut_proveedor = o.rut_proveedor
		LEFT JOIN gen_usuario u ON u.id_usuario = o.id_usuario
		WHERE o.nro_orden_compra = $1 AND o.estado = 'V'`, nro).Scan(
		&o.Numero, &o.TipoOrden, &o.TipoNombre, &fecha, &o.RutProveedor, &o.Proveedor,
		&o.IDUsuario, &o.Usuario, &o.Total, &o.IVA, &o.TotalFinal,
		&desc, &cant, &valor, &tipoImp, &impuesto, &modo, &o.ModoNombre)
	if err != nil {
		return o, err
	}
	o.Fecha = fecha.Format("02-01-2006")
	if desc.Valid {
		o.DescServicio = desc.String
	}
	if cant.Valid {
		o.CantServicio = cant.Float64
	}
	if valor.Valid {
		o.ValorUnitServicio = valor.Float64
	}
	if tipoImp.Valid {
		o.TipoImpuesto = int(tipoImp.Int64)
	}
	o.ImpuestoNombre = strings.TrimSpace(impuesto.String)
	if pct, errPct := strconv.ParseFloat(strings.TrimSuffix(o.ImpuestoNombre, "%"), 64); errPct == nil {
		o.Porcentaje = pct
	}
	if modo.Valid {
		o.ModoOrden = int(modo.Int64)
	}
	lineas, err := r.lineasOrden(nro, false)
	if err != nil {
		return o, err
	}
	o.Lineas = lineas
	imputs, err := r.imputacionesOrden(nro)
	if err != nil {
		return o, err
	}
	o.Imputaciones = imputs
	err = r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM adu_facturapr WHERE nro_orden_compra = $1)`, nro).Scan(&o.TieneFactura)
	if err != nil {
		return o, err
	}
	err = r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM adu_facturapr
			WHERE nro_orden_compra = $1 AND desc_servicio IS NOT NULL)`, nro).Scan(&o.ServicioFacturado)
	if err != nil {
		return o, err
	}
	for _, l := range o.Lineas {
		if !l.Facturado {
			o.TienePendientes = true
			break
		}
	}
	facturadaAlguna := false
	for _, l := range o.Lineas {
		if l.Facturado {
			facturadaAlguna = true
			break
		}
	}
	servicio := strings.TrimSpace(o.DescServicio) != ""
	o.Editable = o.TienePendientes || (servicio && !o.TieneFactura)
	o.Eliminable = !facturadaAlguna && !o.TieneFactura
	return o, nil
}

func (r *AdquisicionesRepository) lineasOrden(nro int, soloPendientes bool) ([]models.LineaCantidad, error) {
	q := `
		SELECT d.corr_det_ordenco, d.cod_material, m.nombre_material,
		       d.cantidad_detalleord, d.valor_unitario_ord, COALESCE(d.valor_totalmat, 0), btrim(d.facturado)
		FROM adu_detalle_ordenco d
		JOIN adu_materialmm m ON m.cod_material = d.cod_material
		WHERE d.nro_orden_compra = $1 AND d.estado = 'V'`
	if soloPendientes {
		q += ` AND btrim(d.facturado) = 'N'`
	}
	q += ` ORDER BY d.corr_det_ordenco`
	rows, err := r.db.Query(q, nro)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.LineaCantidad
	for rows.Next() {
		var l models.LineaCantidad
		var fact string
		if err := rows.Scan(&l.Correlativo, &l.CodMaterial, &l.Nombre, &l.Cantidad, &l.ValorUnitario, &l.ValorTotal, &fact); err != nil {
			return nil, err
		}
		l.Facturado = fact == "S"
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) imputacionesOrden(nro int) ([]models.ImputacionLinea, error) {
	rows, err := r.db.Query(`
		SELECT i.corr_imput, i.corr_asig, btrim(a.cod_asig), a.descripcion, i.monto
		FROM adu_imputacion_orden i
		JOIN adu_asignacion a ON a.corr_asig = i.corr_asig
		WHERE i.nro_orden_compra = $1 AND i.estado = 'V'
		ORDER BY i.corr_imput`, nro)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ImputacionLinea
	for rows.Next() {
		var im models.ImputacionLinea
		if err := rows.Scan(&im.Correlativo, &im.CorrAsig, &im.Codigo, &im.Descripcion, &im.Monto); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) OrdenesPorFacturar(modo int, rut string) ([]models.OrdenCompra, error) {
	rut = strings.TrimSpace(rut)
	q := `
		SELECT o.nro_orden_compra, o.fecha_orden_compra, btrim(o.rut_proveedor), COALESCE(p.nombre_proveedor, '')
		FROM adu_orden_compra o
		LEFT JOIN adu_proveedormm p ON p.rut_proveedor = o.rut_proveedor
		WHERE o.estado = 'V' AND ($1 = '' OR btrim(o.rut_proveedor) = $1) AND `
	if modo == models.ModoFacturaServicios {
		q += `o.desc_servicio IS NOT NULL AND (
			o.nro_orden_compra NOT IN (SELECT nro_orden_compra FROM adu_facturapr WHERE nro_orden_compra IS NOT NULL)
			OR o.nro_orden_compra IN (
				SELECT DISTINCT nro_orden_compra FROM adu_detalle_ordenco WHERE estado = 'V' AND btrim(facturado) = 'N'))`
	} else {
		q += `o.desc_servicio IS NULL AND o.nro_orden_compra IN (
			SELECT DISTINCT nro_orden_compra FROM adu_detalle_ordenco WHERE estado = 'V' AND btrim(facturado) = 'N')`
	}
	q += ` ORDER BY o.nro_orden_compra`
	rows, err := r.db.Query(q, rut)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OrdenCompra
	for rows.Next() {
		var o models.OrdenCompra
		var fecha time.Time
		if err := rows.Scan(&o.Numero, &fecha, &o.RutProveedor, &o.Proveedor); err != nil {
			return nil, err
		}
		o.Fecha = fecha.Format("02-01-2006")
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------- Facturas ----------

func (r *AdquisicionesRepository) CrearFactura(in models.FacturaEntrada) (int, error) {
	if in.NroFactura <= 0 {
		return 0, datosf("Indique el número de factura")
	}
	fecha, err := fechaONil(in.FechaFactura)
	if err != nil {
		return 0, err
	}
	if fecha == nil {
		return 0, datosf("Indique la fecha de la factura")
	}
	if in.Modo != models.ModoFacturaMateriales && in.Modo != models.ModoFacturaServicios {
		return 0, datosf("Seleccione el tipo de factura")
	}
	orden, err := r.Orden(in.NroOrden)
	if err != nil {
		return 0, err
	}
	servicio := strings.TrimSpace(orden.DescServicio) != ""
	if in.Modo == models.ModoFacturaMateriales && servicio {
		return 0, datosf("Esa orden es de servicios")
	}
	if in.Modo == models.ModoFacturaServicios && !servicio {
		return 0, datosf("Esa orden es de materiales")
	}
	pct, err := r.porcentajeImpuesto(in.TipoImpuesto)
	if err != nil {
		return 0, err
	}
	if err := r.validarLineasFactura(orden, in.Lineas); err != nil {
		return 0, err
	}
	guardaServicio := in.Modo == models.ModoFacturaServicios && !orden.ServicioFacturado
	if guardaServicio && (strings.TrimSpace(in.DescServicio) == "" || in.CantServicio <= 0 || in.ValorUnitServicio < 0) {
		return 0, datosf("Indique la descripción, cantidad y valor del servicio")
	}
	if !guardaServicio && len(in.Lineas) == 0 {
		return 0, datosf("Seleccione al menos un material pendiente")
	}

	sub := 0.0
	for _, l := range in.Lineas {
		sub += float64(l.Cantidad * l.ValorUnitario)
	}
	if guardaServicio {
		sub += in.CantServicio * in.ValorUnitServicio
	}
	iva := sub * pct / 100
	total := sub + iva

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var desc, cant, valor any
	if guardaServicio {
		desc = strings.TrimSpace(in.DescServicio)
		cant = in.CantServicio
		valor = in.ValorUnitServicio
	}
	var id int
	err = tx.QueryRow(`
		INSERT INTO adu_facturapr (
			nro_orden_compra, rut_proveedor, nro_factura, fecha_factura,
			total_factura, iva_factura, total_final_factura, fecha_ingreso,
			id_usuario, modo_factura, desc_servicio, cant_servicio, valorunit_servicio, tipo_impuesto)
		VALUES ($1, $2, $3, $4::date, $5, $6, $7, CURRENT_DATE, $8, $9, $10, $11, $12, $13)
		RETURNING nro_corr_fact`,
		orden.Numero, orden.RutProveedor, in.NroFactura, fecha, sub, iva, total,
		in.IDUsuario, in.Modo, desc, cant, valor, in.TipoImpuesto).Scan(&id)
	if err != nil {
		return 0, err
	}
	ahora := time.Now()
	for _, l := range in.Lineas {
		res, err := tx.Exec(`
			UPDATE adu_detalle_ordenco SET facturado = 'S'
			WHERE corr_det_ordenco = $1 AND nro_orden_compra = $2 AND estado = 'V' AND btrim(facturado) = 'N'`,
			l.Correlativo, orden.Numero)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return 0, datosf("Una línea ya estaba facturada o no pertenece a la orden")
		}
		_, err = tx.Exec(`
			INSERT INTO adu_detalle_facturapr (
				cod_material, nro_corr_fact, cantidad_detallefact, valor_unitariofactura, valor_totalfact)
			VALUES ($1, $2, $3, $4, $5)`,
			l.CodMaterial, id, l.Cantidad, l.ValorUnitario, l.Cantidad*l.ValorUnitario)
		if err != nil {
			return 0, err
		}
		if err := sumarStock(tx, l.CodMaterial, l.ValorUnitario, l.Cantidad, ahora); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *AdquisicionesRepository) validarLineasFactura(orden models.OrdenCompra, lineas []models.LineaCantidad) error {
	pend := map[int]models.LineaCantidad{}
	for _, l := range orden.Lineas {
		if !l.Facturado {
			pend[l.Correlativo] = l
		}
	}
	for _, l := range lineas {
		orig, ok := pend[l.Correlativo]
		if !ok || orig.CodMaterial != l.CodMaterial {
			return datosf("Hay una línea que no está pendiente en la orden")
		}
		if l.Cantidad <= 0 || l.ValorUnitario < 0 {
			return datosf("Cada material facturado necesita cantidad y valor unitario")
		}
	}
	return nil
}

func sumarStock(tx *sql.Tx, cod, valor, cantidad int, cuando time.Time) error {
	var actual sql.NullInt64
	err := tx.QueryRow(`
		SELECT cantidad FROM adu_stock
		WHERE correlativo = (
			SELECT max(correlativo) FROM adu_stock
			WHERE cod_material = $1 AND valor_unidad = $2)`, cod, valor).Scan(&actual)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	nuevo := cantidad
	if actual.Valid {
		nuevo = int(actual.Int64) + cantidad
	}
	_, err = tx.Exec(`
		INSERT INTO adu_stock (cod_material, fecha, cantidad, valor_unidad, hora)
		VALUES ($1, $2::date, $3, $4, $5::time)`,
		cod, cuando.Format("2006-01-02"), nuevo, valor, cuando.Format("15:04:05"))
	return err
}

func (r *AdquisicionesRepository) Facturas() ([]models.Factura, error) {
	rows, err := r.db.Query(`
		SELECT f.nro_corr_fact, f.nro_orden_compra, f.nro_factura, f.fecha_factura,
		       f.total_final_factura, COALESCE(p.nombre_proveedor, ''), COALESCE(d.desc_dominio, '')
		FROM adu_facturapr f
		LEFT JOIN adu_proveedormm p ON p.rut_proveedor = f.rut_proveedor
		LEFT JOIN adu_dominio d ON d.cod_dominio = f.modo_factura
		ORDER BY f.nro_corr_fact DESC
		LIMIT 80`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Factura
	for rows.Next() {
		var f models.Factura
		var fecha time.Time
		if err := rows.Scan(&f.Correlativo, &f.NroOrden, &f.NroFactura, &fecha, &f.TotalFinal, &f.Proveedor, &f.ModoNombre); err != nil {
			return nil, err
		}
		f.FechaFactura = fecha.Format("02-01-2006")
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) Factura(id int) (models.Factura, error) {
	var f models.Factura
	var fecha, ingreso time.Time
	var desc, impuesto sql.NullString
	var cant, valor sql.NullFloat64
	err := r.db.QueryRow(`
		SELECT f.nro_corr_fact, f.nro_orden_compra, btrim(f.rut_proveedor), COALESCE(p.nombre_proveedor, ''),
		       f.nro_factura, f.fecha_factura, f.fecha_ingreso, f.total_factura, f.iva_factura, f.total_final_factura,
		       f.modo_factura, COALESCE(md.desc_dominio, ''), f.desc_servicio, f.cant_servicio, f.valorunit_servicio,
		       f.tipo_impuesto, COALESCE(imp.desc_dominio, ''), btrim(f.id_usuario)
		FROM adu_facturapr f
		LEFT JOIN adu_proveedormm p ON p.rut_proveedor = f.rut_proveedor
		LEFT JOIN adu_dominio md ON md.cod_dominio = f.modo_factura
		LEFT JOIN adu_dominio imp ON imp.cod_dominio = f.tipo_impuesto
		WHERE f.nro_corr_fact = $1`, id).Scan(
		&f.Correlativo, &f.NroOrden, &f.RutProveedor, &f.Proveedor,
		&f.NroFactura, &fecha, &ingreso, &f.Total, &f.IVA, &f.TotalFinal,
		&f.Modo, &f.ModoNombre, &desc, &cant, &valor, &f.TipoImpuesto, &impuesto, &f.IDUsuario)
	if err != nil {
		return f, err
	}
	f.FechaFactura = fecha.Format("02-01-2006")
	f.FechaIngreso = ingreso.Format("02-01-2006")
	if desc.Valid {
		f.DescServicio = desc.String
	}
	if cant.Valid {
		f.CantServicio = cant.Float64
	}
	if valor.Valid {
		f.ValorUnitServicio = valor.Float64
	}
	f.ImpuestoNombre = strings.TrimSpace(impuesto.String)
	rows, err := r.db.Query(`
		SELECT d.cod_material, m.nombre_material, d.cantidad_detallefact, d.valor_unitariofactura, d.valor_totalfact
		FROM adu_detalle_facturapr d
		JOIN adu_materialmm m ON m.cod_material = d.cod_material
		WHERE d.nro_corr_fact = $1
		ORDER BY d.nro_det_fact`, id)
	if err != nil {
		return f, err
	}
	defer rows.Close()
	for rows.Next() {
		var l models.LineaCantidad
		if err := rows.Scan(&l.CodMaterial, &l.Nombre, &l.Cantidad, &l.ValorUnitario, &l.ValorTotal); err != nil {
			return f, err
		}
		f.Lineas = append(f.Lineas, l)
	}
	return f, rows.Err()
}

// ---------- Solicitudes ----------

func (r *AdquisicionesRepository) CrearSolicitud(in models.SolicitudEntrada) (int, error) {
	if err := r.validarLineasSolicitud(in); err != nil {
		return 0, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int
	err = tx.QueryRow(`
		INSERT INTO adu_solicitud_materiales (fecha_sol_mat, hora_sol_mat, estado_sol, id_usuario, id_depto, vbjefe)
		VALUES (CURRENT_DATE, LOCALTIME, $1, $2, $3, 'N')
		RETURNING nro_sol_mat`, models.EstadoSolAprobacion, in.IDUsuario, in.IDDepto).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := insertarDetalleSolicitud(tx, id, in.Lineas); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *AdquisicionesRepository) validarLineasSolicitud(in models.SolicitudEntrada) error {
	if strings.TrimSpace(in.IDUsuario) == "" {
		return datosf("No hay un usuario de sesión")
	}
	var n int
	err := r.db.QueryRow(`SELECT 1 FROM gen_departamento WHERE id_depto = $1 AND estado = 'V'`, in.IDDepto).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return datosf("Seleccione el departamento solicitante")
	}
	if err != nil {
		return err
	}
	if len(in.Lineas) == 0 {
		return datosf("Agregue al menos un material")
	}
	for _, l := range in.Lineas {
		if l.CodMaterial <= 0 || l.Cantidad <= 0 {
			return datosf("Cada línea necesita un material y una cantidad")
		}
		ok, err := r.materialExiste(l.CodMaterial)
		if err != nil {
			return err
		}
		if !ok {
			return datosf("Hay un material que no existe")
		}
	}
	return nil
}

func insertarDetalleSolicitud(tx *sql.Tx, id int, lineas []models.LineaCantidad) error {
	for _, l := range lineas {
		_, err := tx.Exec(`
			INSERT INTO adu_detalle_sol_mat (cod_material, nro_sol_mat, cantidad_solicitud)
			VALUES ($1, $2, $3)`, l.CodMaterial, id, l.Cantidad)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *AdquisicionesRepository) AprobarSolicitud(id int, vistoBueno bool, lineas []models.LineaCantidad) error {
	if len(lineas) == 0 {
		return datosf("La solicitud debe conservar al menos un material")
	}
	for _, l := range lineas {
		if l.CodMaterial <= 0 || l.Cantidad <= 0 {
			return datosf("Cada línea necesita un material y una cantidad")
		}
		ok, err := r.materialExiste(l.CodMaterial)
		if err != nil {
			return err
		}
		if !ok {
			return datosf("Hay un material que no existe")
		}
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var estado int
	err = tx.QueryRow(`SELECT estado_sol FROM adu_solicitud_materiales WHERE nro_sol_mat = $1 FOR UPDATE`, id).Scan(&estado)
	if err != nil {
		return err
	}
	if estado != models.EstadoSolAprobacion {
		return datosf("La solicitud ya fue aprobada o rechazada")
	}
	vb := "N"
	if vistoBueno {
		vb = "S"
	}
	if _, err := tx.Exec(`
		UPDATE adu_solicitud_materiales
		SET vbjefe = $2, fecha_aprueba_rechaza = CURRENT_DATE, estado_sol = $3
		WHERE nro_sol_mat = $1`, id, vb, models.EstadoSolPendienteEntrega); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE adu_detalle_sol_mat SET estado = 'E' WHERE nro_sol_mat = $1 AND estado = 'V'`, id); err != nil {
		return err
	}
	if err := insertarDetalleSolicitud(tx, id, lineas); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AdquisicionesRepository) EntregarSolicitud(id int, cantidades map[int]int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var estado int
	var vb string
	err = tx.QueryRow(`
		SELECT estado_sol, COALESCE(btrim(vbjefe), '')
		FROM adu_solicitud_materiales WHERE nro_sol_mat = $1 FOR UPDATE`, id).Scan(&estado, &vb)
	if err != nil {
		return err
	}
	if estado == models.EstadoSolResuelta {
		return datosf("La entrega ya fue registrada")
	}
	if estado != models.EstadoSolPendienteEntrega || vb != "S" {
		return datosf("La solicitud todavía no tiene visto bueno")
	}
	rows, err := tx.Query(`
		SELECT nro_corr_solmat, cod_material FROM adu_detalle_sol_mat
		WHERE nro_sol_mat = $1 AND estado = 'V' ORDER BY nro_corr_solmat`, id)
	if err != nil {
		return err
	}
	type par struct{ corr, cod int }
	var lineas []par
	for rows.Next() {
		var p par
		if err := rows.Scan(&p.corr, &p.cod); err != nil {
			rows.Close()
			return err
		}
		lineas = append(lineas, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(lineas) == 0 {
		return datosf("La solicitud no tiene materiales")
	}
	ahora := time.Now()
	fecha := ahora.Format("2006-01-02")
	hora := ahora.Format("15:04:05")
	for _, l := range lineas {
		cant := cantidades[l.corr]
		if cant < 0 {
			return datosf("La cantidad entregada no puede ser negativa")
		}
		if _, err := tx.Exec(`
			UPDATE adu_detalle_sol_mat
			SET cantidad_autorizada = $2, cantidad_despachada = $2, fecha_despacho = $3::date, hora_despacho = $4::time
			WHERE nro_corr_solmat = $1 AND estado = 'V'`, l.corr, cant, fecha, hora); err != nil {
			return err
		}
		if cant > 0 {
			if err := rebajarStock(tx, l.cod, cant, ahora); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`UPDATE adu_solicitud_materiales SET estado_sol = $2 WHERE nro_sol_mat = $1`, id, models.EstadoSolResuelta); err != nil {
		return err
	}
	return tx.Commit()
}

func rebajarStock(tx *sql.Tx, cod, cantidad int, cuando time.Time) error {
	rows, err := tx.Query(`
		SELECT s.cantidad, s.valor_unidad
		FROM adu_stock s
		WHERE s.cod_material = $1
		  AND s.correlativo = (
		      SELECT max(s2.correlativo) FROM adu_stock s2
		      WHERE s2.cod_material = s.cod_material AND s2.valor_unidad = s.valor_unidad)
		ORDER BY s.correlativo`, cod)
	if err != nil {
		return err
	}
	type lote struct{ cant, valor int }
	var lotes []lote
	for rows.Next() {
		var l lote
		if err := rows.Scan(&l.cant, &l.valor); err != nil {
			rows.Close()
			return err
		}
		lotes = append(lotes, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	resta := cantidad
	for _, l := range lotes {
		if resta <= 0 {
			break
		}
		nuevo := 0
		if l.cant >= resta {
			nuevo = l.cant - resta
			resta = 0
		} else {
			resta -= l.cant
		}
		if _, err := tx.Exec(`
			INSERT INTO adu_stock (cod_material, fecha, cantidad, valor_unidad, hora)
			VALUES ($1, $2::date, $3, $4, $5::time)`,
			cod, cuando.Format("2006-01-02"), nuevo, l.valor, cuando.Format("15:04:05")); err != nil {
			return err
		}
	}
	return nil
}

func (r *AdquisicionesRepository) Solicitudes(estado int, vistoBueno string, f models.FiltroReporte) ([]models.SolicitudMaterial, error) {
	f1, err := fechaONil(f.Fecha1)
	if err != nil {
		return nil, err
	}
	f2, err := fechaONil(f.Fecha2)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT s.nro_sol_mat, s.fecha_sol_mat, to_char(s.hora_sol_mat, 'HH24:MI:SS'),
		       COALESCE(s.estado_sol, 0), COALESCE(btrim(s.vbjefe), ''),
		       btrim(s.id_usuario), COALESCE(u.nombre, btrim(s.id_usuario)),
		       COALESCE(s.id_depto, 0), COALESCE(d.descripcion, '')
		FROM adu_solicitud_materiales s
		LEFT JOIN gen_usuario u ON u.id_usuario = s.id_usuario
		LEFT JOIN gen_departamento d ON d.id_depto = s.id_depto
		WHERE ($1 = 0 OR s.estado_sol = $1)
		  AND ($2 = '' OR btrim(s.vbjefe) = $2)
		  AND ($3 = 0 OR s.id_depto = $3)
		  AND ($4::date IS NULL OR s.fecha_sol_mat >= $4::date)
		  AND ($5::date IS NULL OR s.fecha_sol_mat <= $5::date)
		ORDER BY s.nro_sol_mat DESC`, estado, vistoBueno, f.Depto, f1, f2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SolicitudMaterial
	for rows.Next() {
		var s models.SolicitudMaterial
		var fecha sql.NullTime
		var hora sql.NullString
		if err := rows.Scan(&s.Numero, &fecha, &hora, &s.Estado, &s.VistoBueno, &s.IDUsuario, &s.Usuario, &s.IDDepto, &s.Departamento); err != nil {
			return nil, err
		}
		s.Fecha = fmtFecha(fecha)
		s.Hora = fmtHora(hora)
		s.EstadoNombre = nombreEstadoSolicitud(s.Estado, s.VistoBueno)
		out = append(out, s)
	}
	return out, rows.Err()
}

func nombreEstadoSolicitud(estado int, vb string) string {
	switch estado {
	case models.EstadoSolAprobacion:
		return "Por aprobar"
	case models.EstadoSolPendienteEntrega:
		if vb == "S" {
			return "Pendiente de entrega"
		}
		return "Rechazada"
	case models.EstadoSolResuelta:
		return "Cursada"
	default:
		return strconv.Itoa(estado)
	}
}

func (r *AdquisicionesRepository) Solicitud(id int) (models.SolicitudMaterial, error) {
	var s models.SolicitudMaterial
	var fecha, aprueba sql.NullTime
	var hora sql.NullString
	err := r.db.QueryRow(`
		SELECT s.nro_sol_mat, s.fecha_sol_mat, to_char(s.hora_sol_mat, 'HH24:MI:SS'),
		       COALESCE(s.estado_sol, 0), COALESCE(btrim(s.vbjefe), ''),
		       btrim(s.id_usuario), COALESCE(u.nombre, btrim(s.id_usuario)),
		       COALESCE(s.id_depto, 0), COALESCE(d.descripcion, ''), s.fecha_aprueba_rechaza
		FROM adu_solicitud_materiales s
		LEFT JOIN gen_usuario u ON u.id_usuario = s.id_usuario
		LEFT JOIN gen_departamento d ON d.id_depto = s.id_depto
		WHERE s.nro_sol_mat = $1`, id).Scan(
		&s.Numero, &fecha, &hora, &s.Estado, &s.VistoBueno, &s.IDUsuario, &s.Usuario,
		&s.IDDepto, &s.Departamento, &aprueba)
	if err != nil {
		return s, err
	}
	s.Fecha = fmtFecha(fecha)
	s.Hora = fmtHora(hora)
	s.FechaAprueba = fmtFecha(aprueba)
	s.EstadoNombre = nombreEstadoSolicitud(s.Estado, s.VistoBueno)
	rows, err := r.db.Query(`
		SELECT d.nro_corr_solmat, d.cod_material, m.nombre_material,
		       d.cantidad_solicitud, COALESCE(d.cantidad_autorizada, 0), COALESCE(d.cantidad_despachada, 0)
		FROM adu_detalle_sol_mat d
		JOIN adu_materialmm m ON m.cod_material = d.cod_material
		WHERE d.nro_sol_mat = $1 AND d.estado = 'V'
		ORDER BY d.nro_corr_solmat`, id)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var l models.LineaCantidad
		if err := rows.Scan(&l.Correlativo, &l.CodMaterial, &l.Nombre, &l.Cantidad, &l.CantidadAutorizada, &l.CantidadDespachada); err != nil {
			return s, err
		}
		s.Lineas = append(s.Lineas, l)
	}
	return s, rows.Err()
}

func (r *AdquisicionesRepository) ContextoEntrega(s models.SolicitudMaterial) ([]models.EntregaLinea, error) {
	ahora := time.Now()
	mes := int(ahora.Month())
	anio := ahora.Year()
	desde := time.Date(anio, 1, 1, 0, 0, 0, 0, time.Local)
	hasta := time.Date(anio, time.Month(mes), 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	meses := mes - 1
	if mes == 1 {
		anio--
		desde = time.Date(anio, 1, 1, 0, 0, 0, 0, time.Local)
		hasta = time.Date(anio, 12, 31, 0, 0, 0, 0, time.Local)
		meses = 11
	}
	prev := ahora.AddDate(0, -1, 0)
	var out []models.EntregaLinea
	for _, l := range s.Lineas {
		e := models.EntregaLinea{
			Correlativo: l.Correlativo,
			CodMaterial: l.CodMaterial,
			Nombre:      l.Nombre,
			Solicitada:  l.Cantidad,
		}
		if err := r.db.QueryRow(`
			SELECT COALESCE(SUM(cantidad), 0)
			FROM adu_stock
			WHERE cod_material = $1
			  AND correlativo IN (
			      SELECT max(correlativo) FROM adu_stock WHERE cod_material = $1 GROUP BY valor_unidad)`,
			l.CodMaterial).Scan(&e.Stock); err != nil {
			return nil, err
		}
		var total sql.NullInt64
		if err := r.db.QueryRow(`
			SELECT SUM(cantidad_autorizada) FROM adu_detalle_sol_mat
			WHERE cod_material = $1 AND estado = 'V' AND fecha_despacho BETWEEN $2::date AND $3::date`,
			l.CodMaterial, desde.Format("2006-01-02"), hasta.Format("2006-01-02")).Scan(&total); err != nil {
			return nil, err
		}
		if total.Valid && meses > 0 {
			e.Promedio = int(math.Round(float64(total.Int64) / float64(meses)))
			if e.Promedio == 0 {
				e.Promedio = 1
			}
		}
		if err := r.db.QueryRow(`
			SELECT COALESCE(SUM(cantidad_autorizada), 0) FROM adu_consumo_valorizado
			WHERE cod_material = $1 AND id_depto = $2 AND to_char(fecha_despacho, 'YYYY') = $3`,
			l.CodMaterial, s.IDDepto, strconv.Itoa(ahora.Year())).Scan(&e.EntregaAnio); err != nil {
			return nil, err
		}
		if err := r.db.QueryRow(`
			SELECT COALESCE(SUM(cantidad_autorizada), 0) FROM adu_consumo_valorizado
			WHERE cod_material = $1 AND id_depto = $2
			  AND to_char(fecha_despacho, 'MM') = $3 AND to_char(fecha_despacho, 'YYYY') = $4`,
			l.CodMaterial, s.IDDepto, fmt.Sprintf("%02d", int(prev.Month())), strconv.Itoa(prev.Year())).Scan(&e.EntregaMesAnt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// ---------- Reportes ----------

func (r *AdquisicionesRepository) ReporteConsumo(f models.FiltroReporte) ([][]string, error) {
	f1, err := fechaONil(f.Fecha1)
	if err != nil {
		return nil, err
	}
	f2, err := fechaONil(f.Fecha2)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT COALESCE(dep.descripcion, ''), cv.tipo_material, cv.nombre_material,
		       SUM(cv.cantidad_autorizada), SUM(cv.valor_total)
		FROM adu_consumo_valorizado cv
		LEFT JOIN gen_departamento dep ON dep.id_depto = cv.id_depto
		WHERE cv.cantidad_autorizada > 0
		  AND ($1 = 0 OR cv.id_depto = $1)
		  AND ($2 = 0 OR cv.cod_tipo_mat = $2)
		  AND ($3 = 0 OR cv.cod_material = $3)
		  AND ($4::date IS NULL OR cv.fecha_despacho >= $4::date)
		  AND ($5::date IS NULL OR cv.fecha_despacho <= $5::date)
		GROUP BY dep.descripcion, cv.id_depto, cv.cod_material, cv.nombre_material, cv.tipo_material
		ORDER BY cv.nombre_material`, f.Depto, f.Tipo, f.Material, f1, f2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var depto, tipo, nombre string
		var cant int
		var total float64
		if err := rows.Scan(&depto, &tipo, &nombre, &cant, &total); err != nil {
			return nil, err
		}
		out = append(out, []string{depto, tipo, nombre, strconv.Itoa(cant), strconv.FormatFloat(total, 'f', 0, 64)})
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) ReporteStock(f models.FiltroReporte) ([][]string, error) {
	rows, err := r.db.Query(`
		SELECT desc_dominio, nombre_material, stock_actual, stock_critico
		FROM adu_stock_actual
		WHERE ($1 = 0 OR tipo_material = $1)
		  AND ($2 = 0 OR cod_material = $2)
		ORDER BY nombre_material`, f.Tipo, f.Material)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCuatro(rows)
}

func (r *AdquisicionesRepository) ReporteStockMinimo(f models.FiltroReporte) ([][]string, error) {
	umbral := f.Umbral
	if f.UsaCritico {
		umbral = -1
	}
	rows, err := r.db.Query(`
		SELECT desc_dominio, nombre_material, stock_actual, stock_critico
		FROM adu_stock_actual
		WHERE ($1 < 0 AND stock_actual <= stock_critico) OR ($1 >= 0 AND stock_actual <= $1)
		ORDER BY nombre_material`, umbral)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCuatro(rows)
}

func scanCuatro(rows *sql.Rows) ([][]string, error) {
	var out [][]string
	for rows.Next() {
		var a, b string
		var c, d int
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			return nil, err
		}
		out = append(out, []string{a, b, strconv.Itoa(c), strconv.Itoa(d)})
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) ReporteStockHistorico(f models.FiltroReporte) ([][]string, error) {
	corte, err := fechaONil(f.FechaCorte)
	if err != nil {
		return nil, err
	}
	if corte == nil {
		return nil, datosf("Indique la fecha de corte")
	}
	rows, err := r.db.Query(`
		SELECT dom.desc_dominio, mat.nombre_material, SUM(sto.cantidad)
		FROM adu_stock sto
		JOIN adu_materialmm mat ON mat.cod_material = sto.cod_material
		JOIN adu_dominio dom ON dom.cod_dominio = mat.tipo_material
		WHERE sto.correlativo IN (
		    SELECT max(s.correlativo) FROM adu_stock s
		    WHERE s.fecha <= $1::date
		    GROUP BY s.valor_unidad, s.cod_material)
		  AND ($2 = 0 OR mat.tipo_material = $2)
		  AND ($3 = 0 OR mat.cod_material = $3)
		GROUP BY sto.cod_material, mat.nombre_material, dom.desc_dominio
		ORDER BY dom.desc_dominio, mat.nombre_material`, corte, f.Tipo, f.Material)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var tipo, nombre string
		var cant int
		if err := rows.Scan(&tipo, &nombre, &cant); err != nil {
			return nil, err
		}
		out = append(out, []string{tipo, nombre, strconv.Itoa(cant)})
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) ReporteEntregaPromedio(f models.FiltroReporte) ([][]string, error) {
	if f.Anio < 1990 || f.Mes < 2 || f.Mes > 12 {
		return nil, datosf("Seleccione un año y un mes desde febrero")
	}
	inicio := fmt.Sprintf("%04d-01-01", f.Anio)
	fin := time.Date(f.Anio, time.Month(f.Mes), 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1).Format("2006-01-02")
	meses := f.Mes - 1
	rows, err := r.db.Query(`
		SELECT desc_dominio, nombre_material, SUM(cantidad)
		FROM (
			SELECT det.cod_material, mat.nombre_material, mat.tipo_material, dom.desc_dominio,
			       SUM(det.cantidad_autorizada) AS cantidad
			FROM adu_detalle_sol_mat det
			JOIN adu_materialmm mat ON mat.cod_material = det.cod_material
			JOIN adu_dominio dom ON dom.cod_dominio = mat.tipo_material
			WHERE det.estado = 'V' AND det.fecha_despacho BETWEEN $1::date AND $2::date
			GROUP BY to_char(det.fecha_despacho, 'MM'), det.cod_material, mat.nombre_material, mat.tipo_material, dom.desc_dominio
		) entrega
		WHERE ($3 = 0 OR tipo_material = $3)
		  AND ($4 = 0 OR cod_material = $4)
		GROUP BY cod_material, nombre_material, desc_dominio
		ORDER BY nombre_material`, inicio, fin, f.Tipo, f.Material)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var tipo, nombre string
		var total int
		if err := rows.Scan(&tipo, &nombre, &total); err != nil {
			return nil, err
		}
		prom := int(math.Round(float64(total) / float64(meses)))
		if prom == 0 && total > 0 {
			prom = 1
		}
		out = append(out, []string{tipo, nombre, strconv.Itoa(total), strconv.Itoa(prom)})
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) ReporteIngresoMateriales(f models.FiltroReporte) ([][]string, error) {
	f1, err := fechaONil(f.Fecha1)
	if err != nil {
		return nil, err
	}
	f2, err := fechaONil(f.Fecha2)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT COALESCE(p.nombre_proveedor, ''), fac.nro_factura, fac.nro_orden_compra,
		       to_char(fac.fecha_ingreso, 'DD-MM-YYYY'), dom.desc_dominio, mat.nombre_material,
		       det.cantidad_detallefact, det.valor_unitariofactura
		FROM adu_facturapr fac
		JOIN adu_detalle_facturapr det ON det.nro_corr_fact = fac.nro_corr_fact
		JOIN adu_materialmm mat ON mat.cod_material = det.cod_material
		JOIN adu_dominio dom ON dom.cod_dominio = mat.tipo_material
		JOIN adu_proveedormm p ON p.rut_proveedor = fac.rut_proveedor
		WHERE ($1 = 0 OR mat.tipo_material = $1)
		  AND ($2 = 0 OR mat.cod_material = $2)
		  AND ($3::date IS NULL OR fac.fecha_factura >= $3::date)
		  AND ($4::date IS NULL OR fac.fecha_factura <= $4::date)
		ORDER BY fac.nro_corr_fact, det.nro_det_fact`, f.Tipo, f.Material, f1, f2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var prov, tipo, nombre, fecha string
		var factura, orden, cant, valor int
		if err := rows.Scan(&prov, &factura, &orden, &fecha, &tipo, &nombre, &cant, &valor); err != nil {
			return nil, err
		}
		out = append(out, []string{prov, strconv.Itoa(factura), strconv.Itoa(orden), fecha, tipo, nombre, strconv.Itoa(cant), strconv.Itoa(valor)})
	}
	return out, rows.Err()
}
