package userusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

// ============================================================
// 🆕 v4.4.20 : TESTS COMPLÉMENTAIRES - LOGIN & REGISTER
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createContextWithLoggerForExtended() context.Context {
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	return logger.WithContext(context.Background())
}

func createFakeUserForExtended(id, email, password string) *userentity.UserEntity {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &userentity.UserEntity{
		ID:       id,
		Email:    email,
		Password: string(hashedPassword),
		Role:     userentity.RoleMerchant,
		Active:   true,
		Status:   userentity.StatusActive,
	}
}

func createLockedUser(id, email, password string) *userentity.UserEntity {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	lockedUntil := time.Now().Add(1 * time.Hour)
	return &userentity.UserEntity{
		ID:          id,
		Email:       email,
		Password:    string(hashedPassword),
		Role:        userentity.RoleMerchant,
		Active:      true,
		Status:      userentity.StatusActive,
		LockedUntil: &lockedUntil,
	}
}

func createInactiveUser(id, email, password string) *userentity.UserEntity {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &userentity.UserEntity{
		ID:       id,
		Email:    email,
		Password: string(hashedPassword),
		Role:     userentity.RoleMerchant,
		Active:   false,
		Status:   userentity.StatusSuspended,
	}
}

// ============================================================
// TESTS : LoginUsecase.ExecuteWithContext - Branches d'erreur
// ============================================================

func TestLoginUsecase_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	repo.EXPECT().
		FindUserByEmail("unknown@mail.com").
		Return(nil, userrepository.ErrUserNotFound)

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "unknown@mail.com", "password123", "127.0.0.1", "TestAgent")

	assert.Error(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", refreshToken)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestLoginUsecase_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(nil, errors.New("database connection failed"))

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "password123", "127.0.0.1", "TestAgent")

	assert.Error(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", refreshToken)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestLoginUsecase_CanLoginError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// User inactif (CanLogin() échoue)
	inactiveUser := createInactiveUser("user-123", "john@mail.com", "password123")
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(inactiveUser, nil)

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "password123", "127.0.0.1", "TestAgent")

	assert.Error(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", refreshToken)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestLoginUsecase_AccountLocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// User verrouillé
	lockedUser := createLockedUser("user-123", "john@mail.com", "password123")
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(lockedUser, nil)

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "password123", "127.0.0.1", "TestAgent")

	assert.Error(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", refreshToken)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestLoginUsecase_InvalidPassword_WithContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// User avec mot de passe différent
	user := createFakeUserForExtended("user-123", "john@mail.com", "correct-password")
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(user, nil)

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "wrong-password", "127.0.0.1", "TestAgent")

	assert.Error(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", refreshToken)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestLoginUsecase_Success_WithSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	user := createFakeUserForExtended("user-123", "john@mail.com", "password123")
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(user, nil)

	// Session creation réussie
	sessionRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "password123", "127.0.0.1", "TestAgent")

	assert.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
}

func TestLoginUsecase_Success_WithoutSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	user := createFakeUserForExtended("user-123", "john@mail.com", "password123")
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(user, nil)

	// sessionRepo = nil (pas de session)
	uc := userusecase.NewLoginUsecase(repo, nil, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "password123", "127.0.0.1", "TestAgent")

	assert.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
}

func TestLoginUsecase_SessionCreationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	user := createFakeUserForExtended("user-123", "john@mail.com", "password123")
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(user, nil)

	// Session creation échoue (non bloquant)
	sessionRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "john@mail.com", "password123", "127.0.0.1", "TestAgent")

	// ✅ Continue malgré l'erreur de session
	assert.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
}

// ============================================================
// TESTS : RegisterUsecase.Execute - Branches d'erreur
// ============================================================

func TestRegisterUsecase_UserAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// User existe déjà
	existingUser := &userentity.UserEntity{
		ID:    "existing-id",
		Email: "john@mail.com",
	}
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(existingUser, nil)

	uc := userusecase.NewRegisterUsecase(repo, logger)

	ctx := createContextWithLoggerForExtended()
	user, err := uc.Execute(ctx, "john@mail.com", "password123")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrUserAlreadyExists, err)
}

func TestRegisterUsecase_DBErrorOnCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// Erreur DB (pas ErrUserNotFound)
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(nil, errors.New("database connection failed"))

	uc := userusecase.NewRegisterUsecase(repo, logger)

	ctx := createContextWithLoggerForExtended()
	user, err := uc.Execute(ctx, "john@mail.com", "password123")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestRegisterUsecase_CreateError_RaceCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// Email disponible
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(nil, userrepository.ErrUserNotFound)

	// CreateUser échoue avec ErrUserAlreadyExists (race condition)
	repo.EXPECT().
		CreateUser(gomock.Any()).
		Return(nil, userrepository.ErrUserAlreadyExists)

	uc := userusecase.NewRegisterUsecase(repo, logger)

	ctx := createContextWithLoggerForExtended()
	user, err := uc.Execute(ctx, "john@mail.com", "password123")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrUserAlreadyExists, err)
}

func TestRegisterUsecase_CreateError_Other(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	// Email disponible
	repo.EXPECT().
		FindUserByEmail("john@mail.com").
		Return(nil, userrepository.ErrUserNotFound)

	// CreateUser échoue avec autre erreur
	repo.EXPECT().
		CreateUser(gomock.Any()).
		Return(nil, errors.New("database connection failed"))

	uc := userusecase.NewRegisterUsecase(repo, logger)

	ctx := createContextWithLoggerForExtended()
	user, err := uc.Execute(ctx, "john@mail.com", "password123")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, utils.ErrInternalServer, err)
}

// ============================================================
// TESTS COMPLÉMENTAIRES - EDGE CASES
// ============================================================

func TestLoginUsecase_EmptyEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	sessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	repo.EXPECT().
		FindUserByEmail("").
		Return(nil, userrepository.ErrUserNotFound)

	uc := userusecase.NewLoginUsecase(repo, sessionRepo, nil, logger)

	ctx := createContextWithLoggerForExtended()
	accessToken, refreshToken, err := uc.ExecuteWithContext(ctx, "", "password123", "127.0.0.1", "TestAgent")

	assert.Error(t, err)
	assert.Equal(t, "", accessToken)
	assert.Equal(t, "", refreshToken)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestRegisterUsecase_EmptyEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	logger := setupLogging.GetTestLogger()

	repo.EXPECT().
		FindUserByEmail("").
		Return(nil, userrepository.ErrUserNotFound)

	repo.EXPECT().
		CreateUser(gomock.Any()).
		Return(&userentity.UserEntity{
			ID:    "123",
			Email: "",
		}, nil)

	uc := userusecase.NewRegisterUsecase(repo, logger)

	ctx := createContextWithLoggerForExtended()
	user, err := uc.Execute(ctx, "", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, user)
}
