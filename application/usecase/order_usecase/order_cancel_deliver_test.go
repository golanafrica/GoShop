package orderusecase_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

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
// 🆕 v4.4.17 : TESTS UNITAIRES - CANCEL & DELIVER ORDER USECASES
// ============================================================

// ============================================================
// TESTS : CancelOrderUsecase
// ============================================================

func TestCancelOrderUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

	ctx := context.Background()

	order, err := uc.Execute(ctx, "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestCancelOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestCancelOrderUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

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

func TestCancelOrderUsecase_InvalidStatusTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

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

func TestCancelOrderUsecase_UpdateOrderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createTestOrder(string(entity.OrderStatusPending))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to update order")
}

func TestCancelOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	pendingOrder := createTestOrder(string(entity.OrderStatusPending))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	order, err := uc.Execute(ctx, "order-1")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestCancelOrderUsecase_Success_MobileMoney(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	mobileOrder := createTestOrder(string(entity.OrderStatusPending))
	mobileOrder.PaymentMethod = string(entity.PaymentMethodMobileMoney)
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(mobileOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	order, err := uc.Execute(ctx, "order-1")

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusCancelled), order.Status)
	assert.NotNil(t, order.CancelledAt)
}

func TestCancelOrderUsecase_Success_Cash_PendingConfirmation_WithRestoreStock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

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

	order, err := uc.Execute(ctx, "order-1")

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusCancelled), order.Status)
}

func TestCancelOrderUsecase_Success_Cash_Confirmed_NoRestoreStock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewCancelOrderUsecase(orderRepo, productRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()

	confirmedOrder := createCashOrder(string(entity.OrderStatusConfirmed))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(confirmedOrder, nil)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	order, err := uc.Execute(ctx, "order-1")

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusCancelled), order.Status)
}

// ============================================================
// TESTS : DeliverOrderUsecase
// ============================================================

func TestDeliverOrderUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	ctx := context.Background()
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestDeliverOrderUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestDeliverOrderUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo).AnyTimes()
	shopRepo.EXPECT().WithTX(mockTx).Return(shopRepo).AnyTimes()
	escrowRepo.EXPECT().WithTX(mockTx).Return(escrowRepo).AnyTimes()
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletTxnRepo.EXPECT().WithTX(mockTx).Return(walletTxnRepo).AnyTimes()

	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(nil, sql.ErrNoRows)

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to find order")
}

func TestDeliverOrderUsecase_NotCashOnDelivery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo).AnyTimes()
	shopRepo.EXPECT().WithTX(mockTx).Return(shopRepo).AnyTimes()
	escrowRepo.EXPECT().WithTX(mockTx).Return(escrowRepo).AnyTimes()
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletTxnRepo.EXPECT().WithTX(mockTx).Return(walletTxnRepo).AnyTimes()

	mobileOrder := createTestOrder(string(entity.OrderStatusOutForDelivery))
	mobileOrder.PaymentMethod = string(entity.PaymentMethodMobileMoney)
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(mobileOrder, nil)

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "not cash on delivery")
}

func TestDeliverOrderUsecase_InvalidStatusTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo).AnyTimes()
	shopRepo.EXPECT().WithTX(mockTx).Return(shopRepo).AnyTimes()
	escrowRepo.EXPECT().WithTX(mockTx).Return(escrowRepo).AnyTimes()
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletTxnRepo.EXPECT().WithTX(mockTx).Return(walletTxnRepo).AnyTimes()

	pendingOrder := createCashOrder(string(entity.OrderStatusPending))
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(pendingOrder, nil)

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestDeliverOrderUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo).AnyTimes()
	shopRepo.EXPECT().WithTX(mockTx).Return(shopRepo).AnyTimes()
	escrowRepo.EXPECT().WithTX(mockTx).Return(escrowRepo).AnyTimes()
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletTxnRepo.EXPECT().WithTX(mockTx).Return(walletTxnRepo).AnyTimes()

	outForDeliveryOrder := createCashOrder(string(entity.OrderStatusOutForDelivery))
	outForDeliveryOrder.ID = uuid.New().String()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(outForDeliveryOrder, nil)

	settings := &entity.ShopPaymentSettings{
		ShopID:                shop.ID,
		CashCommissionRate:    250,
		CashOnDeliveryEnabled: true,
	}
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shop.ID).Return(settings, nil)

	paymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	// Simule l'absence d'escrow pour ce test (le code gère gracieusement cette absence)
	escrowRepo.EXPECT().FindByOrderID(gomock.Any(), outForDeliveryOrder.ID).Return(nil, sql.ErrNoRows)

	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	notifService.EXPECT().NotifyClientOrderDelivered(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	notifService.EXPECT().NotifyMerchantCommissionPaid(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	order, err := uc.Execute(ctx, "order-1", req)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, string(entity.OrderStatusDelivered), order.Status)
	assert.NotNil(t, order.DeliveredAt)
	assert.NotNil(t, order.AmountReceivedCents)
	assert.Equal(t, int64(50000), *order.AmountReceivedCents)
	assert.NotNil(t, order.DeliveryNotes)
	assert.Equal(t, "Livré avec succès", *order.DeliveryNotes)
}

