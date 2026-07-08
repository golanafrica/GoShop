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
// 🆕 v4.4.7 : TESTS UNITAIRES - SUBMIT CLIENT PROOF USECASE
// ============================================================
//
// 🎯 Stratégie :
//   - Validation → Erreurs infra → Règles métier → Happy path → Cas limites
//   - Pattern éprouvé sur CollectCommissionUsecase
//   - Tous les mocks existent déjà (pas de nouvelle interface nécessaire)
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createMockTxClient crée un mock de transaction
func createMockTxClient(ctrl *gomock.Controller) *mockrepo.MockTx {
	return mockrepo.NewMockTx(ctrl)
}

// createTestContextClient crée un contexte tenant seul
func createTestContextClient() context.Context {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
	}
	return tenant.WithTenant(ctx, shop)
}

// createTestContextAndProofClient crée un contexte tenant ET une preuve synchronisée
func createTestContextAndProofClient() (context.Context, *entity.CODProof) {
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

// createTestCODOrder crée une commande COD valide pour les tests
func createTestCODOrder(orderID string, totalCents int64) *entity.Order {
	return &entity.Order{
		ID:            orderID,
		TotalCents:    totalCents,
		PaymentMethod: "cash_on_delivery", // ✅ Commande COD
		Status:        "pending",
	}
}

// createTestNonCODOrder crée une commande NON-COD pour les tests
func createTestNonCODOrder(orderID string) *entity.Order {
	return &entity.Order{
		ID:            orderID,
		TotalCents:    50000,
		PaymentMethod: "mobile_money", // ❌ Pas COD
		Status:        "pending",
	}
}

// createValidSubmitRequest crée une requête valide pour les tests
func createValidSubmitRequest() *codusecase.SubmitClientProofRequest {
	return &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Validation
// ============================================================

func TestSubmitClientProofUsecase_Validation_EmptyOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "", // Vide
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "validation error")
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestSubmitClientProofUsecase_Validation_EmptyCustomerID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "", // Vide
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestSubmitClientProofUsecase_Validation_EmptyProofURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "", // Vide
		AmountCents: 50000,
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "proof_url is required")
}

func TestSubmitClientProofUsecase_Validation_ZeroAmountCents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 0, // Zéro
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitClientProofUsecase_Validation_NegativeAmountCents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: -50000, // Négatif
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitClientProofUsecase_Validation_ZeroPaymentDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Time{}, // Zero value
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "payment_date is required")
}

func TestSubmitClientProofUsecase_Validation_FuturePaymentDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now().Add(1 * time.Hour), // Dans le futur
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "payment_date cannot be in the future")
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Multi-tenant
// ============================================================

func TestSubmitClientProofUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	// Contexte SANS tenant
	ctx := context.Background()
	req := createValidSubmitRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestSubmitClientProofUsecase_ProofShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Preuve d'un AUTRE shop
	wrongShopProof, _ := entity.NewCODProof("order-123", "other-shop-456", "customer-123", 50000)
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(wrongShopProof, nil)

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "does not belong to tenant shop")
}

func TestSubmitClientProofUsecase_CustomerMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Preuve d'un AUTRE customer
	wrongCustomerProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "other-customer-456", 50000)
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(wrongCustomerProof, nil)

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "customer does not match")
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Transaction
// ============================================================

func TestSubmitClientProofUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	// Mock : BeginTx échoue
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(nil, errors.New("database connection failed"))

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestSubmitClientProofUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofClient()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// ✅ Mock : Commit ÉCHOUE
	mockTx.EXPECT().Commit().Return(errors.New("database commit failed"))

	req := createValidSubmitRequest()
	req.OrderID = "order-123"
	req.CustomerID = "customer-123"

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to commit transaction")
	_ = shop
}

func TestSubmitClientProofUsecase_RollbackCalledOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// ✅ Vérifie que Rollback est appelé exactement 1 fois
	mockTx.EXPECT().Rollback().Return(nil).Times(1)

	// Mock : Order non trouvée (déclenche rollback)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(nil, errors.New("not found"))

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	// Le test échouera si Rollback n'est pas appelé exactement 1 fois
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Repository
// ============================================================

func TestSubmitClientProofUsecase_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order non trouvée
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(nil, errors.New("order not found"))

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order not found")
}

func TestSubmitClientProofUsecase_ProofNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof non trouvée
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(nil, errors.New("proof not found"))

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "COD proof not found")
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Métier
// ============================================================

func TestSubmitClientProofUsecase_OrderNotCOD(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// ✅ Mock : Order NON-COD
	nonCODOrder := createTestNonCODOrder("order-123")
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(nonCODOrder, nil)

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not a cash-on-delivery order")
}

