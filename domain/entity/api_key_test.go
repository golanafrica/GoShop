package entity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.3 : TESTS UNITAIRES - ENTITÉ API KEY
// ============================================================

// ============================================================
// TESTS : AllScopes() et IsValidScope()
// ============================================================

func TestAllScopes_Count(t *testing.T) {
	scopes := AllScopes()
	assert.Equal(t, 18, len(scopes), "Should have 18 scopes")
}

func TestAllScopes_ContainsAdmin(t *testing.T) {
	scopes := AllScopes()
	assert.Contains(t, scopes, ScopeAdmin, "Should contain admin scope")
}

func TestIsValidScope_Valid(t *testing.T) {
	validScopes := []string{
		"read:shops", "write:shops",
		"read:orders", "write:orders",
		"read:products", "write:products",
		"read:customers", "write:customers",
		"read:payments", "write:payments",
		"read:wallet", "write:wallet",
		"read:tontine", "write:tontine",
		"read:credit", "write:credit",
		"read:reports",
		"admin",
	}

	for _, scope := range validScopes {
		assert.True(t, IsValidScope(scope), "Scope %s should be valid", scope)
	}
}

func TestIsValidScope_Invalid(t *testing.T) {
	invalidScopes := []string{
		"",
		"invalid",
		"read:invalid",
		"admin:all",
		"READ:SHOPS", // Case sensitive
		"read:shops ",
	}

	for _, scope := range invalidScopes {
		assert.False(t, IsValidScope(scope), "Scope %q should be invalid", scope)
	}
}

// ============================================================
// TESTS : GenerateAPIKey()
// ============================================================

func TestGenerateAPIKey_LivePrefix(t *testing.T) {
	fullKey, prefix, hash, err := GenerateAPIKey(false)

	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(fullKey, APIKeyPrefixLive), "Should have live prefix")
	assert.True(t, strings.HasPrefix(prefix, APIKeyPrefixLive), "Prefix should be live")
	assert.NotEmpty(t, hash, "Hash should not be empty")
	assert.Equal(t, 64, len(hash), "Hash should be 64 hex characters (SHA-256)")
}

func TestGenerateAPIKey_TestPrefix(t *testing.T) {
	fullKey, prefix, _, err := GenerateAPIKey(true)

	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(fullKey, APIKeyPrefixTest), "Should have test prefix")
	assert.True(t, strings.HasPrefix(prefix, APIKeyPrefixTest), "Prefix should be test")
}

func TestGenerateAPIKey_Length(t *testing.T) {
	fullKey, _, _, err := GenerateAPIKey(false)

	assert.NoError(t, err)
	expectedLength := len(APIKeyPrefixLive) + APIKeyRandomLength
	assert.Equal(t, expectedLength, len(fullKey),
		"Full key should be prefix (%d) + random (%d) = %d characters",
		len(APIKeyPrefixLive), APIKeyRandomLength, expectedLength)
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, _, _, err := GenerateAPIKey(false)
		assert.NoError(t, err)
		assert.False(t, keys[key], "Key should be unique (iteration %d)", i)
		keys[key] = true
	}
}

func TestGenerateAPIKey_RandomPartHex(t *testing.T) {
	fullKey, _, _, err := GenerateAPIKey(false)
	assert.NoError(t, err)

	// Extraire la partie aléatoire (après le préfixe)
	randomPart := fullKey[len(APIKeyPrefixLive):]

	// Vérifier que c'est de l'hexadécimal valide
	for _, c := range randomPart {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"Random part should be lowercase hexadecimal, got: %c", c)
	}
}

// ============================================================
// TESTS : HashAPIKey()
// ============================================================

func TestHashAPIKey_Consistency(t *testing.T) {
	key := "gsk_test_abc123def456"
	hash1 := HashAPIKey(key)
	hash2 := HashAPIKey(key)

	assert.Equal(t, hash1, hash2, "Same key should produce same hash")
	assert.Equal(t, 64, len(hash1), "SHA-256 should produce 64 hex characters")
}

