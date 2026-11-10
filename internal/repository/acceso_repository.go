package repository

import (
	"database/sql"
	_ "embed"
	"errors"
	"strings"

	"Gobierno2Go/internal/models"

	"github.com/lib/pq"
)

//go:embed schema/001_perfiles.sql
var perfilesSQL string

var (
	ErrPerfilProtegido = errors.New("el perfil de administrador no se puede eliminar")
	ErrUltimoAdmin     = errors.New("debe quedar al menos un usuario con acceso a administración")
	ErrDuplicado       = errors.New("ya existe un registro con ese nombre o login")
	ErrDatos           = errors.New("los datos enviados no son válidos")
)

type AccesoRepository struct {
	db *sql.DB
}

func NewAccesoRepository(db *sql.DB) *AccesoRepository {
	return &AccesoRepository{db: db}
}

// EnsureSchema crea las tablas de perfiles y el usuario administrador.
func (r *AccesoRepository) EnsureSchema() error {
	for _, stmt := range strings.Split(perfilesSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := r.db.Exec(stmt); err != nil {
			return err
		}
	}
	_, _ = r.db.Exec(`ALTER TABLE gen_directorio ADD COLUMN IF NOT EXISTS estado VARCHAR(1) DEFAULT 'V'`)
	_, _ = r.db.Exec(`UPDATE gen_directorio SET estado = 'V' WHERE estado IS NULL OR btrim(estado) = ''`)
	return nil
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
	return scanPerfil(row.Scan)
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
		SELECT id_usuario, login, COALESCE(nombre, ''), COALESCE(estado, ''),
		       COALESCE(id_empleado, 0)
		FROM gen_usuario
		ORDER BY nombre, login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usuarios []models.UsuarioAcceso
	for rows.Next() {
		var u models.UsuarioAcceso
		if err := rows.Scan(&u.IDUsuario, &u.Login, &u.Nombre, &u.Estado, &u.IDEmpleado); err != nil {
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

func (r *AccesoRepository) CrearPerfil(nombre, descripcion string, modulos []string) (int, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" || len(modulos) == 0 {
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
	if err := reemplazarModulos(tx, id, modulos, false); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *AccesoRepository) ActualizarPerfil(id int, nombre, descripcion string, modulos []string) error {
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
		modulos = asegurar(modulos, "administracion")
	}
	if len(modulos) == 0 {
		return ErrDatos
	}
	if _, err := tx.Exec(`
		UPDATE seg_perfil SET nombre = $2, descripcion = $3 WHERE id_perfil = $1 AND estado = 'V'`,
		id, nombre, strings.TrimSpace(descripcion)); err != nil {
		return traducirError(err)
	}
	if err := reemplazarModulos(tx, id, modulos, protegido); err != nil {
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

func (r *AccesoRepository) AsignarPerfiles(idUsuario string, perfiles []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := reemplazarPerfilesUsuario(tx, idUsuario, perfiles); err != nil {
		return err
	}
	if ok, err := quedaAlgunAdmin(tx); err != nil {
		return err
	} else if !ok {
		return ErrUltimoAdmin
	}
	return tx.Commit()
}

func (r *AccesoRepository) CrearUsuario(login, password, nombre string, perfiles []int) error {
	login = strings.TrimSpace(login)
	nombre = strings.TrimSpace(nombre)
	if login == "" || password == "" || nombre == "" || len(login) > 12 {
		return ErrDatos
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

	if _, err := tx.Exec(`
		INSERT INTO gen_usuario (id_usuario, login, password, estado, nombre, tipo)
		VALUES ($1, $1, $2, '1', $3, '3')`, login, hash, nombre); err != nil {
		return traducirError(err)
	}
	for _, id := range perfiles {
		if id <= 0 {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO seg_usuario_perfil (id_usuario, id_perfil)
			VALUES ($1, $2)`, login, id); err != nil {
			return err
		}
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
			SET nombre = $2, estado = $3, id_empleado = $4, password = $5
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
			return ErrDuplicado
		case "23503":
			return ErrDatos
		}
	}
	return err
}
