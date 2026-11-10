package repository

import (
	"database/sql"
	_ "embed"
	"errors"
	"strings"
	"unicode/utf8"

	"Gobierno2Go/internal/models"

	"github.com/lib/pq"
)

//go:embed schema/002_adquisiciones_mantenedores.sql
var mantenedoresSQL string

//go:embed schema/003_adquisiciones_operaciones.sql
var operacionesSQL string

const materialesPorPagina = 15

var (
	ErrRegistroEnUso   = errors.New("el registro está en uso")
	ErrRutReferenciado = errors.New("el rut está referenciado")
)

type AdquisicionesRepository struct {
	db *sql.DB
}

func NewAdquisicionesRepository(db *sql.DB) *AdquisicionesRepository {
	return &AdquisicionesRepository{db: db}
}

func ejecutarScript(db *sql.DB, script string) error {
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// EnsureSchema crea los catálogos de mantenedores, las tablas operacionales y completa proveedores.
func (r *AdquisicionesRepository) EnsureSchema() error {
	if err := ejecutarScript(r.db, mantenedoresSQL); err != nil {
		return err
	}
	if err := ejecutarScript(r.db, operacionesSQL); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		ALTER TABLE adu_proveedormm
		ADD CONSTRAINT adu_proveedormm_region_fkey
		FOREIGN KEY (region_proveedor) REFERENCES adu_dominio(cod_dominio)`)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && (pqErr.Code == "42710" || pqErr.Code == "42P07") {
			return EnsureEstadoVigencia(r.db)
		}
		return err
	}
	return EnsureEstadoVigencia(r.db)
}

func esDuplicado(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func textoDentro(s string, max int) bool {
	return utf8.RuneCountInString(s) <= max
}

// ---------- Tipos de dominio ----------

func (r *AdquisicionesRepository) TiposDominio() ([]models.TipoDominio, error) {
	rows, err := r.db.Query(`
		SELECT cod_tipo_dom, desc_tipo_dom
		FROM adu_tipo_dominio
		WHERE estado = 'V'
		ORDER BY cod_tipo_dom`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.TipoDominio
	for rows.Next() {
		var t models.TipoDominio
		if err := rows.Scan(&t.Codigo, &t.Descripcion); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) TipoDominio(id int) (models.TipoDominio, error) {
	var t models.TipoDominio
	err := r.db.QueryRow(`
		SELECT cod_tipo_dom, desc_tipo_dom
		FROM adu_tipo_dominio
		WHERE cod_tipo_dom = $1 AND estado = 'V'`, id).Scan(&t.Codigo, &t.Descripcion)
	return t, err
}

func (r *AdquisicionesRepository) CrearTipoDominio(descripcion string) error {
	descripcion = strings.TrimSpace(descripcion)
	if descripcion == "" || !textoDentro(descripcion, 100) {
		return ErrDatos
	}
	_, err := r.db.Exec(`INSERT INTO adu_tipo_dominio (desc_tipo_dom) VALUES ($1)`, descripcion)
	return err
}

func (r *AdquisicionesRepository) ActualizarTipoDominio(id int, descripcion string) error {
	descripcion = strings.TrimSpace(descripcion)
	if id <= 0 || descripcion == "" || !textoDentro(descripcion, 100) {
		return ErrDatos
	}
	res, err := r.db.Exec(`
		UPDATE adu_tipo_dominio SET desc_tipo_dom = $2 WHERE cod_tipo_dom = $1 AND estado = 'V'`, id, descripcion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AdquisicionesRepository) EliminarTipoDominio(id int) error {
	if err := r.bloquearSiUsado(`SELECT COUNT(*) FROM adu_dominio WHERE cod_tipo_dom = $1 AND estado = 'V'`, id); err != nil {
		return err
	}
	return sinFilas(r.db.Exec(`UPDATE adu_tipo_dominio SET estado = 'E' WHERE cod_tipo_dom = $1 AND estado = 'V'`, id))
}

// ---------- Dominios ----------

func (r *AdquisicionesRepository) Dominios() ([]models.Dominio, error) {
	rows, err := r.db.Query(`
		SELECT d.cod_dominio, COALESCE(d.cod_tipo_dom, 0), COALESCE(t.desc_tipo_dom, ''), d.desc_dominio
		FROM adu_dominio d
		LEFT JOIN adu_tipo_dominio t ON t.cod_tipo_dom = d.cod_tipo_dom
		WHERE d.estado = 'V'
		ORDER BY d.cod_dominio`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Dominio
	for rows.Next() {
		var d models.Dominio
		if err := rows.Scan(&d.Codigo, &d.CodTipo, &d.TipoDescripcion, &d.Descripcion); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) DominiosPorTipo(codTipo int) ([]models.Dominio, error) {
	rows, err := r.db.Query(`
		SELECT cod_dominio, cod_tipo_dom, desc_dominio
		FROM adu_dominio
		WHERE cod_tipo_dom = $1 AND estado = 'V'
		ORDER BY desc_dominio`, codTipo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Dominio
	for rows.Next() {
		var d models.Dominio
		if err := rows.Scan(&d.Codigo, &d.CodTipo, &d.Descripcion); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) Dominio(id int) (models.Dominio, error) {
	var d models.Dominio
	err := r.db.QueryRow(`
		SELECT d.cod_dominio, COALESCE(d.cod_tipo_dom, 0), COALESCE(t.desc_tipo_dom, ''), d.desc_dominio
		FROM adu_dominio d
		LEFT JOIN adu_tipo_dominio t ON t.cod_tipo_dom = d.cod_tipo_dom
		WHERE d.cod_dominio = $1 AND d.estado = 'V'`, id).Scan(&d.Codigo, &d.CodTipo, &d.TipoDescripcion, &d.Descripcion)
	return d, err
}

func (r *AdquisicionesRepository) tipoExiste(id int) (bool, error) {
	var ok bool
	err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM adu_tipo_dominio WHERE cod_tipo_dom = $1 AND estado = 'V')`, id).Scan(&ok)
	return ok, err
}

