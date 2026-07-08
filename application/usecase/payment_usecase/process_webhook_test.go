package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/infrastructure/payment"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.9 : TESTS UNITAIRES - PROCESS WEBHOOK USECASE
// ============================================================
//
// 🎯 Stratégie :
//   - Provider not found
//   - Webhook validation failed
//   - Payment not found
//   - Shop not found
//   - Status transitions (success, failed, cancelled, refunded)
//   - Tontine webhook delegation
//   - Terminal state ignored
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestWebhookEvent crée un événement webhook de test
func createTestWebhookEvent(providerRef string, status entity.PaymentStatus) *payment.WebhookEvent {
	return &payment.WebhookEvent{
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: providerRef,
		ExternalID:  "ext-123",
		Status:      status,
		Metadata:    map[string]interface{}{},
	}
}

// createMockDBExecutor crée un mock DBExecutor
func createMockDBExecutor(ctrl *gomock.Controller) *mockrepo.MockDBExecutor {
	return mockrepo.NewMockDBExecutor(ctrl)
}

// ============================================================
// TESTS : ProcessWebhookUsecase - Provider errors
// ============================================================

func TestProcessWebhookUsecase_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	// Mock : Provider non trouvé
	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found"))

	// Mock : Enregistrement webhook (signature invalide)
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")
}

func TestProcessWebhookUsecase_WebhookValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	// Mock : Validation échoue (signature invalide)
	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("invalid signature"))

	// Mock : Enregistrement webhook (signature invalide)
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "bad-signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook validation failed")
}

// ============================================================
// TESTS : ProcessWebhookUsecase - Payment/Shop not found
// ============================================================

func TestProcessWebhookUsecase_MissingProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	// Mock : Événement sans ProviderRef
	event := &payment.WebhookEvent{
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "", // ❌ Manquant
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing provider_ref")
}

func TestProcessWebhookUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	// Mock : Payment non trouvé
	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(nil, errors.New("payment not found"))

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment not found")
}

func TestProcessWebhookUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	// Créer un payment
	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	// Mock : Shop non trouvé
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, errors.New("shop not found"))

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop not found")
}

// ============================================================
// TESTS : ProcessWebhookUsecase - Terminal state ignored
// ============================================================

func TestProcessWebhookUsecase_TerminalState_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	// Créer un payment déjà SUCCESS (terminal)
	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()
	paymentEntity.MarkSuccess("TXN-123")

	testShop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
		Slug: "test-shop",
	}

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Update ne doit PAS être appelé (état terminal)
	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
}

// ============================================================
// TESTS : ProcessWebhookUsecase - Status transitions
// ============================================================

func TestProcessWebhookUsecase_StatusSuccess_MarkSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	// Créer un payment PROCESSING
	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	testShop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
		Slug: "test-shop",
	}

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Mock : Update réussit
	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusSuccess, paymentEntity.Status)
}

func TestProcessWebhookUsecase_StatusFailed_MarkFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusFailed)
	event.Metadata["failure_reason"] = "Insufficient funds"

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	testShop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
		Slug: "test-shop",
	}

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusFailed, paymentEntity.Status)
	assert.Equal(t, "Insufficient funds", paymentEntity.Metadata["failure_reason"])
}

func TestProcessWebhookUsecase_StatusCancelled_MarkCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusCancelled)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	testShop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
		Slug: "test-shop",
	}

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusCancelled, paymentEntity.Status)
}

func TestProcessWebhookUsecase_StatusUnknown_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusPending)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	testShop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
		Slug: "test-shop",
	}

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Update ne doit PAS être appelé (statut inconnu)
	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusProcessing, paymentEntity.Status)
}

// ============================================================
// TESTS : ProcessWebhookUsecase - Tontine delegation
// ============================================================

func TestProcessWebhookUsecase_TontineReference_NoHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil) // ❌ nil

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		mockTontineUC,
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	// Événement avec référence TONTINE
	event := &payment.WebhookEvent{
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "TONTINE:abc12345:1:xyz67890",
		ExternalID:  "txn-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata: map[string]interface{}{
			"reference": "TONTINE:abc12345:1:xyz67890",
		},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine webhook handler not configured")
}