func TestHashAPIKey_DifferentKeys(t *testing.T) {
	hash1 := HashAPIKey("gsk_test_key1")
	hash2 := HashAPIKey("gsk_test_key2")

	assert.NotEqual(t, hash1, hash2, "Different keys should produce different hashes")
}

func TestHashAPIKey_Security(t *testing.T) {
	key := "my-secret-api-key"
	hash := HashAPIKey(key)

	assert.NotEqual(t, key, hash, "Hash should not equal plaintext")
	assert.NotContains(t, hash, "secret", "Hash should not contain key parts")
}

// ============================================================
// TESTS : NewAPIKey() - Constructeur
// ============================================================

func TestNewAPIKey_Success(t *testing.T) {
	apiKey, fullKey, err := NewAPIKey(
		"user-123",
		"Integration Test",
		"Test API key",
		[]APIKeyScope{ScopeReadProducts, ScopeReadShops},
		true,
		30*24*time.Hour,
		"192.168.1.1",
		"Mozilla/5.0",
	)

	assert.NoError(t, err)
	assert.NotNil(t, apiKey)
	assert.NotEmpty(t, fullKey)
	assert.Equal(t, "user-123", apiKey.UserID)
	assert.Equal(t, "Integration Test", apiKey.Name)
	assert.Equal(t, "Test API key", apiKey.Description)
	assert.True(t, apiKey.IsActive)
	assert.NotNil(t, apiKey.ExpiresAt)
	assert.Equal(t, 60, apiKey.RateLimitPerMinute)
	assert.Equal(t, 10000, apiKey.RateLimitPerDay)
	assert.Equal(t, "192.168.1.1", apiKey.CreatedIP)
	assert.Equal(t, fullKey, apiKey.TemporaryFullKey)
}

func TestNewAPIKey_EmptyName(t *testing.T) {
	_, _, err := NewAPIKey(
		"user-123",
		"", // Nom vide
		"",
		[]APIKeyScope{ScopeReadProducts},
		true,
		30*24*time.Hour,
		"",
		"",
	)

	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyNameRequired, err)
}

func TestNewAPIKey_NoScopes(t *testing.T) {
	_, _, err := NewAPIKey(
		"user-123",
		"Test",
		"",
		[]APIKeyScope{}, // Aucun scope
		true,
		30*24*time.Hour,
		"",
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one scope is required")
}

func TestNewAPIKey_InvalidScope(t *testing.T) {
	_, _, err := NewAPIKey(
		"user-123",
		"Test",
		"",
		[]APIKeyScope{"invalid:scope"},
		true,
		30*24*time.Hour,
		"",
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
}

func TestNewAPIKey_DefaultLifetime(t *testing.T) {
	apiKey, _, err := NewAPIKey(
		"user-123",
		"Test",
		"",
		[]APIKeyScope{ScopeReadProducts},
		true,
		0, // Durée invalide
		"",
		"",
	)

	assert.NoError(t, err)
	expectedDuration := APIKeyDefaultLifetime
	diff := time.Until(*apiKey.ExpiresAt) - expectedDuration
	assert.True(t, diff < 5*time.Second, "Should use default lifetime (1 year)")
}

func TestNewAPIKey_MaxLifetime(t *testing.T) {
	apiKey, _, err := NewAPIKey(
		"user-123",
		"Test",
		"",
		[]APIKeyScope{ScopeReadProducts},
		true,
		10*365*24*time.Hour, // 10 ans (dépassé max 5 ans)
		"",
		"",
	)

	assert.NoError(t, err)
	maxDiff := time.Until(*apiKey.ExpiresAt) - APIKeyMaxLifetime
	assert.True(t, maxDiff < 5*time.Second, "Should cap at max lifetime (5 years)")
}

func TestNewAPIKey_KeyPrefixTruncated(t *testing.T) {
	apiKey, fullKey, err := NewAPIKey(
		"user-123",
		"Test",
		"",
		[]APIKeyScope{ScopeReadProducts},
		true,
		30*24*time.Hour,
		"",
		"",
	)

	assert.NoError(t, err)
	// KeyPrefix doit être préfixe + 12 caractères
	expectedPrefix := fullKey[:len(APIKeyPrefixTest)+12]
	assert.Equal(t, expectedPrefix, apiKey.KeyPrefix)
}

// ============================================================
// TESTS : ValidateKey()
// ============================================================

func TestValidateKey_ValidLive(t *testing.T) {
	key, _, _, _ := GenerateAPIKey(false)
	err := ValidateKey(key)
	assert.NoError(t, err)
}

func TestValidateKey_ValidTest(t *testing.T) {
	key, _, _, _ := GenerateAPIKey(true)
	err := ValidateKey(key)
	assert.NoError(t, err)
}

func TestValidateKey_TooShort(t *testing.T) {
	err := ValidateKey("gsk_test_short")
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyInvalid, err)
}

func TestValidateKey_InvalidPrefix(t *testing.T) {
	// Clé avec longueur correcte mais mauvais préfixe
	longKey := "invalid_prefix_" + strings.Repeat("a", 50)
	err := ValidateKey(longKey)
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyInvalidPrefix, err)
}

