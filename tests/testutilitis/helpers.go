// tests/testutilitis/helpers.go
// tests/testutilitis/helpers.go
package testutilitis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// HTTPClient est un client HTTP avec helpers pour tests
type HTTPClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

// NewHTTPClient crée un nouveau client HTTP pour tests
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetToken définit le token d'authentification
func (c *HTTPClient) SetToken(token string) {
	c.Token = token
}

// DoRequest envoie une requête HTTP
func (c *HTTPClient) DoRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	// Ajouter le chemin à l'URL de base
	fullURL := c.BaseURL + path

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	// Ajouter un User-Agent pour identification
	req.Header.Set("User-Agent", "GoShop-E2E-Test/1.0")

	return c.Client.Do(req)
}

// MustDoRequest version qui échoue le test en cas d'erreur
func (c *HTTPClient) MustDoRequest(t *testing.T, method, path string, body interface{}) *http.Response {
	t.Helper()

	resp, err := c.DoRequest(method, path, body)
	if err != nil {
		t.Fatalf("❌ Request error %s %s: %v", method, path, err)
	}
	return resp
}

// AssertStatus vérifie le status HTTP attendu
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()

	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset pour relecture

		// Essayer d'extraire un message d'erreur JSON
		var errorResp map[string]interface{}
		if json.Unmarshal(body, &errorResp) == nil {
			if msg, ok := errorResp["message"].(string); ok {
				t.Errorf("❌ Expected status %d, got %d\nMessage: %s\nFull body: %s",
					expected, resp.StatusCode, msg, string(body))
			} else {
				t.Errorf("❌ Expected status %d, got %d\nBody: %s",
					expected, resp.StatusCode, string(body))
			}
		} else {
			t.Errorf("❌ Expected status %d, got %d\nRaw body: %s",
				expected, resp.StatusCode, string(body))
		}
	}
}

// ParseJSONBody parse le body JSON dans la cible
func ParseJSONBody(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("❌ Error reading body: %v", err)
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset pour relecture

	if len(body) == 0 {
		t.Log("⚠️ Empty response body")
		return
	}

	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("❌ JSON unmarshal error: %v\nBody: %s", err, string(body))
	}
}

// ReadBody lit le body et le retourne comme string
func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("❌ Error reading body: %v", err)
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset pour relecture
	return string(body)
}

// ExtractID extrait l'ID d'une réponse JSON
func ExtractID(t *testing.T, resp *http.Response) string {
	t.Helper()

	var result map[string]interface{}
	ParseJSONBody(t, resp, &result)

	id, ok := result["id"].(string)
	if !ok {
		t.Fatalf("❌ 'id' field not found or not string in response: %v", result)
	}
	if id == "" {
		t.Fatalf("❌ Empty 'id' field in response: %v", result)
	}
	return id
}

// DoRequestRaw envoie une requête HTTP personnalisée (utile pour les headers CORS, etc.)
func (c *HTTPClient) DoRequestRaw(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "GoShop-E2E-Test/1.0")
	return c.Client.Do(req)
}
