// tests/e2e/auth_e2e_test.go
// tests/e2e/auth_e2e_test.go
package e2e

import (
	"Goshop/tests/testutils"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// maskEmail pour les tests - identique à celle de ton handler
func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	localPart := parts[0]
	domain := parts[1]
	if len(localPart) > 3 {
		return localPart[:3] + "***@" + domain
	}
	return localPart + "***@" + domain
}

func TestAuthFlowE2E(t *testing.T) {
	t.Log("🧪 Test E2E : Flux d'authentification complet")

	// Utiliser le serveur de test UNIFIÉ (cohérent avec main.go)
	server := testutils.NewTestServer(t)
	client := testutils.NewHTTPClient(server.URL)

	// Email unique pour idempotence
	uniqueEmail := fmt.Sprintf("test.auth.%d.%s@example.com", time.Now().UnixNano(), uuid.New().String()[:6])

	// 1. Inscription
	t.Run("Inscription", func(t *testing.T) {
		userData := map[string]interface{}{
			"email":    uniqueEmail,
			"password": "Password123!",
		}

		resp := client.MustDoRequest(t, "POST", "/register", userData)
		defer resp.Body.Close()

		testutils.AssertStatus(t, resp, 201)
		t.Log("✅ Utilisateur enregistré")
	})

	// 2. Connexion
	t.Run("Connexion", func(t *testing.T) {
		loginData := map[string]interface{}{
			"email":    uniqueEmail,
			"password": "Password123!",
		}

		resp := client.MustDoRequest(t, "POST", "/login", loginData)
		defer resp.Body.Close()

		testutils.AssertStatus(t, resp, 200)

		var loginResp struct {
			Token string `json:"token"`
		}
		testutils.ParseJSONBody(t, resp, &loginResp)

		if loginResp.Token == "" {
			t.Fatal("❌ Token JWT vide")
		}
		client.SetToken(loginResp.Token)
		t.Log("✅ Connexion réussie, token JWT obtenu")
	})

	// 3. Route protégée
	t.Run("Profil utilisateur", func(t *testing.T) {
		resp := client.MustDoRequest(t, "GET", "/auth/me", nil)
		defer resp.Body.Close()

		testutils.AssertStatus(t, resp, 200)

		var profile struct {
			Email string `json:"email"`
		}
		testutils.ParseJSONBody(t, resp, &profile)

		// 🔥 CORRECTION : Compare avec l'email MASQUÉ
		expectedMaskedEmail := maskEmail(uniqueEmail)
		if profile.Email != expectedMaskedEmail {
			t.Errorf("❌ Email masqué incorrect. Attendu: %s, obtenu: %s", expectedMaskedEmail, profile.Email)
		}
		t.Log("✅ Accès au profil utilisateur vérifié")
	})
}