func TestValidateKey_Empty(t *testing.T) {
	err := ValidateKey("")
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyInvalid, err)
}

// ============================================================
// TESTS : IsExpired()
// ============================================================

func TestAPIKey_IsExpired_NotExpired(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{ExpiresAt: &future}
	assert.False(t, apiKey.IsExpired())
}

func TestAPIKey_IsExpired_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{ExpiresAt: &past}
	assert.True(t, apiKey.IsExpired())
}

func TestAPIKey_IsExpired_NoExpiration(t *testing.T) {
	apiKey := &APIKey{ExpiresAt: nil}
	assert.False(t, apiKey.IsExpired(), "No expiration should mean not expired")
}

// ============================================================
// TESTS : IsRevoked()
// ============================================================

func TestAPIKey_IsRevoked_NotRevoked(t *testing.T) {
	apiKey := &APIKey{IsActive: true, RevokedAt: nil}
	assert.False(t, apiKey.IsRevoked())
}

func TestAPIKey_IsRevoked_Revoked(t *testing.T) {
	revokedAt := time.Now()
	apiKey := &APIKey{IsActive: false, RevokedAt: &revokedAt}
	assert.True(t, apiKey.IsRevoked())
}

func TestAPIKey_IsRevoked_InactiveButNoRevokedAt(t *testing.T) {
	// Inactif mais sans RevokedAt (cas edge)
	apiKey := &APIKey{IsActive: false, RevokedAt: nil}
	assert.False(t, apiKey.IsRevoked(), "Should not be revoked without RevokedAt")
}

// ============================================================
// TESTS : IsValid()
// ============================================================

func TestAPIKey_IsValid_Active(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &future,
	}
	assert.True(t, apiKey.IsValid())
}

func TestAPIKey_IsValid_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &past,
	}
	assert.False(t, apiKey.IsValid())
}

func TestAPIKey_IsValid_Revoked(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	revokedAt := time.Now()
	apiKey := &APIKey{
		IsActive:  false,
		ExpiresAt: &future,
		RevokedAt: &revokedAt,
	}
	assert.False(t, apiKey.IsValid())
}

// ============================================================
// TESTS : HasScope()
// ============================================================

func TestAPIKey_HasScope_ExactMatch(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeReadProducts, ScopeReadShops},
	}
	assert.True(t, apiKey.HasScope(ScopeReadProducts))
	assert.True(t, apiKey.HasScope(ScopeReadShops))
	assert.False(t, apiKey.HasScope(ScopeWriteProducts))
}

func TestAPIKey_HasScope_AdminGivesAll(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeAdmin},
	}
	// Admin doit avoir accès à tous les scopes
	assert.True(t, apiKey.HasScope(ScopeReadProducts))
	assert.True(t, apiKey.HasScope(ScopeWriteOrders))
	assert.True(t, apiKey.HasScope(ScopeReadWallet))
	assert.True(t, apiKey.HasScope(ScopeAdmin))
}

func TestAPIKey_HasScope_Empty(t *testing.T) {
	apiKey := &APIKey{Scopes: []APIKeyScope{}}
	assert.False(t, apiKey.HasScope(ScopeReadProducts))
}

