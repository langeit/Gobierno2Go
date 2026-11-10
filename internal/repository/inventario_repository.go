package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
)

type InventarioRepository struct {
	db *sql.DB
}

func NewInventarioRepository(db *sql.DB) *InventarioRepository {
	return &InventarioRepository{db: db}
}

// ---------- Catálogos ----------

func (r *InventarioRepository) TiposBien() ([]models.TipoBien, error) {
	rows, err := r.db.Query(`SELECT corr_tbien, descripcion_tbien, codigo_tbien
		FROM inv_tipo_bien ORDER BY descripcion_tbien`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tipos []models.TipoBien
	for rows.Next() {
		var t models.TipoBien
		if err := rows.Scan(&t.CorrTbien, &t.Descripcion, &t.Codigo); err != nil {
			return nil, err
		}
		tipos = append(tipos, t)
	}
	return tipos, rows.Err()
}

func (r *InventarioRepository) Zonas() ([]models.Zona, error) {
	rows, err := r.db.Query(`SELECT corr_zona, desc_zona, codigo_zona FROM inv_zona ORDER BY desc_zona`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zonas []models.Zona
	for rows.Next() {
		var z models.Zona
		if err := rows.Scan(&z.CorrZona, &z.Descripcion, &z.Codigo); err != nil {
			return nil, err
		}
		zonas = append(zonas, z)
	}
	return zonas, rows.Err()
}

func (r *InventarioRepository) Asignaciones() ([]models.CatalogItem, error) {
	rows, err := r.db.Query(`SELECT corr_asig::text, cod_asig || ' - ' || descripcion FROM adu_asignacion WHERE estado = 'V' ORDER BY cod_asig`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CatalogItem
	for rows.Next() {
		var it models.CatalogItem
		if err := rows.Scan(&it.Codigo, &it.Descripcion); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *InventarioRepository) Empleados() ([]models.Empleado, error) {
	rows, err := r.db.Query(`
		SELECT d.id_empleado, d.nombre || ' ' || d.apellido,
		       COALESCE(dep.descripcion, '')
		FROM gen_directorio d
		LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto
		WHERE COALESCE(nullif(btrim(d.estado), ''), 'V') = 'V'
		ORDER BY d.nombre, d.apellido`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var empleados []models.Empleado
	for rows.Next() {
		var e models.Empleado
		if err := rows.Scan(&e.IDEmpleado, &e.Nombre, &e.Depto); err != nil {
			return nil, err
		}
		empleados = append(empleados, e)
	}
	return empleados, rows.Err()
}

func (r *InventarioRepository) UsuarioInfo(idUsuario string) (idEmpleado, idDepto int, tipo string, err error) {
	err = r.db.QueryRow(`
		SELECT COALESCE(u.id_empleado, 0), COALESCE(d.depto, 0), COALESCE(u.tipo, '3')
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		WHERE u.id_usuario = $1`, idUsuario).Scan(&idEmpleado, &idDepto, &tipo)
	return
}

// EsAdminInventario indica si el usuario pertenece a Adquisiciones, es administrador
// legado o tiene el módulo de administración por sus perfiles.
func (r *InventarioRepository) EsAdminInventario(idUsuario string) (bool, error) {
	_, idDepto, tipo, err := r.UsuarioInfo(idUsuario)
	if err != nil {
		return false, err
	}
	if idDepto == 18 || tipo == "0" {
		return true, nil
	}
	return esAdministradorDeSistema(r.db, idUsuario)
}

// ---------- Bienes ----------

func (r *InventarioRepository) bienBaseQuery() string {
	return `
SELECT b.corr_bien,
       b.codigo_bien,
       COALESCE(b.corr_tbien, 0),
       COALESCE(t.descripcion_tbien, ''),
       COALESCE(b.descripcion, ''),
       b.monto_bien,
       COALESCE(b.estado, ''),
       COALESCE(b.cuenta, ''),
       COALESCE(b.nro_docto_compra::text, ''),
       COALESCE(b.tipo_docto_compra, ''),
       COALESCE(b.rut_proveedor, ''),
       COALESCE(u.piso, ''),
       COALESCE(u.oficina, ''),
       COALESCE(z.desc_zona, ''),
       u.fecha_ubica,
       COALESCE((
           SELECT string_agg(dir.nombre || ' ' || dir.apellido, ', ' ORDER BY dir.nombre)
           FROM inv_responsable ir
           JOIN gen_directorio dir ON dir.id_empleado = ir.id_empleado
           WHERE ir.corr_bien = b.corr_bien AND ir.estado = 'V'
       ), '')
FROM inv_bien b
LEFT JOIN inv_tipo_bien t ON t.corr_tbien = b.corr_tbien
LEFT JOIN inv_ubicacion u ON u.corr_ubica = (
    SELECT MAX(corr_ubica) FROM inv_ubicacion WHERE corr_bien = b.corr_bien
)
LEFT JOIN inv_zona z ON z.corr_zona = u.corr_zona`
}

func scanBien(scan func(...interface{}) error) (models.Bien, error) {
	var b models.Bien
	var fecha sql.NullTime
	err := scan(&b.CorrBien, &b.CodigoBien, &b.CorrTbien, &b.TipoBien, &b.Descripcion,
		&b.Monto, &b.Estado, &b.Cuenta, &b.NroDoctoCompra, &b.TipoDoctoCompra,
		&b.RutProveedor, &b.Piso, &b.Oficina, &b.Zona, &fecha, &b.Responsables)
	if fecha.Valid {
		b.FechaUbicacion = &fecha.Time
	}
	return b, err
}

func (r *InventarioRepository) Bienes(corrTbien int) ([]models.Bien, error) {
	query := r.bienBaseQuery()
	args := []interface{}{}
	if corrTbien > 0 {
		query += ` WHERE b.corr_tbien = $1`
		args = append(args, corrTbien)
	}
	query += ` ORDER BY b.codigo_bien`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		b, err := scanBien(rows.Scan)
		if err != nil {
			return nil, err
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

func (r *InventarioRepository) BienPorID(corrBien int) (models.Bien, error) {
	row := r.db.QueryRow(r.bienBaseQuery()+` WHERE b.corr_bien = $1`, corrBien)
	return scanBien(row.Scan)
}

func (r *InventarioRepository) BienesDeEmpleado(idEmpleado int) ([]models.Bien, error) {
	rows, err := r.db.Query(r.bienBaseQuery()+`
		WHERE EXISTS (SELECT 1 FROM inv_responsable ir
		              WHERE ir.corr_bien = b.corr_bien AND ir.id_empleado = $1 AND ir.estado = 'V')
		ORDER BY b.codigo_bien`, idEmpleado)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		b, err := scanBien(rows.Scan)
		if err != nil {
			return nil, err
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

func (r *InventarioRepository) ResponsablesDe(corrBien int) (map[int]bool, error) {
	rows, err := r.db.Query(`SELECT id_empleado FROM inv_responsable WHERE corr_bien = $1 AND estado = 'V'`, corrBien)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	asignados := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		asignados[id] = true
	}
	return asignados, rows.Err()
}

func (r *InventarioRepository) AsignarResponsables(corrBien int, ids []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE inv_responsable SET estado = 'E' WHERE corr_bien = $1 AND estado = 'V'`, corrBien); err != nil {
		return err
	}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO inv_responsable (corr_bien, id_empleado, fecha_resp, estado)
			VALUES ($1, $2, CURRENT_DATE, 'V')
			ON CONFLICT (corr_bien, id_empleado) DO UPDATE SET estado = 'V', fecha_resp = CURRENT_DATE`, corrBien, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *InventarioRepository) CambiarEstado(corrBien int, estado string) error {
	res, err := r.db.Exec(`UPDATE inv_bien SET estado = $2 WHERE corr_bien = $1`, corrBien, estado)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("el bien no existe")
	}
	return nil
}

func (r *InventarioRepository) AsignarCuenta(corrBien int, cuenta string) error {
	res, err := r.db.Exec(`UPDATE inv_bien SET cuenta = $2 WHERE corr_bien = $1`, corrBien, cuenta)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("el bien no existe")
	}
	return nil
}

// ---------- Ingreso de bienes ----------

func (r *InventarioRepository) CrearIngreso(in models.IngresoBien, idUsuario string) ([]string, error) {
	in, err := prepararIngreso(in)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Evita dos altas simultáneas del mismo origen o de la misma factura.
	clave := fmt.Sprintf("inv-ingreso:%s:%d:%s:%d:%s",
		in.TipoDocOrigen, in.NumDocOrigen, in.TipoDocCompra, in.NumDocCompra, in.RutProveedor)
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, clave); err != nil {
		return nil, err
	}

	var origenRepetido bool
	if err := tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM inv_movimiento
			WHERE num_doc = $1 AND btrim(tipo_doc) = $2
		)`, in.NumDocOrigen, in.TipoDocOrigen).Scan(&origenRepetido); err != nil {
		return nil, err
	}
	if origenRepetido {
		return nil, nuevoErrorUsuario("ya existe un movimiento con el mismo tipo y número de documento de origen")
	}

	var facturaRepetida bool
	if err := tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM inv_bien
			WHERE nro_docto_compra = $1 AND btrim(tipo_docto_compra) = $2 AND btrim(rut_proveedor) = $3
		)`, in.NumDocCompra, in.TipoDocCompra, in.RutProveedor).Scan(&facturaRepetida); err != nil {
		return nil, err
	}
	if facturaRepetida {
		return nil, nuevoErrorUsuario("ya existe un bien con el mismo documento de compra y RUT de proveedor")
	}

	var proveedorOK bool
	if err := tx.QueryRow(`
		SELECT EXISTS (SELECT 1 FROM adu_proveedormm WHERE btrim(rut_proveedor) = $1 AND estado = 'V')`,
		in.RutProveedor).Scan(&proveedorOK); err != nil {
		return nil, err
	}
	if !proveedorOK {
		return nil, nuevoErrorUsuario("el proveedor no está registrado")
	}

	var numMovi int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(num_movi), 0) + 1 FROM inv_movimiento`).Scan(&numMovi); err != nil {
		return nil, err
	}
	var corrMovi int
	// El movimiento de codificación queda asociado al documento de origen.
	if err := tx.QueryRow(`
		INSERT INTO inv_movimiento (num_movi, id_usuario, concepto, num_doc, tipo_doc, fecha_movi, hora_movi)
		VALUES ($1, $2, 'I', $3, $4, CURRENT_DATE, CURRENT_TIME)
		RETURNING corr_movi`,
		numMovi, idUsuario, in.NumDocOrigen, in.TipoDocOrigen).Scan(&corrMovi); err != nil {
		return nil, err
	}

	var codigos []string
	for _, item := range in.Items {
		if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1::bigint)`, int64(900000+item.CorrTbien)); err != nil {
			return nil, err
		}
		var codigoTipo, codigoZona string
		if err := tx.QueryRow(`SELECT codigo_tbien FROM inv_tipo_bien WHERE corr_tbien = $1`,
			item.CorrTbien).Scan(&codigoTipo); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nuevoErrorUsuario("el tipo de bien no existe")
			}
			return nil, err
		}
		if err := tx.QueryRow(`SELECT codigo_zona FROM inv_zona WHERE corr_zona = $1`,
			item.CorrZona).Scan(&codigoZona); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nuevoErrorUsuario("la zona no existe")
			}
			return nil, err
		}
		var asigOK bool
		if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM adu_asignacion WHERE corr_asig = $1 AND estado = 'V')`,
			item.CorrAsig).Scan(&asigOK); err != nil {
			return nil, err
		}
		if !asigOK {
			return nil, nuevoErrorUsuario("la imputación presupuestaria no existe")
		}
		codigoTipo = strings.TrimSpace(codigoTipo)
		codigoZona = strings.TrimSpace(codigoZona)

		var ultimo string
		if err := tx.QueryRow(`
			SELECT COALESCE(MAX(codigo_bien), '')
			FROM inv_bien
			WHERE substring(codigo_bien FROM 1 FOR 2) = $1`, codigoTipo).Scan(&ultimo); err != nil {
			return nil, err
		}
		correlativo := 1
		if len(ultimo) >= 7 {
			fmt.Sscanf(strings.TrimSpace(ultimo[3:7]), "%d", &correlativo)
			correlativo++
		}
		if correlativo > 9999 {
			return nil, nuevoErrorUsuario("se agotó el correlativo para el tipo de bien")
		}
		codigo := fmt.Sprintf("%s-%04d-%s-%s-%02d", codigoTipo, correlativo, codigoZona,
			fechaCorta(), item.Fuente)
		codigos = append(codigos, codigo)

		var corrBien int
		if err := tx.QueryRow(`
			INSERT INTO inv_bien
				(corr_tbien, codigo_bien, monto_bien, nro_docto_compra, rut_proveedor,
				 descripcion, estado, cuenta, tipo_docto_compra)
			VALUES ($1, $2, $3, $4, $5, $6, 'BU', NULL, $7)
			RETURNING corr_bien`,
			item.CorrTbien, codigo, item.Monto, in.NumDocCompra, in.RutProveedor,
			item.Descripcion, in.TipoDocCompra).Scan(&corrBien); err != nil {
			return nil, err
		}

		if _, err := tx.Exec(`
			INSERT INTO inv_ubicacion (fecha_ubica, piso, oficina, corr_zona, corr_bien)
			VALUES (CURRENT_DATE, $1, $2, $3, $4)`,
			item.Piso, item.Oficina, item.CorrZona, corrBien); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO inv_det_movimiento (corr_movi, corr_bien) VALUES ($1, $2)`,
			corrMovi, corrBien); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`
			INSERT INTO inv_imputacion_bien (corr_asig, corr_bien, monto)
			VALUES ($1, $2, $3)`, item.CorrAsig, corrBien, item.Monto); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return codigos, nil
}

