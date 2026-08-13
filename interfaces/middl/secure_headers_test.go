package middl

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestSecureHeaders_HSTS_Production vérifie que HSTS est activé en production
func TestSecureHeaders_HSTS_Production(t *testing.T) {
	// Setup : environnement production
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	// Handler de test
	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/products", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Vérifier HSTS
	hsts := rr.Header().Get("Strict-Transport-Security")
	expectedHSTS := "max-age=63072000; includeSubDomains"

	if hsts != expectedHSTS {
		t.Errorf("HSTS header = %q, want %q", hsts, expectedHSTS)
	}

	// Vérifier max-age = 2 ans (63072000 secondes)
	if !strings.Contains(hsts, "max-age=63072000") {
		t.Errorf("HSTS max-age should be 63072000 (2 years), got %q", hsts)
	}

	// Vérifier includeSubDomains
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Error("HSTS should include includeSubDomains directive")
	}

	// Vérifier que preload n'est PAS inclus (irréversible)
	if strings.Contains(hsts, "preload") {
		t.Error("HSTS should NOT include preload directive (irreversible)")
	}
}

// TestSecureHeaders_HSTS_Staging vérifie que HSTS est activé en staging
func TestSecureHeaders_HSTS_Staging(t *testing.T) {
	os.Setenv("APP_ENV", "staging")
	defer os.Unsetenv("APP_ENV")

	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/products", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("HSTS should be present in staging environment")
	}
}

// TestSecureHeaders_HSTS_Development vérifie que HSTS est DÉSACTIVÉ en dev
func TestSecureHeaders_HSTS_Development(t *testing.T) {
	os.Setenv("APP_ENV", "development")
	defer os.Unsetenv("APP_ENV")

	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/products", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("HSTS should NOT be present in development, got %q", hsts)
	}
}

// TestSecureHeaders_HSTS_NoEnv vérifie que HSTS est DÉSACTIVÉ si APP_ENV non défini
func TestSecureHeaders_HSTS_NoEnv(t *testing.T) {
	os.Unsetenv("APP_ENV")

	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/products", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("HSTS should NOT be present when APP_ENV is not set, got %q", hsts)
	}
}

// TestSecureHeaders_CommonHeaders vérifie les headers communs présents dans tous les environnements
func TestSecureHeaders_CommonHeaders(t *testing.T) {
	os.Setenv("APP_ENV", "development")
	defer os.Unsetenv("APP_ENV")

	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/products", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "0"},
		{"Referrer-Policy", "no-referrer"},
		{"Cache-Control", "no-store"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			value := rr.Header().Get(tt.header)
			if value != tt.expected {
				t.Errorf("%s = %q, want %q", tt.header, value, tt.expected)
			}
		})
	}

	// Vérifier Permissions-Policy (doit contenir restrictions)
	permissions := rr.Header().Get("Permissions-Policy")
	if permissions == "" {
		t.Error("Permissions-Policy header should be set")
	}
	if !strings.Contains(permissions, "camera=()") {
		t.Error("Permissions-Policy should disable camera")
	}
	if !strings.Contains(permissions, "microphone=()") {
		t.Error("Permissions-Policy should disable microphone")
	}
	if !strings.Contains(permissions, "geolocation=()") {
		t.Error("Permissions-Policy should disable geolocation")
	}
}

// TestSecureHeadersSwagger_HSTS vérifie que HSTS est aussi appliqué pour Swagger
func TestSecureHeadersSwagger_HSTS(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	handler := SecureHeadersSwagger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/swagger/index.html", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	expectedHSTS := "max-age=63072000; includeSubDomains"

	if hsts != expectedHSTS {
		t.Errorf("Swagger HSTS = %q, want %q", hsts, expectedHSTS)
	}

	// Vérifier que X-XSS-Protection est aligné avec secure_headers.go
	xssProtection := rr.Header().Get("X-XSS-Protection")
	if xssProtection != "0" {
		t.Errorf("Swagger X-XSS-Protection = %q, want 0 (aligned with secure_headers.go)", xssProtection)
	}
}