// ============================================================
// TESTS : HasAnyScope()
// ============================================================

func TestAPIKey_HasAnyScope_OneMatch(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeReadProducts},
	}
	assert.True(t, apiKey.HasAnyScope(ScopeReadProducts, ScopeWriteProducts))
}

func TestAPIKey_HasAnyScope_NoMatch(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeReadProducts},
	}
	assert.False(t, apiKey.HasAnyScope(ScopeWriteProducts, ScopeWriteOrders))
}

func TestAPIKey_HasAnyScope_Admin(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeAdmin},
	}
	assert.True(t, apiKey.HasAnyScope(ScopeReadProducts, ScopeWriteOrders))
}

// ============================================================
// TESTS : HasAllScopes()
// ============================================================

func TestAPIKey_HasAllScopes_AllPresent(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeReadProducts, ScopeReadShops, ScopeReadOrders},
	}
	assert.True(t, apiKey.HasAllScopes(ScopeReadProducts, ScopeReadShops))
}

func TestAPIKey_HasAllScopes_Missing(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeReadProducts},
	}
	assert.False(t, apiKey.HasAllScopes(ScopeReadProducts, ScopeWriteProducts))
}

func TestAPIKey_HasAllScopes_Admin(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeAdmin},
	}
	assert.True(t, apiKey.HasAllScopes(ScopeReadProducts, ScopeWriteOrders, ScopeReadWallet))
}

// ============================================================
// TESTS : CheckScope()
// ============================================================

func TestAPIKey_CheckScope_Valid(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &future,
		Scopes:    []APIKeyScope{ScopeReadProducts},
	}
	err := apiKey.CheckScope(ScopeReadProducts)
	assert.NoError(t, err)
}

func TestAPIKey_CheckScope_InsufficientScope(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &future,
		Scopes:    []APIKeyScope{ScopeReadProducts},
	}
	err := apiKey.CheckScope(ScopeWriteProducts)
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyInsufficientScope, err)
}

func TestAPIKey_CheckScope_Revoked(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	revokedAt := time.Now()
	apiKey := &APIKey{
		IsActive:  false,
		ExpiresAt: &future,
		RevokedAt: &revokedAt,
		Scopes:    []APIKeyScope{ScopeReadProducts},
	}
	err := apiKey.CheckScope(ScopeReadProducts)
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyRevoked, err)
}

func TestAPIKey_CheckScope_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &past,
		Scopes:    []APIKeyScope{ScopeReadProducts},
	}
	err := apiKey.CheckScope(ScopeReadProducts)
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyExpired, err)
}

// ============================================================
// TESTS : MarkRevoked()
// ============================================================

func TestAPIKey_MarkRevoked_Success(t *testing.T) {
	apiKey := &APIKey{IsActive: true}

	before := time.Now()
	err := apiKey.MarkRevoked("admin-123", "Security breach")
	after := time.Now()

	assert.NoError(t, err)
	assert.False(t, apiKey.IsActive)
	assert.NotNil(t, apiKey.RevokedAt)
	assert.Equal(t, "admin-123", apiKey.RevokedBy)
	assert.Equal(t, "Security breach", apiKey.RevocationReason)
	assert.True(t, !apiKey.RevokedAt.Before(before))
	assert.True(t, !apiKey.RevokedAt.After(after))
}

func TestAPIKey_MarkRevoked_AlreadyRevoked(t *testing.T) {
	revokedAt := time.Now()
	apiKey := &APIKey{
		IsActive:  false,
		RevokedAt: &revokedAt,
	}

	err := apiKey.MarkRevoked("admin-123", "Test")
	assert.Error(t, err)
	assert.Equal(t, ErrAPIKeyAlreadyRevoked, err)
}

// ============================================================
// TESTS : MarkUsed()
// ============================================================

func TestAPIKey_MarkUsed(t *testing.T) {
	apiKey := &APIKey{LastUsedAt: nil}

	before := time.Now()
	apiKey.MarkUsed()
	after := time.Now()

	assert.NotNil(t, apiKey.LastUsedAt)
	assert.True(t, !apiKey.LastUsedAt.Before(before))
	assert.True(t, !apiKey.LastUsedAt.After(after))
}

