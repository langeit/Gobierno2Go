package repository

import (
	"testing"

	"Gobierno2Go/internal/config"
)

func TestActualizarUsuario(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewAccesoRepository(db)
	users := NewUserRepository(db)
	const login = "editauser"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM gen_usuario WHERE id_usuario = $1`, login)
	})
	db.Exec(`DELETE FROM gen_usuario WHERE id_usuario = $1`, login)
	if err := repo.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	var idEmp int
	if err := db.QueryRow(`
		INSERT INTO gen_directorio (nombre, apellido, email, estado)
		VALUES ('Prueba', 'Acceso', 'acceso-test@example.com', 'V')
		RETURNING id_empleado`).Scan(&idEmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec(`UPDATE gen_usuario SET id_empleado = NULL WHERE id_empleado = $1`, idEmp)
		db.Exec(`DELETE FROM gen_directorio WHERE id_empleado = $1`, idEmp)
	})
	if err := repo.CrearUsuario(login, "clave-inicial", "Nombre inicial", 0, nil); err != nil {
		t.Fatal(err)
	}

	if err := repo.ActualizarUsuario(login, "Nombre editado", "1", "clave-nueva", idEmp, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Login(login, "clave-inicial"); err == nil {
		t.Fatal("la contraseña anterior sigue vigente")
	}
	user, err := users.Login(login, "clave-nueva")
	if err != nil {
		t.Fatal(err)
	}
	if user.Nombre != "Nombre editado" {
		t.Fatalf("nombre = %q", user.Nombre)
	}

	var empleado int
	if err := db.QueryRow(`SELECT id_empleado FROM gen_usuario WHERE id_usuario = $1`, login).Scan(&empleado); err != nil {
		t.Fatal(err)
	}
	if empleado != idEmp {
		t.Fatalf("empleado = %d", empleado)
	}

	if err := repo.ActualizarUsuario(login, "Nombre editado", "0", "", 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Login(login, "clave-nueva"); err == nil {
		t.Fatal("un usuario inactivo pudo entrar")
	}
	if _, err := users.Login(login, "clave-nueva"); err != nil && err.Error() != "usuario inactivo" {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE gen_usuario SET estado = '0' WHERE id_usuario <> 'admin'`); err != nil {
		t.Fatal(err)
	}
	ok, err := quedaAlgunAdminActivo(tx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("el administrador activo dejó de contarse")
	}
	if _, err := tx.Exec(`UPDATE gen_usuario SET estado = '0' WHERE id_usuario = 'admin'`); err != nil {
		t.Fatal(err)
	}
	ok, err = quedaAlgunAdminActivo(tx)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("no quedó ningún administrador activo y la verificación no lo detectó")
	}
}