func TestDeliverOrderUsecase_UpdateOrderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo).AnyTimes()
	shopRepo.EXPECT().WithTX(mockTx).Return(shopRepo).AnyTimes()
	escrowRepo.EXPECT().WithTX(mockTx).Return(escrowRepo).AnyTimes()
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletTxnRepo.EXPECT().WithTX(mockTx).Return(walletTxnRepo).AnyTimes()

	outForDeliveryOrder := createCashOrder(string(entity.OrderStatusOutForDelivery))
	outForDeliveryOrder.ID = uuid.New().String()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(outForDeliveryOrder, nil)

	settings := &entity.ShopPaymentSettings{
		ShopID:                shop.ID,
		CashCommissionRate:    250,
		CashOnDeliveryEnabled: true,
	}
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shop.ID).Return(settings, nil)

	paymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	escrowRepo.EXPECT().FindByOrderID(gomock.Any(), outForDeliveryOrder.ID).Return(nil, sql.ErrNoRows)

	// ✅ SIMULER L'ERREUR SUR UpdateOrder
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(errors.New("db connection lost"))

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to update order")
}

func TestDeliverOrderUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderRepo := mockrepo.NewMockOrderRepository(ctrl)
	paymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	walletTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewDeliverOrderUsecase(orderRepo, paymentRepo, shopRepo, escrowRepo, walletRepo, walletTxnRepo, notifService, txManager)

	shop := createTestShopForOrder()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)
	req := &orderusecase.DeliverRequest{
		AmountReceived: 50000,
		Notes:          "Livré avec succès",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	orderRepo.EXPECT().WithTX(mockTx).Return(orderRepo).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo).AnyTimes()
	shopRepo.EXPECT().WithTX(mockTx).Return(shopRepo).AnyTimes()
	escrowRepo.EXPECT().WithTX(mockTx).Return(escrowRepo).AnyTimes()
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletTxnRepo.EXPECT().WithTX(mockTx).Return(walletTxnRepo).AnyTimes()

	outForDeliveryOrder := createCashOrder(string(entity.OrderStatusOutForDelivery))
	outForDeliveryOrder.ID = uuid.New().String()
	orderRepo.EXPECT().FindByID(gomock.Any(), "order-1").Return(outForDeliveryOrder, nil)

	settings := &entity.ShopPaymentSettings{
		ShopID:                shop.ID,
		CashCommissionRate:    250,
		CashOnDeliveryEnabled: true,
	}
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shop.ID).Return(settings, nil)

	paymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	escrowRepo.EXPECT().FindByOrderID(gomock.Any(), outForDeliveryOrder.ID).Return(nil, sql.ErrNoRows)
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(nil)

	// ✅ SIMULER L'ERREUR SUR Commit
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	order, err := uc.Execute(ctx, "order-1", req)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

// ============================================================
// TESTS MANQUANTS : RejectOrderUsecase
// ============================================================

func TestRejectOrderUsecase_UpdateOrderError(t *testing.T) {
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

	// ✅ SIMULER L'ERREUR SUR UpdateOrder
	orderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(errors.New("db connection lost"))

	order, err := uc.Execute(ctx, "order-1", "Stock insuffisant")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "failed to update order")
}
