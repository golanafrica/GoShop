package productuscase_test

import (
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/domain/entity"
	"Goshop/mocks/repository"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestGetAllProductByIdUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	uc := productuscase.NewGetProductByIdUsecaseOld(mockRepo, mockTxManager)

	product := &entity.Product{
		ID:          "p-123",
		Name:        "MacBook Pro",
		Description: "M3 Max",
		PriceCents:  350000,
		Stock:       5,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepo.EXPECT().FindByID(gomock.Any(), "p-123").Return(product, nil).Times(1)

	result, err := uc.Execute(context.Background(), "p-123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "p-123", result.ID)
	assert.Equal(t, "MacBook Pro", result.Name)
}

func TestGetAllProductByIdUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	uc := productuscase.NewGetProductByIdUsecaseOld(mockRepo, mockTxManager)

	mockRepo.EXPECT().
		FindByID(gomock.Any(), "404").
		Return(nil, errors.New("not found")).
		Times(1)

	result, err := uc.Execute(context.Background(), "404")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}
