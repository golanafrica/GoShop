package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// TESTS UNITAIRES - CHECK PAYMENT STATUS USECASE
// ============================================================

func createPendingPayment(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, amountCents)
	return payment
}

func createProcessingPayment(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment := createPendingPayment(shopID, amountCents)
	payment.MarkProcessing()
	providerRef := "TXN-123"
	payment.ProviderRef = &providerRef
	return payment
}

// createTestContextForPayment : list_payments_test.go

func TestCheckPaymentStatusUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := context.Background()
	paymentID := uuid.New().String()

	response, err := uc.Execute(ctx, paymentID)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestCheckPaymentStatusUsecase_InvalidPaymentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()

	response, err := uc.Execute(ctx, "invalid-uuid")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid payment_id")
}

func TestCheckPaymentStatusUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	paymentID := uuid.New()

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentID).
		Return(nil, errors.New("payment not found"))

	response, err := uc.Execute(ctx, paymentID.String())

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "payment not found")
}

func TestCheckPaymentStatusUsecase_TerminalState_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createProcessingPayment(shop.ID, 50000)
	successPayment.MarkSuccess("TXN-123")

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	// ProviderRef présent → Get peut être appelé (fallback provider not found)
	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found")).
		AnyTimes()

	response, err := uc.Execute(ctx, successPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusSuccess, response.Status)
	assert.Equal(t, "TXN-123", response.ProviderRef)
}

func TestCheckPaymentStatusUsecase_TerminalState_Failed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	failedPayment := createProcessingPayment(shop.ID, 50000)
	failedPayment.MarkFailed("Insufficient funds")

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), failedPayment.ID).
		Return(failedPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found")).
		AnyTimes()

	response, err := uc.Execute(ctx, failedPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusFailed, response.Status)
}

func TestCheckPaymentStatusUsecase_TerminalState_Refunded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	refundedPayment := createProcessingPayment(shop.ID, 50000)
	refundedPayment.MarkSuccess("TXN-123")
	refundedPayment.MarkRefunded()

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), refundedPayment.ID).
		Return(refundedPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found")).
		AnyTimes()

	response, err := uc.Execute(ctx, refundedPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusRefunded, response.Status)
}

func TestCheckPaymentStatusUsecase_NonTerminalWithoutProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	pendingPayment := createPendingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), pendingPayment.ID).
		Return(pendingPayment, nil)

	response, err := uc.Execute(ctx, pendingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusPending, response.Status)
	assert.Empty(t, response.ProviderRef)
}

func TestCheckPaymentStatusUsecase_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found"))

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusProcessing, response.Status)
}

func TestCheckPaymentStatusUsecase_ProviderCheckStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("provider unavailable"))

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusProcessing, response.Status)
}

func TestCheckPaymentStatusUsecase_StatusUnchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			Status: entity.PaymentStatusProcessing,
		}, nil)

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusProcessing, response.Status)
}

func TestCheckPaymentStatusUsecase_StatusChangedToSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			ProviderRef: "TXN-123",
			Status:      entity.PaymentStatusSuccess,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	mockOrderRepo.EXPECT().
		UpdateStatus(gomock.Any(), processingPayment.OrderID, "confirmed").
		Return(nil).
		AnyTimes()

	mockEscrowRepo.EXPECT().
		FindByOrderID(gomock.Any(), processingPayment.OrderID.String()).
		Return(nil, errors.New("not found")).
		AnyTimes()

	// ResolveOnlineCommissionBps
	mockCommissionRateRepo.EXPECT().
		FindByShopAndType(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("not found")).
		AnyTimes()

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.OrderID.String()).
		Return(&entity.Order{
			ID:     processingPayment.OrderID.String(),
			ShopID: shop.ID.String(),
		}, nil).
		AnyTimes()

	mockEscrowRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusSuccess, response.Status)
	assert.Equal(t, "TXN-123", response.ProviderRef)
}

func TestCheckPaymentStatusUsecase_StatusChangedToFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			Status:        entity.PaymentStatusFailed,
			FailureReason: "Insufficient funds",
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusFailed, response.Status)
}

func TestCheckPaymentStatusUsecase_OrderUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			ProviderRef: "TXN-123",
			Status:      entity.PaymentStatusSuccess,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	mockOrderRepo.EXPECT().
		UpdateStatus(gomock.Any(), processingPayment.OrderID, "confirmed").
		Return(errors.New("database error")).
		AnyTimes()

	mockEscrowRepo.EXPECT().
		FindByOrderID(gomock.Any(), processingPayment.OrderID.String()).
		Return(nil, errors.New("not found")).
		AnyTimes()

	mockCommissionRateRepo.EXPECT().
		FindByShopAndType(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("not found")).
		AnyTimes()

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.OrderID.String()).
		Return(&entity.Order{
			ID:     processingPayment.OrderID.String(),
			ShopID: shop.ID.String(),
		}, nil).
		AnyTimes()

	mockEscrowRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusSuccess, response.Status)
}

func TestCheckPaymentStatusUsecase_DTO_MappingComplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockCommissionRateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
		mockEscrowRepo,
		mockCommissionRateRepo,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createProcessingPayment(shop.ID, 50000)
	successPayment.MarkSuccess("TXN-123")

	phone := "+22670123456"
	successPayment.CustomerPhone = &phone

	description := "Payment for order"
	successPayment.Description = &description

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found")).
		AnyTimes()

	response, err := uc.Execute(ctx, successPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)

	assert.Equal(t, successPayment.ID.String(), response.ID)
	assert.Equal(t, successPayment.OrderID.String(), response.OrderID)
	assert.Equal(t, entity.ProviderYengaPay, response.Provider)
	assert.Equal(t, "TXN-123", response.ProviderRef)
	assert.Equal(t, int64(50000), response.AmountCents)
	assert.Equal(t, entity.CurrencyXOF, response.Currency)
	assert.Equal(t, entity.PaymentStatusSuccess, response.Status)
	assert.Equal(t, "+22670123456", response.CustomerPhone)
	assert.Equal(t, "Payment for order", response.Description)
	assert.NotEmpty(t, response.CreatedAt)
	assert.NotEmpty(t, response.CompletedAt)
}
