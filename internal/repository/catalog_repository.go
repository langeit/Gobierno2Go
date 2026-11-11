package repository

import (
	"database/sql"

	"Gobierno2Go/internal/models"
)

type CatalogRepository struct {
	db *sql.DB
}

func NewCatalogRepository(db *sql.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) queryCatalog(query string) ([]models.CatalogItem, error) {
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.CatalogItem{}
	for rows.Next() {
		var it models.CatalogItem
		if err := rows.Scan(&it.Codigo, &it.Descripcion); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *CatalogRepository) Tipos() ([]models.CatalogItem, error) {
	return r.queryCatalog(`SELECT cod_tipo, desc_tipo FROM cor_tipo_docto_corr ORDER BY desc_tipo`)
}

func (r *CatalogRepository) Origenes() ([]models.CatalogItem, error) {
	return r.queryCatalog(`SELECT cod_origen, desc_origen FROM cor_origen_docto ORDER BY desc_origen`)
}

func (r *CatalogRepository) Acciones() ([]models.CatalogItem, error) {
	return r.queryCatalog(`SELECT cod_accion, desc_accion FROM cor_accion_docto ORDER BY desc_accion`)
}

// UsuariosPorEtapa lista los usuarios de un centro, excluyendo al usuario actual.
func (r *CatalogRepository) UsuariosPorEtapa(codEtapa int, excluirID string) ([]models.UserOption, error) {
	query := `SELECT u.id_usuario, COALESCE(u.nombre, u.login)
	          FROM cor_usuario_etapa ue
	          JOIN gen_usuario u ON u.id_usuario = ue.id_usuario
	          WHERE ue.cod_etapa = $1 AND u.id_usuario <> $2 AND u.estado = '1'
	          ORDER BY COALESCE(u.nombre, u.login)`
	rows, err := r.db.Query(query, codEtapa, excluirID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.UserOption{}
	for rows.Next() {
		var u models.UserOption
		if err := rows.Scan(&u.ID, &u.Nombre); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// EtapasDistintas lista las etapas distintas de una, para envíos entre centros.
func (r *CatalogRepository) EtapasDistintas(codEtapa int) ([]models.CatalogItem, error) {
	rows, err := r.db.Query(`SELECT cod_etapa::text, descripcion FROM cor_etapa WHERE cod_etapa <> $1 ORDER BY descripcion`, codEtapa)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.CatalogItem{}
	for rows.Next() {
		var it models.CatalogItem
		if err := rows.Scan(&it.Codigo, &it.Descripcion); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
