// tests/loadtest/generator.go
package loadtest

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// TestUser représente un utilisateur de test
type TestUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// TestProduct représente un produit de test
type TestProduct struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	Stock      int    `json:"stock"`
	Category   string `json:"category"`
}

// GenerateTestData génère des données de test pour k6
func GenerateTestData(count int, dataDir string) error {
	// Créer le dossier s'il n'existe pas
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Générer des utilisateurs
	users := generateUsers(count)
	usersFile := filepath.Join(dataDir, "users.json")
	if err := writeJSON(usersFile, users); err != nil {
		return fmt.Errorf("failed to write users file: %v", err)
	}

	// Générer des produits
	products := generateProducts(count * 2) // 2x plus de produits que d'utilisateurs
	productsFile := filepath.Join(dataDir, "products.json")
	if err := writeJSON(productsFile, products); err != nil {
		return fmt.Errorf("failed to write products file: %v", err)
	}

	fmt.Printf("✅ Données générées:\n")
	fmt.Printf("   - %d utilisateurs dans %s\n", len(users), usersFile)
	fmt.Printf("   - %d produits dans %s\n", len(products), productsFile)

	return nil
}

func generateUsers(count int) []TestUser {
	users := make([]TestUser, count)
	roles := []string{"user", "user", "user", "admin"} // 75% user, 25% admin

	for i := 0; i < count; i++ {
		role := roles[rand.Intn(len(roles))]
		users[i] = TestUser{
			Email:    fmt.Sprintf("loadtest_user_%d_%d@example.com", time.Now().Unix(), i),
			Password: "Password123!",
			Role:     role,
		}
	}

	return users
}

func generateProducts(count int) []TestProduct {
	products := make([]TestProduct, count)
	categories := []string{"electronics", "clothing", "books", "home", "sports"}
	names := []string{
		"Laptop", "Smartphone", "Tablet", "Headphones", "Monitor",
		"T-Shirt", "Jeans", "Jacket", "Shoes", "Hat",
		"Book", "Notebook", "Pen", "Desk", "Chair",
	}

	for i := 0; i < count; i++ {
		category := categories[rand.Intn(len(categories))]
		name := names[rand.Intn(len(names))]

		products[i] = TestProduct{
			ID:         fmt.Sprintf("prod_%d_%d", time.Now().Unix(), i),
			Name:       fmt.Sprintf("%s %d", name, i+1),
			PriceCents: (rand.Intn(100) + 10) * 1000, // 10,000 à 110,000 cents
			Stock:      rand.Intn(100) + 1,
			Category:   category,
		}
	}

	return products
}

func writeJSON(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(data)
}