func (r *AdquisicionesRepository) CrearDominio(codTipo int, descripcion string) error {
	descripcion = strings.TrimSpace(descripcion)
	if codTipo <= 0 || descripcion == "" || !textoDentro(descripcion, 100) {
		return ErrDatos
	}
	ok, err := r.tipoExiste(codTipo)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDatos
	}
	_, err = r.db.Exec(`
		INSERT INTO adu_dominio (cod_tipo_dom, desc_dominio) VALUES ($1, $2)`, codTipo, descripcion)
	return err
}

func (r *AdquisicionesRepository) ActualizarDominio(id int, descripcion string) error {
	descripcion = strings.TrimSpace(descripcion)
	if id <= 0 || descripcion == "" || !textoDentro(descripcion, 100) {
		return ErrDatos
	}
	res, err := r.db.Exec(`
		UPDATE adu_dominio SET desc_dominio = $2 WHERE cod_dominio = $1 AND estado = 'V'`, id, descripcion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AdquisicionesRepository) EliminarDominio(id int) error {
	if err := r.bloquearSiUsado(`
		SELECT (SELECT COUNT(*) FROM adu_materialmm WHERE tipo_material = $1 AND estado = 'V')
		     + (SELECT COUNT(*) FROM adu_proveedormm WHERE region_proveedor = $1 AND estado = 'V')`, id); err != nil {
		return err
	}
	return sinFilas(r.db.Exec(`UPDATE adu_dominio SET estado = 'E' WHERE cod_dominio = $1 AND estado = 'V'`, id))
}

// ---------- Materiales ----------

func (r *AdquisicionesRepository) ContarMateriales() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM adu_materialmm WHERE estado = 'V'`).Scan(&n)
	return n, err
}

func (r *AdquisicionesRepository) Materiales(pagina int) ([]models.Material, error) {
	if pagina < 1 {
		pagina = 1
	}
	offset := (pagina - 1) * materialesPorPagina
	rows, err := r.db.Query(`
		SELECT m.cod_material, m.nombre_material, m.tipo_material, COALESCE(d.desc_dominio, ''), m.stock_critico
		FROM adu_materialmm m
		LEFT JOIN adu_dominio d ON d.cod_dominio = m.tipo_material
		WHERE m.estado = 'V'
		ORDER BY m.nombre_material
		LIMIT $1 OFFSET $2`, materialesPorPagina, offset)
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

func (r *AdquisicionesRepository) Material(id int) (models.Material, error) {
	var m models.Material
	err := r.db.QueryRow(`
		SELECT m.cod_material, m.nombre_material, m.tipo_material, COALESCE(d.desc_dominio, ''), m.stock_critico
		FROM adu_materialmm m
		LEFT JOIN adu_dominio d ON d.cod_dominio = m.tipo_material
		WHERE m.cod_material = $1 AND m.estado = 'V'`, id).Scan(&m.Codigo, &m.Nombre, &m.Tipo, &m.TipoNombre, &m.StockCritico)
	return m, err
}

func (r *AdquisicionesRepository) dominioEsTipo(id, codTipo int) (bool, error) {
	var ok bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM adu_dominio WHERE cod_dominio = $1 AND cod_tipo_dom = $2 AND estado = 'V'
		)`, id, codTipo).Scan(&ok)
	return ok, err
}