// prepararIngreso normaliza el alta y rechaza filas incompletas antes de tocar la base.
func prepararIngreso(in models.IngresoBien) (models.IngresoBien, error) {
	in.TipoDocOrigen = strings.ToUpper(strings.TrimSpace(in.TipoDocOrigen))
	in.TipoDocCompra = strings.ToUpper(strings.TrimSpace(in.TipoDocCompra))
	in.RutProveedor = NormalizarRut(in.RutProveedor)

	if !tiposDocOrigen[in.TipoDocOrigen] {
		return in, nuevoErrorUsuario("el documento de origen debe ser resolución exenta, orden de pedido o memo")
	}
	if in.NumDocOrigen <= 0 {
		return in, nuevoErrorUsuario("debe indicar el número del documento de origen")
	}
	if !tiposDocCompra[in.TipoDocCompra] {
		return in, nuevoErrorUsuario("el documento de compra debe ser factura u orden de pedido")
	}
	if in.NumDocCompra <= 0 {
		return in, nuevoErrorUsuario("debe indicar el número del documento de compra")
	}
	if len(in.RutProveedor) < 2 || len(in.RutProveedor) > 9 {
		return in, nuevoErrorUsuario("el RUT del proveedor no es válido")
	}

	items := make([]models.IngresoItem, 0, len(in.Items))
	for _, item := range in.Items {
		if itemVacio(item) {
			continue
		}
		item.Descripcion = strings.TrimSpace(item.Descripcion)
		item.Piso = strings.TrimSpace(item.Piso)
		item.Oficina = strings.TrimSpace(item.Oficina)
		switch {
		case item.CorrTbien <= 0:
			return in, nuevoErrorUsuario("cada bien debe tener un tipo")
		case item.Descripcion == "":
			return in, nuevoErrorUsuario("cada bien debe tener una descripción")
		case len(item.Descripcion) > 100:
			return in, nuevoErrorUsuario("la descripción del bien no puede superar 100 caracteres")
		case item.Monto <= 0:
			return in, nuevoErrorUsuario("el monto de cada bien debe ser mayor que cero")
		case item.CorrZona <= 0:
			return in, nuevoErrorUsuario("cada bien debe tener una zona")
		case item.Piso == "" || len(item.Piso) > 10:
			return in, nuevoErrorUsuario("el piso es obligatorio y admite hasta 10 caracteres")
		case item.Oficina == "" || len(item.Oficina) > 10:
			return in, nuevoErrorUsuario("la oficina es obligatoria y admite hasta 10 caracteres")
		case item.CorrAsig <= 0:
			return in, nuevoErrorUsuario("cada bien debe tener una imputación presupuestaria")
		case item.Fuente < 1 || item.Fuente > 99:
			return in, nuevoErrorUsuario("la fuente de recurso debe estar entre 1 y 99")
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return in, nuevoErrorUsuario("debe registrar al menos un bien")
	}
	in.Items = items
	return in, nil
}

func itemVacio(it models.IngresoItem) bool {
	return it.CorrTbien == 0 && strings.TrimSpace(it.Descripcion) == "" && it.Monto == 0 &&
		it.CorrZona == 0 && strings.TrimSpace(it.Piso) == "" && strings.TrimSpace(it.Oficina) == "" &&
		it.CorrAsig == 0
}

func (r *InventarioRepository) CambiarUbicacion(corrBien, corrZona int, piso, oficina, tipoDoc string, numDoc int64, idUsuario string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO inv_ubicacion (fecha_ubica, piso, oficina, corr_zona, corr_bien)
		VALUES (CURRENT_DATE, $1, $2, $3, $4)`, piso, oficina, corrZona, corrBien)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("no se pudo registrar la ubicación")
	}

	var numMovi int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(num_movi), 0) + 1 FROM inv_movimiento`).Scan(&numMovi); err != nil {
		return err
	}
	if tipoDoc == "" {
		tipoDoc = "ME"
	}
	var corrMovi int
	if err := tx.QueryRow(`
		INSERT INTO inv_movimiento (num_movi, id_usuario, concepto, num_doc, tipo_doc, fecha_movi, hora_movi)
		VALUES ($1, $2, 'T', $3, $4, CURRENT_DATE, CURRENT_TIME)
		RETURNING corr_movi`, numMovi, idUsuario, numDoc, tipoDoc).Scan(&corrMovi); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO inv_det_movimiento (corr_movi, corr_bien) VALUES ($1, $2)`,
		corrMovi, corrBien); err != nil {
		return err
	}
	return tx.Commit()
}

// ---------- Movimientos ----------

func (r *InventarioRepository) Movimientos(concepto, desde, hasta string) ([]models.Movimiento, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if concepto != "" {
		args = append(args, concepto)
		where = append(where, fmt.Sprintf("m.concepto = $%d", len(args)))
	}
	if desde != "" {
		args = append(args, desde)
		where = append(where, fmt.Sprintf("m.fecha_movi >= $%d::date", len(args)))
	}
	if hasta != "" {
		args = append(args, hasta)
		where = append(where, fmt.Sprintf("m.fecha_movi <= $%d::date", len(args)))
	}
	query := `
SELECT m.corr_movi, m.num_movi::text, m.concepto, m.num_doc::text, m.tipo_doc,
       m.fecha_movi, m.hora_movi::text, COALESCE(m.id_usuario, ''),
       COALESCE(string_agg(b.codigo_bien || ' · ' || COALESCE(b.descripcion, ''), '; '), '')
FROM inv_movimiento m
LEFT JOIN inv_det_movimiento d ON d.corr_movi = m.corr_movi
LEFT JOIN inv_bien b ON b.corr_bien = d.corr_bien
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY m.corr_movi, m.num_movi, m.concepto, m.num_doc, m.tipo_doc,
         m.fecha_movi, m.hora_movi, m.id_usuario
ORDER BY m.fecha_movi DESC, m.corr_movi DESC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movimientos []models.Movimiento
	for rows.Next() {
		var m models.Movimiento
		if err := rows.Scan(&m.CorrMovi, &m.NumMovi, &m.Concepto, &m.NumDoc, &m.TipoDoc,
			&m.Fecha, &m.Hora, &m.Usuario, &m.Bienes); err != nil {
			return nil, err
		}
		movimientos = append(movimientos, m)
	}
	return movimientos, rows.Err()
}

