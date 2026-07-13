package userusecase_test

import (
	"context"
	"errors"
	"testing"

	userusecase "Goshop/application/usecase/user_usecase"
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"
	mockrepo "Goshop/mocks/repository"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.20 : TESTS UNITAIRES - GET PROFILE USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createContextWithLoggerForProfile() context.Context {
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	return logger.WithContext(context.Background())
}

// ============================================================
// TESTS : GetProfileUsecase.Execute()
// ============================================================

func TestGetProfileUsecase_EmptyUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewGetProfileUsecase(repo)

	user, err := uc.Execute("")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestGetProfileUsecase_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewGetProfileUsecase(repo)

	repo.EXPECT().
		FindUserByID("unknown-id").
		Return(nil, userrepository.ErrUserNotFound)

	user, err := uc.Execute("unknown-id")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrUserNotFound, err)
}

func TestGetProfileUsecase_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewGetProfileUsecase(repo)

	repo.EXPECT().
		FindUserByID("user-123").
		Return(nil, errors.New("database connection failed"))

	user, err := uc.Execute("user-123")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestGetProfileUsecase_NilUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewGetProfileUsecase(repo)

	// Repository retourne nil sans erreur
	repo.EXPECT().
		FindUserByID("user-123").
		Return(nil, nil)

	user, err := uc.Execute("user-123")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestGetProfileUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewGetProfileUsecase(repo)

	expectedUser := &userentity.UserEntity{
		ID:     "user-123",
		Email:  "john@example.com",
		Role:   userentity.RoleMerchant,
		Active: true,
		Status: userentity.StatusActive,
	}

	repo.EXPECT().
		FindUserByID("user-123").
		Return(expectedUser, nil)

	user, err := uc.Execute("user-123")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "user-123", user.ID)
	assert.Equal(t, "john@example.com", user.Email)
}

func TestGetProfileUsecase_ExecuteWithContext_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewGetProfileUsecase(repo)

	ctx := createContextWithLoggerForProfile()
	expectedUser := &userentity.UserEntity{
		ID:     "user-123",
		Email:  "john@example.com",
		Role:   userentity.RoleMerchant,
		Active: true,
		Status: userentity.StatusActive,
	}

	repo.EXPECT().
		FindUserByID("user-123").
		Return(expectedUser, nil)

	user, err := uc.ExecuteWithContext(ctx, "user-123")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "user-123", user.ID)
}
