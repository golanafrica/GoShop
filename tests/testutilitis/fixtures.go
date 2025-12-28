// tests/testutilitis/fixtures.go
// tests/testutilitis/fixtures.go
package testutilitis

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CustomerFixture retourne des données de test pour un customer
func CustomerFixture() map[string]interface{} {
	uniqueID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String()[:6])
	return map[string]interface{}{
		"first_name": "Jean",
		"last_name":  "Dupont",
		"email":      fmt.Sprintf("jean.dupont.%s@example.com", uniqueID),
	}
}

// CustomerFixtureWithEmail crée un customer avec email spécifique
func CustomerFixtureWithEmail(email string) map[string]interface{} {
	return map[string]interface{}{
		"first_name": "Jean",
		"last_name":  "Dupont",
		"email":      email,
	}
}

// ProductFixture retourne des données de test pour un product
func ProductFixture() map[string]interface{} {
	uniqueID := uuid.New().String()[:8]
	return map[string]interface{}{
		"name":        fmt.Sprintf("Product %s", uniqueID),
		"description": fmt.Sprintf("Test product description %s", uniqueID),
		"price_cents": 2999,
		"stock":       100,
	}
}

// UserFixture retourne des données de test pour un user
func UserFixture() map[string]interface{} {
	uniqueID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String()[:6])
	return map[string]interface{}{
		"email":    fmt.Sprintf("user.%s@example.com", uniqueID),
		"password": "Password123!",
	}
}

// OrderFixture retourne des données de test pour une commande
func OrderFixture(customerID string, productID string) map[string]interface{} {
	return map[string]interface{}{
		"customer_id": customerID,
		"items": []map[string]interface{}{
			{
				"product_id": productID,
				"quantity":   2,
			},
		},
	}
}
