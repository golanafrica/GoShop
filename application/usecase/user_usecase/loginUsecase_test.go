// C:\Users\ifbbu\Desktop\GoShop\application\usecase\user_usecase\loginUsecase_test.go
package userusecase_test

import (
	"context"
	"testing"

	userusecase "Goshop/application/usecase/user_usecase"
	"Goshop/config/setupLogging"
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"
	mockrepo "Goshop/mocks/repository"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

// createContextWithLogger crée un contexte avec un logger silencieux pour les tests
func createContextWithLogger() context.Context {
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	return logger.WithContext(context.Background())
}

// ============================================================
// 🆕 v4.0.0 : Helper pour créer un fakeUser avec RBAC
// ============================================================

func createFakeUser(id, email, password string) *userentity.UserEntity {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &userentity.UserEntity{
		ID:       id,
		Email:    email,
		Password: string(hashedPassword),
		// 🆕 v4.0.0 : Champs RBAC requis pour CanLogin()
		Role:   userentity.RoleMerchant,
		Active: true,
		Status: userentity.StatusActive,
	}
}

// ============================================================
// TESTS
// ============================================================

func TestLoginUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,                         // 1er paramètre: repo
		setupLogging.GetTestLogger(), // 2ème paramètre: logger (DERNIER)
	)

	fakeUser := createFakeUser("123", "test@example.com", "password")

	repo.EXPECT().
		FindUserByEmail("test@example.com").
		Return(fakeUser, nil)

	// 🆕 v4.0.0 : Execute retourne maintenant 3 valeurs
	accessToken, refreshToken, err := uc.Execute(
		createContextWithLogger(),
		"test@example.com",
		"password",
	)

	assert.NoError(t, err)
	assert.NotEmpty(t, accessToken, "Access token should not be empty")
	assert.NotEmpty(t, refreshToken, "Refresh token should not be empty")

	// 🆕 v4.0.0 : Vérifier que les tokens sont différents
	assert.NotEqual(t, accessToken, refreshToken, "Access and refresh tokens should be different")
}

func TestLoginUsecase_EmailNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,                         // 1er paramètre: repo
		setupLogging.GetTestLogger(), // 2ème paramètre: logger
	)

	repo.EXPECT().
		FindUserByEmail("unknown@mail.com").
		Return(nil, userrepository.ErrUserNotFound)

	// 🆕 v4.0.0 : Execute retourne maintenant 3 valeurs
	accessToken, refreshToken, err := uc.Execute(
		createContextWithLogger(),
		"unknown@mail.com",
		"xxx",
	)

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
	assert.Empty(t, accessToken, "Access token should be empty on error")
	assert.Empty(t, refreshToken, "Refresh token should be empty on error")
}

func TestLoginUsecase_InvalidPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,                         // 1er paramètre: repo
		setupLogging.GetTestLogger(), // 2ème paramètre: logger
	)

	fakeUser := createFakeUser("123", "test@example.com", "correctpassword")

	repo.EXPECT().
		FindUserByEmail("test@example.com").
		Return(fakeUser, nil)

	// 🆕 v4.0.0 : Execute retourne maintenant 3 valeurs
	accessToken, refreshToken, err := uc.Execute(
		createContextWithLogger(),
		"test@example.com",
		"wrongpassword",
	)

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
	assert.Empty(t, accessToken, "Access token should be empty on error")
	assert.Empty(t, refreshToken, "Refresh token should be empty on error")
}

// ============================================================
// 🆕 v4.0.0 : Nouveaux tests RBAC
// ============================================================

func TestLoginUsecase_InactiveUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,
		setupLogging.GetTestLogger(),
	)

	// Utilisateur inactif
	inactiveUser := &userentity.UserEntity{
		ID:       "123",
		Email:    "inactive@example.com",
		Password: "$2a$10$dummyhash",
		Role:     userentity.RoleMerchant,
		Active:   false, // ❌ Inactif
		Status:   userentity.StatusSuspended,
	}

	repo.EXPECT().
		FindUserByEmail("inactive@example.com").
		Return(inactiveUser, nil)

	accessToken, refreshToken, err := uc.Execute(
		createContextWithLogger(),
		"inactive@example.com",
		"password",
	)

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
}

func TestLoginUsecase_BannedUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,
		setupLogging.GetTestLogger(),
	)

	// Utilisateur banni
	bannedUser := &userentity.UserEntity{
		ID:       "123",
		Email:    "banned@example.com",
		Password: "$2a$10$dummyhash",
		Role:     userentity.RoleMerchant,
		Active:   false,
		Status:   userentity.StatusBanned, // ❌ Banni
	}

	repo.EXPECT().
		FindUserByEmail("banned@example.com").
		Return(bannedUser, nil)

	accessToken, refreshToken, err := uc.Execute(
		createContextWithLogger(),
		"banned@example.com",
		"password",
	)

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
}
