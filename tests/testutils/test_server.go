// Package testutils fournit des utilitaires pour les tests E2E
package testutils

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"Goshop/config"
	"Goshop/config/setupLogging"
	"Goshop/infrastructure/postgres"
	"Goshop/internal/app"
)

// 🆕 sync.Once garantit que les migrations ne s'exécutent QU'UNE SEULE FOIS
// même si plusieurs tests s'exécutent en parallèle
var migrationOnce sync.Once

// getProjectRoot retourne le chemin racine du projet
func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

// TestServer encapsule un serveur de test avec accès à la DB
type TestServer struct {
	URL string
	DB  *sql.DB
	Srv *httptest.Server
}

// NewTestServer démarre l'application réelle avec la base de test
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	// 🔥 CHARGEMENT DE .env.test
	projectRoot := getProjectRoot()
	envFile := filepath.Join(projectRoot, ".env.test")

	if err := godotenv.Load(envFile); err != nil {
		t.Logf("⚠️  Fichier .env.test non trouvé, utilisation des variables d'environnement")
	}

	os.Setenv("APP_ENV", "test")
	cfg := config.LoadConfig()

	db, err := postgres.Connect(cfg.GetDBConnString())
	if err != nil {
		t.Fatalf("❌ Connexion à la base de test échouée: %v", err)
	}

	// 🆕 Utiliser sync.Once pour éviter les deadlocks lors des migrations parallèles
	migrationOnce.Do(func() {
		migrationDir := filepath.Join(projectRoot, "migrations")
		if err := RunMigrationsFromDir(db, migrationDir); err != nil {
			t.Fatalf("❌ Échec des migrations: %v", err)
		}
	})

	t.Cleanup(func() {
		if !t.Failed() {
			truncateTables(t, db)
		}
		db.Close()
	})

	logger := setupLogging.NewLogger(setupLogging.Config{
		Environment: cfg.Environment,
		ServiceName: cfg.ServiceName,
		Version:     cfg.AppVersion,
		LogLevel:    "warn",
	})

	appInstance := app.NewApp(db, logger)
	server := httptest.NewServer(appInstance.Handler())
	t.Cleanup(server.Close)

	t.Logf("✅ TestServer démarré sur %s", server.URL)
	return &TestServer{
		URL: server.URL,
		DB:  db,
		Srv: server,
	}
}

// RunMigrationsFromDir applique les migrations depuis un répertoire spécifique
func RunMigrationsFromDir(db *sql.DB, migrationDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("lecture répertoire migrations: %w", err)
	}

	sort.Strings(files)

	for _, file := range files {
		migrationSQL, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("lecture migration: %w", err)
		}

		_, err = db.Exec(string(migrationSQL))
		if err != nil {
			return fmt.Errorf("exécution migration %s: %w", file, err)
		}
	}

	return nil
}

// truncateTables vide toutes les tables (ordre inverse des dépendances)
func truncateTables(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{
		"order_items",
		"orders",
		"products",
		"customers",
		"shop_payment_settings",
		"shops",
		"refresh_sessions",
		"users",
	}
	for _, table := range tables {
		_, err := db.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE")
		if err != nil {
			t.Logf("⚠️ Échec TRUNCATE %s: %v", table, err)
		}
	}
}
