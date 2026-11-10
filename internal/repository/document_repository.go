package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
)

type DocumentRepository struct {
	db *sql.DB
}

func NewDocumentRepository(db *sql.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

const documentSelect = `
SELECT d.barra,
       COALESCE(d.tipo, ''), COALESCE(t.desc_tipo, ''),
       COALESCE(d.origen, ''), COALESCE(o.desc_origen, ''),
       COALESCE(d.accion, ''), COALESCE(a.desc_accion, ''),
       COALESCE(d.referencia, ''),
       COALESCE(TO_CHAR(d.fechcreacion, 'YYYY-MM-DD'), ''),
       COALESCE(d.numero, 0),
       COALESCE(TO_CHAR(d.fecha, 'YYYY-MM-DD'), ''),
       COALESCE(d.grupo, 0)::int,
       COALESCE(d.origen2, ''), COALESCE(d.notas, '')
FROM cor_docto d
LEFT JOIN cor_tipo_docto_corr t ON t.cod_tipo = d.tipo
LEFT JOIN cor_origen_docto o ON o.cod_origen = d.origen
LEFT JOIN cor_accion_docto a ON a.cod_accion = d.accion
`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanDocument(s rowScanner) (models.Document, error) {
	var d models.Document
	err := s.Scan(
		&d.Barra, &d.Tipo, &d.TipoDesc,
		&d.Origen, &d.OrigenDesc,
		&d.Accion, &d.AccionDesc,
		&d.Referencia, &d.FechCreacion, &d.Numero,
		&d.Fecha, &d.Grupo, &d.Origen2, &d.Notas,
	)
	return d, err
}

func today() string {
	return time.Now().Format("2006-01-02")
}

// GetAll lista todos los documentos registrados.
func (r *DocumentRepository) GetAll() ([]models.Document, error) {
	rows, err := r.db.Query(documentSelect + ` ORDER BY d.fechcreacion DESC NULLS LAST, d.barra DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []models.Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// GetByBarra recupera un documento por su código de barra.
func (r *DocumentRepository) GetByBarra(barra string) (*models.Document, error) {
	row := r.db.QueryRow(documentSelect+` WHERE d.barra = $1`, barra)
	d, err := scanDocument(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("documento no encontrado")
		}
		return nil, err
	}
	return &d, nil
}

// nextBarra genera el correlativo con el formato legacy: centro(2) + correlativo(5) + año(2).
func nextBarra(tx *sql.Tx, etapa int) (string, error) {
	prefix := fmt.Sprintf("%02d", etapa/100)
	year := time.Now().Format("06")

	var max int
	err := tx.QueryRow(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(barra FROM 3 FOR 5) AS INTEGER)), 0)
		FROM cor_docto
		WHERE LEFT(barra, 2) = $1 AND RIGHT(barra, 2) = $2 AND barra ~ '^[0-9]{9}$'`,
		prefix, year).Scan(&max)
	if err != nil {
		return "", err
	}

	next := max + 1
	if next > 99999 {
		return "", errors.New("se agotó el correlativo para este centro generador")
	}
	return fmt.Sprintf("%s%05d%s", prefix, next, year), nil
}

// Create registra un documento y su ubicación inicial, devolviendo el código de barra.
func (r *DocumentRepository) Create(in models.CreateDocumentInput, userID string, etapaActual int) (string, error) {
	if etapaActual == 0 {
		return "", errors.New("el usuario no tiene un centro generador asignado")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	barra, err := nextBarra(tx, etapaActual)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(`
		INSERT INTO cor_docto
			(barra, tipo, origen, accion, referencia, fechcreacion, numero, fecha, origen2, notas)
		VALUES
			($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, $6,
			 NULLIF($7, 0), NULLIF($8, '')::date, $9, $10)`,
		barra, in.Tipo, in.Origen, in.Accion, in.Referencia, today(),
		in.Numero, in.Fecha, in.Origen2, in.Notas)
	if err != nil {
		return "", err
	}

	var idDestino interface{}
	var etapaDestino interface{}
	if in.EnviarAUsuario != "" {
		idDestino = in.EnviarAUsuario
	} else if in.EnviarAEtapa > 0 {
		etapaDestino = in.EnviarAEtapa
	}

	_, err = tx.Exec(`
		INSERT INTO cor_doctoetapa
			(barra, id_usuario_actual, cod_etapa_actual, id_usuario_destino, cod_etapa_destino)
		VALUES ($1, $2, $3, $4, $5)`,
		barra, userID, etapaActual, idDestino, etapaDestino)
	if err != nil {
		return "", err
	}

	if _, err = tx.Exec(`
		INSERT INTO cor_historia (barra, id_usuario, movimiento, fecha, actual)
		VALUES ($1, $2, 'C', $3, $2)`, barra, userID, today()); err != nil {
		return "", err
	}

	if idDestino != nil || etapaDestino != nil {
		comodin := in.EnviarAUsuario
		if comodin == "" {
			comodin = strconv.Itoa(in.EnviarAEtapa)
		}
		if _, err = tx.Exec(`
			INSERT INTO cor_historia (barra, id_usuario, movimiento, fecha, actual, comodin)
			VALUES ($1, $2, 'A', $3, $2, $4)`, barra, userID, today(), comodin); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return barra, nil
}

// PendingForUser lista los documentos en tránsito hacia el usuario o su centro.
func (r *DocumentRepository) PendingForUser(userID string, etapa int) ([]models.PendingDocument, error) {
	rows, err := r.db.Query(`
		SELECT d.barra, COALESCE(t.desc_tipo, ''), COALESCE(d.referencia, ''),
		       COALESCE(TO_CHAR(d.fechcreacion, 'YYYY-MM-DD'), ''), COALESCE(o.desc_origen, '')
		FROM cor_doctoetapa de
		JOIN cor_docto d ON d.barra = de.barra
		LEFT JOIN cor_tipo_docto_corr t ON t.cod_tipo = d.tipo
		LEFT JOIN cor_origen_docto o ON o.cod_origen = d.origen
		WHERE de.estado = 'V'
		  AND (de.id_usuario_destino = $1
		       OR (de.cod_etapa_destino = $2 AND de.id_usuario_destino IS NULL))
		  AND NOT EXISTS (SELECT 1 FROM cor_archivado ar WHERE ar.barra = d.barra)
		ORDER BY d.fechcreacion, d.barra`, userID, etapa)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []models.PendingDocument{}
	for rows.Next() {
		var d models.PendingDocument
		if err := rows.Scan(&d.Barra, &d.TipoDesc, &d.Referencia, &d.FechCreacion, &d.Origen); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// Receive confirma la recepción de un documento por parte del usuario.
func (r *DocumentRepository) Receive(barra, userID string, etapa int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		UPDATE cor_doctoetapa SET estado = 'E'
		WHERE estado = 'V' AND barra = $1
		  AND (id_usuario_destino = $2
		       OR (cod_etapa_destino = $3 AND id_usuario_destino IS NULL))`,
		barra, userID, etapa)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("el documento no está pendiente de recepción para usted")
	}

	// Evita duplicar la ubicación actual si el usuario ya lo tenía.
	if _, err = tx.Exec(`
		UPDATE cor_doctoetapa SET estado = 'E'
		WHERE estado = 'V' AND barra = $1 AND id_usuario_destino IS NULL AND cod_etapa_destino IS NULL
		  AND (id_usuario_actual = $2 OR (cod_etapa_actual = $3 AND id_usuario_actual IS NULL))`,
		barra, userID, etapa); err != nil {
		return err
	}

	if _, err = tx.Exec(`
		INSERT INTO cor_doctoetapa (barra, id_usuario_actual, cod_etapa_actual)
		VALUES ($1, $2, $3)`, barra, userID, etapa); err != nil {
		return err
	}

	if _, err = tx.Exec(`
		INSERT INTO cor_historia (barra, id_usuario, movimiento, fecha, actual)
		VALUES ($1, $2, 'R', $3, $2)`, barra, userID, today()); err != nil {
		return err
	}

	return tx.Commit()
}

// HeldByUser lista los documentos que el usuario o su centro tienen en su poder.
func (r *DocumentRepository) HeldByUser(userID string, etapa int) ([]models.HeldDocument, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT d.barra, COALESCE(t.desc_tipo, ''), COALESCE(d.referencia, ''),
		       COALESCE(u.nombre, u.login, '')
		FROM cor_doctoetapa de
		JOIN cor_docto d ON d.barra = de.barra
		LEFT JOIN cor_tipo_docto_corr t ON t.cod_tipo = d.tipo
		LEFT JOIN gen_usuario u ON u.id_usuario = de.id_usuario_actual
		WHERE de.estado = 'V'
		  AND de.id_usuario_destino IS NULL AND de.cod_etapa_destino IS NULL
		  AND (de.id_usuario_actual = $1
		       OR (de.cod_etapa_actual = $2 AND de.id_usuario_actual IS NULL))
		  AND NOT EXISTS (SELECT 1 FROM cor_archivado ar WHERE ar.barra = d.barra)
		ORDER BY d.barra`, userID, etapa)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []models.HeldDocument{}
	for rows.Next() {
		var d models.HeldDocument
		if err := rows.Scan(&d.Barra, &d.TipoDesc, &d.Referencia, &d.Actual); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// Dispatch despacha un documento a uno o varios usuarios y/o centros.
func (r *DocumentRepository) Dispatch(barra, userID string, etapa int, destUsers []string, destEtapas []int) error {
	if len(destUsers) == 0 && len(destEtapas) == 0 {
		return errors.New("debe seleccionar al menos un destino")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		UPDATE cor_doctoetapa SET estado = 'E'
		WHERE estado = 'V' AND barra = $1 AND id_usuario_destino IS NULL AND cod_etapa_destino IS NULL
		  AND (id_usuario_actual = $2 OR (cod_etapa_actual = $3 AND id_usuario_actual IS NULL))`,
		barra, userID, etapa)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("el documento no está en su poder para despacharlo")
	}

	for _, dest := range destUsers {
		if _, err := tx.Exec(`
			INSERT INTO cor_doctoetapa
				(barra, id_usuario_actual, cod_etapa_actual, id_usuario_destino)
			VALUES ($1, $2, $3, $4)`, barra, userID, etapa, dest); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO cor_historia (barra, id_usuario, movimiento, fecha, actual, comodin)
			VALUES ($1, $2, 'A', $3, $2, $4)`, barra, userID, today(), dest); err != nil {
			return err
		}
	}

	for _, dest := range destEtapas {
		if _, err := tx.Exec(`
			INSERT INTO cor_doctoetapa
				(barra, id_usuario_actual, cod_etapa_actual, cod_etapa_destino)
			VALUES ($1, $2, $3, $4)`, barra, userID, etapa, dest); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO cor_historia (barra, id_usuario, movimiento, fecha, actual, comodin)
			VALUES ($1, $2, 'A', $3, $2, $4)`, barra, userID, today(), strconv.Itoa(dest)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Archive archiva un documento que el usuario tiene en su poder.
func (r *DocumentRepository) Archive(barra, userID string, etapa int, carpeta string) error {
	if strings.TrimSpace(carpeta) == "" {
		return errors.New("debe indicar el nombre de la carpeta")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM cor_doctoetapa
		WHERE estado = 'V' AND barra = $1 AND id_usuario_destino IS NULL AND cod_etapa_destino IS NULL
		  AND (id_usuario_actual = $2 OR (cod_etapa_actual = $3 AND id_usuario_actual IS NULL))`,
		barra, userID, etapa).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("el documento no está en su poder para archivarlo")
	}

	var folderID int
	err = tx.QueryRow(`SELECT cod_archivador FROM cor_carpeta_archivo WHERE id_usuario = $1 AND nombre = $2`,
		userID, carpeta).Scan(&folderID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.QueryRow(`INSERT INTO cor_carpeta_archivo (id_usuario, nombre) VALUES ($1, $2) RETURNING cod_archivador`,
			userID, carpeta).Scan(&folderID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	var correlativo int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(correlativo), 0) + 1 FROM cor_archivado WHERE barra = $1`, barra).Scan(&correlativo); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO cor_archivado (barra, correlativo, cod_archivador, cod_etapa_ultima, fecha)
		VALUES ($1, $2, $3, $4, $5)`, barra, correlativo, folderID, etapa, today()); err != nil {
		return err
	}

	if _, err := tx.Exec(`UPDATE cor_doctoetapa SET estado = 'E' WHERE barra = $1 AND estado = 'V'`, barra); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO cor_historia (barra, id_usuario, movimiento, fecha, actual, comodin)
		VALUES ($1, $2, 'F', $3, $2, $4)`, barra, userID, today(), carpeta); err != nil {
		return err
	}

	return tx.Commit()
}

// Search busca documentos por barra, referencia, notas, origen2 o número.
func (r *DocumentRepository) Search(q string) ([]models.Document, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return r.GetAll()
	}

	like := "%" + q + "%"
	rows, err := r.db.Query(documentSelect+`
		WHERE d.barra ILIKE $1
		   OR COALESCE(d.referencia, '') ILIKE $1
		   OR COALESCE(d.notas, '') ILIKE $1
		   OR COALESCE(d.origen2, '') ILIKE $1
		   OR CAST(COALESCE(d.numero, 0) AS TEXT) = $2
		ORDER BY d.fechcreacion DESC NULLS LAST, d.barra DESC
		LIMIT 200`, like, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []models.Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// GetFicha devuelve el documento, su ubicación actual y su historial.
func (r *DocumentRepository) GetFicha(barra string) (*models.Ficha, error) {
	doc, err := r.GetByBarra(barra)
	if err != nil {
		return nil, err
	}

	ficha := &models.Ficha{Document: *doc, Historia: []models.HistoriaItem{}}

	err = r.db.QueryRow(`
		SELECT COALESCE(u.nombre, u.login, ''), COALESCE(e.descripcion, ''),
		       COALESCE(ud.nombre, ud.login, ''), COALESCE(ed.descripcion, '')
		FROM cor_doctoetapa de
		LEFT JOIN gen_usuario u ON u.id_usuario = de.id_usuario_actual
		LEFT JOIN cor_etapa e ON e.cod_etapa = de.cod_etapa_actual
		LEFT JOIN gen_usuario ud ON ud.id_usuario = de.id_usuario_destino
		LEFT JOIN cor_etapa ed ON ed.cod_etapa = de.cod_etapa_destino
		WHERE de.barra = $1 AND de.estado = 'V'
		ORDER BY de.correlativo
		LIMIT 1`, barra).Scan(&ficha.Actual, &ficha.Etapa, &ficha.Destino, &ficha.EtapaDestino)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		ficha.Actual = ""
		ficha.Etapa = ""
		ficha.Destino = ""
		ficha.EtapaDestino = ""
	}

	ficha.Archivado, _ = r.IsArchived(barra)

	rows, err := r.db.Query(`
		SELECT h.correlativo, COALESCE(TO_CHAR(h.fecha, 'YYYY-MM-DD'), ''), h.movimiento,
		       COALESCE(u.nombre, u.login, ''), COALESCE(h.actual, ''), COALESCE(h.comodin, '')
		FROM cor_historia h
		LEFT JOIN gen_usuario u ON u.id_usuario = h.id_usuario
		WHERE h.barra = $1
		ORDER BY h.correlativo`, barra)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var h models.HistoriaItem
		if err := rows.Scan(&h.Correlativo, &h.Fecha, &h.Movimiento, &h.Usuario, &h.Actual, &h.Comodin); err != nil {
			return nil, err
		}
		ficha.Historia = append(ficha.Historia, h)
	}
	return ficha, rows.Err()
}

func (r *DocumentRepository) IsArchived(barra string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM cor_archivado WHERE barra = $1)`, barra).Scan(&exists)
	return exists, err
}

// GetStats calcula las métricas principales del módulo.
func (r *DocumentRepository) GetStats() (models.DocumentStats, error) {
	var s models.DocumentStats
	err := r.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM cor_docto),
			(SELECT COUNT(DISTINCT barra) FROM cor_doctoetapa
			  WHERE estado = 'V' AND (id_usuario_destino IS NOT NULL OR cod_etapa_destino IS NOT NULL)),
			(SELECT COUNT(DISTINCT barra) FROM cor_archivado),
			(SELECT COUNT(DISTINCT barra) FROM cor_historia WHERE movimiento = 'R')`).
		Scan(&s.TotalDocuments, &s.Pendientes, &s.Archivados, &s.Recepcionados)
	return s, err
}

// StatsPorTipo agrupa los documentos por tipo.
func (r *DocumentRepository) StatsPorTipo() ([]models.CountItem, error) {
	return r.countQuery(`
		SELECT COALESCE(t.desc_tipo, 'Sin tipo'), COUNT(*)
		FROM cor_docto d
		LEFT JOIN cor_tipo_docto_corr t ON t.cod_tipo = d.tipo
		GROUP BY t.desc_tipo
		ORDER BY COUNT(*) DESC`)
}

// StatsPorEtapa agrupa los documentos por centro (destino si está en tránsito, si no actual).
func (r *DocumentRepository) StatsPorEtapa() ([]models.CountItem, error) {
	return r.countQuery(`
		SELECT COALESCE(e.descripcion, 'Sin centro'), COUNT(DISTINCT de.barra)
		FROM cor_doctoetapa de
		LEFT JOIN cor_etapa e ON e.cod_etapa = COALESCE(de.cod_etapa_destino, de.cod_etapa_actual)
		WHERE de.estado = 'V'
		GROUP BY e.descripcion
		ORDER BY COUNT(DISTINCT de.barra) DESC`)
}

func (r *DocumentRepository) countQuery(query string) ([]models.CountItem, error) {
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.CountItem{}
	for rows.Next() {
		var it models.CountItem
		if err := rows.Scan(&it.Descripcion, &it.Total); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// Archivados lista los documentos archivados más recientes.
func (r *DocumentRepository) Archivados(limit int) ([]models.ArchivedDocument, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(`
		SELECT ar.barra, COALESCE(t.desc_tipo, ''), COALESCE(d.referencia, ''),
		       COALESCE(TO_CHAR(ar.fecha, 'YYYY-MM-DD'), ''), COALESCE(c.nombre, '')
		FROM cor_archivado ar
		JOIN cor_docto d ON d.barra = ar.barra
		LEFT JOIN cor_tipo_docto_corr t ON t.cod_tipo = d.tipo
		LEFT JOIN cor_carpeta_archivo c ON c.cod_archivador = ar.cod_archivador
		ORDER BY ar.fecha DESC, ar.barra
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []models.ArchivedDocument{}
	for rows.Next() {
		var d models.ArchivedDocument
		if err := rows.Scan(&d.Barra, &d.TipoDesc, &d.Referencia, &d.Fecha, &d.Carpeta); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}
