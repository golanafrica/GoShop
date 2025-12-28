// Package config gère la configuration de l'application depuis les variables d'environnement.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config contient toute la configuration de l'application.
type Config struct {
	// Application
	AppPort     int    `mapstructure:"APP_PORT"`
	Environment string `mapstructure:"APP_ENV"` // development, test, production
	LogLevel    string `mapstructure:"LOG_LEVEL"`
	ServiceName string `mapstructure:"SERVICE_NAME"`
	AppVersion  string `mapstructure:"APP_VERSION"`

	// Base de données
	DBHost     string `mapstructure:"DB_HOST"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPort     int    `mapstructure:"DB_PORT"`
	DBPassword string `mapstructure:"DB_PASSWORD"` // sensible
	DBName     string `mapstructure:"DB_NAME"`
	DBSSLMode  string `mapstructure:"DB_SSLMODE"`

	// Redis
	RedisHost string `mapstructure:"REDIS_HOST"`
	RedisPort int    `mapstructure:"REDIS_PORT"`
}

// LoadConfig charge la configuration depuis le bon fichier .env
func LoadConfig() *Config {
	// DÉTECTION SIMPLE : seulement via APP_ENV
	isTest := os.Getenv("APP_ENV") == "test"

	// Charger le bon fichier .env
	envFile := ".env"
	if isTest {
		envFile = ".env.test"
		if err := godotenv.Load(envFile); err != nil {
			log.Printf("⚠️  Fichier %s non trouvé, utilisation des variables d'environnement", envFile)
		}
	} else {
		if err := godotenv.Load(envFile); err != nil {
			log.Printf("⚠️  Fichier %s non trouvé", envFile)
		}
	}

	// Parsing sécurisé avec valeurs par défaut
	port := mustParseInt(getEnv("APP_PORT", "8080"))
	dbPort := mustParseInt(getEnv("DB_PORT", "5432"))
	redisPort := mustParseInt(getEnv("REDIS_PORT", "6379"))

	// Récupère le mot de passe (obligatoire sauf en test)
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" && !isTest {
		log.Fatal("❌ DB_PASSWORD manquant — arrêt immédiat")
	}

	return &Config{
		AppPort:     port,
		Environment: getEnv("APP_ENV", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		ServiceName: getEnv("SERVICE_NAME", "goshop-api"),
		AppVersion:  getEnv("APP_VERSION", "1.0.0"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPort:      dbPort,
		DBPassword:  dbPassword,
		DBName:      getEnv("DB_NAME", "goshop_db"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
		RedisHost:   getEnv("REDIS_HOST", "localhost"),
		RedisPort:   redisPort,
	}
}

// GetDBConnString retourne la chaîne de connexion PostgreSQL.
func (c *Config) GetDBConnString() string {
	return fmt.Sprintf("host=%s user=%s port=%d password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBUser, c.DBPort, c.DBPassword, c.DBName, c.DBSSLMode)
}

// SafeForLogging retourne une version sécurisée de la config (sans mot de passe).
func (c *Config) SafeForLogging() map[string]interface{} {
	return map[string]interface{}{
		"app_port":        c.AppPort,
		"environment":     c.Environment,
		"service_name":    c.ServiceName,
		"version":         c.AppVersion,
		"db_host":         c.DBHost,
		"db_port":         c.DBPort,
		"db_user":         c.DBUser,
		"db_name":         c.DBName,
		"has_db_password": c.DBPassword != "",
		"redis_host":      c.RedisHost,
		"redis_port":      c.RedisPort,
	}
}

// getEnv retourne la valeur de la variable d'environnement ou la valeur par défaut.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// mustParseInt convertit une string en int ou arrête l'application en cas d'erreur.
func mustParseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("❌ Valeur invalide pour port: %s (doit être un entier)", s)
	}
	return i
}
