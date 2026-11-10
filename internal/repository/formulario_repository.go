package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"Gobierno2Go/internal/models"
)

type FormularioRepository struct {
	db *sql.DB
}

func NewFormularioRepository(db *sql.DB) *FormularioRepository {
	return &FormularioRepository{db: db}
}

func (r *FormularioRepository) empleadoBaseQuery() string {
	return `
SELECT d.id_empleado,
       d.nombre || ' ' || d.apellido,
       COALESCE(d.depto, 0),
       COALESCE(dep.descripcion, ''),
       COALESCE(d.email, ''),
       COALESCE(de.id_dias, 0),
       COALESCE(de.feriados_anuales, 0),
       COALESCE(de.administrativos, 0),
       COALESCE((SELECT SUM(dp.dias_tomados) FROM for_dias_permiso dp
                 WHERE dp.id_dias = de.id_dias AND dp.anho = $1 AND dp.estado = 'V'
                   AND dp.tipo_permiso = 'F' AND dp.vbjefe = 'S'), 0),
       COALESCE((SELECT SUM(dp.dias_tomados) FROM for_dias_permiso dp
                 WHERE dp.id_dias = de.id_dias AND dp.anho = $1 AND dp.estado = 'V'
                   AND dp.tipo_permiso = 'A' AND dp.vbjefe = 'S'), 0),
       COALESCE((SELECT SUM(sa.dias_sol) FROM for_solic_acumferiado sa
                 JOIN for_dias_permiso dp ON dp.id_dpermiso = sa.id_dpermiso
                 WHERE dp.id_dias = de.id_dias AND dp.anho = $1 AND dp.estado = 'V'
                   AND sa.estado = 'V' AND sa.vb_personal = TRUE), 0)
FROM gen_directorio d
LEFT JOIN for_dias_empleado de ON de.id_empleado = d.id_empleado
LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto
WHERE COALESCE(nullif(btrim(d.estado), ''), 'V') = 'V'`
}

func scanEmpleado(scan func(...interface{}) error) (models.EmpleadoFormulario, error) {
	var e models.EmpleadoFormulario
	if err := scan(&e.IDEmpleado, &e.Nombre, &e.Depto, &e.DeptoNombre, &e.Email,
		&e.IDDias, &e.FeriadosAnuales, &e.Administrativos,
		&e.FeriadosUsados, &e.AdminUsados, &e.Acumulados); err != nil {
		return e, err
	}
	e.FeriadosRestantes = e.FeriadosAnuales - e.FeriadosUsados
	e.AdminRestantes = e.Administrativos - e.AdminUsados
	return e, nil
}

func (r *FormularioRepository) Empleados(anho int) ([]models.EmpleadoFormulario, error) {
	rows, err := r.db.Query(r.empleadoBaseQuery()+` ORDER BY d.nombre, d.apellido`, anho)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var empleados []models.EmpleadoFormulario
	for rows.Next() {
		e, err := scanEmpleado(rows.Scan)
		if err != nil {
			return nil, err
		}
		empleados = append(empleados, e)
	}
	return empleados, rows.Err()
}

func (r *FormularioRepository) EmpleadoPorID(idEmpleado, anho int) (models.EmpleadoFormulario, error) {
	row := r.db.QueryRow(r.empleadoBaseQuery()+` WHERE d.id_empleado = $2`, anho, idEmpleado)
	return scanEmpleado(row.Scan)
}

func (r *FormularioRepository) EmpleadoDeUsuario(idUsuario string, anho int) (models.EmpleadoFormulario, error) {
	row := r.db.QueryRow(r.empleadoBaseQuery()+`
		AND d.id_empleado = (
			SELECT u.id_empleado FROM gen_usuario u WHERE u.id_usuario = $2
		)`, anho, idUsuario)
	return scanEmpleado(row.Scan)
}

