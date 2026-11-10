package repository

import (
	"strings"
	"testing"

	"Gobierno2Go/internal/config"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("clave-de-prueba")
	if err != nil {
		t.Fatal(err)
	}
	if !esHashBcrypt(hash) {
		t.Fatalf("hash inesperado: %s", hash)
	}
	if !CheckPassword(hash, "clave-de-prueba") {
		t.Fatal("la clave correcta no verificó")
	}
	if CheckPassword(hash, "otra") {
		t.Fatal("una clave distinta verificó")
	}
	if CheckPassword("secret", "secret") {
		t.Fatal("un texto plano no debe aceptarse como hash")
	}
}

func TestMigrarContrasenas(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewUserRepository(db)
	acceso := NewAccesoRepository(db)
	const login = "bcrypttmp"
	const clave = "clave-temporal-1"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM gen_usuario WHERE id_usuario = $1`, login)
	})
	db.Exec(`DELETE FROM gen_usuario WHERE id_usuario = $1`, login)
	if _, err := db.Exec(`
		INSERT INTO gen_usuario (id_usuario, login, password, estado, nombre, tipo)
		VALUES ($1, $1, $2, '1', 'Temporal bcrypt', '3')`, login, clave); err != nil {
		t.Fatal(err)
	}

	if err := repo.MigrarContrasenas(); err != nil {
		t.Fatal(err)
	}

	var enClaro int
	if err := db.QueryRow(`
		SELECT count(*) FROM gen_usuario
		WHERE password NOT LIKE '$2a$%' AND password NOT LIKE '$2b$%' AND password NOT LIKE '$2y%'`).Scan(&enClaro); err != nil {
		t.Fatal(err)
	}
	if enClaro != 0 {
		t.Fatalf("quedaron %d contraseñas en claro", enClaro)
	}

	user, err := repo.Login(login, clave)
	if err != nil {
		t.Fatal(err)
	}
	if user.Login != login || user.Password != "" {
		t.Fatalf("usuario: %+v", user)
	}
	if _, err := repo.Login(login, "incorrecta"); err == nil {
		t.Fatal("una clave incorrecta fue aceptada")
	}

	if _, err := repo.Login("admin", "secret"); err != nil {
		t.Fatal(err)
	}

	otro := "bcryptnuevo"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM gen_usuario WHERE id_usuario = $1`, otro)
	})
	if err := acceso.CrearUsuario(otro, "otra-clave", "Nuevo bcrypt", nil); err != nil {
		t.Fatal(err)
	}
	var guardada string
	if err := db.QueryRow(`SELECT password FROM gen_usuario WHERE id_usuario = $1`, otro).Scan(&guardada); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(guardada, "$2") {
		t.Fatal("el alta guardó la contraseña sin hash")
	}
	if _, err := repo.Login(otro, "otra-clave"); err != nil {
		t.Fatal(err)
	}
}