func TestAPIKey_MarkUsed_UpdatesExisting(t *testing.T) {
	oldTime := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{LastUsedAt: &oldTime}

	apiKey.MarkUsed()
	assert.True(t, apiKey.LastUsedAt.After(oldTime), "Should update to newer time")
}

// ============================================================
// TESTS : ExtendExpiration()
// ============================================================

func TestAPIKey_ExtendExpiration_Success(t *testing.T) {
	future := time.Now().Add(30 * 24 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &future,
		CreatedAt: time.Now().Add(-10 * 24 * time.Hour),
	}

	err := apiKey.ExtendExpiration(60 * 24 * time.Hour)
	assert.NoError(t, err)
	assert.True(t, apiKey.ExpiresAt.After(future), "Should extend expiration")
}

func TestAPIKey_ExtendExpiration_Revoked(t *testing.T) {
	revokedAt := time.Now()
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{
		IsActive:  false,
		RevokedAt: &revokedAt,
		ExpiresAt: &future,
	}

	err := apiKey.ExtendExpiration(24 * time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot extend revoked key")
}

func TestAPIKey_ExtendExpiration_ExceedsMax(t *testing.T) {
	apiKey := &APIKey{
		IsActive:  true,
		CreatedAt: time.Now().Add(-4 * 365 * 24 * time.Hour), // Créé il y a 4 ans
		ExpiresAt: func() *time.Time { t := time.Now().Add(365 * 24 * time.Hour); return &t }(),
	}

	// Tentative d'extension de 3 ans (total > 5 ans max)
	err := apiKey.ExtendExpiration(3 * 365 * 24 * time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceed maximum lifetime")
}

func TestAPIKey_ExtendExpiration_DefaultDuration(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &future,
		CreatedAt: time.Now(),
	}

	err := apiKey.ExtendExpiration(0) // Durée invalide
	assert.NoError(t, err)
	// Devrait utiliser la durée par défaut (1 an)
	expectedDuration := APIKeyDefaultLifetime
	diff := time.Until(*apiKey.ExpiresAt) - expectedDuration
	assert.True(t, diff < 5*time.Second, "Should use default lifetime")
}

// ============================================================
// TESTS : MaskKey()
// ============================================================

func TestAPIKey_MaskKey_WithPrefix(t *testing.T) {
	apiKey := &APIKey{KeyPrefix: "gsk_test_abc123"}
	assert.Equal(t, "gsk_test_abc123...", apiKey.MaskKey())
}

func TestAPIKey_MaskKey_Empty(t *testing.T) {
	apiKey := &APIKey{KeyPrefix: ""}
	assert.Equal(t, "***", apiKey.MaskKey())
}

// ============================================================
// TESTS : GetStatus()
// ============================================================

func TestAPIKey_GetStatus_Revoked(t *testing.T) {
	revokedAt := time.Now()
	apiKey := &APIKey{
		IsActive:  false,
		RevokedAt: &revokedAt,
	}
	assert.Equal(t, "revoked", apiKey.GetStatus())
}

func TestAPIKey_GetStatus_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &past,
	}
	assert.Equal(t, "expired", apiKey.GetStatus())
}

func TestAPIKey_GetStatus_Unused(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	apiKey := &APIKey{
		IsActive:   true,
		ExpiresAt:  &future,
		LastUsedAt: nil,
	}
	assert.Equal(t, "unused", apiKey.GetStatus())
}

func TestAPIKey_GetStatus_Inactive(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	oldTime := time.Now().Add(-60 * 24 * time.Hour) // 60 jours
	apiKey := &APIKey{
		IsActive:   true,
		ExpiresAt:  &future,
		LastUsedAt: &oldTime,
	}
	assert.Equal(t, "inactive", apiKey.GetStatus())
}

func TestAPIKey_GetStatus_Active(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	recent := time.Now().Add(-5 * 24 * time.Hour) // 5 jours
	apiKey := &APIKey{
		IsActive:   true,
		ExpiresAt:  &future,
		LastUsedAt: &recent,
	}
	assert.Equal(t, "active", apiKey.GetStatus())
}

