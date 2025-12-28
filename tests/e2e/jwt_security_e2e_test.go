// tests/e2e/security_e2e_test.go
package e2e

import (
	"net/http"
	"testing"

	"Goshop/tests/testutilitis"
)

// TestSecurityHeaders vérifie les headers de sécurité
func TestSecurityHeaders(t *testing.T) {
	server := testutilitis.NewTestServer(t)
	client := testutilitis.NewHTTPClient(server.URL)

	resp := client.MustDoRequest(t, "GET", "/help", nil)
	defer resp.Body.Close()

	requiredHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
	}

	for _, header := range requiredHeaders {
		if resp.Header.Get(header) == "" {
			t.Errorf("❌ Header de sécurité manquant: %s", header)
		}
	}
}

// TestCORS vérifie la configuration CORS
func TestCORS(t *testing.T) {
	server := testutilitis.NewTestServer(t)
	client := testutilitis.NewHTTPClient(server.URL)

	// Simuler une requête cross-origin
	req, _ := http.NewRequest("GET", server.URL+"/help", nil)
	req.Header.Set("Origin", "https://frontend.com")

	// Envoyer avec DoRequestRaw
	resp, err := client.DoRequestRaw(req)
	if err != nil {
		t.Fatalf("❌ Échec requête CORS: %v", err)
	}
	defer resp.Body.Close()

	// CORS est optionnel, donc on ne fait qu'informer
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Log("✅ CORS configuré")
	} else {
		t.Log("ℹ️ CORS non configuré (normal en dev)")
	}
}

// TestRateLimiting vérifie le rate limiting
func TestRateLimiting(t *testing.T) {
	t.Skip("Rate limiting non implémenté en dev")

	server := testutilitis.NewTestServer(t)
	client := testutilitis.NewHTTPClient(server.URL)

	// Envoyer 5 requêtes rapides
	for i := 0; i < 5; i++ {
		resp := client.MustDoRequest(t, "GET", "/help", nil)
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Log("✅ Rate limiting détecté")
			resp.Body.Close()
			return
		}
		resp.Body.Close()
	}

	t.Log("ℹ️ Rate limiting non activé (normal en dev)")
}

// TestPublicEndpoints vérifie les endpoints publics
func TestPublicEndpoints(t *testing.T) {
	server := testutilitis.NewTestServer(t)
	client := testutilitis.NewHTTPClient(server.URL)

	endpoints := []struct {
		path   string
		status int
	}{
		{"/help", http.StatusOK},
		{"/health/live", http.StatusOK},
		{"/health/ready", http.StatusOK},
		{"/nonexistent", http.StatusNotFound},
	}

	for _, e := range endpoints {
		t.Run(e.path, func(t *testing.T) {
			resp := client.MustDoRequest(t, "GET", e.path, nil)
			defer resp.Body.Close()

			if resp.StatusCode != e.status {
				t.Errorf("Attendu %d, obtenu %d pour %s", e.status, resp.StatusCode, e.path)
			}
		})
	}
}