// ---------- Solicitudes ----------

func (r *InventarioRepository) solicitudBaseQuery() string {
	return `
SELECT s.corr_solic, s.fecha_solic, s.hora_sol::text, s.tipo_sol, s.descripcion,
       s.estado_sol, s.usuario_sol,
       COALESCE(d.nombre || ' ' || d.apellido, ''), COALESCE(dep.descripcion, ''),
       COALESCE(s.vb_jefe, ''), s.fecha_aprb_rchza,
       COALESCE(s.vb_admin, ''), s.fecha_aprch_admin
FROM inv_solic_inventario s
JOIN gen_usuario u ON u.id_usuario = s.usuario_sol
LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto`
}

func scanSolicitudInv(scan func(...interface{}) error) (models.SolicitudInventario, error) {
	var s models.SolicitudInventario
	var fj, fa sql.NullTime
	err := scan(&s.CorrSolic, &s.FechaSolic, &s.HoraSol, &s.TipoSol, &s.Descripcion,
		&s.EstadoSol, &s.UsuarioSol, &s.NombreUsuario, &s.Depto,
		&s.VBJefe, &fj, &s.VBAdmin, &fa)
	if fj.Valid {
		s.FechaAprobJefe = &fj.Time
	}
	if fa.Valid {
		s.FechaAprobAdmin = &fa.Time
	}
	return s, err
}

