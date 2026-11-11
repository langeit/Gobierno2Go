package repository

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"Gobierno2Go/internal/models"

	"github.com/lib/pq"
)

//go:embed schema/001_perfiles.sql
var perfilesSQL string

//go:embed schema/005_acceso_permisos.sql
var accesoPermisosSQL string

//go:embed schema/006_tablas.sql
var tablasSQL string

var (
	ErrPerfilProtegido = errors.New("el perfil de administrador no se puede eliminar")
	ErrUltimoAdmin     = errors.New("debe quedar al menos un usuario con acceso a administración")
	ErrDuplicado       = errors.New("ya existe un registro con ese nombre o login")
	ErrDatos           = errors.New("los datos enviados no son válidos")
	ErrAmbito          = errors.New("el empleado o el centro de responsabilidad queda fuera del ámbito del perfil")
	ErrFichaOcupada    = errors.New("esa ficha ya tiene una cuenta de usuario")
	ErrFichaRequerida  = errors.New("una cuenta con formularios debe tener un empleado")
	ErrEmpleado        = errors.New("el empleado no existe o no está vigente")
)

type AccesoRepository struct {
	db *sql.DB
}

func NewAccesoRepository(db *sql.DB) *AccesoRepository {
	return &AccesoRepository{db: db}
}

// EnsureSchema crea las tablas de perfiles, permisos, las tablas de Bases y el usuario administrador.
func (r *AccesoRepository) EnsureSchema() error {
	if err := ejecutarScript(r.db, perfilesSQL); err != nil {
		return err
	}
	if err := ejecutarScript(r.db, accesoPermisosSQL); err != nil {
		return err
	}
	if err := ejecutarScript(r.db, tablasSQL); err != nil {
		return err
	}
	_, _ = r.db.Exec(`ALTER TABLE gen_directorio ADD COLUMN IF NOT EXISTS estado VARCHAR(1) DEFAULT 'V'`)
	_, _ = r.db.Exec(`UPDATE gen_directorio SET estado = 'V' WHERE estado IS NULL OR btrim(estado) = ''`)
	if err := r.sincronizarPermisosBases(); err != nil {
		return err
	}
	return r.migrarLegadoMantener()
}

// sincronizarPermisosBases registra el módulo Bases y un permiso por cada
// mantenedor de tablas. El perfil protegido conserva todos esos permisos.
func (r *AccesoRepository) sincronizarPermisosBases() error {
	if _, err := r.db.Exec(`
		INSERT INTO seg_modulo (codigo, nombre, descripcion, orden)
		VALUES ('bases', 'Bases de datos', 'Tablas maestras compartidas por el sistema.', 5)
		ON CONFLICT (codigo) DO UPDATE
		SET nombre = EXCLUDED.nombre,
		    descripcion = EXCLUDED.descripcion,
		    orden = EXCLUDED.orden`); err != nil {
		return err
	}
	if _, err := r.db.Exec(`
		UPDATE seg_modulo SET orden = 6
		WHERE codigo = 'administracion' AND orden = 5`); err != nil {
		return err
	}
	for i, tabla := range models.Tablas() {
		if _, err := r.db.Exec(`
			INSERT INTO seg_permiso (codigo, codigo_modulo, accion, nombre, orden)
			VALUES ($1, 'bases', 'mantener', $2, $3)
			ON CONFLICT (codigo) DO UPDATE
			SET codigo_modulo = EXCLUDED.codigo_modulo,
			    accion = EXCLUDED.accion,
			    nombre = EXCLUDED.nombre,
			    orden = EXCLUDED.orden`,
			models.PermisoTabla(tabla.Codigo), tabla.Nombre, 500+i); err != nil {
			return err
		}
	}
	if _, err := r.db.Exec(`
		INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
		SELECT p.id_perfil, pe.codigo, 'V'
		FROM seg_perfil p
		CROSS JOIN seg_permiso pe
		WHERE p.protegido = TRUE
		  AND p.estado = 'V'
		  AND pe.codigo_modulo = 'bases'
		ON CONFLICT (id_perfil, codigo_permiso) DO UPDATE SET estado = 'V'`); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo, estado)
		SELECT DISTINCT pp.id_perfil, 'bases', 'V'
		FROM seg_perfil_permiso pp
		JOIN seg_permiso pe ON pe.codigo = pp.codigo_permiso
		WHERE pp.estado = 'V' AND pe.codigo_modulo = 'bases'
		ON CONFLICT (id_perfil, codigo_modulo) DO UPDATE SET estado = 'V'`)
	return err
}

// migrarLegadoMantener asigna una sola vez los permisos de mantención que antes
// salían del departamento 17, del departamento 18 o de tipo = 0.
func (r *AccesoRepository) migrarLegadoMantener() error {
	var aplicada bool
	if err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM seg_migracion WHERE codigo = 'legado_mantener')`).Scan(&aplicada); err != nil {
		return err
	}
	if aplicada {
		return nil
	}
	if _, err := r.db.Exec(`
		INSERT INTO seg_usuario_perfil (id_usuario, id_perfil, estado)
		SELECT u.id_usuario, p.id_perfil, 'V'
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		JOIN seg_perfil p ON p.nombre = 'Oficina de Personal' AND p.estado = 'V'
		WHERE COALESCE(d.depto, 0) = 17 OR COALESCE(u.tipo, '3') = '0'
		ON CONFLICT (id_usuario, id_perfil) DO UPDATE SET estado = 'V'`); err != nil {
		return err
	}
	if _, err := r.db.Exec(`
		INSERT INTO seg_usuario_perfil (id_usuario, id_perfil, estado)
		SELECT u.id_usuario, p.id_perfil, 'V'
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		JOIN seg_perfil p ON p.nombre = 'Encargado de inventario' AND p.estado = 'V'
		WHERE COALESCE(d.depto, 0) = 18 OR COALESCE(u.tipo, '3') = '0'
		ON CONFLICT (id_usuario, id_perfil) DO UPDATE SET estado = 'V'`); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO seg_migracion (codigo) VALUES ('legado_mantener') ON CONFLICT DO NOTHING`)
	return err
}

// esAdministradorDeSistema indica si los perfiles del usuario incluyen el módulo
// de administración. Ese módulo autoriza a ver y operar los recursos de los demás.
func esAdministradorDeSistema(db *sql.DB, idUsuario string) (bool, error) {
	var ok bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM seg_usuario_perfil up
			JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
			JOIN seg_perfil_modulo pm ON pm.id_perfil = up.id_perfil AND pm.estado = 'V'
			WHERE up.id_usuario = $1
			  AND up.estado = 'V'
			  AND pm.codigo_modulo = 'administracion'
		)`, idUsuario).Scan(&ok)
	return ok, err
}

