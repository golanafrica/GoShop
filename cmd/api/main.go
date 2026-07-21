// cmd/api/main.go
package main

// @title GoShop API
// @version 1.0.0
// @description GoShop - E-commerce API with observability and metrics
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email support@goshop.dev
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /
// @schemes http
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"Goshop/config"
	"Goshop/config/setupLogging"
	"Goshop/infrastructure/postgres"
	"Goshop/interfaces/utils"
	"Goshop/internal/app"

	_ "Goshop/docs"

	_ "github.com/lib/pq"
)

func main() {
	// 1. Charger la config de logging
	loggingConfig := setupLogging.GetDefaultConfig()
	appLogger := setupLogging.NewLogger(loggingConfig)

	appLogger.Info().Msg("🚀 Démarrage de GoShop API")

	// 2. Charger la configuration applicative (DB, port, etc.)
	// C'est ici que le fichier .env est lu et injecté dans l'environnement
	cfg := config.LoadConfig()

	// 🚨 SÉCURITÉ CRITIQUE : Validation stricte du secret JWT
	// On vérifie APRÈS le chargement de la config pour s'assurer que le .env est pris en compte
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" || len(jwtSecret) < 32 {
		log.Fatal("🚨 ERREUR FATALE DE SÉCURITÉ : JWT_SECRET est manquant ou trop court (< 32 caractères). Veuillez le définir dans votre fichier .env. Arrêt du serveur.")
	}

	// Initialisation sécurisée du package JWT
	utils.InitJWT(jwtSecret)
	appLogger.Info().Msg("✅ Secret JWT initialisé et validé avec succès")

	// Log de la configuration (version safe, sans mot de passe)
	appLogger.Info().
		Int("app_port", cfg.AppPort).
		Str("db_host", cfg.DBHost).
		Int("db_port", cfg.DBPort).
		Str("db_user", cfg.DBUser).
		Str("db_name", cfg.DBName).
		Bool("has_db_password", cfg.DBPassword != "").
		Msg("Configuration chargée")

	// 3. Connexion à la base de données
	appLogger.Info().Msg("Connexion à la base de données...")
	db, err := postgres.Connect(cfg.GetDBConnString())
	if err != nil {
		appLogger.Fatal().
			Err(err).
			Str("db_host", cfg.DBHost).
			Str("db_name", cfg.DBName).
			Str("db_user", cfg.DBUser).
			Msg("Échec de connexion à la base de données")
	}

	appLogger.Info().Msg("✅ Connexion à la base de données établie")

	// 🆕 v4.4.21 : Initialisation de Redis (pour rate limiting + cache)
	appLogger.Info().Msg("Connexion à Redis...")
	if err := utils.InitRedis(); err != nil {
		appLogger.Warn().
			Err(err).
			Msg("⚠️ Redis non disponible - fallback sur mémoire pour le rate limiting")
	} else {
		appLogger.Info().Msg("✅ Connexion à Redis établie")
	}

	// 4. Créer l'application avec logging
	appLogger.Info().Msg("Initialisation de l'application...")
	appInstance := app.NewApp(db, appLogger)

	// 5. Configurer le serveur
	server := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.AppPort),
		Handler:      appInstance.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 6. Démarrer le serveur dans une goroutine
	go func() {
		appLogger.Info().
			Str("address", server.Addr).
			Str("environment", loggingConfig.Environment).
			Str("log_level", loggingConfig.LogLevel).
			Str("service_name", loggingConfig.ServiceName).
			Str("version", loggingConfig.Version).
			Msg("🚀 Serveur HTTP démarré")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal().Err(err).Msg("❌ Erreur critique du serveur")
		}
	}()

	// 7. Graceful shutdown complet
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info().Msg("👋 Arrêt gracieux du serveur demandé...")

	// Shutdown HTTP propre
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal().Err(err).Msg("💀 Forcé à arrêter immédiatement")
	}

	// Fermer les dépendances dans l'ordre inverse
	appLogger.Info().Msg("Fermeture des connexions...")
	db.Close()
	if utils.Rdb != nil {
		utils.Rdb.Close()
	}

	appLogger.Info().Msg("✅ Serveur arrêté proprement")
}
