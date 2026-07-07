package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.3 : TESTS UNITAIRES - ENTITÉ USER 2FA
// ============================================================

// ============================================================
// TESTS : IsLocked()
// ============================================================

func TestUser2FA_IsLocked_NotLocked(t *testing.T) {
	user2fa := &User2FA{
		FailedAttempts: 0,
		LockedUntil:    nil,
	}

	assert.False(t, user2fa.IsLocked(), "Should not be locked when no lock time")
}

func TestUser2FA_IsLocked_LockedInFuture(t *testing.T) {
	futureTime := time.Now().Add(15 * time.Minute)
	user2fa := &User2FA{
		FailedAttempts: 5,
		LockedUntil:    &futureTime,
	}

	assert.True(t, user2fa.IsLocked(), "Should be locked when lock time is in future")
}

func TestUser2FA_IsLocked_LockExpired(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Minute)
	user2fa := &User2FA{
		FailedAttempts: 5,
		LockedUntil:    &pastTime,
	}

	assert.False(t, user2fa.IsLocked(), "Should not be locked when lock time has passed")
}

// ============================================================
// TESTS : IsSetupComplete()
// ============================================================

func TestUser2FA_IsSetupComplete_Complete(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       true,
	}

	assert.True(t, user2fa.IsSetupComplete(), "Should be complete when secret exists")
}

func TestUser2FA_IsSetupComplete_Incomplete(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "",
		IsEnabled:       false,
	}

	assert.False(t, user2fa.IsSetupComplete(), "Should not be complete without secret")
}

// ============================================================
// TESTS : IsCodeReplay() - Anti-replay (RFC 6238)
// ============================================================

func TestUser2FA_IsCodeReplay_FirstUse(t *testing.T) {
	user2fa := &User2FA{
		LastUsedCode: "",
		LastUsedAt:   nil,
	}

	assert.False(t, user2fa.IsCodeReplay("123456"), "Should not be replay on first use")
}

func TestUser2FA_IsCodeReplay_SameCodeWithinWindow(t *testing.T) {
	recentTime := time.Now().Add(-10 * time.Second)
	user2fa := &User2FA{
		LastUsedCode: "123456",
		LastUsedAt:   &recentTime,
	}

	assert.True(t, user2fa.IsCodeReplay("123456"), "Should detect replay within 30s window")
}

func TestUser2FA_IsCodeReplay_SameCodeAfterWindow(t *testing.T) {
	oldTime := time.Now().Add(-60 * time.Second)
	user2fa := &User2FA{
		LastUsedCode: "123456",
		LastUsedAt:   &oldTime,
	}

	assert.False(t, user2fa.IsCodeReplay("123456"), "Should allow reuse after 30s window")
}

func TestUser2FA_IsCodeReplay_DifferentCode(t *testing.T) {
	recentTime := time.Now().Add(-10 * time.Second)
	user2fa := &User2FA{
		LastUsedCode: "123456",
		LastUsedAt:   &recentTime,
	}

	assert.False(t, user2fa.IsCodeReplay("654321"), "Should allow different code")
}

// ============================================================
// TESTS : RecordCodeUsed()
// ============================================================

func TestUser2FA_RecordCodeUsed(t *testing.T) {
	user2fa := &User2FA{
		LastUsedCode: "",
		LastUsedAt:   nil,
	}

	before := time.Now()
	user2fa.RecordCodeUsed("123456")
	after := time.Now()

	assert.Equal(t, "123456", user2fa.LastUsedCode, "Should store the code")
	assert.NotNil(t, user2fa.LastUsedAt, "Should set timestamp")
	assert.True(t, !user2fa.LastUsedAt.Before(before), "Timestamp should be recent")
	assert.True(t, !user2fa.LastUsedAt.After(after), "Timestamp should be recent")
}

// ============================================================
// TESTS : RecordFailedAttempt()
// ============================================================

func TestUser2FA_RecordFailedAttempt(t *testing.T) {
	user2fa := &User2FA{
		FailedAttempts: 0,
	}

	user2fa.RecordFailedAttempt()
	assert.Equal(t, 1, user2fa.FailedAttempts, "Should increment to 1")

	user2fa.RecordFailedAttempt()
	assert.Equal(t, 2, user2fa.FailedAttempts, "Should increment to 2")
}

func TestUser2FA_RecordFailedAttempt_LockAfter5(t *testing.T) {
	user2fa := &User2FA{
		FailedAttempts: 0,
	}

	// 5 tentatives échouées
	for i := 0; i < 5; i++ {
		user2fa.RecordFailedAttempt()
	}

	assert.Equal(t, 5, user2fa.FailedAttempts, "Should have 5 failed attempts")
	assert.NotNil(t, user2fa.LockedUntil, "Should be locked after 5 attempts")
	assert.True(t, user2fa.IsLocked(), "Should be locked")
}

// ============================================================
// TESTS : RecordSuccessfulVerification()
// ============================================================