func (r *InventarioRepository) Solicitudes(idUsuario string, todos bool) ([]models.SolicitudInventario, error) {
	query := r.solicitudBaseQuery()
	args := []interface{}{}
	if !todos {
		query += ` WHERE s.usuario_sol = $1`
		args = append(args, idUsuario)
	}
	query += ` ORDER BY s.fecha_solic DESC, s.corr_solic DESC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solicitudes []models.SolicitudInventario
	for rows.Next() {
		s, err := scanSolicitudInv(rows.Scan)
		if err != nil {
			return nil, err
		}
		solicitudes = append(solicitudes, s)
	}
	return solicitudes, rows.Err()
}

func (r *InventarioRepository) SolicitudPorID(id int) (models.SolicitudInventario, error) {
	row := r.db.QueryRow(r.solicitudBaseQuery()+` WHERE s.corr_solic = $1`, id)
	s, err := scanSolicitudInv(row.Scan)
	if err != nil {
		return s, err
	}
	bienes, err := r.bienesDeSolicitud(id)
	if err != nil {
		return s, err
	}
	s.Bienes = bienes
	return s, nil
}

func (r *InventarioRepository) bienesDeSolicitud(id int) ([]models.Bien, error) {
	rows, err := r.db.Query(`
		SELECT b.corr_bien, b.codigo_bien, COALESCE(b.corr_tbien, 0),
		       COALESCE(t.descripcion_tbien, ''), COALESCE(b.descripcion, ''),
		       b.monto_bien, COALESCE(b.estado, ''), COALESCE(b.cuenta, ''),
		       COALESCE(b.nro_docto_compra::text, ''), COALESCE(b.tipo_docto_compra, ''),
		       COALESCE(b.rut_proveedor, ''),
		       COALESCE(u.piso, ''), COALESCE(u.oficina, ''),
		       COALESCE(z.desc_zona, ''), u.fecha_ubica, ''
		FROM inv_det_solic_inv ds
		JOIN inv_bien b ON b.corr_bien = ds.corr_bien
		LEFT JOIN inv_tipo_bien t ON t.corr_tbien = b.corr_tbien
		LEFT JOIN inv_ubicacion u ON u.corr_ubica = (
		    SELECT MAX(corr_ubica) FROM inv_ubicacion WHERE corr_bien = b.corr_bien)
		LEFT JOIN inv_zona z ON z.corr_zona = u.corr_zona
		WHERE ds.corr_solic = $1
		ORDER BY b.codigo_bien`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		b, err := scanBien(rows.Scan)
		if err != nil {
			return nil, err
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

func (r *InventarioRepository) CrearSolicitud(idUsuario, tipo, descripcion string, idEmpleado int, bienes []int) (int, error) {
	if tipo == "" {
		return 0, errors.New("debe indicar el tipo de solicitud")
	}
	if strings.TrimSpace(descripcion) == "" {
		return 0, errors.New("debe indicar una descripción")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if tipo == "R" || tipo == "D" {
		if len(bienes) == 0 {
			return 0, errors.New("debe seleccionar al menos un bien")
		}
		for _, id := range bienes {
			var ok bool
			if err := tx.QueryRow(`
				SELECT EXISTS (SELECT 1 FROM inv_responsable
				               WHERE corr_bien = $1 AND id_empleado = $2 AND estado = 'V')`,
				id, idEmpleado).Scan(&ok); err != nil {
				return 0, err
			}
			if !ok {
				return 0, errors.New("sólo puede solicitar bienes bajo su responsabilidad")
			}
		}
	}

	var id int
	if err := tx.QueryRow(`
		INSERT INTO inv_solic_inventario
			(fecha_solic, hora_sol, tipo_sol, descripcion, estado_sol, usuario_sol, vb_jefe)
		VALUES (CURRENT_DATE, CURRENT_TIME, $1, $2, 'P', $3, 'N')
		RETURNING corr_solic`, tipo, strings.TrimSpace(descripcion), idUsuario).Scan(&id); err != nil {
		return 0, err
	}
	for _, corrBien := range bienes {
		if _, err := tx.Exec(`
			INSERT INTO inv_det_solic_inv (corr_bien, corr_solic)
			VALUES ($1, $2)`, corrBien, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// SolicitudesPendientesJefe devuelve las solicitudes pendientes del jefe directo.
func (r *InventarioRepository) SolicitudesPendientesJefe(idUsuario string) ([]models.SolicitudInventario, error) {
	query := r.solicitudBaseQuery() + `
		WHERE s.estado_sol = 'P' AND EXISTS (
			SELECT 1
			FROM gen_usuario us
			JOIN gen_directorio d ON d.id_empleado = us.id_empleado
			JOIN for_jefe_depto jd ON jd.id_depto = d.depto
			JOIN gen_usuario u ON u.id_usuario = $1
			WHERE us.id_usuario = s.usuario_sol
			  AND (COALESCE(u.tipo, '3') = '0'
			       OR jd.id_empleado = u.id_empleado
			       OR jd.aprueba_jefe = u.id_empleado)
		)
		ORDER BY s.fecha_solic, s.corr_solic`
	rows, err := r.db.Query(query, idUsuario)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solicitudes []models.SolicitudInventario
	for rows.Next() {
		s, err := scanSolicitudInv(rows.Scan)
		if err != nil {
			return nil, err
		}
		solicitudes = append(solicitudes, s)
	}
	return solicitudes, rows.Err()
}

// SolicitudesPendientesAdmin devuelve las solicitudes aprobadas por jefe.
func (r *InventarioRepository) SolicitudesPendientesAdmin() ([]models.SolicitudInventario, error) {
	rows, err := r.db.Query(r.solicitudBaseQuery() + `
		WHERE s.estado_sol = 'C'
		ORDER BY s.fecha_solic, s.corr_solic`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solicitudes []models.SolicitudInventario
	for rows.Next() {
		s, err := scanSolicitudInv(rows.Scan)
		if err != nil {
			return nil, err
		}
		solicitudes = append(solicitudes, s)
	}
	return solicitudes, rows.Err()
}

// Conteos entrega cifras resumidas para el menú del módulo.
func (r *InventarioRepository) Conteos(idUsuario string, idEmpleado int) (totalBienes, misBienes, pendientes int, err error) {
	if err = r.db.QueryRow(`SELECT COUNT(*) FROM inv_bien`).Scan(&totalBienes); err != nil {
		return
	}
	if idEmpleado > 0 {
		if err = r.db.QueryRow(`
			SELECT COUNT(DISTINCT corr_bien) FROM inv_responsable WHERE id_empleado = $1 AND estado = 'V'`,
			idEmpleado).Scan(&misBienes); err != nil {
			return
		}
	}
	if err = r.db.QueryRow(`
		SELECT COUNT(*) FROM inv_solic_inventario
		WHERE usuario_sol = $1 AND estado_sol IN ('P', 'C')`, idUsuario).Scan(&pendientes); err != nil {
		return
	}
	return
}

// PuedeAprobarJefe indica si el usuario puede resolver la solicitud como jefe directo.
func (r *InventarioRepository) PuedeAprobarJefe(idUsuario string, idSolicitud int) (bool, error) {
	var ok bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM inv_solic_inventario s
			JOIN gen_usuario us ON us.id_usuario = s.usuario_sol
			JOIN gen_directorio d ON d.id_empleado = us.id_empleado
			JOIN for_jefe_depto jd ON jd.id_depto = d.depto
			JOIN gen_usuario u ON u.id_usuario = $1
			WHERE s.corr_solic = $2 AND s.estado_sol = 'P'
			  AND (COALESCE(u.tipo, '3') = '0'
			       OR jd.id_empleado = u.id_empleado
			       OR jd.aprueba_jefe = u.id_empleado)
		)`, idUsuario, idSolicitud).Scan(&ok)
	return ok, err
}

