package codusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	codusecase "Goshop/application/usecase/cod_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.7 : TESTS UNITAIRES - SUBMIT MERCHANT PROOF USECASE
// ============================================================
//
// 🎯 Stratégie :
//   - Validation → Erreurs infra → Règles métier → Happy path → Cas limites
//   - Pattern éprouvé sur SubmitClientProofUsecase
//   - Tous les mocks existent déjà (pas de nouvelle interface nécessaire)
//
// 🔄 Différences avec SubmitClientProofUsecase :
//   - Pas de CustomerID dans la requête
//   - ReceiptDate au lieu de PaymentDate
//   - Statut valide : PendingProofs ou ClientProofSent (pas MerchantProofSent)
//   - Auto-coherence avec preuve client déjà soumise
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createMockTxMerchant crée un mock de transaction
func createMockTxMerchant(ctrl *gomock.Controller) *mockrepo.MockTx {
	return mockrepo.NewMockTx(ctrl)
}

// createTestContextMerchant crée un contexte tenant seul
func createTestContextMerchant() context.Context {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
	}
	return tenant.WithTenant(ctx, shop)
}

// createTestContextAndProofMerchant crée un contexte tenant ET une preuve synchronisée
func createTestContextAndProofMerchant() (context.Context, *entity.CODProof) {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
	}
	ctx = tenant.WithTenant(ctx, shop)

	proof, _ := entity.NewCODProof("order-123", shopID.String(), "customer-123", 50000)
	return ctx, proof
}

// createTestCODOrderMerchant crée une commande COD valide pour les tests
func createTestCODOrderMerchant(orderID string, totalCents int64) *entity.Order {
	return &entity.Order{
		ID:            orderID,
		TotalCents:    totalCents,
		PaymentMethod: "cash_on_delivery", // ✅ Commande COD
		Status:        "pending",
	}
}

// createTestNonCODOrderMerchant crée une commande NON-COD pour les tests
func createTestNonCODOrderMerchant(orderID string) *entity.Order {
	return &entity.Order{
		ID:            orderID,
		TotalCents:    50000,
		PaymentMethod: "mobile_money", // ❌ Pas COD
		Status:        "pending",
	}
}

// createValidMerchantSubmitRequest crée une requête valide pour les tests
func createValidMerchantSubmitRequest() *codusecase.SubmitMerchantProofRequest {
	return &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(-1 * time.Hour),
	}
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Validation
// ============================================================

func TestSubmitMerchantProofUsecase_Validation_EmptyOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "", // Vide
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "validation error")
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestSubmitMerchantProofUsecase_Validation_EmptyProofURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "", // Vide
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "proof_url is required")
}

func TestSubmitMerchantProofUsecase_Validation_ZeroAmountCents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 0, // Zéro
		ReceiptDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitMerchantProofUsecase_Validation_NegativeAmountCents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: -50000, // Négatif
		ReceiptDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitMerchantProofUsecase_Validation_ZeroReceiptDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Time{}, // Zero value
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "receipt_date is required")
}

func TestSubmitMerchantProofUsecase_Validation_FutureReceiptDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(1 * time.Hour), // Dans le futur
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "receipt_date cannot be in the future")
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Multi-tenant
// ============================================================

func TestSubmitMerchantProofUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	// Contexte SANS tenant
	ctx := context.Background()
	req := createValidMerchantSubmitRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestSubmitMerchantProofUsecase_ProofShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Preuve d'un AUTRE shop
	wrongShopProof, _ := entity.NewCODProof("order-123", "other-shop-456", "customer-123", 50000)
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(wrongShopProof, nil)

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "does not belong to tenant shop")
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Transaction
// ============================================================

func TestSubmitMerchantProofUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	// Mock : BeginTx échoue
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(nil, errors.New("database connection failed"))

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestSubmitMerchantProofUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// ✅ Mock : Commit ÉCHOUE
	mockTx.EXPECT().Commit().Return(errors.New("database commit failed"))

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestSubmitMerchantProofUsecase_RollbackCalledOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// ✅ Vérifie que Rollback est appelé exactement 1 fois
	mockTx.EXPECT().Rollback().Return(nil).Times(1)

	// Mock : Order non trouvée (déclenche rollback)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(nil, errors.New("not found"))

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Repository
// ============================================================

func TestSubmitMerchantProofUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order non trouvée
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(nil, errors.New("order not found"))

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order not found")
}

func TestSubmitMerchantProofUsecase_ProofNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof non trouvée
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(nil, errors.New("proof not found"))

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "COD proof not found")
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Métier
// ============================================================

func TestSubmitMerchantProofUsecase_OrderNotCOD(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// ✅ Mock : Order NON-COD
	nonCODOrder := createTestNonCODOrderMerchant("order-123")
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(nonCODOrder, nil)

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not a cash-on-delivery order")
}

func TestSubmitMerchantProofUsecase_ProofInvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// ✅ Mock : Preuve avec statut invalide (Completed)
	invalidStatusProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	invalidStatusProof.Status = entity.CODProofCompleted // Statut terminal
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(invalidStatusProof, nil)

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot submit merchant proof with status")
}

func TestSubmitMerchantProofUsecase_DeadlineExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// ✅ Mock : Preuve avec deadline dépassée
	expiredProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	expiredProof.CreatedAt = time.Now().AddDate(0, 0, -10) // Créé il y a 10 jours
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(expiredProof, nil)

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Happy Paths
// ============================================================

func TestSubmitMerchantProofUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// Mock : Order
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofMerchantProofSent, response.Status)
	assert.Equal(t, "https://example.com/receipt.jpg", response.MerchantProofURL)
	assert.Equal(t, int64(50000), response.MerchantAmountCents)
	assert.False(t, response.HasClientProof) // Pas de preuve client
	assert.False(t, response.IsCoherent)     // Pas encore cohérent
	assert.Greater(t, response.DaysRemaining, 0)
}

func TestSubmitMerchantProofUsecase_SuccessWithAutoCoherence(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// Mock : Order
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// ✅ Mock : Preuve avec preuve client déjà soumise (ClientProofSent)
	proofWithClient, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	paymentDate := time.Now().Add(-1 * time.Hour)
	proofWithClient.SubmitClientProof(
		"https://example.com/client.jpg",
		50000,
		paymentDate,
		"",
		"",
	)
	assert.Equal(t, entity.CODProofClientProofSent, proofWithClient.Status)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(proofWithClient, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := createValidMerchantSubmitRequest()
	req.AmountCents = 50000
	req.ReceiptDate = paymentDate

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofConfirmed, response.Status) // ✅ Auto-coherence
	assert.True(t, response.HasClientProof)
	assert.True(t, response.IsCoherent) // ✅ Cohérent
	assert.NotNil(t, response.AmountsMatch)
	assert.True(t, *response.AmountsMatch)
	assert.NotNil(t, response.DatesMatch)
	assert.True(t, *response.DatesMatch)
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - Cas limites
// ============================================================

func TestSubmitMerchantProofUsecase_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// ✅ Mock : Update ÉCHOUE
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("database update failed"))

	req := createValidMerchantSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to update COD proof")
}

func TestSubmitMerchantProofUsecase_WithOptionalNotes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofMerchant()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// Mock : Order
	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	notes := "Cash received from customer"

	req := createValidMerchantSubmitRequest()
	req.Notes = &notes

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofMerchantProofSent, response.Status)
}

// ============================================================
// TESTS : SubmitMerchantProofUsecase - WithAmounts wrapper
// ============================================================

func TestSubmitMerchantProofUsecase_WithAmounts_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxMerchant(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofMerchant()

	codOrder := createTestCODOrderMerchant("order-123", 50000)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil).Times(1)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	notes := "Cash received"
	receiptDate := time.Now().Add(-1 * time.Hour)

	response, err := uc.SubmitMerchantProofWithAmounts(
		ctx,
		"order-123",
		"https://example.com/receipt.jpg",
		receiptDate,
		&notes,
	)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofMerchantProofSent, response.Status)
	assert.Equal(t, int64(50000), response.MerchantAmountCents)
}

func TestSubmitMerchantProofUsecase_WithAmounts_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitMerchantProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextMerchant()

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), "order-not-found").
		Return(nil, errors.New("order not found"))

	response, err := uc.SubmitMerchantProofWithAmounts(
		ctx,
		"order-not-found",
		"https://example.com/receipt.jpg",
		time.Now().Add(-1*time.Hour),
		nil,
	)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order not found")
}
