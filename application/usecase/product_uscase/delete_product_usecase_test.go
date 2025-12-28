package productuscase_test

import (
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/domain/entity"
	"Goshop/mocks/repository"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestDeleteProductUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	// Setup des mocks dans l'ordre correct
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)

	// ✅ Utilisez entity.Product (et non domain.Product)
	mockRepoWithTx.EXPECT().FindByID(gomock.Any(), "ID123").Return(
		&entity.Product{
			ID:   "ID123",
			Name: "Test Product",
		}, nil,
	).Times(1)

	mockRepoWithTx.EXPECT().Delete(gomock.Any(), "ID123").Return(nil).Times(1)

	mockTx.EXPECT().Commit().Return(nil).Times(1)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// ❗ Vérifiez que cette fonction existe
	// Si elle n'existe pas, utilisez NewDeleteProductUsecase
	uc := productuscase.NewDeleteProductUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "ID123")

	assert.NoError(t, err)
}

func TestDeleteProductUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)
	mockTx := repository.NewMockTx(ctrl)
	mockRepoWithTx := repository.NewMockProductRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).Times(1)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx).Times(1)

	// ✅ Utilisez entity.Product
	mockRepoWithTx.EXPECT().FindByID(gomock.Any(), "ID123").Return(
		&entity.Product{
			ID:   "ID123",
			Name: "Test Product",
		}, nil,
	).Times(1)

	mockRepoWithTx.EXPECT().Delete(gomock.Any(), "ID123").
		Return(errors.New("db failure")).Times(1)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := productuscase.NewDeleteProductUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "ID123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to delete product")
}

func TestDeleteProductUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repository.NewMockProductRepository(ctrl)
	mockTxManager := repository.NewMockTxManager(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).
		Return(nil, errors.New("cannot start tx")).Times(1)

	uc := productuscase.NewDeleteProductUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "ID123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to begin transaction") // CORRIGÃ‰
}
