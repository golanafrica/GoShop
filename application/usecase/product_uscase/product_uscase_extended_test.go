package productuscase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	dto "Goshop/application/dto/product_dto"
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/domain/entity"
	"Goshop/interfaces/utils"
	"Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.25 : TESTS COMPLÉMENTAIRES - PRODUCT USECASES
// ============================================================

// ============================================================
// TESTS : ListProductUsecase (0% -> 100%)
// ============================================================

func TestListProductUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewListProductUsecase(mockRepo, mockTxManager)

	products := []*entity.Product{
		{
			ID:          "p1",
			Name:        "Product 1",
			Description: "Desc 1",
			PriceCents:  1000,
			Stock:       10,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	mockRepo.EXPECT().FindAll(gomock.Any(), 10, 0).Return(products, nil)

	ctx := context.Background()
	result, err := uc.Execute(ctx, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Product 1", result[0].Name)
}

func TestListProductUsecase_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewListProductUsecase(mockRepo, mockTxManager)

	mockRepo.EXPECT().FindAll(gomock.Any(), 10, 0).Return([]*entity.Product{}, nil)

	ctx := context.Background()
	result, err := uc.Execute(ctx, 10, 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestListProductUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewListProductUsecase(mockRepo, mockTxManager)

	mockRepo.EXPECT().FindAll(gomock.Any(), 10, 0).Return(nil, errors.New("db error"))

	ctx := context.Background()
	result, err := uc.Execute(ctx, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// ============================================================
// TESTS : CreateProductUsecase (Branches d'erreur transaction)
// ============================================================

func TestCreateProductUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	ctx := context.Background()
	input := dto.CreateProductRequest{Name: "Test", PriceCents: 100, Stock: 10}
	result, err := uc.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrTransactionBegin, err)
}

func TestCreateProductUsecase_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)
	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	input := dto.CreateProductRequest{Name: "Test", PriceCents: 100, Stock: 10}
	result, err := uc.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductCreateFail, err)
}

func TestCreateProductUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)
	uc := productuscase.NewCreateProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit error"))
	mockTx.EXPECT().Rollback().Return(nil) // ✅ Ajouté : le defer appelle Rollback si err != nil

	ctx := context.Background()
	input := dto.CreateProductRequest{Name: "Test", PriceCents: 100, Stock: 10}
	result, err := uc.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrTransactionCommit, err)
}

// ============================================================
// TESTS : DeleteProductUsecase (Branches d'erreur transaction)
// ============================================================

func TestDeleteProductUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewDeleteProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	ctx := context.Background()
	err := uc.Execute(ctx, "p1")

	assert.Error(t, err)
	assert.Equal(t, utils.ErrTransactionBegin, err)
}

func TestDeleteProductUsecase_DeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)
	uc := productuscase.NewDeleteProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByID(gomock.Any(), "p1").Return(&entity.Product{ID: "p1", Name: "Test"}, nil)
	mockRepoWithTx.EXPECT().Delete(gomock.Any(), "p1").Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	err := uc.Execute(ctx, "p1")

	assert.Error(t, err)
	assert.Equal(t, utils.ErrProductDeleteFail, err)
}

func TestDeleteProductUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)
	uc := productuscase.NewDeleteProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByID(gomock.Any(), "p1").Return(&entity.Product{ID: "p1", Name: "Test"}, nil)
	mockRepoWithTx.EXPECT().Delete(gomock.Any(), "p1").Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit error"))
	mockTx.EXPECT().Rollback().Return(nil) // ✅ Ajouté

	ctx := context.Background()
	err := uc.Execute(ctx, "p1")

	assert.Error(t, err)
	assert.Equal(t, utils.ErrTransactionCommit, err)
}

// ============================================================
// TESTS : UpdateProductUsecase (Branches d'erreur transaction & logChanges)
// ============================================================

func TestUpdateProductUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	ctx := context.Background()
	input := &entity.Product{ID: "p1", Name: "Test", PriceCents: 100, Stock: 10}
	result, err := uc.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrTransactionBegin, err)
}

func TestUpdateProductUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)
	uc := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	existing := &entity.Product{ID: "p1", Name: "Old", PriceCents: 100, Stock: 10, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	updated := &entity.Product{ID: "p1", Name: "New", PriceCents: 200, Stock: 20, CreatedAt: existing.CreatedAt, UpdatedAt: time.Now()}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByID(gomock.Any(), "p1").Return(existing, nil)
	mockRepoWithTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(updated, nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit error"))
	mockTx.EXPECT().Rollback().Return(nil) // ✅ Ajouté

	ctx := context.Background()
	input := &entity.Product{ID: "p1", Name: "New", PriceCents: 200, Stock: 20}
	result, err := uc.Execute(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrTransactionCommit, err)
}

func TestUpdateProductUsecase_NoChangesDetected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)
	uc := productuscase.NewUpdateProductUsecase(mockRepo, mockTxManager)

	now := time.Now()
	existing := &entity.Product{ID: "p1", Name: "Same", Description: "Same", PriceCents: 100, Stock: 10, CreatedAt: now, UpdatedAt: now}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByID(gomock.Any(), "p1").Return(existing, nil)
	mockRepoWithTx.EXPECT().Update(gomock.Any(), gomock.Any()).Return(existing, nil)
	mockTx.EXPECT().Commit().Return(nil)

	ctx := context.Background()
	// Input identique à existing pour déclencher "No changes detected" dans logChanges
	input := &entity.Product{ID: "p1", Name: "Same", Description: "Same", PriceCents: 100, Stock: 10}

	result, err := uc.Execute(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// ============================================================
// TESTS : GetProductByIdUsecase (Branche manquante)
// ============================================================

func TestGetProductByIdUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	uc := productuscase.NewGetProductByIdUsecase(mockRepo, mockTxManager)

	mockRepo.EXPECT().FindByID(gomock.Any(), "p1").Return(nil, errors.New("not found"))

	ctx := context.Background()
	result, err := uc.Execute(ctx, "p1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, utils.ErrProductNotFound, err)
}