// UsuarioInfo entrega el empleado, departamento y permisos del usuario.
func (r *FormularioRepository) UsuarioInfo(idUsuario string) (idEmpleado, idDepto int, esPersonal, esAdmin bool, err error) {
	var tipo string
	err = r.db.QueryRow(`
		SELECT COALESCE(u.id_empleado, 0), COALESCE(d.depto, 0), COALESCE(u.tipo, '3')
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		WHERE u.id_usuario = $1`, idUsuario).Scan(&idEmpleado, &idDepto, &tipo)
	if err != nil {
		return 0, 0, false, false, err
	}
	esPersonal = idDepto == 17
	adminSistema, errAdmin := esAdministradorDeSistema(r.db, idUsuario)
	if errAdmin != nil {
		return 0, 0, false, false, errAdmin
	}
	esAdmin = tipo == "0" || esPersonal || adminSistema
	return idEmpleado, idDepto, esPersonal, esAdmin, nil
}

func (r *FormularioRepository) CrearSolicitud(in models.CrearSolicitudFormulario, anho int) (int, error) {
	var id int
	err := r.db.QueryRow(`
		INSERT INTO for_dias_permiso
			(id_dias, dias_tomados, fecha_inicio, fecha_fin, vbjefe, anho,
			 tipo_permiso, fecha_solicitud, hora_solicitud,
			 fecha_resolucion, hora_resolucion, mdia)
		SELECT de.id_dias, $2, $3, $4, $5::char, $6, $7, CURRENT_DATE, CURRENT_TIME,
		       CASE WHEN $5::text = 'S' THEN CURRENT_DATE END,
		       CASE WHEN $5::text = 'S' THEN CURRENT_TIME END,
		       NULLIF($8, '')
		FROM for_dias_empleado de
		WHERE de.id_empleado = $1
		RETURNING id_dpermiso`,
		in.IDEmpleado, in.DiasTomados, in.FechaInicio, in.FechaFin,
		in.VBJefe, anho, in.TipoPermiso, in.MDia).Scan(&id)
	return id, err
}

func (r *FormularioRepository) solicitudBaseQuery() string {
	return `
SELECT dp.id_dpermiso,
       d.id_empleado,
       d.nombre || ' ' || d.apellido,
       COALESCE(dep.descripcion, ''),
       dp.tipo_permiso,
       dp.dias_tomados,
       dp.fecha_inicio,
       dp.fecha_fin,
       dp.anho,
       dp.vbjefe,
       dp.fecha_solicitud,
       dp.fecha_resolucion,
       dp.fecha_posterga_anticipa,
       COALESCE(dp.mdia, ''),
       COALESCE(dp.motivo_rechazo, ''),
       dp.acumulado
FROM for_dias_permiso dp
JOIN for_dias_empleado de ON de.id_dias = dp.id_dias
JOIN gen_directorio d ON d.id_empleado = de.id_empleado
LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto`
}

func scanSolicitud(scan func(...interface{}) error) (models.SolicitudFormulario, error) {
	var s models.SolicitudFormulario
	var fechaRes, fechaPost sql.NullTime
	err := scan(&s.IDDpermiso, &s.IDEmpleado, &s.Empleado, &s.Depto,
		&s.TipoPermiso, &s.DiasTomados, &s.FechaInicio, &s.FechaFin,
		&s.Anho, &s.VBJefe, &s.FechaSolicitud, &fechaRes, &fechaPost,
		&s.MDia, &s.MotivoRechazo, &s.Acumulado)
	if fechaRes.Valid {
		s.FechaResolucion = &fechaRes.Time
	}
	if fechaPost.Valid {
		s.FechaPosterga = &fechaPost.Time
	}
	return s, err
}