func (r *InventarioRepository) ResolverJefe(idSolicitud int, aprobar bool) error {
	estado := "R"
	vb := "N"
	if aprobar {
		estado = "C"
		vb = "S"
	}
	res, err := r.db.Exec(`
		UPDATE inv_solic_inventario
		SET vb_jefe = $2, fecha_aprb_rchza = CURRENT_DATE, estado_sol = $3
		WHERE corr_solic = $1 AND estado_sol = 'P'`, idSolicitud, vb, estado)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("la solicitud ya fue resuelta o no existe")
	}
	return nil
}

// PuedeAprobarAdmin indica si el usuario pertenece a la oficina de Adquisiciones.
func (r *InventarioRepository) PuedeAprobarAdmin(idUsuario string) (bool, error) {
	return r.EsAdminInventario(idUsuario)
}

func (r *InventarioRepository) ResolverAdmin(idSolicitud int, aprobar bool) error {
	estado := "R"
	vb := "N"
	if aprobar {
		estado = "A"
		vb = "S"
	}
	res, err := r.db.Exec(`
		UPDATE inv_solic_inventario
		SET vb_admin = $2, fecha_aprch_admin = CURRENT_DATE, estado_sol = $3
		WHERE corr_solic = $1 AND estado_sol = 'C' AND vb_jefe = 'S'`, idSolicitud, vb, estado)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("la solicitud no está pendiente de aprobación administrativa")
	}
	return nil
}

