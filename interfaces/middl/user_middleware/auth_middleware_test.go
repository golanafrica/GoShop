package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mw "Goshop/interfaces/middl/user_middleware"
	iutils "Goshop/interfaces/utils" // Pour GetUserID et autres fonctions utilitaires
	mockutils "Goshop/mocks/utils"   // Mock de JWTValidator
)

// ========================================
// Tests avec la nouvelle version configurable
// ========================================

func TestNewAuthMiddleware_ValidToken(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockValidator := mockutils.NewMockJWTValidator(ctrl)

	// Mock claims valides
	expectedUserID := "user-123"
	validClaims := jwt.MapClaims{
		"sub":  expectedUserID,
		"type": "access",
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
	}

	mockValidator.EXPECT().ValidateToken("valid.token.here").Return(validClaims, nil)

	// Handler de test qui vérifie le contexte
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := iutils.GetUserID(r.Context())
		assert.True(t, ok, "UserID should be in context")
		assert.Equal(t, expectedUserID, userID, "UserID should match")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Middleware avec mock - utilisez 'mw' (l'alias)
	middleware := mw.NewAuthMiddleware(mw.AuthMiddlewareConfig{
		JWTValidator: mockValidator,
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid.token.here")
	w := httptest.NewRecorder()

	// Act
	handler := middleware(testHandler)
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestNewAuthMiddleware_InvalidToken(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockValidator := mockutils.NewMockJWTValidator(ctrl)

	// Simuler une erreur de validation
	mockValidator.EXPECT().ValidateToken("invalid.token").Return(nil, assert.AnError)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	middleware := mw.NewAuthMiddleware(mw.AuthMiddlewareConfig{
		JWTValidator: mockValidator,
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")
	w := httptest.NewRecorder()

	// Act
	handler := middleware(testHandler)
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid or corrupted token")
}

func TestNewAuthMiddleware_WrongTokenType(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockValidator := mockutils.NewMockJWTValidator(ctrl)

	// Claims avec mauvais type (refresh au lieu de access)
	invalidClaims := jwt.MapClaims{
		"sub":  "user-123",
		"type": "refresh", // Mauvais type!
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
	}

	mockValidator.EXPECT().ValidateToken("refresh.token").Return(invalidClaims, nil)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	middleware := mw.NewAuthMiddleware(mw.AuthMiddlewareConfig{
		JWTValidator: mockValidator,
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer refresh.token")
	w := httptest.NewRecorder()

	// Act
	handler := middleware(testHandler)
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid token type")
}

func TestNewAuthMiddleware_ExpiredToken(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockValidator := mockutils.NewMockJWTValidator(ctrl)

	// Claims expirés
	expiredClaims := jwt.MapClaims{
		"sub":  "user-123",
		"type": "access",
		"exp":  float64(time.Now().Add(-1 * time.Hour).Unix()), // Expiré
	}

	mockValidator.EXPECT().ValidateToken("expired.token").Return(expiredClaims, nil)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	middleware := mw.NewAuthMiddleware(mw.AuthMiddlewareConfig{
		JWTValidator: mockValidator,
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired.token")
	w := httptest.NewRecorder()

	// Act
	handler := middleware(testHandler)
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "token expired")
}

func TestNewAuthMiddleware_MissingSubject(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockValidator := mockutils.NewMockJWTValidator(ctrl)

	// Claims sans "sub"
	invalidClaims := jwt.MapClaims{
		"type": "access",
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
		// Pas de "sub"!
	}

	mockValidator.EXPECT().ValidateToken("no.subject.token").Return(invalidClaims, nil)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	middleware := mw.NewAuthMiddleware(mw.AuthMiddlewareConfig{
		JWTValidator: mockValidator,
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer no.subject.token")
	w := httptest.NewRecorder()

	// Act
	handler := middleware(testHandler)
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid token subject")
}

// ========================================
// Tests avec l'ancienne version (compatibilité)
// ========================================

func TestAuthMiddleware_NoToken(t *testing.T) {
	// Test de l'ancienne fonction (compatibilité)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called when no token")
	})

	handler := mw.AuthMiddleware(nextHandler) // Utilisez 'mw'

	req := httptest.NewRequest("GET", "/protected", nil)
	// Pas de header Authorization
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing Authorization header")
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	// Test de l'ancienne fonction
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called with invalid format")
	})

	handler := mw.AuthMiddleware(nextHandler) // Utilisez 'mw'

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormat token") // Pas "Bearer "
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid token format")
}

func TestAuthMiddleware_EmptyToken(t *testing.T) {
	// Test de l'ancienne fonction
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called with empty token")
	})

	handler := mw.AuthMiddleware(nextHandler) // Utilisez 'mw'

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer ") // Token vide
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	// ValidateToken sera appelé avec une string vide et échouera
}
