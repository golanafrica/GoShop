package paymentusecase_test

import (
	"context"
	"errors"
	"testing"

	paymentdto "Goshop/application/dto/payment_dto"
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
// HELPERS
// ============================================================

func createValidRefundRequest(paymentID string) *paymentdto.RefundPaymentRequest {
	return &paymentdto.RefundPaymentRequest{
		PaymentID:   paymentID,
		AmountCents: 0, // 0 = remboursement du reste
		Reason:      "Customer request",
	}
}

func createSuccessfulPayment(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, amountCents)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")
	return payment
}

func createPartiallyRefundedPayment(shopID uuid.UUID, amountCents, alreadyRefunded int64) *entity.Payment {
	payment := createSuccessfulPayment(shopID, amountCents)
	payment.MarkPartiallyRefunded(alreadyRefunded)
	return payment
}

// paymentAfterReserve simule l'état DB après AddRefundAmount réussi.
func paymentAfterReserve(base *entity.Payment, refundAmount int64, newStatus entity.PaymentStatus) *entity.Payment {
	clone := *base
	clone.RefundedAmountCents = base.RefundedAmountCents + refundAmount
	clone.Status = newStatus
	if clone.Metadata == nil {
		clone.Metadata = make(map[string]interface{})
	} else {
		md := make(map[string]interface{}, len(base.Metadata)+3)
		for k, v := range base.Metadata {
			md[k] = v
		}
		clone.Metadata = md
	}
	return &clone
}

// ============================================================
// TESTS : Validation & Multi-tenant
// ============================================================

func TestRefundPaymentUsecase_Validation_InvalidPaymentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uc := paymentusecase.NewRefundPaymentUsecase(mockrepo.NewMockPaymentRepository(ctrl), mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := createTestContextForPayment()
	req := &paymentdto.RefundPaymentRequest{PaymentID: "invalid-uuid", AmountCents: 0, Reason: "test"}
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment_id")
}

func TestRefundPaymentUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uc := paymentusecase.NewRefundPaymentUsecase(mockrepo.NewMockPaymentRepository(ctrl), mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := context.Background()
	req := createValidRefundRequest(uuid.New().String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : Règles métier
// ============================================================

func TestRefundPaymentUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := createTestContextForPayment()
	paymentID := uuid.New()
	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), paymentID).Return(nil, errors.New("payment not found"))
	req := createValidRefundRequest(paymentID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment not found")
}

func TestRefundPaymentUsecase_PaymentNotBelongToShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := createTestContextForPayment()
	wrongShopPayment := createSuccessfulPayment(uuid.New(), 50000)
	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), wrongShopPayment.ID).Return(wrongShopPayment, nil)
	req := createValidRefundRequest(wrongShopPayment.ID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to current shop")
}

func TestRefundPaymentUsecase_PaymentNotRefundable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)
	pendingPayment, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)
	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), pendingPayment.ID).Return(pendingPayment, nil)
	req := createValidRefundRequest(pendingPayment.ID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only successful or partially refunded payments can be refunded")
}

func TestRefundPaymentUsecase_AlreadyFullyRefunded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)
	refundedPayment := createSuccessfulPayment(shop.ID, 50000)
	refundedPayment.MarkRefunded()
	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), refundedPayment.ID).Return(refundedPayment, nil)
	req := createValidRefundRequest(refundedPayment.ID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already been fully refunded")
}

func TestRefundPaymentUsecase_RefundAmountExceedsRemaining(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockusecase.NewMockPaymentRegistry(ctrl))
	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// 50000 total, déjà 30000 → reste 20000
	partialPayment := createPartiallyRefundedPayment(shop.ID, 50000, 30000)
	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), partialPayment.ID).Return(partialPayment, nil)

	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   partialPayment.ID.String(),
		AmountCents: 25000,
		Reason:      "Customer request",
	}
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds remaining refundable amount")
}

// ============================================================
// TESTS : Provider errors (après réservation atomique)
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

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(successPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), successPayment.ID, int64(50000), entity.PaymentStatusRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(nil, errors.New("provider not found"))

	// Reverse de la réservation avec recomputation du statut
	mockPaymentRepo.EXPECT().
		ReverseRefundAmountAndRecompute(gomock.Any(), successPayment.ID, int64(50000)).
		Return(true, nil)

	// Mise à jour du statut à failed (appelé par le helper reverseRefundAndLog)
	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundFailed, "provider not found").
		Return(nil)

	req := createValidRefundRequest(successPayment.ID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
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

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(successPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), successPayment.ID, int64(50000), entity.PaymentStatusRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)

	// 🆕 PR 5 AUDIT : Utilisation de l'erreur typée pour simuler un refus définitif
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(50000), gomock.Any(), gomock.Any()).
		Return(&payment.ErrCashOutDefinitive{Message: "insufficient funds"})

	// Reverse de la réservation avec recomputation du statut
	mockPaymentRepo.EXPECT().
		ReverseRefundAmountAndRecompute(gomock.Any(), successPayment.ID, int64(50000)).
		Return(true, nil)

	// Mise à jour du statut à failed (appelé par le helper reverseRefundAndLog)
	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundFailed, "provider definitive rejection").
		Return(nil)

	req := createValidRefundRequest(successPayment.ID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refund rejected by provider")
}

