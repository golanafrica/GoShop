package userentity

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGetBcryptCost_FailSafe(t *testing.T) {
	tests := []struct {
		name     string
		appEnv   string
		bcrypt   string
		expected int
	}{
		// ✅ Scénarios normaux
		{"Production explicite", "production", "", 12},
		{"Staging explicite", "staging", "", 10},
		{"Development explicite", "development", "", 4},
		{"Test explicite", "test", "", 4},

		// 🛡️ Scénarios fail-safe (CRITIQUES)
		{"APP_ENV vide (fail-safe)", "", "", 12},
		{"APP_ENV invalide (fail-safe)", "invalid", "", 12},
		{"APP_ENV typo (fail-safe)", "prod", "", 12},

		// 🔧 Overrides BCRYPT_COST
		{"Override BCRYPT_COST valide", "production", "11", 11},
		{"Override BCRYPT_COST max", "production", "14", 14},
		{"Override BCRYPT_COST faible en prod (enforce min 10)", "production", "6", 10},
		{"Override BCRYPT_COST faible en staging (enforce min 10)", "staging", "8", 10},
		{"Override BCRYPT_COST faible en dev (autorisé)", "development", "4", 4},
		{"Override BCRYPT_COST faible en test (autorisé)", "test", "4", 4},

		// ❌ Scénarios d'erreur (fallback)
		{"BCRYPT_COST invalide (ignore)", "production", "invalid", 12},
		{"BCRYPT_COST trop bas (ignore)", "production", "2", 12},
		{"BCRYPT_COST trop haut (ignore)", "production", "20", 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.appEnv != "" {
				os.Setenv("APP_ENV", tt.appEnv)
				defer os.Unsetenv("APP_ENV")
			} else {
				os.Unsetenv("APP_ENV")
			}

			if tt.bcrypt != "" {
				os.Setenv("BCRYPT_COST", tt.bcrypt)
				defer os.Unsetenv("BCRYPT_COST")
			} else {
				os.Unsetenv("BCRYPT_COST")
			}

			// Test
			cost := getBcryptCost()
			if cost != tt.expected {
				t.Errorf("getBcryptCost() = %d, want %d (APP_ENV=%s, BCRYPT_COST=%s)",
					cost, tt.expected, tt.appEnv, tt.bcrypt)
			}
		})
	}
}

// TestHashPassword_UsesBcryptCost vérifie que HashPassword utilise bien le coût configuré
func TestHashPassword_UsesBcryptCost(t *testing.T) {
	// Force dev mode pour rapidité
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	hash, err := HashPassword("TestPassword123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if len(hash) == 0 {
		t.Error("HashPassword() returned empty hash")
	}

	// Vérifier que le hash peut être vérifié
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost() error = %v", err)
	}

	if cost != 4 { // dev/test = 4
		t.Errorf("bcrypt.Cost() = %d, want 4 (dev/test mode)", cost)
	}
}
