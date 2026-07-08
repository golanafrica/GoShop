package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentdto "Goshop/application/dto/payment_dto"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.9 : TESTS UNITAIRES - REFUND PAYMENT USECASE
// ============================================================
//
// 🎯 Stratégie :
//   - Validation de la requête
//   - Multi-tenant validation
//   - Règles métier (statut, montant, propriétaire)
//   - Provider errors
//   - Happy paths (remboursement complet/partiel)
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createValidRefundRequest crée une requête de remboursement valide
func createValidRefundRequest(paymentID string) *paymentdto.RefundPaymentRequest {
	return &paymentdto.RefundPaymentRequest{
		PaymentID:   paymentID,
		AmountCents: 0, // 0 = remboursement complet
		Reason:      "Customer request",
	}
}

// createSuccessfulPayment crée un paiement réussi pour les tests
func createSuccessfulPayment(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, amountCents)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")
	return payment
}

// ============================================================
// TESTS : RefundPaymentUsecase - Validation
// ============================================================

func TestRefundPaymentUsecase_Validation_InvalidPaymentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   "invalid-uuid", // ❌ UUID invalide
		AmountCents: 0,
		Reason:      "Customer request",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid payment_id")
}

// ============================================================
// TESTS : RefundPaymentUsecase - Multi-tenant
// ============================================================

func TestRefundPaymentUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	// Contexte SANS tenant
	ctx := context.Background()
	req := createValidRefundRequest(uuid.New().String())

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : RefundPaymentUsecase - Repository errors
// ============================================================

func TestRefundPaymentUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	paymentID := uuid.New()

	// Mock : Payment non trouvé
	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentID).
		Return(nil, errors.New("payment not found"))

	req := createValidRefundRequest(paymentID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "payment not found")
}

// ============================================================
// TESTS : RefundPaymentUsecase - Règles métier
// ============================================================

func TestRefundPaymentUsecase_PaymentNotBelongToShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement pour un AUTRE shop
	otherShopID := uuid.New()
	wrongShopPayment := createSuccessfulPayment(otherShopID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), wrongShopPayment.ID).
		Return(wrongShopPayment, nil)

	req := createValidRefundRequest(wrongShopPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "does not belong to current shop")
	_ = shop
}

func TestRefundPaymentUsecase_PaymentNotSuccessful(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement PENDING (pas SUCCESS)
	pendingPayment, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)
	// Statut = PENDING (pas SUCCESS)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), pendingPayment.ID).
		Return(pendingPayment, nil)

	req := createValidRefundRequest(pendingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "only successful payments can be refunded")
}

func TestRefundPaymentUsecase_RefundAmountExceedsOriginal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// Créer un paiement réussi de 50000
	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	// Demander un remboursement de 60000 (> 50000)
	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   successPayment.ID.String(),
		AmountCents: 60000, // ❌ Dépasse le montant original
		Reason:      "Customer request",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "exceeds original payment")
}

// ============================================================
// TESTS : RefundPaymentUsecase - Provider errors
// ============================================================

func TestRefundPaymentUsecase_ProviderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	// Mock : Provider non trouvé
	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(nil, errors.New("provider not found"))

	req := createValidRefundRequest(successPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "provider not found")
}

func TestRefundPaymentUsecase_ProviderRefundFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	// Mock : Provider échoue le remboursement
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(50000)).
		Return(errors.New("insufficient funds"))

	req := createValidRefundRequest(successPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "refund failed at provider")
}

// ============================================================
// TESTS : RefundPaymentUsecase - Happy paths
// ============================================================

func TestRefundPaymentUsecase_Success_FullRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	// Mock : Provider réussit le remboursement complet
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(50000)).
		Return(nil)

	// Mock : Update réussit
	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidRefundRequest(successPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusRefunded, response.Status)
	assert.Equal(t, int64(50000), response.AmountCents)
}

func TestRefundPaymentUsecase_Success_PartialRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	// Mock : Provider réussit le remboursement partiel (30000 sur 50000)
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(30000)).
		Return(nil)

	// Mock : Update réussit
	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   successPayment.ID.String(),
		AmountCents: 30000, // Remboursement partiel
		Reason:      "Partial refund",
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusRefunded, response.Status)
}

func TestRefundPaymentUsecase_Success_WithMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), successPayment.ID).
		Return(successPayment, nil)

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(50000)).
		Return(nil)

	// Mock : Update avec vérification des métadonnées
	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, payment *entity.Payment) error {
			// Vérifier que les métadonnées de remboursement sont ajoutées
			assert.NotNil(t, payment.Metadata)
			assert.Equal(t, "Customer request", payment.Metadata["refund_reason"])
			assert.Equal(t, int64(50000), payment.Metadata["refund_amount"])
			return nil
		})

	req := createValidRefundRequest(successPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusRefunded, response.Status)
}