// ModulosDeUsuario devuelve los códigos de módulo habilitados para el usuario.
func (r *AccesoRepository) ModulosDeUsuario(idUsuario string) (map[string]bool, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT pm.codigo_modulo
		FROM seg_usuario_perfil up
		JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		JOIN seg_perfil_modulo pm ON pm.id_perfil = up.id_perfil AND pm.estado = 'V'
		WHERE up.id_usuario = $1 AND up.estado = 'V'`, idUsuario)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mods := map[string]bool{}
	for rows.Next() {
		var codigo string
		if err := rows.Scan(&codigo); err != nil {
			return nil, err
		}
		mods[codigo] = true
	}
	return mods, rows.Err()
}

func (r *AccesoRepository) Modulos() ([]models.Modulo, error) {
	rows, err := r.db.Query(`
		SELECT codigo, nombre, descripcion, orden
		FROM seg_modulo
		ORDER BY orden, nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []models.Modulo
	for rows.Next() {
		var m models.Modulo
		if err := rows.Scan(&m.Codigo, &m.Nombre, &m.Descripcion, &m.Orden); err != nil {
			return nil, err
		}
		mods = append(mods, m)
	}
	return mods, rows.Err()
}

func (r *AccesoRepository) Perfiles() ([]models.Perfil, error) {
	rows, err := r.db.Query(`
		SELECT p.id_perfil, p.nombre, p.descripcion, p.protegido,
		       COALESCE(string_agg(m.codigo, ',' ORDER BY m.orden), ''),
		       COALESCE(string_agg(m.nombre, ', ' ORDER BY m.orden), ''),
		       (SELECT COUNT(*) FROM seg_usuario_perfil up WHERE up.id_perfil = p.id_perfil AND up.estado = 'V')
		FROM seg_perfil p
		LEFT JOIN seg_perfil_modulo pm ON pm.id_perfil = p.id_perfil AND pm.estado = 'V'
		LEFT JOIN seg_modulo m ON m.codigo = pm.codigo_modulo
		WHERE p.estado = 'V'
		GROUP BY p.id_perfil
		ORDER BY p.protegido DESC, p.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perfiles []models.Perfil
	for rows.Next() {
		p, err := scanPerfil(rows.Scan)
		if err != nil {
			return nil, err
		}
		perfiles = append(perfiles, p)
	}
	return perfiles, rows.Err()
}

// PerfilesPagina devuelve una página de perfiles vigentes con búsqueda y orden.
func (r *AccesoRepository) PerfilesPagina(busqueda, orden string, ascendente bool, pagina, porPagina int) ([]models.Perfil, int, int, error) {
	filtro := ""
	var args []interface{}
	if busqueda = strings.TrimSpace(busqueda); busqueda != "" {
		args = append(args, "%"+busqueda+"%")
		filtro = ` AND (p.nombre ILIKE $1 OR p.descripcion ILIKE $1 OR EXISTS (
			SELECT 1 FROM seg_perfil_modulo pmb
			JOIN seg_modulo mb ON mb.codigo = pmb.codigo_modulo
			WHERE pmb.id_perfil = p.id_perfil AND pmb.estado = 'V' AND mb.nombre ILIKE $1))`
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM seg_perfil p WHERE p.estado = 'V'`+filtro, args...).Scan(&total); err != nil {
		return nil, 0, 1, err
	}
	pagina, _ = normalizarPagina(total, porPagina, pagina)

	expresion := "p.nombre"
	switch orden {
	case "modulos":
		expresion = `COALESCE(string_agg(m.nombre, ', ' ORDER BY m.orden), '')`
	case "usuarios":
		expresion = `(SELECT COUNT(*) FROM seg_usuario_perfil upc WHERE upc.id_perfil = p.id_perfil AND upc.estado = 'V')`
	}
	direccion := "ASC"
	if !ascendente {
		direccion = "DESC"
	}

	args = append(args, porPagina, (pagina-1)*porPagina)
	query := fmt.Sprintf(`
		SELECT p.id_perfil, p.nombre, p.descripcion, p.protegido,
		       COALESCE(string_agg(m.codigo, ',' ORDER BY m.orden), ''),
		       COALESCE(string_agg(m.nombre, ', ' ORDER BY m.orden), ''),
		       (SELECT COUNT(*) FROM seg_usuario_perfil up WHERE up.id_perfil = p.id_perfil AND up.estado = 'V')
		FROM seg_perfil p
		LEFT JOIN seg_perfil_modulo pm ON pm.id_perfil = p.id_perfil AND pm.estado = 'V'
		LEFT JOIN seg_modulo m ON m.codigo = pm.codigo_modulo
		WHERE p.estado = 'V'%s
		GROUP BY p.id_perfil
		ORDER BY %s %s, p.id_perfil ASC
		LIMIT $%d OFFSET $%d`, filtro, expresion, direccion, len(args)-1, len(args))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, pagina, err
	}
	defer rows.Close()

	var perfiles []models.Perfil
	for rows.Next() {
		p, err := scanPerfil(rows.Scan)
		if err != nil {
			return nil, 0, pagina, err
		}
		perfiles = append(perfiles, p)
	}
	return perfiles, total, pagina, rows.Err()
}

