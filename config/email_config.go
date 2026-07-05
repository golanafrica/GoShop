package config

import (
	"os"
	"strconv"
)

// ============================================================
// 🆕 v4.3.2 : CONFIGURATION EMAIL (SMTP)
// ============================================================
//
// 🎯 Objectif :
//   Configurer les paramètres SMTP pour l'envoi d'emails
//   (invitations collaborateurs, notifications, etc.)
//
// 📋 Variables d'environnement requises :
//   - SMTP_HOST     : Hôte du serveur SMTP (ex: smtp.gmail.com)
//   - SMTP_PORT     : Port SMTP (ex: 587 pour TLS, 465 pour SSL)
//   - SMTP_USER     : Email de l'expéditeur
//   - SMTP_PASSWORD : Mot de passe ou app password
//   - SMTP_FROM     : Email affiché dans le champ "From"
//   - SMTP_FROM_NAME: Nom affiché dans le champ "From"
//   - SMTP_TLS      : true/false (défaut: true)
//
// 🔐 Sécurité :
//   - Jamais de credentials en dur dans le code
//   - Utilisation de variables d'environnement
//   - Support des app passwords (Google, Outlook)
//
// ============================================================

// EmailConfig représente la configuration SMTP
type EmailConfig struct {
	// Serveur SMTP
	Host     string
	Port     int
	Username string
	Password string

	// Expéditeur
	From     string
	FromName string

	// Sécurité
	UseTLS bool

	// URLs pour les liens dans les emails
	AppURL string

	// Mode debug (affiche les emails dans les logs au lieu d'envoyer)
	DebugMode bool
}

// LoadEmailConfig charge la configuration email depuis les variables d'environnement
func LoadEmailConfig() *EmailConfig {
	config := &EmailConfig{
		Host:      getEnvOrDefault("SMTP_HOST", "smtp.gmail.com"),
		Port:      getEnvAsInt("SMTP_PORT", 587),
		Username:  getEnvOrDefault("SMTP_USER", ""),
		Password:  getEnvOrDefault("SMTP_PASSWORD", ""),
		From:      getEnvOrDefault("SMTP_FROM", "noreply@goshop.com"),
		FromName:  getEnvOrDefault("SMTP_FROM_NAME", "GoShop Team"),
		UseTLS:    getEnvAsBool("SMTP_TLS", true),
		AppURL:    getEnvOrDefault("APP_URL", "http://localhost:3000"),
		DebugMode: getEnvAsBool("EMAIL_DEBUG", false),
	}

	return config
}

// IsValid vérifie si la configuration est valide
func (c *EmailConfig) IsValid() bool {
	if c.Host == "" || c.Port == 0 {
		return false
	}

	// En mode debug, pas besoin de credentials
	if c.DebugMode {
		return true
	}

	// En production, credentials requis
	return c.Username != "" && c.Password != ""
}

// IsConfigured vérifie si l'email est configuré (pas en mode debug)
func (c *EmailConfig) IsConfigured() bool {
	return c.Username != "" && c.Password != ""
}

// ============================================================
// HELPERS
// ============================================================

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
