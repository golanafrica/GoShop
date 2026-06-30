package testutils

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"Goshop/domain/repository"
	walletinfra "Goshop/infrastructure/postgres/wallet"
)

// TestContext encapsule le contexte de test pour les tests d'intégration
type TestContext struct {
	T          *testing.T
	DB         *sql.DB
	WalletRepo repository.MerchantWalletRepository // ✅ Interface au lieu de type concret
}

// NewTestContext crée un nouveau contexte de test
func NewTestContext(t *testing.T, db *sql.DB) *TestContext {
	t.Helper()
	return &TestContext{
		T:          t,
		DB:         db,
		WalletRepo: walletinfra.NewMerchantWalletRepositoryInfrastructure(db),
	}
}

// Cleanup nettoie après le test
func (tc *TestContext) Cleanup() {
	if tc.DB != nil {
		// Truncate les tables dans l'ordre inverse des dépendances
		tables := []string{
			"wallet_transactions",
			"account_freezes",
			"merchant_wallets",
			"cod_proofs",
			"credit_installments",
			"credit_contracts",
			"credit_applications",
			"credit_scores",
			"credit_plans",
			"order_items",
			"orders",
			"products",
			"customers",
			"shop_payment_settings",
			"shops",
		}
		for _, table := range tables {
			_, err := tc.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
			if err != nil {
				tc.T.Logf("⚠️  Warning truncating %s: %v", table, err)
			}
		}
		tc.DB.Close()
	}
}

// GetTestDBConnString retourne la chaîne de connexion test
// GetTestDBConnString retourne la chaîne de connexion test
// ✅ Utilise DB_NAME (comme .env.test) au lieu de DB_NAME_TEST
func GetTestDBConnString() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		getEnvOrDefault("DB_HOST", "localhost"),
		getEnvOrDefault("DB_USER", "postgres"),
		getEnvOrDefault("DB_PASSWORD", "root"),    // ✅ root au lieu de postgres
		getEnvOrDefault("DB_NAME", "goshop_test"), // ✅ DB_NAME au lieu de DB_NAME_TEST
		getEnvOrDefault("DB_PORT", "5432"),
	)
}
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
