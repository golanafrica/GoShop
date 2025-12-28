package productuscase_test

import (
	dto "Goshop/application/dto/product_dto"
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/domain/entity"
	"Goshop/interfaces/utils"
	"Goshop/mocks/repository"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCreateProductUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)
	mockRepoWithTx.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, p *entity.Product) error {
		p.ID = "test-product-123"
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()
		return nil
	}).Times(1)
	mockTx.EXPECT().Commit().Return(nil).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	input := dto.CreateProductRequest{
		Name:        "Laptop Dell",
		Description: "XPS 15",
		PriceCents:  150000,
		Stock:       10,
	}

	response, err := uc.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-product-123", response.ID)
	assert.Equal(t, "Laptop Dell", response.Name)
}

func TestCreateProductUsecase_ValidationError_EmptyName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ Pas de mocks attendus pour les erreurs de validation
	// La transaction n'est pas démarrée quand la validation échoue

	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	input := dto.CreateProductRequest{
		Name:       "", // Nom vide
		PriceCents: 10000,
		Stock:      5,
	}

	response, err := uc.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, utils.ErrProductInvalidName, err)
}

func TestCreateProductUsecase_NegativePrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ Pas de mocks attendus pour les erreurs de validation
	// La transaction n'est pas démarrée quand la validation échoue

	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	input := dto.CreateProductRequest{
		Name:       "Test",
		PriceCents: -100, // Prix négatif
		Stock:      5,
	}

	response, err := uc.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, utils.ErrProductInvalidPrice, err) // ✅ CORRIGÉ
}

func TestCreateProductUsecase_NegativeStock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ Pas de mocks attendus pour les erreurs de validation
	// La transaction n'est pas démarrée quand la validation échoue

	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	input := dto.CreateProductRequest{
		Name:       "Test",
		PriceCents: 10000,
		Stock:      -5, // Stock négatif
	}

	response, err := uc.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, utils.ErrProductInvalidStock, err) // ✅ CORRIGÉ
}

func TestCreateProductUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	// ✅ Mocks attendus car la validation passe mais la création échoue
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)
	mockRepoWithTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("connection lost")).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	input := dto.CreateProductRequest{
		Name:       "Test",
		PriceCents: 10000,
		Stock:      5,
	}

	response, err := uc.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, utils.ErrProductCreateFail, err) // ✅ CORRIGÉ
}