// ---------- Porcentajes y depreciación ----------

func (r *InventarioRepository) PorcentajeAnio(anho string) (models.Porcentaje, bool, error) {
	var p models.Porcentaje
	err := r.db.QueryRow(`
		SELECT corr_porcent, valor_porcent, anho_porcent, fecha_update
		FROM inv_porcentaje WHERE anho_porcent = $1
		ORDER BY corr_porcent DESC LIMIT 1`, anho).
		Scan(&p.CorrPorcent, &p.Valor, &p.Anho, &p.FechaUpdate)
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	return p, err == nil, err
}

func (r *InventarioRepository) DepreciacionEjecutada(anho string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM inv_anho_deprec WHERE anho = $1)`,
		anho).Scan(&ok)
	return ok, err
}

func (r *InventarioRepository) AnosDepreciados() ([]models.AnhoDepreciacion, error) {
	rows, err := r.db.Query(`
		SELECT corr_andep, anho, fecha_andep, usuario_andep
		FROM inv_anho_deprec ORDER BY anho DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anos []models.AnhoDepreciacion
	for rows.Next() {
		var a models.AnhoDepreciacion
		if err := rows.Scan(&a.CorrAndep, &a.Anho, &a.Fecha, &a.Usuario); err != nil {
			return nil, err
		}
		anos = append(anos, a)
	}
	return anos, rows.Err()
}

func (r *InventarioRepository) GuardarPorcentaje(anho string, valor float64, idUsuario string) error {
	ejecutada, err := r.DepreciacionEjecutada(anho)
	if err != nil {
		return err
	}
	if ejecutada {
		return errors.New("no es posible modificar el porcentaje: la depreciación del año ya fue ejecutada")
	}
	_, err = r.db.Exec(`
		INSERT INTO inv_porcentaje (valor_porcent, anho_porcent, id_usuario, fecha_update)
		VALUES ($1, $2, $3, CURRENT_DATE)`, valor, anho, idUsuario)
	return err
}

// EjecutarDepreciacion calcula y registra la depreciación del año indicado.
func (r *InventarioRepository) EjecutarDepreciacion(anho, idUsuario string) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var ejecutada bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM inv_anho_deprec WHERE anho = $1)`,
		anho).Scan(&ejecutada); err != nil {
		return 0, err
	}
	if ejecutada {
		return 0, errors.New("la depreciación de este año ya fue ejecutada")
	}

	var corrPorcent int
	var valorPorcent float64
	if err := tx.QueryRow(`
		SELECT corr_porcent, valor_porcent FROM inv_porcentaje
		WHERE anho_porcent = $1 ORDER BY corr_porcent DESC LIMIT 1`, anho).
		Scan(&corrPorcent, &valorPorcent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New("debe ingresar el porcentaje de actualización del año")
		}
		return 0, err
	}
	factor := valorPorcent / 100.0

	rows, err := tx.Query(`
		SELECT b.corr_bien, b.monto_bien
		FROM inv_bien b
		WHERE b.corr_bien NOT IN (
			SELECT corr_bien FROM inv_depreciacion WHERE ano_vida_util = 1
		)
		ORDER BY b.corr_bien`)
	if err != nil {
		return 0, err
	}
	type pendiente struct {
		id    int
		monto float64
	}
	var bienes []pendiente
	for rows.Next() {
		var p pendiente
		if err := rows.Scan(&p.id, &p.monto); err != nil {
			rows.Close()
			return 0, err
		}
		bienes = append(bienes, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	procesados := 0
	for _, b := range bienes {
		var bienActua, depreAcum float64
		var anoVida int
		err := tx.QueryRow(`
			SELECT bien_actua, depre_acum, ano_vida_util
			FROM inv_depreciacion
			WHERE corr_bien = $1
			ORDER BY corr_depre DESC LIMIT 1`, b.id).
			Scan(&bienActua, &depreAcum, &anoVida)
		if errors.Is(err, sql.ErrNoRows) {
			cuota := b.monto * factor
			bienActua = b.monto + cuota
			anoVida = 10
			depreAnho := bienActua / float64(anoVida)
			valorLibro := bienActua - depreAnho
			if _, err := tx.Exec(`
				INSERT INTO inv_depreciacion
					(corr_bien, corr_porcent, anho, cuota_actua, bien_actua, ano_vida_util,
					 actua_depre, depre_actua, depre_anho, depre_acum, valor_libro)
				VALUES ($1, $2, $3, $4, $5, $6, 0, 0, $7, $7, $8)`,
				b.id, corrPorcent, anho, cuota, bienActua, anoVida, depreAnho, valorLibro); err != nil {
				return 0, err
			}
			procesados++
			continue
		}
		if err != nil {
			return 0, err
		}

		cuota := bienActua * factor
		nuevoBien := bienActua + cuota
		nuevoAnoVida := anoVida - 1
		if nuevoAnoVida < 1 {
			nuevoAnoVida = 1
		}
		actuaDepre := depreAcum * factor
		depreActua := depreAcum + actuaDepre
		depreAnho := (nuevoBien - depreActua) / float64(nuevoAnoVida)
		depreAcumNuevo := depreActua + depreAnho
		valorLibro := nuevoBien - depreAcumNuevo
		if nuevoAnoVida == 1 {
			valorLibro = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO inv_depreciacion
				(corr_bien, corr_porcent, anho, cuota_actua, bien_actua, ano_vida_util,
				 actua_depre, depre_actua, depre_anho, depre_acum, valor_libro)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			b.id, corrPorcent, anho, cuota, nuevoBien, nuevoAnoVida, actuaDepre,
			depreActua, depreAnho, depreAcumNuevo, valorLibro); err != nil {
			return 0, err
		}
		procesados++
	}

	if _, err := tx.Exec(`
		INSERT INTO inv_anho_deprec (anho, fecha_andep, usuario_andep)
		VALUES ($1, CURRENT_DATE, $2)`, anho, idUsuario); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return procesados, nil
}

// ---------- Reportes ----------

func (r *InventarioRepository) BienesPorResponsable(idEmpleado int, desde, hasta string) ([]models.Bien, error) {
	query := `
