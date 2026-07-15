package twofausecase_test

import (
	"context"
	"errors"
	"os"
	"testing"

	twofausecase "Goshop/application/usecase/twofa_usecase"
	"Goshop/domain/entity"
	userentity "Goshop/domain/entity/user_entity"
	"Goshop/domain/repository"
	"Goshop/infrastructure/crypto"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.3 : CONFIGURATION DES TESTS
// ============================================================

var validEncryptedSecret string
var validEncryptedRecoveryCodes string

func init() {
	// 1. Définir une clé de chiffrement factice de 32 octets (AES-256)
	os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")

	// 2. Pré-chiffrer un secret valide pour que crypto.Decrypt réussisse
	encrypted, err := crypto.Encrypt("DUMMY_TOTP_SECRET_12345678901234567890")
	if err != nil {
		panic("failed to encrypt test secret: " + err.Error())
	}
	validEncryptedSecret = encrypted

	// 3. Pré-chiffrer des codes de récupération valides
	encryptedCodes, err := crypto.Encrypt(`["REC-1111", "REC-2222", "REC-3333"]`)
	if err != nil {
		panic("failed to encrypt test recovery codes: " + err.Error())
	}
	validEncryptedRecoveryCodes = encryptedCodes
}

// ============================================================
// HELPERS
// ============================================================

func createAdminContextExtended() *twofausecase.AdminContext {
	return &twofausecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@example.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.100",
		UserAgent:  "TestAgent",
	}
}

func createTestUser2FAExtended() *entity.User2FA {
	return &entity.User2FA{
		ID:                     "2fa-123",
		UserID:                 "user-123",
		SecretEncrypted:        validEncryptedSecret, // ✅ Utilise le secret valide chiffré
		IsEnabled:              true,
		RecoveryCodesEncrypted: validEncryptedRecoveryCodes,
		RecoveryCodesUsed:      []int{},
		FailedAttempts:         0,
	}
}

// ============================================================
// TESTS : Setup2FAUsecase (Extended)
// ============================================================

func TestSetup2FAUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)
	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminContextExtended()

	mockUser.EXPECT().
		FindUserByID("admin-123").
		Return(&userentity.UserEntity{ID: "admin-123", Email: "admin@example.com"}, nil)

	mockUser2FA.EXPECT().
		Exists(gomock.Any(), "admin-123").
		Return(false, nil)

	mockUser2FA.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &twofausecase.Setup2FARequest{
		UserID: "", // Should default to admin.AdminID
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Secret)
}

func TestSetup2FAUsecase_UserNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)
	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminContextExtended()

	mockUser.EXPECT().
		FindUserByID("user-123").
		Return(nil, nil) // Returns nil, nil

	req := &twofausecase.Setup2FARequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "user not found")
}

func TestSetup2FAUsecase_ExistsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)
	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminContextExtended()

	mockUser.EXPECT().
		FindUserByID("user-123").
		Return(&userentity.UserEntity{ID: "user-123", Email: "test@example.com"}, nil)

	mockUser2FA.EXPECT().
		Exists(gomock.Any(), "user-123").
		Return(false, errors.New("db error"))

	req := &twofausecase.Setup2FARequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "check 2fa exists")
}

func TestSetup2FAUsecase_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)
	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminContextExtended()

	mockUser.EXPECT().
		FindUserByID("user-123").
		Return(&userentity.UserEntity{ID: "user-123", Email: "test@example.com"}, nil)

	mockUser2FA.EXPECT().
		Exists(gomock.Any(), "user-123").
		Return(false, nil)

	mockUser2FA.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	req := &twofausecase.Setup2FARequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "create 2fa config")
}

// ============================================================
// TESTS : VerifyAndEnable2FAUsecase (Extended)
// ============================================================

func TestVerifyAndEnable2FAUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "admin-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "", // Defaults to admin-123
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotSetup, err)
}

func TestVerifyAndEnable2FAUsecase_FindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, errors.New("db error"))

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find 2fa config")
}

func TestVerifyAndEnable2FAUsecase_InvalidCode_IncrementError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	test2FA := createTestUser2FAExtended()
	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	// totp.Validate échouera car "000000" n'est pas le bon code pour le secret.
	// Le code appellera donc IncrementFailedAttempts.
	mockUser2FA.EXPECT().
		IncrementFailedAttempts(gomock.Any(), "user-123").
		Return(errors.New("db error"))

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   "000000", // Code invalide
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FAInvalidCode, err)
}

// ============================================================
// TESTS : Disable2FAUsecase (Extended)
// ============================================================

func TestDisable2FAUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "admin-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.Disable2FARequest{
		UserID: "",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotSetup, err)
}

func TestDisable2FAUsecase_FindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, errors.New("db error"))

	req := &twofausecase.Disable2FARequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find 2fa config")
}

func TestDisable2FAUsecase_Disable2FAError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	test2FA := createTestUser2FAExtended()
	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	// totp.Validate échouera, donc IncrementFailedAttempts sera appelé
	mockUser2FA.EXPECT().
		IncrementFailedAttempts(gomock.Any(), "user-123").
		Return(nil)

	req := &twofausecase.Disable2FARequest{
		UserID:     "user-123",
		Code:       "000000",
		IsRecovery: false,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FAInvalidCode, err)
}

// ============================================================
// TESTS : Disable2FAUsecase.verifyRecoveryCode (Extended)
// ============================================================

func TestDisable2FAUsecase_VerifyRecoveryCode_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	test2FA := &entity.User2FA{
		UserID:                 "user-123",
		IsEnabled:              true,
		RecoveryCodesEncrypted: "", // Vide !
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.Disable2FARequest{
		UserID:     "user-123",
		Code:       "REC-1234",
		IsRecovery: true,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FARecoveryCodeNotFound, err)
}

func TestDisable2FAUsecase_VerifyRecoveryCode_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	test2FA := &entity.User2FA{
		UserID:                 "user-123",
		IsEnabled:              true,
		RecoveryCodesEncrypted: "invalid-encrypted-string", // Échouera au déchiffrement
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.Disable2FARequest{
		UserID:     "user-123",
		Code:       "REC-1234",
		IsRecovery: true,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "decrypt recovery codes")
}

// ============================================================
// TESTS : RegenerateRecoveryCodesUsecase (Extended)
// ============================================================

func TestRegenerateRecoveryCodesUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "admin-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotSetup, err)
}

func TestRegenerateRecoveryCodesUsecase_FindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContextExtended()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, errors.New("db error"))

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find 2fa config")
}
