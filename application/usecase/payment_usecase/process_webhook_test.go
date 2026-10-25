package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/infrastructure/payment" // 🆕 Ajouté pour WebhookEvent
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

type mockResult struct {
	lastInsertId int64
	rowsAffected int64
}

func (m *mockResult) LastInsertId() (int64, error) { return m.lastInsertId, nil }
func (m *mockResult) RowsAffected() (int64, error) { return m.rowsAffected, nil }

// Note : createTestContextForPayment est déjà définie dans list_payments_test.go
// et est partagée automatiquement car les deux fichiers sont dans le même package.

// expectWebhookAudit configure le mock pour accepter les appels ExecContext avec les différents nombres d'arguments
// 🆕 C1 AUDIT : recordWebhook envoie maintenant 9 arguments (ajout du lease duration)
func expectWebhookAudit(mockDB *mockrepo.MockDBExecutor) {
	// 1. Pour recordWebhook (9 arguments : ctx, query, provider, eventType, externalID, payload, signature, signatureValid, leaseSeconds)
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).
		AnyTimes()

	// 2. Pour markWebhookProcessed (4 arguments : ctx, query, provider, externalID)
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).
		AnyTimes()

	// 3. Pour markWebhookFailed (5 arguments : ctx, query, provider, externalID, errMsg)
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&mockResult{rowsAffected: 1}, nil).
		AnyTimes()
}

// ============================================================
// TESTS : Provider / validation
// ============================================================

func TestProcessWebhookUsecase_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
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
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
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
// TESTS : Payment / shop not found
// ============================================================

func TestProcessWebhookUsecase_MissingProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé : payment.WebhookEvent au lieu de paymentusecase.WebhookEvent
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
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{},
	}

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
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity := &entity.Payment{
		ID:       uuid.New(),
		ShopID:   shopID,
		Provider: entity.ProviderYengaPay,
		Status:   entity.PaymentStatusProcessing,
	}

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
// TESTS : Terminal + transitions
// ============================================================

func TestProcessWebhookUsecase_TerminalState_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity := &entity.Payment{
		ID:       uuid.New(),
		ShopID:   shopID,
		Provider: entity.ProviderYengaPay,
		Status:   entity.PaymentStatusSuccess,
		Metadata: map[string]interface{}{"escrow_created": true},
	}

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
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.success",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusSuccess,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity := &entity.Payment{
		ID:       uuid.New(),
		ShopID:   shopID,
		Provider: entity.ProviderYengaPay,
		Status:   entity.PaymentStatusProcessing,
	}

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
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.failed",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusFailed,
		Metadata:    map[string]interface{}{"failure_reason": "Insufficient funds"},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity := &entity.Payment{
		ID:       uuid.New(),
		ShopID:   shopID,
		Provider: entity.ProviderYengaPay,
		Status:   entity.PaymentStatusProcessing,
	}

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
}

func TestProcessWebhookUsecase_StatusCancelled_MarkCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.cancelled",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusCancelled,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity := &entity.Payment{
		ID:       uuid.New(),
		ShopID:   shopID,
		Provider: entity.ProviderYengaPay,
		Status:   entity.PaymentStatusProcessing,
	}

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
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
		Provider:    entity.ProviderYengaPay,
		EventType:   "payment.unknown",
		ProviderRef: "TXN-123",
		ExternalID:  "ext-123",
		Status:      entity.PaymentStatusPending,
		Metadata:    map[string]interface{}{},
	}

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	expectWebhookAudit(mockDB)

	shopID := uuid.New()
	paymentEntity := &entity.Payment{
		ID:       uuid.New(),
		ShopID:   shopID,
		Provider: entity.ProviderYengaPay,
		Status:   entity.PaymentStatusProcessing,
	}

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

// ============================================================
// TESTS : Tontine
// ============================================================

func TestProcessWebhookUsecase_TontineReference_NoHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := mockrepo.NewMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockrepo.NewMockTxManager(ctrl), mockDB, mockShopRepo, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := &payment.WebhookEvent{ // 🆕 Corrigé
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
