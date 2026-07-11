package orderusecase_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.17 : TESTS UNITAIRES - ORDER QUERY USECASES
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createTestOrder(status string) *entity.Order {
	return &entity.Order{
		ID:            uuid.New().String(),
		CustomerID:    "customer-1",
		TotalCents:    50000,
		Status:        status,
		PaymentMethod: string(entity.PaymentMethodMobileMoney),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Items: []*entity.OrderItem{
			{
				ID:             uuid.New().String(),
				ProductID:      "product-1",
				Quantity:       2,
				PriceCents:     25000,
				SubTotal_Cents: 50000,
			},
		},
	}
}

// ============================================================
// TESTS : GetAllOrderUsecase
// ============================================================

func TestGetAllOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewGetAllOrderUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	orders, err := uc.Execute(context.Background())

	assert.Error(t, err)
	assert.Nil(t, orders)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestGetAllOrderUsecase_FindAllError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetAllOrderUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	orderRepo.EXPECT().FindAll(gomock.Any()).Return(nil, errors.New("db error"))

	orders, err := uc.Execute(context.Background())

	assert.Error(t, err)
	assert.Nil(t, orders)
	assert.Contains(t, err.Error(), "failed to fetch orders")
}

func TestGetAllOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetAllOrderUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	orderRepo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Order{}, nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	orders, err := uc.Execute(context.Background())

	assert.Error(t, err)
	assert.Nil(t, orders)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestGetAllOrderUsecase_Success_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetAllOrderUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	orderRepo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Order{}, nil)

	orders, err := uc.Execute(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, orders)
	assert.Len(t, orders, 0)
}

func TestGetAllOrderUsecase_Success_WithOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetAllOrderUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	orders := []*entity.Order{
		createTestOrder(string(entity.OrderStatusPending)),
		createTestOrder(string(entity.OrderStatusDelivered)),
	}
	orderRepo.EXPECT().FindAll(gomock.Any()).Return(orders, nil)

	result, err := uc.Execute(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
}

// ============================================================
// TESTS : GetOrderByIdUsecase
// ============================================================

func TestGetOrderByIdUsecase_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewGetOrderByIdUsecase(orderRepo, txManager)

	order, err := uc.Execute(context.Background(), "")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "empty order ID")
}

func TestGetOrderByIdUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewGetOrderByIdUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(context.Background(), "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestGetOrderByIdUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetOrderByIdUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	// ✅ CORRECTION : Utiliser sql.ErrNoRows directement (pas un string)
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(nil, sql.ErrNoRows)

	order, err := uc.Execute(context.Background(), "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "order not found")
}

func TestGetOrderByIdUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetOrderByIdUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(createTestOrder(string(entity.OrderStatusPending)), nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	order, err := uc.Execute(context.Background(), "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestGetOrderByIdUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := orderusecase.NewGetOrderByIdUsecase(orderRepo, txManager)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	expectedOrder := createTestOrder(string(entity.OrderStatusPending))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(expectedOrder, nil)

	order, err := uc.Execute(context.Background(), "order-1")

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, expectedOrder.ID, order.ID)
	assert.Equal(t, expectedOrder.CustomerID, order.CustomerID)
	assert.Equal(t, expectedOrder.TotalCents, order.TotalCents)
}