func (r *AdquisicionesRepository) validarMaterial(m models.Material) error {
	m.Nombre = strings.TrimSpace(m.Nombre)
	if m.Nombre == "" || !textoDentro(m.Nombre, 100) || m.StockCritico < 0 || m.Tipo <= 0 {
		return ErrDatos
	}
	ok, err := r.dominioEsTipo(m.Tipo, models.CodTipoMaterial)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDatos
	}
	return nil
}

func (r *AdquisicionesRepository) CrearMaterial(m models.Material) error {
	if err := r.validarMaterial(m); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO adu_materialmm (nombre_material, tipo_material, stock_critico)
		VALUES ($1, $2, $3)`, strings.TrimSpace(m.Nombre), m.Tipo, m.StockCritico)
	return err
}

func (r *AdquisicionesRepository) ActualizarMaterial(m models.Material) error {
	if m.Codigo <= 0 {
		return ErrDatos
	}
	if err := r.validarMaterial(m); err != nil {
		return err
	}
	res, err := r.db.Exec(`
		UPDATE adu_materialmm
		SET nombre_material = $2, tipo_material = $3, stock_critico = $4
		WHERE cod_material = $1 AND estado = 'V'`, m.Codigo, strings.TrimSpace(m.Nombre), m.Tipo, m.StockCritico)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AdquisicionesRepository) EliminarMaterial(id int) error {
	if err := r.bloquearSiUsado(`
		SELECT (SELECT COUNT(*) FROM adu_stock WHERE cod_material = $1)
		     + (SELECT COUNT(*) FROM adu_detalle_facturapr WHERE cod_material = $1)
		     + (SELECT COUNT(*) FROM adu_detalle_ordenco WHERE cod_material = $1 AND estado = 'V')
		     + (SELECT COUNT(*) FROM adu_detalle_sol_mat WHERE cod_material = $1 AND estado = 'V')`, id); err != nil {
		return err
	}
	return sinFilas(r.db.Exec(`UPDATE adu_materialmm SET estado = 'E' WHERE cod_material = $1 AND estado = 'V'`, id))
}

// ---------- Proveedores ----------

func (r *AdquisicionesRepository) Proveedores() ([]models.Proveedor, error) {
	rows, err := r.db.Query(`
		SELECT btrim(p.rut_proveedor), p.nombre_proveedor,
		       COALESCE(p.nom_fan_proveedor, ''), COALESCE(p.direcion_proveedor, ''),
		       COALESCE(p.ciudad_proveedor, ''), COALESCE(p.region_proveedor, 0),
		       COALESCE(d.desc_dominio, ''), COALESCE(btrim(p.fono_proveedor), ''),
		       COALESCE(p.contacto_proveedor, ''), COALESCE(btrim(p.fax_proveedor), ''),
		       COALESCE(p.mail_proveedor, '')
		FROM adu_proveedormm p
		LEFT JOIN adu_dominio d ON d.cod_dominio = p.region_proveedor
		WHERE p.estado = 'V'
		ORDER BY p.nombre_proveedor`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Proveedor
	for rows.Next() {
		var p models.Proveedor
		if err := rows.Scan(&p.Rut, &p.Nombre, &p.NombreFantasia, &p.Direccion, &p.Ciudad,
			&p.Region, &p.RegionNombre, &p.Fono, &p.Contacto, &p.Fax, &p.Mail); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) Proveedor(rut string) (models.Proveedor, error) {
	var p models.Proveedor
	err := r.db.QueryRow(`
		SELECT btrim(p.rut_proveedor), p.nombre_proveedor,
		       COALESCE(p.nom_fan_proveedor, ''), COALESCE(p.direcion_proveedor, ''),
		       COALESCE(p.ciudad_proveedor, ''), COALESCE(p.region_proveedor, 0),
		       COALESCE(d.desc_dominio, ''), COALESCE(btrim(p.fono_proveedor), ''),
		       COALESCE(p.contacto_proveedor, ''), COALESCE(btrim(p.fax_proveedor), ''),
		       COALESCE(p.mail_proveedor, '')
		FROM adu_proveedormm p
		LEFT JOIN adu_dominio d ON d.cod_dominio = p.region_proveedor
		WHERE btrim(p.rut_proveedor) = $1 AND p.estado = 'V'`, NormalizarRut(rut)).
		Scan(&p.Rut, &p.Nombre, &p.NombreFantasia, &p.Direccion, &p.Ciudad,
			&p.Region, &p.RegionNombre, &p.Fono, &p.Contacto, &p.Fax, &p.Mail)
	return p, err
}

func (r *AdquisicionesRepository) validarProveedor(p *models.Proveedor) error {
	p.Rut = NormalizarRut(p.Rut)
	p.Nombre = strings.TrimSpace(p.Nombre)
	p.NombreFantasia = strings.TrimSpace(p.NombreFantasia)
	p.Direccion = strings.TrimSpace(p.Direccion)
	p.Ciudad = strings.TrimSpace(p.Ciudad)
	p.Fono = strings.TrimSpace(p.Fono)
	p.Contacto = strings.TrimSpace(p.Contacto)
	p.Fax = strings.TrimSpace(p.Fax)
	p.Mail = strings.TrimSpace(p.Mail)
	if p.Rut == "" || !textoDentro(p.Rut, 12) || p.Nombre == "" || !textoDentro(p.Nombre, 100) {
		return ErrDatos
	}
	if !textoDentro(p.NombreFantasia, 100) || !textoDentro(p.Direccion, 100) || !textoDentro(p.Ciudad, 100) ||
		!textoDentro(p.Fono, 15) || !textoDentro(p.Contacto, 100) || !textoDentro(p.Fax, 15) || !textoDentro(p.Mail, 50) {
		return ErrDatos
	}
	if p.Mail != "" && !mailAceptable(p.Mail) {
		return ErrDatos
	}
	if p.Region != 0 {
		ok, err := r.dominioEsTipo(p.Region, models.CodTipoRegion)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDatos
		}
	}
	return nil
}

func mailAceptable(mail string) bool {
	if strings.ContainsAny(mail, " \"'\\$") || strings.Contains(mail, " ") {
		return false
	}
	at := strings.Index(mail, "@")
	if at <= 0 || at != strings.LastIndex(mail, "@") || at == len(mail)-1 {
		return false
	}
	dom := mail[at+1:]
	dot := strings.LastIndex(dom, ".")
	return dot > 0 && dot < len(dom)-1
}

func nullTexto(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullEntero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func (r *AdquisicionesRepository) CrearProveedor(p models.Proveedor) error {
	if err := r.validarProveedor(&p); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO adu_proveedormm (
			rut_proveedor, nombre_proveedor, nom_fan_proveedor, direcion_proveedor,
			ciudad_proveedor, region_proveedor, fono_proveedor, contacto_proveedor,
			fax_proveedor, mail_proveedor
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		p.Rut, p.Nombre, nullTexto(p.NombreFantasia), nullTexto(p.Direccion),
		nullTexto(p.Ciudad), nullEntero(p.Region), nullTexto(p.Fono), nullTexto(p.Contacto),
		nullTexto(p.Fax), nullTexto(p.Mail))
	if esDuplicado(err) {
		return ErrDuplicado
	}
	return err
}

func (r *AdquisicionesRepository) ActualizarProveedor(rutActual string, p models.Proveedor) error {
	rutActual = NormalizarRut(rutActual)
	if rutActual == "" {
		return ErrDatos
	}
	if err := r.validarProveedor(&p); err != nil {
		return err
	}
	if p.Rut != rutActual {
		var usado bool
		err := r.db.QueryRow(`
			SELECT EXISTS (SELECT 1 FROM adu_orden_compra WHERE btrim(rut_proveedor) = $1)
			    OR EXISTS (SELECT 1 FROM inv_bien WHERE btrim(rut_proveedor) = $1)`, rutActual).Scan(&usado)
		if err != nil {
			return err
		}
		if usado {
			return ErrRutReferenciado
		}
	}
	res, err := r.db.Exec(`
		UPDATE adu_proveedormm SET
			rut_proveedor = $1,
			nombre_proveedor = $2,
			nom_fan_proveedor = $3,
			direcion_proveedor = $4,
			ciudad_proveedor = $5,
			region_proveedor = $6,
			fono_proveedor = $7,
			contacto_proveedor = $8,
			fax_proveedor = $9,
			mail_proveedor = $10
		WHERE btrim(rut_proveedor) = $11 AND estado = 'V'`,
		p.Rut, p.Nombre, nullTexto(p.NombreFantasia), nullTexto(p.Direccion),
		nullTexto(p.Ciudad), nullEntero(p.Region), nullTexto(p.Fono), nullTexto(p.Contacto),
		nullTexto(p.Fax), nullTexto(p.Mail), rutActual)
	if esDuplicado(err) {
		return ErrDuplicado
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AdquisicionesRepository) EliminarProveedor(rut string) error {
	rut = NormalizarRut(rut)
	if err := r.bloquearSiUsado(`
		SELECT (SELECT COUNT(*) FROM adu_orden_compra WHERE btrim(rut_proveedor) = $1 AND estado = 'V')
		     + (SELECT COUNT(*) FROM adu_facturapr WHERE btrim(rut_proveedor) = $1)
		     + (SELECT COUNT(*) FROM inv_bien WHERE btrim(rut_proveedor) = $1)`, rut); err != nil {
		return err
	}
	return sinFilas(r.db.Exec(`UPDATE adu_proveedormm SET estado = 'E' WHERE btrim(rut_proveedor) = $1 AND estado = 'V'`, rut))
}

// ---------- Asignaciones ----------

func (r *AdquisicionesRepository) AsignacionesMantenedor() ([]models.Asignacion, error) {
	rows, err := r.db.Query(`
		SELECT corr_asig, btrim(cod_asig), descripcion
		FROM adu_asignacion
		WHERE estado = 'V'
		ORDER BY corr_asig`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Asignacion
	for rows.Next() {
		var a models.Asignacion
		if err := rows.Scan(&a.Correlativo, &a.Codigo, &a.Descripcion); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AdquisicionesRepository) Asignacion(id int) (models.Asignacion, error) {
	var a models.Asignacion
	err := r.db.QueryRow(`
		SELECT corr_asig, btrim(cod_asig), descripcion
		FROM adu_asignacion
		WHERE corr_asig = $1 AND estado = 'V'`, id).Scan(&a.Correlativo, &a.Codigo, &a.Descripcion)
	return a, err
}

func validarAsignacion(a *models.Asignacion) error {
	a.Codigo = strings.TrimSpace(a.Codigo)
	a.Descripcion = strings.TrimSpace(a.Descripcion)
	if a.Codigo == "" || !textoDentro(a.Codigo, 20) || a.Descripcion == "" || !textoDentro(a.Descripcion, 200) {
		return ErrDatos
	}
	return nil
}

func (r *AdquisicionesRepository) CrearAsignacion(a models.Asignacion) error {
	if err := validarAsignacion(&a); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO adu_asignacion (cod_asig, descripcion) VALUES ($1, $2)`, a.Codigo, a.Descripcion)
	return err
}