func (r *AccesoRepository) Perfil(id int) (models.Perfil, error) {
	row := r.db.QueryRow(`
		SELECT p.id_perfil, p.nombre, p.descripcion, p.protegido,
		       COALESCE(string_agg(m.codigo, ',' ORDER BY m.orden), ''),
		       COALESCE(string_agg(m.nombre, ', ' ORDER BY m.orden), ''),
		       (SELECT COUNT(*) FROM seg_usuario_perfil up WHERE up.id_perfil = p.id_perfil AND up.estado = 'V')
		FROM seg_perfil p
		LEFT JOIN seg_perfil_modulo pm ON pm.id_perfil = p.id_perfil AND pm.estado = 'V'
		LEFT JOIN seg_modulo m ON m.codigo = pm.codigo_modulo
		WHERE p.id_perfil = $1 AND p.estado = 'V'
		GROUP BY p.id_perfil`, id)
	p, err := scanPerfil(row.Scan)
	if err != nil {
		return p, err
	}
	p.Permisos, err = r.codigosPerfil(id)
	if err != nil {
		return p, err
	}
	p.Deptos, err = r.ambitosPerfil(id, "departamento")
	if err != nil {
		return p, err
	}
	p.Etapas, err = r.ambitosPerfil(id, "etapa")
	return p, err
}

