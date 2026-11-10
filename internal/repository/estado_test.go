package repository

import (
	"fmt"
	"os"
	"testing"

	"Gobierno2Go/internal/config"
)

func TestMain(m *testing.M) {
	db, err := config.ConnectDB()
	if err == nil {
		err = EnsureEstadoVigencia(db)
		db.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "soft delete: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}
