// interfaces/utils/secure_logging.go
package utils

import (
	"strings"
)

// SecureLogEmail masque partiellement l'email pour les logs
func SecureLogEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "invalid_email"
	}

	localPart := parts[0]
	domain := parts[1]

	// En développement, montre un peu plus
	if len(localPart) > 3 {
		return localPart[:3] + "***@" + domain
	}

	return localPart + "***@" + domain
}

// SecureLogUserID masque partiellement l'ID utilisateur
func SecureLogUserID(userID string) string {
	if len(userID) <= 8 {
		return userID
	}

	// Pour les IDs plus longs, montre juste le début et la fin
	return userID[:4] + "..." + userID[len(userID)-4:]
}

// IsProduction vérifie si on est en environnement production
func IsProduction() bool {
	// À adapter selon ta configuration
	// Par exemple, tu peux vérifier une variable d'environnement
	// return os.Getenv("APP_ENV") == "production"
	return false
}
