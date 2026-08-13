package middl

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractAPIKey_HeadersSupported(t *testing.T) {
	tests := []struct {
		name          string
		setupRequest  func(*http.Request)
		expectedKey   string
		expectedInsec bool
	}{
		{
			name: "X-API-Key header supporté",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-API-Key", "gsk_live_test123")
			},
			expectedKey:   "gsk_live_test123",
			expectedInsec: false,
		},
		{
			name: "Authorization Bearer supporté",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer gsk_live_test456")
			},
			expectedKey:   "gsk_live_test456",
			expectedInsec: false,
		},
		{
			name: "Query param ?api_key= NON supporté (insecure)",
			setupRequest: func(r *http.Request) {
				r.URL.RawQuery = "api_key=gsk_live_insecure"
			},
			expectedKey:   "",
			expectedInsec: true,
		},
		{
			name: "Aucune clé (manquante)",
			setupRequest: func(r *http.Request) {
				// Aucun header, aucun query
			},
			expectedKey:   "",
			expectedInsec: false,
		},
		{
			name: "X-API-Key prioritaire sur Authorization",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-API-Key", "gsk_live_first")
				r.Header.Set("Authorization", "Bearer gsk_live_second")
			},
			expectedKey:   "gsk_live_first",
			expectedInsec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api/test", nil)
			tt.setupRequest(req)

			key, insecure := extractAPIKey(req)
			if key != tt.expectedKey {
				t.Errorf("extractAPIKey() key = %q, want %q", key, tt.expectedKey)
			}
			if insecure != tt.expectedInsec {
				t.Errorf("extractAPIKey() insecure = %v, want %v", insecure, tt.expectedInsec)
			}
		})
	}
}
