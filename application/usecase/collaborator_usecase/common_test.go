package collaboratorusecase_test

import (
	"testing"

	collaboratorusecase "Goshop/application/usecase/collaborator_usecase"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.12 : TESTS UNITAIRES - HELPERS COMMUNS
// ============================================================

// ============================================================
// TESTS : NormalizeToken
// ============================================================

func TestNormalizeToken_Empty(t *testing.T) {
	result := collaboratorusecase.NormalizeToken("")
	assert.Equal(t, "", result)
}

func TestNormalizeToken_WithSpaces(t *testing.T) {
	result := collaboratorusecase.NormalizeToken("  ABCDEF1234567890  ")
	assert.Equal(t, "abcdef1234567890", result)
}

func TestNormalizeToken_WithUppercase(t *testing.T) {
	result := collaboratorusecase.NormalizeToken("ABCDEF1234567890")
	assert.Equal(t, "abcdef1234567890", result)
}

func TestNormalizeToken_AlreadyNormalized(t *testing.T) {
	result := collaboratorusecase.NormalizeToken("abcdef1234567890")
	assert.Equal(t, "abcdef1234567890", result)
}

func TestNormalizeToken_MixedCase(t *testing.T) {
	result := collaboratorusecase.NormalizeToken("AbCdEf1234567890")
	assert.Equal(t, "abcdef1234567890", result)
}

// ============================================================
// TESTS : AdminContext (structure)
// ============================================================

func TestAdminContext_Fields(t *testing.T) {
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@goshop.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		RequestID:  "req-abc",
	}

	assert.Equal(t, "admin-123", admin.AdminID)
	assert.Equal(t, "admin@goshop.com", admin.AdminEmail)
	assert.Equal(t, "super_admin", admin.AdminRole)
	assert.Equal(t, "192.168.1.1", admin.IPAddress)
	assert.Equal(t, "Mozilla/5.0", admin.UserAgent)
	assert.Equal(t, "req-abc", admin.RequestID)
}
