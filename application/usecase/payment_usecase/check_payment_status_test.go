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
// 🆕 v4.4.9 : TESTS UNITAIRES - CHECK PAYMENT STATUS USECASE
// ============================================================
//
// 🎯 Stratégie :
//   - Multi-tenant validation
//   - Payment ID validation
//   - Repository errors
//   - Provider interactions (sync status)
//   - Happy paths (terminal/non-terminal)
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createPendingPayment crée un paiement en attente
func createPendingPayment(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, amountCents)
	return payment
}

// createProcessingPayment crée un paiement en cours avec ProviderRef
func createProcessingPayment(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment := createPendingPayment(shopID, amountCents)
	payment.MarkProcessing()

	// ✅ CORRECTION : Définir ProviderRef pour que le provider soit appelé
	providerRef := "TXN-123"
	payment.ProviderRef = &providerRef

	return payment
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Multi-tenant
// ============================================================

func TestCheckPaymentStatusUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	// Contexte SANS tenant
	ctx := context.Background()
	paymentID := uuid.New().String()

	response, err := uc.Execute(ctx, paymentID)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Validation
// ============================================================

func TestCheckPaymentStatusUsecase_InvalidPaymentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()

	response, err := uc.Execute(ctx, "invalid-uuid")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid payment_id")
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Repository errors
// ============================================================

func TestCheckPaymentStatusUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	paymentID := uuid.New()

	// Mock : Payment non trouvé
	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentID).
		Return(nil, errors.New("payment not found"))

	response, err := uc.Execute(ctx, paymentID.String())

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "payment not found")
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Terminal states
// ============================================================

func TestCheckPaymentStatusUsecase_TerminalState_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement SUCCESS (terminal)
	successPayment := createProcessingPayment(shop.ID, 50000)
	successPayment.MarkSuccess("TXN-123")

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	// Provider ne doit PAS être appelé (état terminal)
	// mockRegistry.EXPECT().Get(...).Times(0)

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

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement FAILED (terminal)
	failedPayment := createProcessingPayment(shop.ID, 50000)
	failedPayment.MarkFailed("Insufficient funds")

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), failedPayment.ID).
		Return(failedPayment, nil)

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

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement REFUNDED (terminal)
	refundedPayment := createProcessingPayment(shop.ID, 50000)
	refundedPayment.MarkSuccess("TXN-123")
	refundedPayment.MarkRefunded()

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), refundedPayment.ID).
		Return(refundedPayment, nil)

	response, err := uc.Execute(ctx, refundedPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusRefunded, response.Status)
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Non-terminal without ProviderRef
// ============================================================

func TestCheckPaymentStatusUsecase_NonTerminalWithoutProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement PENDING sans ProviderRef
	pendingPayment := createPendingPayment(shop.ID, 50000)
	// ProviderRef = nil

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), pendingPayment.ID).
		Return(pendingPayment, nil)

	// Provider ne doit PAS être appelé (pas de ProviderRef)
	response, err := uc.Execute(ctx, pendingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusPending, response.Status)
	assert.Empty(t, response.ProviderRef)
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Provider interactions
// ============================================================

func TestCheckPaymentStatusUsecase_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement PROCESSING avec ProviderRef
	processingPayment := createProcessingPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	// Mock : Provider non trouvé (erreur ignorée)
	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found"))

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	// Erreur ignorée, on retourne le statut actuel
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

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
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

	// Mock : Provider échoue la vérification (erreur ignorée)
	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("provider unavailable"))

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	// Erreur ignorée, on retourne le statut actuel
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

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
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

	// Mock : Provider retourne le même statut (PROCESSING)
	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			Status: entity.PaymentStatusProcessing,
		}, nil)

	// Update ne doit PAS être appelé (statut inchangé)
	// mockPaymentRepo.EXPECT().Update(...).Times(0)

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusProcessing, response.Status)
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - Status changed
// ============================================================

func TestCheckPaymentStatusUsecase_StatusChangedToSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
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

	// Mock : Provider retourne SUCCESS (changement de statut)
	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			ProviderRef: "TXN-123",
			Status:      entity.PaymentStatusSuccess,
		}, nil)

	// Mock : Update payment
	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : Update order status to PAID
	mockOrderRepo.EXPECT().
		UpdateStatus(gomock.Any(), processingPayment.OrderID, "paid").
		Return(nil)

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

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
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

	// Mock : Provider retourne FAILED (changement de statut)
	mockProvider.EXPECT().
		CheckStatus(gomock.Any(), gomock.Any()).
		Return(&payment.PaymentStatus{
			Status:        entity.PaymentStatusFailed,
			FailureReason: "Insufficient funds",
		}, nil)

	// Mock : Update payment
	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	// Order update ne doit PAS être appelé (statut FAILED)
	// mockOrderRepo.EXPECT().UpdateStatus(...).Times(0)

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

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
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
		Return(nil)

	// Mock : Order update échoue (erreur loggée mais continue)
	mockOrderRepo.EXPECT().
		UpdateStatus(gomock.Any(), processingPayment.OrderID, "paid").
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, processingPayment.ID.String())

	// Erreur ignorée, on continue
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusSuccess, response.Status)
}

// ============================================================
// TESTS : CheckPaymentStatusUsecase - DTO Mapping
// ============================================================

func TestCheckPaymentStatusUsecase_DTO_MappingComplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewCheckPaymentStatusUsecase(
		mockPaymentRepo,
		mockOrderRepo,
		mockRegistry,
	)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement avec tous les champs
	successPayment := createProcessingPayment(shop.ID, 50000)
	successPayment.MarkSuccess("TXN-123")

	phone := "+22670123456"
	successPayment.CustomerPhone = &phone

	description := "Payment for order"
	successPayment.Description = &description

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	response, err := uc.Execute(ctx, successPayment.ID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier tous les champs du DTO
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
