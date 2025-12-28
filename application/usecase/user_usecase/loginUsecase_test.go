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

func TestLoginUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,                         // 1er paramètre: repo
		setupLogging.GetTestLogger(), // 2ème paramètre: logger (DERNIER)
	)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	fakeUser := &userentity.UserEntity{
		ID:       "123",
		Email:    "test@example.com",
		Password: string(hashedPassword),
	}

	repo.EXPECT().
		FindUserByEmail("test@example.com").
		Return(fakeUser, nil)

	token, err := uc.Execute(createContextWithLogger(), "test@example.com", "password")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
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

	_, err := uc.Execute(createContextWithLogger(), "unknown@mail.com", "xxx")

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}

func TestLoginUsecase_InvalidPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewLoginUsecase(
		repo,                         // 1er paramètre: repo
		setupLogging.GetTestLogger(), // 2ème paramètre: logger
	)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	fakeUser := &userentity.UserEntity{
		ID:       "123",
		Email:    "test@example.com",
		Password: string(hashedPassword),
	}

	repo.EXPECT().
		FindUserByEmail("test@example.com").
		Return(fakeUser, nil)

	_, err := uc.Execute(createContextWithLogger(), "test@example.com", "wrongpassword")

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInvalidCredentials, err)
}