func TestAPIKey_GetStatus_RevokedTakesPrecedence(t *testing.T) {
	revokedAt := time.Now()
	past := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{
		IsActive:  false,
		RevokedAt: &revokedAt,
		ExpiresAt: &past, // Même expiré
	}
	assert.Equal(t, "revoked", apiKey.GetStatus(), "Revoked should take precedence")
}

// ============================================================
// TESTS : GetDaysUntilExpiration()
// ============================================================

func TestAPIKey_GetDaysUntilExpiration_Future(t *testing.T) {
	future := time.Now().Add(10 * 24 * time.Hour)
	apiKey := &APIKey{ExpiresAt: &future}
	days := apiKey.GetDaysUntilExpiration()
	assert.True(t, days >= 9 && days <= 10, "Should be approximately 10 days")
}

func TestAPIKey_GetDaysUntilExpiration_Past(t *testing.T) {
	past := time.Now().Add(-1 * 24 * time.Hour)
	apiKey := &APIKey{ExpiresAt: &past}
	days := apiKey.GetDaysUntilExpiration()
	assert.Equal(t, 0, days, "Past expiration should return 0")
}

func TestAPIKey_GetDaysUntilExpiration_NoExpiration(t *testing.T) {
	apiKey := &APIKey{ExpiresAt: nil}
	days := apiKey.GetDaysUntilExpiration()
	assert.Equal(t, -1, days, "No expiration should return -1")
}

// ============================================================
// TESTS : IsExpiringSoon()
// ============================================================

func TestAPIKey_IsExpiringSoon_True(t *testing.T) {
	future := time.Now().Add(15 * 24 * time.Hour) // 15 jours
	apiKey := &APIKey{ExpiresAt: &future}
	assert.True(t, apiKey.IsExpiringSoon(), "15 days should be expiring soon")
}

func TestAPIKey_IsExpiringSoon_False(t *testing.T) {
	future := time.Now().Add(60 * 24 * time.Hour) // 60 jours
	apiKey := &APIKey{ExpiresAt: &future}
	assert.False(t, apiKey.IsExpiringSoon(), "60 days should not be expiring soon")
}

func TestAPIKey_IsExpiringSoon_NoExpiration(t *testing.T) {
	apiKey := &APIKey{ExpiresAt: nil}
	assert.False(t, apiKey.IsExpiringSoon(), "No expiration should not be expiring soon")
}

func TestAPIKey_IsExpiringSoon_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	apiKey := &APIKey{ExpiresAt: &past}
	assert.True(t, apiKey.IsExpiringSoon(), "Expired should be expiring soon")
}

// ============================================================
// TESTS : ToSummary()
// ============================================================

func TestAPIKey_ToSummary(t *testing.T) {
	future := time.Now().Add(30 * 24 * time.Hour)
	apiKey := &APIKey{
		ID:                 "key-123",
		UserID:             "user-456",
		Name:               "Test Key",
		Description:        "Test description",
		KeyPrefix:          "gsk_test_abc123",
		Scopes:             []APIKeyScope{ScopeReadProducts, ScopeReadShops},
		ExpiresAt:          &future,
		IsActive:           true,
		RateLimitPerMinute: 60,
		RateLimitPerDay:    10000,
		CreatedAt:          time.Now(),
	}

	summary := apiKey.ToSummary()

	assert.Equal(t, "key-123", summary.ID)
	assert.Equal(t, "user-456", summary.UserID)
	assert.Equal(t, "Test Key", summary.Name)
	assert.Equal(t, "Test description", summary.Description)
	assert.Equal(t, "gsk_test_abc123...", summary.KeyPrefix)
	assert.Equal(t, 2, len(summary.Scopes))
	assert.Equal(t, "unused", summary.Status)
	assert.Equal(t, 60, summary.RateLimitPerMinute)
	assert.Equal(t, 10000, summary.RateLimitPerDay)
	assert.True(t, summary.DaysUntilExpiration >= 29)
}