func (r *FormularioRepository) Solicitudes(anho, idEmpleado int, soloPendientes bool) ([]models.SolicitudFormulario, error) {
	where := []string{"dp.anho = $1", "dp.estado = 'V'"}
	args := []interface{}{anho}
	if idEmpleado > 0 {
		args = append(args, idEmpleado)
		where = append(where, fmt.Sprintf("d.id_empleado = $%d", len(args)))
	}
	if soloPendientes {
		where = append(where, "dp.vbjefe = 'N' AND dp.fecha_resolucion IS NULL")
	}
	query := r.solicitudBaseQuery() + " WHERE " + strings.Join(where, " AND ") +
		" ORDER BY dp.fecha_solicitud DESC, dp.id_dpermiso DESC"
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solicitudes []models.SolicitudFormulario
	for rows.Next() {
		s, err := scanSolicitud(rows.Scan)
		if err != nil {
			return nil, err
		}
		solicitudes = append(solicitudes, s)
	}
	return solicitudes, rows.Err()
}

func (r *FormularioRepository) SolicitudPorID(id int) (models.SolicitudFormulario, error) {
	row := r.db.QueryRow(r.solicitudBaseQuery()+` WHERE dp.id_dpermiso = $1 AND dp.estado = 'V'`, id)
	return scanSolicitud(row.Scan)
}