func TestUser2FA_RecordSuccessfulVerification(t *testing.T) {
	futureTime := time.Now().Add(15 * time.Minute)
	user2fa := &User2FA{
		FailedAttempts: 3,
		LockedUntil:    &futureTime,
	}

	before := time.Now()
	user2fa.RecordSuccessfulVerification()
	after := time.Now()

	assert.Equal(t, 0, user2fa.FailedAttempts, "Should reset failed attempts")
	assert.Nil(t, user2fa.LockedUntil, "Should clear lock")
	assert.NotNil(t, user2fa.LastVerifiedAt, "Should set last verified")
	assert.True(t, !user2fa.LastVerifiedAt.Before(before), "Timestamp should be recent")
	assert.True(t, !user2fa.LastVerifiedAt.After(after), "Timestamp should be recent")
}

// ============================================================
// TESTS : GetStatus() - Basé sur la logique réelle
// ============================================================

func TestUser2FA_GetStatus_NotSetup(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "",
		IsEnabled:       false,
	}

	assert.Equal(t, "not_setup", user2fa.GetStatus(),
		"Should return not_setup when no secret")
}

func TestUser2FA_GetStatus_Enabled(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       true,
	}

	assert.Equal(t, "enabled", user2fa.GetStatus(),
		"Should return enabled when secret exists and is enabled")
}

func TestUser2FA_GetStatus_Disabled(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       false,
	}

	assert.Equal(t, "disabled", user2fa.GetStatus(),
		"Should return disabled when secret exists but not enabled")
}

func TestUser2FA_GetStatus_Locked(t *testing.T) {
	futureTime := time.Now().Add(15 * time.Minute)
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       true,
		FailedAttempts:  5,
		LockedUntil:     &futureTime,
	}

	assert.Equal(t, "locked", user2fa.GetStatus(),
		"Should return locked when account is locked")
}

func TestUser2FA_GetStatus_LockedTakesPrecedence(t *testing.T) {
	futureTime := time.Now().Add(15 * time.Minute)
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       true,
		FailedAttempts:  5,
		LockedUntil:     &futureTime,
	}

	// Locked doit être prioritaire sur enabled
	assert.Equal(t, "locked", user2fa.GetStatus(),
		"Locked should take precedence over enabled")
}

func TestUser2FA_GetStatus_NotSetupTakesPrecedence(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "",
		IsEnabled:       true, // Même si enabled, sans secret c'est not_setup
	}

	// Not setup doit être prioritaire
	assert.Equal(t, "not_setup", user2fa.GetStatus(),
		"Not setup should take precedence")
}

// ============================================================
// TESTS : GetRemainingRecoveryCodes()
// ============================================================

func TestUser2FA_GetRemainingRecoveryCodes_NoCodes(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{},
	}

	assert.Equal(t, 10, user2fa.GetRemainingRecoveryCodes(),
		"Should return 10 when no codes used")
}

func TestUser2FA_GetRemainingRecoveryCodes_SomeUsed(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{0, 2, 4},
	}

	assert.Equal(t, 7, user2fa.GetRemainingRecoveryCodes(),
		"Should return 7 when 3 codes used")
}

func TestUser2FA_GetRemainingRecoveryCodes_AllUsed(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	assert.Equal(t, 0, user2fa.GetRemainingRecoveryCodes(),
		"Should return 0 when all codes used")
}

// ============================================================
// TESTS : ValidateRecoveryCode()
// ============================================================

func TestUser2FA_ValidateRecoveryCode_ValidCode(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{},
	}

	codes := []string{"ABCD-1234", "EFGH-5678", "IJKL-9012"}

	index, err := user2fa.ValidateRecoveryCode("EFGH-5678", codes)
	assert.NoError(t, err, "Should not return error for valid code")
	assert.Equal(t, 1, index, "Should return correct index")
}

func TestUser2FA_ValidateRecoveryCode_InvalidCode(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{},
	}

	codes := []string{"ABCD-1234", "EFGH-5678", "IJKL-9012"}

	_, err := user2fa.ValidateRecoveryCode("INVALID", codes)
	assert.Error(t, err, "Should return error for invalid code")
	assert.Equal(t, Err2FARecoveryCodeNotFound, err, "Should return correct error")
}

func TestUser2FA_ValidateRecoveryCode_AlreadyUsed(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{1},
	}

	codes := []string{"ABCD-1234", "EFGH-5678", "IJKL-9012"}

	_, err := user2fa.ValidateRecoveryCode("EFGH-5678", codes)
	assert.Error(t, err, "Should return error for already used code")
	assert.Equal(t, Err2FARecoveryCodeUsed, err, "Should return correct error")
}

func TestUser2FA_ValidateRecoveryCode_CaseInsensitive(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{},
	}

	codes := []string{"ABCD-1234", "EFGH-5678", "IJKL-9012"}

	// Test en minuscules
	index, err := user2fa.ValidateRecoveryCode("efgh-5678", codes)
	assert.NoError(t, err, "Should accept lowercase")
	assert.Equal(t, 1, index, "Should return correct index")
}

// ============================================================
// TESTS : MarkRecoveryCodeUsed()
// ============================================================

