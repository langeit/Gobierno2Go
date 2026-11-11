package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Login valida credenciales contra gen_usuario. La contraseña se compara con bcrypt.
// Tras cinco claves incorrectas la cuenta queda bloqueada 15 minutos.
func (r *UserRepository) Login(login, password string) (*models.User, error) {
	query := `SELECT id_usuario, login, password,
	                 COALESCE(nombre, ''), COALESCE(tipo, '3'), estado,
	                 COALESCE(debe_cambiar_password, FALSE), password_expira, bloqueado_hasta
	          FROM gen_usuario
	          WHERE login = $1`
	row := r.db.QueryRow(query, login)

	var user models.User
	var hash string
	var expira, bloqueo sql.NullTime
	err := row.Scan(&user.IDUsuario, &user.Login, &hash, &user.Nombre, &user.Tipo, &user.Estado, &user.DebeCambiar, &expira, &bloqueo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			CheckPassword(string(dummyPasswordHash), password)
			return nil, ErrCredenciales
		}
		return nil, err
	}
	if bloqueo.Valid && bloqueo.Time.After(time.Now()) {
		return nil, ErrUsuarioBloqueado
	}
	if !CheckPassword(hash, password) {
		r.registrarFallo(user.IDUsuario)
		return nil, ErrCredenciales
	}

	estado := strings.TrimSpace(user.Estado)
	if estado != "1" && estado != "t" {
		return nil, ErrUsuarioInactivo
	}
	if _, err := r.db.Exec(`
		UPDATE gen_usuario
		SET intentos_fallidos = 0, bloqueado_hasta = NULL
		WHERE id_usuario = $1`, user.IDUsuario); err != nil {
		return nil, err
	}
	user.Estado = estado
	user.Password = ""
	if expira.Valid && expira.Time.Before(time.Now()) {
		user.DebeCambiar = true
	}
	return &user, nil
}

func (r *UserRepository) registrarFallo(id string) {
	var intentos int
	err := r.db.QueryRow(`
		UPDATE gen_usuario
		SET intentos_fallidos = intentos_fallidos + 1
		WHERE id_usuario = $1
		RETURNING intentos_fallidos`, id).Scan(&intentos)
	if err != nil || intentos < maxIntentosClave {
		return
	}
	hasta := time.Now().Add(bloqueoClave)
	_, _ = r.db.Exec(`
		UPDATE gen_usuario
		SET bloqueado_hasta = $2, intentos_fallidos = 0
		WHERE id_usuario = $1`, id, hasta)
}

// CambiarClave reemplaza la contraseña de la sesión y reinicia su vigencia.
func (r *UserRepository) CambiarClave(id, actual, nueva string) error {
	var hash string
	var bloqueo sql.NullTime
	err := r.db.QueryRow(`
		SELECT password, bloqueado_hasta FROM gen_usuario WHERE id_usuario = $1`, id).Scan(&hash, &bloqueo)
	if err != nil {
		return ErrCredenciales
	}
	if bloqueo.Valid && bloqueo.Time.After(time.Now()) {
		return ErrUsuarioBloqueado
	}
	if !CheckPassword(hash, actual) {
		r.registrarFallo(id)
		return ErrCredenciales
	}
	if actual == nueva {
		return ErrClaveIgual
	}
	if err := ValidarPassword(nueva); err != nil {
		return err
	}
	nuevo, err := HashPassword(nueva)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		UPDATE gen_usuario
		SET password = $2,
		    debe_cambiar_password = FALSE,
		    intentos_fallidos = 0,
		    bloqueado_hasta = NULL,
		    password_cambiada_en = CURRENT_TIMESTAMP,
		    password_expira = CURRENT_TIMESTAMP + INTERVAL '90 days'
		WHERE id_usuario = $1`, id, nuevo)
	return err
}

// MigrarContrasenas reemplaza las contraseñas en claro por su hash bcrypt.
func (r *UserRepository) MigrarContrasenas() error {
	rows, err := r.db.Query(`SELECT id_usuario, password FROM gen_usuario`)
	if err != nil {
		return err
	}
	type pendiente struct {
		id       string
		password string
	}
	var pendientes []pendiente
	for rows.Next() {
		var item pendiente
		if err := rows.Scan(&item.id, &item.password); err != nil {
			rows.Close()
			return err
		}
		if !esHashBcrypt(item.password) {
			pendientes = append(pendientes, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range pendientes {
		hash, err := HashPassword(item.password)
		if err != nil {
			return err
		}
		if _, err := r.db.Exec(`
			UPDATE gen_usuario SET password = $1
			WHERE id_usuario = $2 AND password = $3`, hash, item.id, item.password); err != nil {
			return err
		}
	}
	return nil
}

// GetByID recupera un usuario por su identificador.
func (r *UserRepository) GetByID(id string) (*models.User, error) {
	query := `SELECT id_usuario, login, COALESCE(nombre, ''), COALESCE(tipo, '3'), estado
	          FROM gen_usuario WHERE id_usuario = $1`
	row := r.db.QueryRow(query, id)

	var user models.User
	if err := row.Scan(&user.IDUsuario, &user.Login, &user.Nombre, &user.Tipo, &user.Estado); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetCorrespondenciaInfo devuelve la etapa (centro) principal del usuario.
func (r *UserRepository) GetCorrespondenciaInfo(idUsuario string) (int, string, error) {
	query := `SELECT ue.cod_etapa, e.descripcion
	          FROM cor_usuario_etapa ue
	          JOIN cor_etapa e ON e.cod_etapa = ue.cod_etapa
	          WHERE ue.id_usuario = $1
	          ORDER BY ue.cod_etapa
	          LIMIT 1`
	var etapa int
	var nombre string
	err := r.db.QueryRow(query, idUsuario).Scan(&etapa, &nombre)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "Sin centro asignado", nil
	}
	return etapa, nombre, err
}