// ============================================================
// TESTS : Happy paths (réservation atomique)
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
	after := paymentAfterReserve(successPayment, 50000, entity.PaymentStatusRefunded)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(successPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), successPayment.ID, int64(50000), entity.PaymentStatusRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(50000), gomock.Any(), gomock.Any()).
		Return(nil)

	// Reload après succès provider
	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(after, nil)
	mockPaymentRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundCompleted, "").
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
	after := paymentAfterReserve(successPayment, 30000, entity.PaymentStatusPartiallyRefunded)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(successPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), successPayment.ID, int64(30000), entity.PaymentStatusPartiallyRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(30000), gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(after, nil)
	mockPaymentRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, p *entity.Payment) error {
		assert.Equal(t, entity.PaymentStatusPartiallyRefunded, p.Status)
		assert.Equal(t, int64(30000), p.RefundedAmountCents)
		assert.Equal(t, int64(30000), p.Metadata["total_refunded_amount"])
		assert.Equal(t, int64(30000), p.Metadata["last_refund_amount"])
		return nil
	})

	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundCompleted, "").
		Return(nil)

	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   successPayment.ID.String(),
		AmountCents: 30000,
		Reason:      "Partial refund",
	}
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusPartiallyRefunded, response.Status)
}

func TestRefundPaymentUsecase_Success_MultiplePartialRefunds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)
	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	// 50000, déjà 20000 → reste 30000 ; on ajoute 15000 → cumul 35000
	partialPayment := createPartiallyRefundedPayment(shop.ID, 50000, 20000)
	after := paymentAfterReserve(partialPayment, 15000, entity.PaymentStatusPartiallyRefunded)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), partialPayment.ID).Return(partialPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), partialPayment.ID, int64(15000), entity.PaymentStatusPartiallyRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(15000), gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), partialPayment.ID).Return(after, nil)
	mockPaymentRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, p *entity.Payment) error {
		assert.Equal(t, entity.PaymentStatusPartiallyRefunded, p.Status)
		assert.Equal(t, int64(35000), p.RefundedAmountCents)
		assert.Equal(t, int64(35000), p.Metadata["total_refunded_amount"])
		return nil
	})

	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundCompleted, "").
		Return(nil)

	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   partialPayment.ID.String(),
		AmountCents: 15000,
		Reason:      "Second partial refund",
	}
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusPartiallyRefunded, response.Status)
}

func TestRefundPaymentUsecase_Success_FinalPartialRefundBecomesFull(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)
	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	partialPayment := createPartiallyRefundedPayment(shop.ID, 50000, 20000)
	after := paymentAfterReserve(partialPayment, 30000, entity.PaymentStatusRefunded)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), partialPayment.ID).Return(partialPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), partialPayment.ID, int64(30000), entity.PaymentStatusRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(30000), gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), partialPayment.ID).Return(after, nil)
	mockPaymentRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, p *entity.Payment) error {
		assert.Equal(t, entity.PaymentStatusRefunded, p.Status)
		assert.Equal(t, int64(50000), p.RefundedAmountCents)
		return nil
	})

	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundCompleted, "").
		Return(nil)

	req := &paymentdto.RefundPaymentRequest{
		PaymentID:   partialPayment.ID.String(),
		AmountCents: 30000,
		Reason:      "Final partial refund",
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
	after := paymentAfterReserve(successPayment, 50000, entity.PaymentStatusRefunded)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(successPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), successPayment.ID, int64(50000), entity.PaymentStatusRefunded).
		Return(true, nil)

	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().
		Refund(gomock.Any(), "TXN-123", int64(50000), gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(after, nil)
	mockPaymentRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, payment *entity.Payment) error {
		assert.NotNil(t, payment.Metadata)
		assert.Equal(t, "Customer request", payment.Metadata["last_refund_reason"])
		assert.Equal(t, int64(50000), payment.Metadata["last_refund_amount"])
		assert.Equal(t, int64(50000), payment.Metadata["total_refunded_amount"])
		return nil
	})

	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundCompleted, "").
		Return(nil)

	req := createValidRefundRequest(successPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusRefunded, response.Status)
}

// Concurrent : 2e réservation refusée par AddRefundAmount (ok=false)
func TestRefundPaymentUsecase_ConcurrentReservationRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	uc := paymentusecase.NewRefundPaymentUsecase(mockPaymentRepo, mockRegistry)
	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)
	successPayment := createSuccessfulPayment(shop.ID, 50000)

	mockPaymentRepo.EXPECT().FindByID(gomock.Any(), successPayment.ID).Return(successPayment, nil)

	mockPaymentRepo.EXPECT().
		FindPaymentRefundByIdempotency(gomock.Any(), shop.ID, gomock.Any()).
		Return(nil, nil)

	mockPaymentRepo.EXPECT().
		CreatePaymentRefund(gomock.Any(), gomock.Any()).
		Return(nil)

	mockPaymentRepo.EXPECT().
		AddRefundAmount(gomock.Any(), successPayment.ID, int64(50000), entity.PaymentStatusRefunded).
		Return(false, nil) // course perdue

	mockPaymentRepo.EXPECT().
		UpdatePaymentRefundStatus(gomock.Any(), gomock.Any(), entity.PaymentRefundFailed, gomock.Any()).
		Return(nil)

	req := createValidRefundRequest(successPayment.ID.String())
	_, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds remaining refundable amount")
}
