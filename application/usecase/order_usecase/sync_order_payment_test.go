package orderusecase_test

import (
	"context"
	"testing"
	"time"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 Phase 3 : TESTS UNITAIRES - SYNC ORDER PAYMENT
// ============================================================

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

	// Mocks
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	// Usecase
	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo,
		mockPaymentRepo,
		mockEscrowRepo,
		mockDeliveryProofRepo,
		mockPaymentRegistry,
		nil, // notifService
		mockTxManager,
	)

	// Données de test
	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusPending))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	// Contexte avec tenant
	ctx := tenant.WithTenant(context.Background(), shop)

	// Mocks - Ordre des appels
	// 1. FindByID retourne l'order pending
	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), order.ID).
		Return(order, nil)

	// 2. FindByOrderID retourne le payment success
	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{payment}, nil)

	// 3. FindByOrderID sur escrow retourne l'escrow existant
	mockEscrowRepo.EXPECT().
		FindByOrderID(gomock.Any(), order.ID).
		Return(escrow, nil)

	// 4. ✅ CRITIQUE : UpdateOrder doit être appelé (confirmation de l'order)
	mockOrderRepo.EXPECT().
		UpdateOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o *entity.Order) error {
			// Vérifier que le status a bien été changé en confirmed
			assert.Equal(t, string(entity.OrderStatusConfirmed), o.Status,
				"Order should be confirmed after sync")
			return nil
		})

	// Exécution
	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Synced, "Synced should be true when order was confirmed")
	assert.Equal(t, string(entity.OrderStatusConfirmed), resp.OrderStatus,
		"Response OrderStatus should be confirmed")
	assert.Equal(t, string(entity.PaymentStatusSuccess), resp.PaymentStatus)
	assert.Equal(t, string(entity.EscrowAccountFundsHeld), resp.EscrowStatus)
	assert.Contains(t, resp.Message, "order ensured confirmed")
}

// ============================================================
// TEST 2 : Payment success + Order confirmed + Escrow exists → No-op
// ============================================================

func TestSyncOrderPayment_AlreadySuccess_ConfirmedOrder_NoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mocks
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	// Usecase
	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo,
		mockPaymentRepo,
		mockEscrowRepo,
		mockDeliveryProofRepo,
		mockPaymentRegistry,
		nil, // notifService
		mockTxManager,
	)

	// Données de test - Order DÉJÀ confirmed
	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusConfirmed))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	// Contexte avec tenant
	ctx := tenant.WithTenant(context.Background(), shop)

	// Mocks
	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), order.ID).
		Return(order, nil)

	mockPaymentRepo.EXPECT().
		FindByOrderID(gomock.Any(), orderUUID).
		Return([]*entity.Payment{payment}, nil)

	mockEscrowRepo.EXPECT().
		FindByOrderID(gomock.Any(), order.ID).
		Return(escrow, nil)

	// ✅ CRITIQUE : UpdateOrder NE doit PAS être appelé (déjà confirmed)
	// Pas de mock pour UpdateOrder → gomock échouera si appelé

	// Exécution
	req := &orderusecase.SyncOrderPaymentRequest{OrderID: order.ID}
	resp, err := uc.Execute(ctx, req)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Synced, "Synced should be false when order was already confirmed")
	assert.Equal(t, string(entity.OrderStatusConfirmed), resp.OrderStatus)
	assert.Equal(t, string(entity.PaymentStatusSuccess), resp.PaymentStatus)
	assert.Equal(t, "Payment already confirmed, escrow exists", resp.Message)
}

// ============================================================
// TEST 3 : Payment success + Order out_for_delivery → No-op
// ============================================================

func TestSyncOrderPayment_AlreadySuccess_OutForDeliveryOrder_NoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mocks
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockDeliveryProofRepo := mockrepo.NewMockDeliveryProofRepository(ctrl)
	mockPaymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := orderusecase.NewSyncOrderPaymentUsecase(
		mockOrderRepo,
		mockPaymentRepo,
		mockEscrowRepo,
		mockDeliveryProofRepo,
		mockPaymentRegistry,
		nil,
		mockTxManager,
	)

	shop := createSyncTestShop()
	order := createSyncTestOrder(shop.ID.String(), string(entity.OrderStatusOutForDelivery))
	orderUUID, _ := uuid.Parse(order.ID)
	payment := createSyncTestPayment(orderUUID, shop.ID, entity.PaymentStatusSuccess)
	escrow := createSyncTestEscrow(order.ID)

	ctx := tenant.WithTenant(context.Background(), shop)

	mockOrderRepo.EXPECT().FindByID(gomock.Any(), order.ID).Return(order, nil)
	mockPaymentRepo.EXPECT().FindByOrderID(gomock.Any(), orderUUID).Return([]*entity.Payment{payment}, nil)
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
