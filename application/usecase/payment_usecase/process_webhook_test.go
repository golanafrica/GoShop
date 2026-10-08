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
// TESTS UNITAIRES - PROCESS WEBHOOK USECASE (P1-C + txManager)
// ============================================================

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

func createMockDBExecutor(ctrl *gomock.Controller) *mockrepo.MockDBExecutor {
	return mockrepo.NewMockDBExecutor(ctrl)
}

// INSERT processed=false + mark processed/failed (plusieurs ExecContext)
func expectWebhookAudit(mockDB *mockrepo.MockDBExecutor) {
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).
		AnyTimes()
}

func newProcessWebhookUC(
	ctrl *gomock.Controller,
	paymentRepo *mockrepo.MockPaymentRepository,
	registry *mockusecase.MockPaymentRegistry,
	db *mockrepo.MockDBExecutor,
	shopRepo *mockrepo.MockShopRepository,
) *paymentusecase.ProcessWebhookUsecase {
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	return paymentusecase.NewProcessWebhookUsecase(
		paymentRepo,
		registry,
		mockTxManager,
		db,
		shopRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

// ============================================================
// Provider / validation
// ============================================================

func TestProcessWebhookUsecase_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found"))

	expectWebhookAudit(mockDB)

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

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("invalid signature"))

	expectWebhookAudit(mockDB)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "bad-signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook validation failed")
}

// ============================================================
// Payment / shop not found
// ============================================================

func TestProcessWebhookUsecase_MissingProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment not found for provider_ref")
}

func TestProcessWebhookUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

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

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	mockPaymentRepo.EXPECT().
		FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").
		Return(paymentEntity, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, errors.New("shop not found"))

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop not found")
}

// ============================================================
// Terminal + transitions
// ============================================================

func TestProcessWebhookUsecase_TerminalState_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()
	paymentEntity.MarkSuccess("TXN-123")
	paymentEntity.Metadata = map[string]interface{}{"escrow_created": true}

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

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
}

func TestProcessWebhookUsecase_StatusSuccess_MarkSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

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

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusFailed)
	event.Metadata["failure_reason"] = "Insufficient funds"

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

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

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusCancelled)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

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

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusPending)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

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

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusProcessing, paymentEntity.Status)
}

// ============================================================
// Tontine
// ============================================================

func TestProcessWebhookUsecase_TontineReference_NoHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := newProcessWebhookUC(ctrl, mockPaymentRepo, mockRegistry, mockDB, mockShopRepo)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

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

	expectWebhookAudit(mockDB)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine webhook handler not configured")
}