func (r *AdquisicionesRepository) ActualizarAsignacion(a models.Asignacion) error {
	if a.Correlativo <= 0 {
		return ErrDatos
	}
	if err := validarAsignacion(&a); err != nil {
		return err
	}
	res, err := r.db.Exec(`
		UPDATE adu_asignacion SET cod_asig = $2, descripcion = $3 WHERE corr_asig = $1 AND estado = 'V'`,
		a.Correlativo, a.Codigo, a.Descripcion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AdquisicionesRepository) EliminarAsignacion(id int) error {
	if err := r.bloquearSiUsado(`
		SELECT (SELECT COUNT(*) FROM adu_imputacion_orden WHERE corr_asig = $1 AND estado = 'V')
		     + (SELECT COUNT(*) FROM inv_imputacion_bien WHERE corr_asig = $1)`, id); err != nil {
		return err
	}
	return sinFilas(r.db.Exec(`UPDATE adu_asignacion SET estado = 'E' WHERE corr_asig = $1 AND estado = 'V'`, id))
}

func (r *AdquisicionesRepository) bloquearSiUsado(q string, args ...any) error {
	var n int
	if err := r.db.QueryRow(q, args...).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrRegistroEnUso
	}
	return nil
}

// PaginaMateriales arma los datos de paginación de 15 registros, como el listado PHP.
func PaginaMateriales(total, actual int) models.Pagina {
	if actual < 1 {
		actual = 1
	}
	paginas := (total + materialesPorPagina - 1) / materialesPorPagina
	if paginas == 0 {
		paginas = 1
	}
	if actual > paginas {
		actual = paginas
	}
	nums := make([]int, 0, paginas)
	for i := 1; i <= paginas; i++ {
		nums = append(nums, i)
	}
	return models.Pagina{Actual: actual, Total: paginas, Paginas: nums}
}
