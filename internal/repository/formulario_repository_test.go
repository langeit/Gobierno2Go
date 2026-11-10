package repository

import (
	"database/sql"
	"errors"
	"testing"

	"Gobierno2Go/internal/config"
)

func TestEmpleadoDeUsuario(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	const login = "pruebaemp"
	limpiar := func() {
		db.Exec(`DELETE FROM gen_usuario WHERE id_usuario = $1`, login)
		db.Exec(`DELETE FROM gen_directorio WHERE nombre = 'Prueba' AND apellido = 'Empleado'`)
	}
	limpiar()
	t.Cleanup(limpiar)

	repo := NewFormularioRepository(db)

	var idEmpleado int
	if err := db.QueryRow(`
		INSERT INTO gen_directorio (nombre, apellido, estado)
		VALUES ('Prueba', 'Empleado', 'V')
		RETURNING id_empleado`).Scan(&idEmpleado); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO gen_usuario (id_usuario, login, password, estado, nombre, tipo, id_empleado)
		VALUES ($1, $1, 'x', '1', 'Prueba', '3', $2)`, login, idEmpleado); err != nil {
		t.Fatal(err)
	}

	emp, err := repo.EmpleadoDeUsuario(login, 2026)
	if err != nil {
		t.Fatalf("el usuario con empleado asignado debe encontrarlo: %v", err)
	}
	if emp.IDEmpleado != idEmpleado {
		t.Fatalf("id_empleado = %d, quiere %d", emp.IDEmpleado, idEmpleado)
	}

	if _, err := repo.EmpleadoDeUsuario("usuario-sin-empleado", 2026); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("un usuario sin empleado debe devolver sql.ErrNoRows, obtuvo %v", err)
	}
}
