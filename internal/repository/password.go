package repository

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = bcrypt.DefaultCost

const (
	maxIntentosClave = 5
	bloqueoClave     = 15 * time.Minute
	vigenciaClave    = 90 * 24 * time.Hour
)

var dummyPasswordHash []byte

var (
	ErrPasswordDebil    = errors.New("la contraseña debe tener al menos 6 caracteres")
	ErrCredenciales     = errors.New("credenciales inválidas")
	ErrUsuarioInactivo  = errors.New("usuario inactivo")
	ErrUsuarioBloqueado = errors.New("usuario bloqueado, intente más tarde")
	ErrClaveIgual       = errors.New("la contraseña nueva debe ser distinta de la actual")
)

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("invalid-login"), bcryptCost)
	if err != nil {
		panic(err)
	}
	dummyPasswordHash = hash
}

// ValidarPassword verifica que la contraseña cumpla con la política de seguridad mínima.
func ValidarPassword(password string) error {
	if len(password) < 6 {
		return ErrPasswordDebil
	}
	return nil
}

// HashPassword devuelve el hash bcrypt de una contraseña en claro.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compara una contraseña en claro con un hash bcrypt.
func CheckPassword(hash, plain string) bool {
	if !esHashBcrypt(hash) {
		bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(plain))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func esHashBcrypt(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$")
}
