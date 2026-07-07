package twofausecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	twofausecase "Goshop/application/usecase/twofa_usecase"
	"Goshop/domain/entity"
	userentity "Goshop/domain/entity/user_entity"
	"Goshop/domain/repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.3 : TESTS UNITAIRES - USECASES 2FA
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createAdminContext() *twofausecase.AdminContext {
	return &twofausecase.AdminContext{
		AdminID:    "user-123",
		AdminEmail: "admin@example.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
	}
}

func createTestUser2FA() *entity.User2FA {
	return &entity.User2FA{
		ID:                     "2fa-123",
		UserID:                 "user-123",
		SecretEncrypted:        "encrypted-secret",
		IsEnabled:              true,
		RecoveryCodesEncrypted: "encrypted-codes",
		RecoveryCodesUsed:      []int{},
		FailedAttempts:         0,
	}
}

// ============================================================
// TESTS : Get2FAStatusUsecase
// ============================================================

func TestGet2FAStatusUsecase_NotSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "user-123", response.UserID)
	assert.False(t, response.IsEnabled)
	assert.False(t, response.IsSetup)
	assert.False(t, response.IsLocked)
	assert.Equal(t, "not_setup", response.Status)
}

func TestGet2FAStatusUsecase_Enabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()
	test2FA := createTestUser2FA()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.True(t, response.IsEnabled)
	assert.True(t, response.IsSetup)
	assert.False(t, response.IsLocked)
	assert.Equal(t, 0, response.FailedAttempts)
}

func TestGet2FAStatusUsecase_Locked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	lockTime := time.Now().Add(10 * time.Minute)
	test2FA := &entity.User2FA{
		ID:              "2fa-123",
		UserID:          "user-123",
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       true,
		FailedAttempts:  5,
		LockedUntil:     &lockTime,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.IsLocked)
	assert.Equal(t, 5, response.FailedAttempts)
	assert.NotNil(t, response.LockedUntil)
}

func TestGet2FAStatusUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "user-123", response.UserID)
}

func TestGet2FAStatusUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, errors.New("database connection error"))

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "database connection error")
}

func TestGet2FAStatusUsecase_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, context.Canceled)

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(ctx, adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.True(t, errors.Is(err, context.Canceled),
		"Error should wrap context.Canceled, got: %v", err)

}

func TestGet2FAStatusUsecase_Performance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewGet2FAStatusUsecase(mockUser2FA)
	adminCtx := createAdminContext()
	test2FA := createTestUser2FA()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.Get2FAStatusRequest{
		UserID: "user-123",
	}

	start := time.Now()
	response, err := uc.Execute(context.Background(), adminCtx, req)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, duration < 100*time.Millisecond,
		"GetStatus should complete in less than 100ms, got %v", duration)
}

// ============================================================
// TESTS : Setup2FAUsecase
// ============================================================

func TestSetup2FAUsecase_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)

	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminContext()

	mockUser.EXPECT().
		FindUserByID("unknown-user").
		Return(nil, errors.New("user not found"))

	req := &twofausecase.Setup2FARequest{
		UserID: "unknown-user",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "user not found")
}

func TestSetup2FAUsecase_AlreadyEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	mockUser := mockrepo.NewMockUserRepository(ctrl)

	uc := twofausecase.NewSetup2FAUsecase(mockUser2FA, mockUser)
	adminCtx := createAdminContext()

	mockUser.EXPECT().
		FindUserByID("user-123").
		Return(&userentity.UserEntity{
			ID:    "user-123",
			Email: "test@example.com",
		}, nil)

	mockUser2FA.EXPECT().
		Exists(gomock.Any(), "user-123").
		Return(true, nil)

	req := &twofausecase.Setup2FARequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FAAlreadyEnabled, err)
}

// ============================================================
// TESTS : Disable2FAUsecase
// ============================================================

func TestDisable2FAUsecase_NotSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.Disable2FARequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotSetup, err)
}

func TestDisable2FAUsecase_NotEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewDisable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	test2FA := &entity.User2FA{
		ID:        "2fa-123",
		UserID:    "user-123",
		IsEnabled: false,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.Disable2FARequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotEnabled, err)
}

// ============================================================
// TESTS : RegenerateRecoveryCodesUsecase
// ============================================================

func TestRegenerateRecoveryCodesUsecase_NotSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotSetup, err)
}

func TestRegenerateRecoveryCodesUsecase_NotEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	test2FA := &entity.User2FA{
		ID:        "2fa-123",
		UserID:    "user-123",
		IsEnabled: false,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotEnabled, err)
}

func TestRegenerateRecoveryCodesUsecase_InvalidCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewRegenerateRecoveryCodesUsecase(mockUser2FA)
	adminCtx := createAdminContext()
	test2FA := createTestUser2FA()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.RegenerateRecoveryCodesRequest{
		UserID: "user-123",
		Code:   "123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FAInvalidCode, err)
}

// ============================================================
// TESTS : VerifyAndEnable2FAUsecase
// ============================================================

func TestVerifyAndEnable2FAUsecase_NotSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, repository.ErrUser2FANotFound)

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FANotSetup, err)
}

func TestVerifyAndEnable2FAUsecase_InvalidCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   "123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FAInvalidCode, err)
}

func TestVerifyAndEnable2FAUsecase_AccountLocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUser2FA := mockrepo.NewMockUser2FARepository(ctrl)
	uc := twofausecase.NewVerifyAndEnable2FAUsecase(mockUser2FA)
	adminCtx := createAdminContext()

	lockTime := time.Now().Add(10 * time.Minute)
	test2FA := &entity.User2FA{
		ID:              "2fa-123",
		UserID:          "user-123",
		SecretEncrypted: "encrypted-secret",
		IsEnabled:       false,
		FailedAttempts:  5,
		LockedUntil:     &lockTime,
	}

	mockUser2FA.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(test2FA, nil)

	req := &twofausecase.VerifyAndEnable2FARequest{
		UserID: "user-123",
		Code:   "123456",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.Err2FAAccountLocked, err)
}
