package twofausecase_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	twofausecase "Goshop/application/usecase/twofa_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/infrastructure/crypto"
	mockrepo "Goshop/mocks/repository"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.3 : TESTS DES CHEMINS DE SUCCÈS (HAPPY PATHS)
// ============================================================

// generateValidTOTPCode génère un code TOTP valide pour un secret donné
func generateValidTOTPCode(secret string) (string, error) {
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", err
	}
	return code, nil
}

// createTestUser2FAWithValidSecret crée un User2FA avec un vrai secret TOTP chiffré
func createTestUser2FAWithValidSecret(t *testing.T, userID string) (*entity.User2FA, string) {
	// Générer un vrai secret TOTP
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoShop",
		AccountName: "test@example.com",
	})
	assert.NoError(t, err)

	// Chiffrer le secret
	secretEncrypted, err := crypto.Encrypt(key.Secret())
	assert.NoError(t, err)

	// Chiffrer des codes de récupération
	recoveryCodes := []string{"REC-1111", "REC-2222", "REC-3333", "REC-4444", "REC-5555"}
	recoveryCodesJSON, err := json.Marshal(recoveryCodes)
	assert.NoError(t, err)

	recoveryCodesEncrypted, err := crypto.Encrypt(string(recoveryCodesJSON))
	assert.NoError(t, err)

	user2fa := &entity.User2FA{
		ID:                     "2fa-" + userID,
		UserID:                 userID,
		SecretEncrypted:        secretEncrypted,
		IsEnabled:              true,
		RecoveryCodesEncrypted: recoveryCodesEncrypted,
		RecoveryCodesUsed:      []int{},
		FailedAttempts:         0,
	}

	return user2fa, key.Secret()
}

// ============================================================
// TESTS : VerifyAndEnable2FAUsecase - Succès complet
// ============================================================

func TestVerifyAndEnable2FAUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	// Créer un User2FA avec un vrai secret
	user2fa, secret := createTestUser2FAWithValidSecret(t, "user-123")
	user2fa.IsEnabled = false // Pas encore activé

	// Générer un code TOTP valide
	validCode, err := generateValidTOTPCode(secret)
	assert.NoError(t, err)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	// Mocks pour le chemin de succès
	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	mockUser2FA.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), "user-123", gomock.Any()).
		Return(nil)

	mockUser2FA.EXPECT().
		Enable2FA(gomock.Any(), "user-123").
		Return(nil)

	mockUser2FA.EXPECT().
		RecordSuccessfulVerification(gomock.Any(), "user-123", "192.168.1.100", "TestAgent").
		Return(nil)

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   validCode,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.True(t, response.IsEnabled)
	assert.NotEmpty(t, response.RecoveryCodes)
	assert.Contains(t, response.Message, "2FA activée avec succès")
}

// ============================================================
// TESTS : Disable2FAUsecase - Succès avec code TOTP
// ============================================================

func TestDisable2FAUsecase_Success_WithTOTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	user2fa, secret := createTestUser2FAWithValidSecret(t, "user-123")
	validCode, err := generateValidTOTPCode(secret)
	assert.NoError(t, err)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	mockUser2FA.EXPECT().
		Disable2FA(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.Disable2FARequest{
		UserID:     "user-123",
		Code:       validCode,
		IsRecovery: false,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Contains(t, response.Message, "2FA désactivée avec succès")
}

// ============================================================
// TESTS : Disable2FAUsecase - Succès avec code de récupération
// ============================================================

func TestDisable2FAUsecase_Success_WithRecoveryCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	user2fa, _ := createTestUser2FAWithValidSecret(t, "user-123")

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		Disable2FA(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.Disable2FARequest{
		UserID:     "user-123",
		Code:       "REC-1111", // Code de récupération valide
		IsRecovery: true,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Contains(t, response.Message, "2FA désactivée avec succès")
}

// ============================================================
// TESTS : RegenerateRecoveryCodesUsecase - Succès complet
// ============================================================

func TestRegenerateRecoveryCodesUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	user2fa, secret := createTestUser2FAWithValidSecret(t, "user-123")
	validCode, err := generateValidTOTPCode(secret)
	assert.NoError(t, err)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	mockUser2FA.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), "user-123", gomock.Any()).
		Return(nil)

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "user-123",
		Code:   validCode,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.RecoveryCodes)
	assert.Contains(t, response.Message, "Codes de récupération régénérés avec succès")
}

// ============================================================
// TESTS : VerifyAndEnable2FAUsecase - Erreurs intermédiaires
// ============================================================

func TestVerifyAndEnable2FAUsecase_UpdateRecoveryCodesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	user2fa, secret := createTestUser2FAWithValidSecret(t, "user-123")
	user2fa.IsEnabled = false
	validCode, err := generateValidTOTPCode(secret)
	assert.NoError(t, err)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	mockUser2FA.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), "user-123", gomock.Any()).
		Return(repository.ErrUser2FANotFound) // Erreur simulée

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   validCode,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update recovery codes")
}

func TestVerifyAndEnable2FAUsecase_Enable2FAError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	user2fa, secret := createTestUser2FAWithValidSecret(t, "user-123")
	user2fa.IsEnabled = false
	validCode, err := generateValidTOTPCode(secret)
	assert.NoError(t, err)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	mockUser2FA.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), "user-123", gomock.Any()).
		Return(nil)

	mockUser2FA.EXPECT().
		Enable2FA(gomock.Any(), "user-123").
		Return(repository.ErrUser2FANotFound) // Erreur simulée

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   validCode,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "enable 2fa")
}

// ============================================================
// TESTS : Disable2FAUsecase - Erreurs intermédiaires
// ============================================================

// ============================================================
// TESTS : RegenerateRecoveryCodesUsecase - Erreurs intermédiaires
// ============================================================

func TestRegenerateRecoveryCodesUsecase_UpdateRecoveryCodesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	user2fa, secret := createTestUser2FAWithValidSecret(t, "user-123")
	validCode, err := generateValidTOTPCode(secret)
	assert.NoError(t, err)

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(user2fa, nil)

	mockUser2FA.EXPECT().
		RecordCodeUsed(gomock.Any(), "user-123", validCode).
		Return(nil)

	mockUser2FA.EXPECT().
		UpdateRecoveryCodes(gomock.Any(), "user-123", gomock.Any()).
		Return(repository.ErrUser2FANotFound) // Erreur simulée

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "user-123",
		Code:   validCode,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update recovery codes")
}
