package productuscase_test

import (
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

func TestUpdateProductUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	ctx := context.Background()

	existing := &entity.Product{
		ID:          "product-123",
		Name:        "Old Name",
		Description: "Old Desc",
		PriceCents:  10000,
		Stock:       5,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	updated := &entity.Product{
		ID:          "product-123",
		Name:        "New Product",
		Description: "New Desc",
		PriceCents:  20000,
		Stock:       10,
		CreatedAt:   existing.CreatedAt,
		UpdatedAt:   time.Now(),
	}

	// Expectations
	mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)

	mockRepoWithTx.EXPECT().FindByID(ctx, "product-123").Return(existing, nil).Times(1)
	mockRepoWithTx.EXPECT().Update(ctx, gomock.Any()).Return(updated, nil).Times(1)

	mockTx.EXPECT().Commit().Return(nil).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// ✅ Utilisez le constructeur avec logger
	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	// ✅ Prend *entity.Product comme paramètre
	input := &entity.Product{
		ID:          "product-123",
		Name:        "New Product",
		Description: "New Desc",
		PriceCents:  20000,
		Stock:       10,
	}

	result, err := usecase.Execute(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, updated.Name, result.Name)
}

func TestUpdateProductUsecase_EmptyName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ SUPPRIMEZ ces mocks - la validation échoue avant la transaction
	// mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil)
	// mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	// mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	input := &entity.Product{
		ID:         "any",
		Name:       "", // Nom vide
		PriceCents: 20000,
		Stock:      10,
	}

	result, err := usecase.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductInvalidName, err) // ✅ Utilisez l'erreur exacte
}

func TestUpdateProductUsecase_NegativePrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ SUPPRIMEZ ces mocks
	// mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil)
	// mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	// mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	input := &entity.Product{
		ID:         "p1",
		Name:       "Test",
		PriceCents: -100, // Prix négatif
		Stock:      10,
	}

	result, err := usecase.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductInvalidPrice, err) // ✅ Utilisez l'erreur exacte
}

func TestUpdateProductUsecase_NegativeStock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ SUPPRIMEZ ces mocks
	// mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil)
	// mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	// mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	input := &entity.Product{
		ID:         "p1",
		Name:       "Test",
		PriceCents: 100,
		Stock:      -5, // Stock négatif
	}

	result, err := usecase.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductInvalidStock, err) // ✅ Utilisez l'erreur exacte
}

func TestUpdateProductUsecase_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	// ❌ SUPPRIMEZ ces mocks
	// mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil)
	// mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	// mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	input := &entity.Product{
		ID:         "", // ID vide
		Name:       "Test",
		PriceCents: 100,
		Stock:      10,
	}

	result, err := usecase.Execute(context.Background(), input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductNotFound, err) // ✅ Selon votre code
}

func TestUpdateProductUsecase_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	ctx := context.Background()

	// ✅ GARDEZ ces mocks car la validation passe mais FindByID échoue
	mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)
	mockRepoWithTx.EXPECT().FindByID(ctx, "p1").Return(nil, errors.New("not found")).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	input := &entity.Product{
		ID:         "p1",
		Name:       "Test",
		PriceCents: 100,
		Stock:      10,
	}

	result, err := usecase.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductNotFound, err) // ✅ Utilisez l'erreur exacte
}

func TestUpdateProductUsecase_RepositoryUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	ctx := context.Background()

	existing := &entity.Product{
		ID:         "p1",
		Name:       "Old Name",
		PriceCents: 5000,
		Stock:      5,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// ✅ GARDEZ ces mocks car la validation passe mais Update échoue
	mockTxManager.EXPECT().BeginTx(ctx).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)
	mockRepoWithTx.EXPECT().FindByID(ctx, "p1").Return(existing, nil).Times(1)
	mockRepoWithTx.EXPECT().Update(ctx, gomock.Any()).Return(nil, errors.New("db error")).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	usecase := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	input := &entity.Product{
		ID:         "p1",
		Name:       "Test",
		PriceCents: 100,
		Stock:      10,
	}

	result, err := usecase.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductUpdateFail, err) // ✅ Utilisez l'erreur exacte
}