// ============================================================
// TESTS : Sécurité JSON
// ============================================================

func TestAPIKey_JSON_ExcludesSensitiveFields(t *testing.T) {
	apiKey := &APIKey{
		ID:               "key-123",
		UserID:           "user-456",
		KeyHash:          "secret-hash-should-not-appear",
		TemporaryFullKey: "full-key-should-appear-once",
		IsActive:         true,
	}

	data, err := json.Marshal(apiKey)
	assert.NoError(t, err)

	jsonStr := string(data)

	// KeyHash ne doit JAMAIS être sérialisé (sécurité)
	assert.NotContains(t, jsonStr, "secret-hash-should-not-appear",
		"KeyHash should not be serialized (json:\"-\")")

	// TemporaryFullKey DOIT être sérialisé (pour affichage à la création)
	assert.Contains(t, jsonStr, "full-key-should-appear-once",
		"TemporaryFullKey should be serialized (for one-time display)")

	// Les autres champs doivent être présents
	assert.Contains(t, jsonStr, "key-123", "ID should be serialized")
	assert.Contains(t, jsonStr, "user-456", "UserID should be serialized")
}

// Test supplémentaire : Vérifier que TemporaryFullKey est omis si vide
func TestAPIKey_JSON_OmitsEmptyTemporaryKey(t *testing.T) {
	apiKey := &APIKey{
		ID:               "key-123",
		UserID:           "user-456",
		TemporaryFullKey: "", // Vide
		IsActive:         true,
	}

	data, err := json.Marshal(apiKey)
	assert.NoError(t, err)

	jsonStr := string(data)

	// Avec omitempty, le champ vide ne doit pas apparaître
	assert.NotContains(t, jsonStr, "temporary_full_key",
		"Empty TemporaryFullKey should be omitted (omitempty)")
}

// ============================================================
// TESTS : Edge Cases
// ============================================================

func TestAPIKey_JustExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Second)
	apiKey := &APIKey{
		IsActive:  true,
		ExpiresAt: &past,
	}
	assert.True(t, apiKey.IsExpired())
	assert.Equal(t, "expired", apiKey.GetStatus())
}

func TestAPIKey_AdminScopeOverridesEverything(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{ScopeAdmin},
	}

	// Admin doit donner accès à tous les scopes
	allScopes := AllScopes()
	for _, scope := range allScopes {
		assert.True(t, apiKey.HasScope(scope),
			"Admin should have access to scope: %s", scope)
	}
}

func TestAPIKey_MultipleScopes(t *testing.T) {
	apiKey := &APIKey{
		Scopes: []APIKeyScope{
			ScopeReadProducts,
			ScopeWriteProducts,
			ScopeReadOrders,
			ScopeAdmin,
		},
	}

	assert.True(t, apiKey.HasAllScopes(
		ScopeReadProducts,
		ScopeWriteProducts,
		ScopeReadOrders,
		ScopeAdmin,
	))
}

func TestGenerateAPIKey_CryptographicStrength(t *testing.T) {
	// Vérifier que les clés générées ont une entropie suffisante
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key, _, _, err := GenerateAPIKey(false)
		assert.NoError(t, err)
		keys[i] = key
	}

	// Vérifier l'unicité (très improbable d'avoir des doublons)
	uniqueKeys := make(map[string]bool)
	for _, key := range keys {
		assert.False(t, uniqueKeys[key], "Keys should be unique")
		uniqueKeys[key] = true
	}
	assert.Equal(t, 1000, len(uniqueKeys), "All 1000 keys should be unique")
}

func TestAPIKey_RateLimits_Defaults(t *testing.T) {
	apiKey, _, err := NewAPIKey(
		"user-123",
		"Test",
		"",
		[]APIKeyScope{ScopeReadProducts},
		true,
		30*24*time.Hour,
		"",
		"",
	)

	assert.NoError(t, err)
	assert.Equal(t, APIKeyDefaultRateLimitMinute, apiKey.RateLimitPerMinute)
	assert.Equal(t, APIKeyDefaultRateLimitDay, apiKey.RateLimitPerDay)
}
