package twofausecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	twofausecase "Goshop/application/usecase/twofa_usecase"
	"Goshop/domain/entity"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/infrastructure/crypto"
	mockrepo "Goshop/mocks/repository"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.3 : TESTS DES BRANCHES MANQUANTES (CIBLE > 90%)
// ============================================================

// Helpers locaux pour ce fichier
func createAdminCtx() *twofausecase.AdminContext {
	return &twofausecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@example.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.100",
		UserAgent:  "TestAgent",
	}
}

func createValidEncryptedUser2FA(t *testing.T, userID string) (*entity.User2FA, string) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoShop",
		AccountName: "test@example.com",
	})
	assert.NoError(t, err)

	secretEncrypted, err := crypto.Encrypt(key.Secret())
	assert.NoError(t, err)

	recoveryCodes := []string{"REC-1111", "REC-2222"}
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)
	recoveryCodesEncrypted, _ := crypto.Encrypt(string(recoveryCodesJSON))

	return &entity.User2FA{
		ID:                     "2fa-" + userID,
		UserID:                 userID,
		SecretEncrypted:        secretEncrypted,
		IsEnabled:              true,
		RecoveryCodesEncrypted: recoveryCodesEncrypted,
		RecoveryCodesUsed:      []int{},
		FailedAttempts:         0,
	}, key.Secret()
}

func generateValidCode(secret string) string {
	code, _ := totp.GenerateCode(secret, time.Now())
	return code
}

// ============================================================
// 1. Setup2FAUsecase - Sentinel Error ErrUserNotFound
// ============================================================

func TestSetup2FAUsecase_UserNotFoundSentinelError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)
	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminCtx()

	// ✅ Retourne EXACTEMENT le sentinel error
	mockUser.EXPECT().
		FindUserByID("user-123").
		Return(nil, userrepository.ErrUserNotFound)

	req := &twofausecase.Setup2FARequest{UserID: "user-123"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	// Vérifie que c'est bien l'erreur spécifique "user not found"
	assert.Contains(t, err.Error(), "user not found")
}

// ============================================================
// 2. VerifyAndEnable2FAUsecase - Anti-Replay & Decryption Fail
// ============================================================

func TestVerifyAndEnable2FAUsecase_AntiReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	now := time.Now()
	user2fa := &entity.User2FA{
		UserID:         "user-123",
		IsEnabled:      false,
		LastUsedCode:   "123456", // ✅ Code déjà utilisé
		LastUsedAt:     &now,     // ✅ Dans la fenêtre de 30s
		FailedAttempts: 0,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		IncrementFailedAttempts(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.VerifyAndEnable2FARequest{UserID: "user-123", Code: "123456"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FACodeAlreadyUsed, err)
}

func TestVerifyAndEnable2FAUsecase_DecryptSecretError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	// ✅ Secret chiffré invalide (garbage)
	user2fa := &entity.User2FA{
		UserID:          "user-123",
		IsEnabled:       false,
		SecretEncrypted: "invalid-garbage-encrypted-data",
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	req := &twofausecase.VerifyAndEnable2FARequest{UserID: "user-123", Code: "123456"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "decrypt secret")
}

// ============================================================
// 3. Disable2FAUsecase - Disable2FA Repo Error & Anti-Replay
// ============================================================

func TestDisable2FAUsecase_Disable2FARepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	user2fa, secret := createValidEncryptedUser2FA(t, "user-123")
	validCode := generateValidCode(secret)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	// ✅ Simule l'échec de la désactivation en base de données
	mockUser2FA.EXPECT().
		Disable2FA(gomock.Any(), "user-123").
		Return(errors.New("database error"))

	req := &twofausecase.Disable2FARequest{
		UserID:     "user-123",
		Code:       validCode,
		IsRecovery: false,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "disable 2fa")
}

func TestDisable2FAUsecase_AntiReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	now := time.Now()
	user2fa := &entity.User2FA{
		UserID:         "user-123",
		IsEnabled:      true,
		LastUsedCode:   "123456",
		LastUsedAt:     &now,
		FailedAttempts: 0,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		IncrementFailedAttempts(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.Disable2FARequest{UserID: "user-123", Code: "123456", IsRecovery: false}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FACodeAlreadyUsed, err)
}

func TestDisable2FAUsecase_DecryptSecretError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	user2fa := &entity.User2FA{
		UserID:          "user-123",
		IsEnabled:       true,
		SecretEncrypted: "invalid-garbage-encrypted-data",
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	req := &twofausecase.Disable2FARequest{UserID: "user-123", Code: "123456", IsRecovery: false}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "decrypt secret")
}

// ============================================================
// 4. verifyRecoveryCode - JSON Unmarshal Fail & Invalid Code
// ============================================================

func TestDisable2FAUsecase_VerifyRecoveryCode_UnmarshalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	// ✅ Chiffre une chaîne qui n'est PAS un tableau JSON valide
	invalidJSONEncrypted, _ := crypto.Encrypt("pas-un-tableau-json")

	user2fa := &entity.User2FA{
		UserID:                 "user-123",
		IsEnabled:              true,
		RecoveryCodesEncrypted: invalidJSONEncrypted,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	req := &twofausecase.Disable2FARequest{UserID: "user-123", Code: "REC-1234", IsRecovery: true}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "unmarshal recovery codes")
}

func TestDisable2FAUsecase_VerifyRecoveryCode_InvalidCode_Increments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	// Codes valides chiffrés
	validCodes := []string{"REC-AAAA", "REC-BBBB"}
	codesJSON, _ := json.Marshal(validCodes)
	encryptedCodes, _ := crypto.Encrypt(string(codesJSON))

	user2fa := &entity.User2FA{
		UserID:                 "user-123",
		IsEnabled:              true,
		RecoveryCodesEncrypted: encryptedCodes,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	// ✅ Le code est invalide, donc IncrementFailedAttempts DOIT être appelé
	mockUser2FA.EXPECT().
		IncrementFailedAttempts(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.Disable2FARequest{UserID: "user-123", Code: "REC-WRONG", IsRecovery: true}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FARecoveryCodeNotFound, err)
}

// ============================================================
// 5. RegenerateRecoveryCodesUsecase - Anti-Replay & Decryption
// ============================================================

func TestRegenerateRecoveryCodesUsecase_AntiReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	now := time.Now()
	user2fa := &entity.User2FA{
		UserID:         "user-123",
		IsEnabled:      true,
		LastUsedCode:   "123456",
		LastUsedAt:     &now,
		FailedAttempts: 0,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		IncrementFailedAttempts(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.RegenerateRecoveryCodesRequest{UserID: "user-123", Code: "123456"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FACodeAlreadyUsed, err)
}

func TestRegenerateRecoveryCodesUsecase_DecryptSecretError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminCtx()

	user2fa := &entity.User2FA{
		UserID:          "user-123",
		IsEnabled:       true,
		SecretEncrypted: "invalid-garbage-encrypted-data",
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	req := &twofausecase.RegenerateRecoveryCodesRequest{UserID: "user-123", Code: "123456"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "decrypt secret")
}
