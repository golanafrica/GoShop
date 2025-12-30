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
	cfg := config.LoadConfig()

	// Log de la configuration (version safe)
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
	appLogger.Info().Msg("CloseOperation des connexions...")
	db.Close()
	if utils.Rdb != nil {
		utils.Rdb.Close()
	}

	appLogger.Info().Msg("✅ Serveur arrêté proprement")
}
