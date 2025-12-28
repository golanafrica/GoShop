package userusecase_test

import (
	"context"
	"testing"

	userusecase "Goshop/application/usecase/user_usecase"
	"Goshop/config/setupLogging"
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// createContextWithLogger crée un contexte avec un logger silencieux pour les tests
func RecreateContextWithLogger() context.Context {
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	return logger.WithContext(context.Background())
}

func TestRegisterUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	uc := userusecase.NewRegisterUsecase(
		repo,                         // 1er paramètre: repo
		setupLogging.GetTestLogger(), // 2ème paramètre: logger (DERNIER)
	)

	repo.EXPECT().
		FindUserByEmail("new@mail.com").
		Return(nil, userrepository.ErrUserNotFound)

	repo.EXPECT().
		CreateUser(gomock.Any()).
		Return(&userentity.UserEntity{
			ID:    "123",
			Email: "new@mail.com",
		}, nil)

	user, err := uc.Execute(RecreateContextWithLogger(), "new@mail.com", "pass123")
	assert.NoError(t, err)
	assert.Equal(t, "new@mail.com", user.Email)
}
