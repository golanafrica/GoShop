// Package testutils fournit des utilitaires pour les tests E2E et d'intégration
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
	"time" // 🆕 Ajouté pour le time.Sleep de sécurité

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

// TestServer encapsule un serveur de test avec accès à la DB et à l'App
type TestServer struct {
	URL string
	DB  *sql.DB
	Srv *httptest.Server
	App *app.App // 🆕 Ajouté : pour pouvoir arrêter le scheduler Cron proprement
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

	logger := setupLogging.NewLogger(setupLogging.Config{
		Environment: cfg.Environment,
		ServiceName: cfg.ServiceName,
		Version:     cfg.AppVersion,
		LogLevel:    "warn", // Réduit le bruit dans les logs de test
	})

	appInstance := app.NewApp(db, logger)
	server := httptest.NewServer(appInstance.Handler())

	// 🛡️ CORRECTION CRITIQUE DU TIMEOUT : Nettoyage propre en fin de test
	t.Cleanup(func() {
		// 1. ARRÊTER le scheduler Cron AVANT de tronquer (sinon il garde des verrous DB)
		if appInstance.Scheduler != nil {
			appInstance.Scheduler.Stop()
		}

		// 2. Laisser un court instant (200ms) aux goroutines pour finir et libérer les connexions
		time.Sleep(200 * time.Millisecond)

		// 3. Vider les tables uniquement si le test a réussi
		if !t.Failed() {
			truncateTables(t, db)
		}

		// 4. Fermer proprement la base de données
		db.Close()
	})

	t.Logf("✅ TestServer démarré sur %s", server.URL)

	return &TestServer{
		URL: server.URL,
		DB:  db,
		Srv: server,
		App: appInstance, // 🆕 On garde une référence pour le cleanup
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

	// 🛡️ Astuce : Réduire le pool à 1 connexion force la fermeture des connexions "idle"
	// qui pourraient bloquer le TRUNCATE.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)

	// Petite pause pour laisser le pool se rétracter
	time.Sleep(100 * time.Millisecond)

	tables := []string{
		"order_items",
		"orders",
		"products",
		"customers",
		"shop_payment_settings",
		"shops",
		"refresh_sessions",
		"users",
		// Ajoute ici les nouvelles tables si nécessaire (ex: payments, withdrawals, etc.)
		"payments",
		"withdrawals",
		"escrow_accounts",
		"wallet_transactions",
		"merchant_wallets",
	}

	for _, table := range tables {
		// RESTART IDENTITY CASCADE est crucial pour réinitialiser les séquences (IDs)
		_, err := db.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE")
		if err != nil {
			// On logue mais on ne fait pas panic, car le test est déjà fini
			t.Logf("⚠️ Échec TRUNCATE %s: %v", table, err)
		}
	}

	// Remettre le pool à une valeur normale (au cas où la DB serait réutilisée, bien que Close() suive)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
}
