package repository

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = bcrypt.DefaultCost

var dummyPasswordHash []byte

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("invalid-login"), bcryptCost)
	if err != nil {
		panic(err)
	}
	dummyPasswordHash = hash
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
