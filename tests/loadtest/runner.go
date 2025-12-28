// tests/loadtest/runner.go
package loadtest

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// LoadTestConfig contient la configuration des tests
type LoadTestConfig struct {
	ScriptPath string            // Chemin vers le script k6
	BaseURL    string            // URL de l'API
	VUs        int               // Nombre d'utilisateurs virtuels
	Duration   time.Duration     // Durée du test
	OutputFile string            // Fichier de sortie
	EnvVars    map[string]string // Variables d'environnement
}

// LoadTestResult contient les résultats du test
type LoadTestResult struct {
	Success      bool
	Duration     time.Duration
	Requests     int
	ErrorRate    float64
	AvgLatency   float64
	P95Latency   float64
	Throughput   float64
	OutputFile   string
	ErrorMessage string
}

// RunLoadTest exécute un test de charge avec k6
func RunLoadTest(config LoadTestConfig) (*LoadTestResult, error) {
	log.Printf("🚀 Démarrage du test de charge: %s", filepath.Base(config.ScriptPath))

	// Construire la commande k6
	args := []string{"run", "--quiet"}

	// Ajouter les variables d'environnement
	for key, value := range config.EnvVars {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	// Ajouter l'URL de base
	if config.BaseURL != "" {
		args = append(args, "--env", fmt.Sprintf("BASE_URL=%s", config.BaseURL))
	}

	// Ajouter les options VUs et durée
	if config.VUs > 0 {
		args = append(args, "--vus", fmt.Sprintf("%d", config.VUs))
	}
	if config.Duration > 0 {
		args = append(args, "--duration", config.Duration.String())
	}

	// Ajouter le fichier de sortie
	if config.OutputFile != "" {
		args = append(args, "--out", fmt.Sprintf("json=%s", config.OutputFile))
	}

	// Ajouter le script
	args = append(args, config.ScriptPath)

	// Exécuter la commande
	cmd := exec.Command("k6", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &LoadTestResult{
		Duration:   duration,
		OutputFile: config.OutputFile,
	}

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	result.Success = true

	// TODO: Parser le fichier de sortie JSON pour extraire les métriques
	// Vous pouvez ajouter un parser JSON ici

	return result, nil
}

// RunSmokeTest exécute un test de validation rapide
func RunSmokeTest(serverURL string) error {
	config := LoadTestConfig{
		ScriptPath: filepath.Join("scripts", "auth_smoke.js"),
		BaseURL:    serverURL,
		VUs:        5,
		Duration:   30 * time.Second,
		OutputFile: filepath.Join("results", fmt.Sprintf("smoke_%d.json", time.Now().Unix())),
		EnvVars: map[string]string{
			"TEST_TYPE": "smoke",
		},
	}

	result, err := RunLoadTest(config)
	if err != nil {
		return fmt.Errorf("smoke test échoué: %v", err)
	}

	log.Printf("✅ Smoke test terminé en %v", result.Duration)
	return nil
}

// RunLoadTestScenario exécute un scénario de test complet
func RunLoadTestScenario(serverURL string, scenario string) (*LoadTestResult, error) {
	var script string
	var vus int
	var duration time.Duration

	switch scenario {
	case "auth":
		script = "auth_load.js"
		vus = 50
		duration = 2 * time.Minute
	case "products":
		script = "products_load.js"
		vus = 100
		duration = 3 * time.Minute
	case "stress":
		script = "stress_test.js"
		vus = 200
		duration = 5 * time.Minute
	default:
		return nil, fmt.Errorf("scénario inconnu: %s", scenario)
	}

	config := LoadTestConfig{
		ScriptPath: filepath.Join("scripts", script),
		BaseURL:    serverURL,
		VUs:        vus,
		Duration:   duration,
		OutputFile: filepath.Join("results", fmt.Sprintf("%s_%d.json", scenario, time.Now().Unix())),
		EnvVars: map[string]string{
			"SCENARIO": scenario,
		},
	}

	return RunLoadTest(config)
}
