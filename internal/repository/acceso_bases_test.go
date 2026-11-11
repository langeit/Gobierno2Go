package repository

import (
	"testing"

	"Gobierno2Go/internal/config"
	"Gobierno2Go/internal/models"
)

func TestPermisosBasesEnPerfiles(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewAccesoRepository(db)
	if err := repo.EnsureSchema(); err != nil {
		t.Fatal(err)
	}

	modulos, err := repo.ModulosDeUsuario("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !modulos[models.ModuloBases] {
		t.Fatal("el perfil administrador no tiene el módulo bases")
	}

	permisos, err := repo.PermisosDeUsuario("admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tabla := range models.Tablas() {
		if !permisos[models.PermisoTabla(tabla.Codigo)] {
			t.Errorf("el perfil administrador no tiene el permiso %s", models.PermisoTabla(tabla.Codigo))
		}
	}
}
