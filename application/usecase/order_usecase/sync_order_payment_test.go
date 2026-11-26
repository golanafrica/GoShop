package orderusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

func createSyncTestShop() *entity.Shop {
	return &entity.Shop{
		ID:       uuid.New(),
		Name:     "Sync Test Shop",
		Slug:     "sync-test-shop",
		IsActive: true,
	}
}

func createSyncTestOrder(shopID string, status string) *entity.Order {
	return &entity.Order{
		ID:            uuid.New().String(),
		ShopID:        shopID,
		CustomerID:    "customer-sync-1",
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

func createSyncTestPayment(orderID uuid.UUID, shopID uuid.UUID, status entity.PaymentStatus) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.Status = status
	if status == entity.PaymentStatusSuccess {
		providerRef := "TXN-SYNC-123"
		payment.ProviderRef = &providerRef
		now := time.Now().UTC()
		payment.CompletedAt = &now
	}
	return payment
}

func createSyncTestEscrow(orderID string) *entity.EscrowAccount {
	now := time.Now().UTC()
	return &entity.EscrowAccount{
		ID:                  uuid.New().String(),
		OrderID:             &orderID,
		SourceType:          entity.EscrowSourceOrder,
		TotalAmountCents:    50000,
		CommissionCents:     1250,
		ReleasedAmountCents: 0,
		Status:              entity.EscrowAccountFundsHeld,
		FundsHeldAt:         now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// ============================================================
// TEST 1 : Payment success + Order pending + Escrow exists → Confirme l'order
// ============================================================
func TestSyncOrderPayment_AlreadySuccess_PendingOrder_ConfirmsOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo, mockPaymentRepo, mockEscrowRepo, mockDeliveryProofRepo,
		mockPaymentRegistry, nil, mockTxManager,
	)

	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusPending))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	ctx := tenant.WithTenant(context.Background(), shop)

	mockOrderRepo.EXPECT().FindByID(gomock.Any(), order.ID).Return(order, nil)

	// 🆕 PR-2 FIX : Utilisation de FindLatestByOrderID au lieu de FindByOrderID
	mockPaymentRepo.EXPECT().FindLatestByOrderID(gomock.Any(), orderUUID).Return(payment, nil)

	mockEscrowRepo.EXPECT().FindByOrderID(gomock.Any(), order.ID).Return(escrow, nil)

	mockOrderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o *entity.Order) error {
			assert.Equal(t, string(entity.OrderStatusConfirmed), o.Status, "Order should be confirmed after sync")
			return nil
		})

	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Synced, "Synced should be true when payment was already confirmed")
	assert.Equal(t, string(entity.OrderStatusConfirmed), resp.OrderStatus)
	assert.Contains(t, resp.Message, "Payment already confirmed, escrow exists")
}

// ============================================================
// TEST 2 : Payment success + Order confirmed + Escrow exists → No-op
// ============================================================
func TestSyncOrderPayment_AlreadySuccess_ConfirmedOrder_NoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo, mockPaymentRepo, mockEscrowRepo, mockDeliveryProofRepo,
		mockPaymentRegistry, nil, mockTxManager,
	)

	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusConfirmed))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	ctx := tenant.WithTenant(context.Background(), shop)

	mockOrderRepo.EXPECT().FindByID(gomock.Any(), order.ID).Return(order, nil)
	mockPaymentRepo.EXPECT().FindLatestByOrderID(gomock.Any(), orderUUID).Return(payment, nil)
	mockEscrowRepo.EXPECT().FindByOrderID(gomock.Any(), order.ID).Return(escrow, nil)
	// UpdateOrder NE doit PAS être appelé

	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Synced, "Synced should be false when order was already confirmed")
	assert.Equal(t, "Payment already confirmed, escrow exists", resp.Message)
}

// ============================================================
// TEST 3 : Payment success + Order out_for_delivery → No-op
// ============================================================
func TestSyncOrderPayment_AlreadySuccess_OutForDeliveryOrder_NoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo, mockPaymentRepo, mockEscrowRepo, mockDeliveryProofRepo,
		mockPaymentRegistry, nil, mockTxManager,
	)

	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusOutForDelivery))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	ctx := tenant.WithTenant(context.Background(), shop)

	mockOrderRepo.EXPECT().FindByID(gomock.Any(), order.ID).Return(order, nil)
	mockPaymentRepo.EXPECT().FindLatestByOrderID(gomock.Any(), orderUUID).Return(payment, nil)
	mockEscrowRepo.EXPECT().FindByOrderID(gomock.Any(), order.ID).Return(escrow, nil)
	// UpdateOrder NE doit PAS être appelé

	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Synced, "Synced should be false for non-pending orders")
	assert.Equal(t, string(entity.OrderStatusOutForDelivery), resp.OrderStatus)
	assert.Equal(t, "Payment already confirmed, escrow exists", resp.Message)
}

// ============================================================
// 🆕 TEST 4 : PR-2 FIX - UpdateOrder échoue → L'erreur est propagée, pas de faux positif
// ============================================================
func TestSyncOrderPayment_UpdateOrderFailure_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo, mockPaymentRepo, mockEscrowRepo, mockDeliveryProofRepo,
		mockPaymentRegistry, nil, mockTxManager,
	)

	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusPending))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	ctx := tenant.WithTenant(context.Background(), shop)

	mockOrderRepo.EXPECT().FindByID(gomock.Any(), order.ID).Return(order, nil)
	mockPaymentRepo.EXPECT().FindLatestByOrderID(gomock.Any(), orderUUID).Return(payment, nil)
	mockEscrowRepo.EXPECT().FindByOrderID(gomock.Any(), order.ID).Return(escrow, nil)

	// 🆕 PR-2 FIX : Simulation d'un échec d'écriture en base
	mockOrderRepo.EXPECT().UpdateOrder(gomock.Any(), gomock.Any()).Return(errors.New("db connection lost"))

	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	// Assertions : L'erreur doit être propagée, pas avalée
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to persist accepted order")
	assert.Nil(t, resp) // Pas de réponse partielle trompeuse
}

// ============================================================
// 🆕 TEST 5 : Aucun paiement trouvé → Erreur explicite
// ============================================================
func TestSyncOrderPayment_NoPaymentsFound_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo, mockPaymentRepo, mockEscrowRepo, mockDeliveryProofRepo,
		mockPaymentRegistry, nil, mockTxManager,
	)

	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusPending))
	orderUUID, _ := uuid.Parse(order.ID)

	ctx := tenant.WithTenant(context.Background(), shop)

	mockOrderRepo.EXPECT().FindByID(gomock.Any(), order.ID).Return(order, nil)
	mockPaymentRepo.EXPECT().FindLatestByOrderID(gomock.Any(), orderUUID).Return(nil, repository.ErrPaymentNotFound)

	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no payment found for order")
	assert.Nil(t, resp)
}
