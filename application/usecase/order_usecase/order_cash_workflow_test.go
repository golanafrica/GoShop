package orderusecase_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"
	mockservice "Goshop/mocks/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.17 : TESTS UNITAIRES - ORDER CASH WORKFLOW
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createCashOrder(status string) *entity.Order {
	reservedUntil := time.Now().Add(24 * time.Hour)
	return &entity.Order{
		ID:            uuid.New().String(),
		ShopID:        uuid.New().String(), // 🆕 v4.8.1 FIX B1 : ShopID obligatoire
		CustomerID:    "customer-1",
		TotalCents:    50000,
		Status:        status,
		PaymentMethod: string(entity.PaymentMethodCashOnDelivery),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ReservedUntil: &reservedUntil,
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

func createExpiredCashOrder() *entity.Order {
	reservedUntil := time.Now().Add(-24 * time.Hour)
	return &entity.Order{
		ID:            uuid.New().String(),
		ShopID:        uuid.New().String(), // 🆕 v4.8.1 FIX B1
		CustomerID:    "customer-1",
		TotalCents:    50000,
		Status:        string(entity.OrderStatusPendingConfirmation),
		PaymentMethod: string(entity.PaymentMethodCashOnDelivery),
		CreatedAt:     time.Now().Add(-48 * time.Hour),
		UpdatedAt:     time.Now().Add(-48 * time.Hour),
		ReservedUntil: &reservedUntil,
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

func createTestShopForOrder() *entity.Shop {
	return &entity.Shop{
		ID:       uuid.New(),
		Name:     "Test Shop",
		IsActive: true,
	}
}

// ============================================================
// TESTS : AcceptOrderUsecase
// ============================================================

func TestAcceptOrderUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	ctx := context.Background()
	order, err := uc.Execute(ctx, "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestAcceptOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestAcceptOrderUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(nil, sql.ErrNoRows)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to find order")
}

func TestAcceptOrderUsecase_NotSupportedPaymentMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	unsupportedOrder := createCashOrder(string(entity.OrderStatusPending))
	unsupportedOrder.PaymentMethod = "card"
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(unsupportedOrder, nil)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "not supported for this flow")
}

func TestAcceptOrderUsecase_OrderExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	expiredOrder := createExpiredCashOrder()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(expiredOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	product := &entity.Product{ID: "product-1", Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "order has expired")
}

func TestAcceptOrderUsecase_InvalidStatusTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	deliveredOrder := createCashOrder(string(entity.OrderStatusDelivered))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(deliveredOrder, nil)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestAcceptOrderUsecase_UpdateOrderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to update order")
}

func TestAcceptOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestAcceptOrderUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	notifService.EXPECT().NotifyClientOrderConfirmed(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	order, err := uc.Execute(ctx, "order-1")
	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusConfirmed), order.Status)
	assert.NotNil(t, order.AcceptedAt)
}

// ============================================================
// TESTS : RejectOrderUsecase
// ============================================================

func TestRejectOrderUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	ctx := context.Background()
	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestRejectOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestRejectOrderUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(nil, sql.ErrNoRows)

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to find order")
}

func TestRejectOrderUsecase_NotCashOnDelivery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	mobileOrder := createCashOrder(string(entity.OrderStatusPending))
	mobileOrder.PaymentMethod = string(entity.PaymentMethodMobileMoney)
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(mobileOrder, nil)

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "not cash on delivery")
}

func TestRejectOrderUsecase_InvalidStatusTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	deliveredOrder := createCashOrder(string(entity.OrderStatusDelivered))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(deliveredOrder, nil)

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestRejectOrderUsecase_RestoreStock_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to find product")
}

func TestRejectOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	product := &entity.Product{ID: "product-1", Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestRejectOrderUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	product := &entity.Product{ID: "product-1", Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *entity.Product) (*entity.Product, error) {
			assert.Equal(t, 12, p.Stock)
			return p, nil
		},
	)

	notifService.EXPECT().NotifyClientOrderRejected(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusRejected), order.Status)
	assert.NotNil(t, order.RejectedAt)
}

// ============================================================
// TESTS : OutForDeliveryUsecase
// ============================================================

func TestOutForDeliveryUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	ctx := context.Background()
	order, err := uc.Execute(ctx, "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestOutForDeliveryUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestOutForDeliveryUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(nil, sql.ErrNoRows)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to find order")
}

func TestOutForDeliveryUsecase_NotSupportedPaymentMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	unsupportedOrder := createCashOrder(string(entity.OrderStatusConfirmed))
	unsupportedOrder.PaymentMethod = "card"
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(unsupportedOrder, nil)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "not supported for this flow")
}

func TestOutForDeliveryUsecase_InvalidStatusTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestOutForDeliveryUsecase_UpdateOrderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	confirmedOrder := createCashOrder(string(entity.OrderStatusConfirmed))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(confirmedOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to update order")
}

func TestOutForDeliveryUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	confirmedOrder := createCashOrder(string(entity.OrderStatusConfirmed))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(confirmedOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestOutForDeliveryUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewOutForDeliveryUsecase(orderRepo, txManager, notifService)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()

	confirmedOrder := createCashOrder(string(entity.OrderStatusConfirmed))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(confirmedOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	order, err := uc.Execute(ctx, "order-1")
	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusOutForDelivery), order.Status)
}

// ============================================================
// 🆕 v4.4.18 : TESTS COMPLÉMENTAIRES - BRANCHES NON COUVERTES
// ============================================================

