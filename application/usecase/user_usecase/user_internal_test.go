package userusecase

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.20 : TESTS UNITAIRES - FONCTIONS PRIVÉES
// ============================================================

// ============================================================
// TESTS : maskEmails (loginUsecase.go)
// ============================================================

func TestMaskEmails_Empty(t *testing.T) {
	result := maskEmails("")
	assert.Equal(t, "", result)
}

func TestMaskEmails_InvalidFormat(t *testing.T) {
	result := maskEmails("invalid-email")
	assert.Equal(t, "invalid_email", result)
}

func TestMaskEmails_ShortLocalPart(t *testing.T) {
	result := maskEmails("ab@example.com")
	assert.Equal(t, "ab***@example.com", result)
}

func TestMaskEmails_LongLocalPart(t *testing.T) {
	result := maskEmails("john.doe@example.com")
	assert.Equal(t, "joh***@example.com", result)
}

// ============================================================
// TESTS : maskUsersID (loginUsecase.go)
// ============================================================

func TestMaskUsersID_Short(t *testing.T) {
	result := maskUsersID("12345678")
	assert.Equal(t, "12345678", result)
}

func TestMaskUsersID_Long(t *testing.T) {
	result := maskUsersID("1234567890abcdef")
	assert.Equal(t, "1234...cdef", result)
}

// ============================================================
// TESTS : maskEmail (registerUsecase.go)
// ============================================================

func TestMaskEmail_Long(t *testing.T) {
	result := maskEmail("john.doe@example.com")
	assert.Equal(t, "joh***@example.com", result)
}

func TestMaskEmail_Short(t *testing.T) {
	// len("ab@x.com") = 8 (> 3 && < 100) → email[:3] + "***@" + domain
	result := maskEmail("ab@x.com")
	assert.Equal(t, "ab@***@x.com", result)
}

func TestMaskEmail_VeryLong(t *testing.T) {
	// len("a@example.com") = 13 (> 3 && < 100) → email[:3] + "***@" + domain
	result := maskEmail("a@example.com")
	assert.Equal(t, "a@e***@example.com", result)
}

// ✅ NOUVEAU : Vrai test pour "***@***" (len ≤ 3)
func TestMaskEmail_TooShort(t *testing.T) {
	// len("a@b") = 3 (pas > 3) → "***@***"
	result := maskEmail("a@b")
	assert.Equal(t, "***@***", result)
}

// ✅ NOUVEAU : Vrai test pour "***@***" (len ≥ 100)
func TestMaskEmail_TooLong(t *testing.T) {
	// Email de 100+ caractères
	longEmail := "a" + strings.Repeat("b", 95) + "@example.com"
	result := maskEmail(longEmail)
	assert.Equal(t, "***@***", result)
}

// ============================================================
// TESTS : maskUserID (registerUsecase.go)
// ============================================================

func TestMaskUserID_Short(t *testing.T) {
	result := maskUserID("12345678")
	assert.Equal(t, "12345678", result)
}

func TestMaskUserID_Long(t *testing.T) {
	result := maskUserID("1234567890abcdef")
	assert.Equal(t, "1234...cdef", result)
}

// ============================================================
// TESTS COMPLÉMENTAIRES - EDGE CASES
// ============================================================

func TestMaskEmails_LocalPartExactly3Chars(t *testing.T) {
	// len("abc") = 3 (pas > 3) → "abc***@example.com"
	result := maskEmails("abc@example.com")
	assert.Equal(t, "abc***@example.com", result)
}

func TestMaskUsersID_Exactly9Chars(t *testing.T) {
	// len("123456789") = 9 (> 8) → "1234...6789"
	result := maskUsersID("123456789")
	assert.Equal(t, "1234...6789", result)
}