func (r *FormularioRepository) SolicitudesPendientesAprobador(idUsuario string) ([]models.SolicitudFormulario, error) {
	idEmpleado, _, esPersonal, esAdmin, err := r.UsuarioInfo(idUsuario)
	if err != nil {
		return nil, err
	}
	query := r.solicitudBaseQuery() + `
WHERE dp.estado = 'V' AND dp.vbjefe = 'N' AND dp.fecha_resolucion IS NULL
  AND (
    $2::boolean
    OR (d.id_empleado <> $1 AND EXISTS (
          SELECT 1 FROM for_jefe_depto jd
          WHERE jd.id_depto = d.depto AND jd.id_empleado = $1))
    OR EXISTS (
          SELECT 1 FROM for_jefe_depto jd
          WHERE jd.id_depto = d.depto AND jd.id_empleado = d.id_empleado
            AND jd.aprueba_jefe = $1)
  )
ORDER BY dp.fecha_solicitud, dp.id_dpermiso`
	rows, err := r.db.Query(query, idEmpleado, esAdmin || esPersonal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var solicitudes []models.SolicitudFormulario
	for rows.Next() {
		s, err := scanSolicitud(rows.Scan)
		if err != nil {
			return nil, err
		}
		solicitudes = append(solicitudes, s)
	}
	return solicitudes, rows.Err()
}

func (r *FormularioRepository) PuedeResolver(idUsuario string, idSolicitud int) (bool, error) {
	var existe bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM for_dias_permiso dp
			JOIN for_dias_empleado de ON de.id_dias = dp.id_dias
			JOIN gen_directorio d ON d.id_empleado = de.id_empleado
			JOIN gen_usuario u ON u.id_usuario = $1
			LEFT JOIN gen_directorio ud ON ud.id_empleado = u.id_empleado
			WHERE dp.id_dpermiso = $2
			  AND dp.estado = 'V'
			  AND dp.vbjefe = 'N' AND dp.fecha_resolucion IS NULL
			  AND (
			    COALESCE(u.tipo, '3') = '0' OR COALESCE(ud.depto, 0) = 17
			    OR EXISTS (
			          SELECT 1
			          FROM seg_usuario_perfil up
			          JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
			          JOIN seg_perfil_modulo pm ON pm.id_perfil = up.id_perfil AND pm.estado = 'V'
			          WHERE up.id_usuario = u.id_usuario
			            AND up.estado = 'V'
			            AND pm.codigo_modulo = 'administracion')
			    OR (d.id_empleado <> COALESCE(u.id_empleado, 0) AND EXISTS (
			          SELECT 1 FROM for_jefe_depto jd
			          WHERE jd.id_depto = d.depto AND jd.id_empleado = u.id_empleado))
			    OR EXISTS (
			          SELECT 1 FROM for_jefe_depto jd
			          WHERE jd.id_depto = d.depto AND jd.id_empleado = d.id_empleado
			            AND jd.aprueba_jefe = u.id_empleado)
			  )
		)`, idUsuario, idSolicitud).Scan(&existe)
	return existe, err
}

func (r *FormularioRepository) ResolverSolicitud(id int, aprobar bool, motivo string) error {
	vb := "N"
	if aprobar {
		vb = "S"
		motivo = ""
	}
	res, err := r.db.Exec(`
		UPDATE for_dias_permiso
		SET vbjefe = $2, fecha_resolucion = CURRENT_DATE, hora_resolucion = CURRENT_TIME,
		    motivo_rechazo = NULLIF($3, '')
		WHERE id_dpermiso = $1 AND estado = 'V' AND vbjefe = 'N' AND fecha_resolucion IS NULL`,
		id, vb, motivo)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("la solicitud ya fue resuelta o no existe")
	}
	return nil
}

func (r *FormularioRepository) EliminarSolicitud(id int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE for_solic_acumferiado SET estado = 'E' WHERE id_dpermiso = $1 AND estado = 'V'`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE for_dias_permiso SET estado = 'E' WHERE id_dpermiso = $1 AND estado = 'V'`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *FormularioRepository) Acumulaciones(idEmpleado int, soloPendientes bool) ([]models.AcumulacionFormulario, error) {
	where := []string{"sa.estado = 'V'", "dp.estado = 'V'"}
	args := []interface{}{}
	if idEmpleado > 0 {
		args = append(args, idEmpleado)
		where = append(where, fmt.Sprintf("d.id_empleado = $%d", len(args)))
	}
	if soloPendientes {
		where = append(where, "sa.vb_personal = FALSE")
	}
	query := `
SELECT sa.correlativo, sa.id_dpermiso,
       d.nombre || ' ' || d.apellido,
       COALESCE(dep.descripcion, ''),
       sa.dias_sol, sa.vb_personal, sa.fecha_sol, sa.fecha_vb,
       dp.tipo_permiso, dp.fecha_inicio, dp.fecha_fin
FROM for_solic_acumferiado sa
JOIN for_dias_permiso dp ON dp.id_dpermiso = sa.id_dpermiso
JOIN for_dias_empleado de ON de.id_dias = dp.id_dias
JOIN gen_directorio d ON d.id_empleado = de.id_empleado
LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY sa.fecha_sol DESC, sa.correlativo DESC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acumulaciones []models.AcumulacionFormulario
	for rows.Next() {
		var a models.AcumulacionFormulario
		var fechaVB sql.NullTime
		if err := rows.Scan(&a.Correlativo, &a.IDDpermiso, &a.Empleado, &a.Depto,
			&a.DiasSol, &a.VBPersonal, &a.FechaSol, &fechaVB,
			&a.TipoPermiso, &a.FechaInicio, &a.FechaFin); err != nil {
			return nil, err
		}
		if fechaVB.Valid {
			a.FechaVB = &fechaVB.Time
		}
		acumulaciones = append(acumulaciones, a)
	}
	return acumulaciones, rows.Err()
}

func (r *FormularioRepository) CrearAcumulacion(idDpermiso, dias int) error {
	_, err := r.db.Exec(`
		INSERT INTO for_solic_acumferiado (id_dpermiso, dias_sol)
		VALUES ($1, $2)`, idDpermiso, dias)
	return err
}

func (r *FormularioRepository) ResolverAcumulacion(correlativo int, aprobar bool) error {
	if !aprobar {
		_, err := r.db.Exec(`UPDATE for_solic_acumferiado SET estado = 'E' WHERE correlativo = $1 AND estado = 'V'`, correlativo)
		return err
	}
	var idDpermiso int
	err := r.db.QueryRow(`
		UPDATE for_solic_acumferiado
		SET vb_personal = TRUE, fecha_vb = CURRENT_DATE, hora_vb = CURRENT_TIME
		WHERE correlativo = $1 AND estado = 'V' AND vb_personal = FALSE
		RETURNING id_dpermiso`, correlativo).Scan(&idDpermiso)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`UPDATE for_dias_permiso SET acumulado = TRUE WHERE id_dpermiso = $1`, idDpermiso)
	return err
}

