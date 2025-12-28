// tests/loadtest/loadtest_test.go
package loadtest_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"Goshop/tests/testutilitis"
)

// skipIfNoK6 arrête le test si k6 n'est pas installé
func skipIfNoK6(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("k6"); err != nil {
		t.Skip("k6 non installé. Installer avec: winget install k6")
	}
}

func TestLoadSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	skipIfNoK6(t)

	// Démarrer le serveur réel
	server := testutilitis.NewTestServer(t)
	serverURL := server.URL

	t.Run("Auth_smoke", func(t *testing.T) {
		cmd := exec.Command("k6", "run",
			"--vus", "1",
			"--duration", "5s",
			"--env", "BASE_URL="+serverURL,
			"scripts/auth_smoke.js")

		// Rediriger les logs pour le CI/CD
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("❌ Auth smoke test échoué: %v", err)
		}
		t.Log("✅ Auth smoke test réussi")
	})
}

func TestLoadAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	skipIfNoK6(t)

	server := testutilitis.NewTestServer(t)
	serverURL := server.URL

	t.Run("Auth_load", func(t *testing.T) {
		// Créer le dossier results si nécessaire
		resultsDir := "results"
		os.MkdirAll(resultsDir, 0755)

		outputFile := filepath.Join(resultsDir, fmt.Sprintf("auth_load_%d.json", time.Now().Unix()))

		cmd := exec.Command("k6", "run",
			"--vus", "10",
			"--duration", "30s",
			"--env", "BASE_URL="+serverURL,
			"--out", "json="+outputFile,
			"scripts/auth_load.js")

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("❌ Auth load test échoué: %v", err)
		}

		// Vérifier que le fichier de résultat existe
		if _, err := os.Stat(outputFile); os.IsNotExist(err) {
			t.Errorf("Fichier de résultat manquant: %s", outputFile)
		} else {
			t.Logf("✅ Résultats sauvegardés: %s", outputFile)
		}
	})
}

func TestK6Scripts(t *testing.T) {
	skipIfNoK6(t)

	scripts := []string{
		"scripts/auth_smoke.js",
		"scripts/auth_load.js",
		"scripts/products_load.js",
	}

	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			if _, err := os.Stat(script); os.IsNotExist(err) {
				t.Fatalf("Script manquant: %s", script)
			}

			// Vérifier la syntaxe avec `k6 archive`
			cmd := exec.Command("k6", "archive", script)
			if err := cmd.Run(); err != nil {
				t.Fatalf("Erreur de syntaxe dans %s: %v", script, err)
			}
			t.Logf("✅ Syntaxe valide: %s", script)
		})
	}
}