func TestAcceptOrderUsecase_Expired_RestoreStock_FindProductError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	expiredOrder := createExpiredCashOrder()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(expiredOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(nil, errors.New("product not found"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to restore stock")
	assert.Contains(t, err.Error(), "failed to find product")
}

func TestAcceptOrderUsecase_Expired_RestoreStock_UpdateProductError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	expiredOrder := createExpiredCashOrder()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(expiredOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	product := &entity.Product{ID: "product-1", Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to restore stock")
	assert.Contains(t, err.Error(), "failed to update product")
}

func TestAcceptOrderUsecase_Success_NotificationError_ContinuesAnyway(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewAcceptOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	notifService.EXPECT().NotifyClientOrderConfirmed(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("SMS service down"))

	order, err := uc.Execute(ctx, "order-1")
	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusConfirmed), order.Status)
}

func TestRejectOrderUsecase_RestoreStock_UpdateProductError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	product := &entity.Product{ID: "product-1", Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to update product")
}

func TestRejectOrderUsecase_Success_NotificationError_ContinuesAnyway(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewRejectOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPendingConfirmation))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	product := &entity.Product{ID: "product-1", Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)

	notifService.EXPECT().NotifyClientOrderRejected(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("SMS service down"))

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")
	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusRejected), order.Status)
}

// ============================================================
// TESTS : CreateOrderUsecase - Cash On Delivery branch
// ============================================================

func TestCreateOrderUsecase_Success_CashOnDelivery_WithCODProof(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := mockrepo.NewMockTxManager(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	orderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	codProofRepo := mockrepo.NewMockCODProofRepository(ctrl)

	uc := orderusecase.NewCreateOrderUsecase(
		txManager, productRepo, customerRepo, orderItemRepo, orderRepo, codProofRepo,
	)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()
	orderItemRepo.EXPECT().WithTX(mockTx).Return(orderItemRepo).AnyTimes()
	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	codProofRepo.EXPECT().WithTX(mockTx).Return(codProofRepo).AnyTimes()

	customer := &entity.Customer{ID: uuid.New().String(), FirstName: "John", LastName: "Doe"}
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), gomock.Any()).Return(customer, nil)

	product := &entity.Product{ID: uuid.New().String(), Name: "Test Product", PriceCents: 10000, Stock: 10}
	productRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(product, nil)
	productRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(product, nil)

	createdOrder := &entity.Order{
		ID:            uuid.New().String(),
		ShopID:        shop.ID.String(), // 🆕 v4.8.1 FIX B1
		CustomerID:    customer.ID,
		PaymentMethod: string(entity.PaymentMethodCashOnDelivery),
		TotalCents:    10000,
		Status:        string(entity.OrderStatusPendingConfirmation),
		Items:         []*entity.OrderItem{{ProductID: product.ID, Quantity: 1, PriceCents: 10000, SubTotal_Cents: 10000}},
	}
	orderRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, order *entity.Order) (*entity.Order, error) {
			order.ID = createdOrder.ID
			order.ShopID = shop.ID.String() // 🆕 v4.8.1 FIX B1
			return order, nil
		},
	)
	orderItemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.OrderItem{}, nil)

	codProofRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, proof *entity.CODProof) error {
			assert.Equal(t, createdOrder.ID, proof.OrderID)
			assert.Equal(t, shop.ID.String(), proof.ShopID)
			assert.Equal(t, int64(250), proof.CommissionCents)
			return nil
		},
	)

	order := &entity.Order{
		CustomerID:    customer.ID,
		PaymentMethod: string(entity.PaymentMethodCashOnDelivery),
		Items:         []*entity.OrderItem{{ProductID: product.ID, Quantity: 1}},
	}

	result, err := uc.Execute(ctx, order)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, string(entity.OrderStatusPendingConfirmation), result.Status)
	assert.Equal(t, shop.ID.String(), result.ShopID) // 🆕 v4.8.1 FIX B1
	assert.NotNil(t, result.ReservedUntil)
}

// 🆕 v4.8.1 FIX B1 : Renommé - Sans tenant → erreur (pas de skip COD proof)
func TestCreateOrderUsecase_CashOnDelivery_NoTenant_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txManager := mockrepo.NewMockTxManager(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	orderItemRepo := mockrepo.NewMockOrderItemRepository(ctrl)
	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	codProofRepo := mockrepo.NewMockCODProofRepository(ctrl)

	uc := orderusecase.NewCreateOrderUsecase(
		txManager, productRepo, customerRepo, orderItemRepo, orderRepo, codProofRepo,
	)

	ctx := context.Background() // SANS tenant
	mockTx := mockrepo.NewMockTx(ctrl)

	// L'erreur se produit APRÈS BeginTx et WithTX, mais AVANT Create
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()
	orderItemRepo.EXPECT().WithTX(mockTx).Return(orderItemRepo).AnyTimes()
	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	codProofRepo.EXPECT().WithTX(mockTx).Return(codProofRepo).AnyTimes()

	order := &entity.Order{
		CustomerID:    "customer-1",
		PaymentMethod: string(entity.PaymentMethodCashOnDelivery),
		Items:         []*entity.OrderItem{{ProductID: "product-1", Quantity: 1}},
	}

	result, err := uc.Execute(ctx, order)

	// 🆕 v4.8.1 FIX B1 : L'erreur se produit maintenant
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "multi-tenant")
}