func (r *FormularioRepository) ActualizarDiasEmpleado(idEmpleado int, feriados, administrativos float64) error {
	res, err := r.db.Exec(`
		UPDATE for_dias_empleado
		SET feriados_anuales = $2, administrativos = $3, fecha_update = CURRENT_DATE
		WHERE id_empleado = $1`, idEmpleado, feriados, administrativos)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_, err = r.db.Exec(`
			INSERT INTO for_dias_empleado (id_empleado, feriados_anuales, administrativos)
			VALUES ($1, $2, $3)`, idEmpleado, feriados, administrativos)
	}
	return err
}

func (r *FormularioRepository) Jefes() ([]models.JefeDepto, error) {
	rows, err := r.db.Query(`
		SELECT dep.id_depto, dep.descripcion,
		       COALESCE(jd.id_empleado, 0),
		       COALESCE(d.nombre || ' ' || d.apellido, ''),
		       COALESCE(jd.aprueba_jefe, 0),
		       COALESCE(a.nombre || ' ' || a.apellido, '')
		FROM gen_departamento dep
		LEFT JOIN for_jefe_depto jd ON jd.id_depto = dep.id_depto
		LEFT JOIN gen_directorio d ON d.id_empleado = jd.id_empleado
		LEFT JOIN gen_directorio a ON a.id_empleado = jd.aprueba_jefe
		WHERE dep.estado = 'V'
		ORDER BY dep.descripcion`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jefes []models.JefeDepto
	for rows.Next() {
		var j models.JefeDepto
		if err := rows.Scan(&j.IDDepto, &j.DeptoNombre, &j.IDEmpleado, &j.Empleado,
			&j.IDAprueba, &j.Aprueba); err != nil {
			return nil, err
		}
		jefes = append(jefes, j)
	}
	return jefes, rows.Err()
}

func (r *FormularioRepository) GuardarJefes(config map[int][2]int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for idDepto, ids := range config {
		_, err := tx.Exec(`
			INSERT INTO for_jefe_depto (id_depto, id_empleado, aprueba_jefe)
			VALUES ($1, NULLIF($2, 0), NULLIF($3, 0))
			ON CONFLICT (id_depto) DO UPDATE
			SET id_empleado = EXCLUDED.id_empleado,
			    aprueba_jefe = EXCLUDED.aprueba_jefe`,
			idDepto, ids[0], ids[1])
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *FormularioRepository) Feriados(anho int) (map[string]bool, error) {
	rows, err := r.db.Query(`
		SELECT fecha FROM for_feriado
		WHERE EXTRACT(YEAR FROM fecha) = $1`, anho)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feriados := make(map[string]bool)
	for rows.Next() {
		var fecha sql.NullTime
		if err := rows.Scan(&fecha); err != nil {
			return nil, err
		}
		if fecha.Valid {
			feriados[fecha.Time.Format("2006-01-02")] = true
		}
	}
	return feriados, rows.Err()
}

func (r *FormularioRepository) Departamentos() ([]models.Departamento, error) {
	rows, err := r.db.Query(`SELECT id_depto, descripcion FROM gen_departamento WHERE estado = 'V' ORDER BY descripcion`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []models.Departamento
	for rows.Next() {
		var d models.Departamento
		if err := rows.Scan(&d.IDDepto, &d.Descripcion); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

func (r *FormularioRepository) TrabajadoresCRUD() ([]models.EmpleadoCRUD, error) {
	rows, err := r.db.Query(`
		SELECT d.id_empleado, COALESCE(d.nombre, ''), COALESCE(d.apellido, ''), COALESCE(d.email, ''), COALESCE(d.depto, 0), COALESCE(dep.descripcion, '')
		FROM gen_directorio d
		LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto
		WHERE COALESCE(nullif(btrim(d.estado), ''), 'V') = 'V'
		ORDER BY d.apellido, d.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.EmpleadoCRUD
	for rows.Next() {
		var e models.EmpleadoCRUD
		if err := rows.Scan(&e.IDEmpleado, &e.Nombre, &e.Apellido, &e.Email, &e.Depto, &e.DeptoNombre); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *FormularioRepository) TrabajadorCRUDPorID(id int) (models.EmpleadoCRUD, error) {
	var e models.EmpleadoCRUD
	err := r.db.QueryRow(`
		SELECT d.id_empleado, COALESCE(d.nombre, ''), COALESCE(d.apellido, ''), COALESCE(d.email, ''), COALESCE(d.depto, 0), COALESCE(dep.descripcion, '')
		FROM gen_directorio d
		LEFT JOIN gen_departamento dep ON dep.id_depto = d.depto
		WHERE d.id_empleado = $1 AND COALESCE(nullif(btrim(d.estado), ''), 'V') = 'V'`, id).Scan(&e.IDEmpleado, &e.Nombre, &e.Apellido, &e.Email, &e.Depto, &e.DeptoNombre)
	return e, err
}

func (r *FormularioRepository) CrearTrabajador(nombre, apellido, email string, depto int) error {
	_, err := r.db.Exec(`
		INSERT INTO gen_directorio (nombre, apellido, email, depto, estado)
		VALUES ($1, $2, $3, NULLIF($4, 0), 'V')`,
		nombre, apellido, email, depto)
	return err
}

func (r *FormularioRepository) ActualizarTrabajador(id int, nombre, apellido, email string, depto int) error {
	_, err := r.db.Exec(`
		UPDATE gen_directorio
		SET nombre = $2, apellido = $3, email = $4, depto = NULLIF($5, 0)
		WHERE id_empleado = $1 AND COALESCE(nullif(btrim(estado), ''), 'V') = 'V'`,
		id, nombre, apellido, email, depto)
	return err
}

func (r *FormularioRepository) EliminarTrabajador(id int) error {
	_, err := r.db.Exec(`UPDATE gen_directorio SET estado = 'E' WHERE id_empleado = $1`, id)
	return err
}

// ---------- Mantenedor de departamentos ----------

// ErrDepartamentoEnUso indica que el departamento tiene registros asociados.
var ErrDepartamentoEnUso = errors.New("el departamento tiene registros asociados y no se puede eliminar")

func (r *FormularioRepository) DepartamentoPorID(id int) (models.Departamento, error) {
	var d models.Departamento
	err := r.db.QueryRow(`
		SELECT id_depto, descripcion
		FROM gen_departamento
		WHERE id_depto = $1 AND estado = 'V'`, id).Scan(&d.IDDepto, &d.Descripcion)
	return d, err
}

func (r *FormularioRepository) CrearDepartamento(descripcion string) error {
	_, err := r.db.Exec(`
		INSERT INTO gen_departamento (descripcion)
		VALUES ($1)`, descripcion)
	return err
}

func (r *FormularioRepository) ActualizarDepartamento(id int, descripcion string) error {
	res, err := r.db.Exec(`
		UPDATE gen_departamento
		SET descripcion = $2
		WHERE id_depto = $1 AND estado = 'V'`, id, descripcion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *FormularioRepository) EliminarDepartamento(id int) error {
	var n int
	err := r.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM gen_directorio WHERE depto = $1 AND COALESCE(nullif(btrim(estado), ''), 'V') = 'V')
		     + (SELECT COUNT(*) FROM adu_solicitud_materiales WHERE id_depto = $1)
		     + (SELECT COUNT(*) FROM for_jefe_depto WHERE id_depto = $1)`, id).Scan(&n)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrDepartamentoEnUso
	}
	return sinFilas(r.db.Exec(`UPDATE gen_departamento SET estado = 'E' WHERE id_depto = $1 AND estado = 'V'`, id))
}
