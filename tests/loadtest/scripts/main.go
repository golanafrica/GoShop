// tests/loadtest/scripts/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	// Flags
	generate := flag.Bool("generate", false, "Générer des données de test")
	scenario := flag.String("scenario", "smoke", "Scénario: smoke, auth, products, stress")
	duration := flag.String("duration", "30s", "Durée du test")
	vus := flag.Int("vus", 5, "Nombre d'utilisateurs virtuels")
	url := flag.String("url", "http://localhost:8080", "URL de l'API")

	flag.Parse()

	// Obtenir le répertoire de travail
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Créer le dossier results s'il n'existe pas
	resultsDir := filepath.Join(wd, "..", "results")
	os.MkdirAll(resultsDir, 0755)

	if *generate {
		fmt.Println("✅ Les données sont déjà générées (voir tests/loadtest/data/)")
		return
	}

	// Vérifier que k6 est installé
	if err := exec.Command("k6", "version").Run(); err != nil {
		log.Fatal("❌ k6 n'est pas installé. Exécutez: winget install k6")
	}

	// Déterminer le script k6 à exécuter
	var script string
	switch *scenario {
	case "smoke":
		script = "auth_smoke.js"
	case "auth":
		script = "auth_load.js"
	case "products":
		script = "products_load.js"
	case "stress":
		script = "stress_test.js"
	default:
		log.Fatalf("❌ Scénario inconnu: %s", *scenario)
	}

	scriptPath := filepath.Join(wd, script)

	// Vérifier que le script existe
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Fatalf("❌ Script non trouvé: %s", scriptPath)
	}

	fmt.Printf("🚀 Exécution du test %s\n", *scenario)
	fmt.Printf("📊 Configuration: %d VUs, durée %s\n", *vus, *duration)
	fmt.Printf("🌐 URL: %s\n", *url)
	fmt.Println("=====================================")

	// Construire la commande k6
	args := []string{
		"run",
		"--quiet",
		"--vus", fmt.Sprintf("%d", *vus),
		"--duration", *duration,
		"--env", fmt.Sprintf("BASE_URL=%s", *url),
		scriptPath,
	}

	// Exécuter k6
	cmd := exec.Command("k6", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = wd

	start := time.Now()
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Test échoué: %v", err)
	}

	fmt.Printf("\n✅ Test terminé en %v\n", time.Since(start))
	fmt.Printf("📁 Résultats dans: %s\n", resultsDir)
}