func TestUser2FA_MarkRecoveryCodeUsed(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{},
	}

	user2fa.MarkRecoveryCodeUsed(2)
	assert.Contains(t, user2fa.RecoveryCodesUsed, 2, "Should mark code as used")
	assert.Equal(t, 1, len(user2fa.RecoveryCodesUsed), "Should have 1 used code")
}

func TestUser2FA_MarkRecoveryCodeUsed_Multiple(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{0, 1},
	}

	user2fa.MarkRecoveryCodeUsed(3)
	assert.Contains(t, user2fa.RecoveryCodesUsed, 0, "Should keep existing codes")
	assert.Contains(t, user2fa.RecoveryCodesUsed, 1, "Should keep existing codes")
	assert.Contains(t, user2fa.RecoveryCodesUsed, 3, "Should add new code")
	assert.Equal(t, 3, len(user2fa.RecoveryCodesUsed), "Should have 3 used codes")
}

func TestUser2FA_MarkRecoveryCodeUsed_NoDuplicate(t *testing.T) {
	user2fa := &User2FA{
		RecoveryCodesUsed: []int{1},
	}

	// Marquer le même code deux fois
	user2fa.MarkRecoveryCodeUsed(1)
	user2fa.MarkRecoveryCodeUsed(1)

	// Ne doit pas créer de doublon
	assert.Equal(t, 1, len(user2fa.RecoveryCodesUsed),
		"Should not create duplicate")
}

// ============================================================
// TESTS : GenerateRecoveryCodes()
// ============================================================

func TestGenerateRecoveryCodes_Success(t *testing.T) {
	codes, err := GenerateRecoveryCodes()

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, 10, len(codes), "Should generate 10 codes")

	// Vérifier le format XXXX-XXXX
	for _, code := range codes {
		assert.Len(t, code, 9, "Code should be 9 characters (XXXX-XXXX)")
		assert.Contains(t, code, "-", "Code should contain hyphen")
	}

	// Vérifier l'unicité
	uniqueCodes := make(map[string]bool)
	for _, code := range codes {
		assert.False(t, uniqueCodes[code], "Codes should be unique")
		uniqueCodes[code] = true
	}
}

func TestGenerateRecoveryCodes_Uniqueness(t *testing.T) {
	// Générer plusieurs fois et vérifier qu'on obtient des codes différents
	codes1, _ := GenerateRecoveryCodes()
	codes2, _ := GenerateRecoveryCodes()

	// Très improbable d'avoir les mêmes codes
	assert.NotEqual(t, codes1, codes2,
		"Two generations should produce different codes")
}

// ============================================================
// TESTS : Enable() et Disable()
// ============================================================

func TestUser2FA_Enable_Success(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       false,
	}

	err := user2fa.Enable()
	assert.NoError(t, err, "Should enable successfully")
	assert.True(t, user2fa.IsEnabled, "Should be enabled")
	assert.NotNil(t, user2fa.EnabledAt, "Should set enabled timestamp")
}

func TestUser2FA_Enable_NoSecret(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "",
		IsEnabled:       false,
	}

	err := user2fa.Enable()
	assert.Error(t, err, "Should return error without secret")
	assert.Equal(t, Err2FASecretRequired, err, "Should return correct error")
}

func TestUser2FA_Enable_AlreadyEnabled(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       true,
	}

	err := user2fa.Enable()
	assert.Error(t, err, "Should return error if already enabled")
	assert.Equal(t, Err2FAAlreadyEnabled, err, "Should return correct error")
}

func TestUser2FA_Disable_Success(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted:   "encrypted-secret",
		IsEnabled:         true,
		RecoveryCodesUsed: []int{1, 2, 3},
		FailedAttempts:    2,
	}

	user2fa.Disable()
	assert.False(t, user2fa.IsEnabled, "Should be disabled")
	assert.NotNil(t, user2fa.DisabledAt, "Should set disabled timestamp")
	assert.Equal(t, "", user2fa.SecretEncrypted, "Should clear secret")
	assert.Equal(t, "", user2fa.RecoveryCodesEncrypted, "Should clear recovery codes")
	assert.Equal(t, 0, len(user2fa.RecoveryCodesUsed), "Should clear used codes")
	assert.Equal(t, 0, user2fa.FailedAttempts, "Should reset failed attempts")
}

func TestUser2FA_Disable_NotEnabled(t *testing.T) {
	user2fa := &User2FA{
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       false,
	}

	// Ne devrait rien faire
	user2fa.Disable()
	assert.False(t, user2fa.IsEnabled, "Should remain disabled")
	assert.Nil(t, user2fa.DisabledAt, "Should not set disabled timestamp")
}

// ============================================================
// TESTS : Unlock()
// ============================================================

func TestUser2FA_Unlock(t *testing.T) {
	futureTime := time.Now().Add(15 * time.Minute)
	user2fa := &User2FA{
		FailedAttempts: 5,
		LockedUntil:    &futureTime,
	}

	user2fa.Unlock()
	assert.Equal(t, 0, user2fa.FailedAttempts, "Should reset failed attempts")
	assert.Nil(t, user2fa.LockedUntil, "Should clear lock")
	assert.False(t, user2fa.IsLocked(), "Should not be locked")
}