SELECT b.corr_bien, b.codigo_bien, COALESCE(b.corr_tbien, 0),
       COALESCE(t.descripcion_tbien, ''), COALESCE(b.descripcion, ''),
       b.monto_bien, COALESCE(b.estado, ''), COALESCE(b.cuenta, ''),
       COALESCE(b.nro_docto_compra::text, ''), COALESCE(b.tipo_docto_compra, ''),
       COALESCE(b.rut_proveedor, ''), '', '', '', NULL, '', r.fecha_resp
FROM inv_bien b
JOIN inv_responsable r ON r.corr_bien = b.corr_bien AND r.estado = 'V'
JOIN inv_tipo_bien t ON t.corr_tbien = b.corr_tbien
WHERE r.id_empleado = $1`
	args := []interface{}{idEmpleado}
	if desde != "" {
		args = append(args, desde)
		query += fmt.Sprintf(" AND r.fecha_resp >= $%d::date", len(args))
	}
	if hasta != "" {
		args = append(args, hasta)
		query += fmt.Sprintf(" AND r.fecha_resp <= $%d::date", len(args))
	}
	query += ` ORDER BY b.codigo_bien`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		var b models.Bien
		var fecha sql.NullTime
		if err := rows.Scan(&b.CorrBien, &b.CodigoBien, &b.CorrTbien, &b.TipoBien,
			&b.Descripcion, &b.Monto, &b.Estado, &b.Cuenta, &b.NroDoctoCompra,
			&b.TipoDoctoCompra, &b.RutProveedor, &b.Piso, &b.Oficina, &b.Zona,
			&b.FechaUbicacion, &b.Responsables, &fecha); err != nil {
			return nil, err
		}
		if fecha.Valid {
			b.FechaAsignacion = &fecha.Time
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

func (r *InventarioRepository) BienesPorUbicacion(corrZona, corrTbien int, piso, oficina string) ([]models.Bien, error) {
	where := []string{"u.corr_ubica IN (SELECT MAX(corr_ubica) FROM inv_ubicacion GROUP BY corr_bien)"}
	args := []interface{}{}
	if corrZona > 0 {
		args = append(args, corrZona)
		where = append(where, fmt.Sprintf("u.corr_zona = $%d", len(args)))
	}
	if corrTbien > 0 {
		args = append(args, corrTbien)
		where = append(where, fmt.Sprintf("b.corr_tbien = $%d", len(args)))
	}
	if piso != "" {
		args = append(args, piso)
		where = append(where, fmt.Sprintf("u.piso = $%d", len(args)))
	}
	if oficina != "" {
		args = append(args, oficina)
		where = append(where, fmt.Sprintf("u.oficina = $%d", len(args)))
	}
	query := `
SELECT b.corr_bien, b.codigo_bien, COALESCE(b.corr_tbien, 0),
       COALESCE(t.descripcion_tbien, ''), COALESCE(b.descripcion, ''),
       b.monto_bien, COALESCE(b.estado, ''), COALESCE(b.cuenta, ''),
       COALESCE(b.nro_docto_compra::text, ''), COALESCE(b.tipo_docto_compra, ''),
       COALESCE(b.rut_proveedor, ''), COALESCE(u.piso, ''), COALESCE(u.oficina, ''),
       COALESCE(z.desc_zona, ''), u.fecha_ubica, ''
FROM inv_bien b
JOIN inv_ubicacion u ON u.corr_bien = b.corr_bien
JOIN inv_tipo_bien t ON t.corr_tbien = b.corr_tbien
LEFT JOIN inv_zona z ON z.corr_zona = u.corr_zona
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY b.codigo_bien`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		b, err := scanBien(rows.Scan)
		if err != nil {
			return nil, err
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

func (r *InventarioRepository) BienesAdquiridosAnio(anho string, corrTbien int) ([]models.Bien, error) {
	where := []string{"m.concepto = 'I'", "EXTRACT(YEAR FROM m.fecha_movi) = $1::int"}
	args := []interface{}{anho}
	if corrTbien > 0 {
		args = append(args, corrTbien)
		where = append(where, fmt.Sprintf("b.corr_tbien = $%d", len(args)))
	}
	query := `
SELECT b.corr_bien, b.codigo_bien, COALESCE(b.corr_tbien, 0),
       COALESCE(t.descripcion_tbien, ''), COALESCE(b.descripcion, ''),
       b.monto_bien, COALESCE(b.estado, ''), COALESCE(b.cuenta, ''),
       COALESCE(b.nro_docto_compra::text, ''), COALESCE(b.tipo_docto_compra, ''),
       COALESCE(b.rut_proveedor, ''), COALESCE(u.piso, ''), COALESCE(u.oficina, ''),
       COALESCE(z.desc_zona, ''), u.fecha_ubica, '', m.fecha_movi
FROM inv_bien b
JOIN inv_tipo_bien t ON t.corr_tbien = b.corr_tbien
JOIN inv_det_movimiento d ON d.corr_bien = b.corr_bien
JOIN inv_movimiento m ON m.corr_movi = d.corr_movi
LEFT JOIN inv_ubicacion u ON u.corr_ubica = (
    SELECT MAX(corr_ubica) FROM inv_ubicacion WHERE corr_bien = b.corr_bien)
