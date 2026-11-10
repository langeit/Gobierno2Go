package repository

import (
	"database/sql"
	"errors"
	"strings"

	"Gobierno2Go/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Login valida credenciales contra gen_usuario. La contraseña se compara con bcrypt.
func (r *UserRepository) Login(login, password string) (*models.User, error) {
	query := `SELECT id_usuario, login, password,
	                 COALESCE(nombre, ''), COALESCE(tipo, '3'), estado
	          FROM gen_usuario
	          WHERE login = $1`
	row := r.db.QueryRow(query, login)

	var user models.User
	var hash string
	err := row.Scan(&user.IDUsuario, &user.Login, &hash, &user.Nombre, &user.Tipo, &user.Estado)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			CheckPassword(string(dummyPasswordHash), password)
			return nil, errors.New("credenciales inválidas")
		}
		return nil, err
	}

	if !CheckPassword(hash, password) {
		return nil, errors.New("credenciales inválidas")
	}

	estado := strings.TrimSpace(user.Estado)
	if estado != "1" && estado != "t" {
		return nil, errors.New("usuario inactivo")
	}
	user.Estado = estado
	user.Password = ""
	return &user, nil
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
