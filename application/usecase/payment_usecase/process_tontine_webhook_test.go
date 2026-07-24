package paymentusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.9 : TESTS UNITAIRES - PROCESS TONTINE WEBHOOK USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTontinePayment crée un paiement tontine de test
func createTontinePayment(groupID string, cycleNumber int) *entity.TontinePayment {
	return &entity.TontinePayment{
		ID:          uuid.New().String(),
		GroupID:     groupID,
		CycleNumber: cycleNumber,
		AmountCents: 50000,
		Status:      entity.TontinePaymentPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// createTontineGroup crée un groupe tontine de test
func createTontineGroup(shopID string, totalCycles int) *entity.TontineGroup {
	return &entity.TontineGroup{
		ID:           uuid.New().String(),
		ShopID:       shopID,
		TotalCycles:  totalCycles,
		CurrentCycle: 1,
		Status:       entity.TontineStatusActive,
		ProductID:    uuid.New().String(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// createTontineParticipant crée un participant tontine de test
func createTontineParticipant(groupID string, payoutPosition int) *entity.TontineParticipant {
	return &entity.TontineParticipant{
		ID:             uuid.New().String(),
		GroupID:        groupID,
		CustomerID:     uuid.New().String(),
		PayoutPosition: payoutPosition,
		Status:         entity.ParticipantStatusActive,
		JoinedAt:       time.Now(),
	}
}

// createTestShop crée un shop de test
func createTestShop() *entity.Shop {
	return &entity.Shop{
		ID:   uuid.New(),
		Name: "Test Shop",
		Slug: "test-shop",
	}
}

// ============================================================
// TESTS : ProcessTontineWebhookUsecase - Parsing errors
// ============================================================

func TestProcessTontineWebhookUsecase_InvalidReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()

	err := uc.Execute(ctx, "ORDER:123", "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse tontine reference")
}

func TestProcessTontineWebhookUsecase_InvalidReferenceFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()

	err := uc.Execute(ctx, "TONTINE:abc:1", "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tontine reference format")
}

// ============================================================
// TESTS : ProcessTontineWebhookUsecase - Payment not found
// ============================================================

func TestProcessTontineWebhookUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(nil, errors.New("payment not found"))

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine payment not found")
}

// ============================================================
// TESTS : ProcessTontineWebhookUsecase - Payment already done
// ============================================================

func TestProcessTontineWebhookUsecase_PaymentAlreadyDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	donePayment := createTontinePayment("group-1", 1)
	donePayment.Status = entity.TontinePaymentDone

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(donePayment, nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.NoError(t, err)
}

// ============================================================
// TESTS : ProcessTontineWebhookUsecase - Group/Shop not found
// ============================================================

func TestProcessTontineWebhookUsecase_GroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(nil, errors.New("group not found"))

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine group not found")
}

func TestProcessTontineWebhookUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("shop not found"))

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop not found")
}

// ============================================================
// TESTS : ProcessTontineWebhookUsecase - Status transitions
// ============================================================

func TestProcessTontineWebhookUsecase_StatusSuccess_MarkDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").
		Return(nil)

	// checkAndCompleteCycle - pas encore tous payés
	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), testGroup.ID).
		Return(testGroup, nil)

	mockTontinePaymentRepo.EXPECT().
		CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).
		Return(2, nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_StatusFailed_MarkFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		UpdateStatus(gomock.Any(), pendingPayment.ID, entity.TontinePaymentFailed).
		Return(nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusFailed)

	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_StatusUnknown_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusPending)

	assert.NoError(t, err)
}

// ============================================================
// TESTS : ProcessTontineWebhookUsecase - Cycle completion
// ============================================================

func TestProcessTontineWebhookUsecase_CycleNotComplete_Waiting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), testGroup.ID).
		Return(testGroup, nil)

	mockTontinePaymentRepo.EXPECT().
		CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).
		Return(3, nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_CycleComplete_GenerateVoucher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()
	beneficiary := createTontineParticipant(testGroup.ID, 1)

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), testGroup.ID).
		Return(testGroup, nil)

	mockTontinePaymentRepo.EXPECT().
		CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).
		Return(5, nil)

	mockTontineParticipantRepo.EXPECT().
		FindByPosition(gomock.Any(), testGroup.ID, 1).
		Return(beneficiary, nil)

	mockTontineVoucherRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		IncrementCycle(gomock.Any(), testGroup.ID).
		Return(nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_LastCycle_CompleteGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:5:xyz67890"

	pendingPayment := createTontinePayment("group-1", 5)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testGroup.CurrentCycle = 5
	testShop := createTestShop()
	beneficiary := createTontineParticipant(testGroup.ID, 5)

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), testGroup.ID).
		Return(testGroup, nil)

	mockTontinePaymentRepo.EXPECT().
		CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 5).
		Return(5, nil)

	mockTontineParticipantRepo.EXPECT().
		FindByPosition(gomock.Any(), testGroup.ID, 5).
		Return(beneficiary, nil)

	mockTontineVoucherRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		Complete(gomock.Any(), testGroup.ID).
		Return(nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_GroupNotActive_SkipCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testGroup.Status = entity.TontineStatusCompleted
	testShop := createTestShop()

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), testGroup.ID).
		Return(testGroup, nil)

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_VoucherCollision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo,
		mockTontineGroupRepo,
		mockTontineParticipantRepo,
		mockTontineVoucherRepo,
		mockShopRepo,
	)

	ctx := context.Background()
	reference := "TONTINE:abc12345:1:xyz67890"

	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()
	beneficiary := createTontineParticipant(testGroup.ID, 1)

	mockTontinePaymentRepo.EXPECT().
		FindByReference(gomock.Any(), reference).
		Return(pendingPayment, nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), "group-1").
		Return(testGroup, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), gomock.Any()).
		Return(testShop, nil)

	mockTontinePaymentRepo.EXPECT().
		MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").
		Return(nil)

	mockTontineGroupRepo.EXPECT().
		FindByID(gomock.Any(), testGroup.ID).
		Return(testGroup, nil)

	mockTontinePaymentRepo.EXPECT().
		CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).
		Return(5, nil)

	mockTontineParticipantRepo.EXPECT().
		FindByPosition(gomock.Any(), testGroup.ID, 1).
		Return(beneficiary, nil)

	// ✅ Mock : Create échoue (collision de voucher)
	mockTontineVoucherRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("duplicate voucher code"))

	err := uc.Execute(ctx, reference, "txn-123", entity.PaymentStatusSuccess)

	// Le usecase retourne l'erreur de Create
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save voucher")
}

func TestProcessWebhookUsecase_RecordWebhookError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	// 🆕 CORRECTION : Ajout des 2 nouveaux paramètres nil (shopSettingsRepo et walletUpdater)
	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo,
		mockRegistry,
		mockDB,
		mockShopRepo,
		nil, // shopSettingsRepo
		mockTontineUC,
		nil, // creditUpdater
		nil, // walletUpdater
	)

	ctx := context.Background()

	mockRegistry.EXPECT().
		Get(entity.ProviderYengaPay).
		Return(mockProvider, nil)

	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)

	mockProvider.EXPECT().
		ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(event, nil)

	// ✅ Mock : recordWebhook échoue (DB error)
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database error"))

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

	// L'erreur de recordWebhook ne doit pas bloquer le traitement
	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")

	assert.NoError(t, err) // Continue malgré l'erreur d'audit
}