func (r *AccesoRepository) codigosPerfil(id int) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT codigo_permiso FROM seg_perfil_permiso
		WHERE id_perfil = $1 AND estado = 'V'
		ORDER BY codigo_permiso`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var codigos []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		codigos = append(codigos, c)
	}
	return codigos, rows.Err()
}

func (r *AccesoRepository) ambitosPerfil(id int, tipo string) ([]int, error) {
	rows, err := r.db.Query(`
		SELECT referencia FROM seg_perfil_ambito
		WHERE id_perfil = $1 AND tipo = $2 AND estado = 'V'
		ORDER BY referencia`, id, tipo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		ids = append(ids, n)
	}
	return ids, rows.Err()
}

func scanPerfil(scan func(...interface{}) error) (models.Perfil, error) {
	var p models.Perfil
	var codigos, nombres string
	err := scan(&p.ID, &p.Nombre, &p.Descripcion, &p.Protegido, &codigos, &nombres, &p.Usuarios)
	if codigos != "" {
		p.Modulos = strings.Split(codigos, ",")
	}
	p.Nombres = nombres
	return p, err
}

func (r *AccesoRepository) Usuarios() ([]models.UsuarioAcceso, error) {
	rows, err := r.db.Query(`
		SELECT u.id_usuario, u.login, COALESCE(u.nombre, ''), COALESCE(u.estado, ''),
		       COALESCE(u.id_empleado, 0),
		       COALESCE(NULLIF(btrim(COALESCE(d.nombre, '') || ' ' || COALESCE(d.apellido, '')), ''), '')
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		ORDER BY u.nombre, u.login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usuarios []models.UsuarioAcceso
	for rows.Next() {
		var u models.UsuarioAcceso
		if err := rows.Scan(&u.IDUsuario, &u.Login, &u.Nombre, &u.Estado, &u.IDEmpleado, &u.Empleado); err != nil {
			return nil, err
		}
		u.Estado = strings.TrimSpace(u.Estado)
		usuarios = append(usuarios, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	asignaciones, err := r.asignaciones()
	if err != nil {
		return nil, err
	}
	for i := range usuarios {
		if a, ok := asignaciones[usuarios[i].IDUsuario]; ok {
			usuarios[i].PerfilIDs = a.ids
			usuarios[i].Perfiles = a.nombres
		}
	}
	return usuarios, nil
}

// UsuariosPagina devuelve una página de usuarios con búsqueda y orden.
func (r *AccesoRepository) UsuariosPagina(busqueda, orden string, ascendente bool, pagina, porPagina int) ([]models.UsuarioAcceso, int, int, error) {
	filtro := ""
	var args []interface{}
	if busqueda = strings.TrimSpace(busqueda); busqueda != "" {
		args = append(args, "%"+busqueda+"%")
		filtro = ` AND (u.login ILIKE $1 OR COALESCE(u.nombre, '') ILIKE $1
			OR COALESCE(d.nombre, '') ILIKE $1 OR COALESCE(d.apellido, '') ILIKE $1
			OR EXISTS (
				SELECT 1 FROM seg_usuario_perfil upb
				JOIN seg_perfil pb ON pb.id_perfil = upb.id_perfil AND pb.estado = 'V'
				WHERE upb.id_usuario = u.id_usuario AND upb.estado = 'V' AND pb.nombre ILIKE $1))`
	}

	var total int
	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		WHERE TRUE`+filtro, args...).Scan(&total); err != nil {
		return nil, 0, 1, err
	}
	pagina, _ = normalizarPagina(total, porPagina, pagina)

	empleado := `COALESCE(NULLIF(btrim(COALESCE(d.nombre, '') || ' ' || COALESCE(d.apellido, '')), ''), '')`
	perfiles := `COALESCE(string_agg(p.nombre, ', ' ORDER BY p.nombre), '')`
	expresion := "u.nombre"
	switch orden {
	case "login":
		expresion = "u.login"
	case "estado":
		expresion = "u.estado"
	case "empleado":
		expresion = empleado
	case "perfiles":
		expresion = perfiles
	}
	direccion := "ASC"
	if !ascendente {
		direccion = "DESC"
	}

	args = append(args, porPagina, (pagina-1)*porPagina)
	query := fmt.Sprintf(`
		SELECT u.id_usuario, u.login, COALESCE(u.nombre, ''), COALESCE(u.estado, ''),
		       COALESCE(u.id_empleado, 0), %s, %s
		FROM gen_usuario u
		LEFT JOIN gen_directorio d ON d.id_empleado = u.id_empleado
		LEFT JOIN seg_usuario_perfil up ON up.id_usuario = u.id_usuario AND up.estado = 'V'
		LEFT JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		WHERE TRUE%s
		GROUP BY u.id_usuario, d.nombre, d.apellido
		ORDER BY %s %s, u.login ASC
		LIMIT $%d OFFSET $%d`, empleado, perfiles, filtro, expresion, direccion, len(args)-1, len(args))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, pagina, err
	}
	defer rows.Close()

	var usuarios []models.UsuarioAcceso
	for rows.Next() {
		var u models.UsuarioAcceso
		if err := rows.Scan(&u.IDUsuario, &u.Login, &u.Nombre, &u.Estado, &u.IDEmpleado, &u.Empleado, &u.Perfiles); err != nil {
			return nil, 0, pagina, err
		}
		u.Estado = strings.TrimSpace(u.Estado)
		usuarios = append(usuarios, u)
	}
	return usuarios, total, pagina, rows.Err()
}

// Usuario devuelve un usuario con sus perfiles.
func (r *AccesoRepository) Usuario(id string) (models.UsuarioAcceso, error) {
	usuarios, err := r.Usuarios()
	if err != nil {
		return models.UsuarioAcceso{}, err
	}
	for _, u := range usuarios {
		if u.IDUsuario == id {
			return u, nil
		}
	}
	return models.UsuarioAcceso{}, sql.ErrNoRows
}

// EliminarUsuario desactiva la cuenta y conserva su historial de auditoría.
func (r *AccesoRepository) EliminarUsuario(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existe bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM gen_usuario WHERE id_usuario = $1)`, id).Scan(&existe); err != nil {
		return err
	}
	if !existe {
		return ErrDatos
	}
	if _, err := tx.Exec(`UPDATE gen_usuario SET estado = '0' WHERE id_usuario = $1`, id); err != nil {
		return err
	}
	if ok, err := quedaAlgunAdminActivo(tx); err != nil {
		return err
	} else if !ok {
		return ErrUltimoAdmin
	}
	return tx.Commit()
}

type asignacion struct {
	ids     []int
	nombres string
}

func (r *AccesoRepository) asignaciones() (map[string]asignacion, error) {
	rows, err := r.db.Query(`
		SELECT up.id_usuario,
		       COALESCE(array_agg(p.id_perfil ORDER BY p.nombre), '{}'),
		       COALESCE(string_agg(p.nombre, ', ' ORDER BY p.nombre), '')
		FROM seg_usuario_perfil up
		JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		WHERE up.estado = 'V'
		GROUP BY up.id_usuario`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]asignacion{}
	for rows.Next() {
		var id, nombres string
		var ids pq.Int64Array
		if err := rows.Scan(&id, &ids, &nombres); err != nil {
			return nil, err
		}
		a := asignacion{nombres: nombres}
		for _, n := range ids {
			a.ids = append(a.ids, int(n))
		}
		out[id] = a
	}
	return out, rows.Err()
}

