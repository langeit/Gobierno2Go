package repository

import (
	"database/sql"
	_ "embed"
)

//go:embed schema/004_soft_delete.sql
var softDeleteSQL string

// EnsureEstadoVigencia agrega estado 'V' (vigente) o 'E' (eliminado) y deja
// las vistas de stock sin las filas eliminadas.
func EnsureEstadoVigencia(db *sql.DB) error {
	return ejecutarScript(db, softDeleteSQL)
}

func sinFilas(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