func TestSubmitClientProofUsecase_ProofInvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// ✅ Mock : Preuve avec statut invalide (Completed)
	invalidStatusProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	invalidStatusProof.Status = entity.CODProofCompleted // Statut terminal
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(invalidStatusProof, nil)

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot submit client proof with status")
}

func TestSubmitClientProofUsecase_DeadlineExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order trouvée
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// ✅ Mock : Preuve avec deadline dépassée
	expiredProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	expiredProof.CreatedAt = time.Now().AddDate(0, 0, -10) // Créé il y a 10 jours
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo)
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(expiredProof, nil)

	req := createValidSubmitRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Happy Paths
// ============================================================

func TestSubmitClientProofUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofClient()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// Mock : Order
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := createValidSubmitRequest()
	req.OrderID = "order-123"
	req.CustomerID = "customer-123"

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofClientProofSent, response.Status)
	assert.Equal(t, "https://example.com/proof.jpg", response.ClientProofURL)
	assert.Equal(t, int64(50000), response.ClientAmountCents)
	assert.False(t, response.HasMerchantProof) // Pas de preuve marchand
	assert.False(t, response.IsCoherent)       // Pas encore cohérent
	assert.Greater(t, response.DaysRemaining, 0)
	_ = shop
}

func TestSubmitClientProofUsecase_SuccessWithAutoCoherence(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// Mock : Order
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// ✅ Mock : Preuve avec preuve marchand déjà soumise (MerchantProofSent)
	proofWithMerchant, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	paymentDate := time.Now().Add(-1 * time.Hour)
	proofWithMerchant.SubmitMerchantProof(
		"https://example.com/merchant.jpg",
		50000,
		paymentDate,
		"",
	)
	assert.Equal(t, entity.CODProofMerchantProofSent, proofWithMerchant.Status)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(proofWithMerchant, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := createValidSubmitRequest()
	req.OrderID = "order-123"
	req.CustomerID = "customer-123"
	req.AmountCents = 50000
	req.PaymentDate = paymentDate

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofConfirmed, response.Status) // ✅ Auto-coherence
	assert.True(t, response.HasMerchantProof)
	assert.True(t, response.IsCoherent) // ✅ Cohérent
	assert.NotNil(t, response.AmountsMatch)
	assert.True(t, *response.AmountsMatch)
	assert.NotNil(t, response.DatesMatch)
	assert.True(t, *response.DatesMatch)
}

// ============================================================
// TESTS : SubmitClientProofUsecase - Cas limites
// ============================================================

func TestSubmitClientProofUsecase_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : Order
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// ✅ Mock : Update ÉCHOUE
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("database update failed"))

	req := createValidSubmitRequest()
	req.OrderID = "order-123"
	req.CustomerID = "customer-123"

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to update COD proof")
}

func TestSubmitClientProofUsecase_WithOptionalFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofClient()

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// Mock : Order
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	receiptNumber := "RECEIPT-123"
	notes := "Payment completed successfully"

	req := createValidSubmitRequest()
	req.OrderID = "order-123"
	req.CustomerID = "customer-123"
	req.ReceiptNumber = &receiptNumber
	req.Notes = &notes

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofClientProofSent, response.Status)
}

// ============================================================
// TESTS : SubmitClientProofUsecase - WithAmounts wrapper
// ============================================================

func TestSubmitClientProofUsecase_WithAmounts_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTxClient(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProofClient()

	// Mock : Order (appelé 2 fois : 1x dans WithAmounts, 1x dans Execute)
	codOrder := createTestCODOrder("order-123", 50000)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil).Times(1)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	mockOrderRepo.EXPECT().WithTX(gomock.Any()).Return(mockOrderRepo)
	mockOrderRepo.EXPECT().FindByID(gomock.Any(), "order-123").Return(codOrder, nil)

	// Mock : Proof
	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Update
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	receiptNumber := "RECEIPT-456"
	notes := "Test notes"
	paymentDate := time.Now().Add(-1 * time.Hour)

	response, err := uc.SubmitClientProofWithAmounts(
		ctx,
		"order-123",
		"customer-123",
		"https://example.com/proof.jpg",
		paymentDate,
		&receiptNumber,
		&notes,
	)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODProofClientProofSent, response.Status)
	assert.Equal(t, int64(50000), response.ClientAmountCents) // Montant de la commande
}

func TestSubmitClientProofUsecase_WithAmounts_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewSubmitClientProofUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockTxManager,
	)

	ctx := createTestContextClient()

	mockOrderRepo.EXPECT().
		FindByID(gomock.Any(), "order-not-found").
		Return(nil, errors.New("order not found"))

	response, err := uc.SubmitClientProofWithAmounts(
		ctx,
		"order-not-found",
		"customer-123",
		"https://example.com/proof.jpg",
		time.Now().Add(-1*time.Hour),
		nil,
		nil,
	)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "order not found")
}