func (r *AccesoRepository) CrearPerfil(nombre, descripcion string, permisos []string, deptos, etapas []int) (int, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" || len(permisos) == 0 {
		return 0, ErrDatos
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int
	err = tx.QueryRow(`
		INSERT INTO seg_perfil (nombre, descripcion)
		VALUES ($1, $2)
		RETURNING id_perfil`, nombre, strings.TrimSpace(descripcion)).Scan(&id)
	if err != nil {
		return 0, traducirError(err)
	}
	if err := reemplazarPermisos(tx, id, permisos, false); err != nil {
		return 0, err
	}
	if err := reemplazarAmbitos(tx, id, deptos, etapas); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *AccesoRepository) ActualizarPerfil(id int, nombre, descripcion string, permisos []string, deptos, etapas []int) error {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return ErrDatos
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var protegido bool
	if err := tx.QueryRow(`SELECT protegido FROM seg_perfil WHERE id_perfil = $1 AND estado = 'V'`, id).Scan(&protegido); err != nil {
		return err
	}
	if protegido {
		for _, accion := range []string{"consultar", "ingresar", "aprobar", "mantener"} {
			permisos = asegurar(permisos, "administracion."+accion)
		}
		for _, tabla := range models.Tablas() {
			permisos = asegurar(permisos, models.PermisoTabla(tabla.Codigo))
		}
	}
	if len(permisos) == 0 {
		return ErrDatos
	}
	if _, err := tx.Exec(`
		UPDATE seg_perfil SET nombre = $2, descripcion = $3 WHERE id_perfil = $1 AND estado = 'V'`,
		id, nombre, strings.TrimSpace(descripcion)); err != nil {
		return traducirError(err)
	}
	if err := reemplazarPermisos(tx, id, permisos, protegido); err != nil {
		return err
	}
	if err := reemplazarAmbitos(tx, id, deptos, etapas); err != nil {
		return err
	}
	if ok, err := quedaAlgunAdmin(tx); err != nil {
		return err
	} else if !ok {
		return ErrUltimoAdmin
	}
	return tx.Commit()
}

func (r *AccesoRepository) EliminarPerfil(id int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var protegido bool
	if err := tx.QueryRow(`SELECT protegido FROM seg_perfil WHERE id_perfil = $1 AND estado = 'V'`, id).Scan(&protegido); err != nil {
		return err
	}
	if protegido {
		return ErrPerfilProtegido
	}
	if err := sinFilas(tx.Exec(`UPDATE seg_perfil SET estado = 'E' WHERE id_perfil = $1 AND estado = 'V'`, id)); err != nil {
		return err
	}
	if ok, err := quedaAlgunAdmin(tx); err != nil {
		return err
	} else if !ok {
		return ErrUltimoAdmin
	}
	return tx.Commit()
}

func (r *AccesoRepository) CrearUsuario(login, password, nombre string, idEmpleado int, perfiles []int) error {
	login = strings.TrimSpace(login)
	nombre = strings.TrimSpace(nombre)
	if login == "" || password == "" || nombre == "" || len(login) > 12 {
		return ErrDatos
	}
	if err := ValidarPassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return ErrDatos
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := validarFichaYAmbito(tx, login, idEmpleado, perfiles); err != nil {
		return err
	}
	var emp interface{}
	if idEmpleado > 0 {
		emp = idEmpleado
	}
	if _, err := tx.Exec(`
		INSERT INTO gen_usuario (id_usuario, login, password, estado, nombre, tipo, id_empleado,
		                         debe_cambiar_password, password_cambiada_en, password_expira)
		VALUES ($1, $1, $2, '1', $3, '3', $4, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '90 days')`,
		login, hash, nombre, emp); err != nil {
		return traducirError(err)
	}
	if err := reemplazarPerfilesUsuario(tx, login, perfiles); err != nil {
		return err
	}
	return tx.Commit()
}

// ActualizarUsuario modifica nombre, estado, empleado, perfiles y, si se indica, la contraseña.
// El login no cambia: es el identificador del usuario.
func (r *AccesoRepository) ActualizarUsuario(id, nombre, estado, password string, idEmpleado int, perfiles []int) error {
	nombre = strings.TrimSpace(nombre)
	estado = strings.TrimSpace(estado)
	if id == "" || nombre == "" || len(nombre) > 100 || (estado != "1" && estado != "0") {
		return ErrDatos
	}
	var hash string
	if password != "" {
		if err := ValidarPassword(password); err != nil {
			return err
		}
		var err error
		hash, err = HashPassword(password)
		if err != nil {
			return ErrDatos
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existe bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM gen_usuario WHERE id_usuario = $1)`, id).Scan(&existe); err != nil {
		return err
	}
	if !existe {
		return ErrDatos
	}

	if err := validarFichaYAmbito(tx, id, idEmpleado, perfiles); err != nil {
		return err
	}
	var emp interface{}
	if idEmpleado > 0 {
		emp = idEmpleado
	}
	if hash == "" {
		_, err = tx.Exec(`
			UPDATE gen_usuario
			SET nombre = $2, estado = $3, id_empleado = $4
			WHERE id_usuario = $1`, id, nombre, estado, emp)
	} else {
		_, err = tx.Exec(`
			UPDATE gen_usuario
			SET nombre = $2, estado = $3, id_empleado = $4, password = $5,
			    debe_cambiar_password = TRUE,
			    intentos_fallidos = 0,
			    bloqueado_hasta = NULL,
			    password_cambiada_en = CURRENT_TIMESTAMP,
			    password_expira = CURRENT_TIMESTAMP + INTERVAL '90 days'
			WHERE id_usuario = $1`, id, nombre, estado, emp, hash)
	}
	if err != nil {
		return traducirError(err)
	}

	if err := reemplazarPerfilesUsuario(tx, id, perfiles); err != nil {
		return err
	}
	if ok, err := quedaAlgunAdminActivo(tx); err != nil {
		return err
	} else if !ok {
		return ErrUltimoAdmin
	}
	return tx.Commit()
}

func (r *AccesoRepository) EmpleadosDirectorio() ([]models.Empleado, error) {
	rows, err := r.db.Query(`
		SELECT id_empleado, nombre || ' ' || apellido
		FROM gen_directorio
		WHERE COALESCE(nullif(btrim(estado), ''), 'V') = 'V'
		ORDER BY nombre, apellido`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var empleados []models.Empleado
	for rows.Next() {
		var e models.Empleado
		if err := rows.Scan(&e.IDEmpleado, &e.Nombre); err != nil {
			return nil, err
		}
		empleados = append(empleados, e)
	}
	return empleados, rows.Err()
}

func reemplazarPerfilesUsuario(tx *sql.Tx, idUsuario string, perfiles []int) error {
	if _, err := tx.Exec(`UPDATE seg_usuario_perfil SET estado = 'E' WHERE id_usuario = $1 AND estado = 'V'`, idUsuario); err != nil {
		return err
	}
	for _, id := range perfiles {
		if id <= 0 {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO seg_usuario_perfil (id_usuario, id_perfil, estado)
			VALUES ($1, $2, 'V')
			ON CONFLICT (id_usuario, id_perfil) DO UPDATE SET estado = 'V'`, idUsuario, id); err != nil {
			return err
		}
	}
	return nil
}

