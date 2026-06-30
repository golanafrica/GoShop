package integration

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"Goshop/infrastructure/postgres"
	"Goshop/tests/testutils"

	"github.com/joho/godotenv"
)

// sharedDB est la connexion DB partagée entre tous les tests
var sharedDB *sql.DB

// TestMain s'exécute UNE SEULE FOIS avant tous les tests
func TestMain(m *testing.M) {
	// Charger .env.test
	if err := godotenv.Load("../../.env.test"); err != nil {
		fmt.Println("⚠️  Warning: .env.test not found")
	}

	// Connexion unique à la DB
	var err error
	sharedDB, err = postgres.Connect(testutils.GetTestDBConnString())
	if err != nil {
		fmt.Printf("❌ Failed to connect to test DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Shared DB connection established")

	// Exécuter tous les tests
	code := m.Run()

	// Cleanup final
	sharedDB.Close()
	fmt.Println("✅ Shared DB connection closed")

	os.Exit(code)
}
