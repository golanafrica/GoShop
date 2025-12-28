// refresh_handler_test.go
package refreshhandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	refreshhandler "Goshop/interfaces/handler/refresh_handler"
	"Goshop/interfaces/utils"
)

// Mock simple
type mockRefreshUsecase struct {
	executeFunc func(string) (string, string, error)
}

func (m *mockRefreshUsecase) Execute(ctx context.Context, token string) (string, string, error) {
	return m.executeFunc(token)
}

func TestRefreshHandler_Success_Body(t *testing.T) {
	mockUc := &mockRefreshUsecase{
		executeFunc: func(token string) (string, string, error) {
			if token == "valid-token" {
				return "access-123", "refresh-456", nil
			}
			return "", "", utils.ErrRefreshTokenInvalid
		},
	}

	handler := refreshhandler.NewRefreshHandler(mockUc)

	body := map[string]string{"refresh_token": "valid-token"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h := utils.HttpHandlerFunc(handler.Refresh)
	h.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "access-123", resp["access_token"])
	assert.Equal(t, "refresh-456", resp["refresh_token"])
}

func TestRefreshHandler_Success_Header(t *testing.T) {
	mockUc := &mockRefreshUsecase{
		executeFunc: func(token string) (string, string, error) {
			if token == "header-token" {
				return "access-from-header", "refresh-from-header", nil
			}
			return "", "", utils.ErrRefreshTokenInvalid
		},
	}

	handler := refreshhandler.NewRefreshHandler(mockUc)

	req := httptest.NewRequest("POST", "/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Refresh-Token", "header-token")
	w := httptest.NewRecorder()

	h := utils.HttpHandlerFunc(handler.Refresh)
	h.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "access-from-header", resp["access_token"])
}

func TestRefreshHandler_NoToken(t *testing.T) {
	mockUc := &mockRefreshUsecase{
		executeFunc: func(token string) (string, string, error) {
			t.Error("Execute ne devrait pas Ãªtre appelÃ©")
			return "", "", nil
		},
	}

	handler := refreshhandler.NewRefreshHandler(mockUc)

	req := httptest.NewRequest("POST", "/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h := utils.HttpHandlerFunc(handler.Refresh)
	h.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code) // Bad Request - token manquant
}

func TestRefreshHandler_InvalidToken(t *testing.T) {
	mockUc := &mockRefreshUsecase{
		executeFunc: func(token string) (string, string, error) {
			return "", "", utils.ErrRefreshTokenInvalid
		},
	}

	handler := refreshhandler.NewRefreshHandler(mockUc)

	body := map[string]string{"refresh_token": "invalid"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h := utils.HttpHandlerFunc(handler.Refresh)
	h.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code) // Unauthorized - token invalide
}