func reemplazarModulos(tx *sql.Tx, idPerfil int, modulos []string, mantenerAdmin bool) error {
	if _, err := tx.Exec(`UPDATE seg_perfil_modulo SET estado = 'E' WHERE id_perfil = $1 AND estado = 'V'`, idPerfil); err != nil {
		return err
	}
	vistos := map[string]bool{}
	for _, codigo := range modulos {
		codigo = strings.TrimSpace(codigo)
		if codigo == "" || vistos[codigo] {
			continue
		}
		vistos[codigo] = true
		if _, err := tx.Exec(`
			INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo, estado)
			VALUES ($1, $2, 'V')
			ON CONFLICT (id_perfil, codigo_modulo) DO UPDATE SET estado = 'V'`, idPerfil, codigo); err != nil {
			return traducirError(err)
		}
	}
	if mantenerAdmin && !vistos["administracion"] {
		if _, err := tx.Exec(`
			INSERT INTO seg_perfil_modulo (id_perfil, codigo_modulo, estado)
			VALUES ($1, 'administracion', 'V')
			ON CONFLICT (id_perfil, codigo_modulo) DO UPDATE SET estado = 'V'`, idPerfil); err != nil {
			return err
		}
	}
	return nil
}

func quedaAlgunAdmin(tx *sql.Tx) (bool, error) {
	var n int
	err := tx.QueryRow(`
		SELECT COUNT(DISTINCT up.id_usuario)
		FROM seg_usuario_perfil up
		JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		JOIN seg_perfil_modulo pm ON pm.id_perfil = up.id_perfil AND pm.estado = 'V'
		WHERE up.estado = 'V' AND pm.codigo_modulo = 'administracion'`).Scan(&n)
	return n > 0, err
}

func quedaAlgunAdminActivo(tx *sql.Tx) (bool, error) {
	var n int
	err := tx.QueryRow(`
		SELECT COUNT(DISTINCT up.id_usuario)
		FROM seg_usuario_perfil up
		JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		JOIN seg_perfil_modulo pm ON pm.id_perfil = up.id_perfil AND pm.estado = 'V'
		JOIN gen_usuario u ON u.id_usuario = up.id_usuario
		WHERE up.estado = 'V'
		  AND pm.codigo_modulo = 'administracion'
		  AND btrim(u.estado) IN ('1', 't')`).Scan(&n)
	return n > 0, err
}

func (r *AccesoRepository) PermisosDeUsuario(idUsuario string) (map[string]bool, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT pp.codigo_permiso
		FROM seg_usuario_perfil up
		JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		JOIN seg_perfil_permiso pp ON pp.id_perfil = up.id_perfil AND pp.estado = 'V'
		WHERE up.id_usuario = $1 AND up.estado = 'V'`, idUsuario)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var codigo string
		if err := rows.Scan(&codigo); err != nil {
			return nil, err
		}
		out[codigo] = true
	}
	return out, rows.Err()
}

func (r *AccesoRepository) Permisos() ([]models.Permiso, error) {
	rows, err := r.db.Query(`
		SELECT codigo, codigo_modulo, accion, nombre, orden
		FROM seg_permiso
		ORDER BY orden, codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []models.Permiso
	for rows.Next() {
		var p models.Permiso
		if err := rows.Scan(&p.Codigo, &p.Modulo, &p.Accion, &p.Nombre, &p.Orden); err != nil {
			return nil, err
		}
		lista = append(lista, p)
	}
	return lista, rows.Err()
}

func (r *AccesoRepository) Departamentos() ([]models.OpcionAmbito, error) {
	return r.opciones(`
		SELECT id_depto, descripcion
		FROM gen_departamento
		WHERE COALESCE(nullif(btrim(estado), ''), 'V') = 'V'
		ORDER BY descripcion`)
}