LEFT JOIN inv_zona z ON z.corr_zona = u.corr_zona
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY m.fecha_movi, b.codigo_bien`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		var b models.Bien
		var fechaAdq sql.NullTime
		if err := rows.Scan(&b.CorrBien, &b.CodigoBien, &b.CorrTbien, &b.TipoBien,
			&b.Descripcion, &b.Monto, &b.Estado, &b.Cuenta, &b.NroDoctoCompra,
			&b.TipoDoctoCompra, &b.RutProveedor, &b.Piso, &b.Oficina, &b.Zona,
			&b.FechaUbicacion, &b.Responsables, &fechaAdq); err != nil {
			return nil, err
		}
		if fechaAdq.Valid {
			b.FechaAdquisicion = &fechaAdq.Time
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

// BienesDepreciados lista los bienes que tienen registros de depreciación.
func (r *InventarioRepository) BienesDepreciados(corrTbien int, codigo string) ([]models.Bien, error) {
	where := []string{"b.corr_bien IN (SELECT DISTINCT corr_bien FROM inv_depreciacion)"}
	args := []interface{}{}
	if corrTbien > 0 {
		args = append(args, corrTbien)
		where = append(where, fmt.Sprintf("b.corr_tbien = $%d", len(args)))
	}
	if codigo != "" {
		args = append(args, "%"+codigo+"%")
		where = append(where, fmt.Sprintf("b.codigo_bien ILIKE $%d", len(args)))
	}
	query := `
SELECT b.corr_bien, b.codigo_bien, COALESCE(b.corr_tbien, 0),
       COALESCE(t.descripcion_tbien, ''), COALESCE(b.descripcion, ''),
       b.monto_bien, COALESCE(b.estado, ''), COALESCE(b.cuenta, ''),
       COALESCE(b.nro_docto_compra::text, ''), COALESCE(b.tipo_docto_compra, ''),
       COALESCE(b.rut_proveedor, ''), '', '', '', NULL, '', m.fecha_movi
FROM inv_bien b
JOIN inv_tipo_bien t ON t.corr_tbien = b.corr_tbien
LEFT JOIN inv_det_movimiento d ON d.corr_bien = b.corr_bien
LEFT JOIN inv_movimiento m ON m.corr_movi = d.corr_movi AND m.concepto = 'I'
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY b.codigo_bien`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bienes []models.Bien
	for rows.Next() {
		var b models.Bien
		var fecha sql.NullTime
		if err := rows.Scan(&b.CorrBien, &b.CodigoBien, &b.CorrTbien, &b.TipoBien,
			&b.Descripcion, &b.Monto, &b.Estado, &b.Cuenta, &b.NroDoctoCompra,
			&b.TipoDoctoCompra, &b.RutProveedor, &b.Piso, &b.Oficina, &b.Zona,
			&b.FechaUbicacion, &b.Responsables, &fecha); err != nil {
			return nil, err
		}
		if fecha.Valid {
			b.FechaAdquisicion = &fecha.Time
		}
		bienes = append(bienes, b)
	}
	return bienes, rows.Err()
}

func (r *InventarioRepository) DepreciacionDetalle(corrBien int) ([]models.Depreciacion, error) {
	rows, err := r.db.Query(`
		SELECT d.corr_depre, d.anho, p.valor_porcent, d.cuota_actua, d.bien_actua,
		       d.ano_vida_util, d.actua_depre, d.depre_actua, d.depre_anho,
		       d.depre_acum, d.valor_libro
		FROM inv_depreciacion d
		JOIN inv_porcentaje p ON p.corr_porcent = d.corr_porcent
		WHERE d.corr_bien = $1
		ORDER BY d.corr_depre`, corrBien)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filas []models.Depreciacion
	for rows.Next() {
		var d models.Depreciacion
		if err := rows.Scan(&d.CorrDepre, &d.Anho, &d.ValorPorcent, &d.CuotaActua,
			&d.BienActua, &d.AnoVidaUtil, &d.ActuaDepre, &d.DepreActua,
			&d.DepreAnho, &d.DepreAcum, &d.ValorLibro); err != nil {
			return nil, err
		}
		filas = append(filas, d)
	}
	return filas, rows.Err()
}

// ---------- Administración de catálogos ----------

func (r *InventarioRepository) CrearTipoBien(descripcion, codigo string) error {
	_, err := r.db.Exec(`
		INSERT INTO inv_tipo_bien (descripcion_tbien, codigo_tbien)
		VALUES ($1, $2)`, strings.TrimSpace(descripcion), strings.TrimSpace(codigo))
	return err
}

func (r *InventarioRepository) CrearZona(descripcion, codigo string) error {
	_, err := r.db.Exec(`
		INSERT INTO inv_zona (desc_zona, codigo_zona)
		VALUES ($1, $2)`, strings.TrimSpace(descripcion), strings.TrimSpace(codigo))
	return err
}

func (r *InventarioRepository) Proveedores() ([]models.CatalogItem, error) {
	rows, err := r.db.Query(`SELECT rut_proveedor, nombre_proveedor FROM adu_proveedormm WHERE estado = 'V' ORDER BY nombre_proveedor`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CatalogItem
	for rows.Next() {
		var it models.CatalogItem
		if err := rows.Scan(&it.Codigo, &it.Descripcion); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ---------- Utilidades ----------

// fechaCorta devuelve los últimos dos dígitos del año en curso.
func fechaCorta() string {
	return time.Now().Format("06")
}

// NormalizarRut quita puntos, guiones y espacios, y deja el RUT en mayúsculas.
func NormalizarRut(rut string) string {
	r := strings.ToUpper(strings.TrimSpace(rut))
	r = strings.NewReplacer(".", "", "-", "", " ", "").Replace(r)
	return r
}

var tiposDocOrigen = map[string]bool{"RE": true, "OP": true, "ME": true}
var tiposDocCompra = map[string]bool{"FA": true, "OP": true}

// errorUsuario es un rechazo del formulario, apto para mostrarse tal cual.
type errorUsuario struct{ msg string }

func (e errorUsuario) Error() string { return e.msg }

func nuevoErrorUsuario(msg string) error { return errorUsuario{msg: msg} }

// MensajeUsuario devuelve el texto cuando el error puede mostrarse en pantalla.
func MensajeUsuario(err error) (string, bool) {
	var eu errorUsuario
	if errors.As(err, &eu) {
		return eu.msg, true
	}
	return "", false
}
