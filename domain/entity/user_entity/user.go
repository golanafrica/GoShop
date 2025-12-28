package userentity

import (
	"os"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

type UserEntity struct {
	ID       string
	Email    string
	Password string
}

func getBcryptCost() int {
	if costStr := os.Getenv("BCRYPT_COST"); costStr != "" {
		if cost, err := strconv.Atoi(costStr); err == nil && cost >= 4 && cost <= 12 {
			return cost
		}
	}

	// Défaut selon l'environnement
	env := os.Getenv("APP_ENV")
	switch env {
	case "production":
		return 12
	case "staging":
		return 10
	default: // development, test
		return 4 // ⚡ Ultra rapide en dev
	}
}

// Utilise cette fonction dans ton hash
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), getBcryptCost())
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}