func (r *AccesoRepository) Etapas() ([]models.OpcionAmbito, error) {
	return r.opciones(`SELECT cod_etapa, descripcion FROM cor_etapa ORDER BY descripcion`)
}

func (r *AccesoRepository) opciones(query string) ([]models.OpcionAmbito, error) {
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []models.OpcionAmbito
	for rows.Next() {
		var o models.OpcionAmbito
		if err := rows.Scan(&o.ID, &o.Nombre); err != nil {
			return nil, err
		}
		lista = append(lista, o)
	}
	return lista, rows.Err()
}

func (r *AccesoRepository) PermisosEfectivos() ([]models.PermisoEfectivo, error) {
	rows, err := r.db.Query(`
		SELECT u.login, COALESCE(u.nombre, ''),
		       COALESCE(string_agg(DISTINCT m.nombre || ' · ' || pe.nombre, ', ' ORDER BY m.nombre || ' · ' || pe.nombre), '')
		FROM gen_usuario u
		LEFT JOIN seg_usuario_perfil up ON up.id_usuario = u.id_usuario AND up.estado = 'V'
		LEFT JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
		LEFT JOIN seg_perfil_permiso pp ON pp.id_perfil = p.id_perfil AND pp.estado = 'V'
		LEFT JOIN seg_permiso pe ON pe.codigo = pp.codigo_permiso
		LEFT JOIN seg_modulo m ON m.codigo = pe.codigo_modulo
		WHERE btrim(u.estado) IN ('1', 't')
		GROUP BY u.id_usuario, u.login, u.nombre
		ORDER BY u.nombre, u.login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []models.PermisoEfectivo
	for rows.Next() {
		var item models.PermisoEfectivo
		if err := rows.Scan(&item.Usuario, &item.Nombre, &item.Permisos); err != nil {
			return nil, err
		}
		lista = append(lista, item)
	}
	return lista, rows.Err()
}

func (r *AccesoRepository) QuienPuede() ([]models.PermisoCobertura, error) {
	rows, err := r.db.Query(`
		SELECT pe.codigo, m.nombre, pe.nombre,
		       COALESCE(string_agg(DISTINCT u.nombre, ', ' ORDER BY u.nombre), '')
		FROM seg_permiso pe
		JOIN seg_modulo m ON m.codigo = pe.codigo_modulo
		LEFT JOIN seg_perfil_permiso pp ON pp.codigo_permiso = pe.codigo AND pp.estado = 'V'
		LEFT JOIN seg_perfil p ON p.id_perfil = pp.id_perfil AND p.estado = 'V'
		LEFT JOIN seg_usuario_perfil up ON up.id_perfil = p.id_perfil AND up.estado = 'V'
		LEFT JOIN gen_usuario u ON u.id_usuario = up.id_usuario AND btrim(u.estado) IN ('1', 't')
		GROUP BY pe.codigo, m.nombre, pe.nombre, pe.orden
		ORDER BY pe.orden`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []models.PermisoCobertura
	for rows.Next() {
		var item models.PermisoCobertura
		if err := rows.Scan(&item.Codigo, &item.Modulo, &item.Nombre, &item.Usuarios); err != nil {
			return nil, err
		}
		lista = append(lista, item)
	}
	return lista, rows.Err()
}

func (r *AccesoRepository) AuditoriaReciente() ([]models.EventoAuditoria, error) {
	rows, err := r.db.Query(`
		SELECT fecha, id_usuario, accion, detalle, ip
		FROM seg_auditoria
		ORDER BY id_auditoria DESC
		LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []models.EventoAuditoria
	for rows.Next() {
		var item models.EventoAuditoria
		var fecha time.Time
		if err := rows.Scan(&fecha, &item.Usuario, &item.Accion, &item.Detalle, &item.IP); err != nil {
			return nil, err
		}
		item.Fecha = fecha.Format("2006-01-02 15:04")
		lista = append(lista, item)
	}
	return lista, rows.Err()
}

func (r *AccesoRepository) Registrar(idUsuario, accion, detalle, ip string) error {
	idUsuario = recortar(idUsuario, 12)
	detalle = recortar(detalle, 400)
	ip = recortar(ip, 64)
	_, err := r.db.Exec(`
		INSERT INTO seg_auditoria (id_usuario, accion, detalle, ip)
		VALUES ($1, $2, $3, $4)`, idUsuario, accion, detalle, ip)
	return err
}

func recortar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func tienePermiso(db *sql.DB, idUsuario, codigo string) (bool, error) {
	var ok bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM seg_usuario_perfil up
			JOIN seg_perfil p ON p.id_perfil = up.id_perfil AND p.estado = 'V'
			JOIN seg_perfil_permiso pp ON pp.id_perfil = up.id_perfil AND pp.estado = 'V'
			WHERE up.id_usuario = $1
			  AND up.estado = 'V'
			  AND pp.codigo_permiso = $2
		)`, idUsuario, codigo).Scan(&ok)
	return ok, err
}

func reemplazarPermisos(tx *sql.Tx, idPerfil int, permisos []string, mantenerAdmin bool) error {
	if _, err := tx.Exec(`UPDATE seg_perfil_permiso SET estado = 'E' WHERE id_perfil = $1 AND estado = 'V'`, idPerfil); err != nil {
		return err
	}
	vistos := map[string]bool{}
	var modulos []string
	for _, codigo := range permisos {
		codigo = strings.TrimSpace(codigo)
		if codigo == "" || vistos[codigo] {
			continue
		}
		vistos[codigo] = true
		var modulo string
		if err := tx.QueryRow(`SELECT codigo_modulo FROM seg_permiso WHERE codigo = $1`, codigo).Scan(&modulo); err != nil {
			return ErrDatos
		}
		modulos = append(modulos, modulo)
		if _, err := tx.Exec(`
			INSERT INTO seg_perfil_permiso (id_perfil, codigo_permiso, estado)
			VALUES ($1, $2, 'V')
			ON CONFLICT (id_perfil, codigo_permiso) DO UPDATE SET estado = 'V'`, idPerfil, codigo); err != nil {
			return err
		}
	}
	if mantenerAdmin {
		modulos = asegurar(modulos, "administracion")
	}
	return reemplazarModulos(tx, idPerfil, modulos, mantenerAdmin)
}

func reemplazarAmbitos(tx *sql.Tx, idPerfil int, deptos, etapas []int) error {
	if _, err := tx.Exec(`UPDATE seg_perfil_ambito SET estado = 'E' WHERE id_perfil = $1 AND estado = 'V'`, idPerfil); err != nil {
		return err
	}
	if err := insertarAmbitos(tx, idPerfil, "departamento", deptos); err != nil {
		return err
	}
	return insertarAmbitos(tx, idPerfil, "etapa", etapas)
}

func insertarAmbitos(tx *sql.Tx, idPerfil int, tipo string, refs []int) error {
	vistos := map[int]bool{}
	for _, id := range refs {
		if id <= 0 || vistos[id] {
			continue
		}
		vistos[id] = true
		if _, err := tx.Exec(`
			INSERT INTO seg_perfil_ambito (id_perfil, tipo, referencia, estado)
			VALUES ($1, $2, $3, 'V')
			ON CONFLICT (id_perfil, tipo, referencia) DO UPDATE SET estado = 'V'`, idPerfil, tipo, id); err != nil {
			return err
		}
	}
	return nil
}

func validarFichaYAmbito(tx *sql.Tx, idUsuario string, idEmpleado int, perfiles []int) error {
	depto := 0
	if idEmpleado > 0 {
		err := tx.QueryRow(`
			SELECT COALESCE(depto, 0)
			FROM gen_directorio
			WHERE id_empleado = $1
			  AND COALESCE(nullif(btrim(estado), ''), 'V') = 'V'`, idEmpleado).Scan(&depto)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmpleado
		}
		if err != nil {
			return err
		}
		var ocupada bool
		if err := tx.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM gen_usuario WHERE id_empleado = $1 AND id_usuario <> $2
			)`, idEmpleado, idUsuario).Scan(&ocupada); err != nil {
			return err
		}
		if ocupada {
			return ErrFichaOcupada
		}
	}
	if len(perfiles) == 0 {
		return nil
	}
	var pideFicha bool
	if err := tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM seg_perfil_permiso
			WHERE id_perfil = ANY($1) AND estado = 'V' AND codigo_permiso LIKE 'formularios.%'
		)`, pq.Array(perfiles)).Scan(&pideFicha); err != nil {
		return err
	}
	if pideFicha && idEmpleado == 0 {
		return ErrFichaRequerida
	}
	for _, idPerfil := range perfiles {
		if idPerfil <= 0 {
			continue
		}
		var libre bool
		if err := tx.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM seg_perfil_permiso
				WHERE id_perfil = $1 AND estado = 'V'
				  AND codigo_permiso IN ('formularios.mantener', 'administracion.mantener')
			)`, idPerfil).Scan(&libre); err != nil {
			return err
		}
		if libre {
			continue
		}
		deptos, err := ambitosTx(tx, idPerfil, "departamento")
		if err != nil {
			return err
		}
		if len(deptos) > 0 && (idEmpleado == 0 || !contiene(deptos, depto)) {
			return ErrAmbito
		}
		etapas, err := ambitosTx(tx, idPerfil, "etapa")
		if err != nil {
			return err
		}
		if len(etapas) == 0 {
			continue
		}
		var enCentro bool
		if err := tx.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM cor_usuario_etapa
				WHERE id_usuario = $1 AND cod_etapa = ANY($2)
			)`, idUsuario, pq.Array(etapas)).Scan(&enCentro); err != nil {
			return err
		}
		if !enCentro {
			return ErrAmbito
		}
	}
	return nil
}

func ambitosTx(tx *sql.Tx, idPerfil int, tipo string) ([]int, error) {
	rows, err := tx.Query(`
		SELECT referencia FROM seg_perfil_ambito
		WHERE id_perfil = $1 AND tipo = $2 AND estado = 'V'`, idPerfil, tipo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		ids = append(ids, n)
	}
	return ids, rows.Err()
}

func contiene(lista []int, n int) bool {
	for _, v := range lista {
		if v == n {
			return true
		}
	}
	return false
}

func asegurar(lista []string, item string) []string {
	for _, v := range lista {
		if v == item {
			return lista
		}
	}
	return append(lista, item)
}

func traducirError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23505":
			if pqErr.Constraint == "gen_usuario_empleado_key" {
				return ErrFichaOcupada
			}
			return ErrDuplicado
		case "23503":
			return ErrDatos
		}
	}
	return err
}
