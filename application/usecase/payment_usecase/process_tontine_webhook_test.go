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

func createTestShop() *entity.Shop {
	return &entity.Shop{
		ID:   uuid.New(),
		Name: "Test Shop",
		Slug: "test-shop",
	}
}

// newTontineWebhookUC crée un usecase avec tous les mocks nécessaires (v4.11.0 : avec txManager)
func newTontineWebhookUC(ctrl *gomock.Controller) (
	*paymentusecase.ProcessTontineWebhookUsecase,
	*mockrepo.MockTontinePaymentRepository,
	*mockrepo.MockTontineGroupRepository,
	*mockrepo.MockTontineParticipantRepository,
	*mockrepo.MockTontineVoucherRepository,
	*mockrepo.MockShopRepository,
	*mockrepo.MockTxManager,
) {
	mockTontinePaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockTontineGroupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	mockTontineParticipantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	mockTontineVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := paymentusecase.NewProcessTontineWebhookUsecase(
		mockTontinePaymentRepo, mockTontineGroupRepo, mockTontineParticipantRepo,
		mockTontineVoucherRepo, mockShopRepo, nil, nil, mockTxManager,
	)

	return uc, mockTontinePaymentRepo, mockTontineGroupRepo, mockTontineParticipantRepo,
		mockTontineVoucherRepo, mockShopRepo, mockTxManager
}

// ============================================================
// TESTS : Parsing errors
// ============================================================

func TestProcessTontineWebhookUsecase_InvalidReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _ := newTontineWebhookUC(ctrl)

	err := uc.Execute(context.Background(), "ORDER:123", "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse tontine reference")
}

func TestProcessTontineWebhookUsecase_InvalidReferenceFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _ := newTontineWebhookUC(ctrl)

	err := uc.Execute(context.Background(), "TONTINE:abc:1", "txn-123", entity.PaymentStatusSuccess)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tontine reference format")
}

// ============================================================
// TESTS : BeginTx Error
// ============================================================

func TestProcessTontineWebhookUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, txManager := newTontineWebhookUC(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	reference := "TONTINE:abc12345:1:xyz67890"
	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

// ============================================================
// TESTS : Payment not found / already done
// ============================================================

func TestProcessTontineWebhookUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, _, _, _, _, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(nil, errors.New("payment not found"))

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine payment not found")
}

func TestProcessTontineWebhookUsecase_PaymentAlreadyDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, _, _, _, _, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	donePayment := createTontinePayment("group-1", 1)
	donePayment.Status = entity.TontinePaymentDone

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(donePayment, nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.NoError(t, err)
}

// ============================================================
// TESTS : Group/Shop not found
// ============================================================

func TestProcessTontineWebhookUsecase_GroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, _, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(nil, errors.New("group not found"))

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tontine group not found")
}

func TestProcessTontineWebhookUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("shop not found"))

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop not found")
}

// ============================================================
// TESTS : Status transitions
// ============================================================

func TestProcessTontineWebhookUsecase_StatusSuccess_MarkDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").Return(nil)

	// checkAndCompleteCycle
	groupRepo.EXPECT().FindByID(gomock.Any(), testGroup.ID).Return(testGroup, nil)
	paymentRepo.EXPECT().CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).Return(2, nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_StatusFailed_MarkFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().UpdateStatus(gomock.Any(), pendingPayment.ID, entity.TontinePaymentFailed).Return(nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusFailed)
	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_StatusUnknown_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusPending)
	assert.NoError(t, err)
}

// ============================================================
// TESTS : Cycle completion
// ============================================================

func TestProcessTontineWebhookUsecase_CycleNotComplete_Waiting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").Return(nil)

	groupRepo.EXPECT().FindByID(gomock.Any(), testGroup.ID).Return(testGroup, nil)
	paymentRepo.EXPECT().CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).Return(3, nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_CycleComplete_GenerateVoucher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, participantRepo, voucherRepo, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()
	beneficiary := createTontineParticipant(testGroup.ID, 1)

	donePay := createTontinePayment(testGroup.ID, 1)
	donePay.Status = entity.TontinePaymentDone

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").Return(nil)

	groupRepo.EXPECT().FindByID(gomock.Any(), testGroup.ID).Return(testGroup, nil)
	paymentRepo.EXPECT().CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).Return(5, nil)
	participantRepo.EXPECT().FindByPosition(gomock.Any(), testGroup.ID, 1).Return(beneficiary, nil)
	voucherRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	paymentRepo.EXPECT().
		FindByGroupAndCycle(gomock.Any(), testGroup.ID, 1).
		Return([]*entity.TontinePayment{donePay}, nil)
	paymentRepo.EXPECT().
		UpdateTontineCommissionStatus(gomock.Any(), donePay.ID, entity.CommissionStatusCollected, nil).
		Return(nil)

	groupRepo.EXPECT().IncrementCycle(gomock.Any(), testGroup.ID).Return(nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_LastCycle_CompleteGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, participantRepo, voucherRepo, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:5:xyz67890"
	pendingPayment := createTontinePayment("group-1", 5)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testGroup.CurrentCycle = 5
	testShop := createTestShop()
	beneficiary := createTontineParticipant(testGroup.ID, 5)

	donePay := createTontinePayment(testGroup.ID, 5)
	donePay.Status = entity.TontinePaymentDone

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").Return(nil)

	groupRepo.EXPECT().FindByID(gomock.Any(), testGroup.ID).Return(testGroup, nil)
	paymentRepo.EXPECT().CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 5).Return(5, nil)
	participantRepo.EXPECT().FindByPosition(gomock.Any(), testGroup.ID, 5).Return(beneficiary, nil)
	voucherRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	paymentRepo.EXPECT().
		FindByGroupAndCycle(gomock.Any(), testGroup.ID, 5).
		Return([]*entity.TontinePayment{donePay}, nil)
	paymentRepo.EXPECT().
		UpdateTontineCommissionStatus(gomock.Any(), donePay.ID, entity.CommissionStatusCollected, nil).
		Return(nil)

	groupRepo.EXPECT().Complete(gomock.Any(), testGroup.ID).Return(nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_GroupNotActive_SkipCompletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, _, _, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testGroup.Status = entity.TontineStatusCompleted
	testShop := createTestShop()

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").Return(nil)

	groupRepo.EXPECT().FindByID(gomock.Any(), testGroup.ID).Return(testGroup, nil)

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.NoError(t, err)
}

func TestProcessTontineWebhookUsecase_VoucherCollision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, paymentRepo, groupRepo, participantRepo, voucherRepo, shopRepo, txManager := newTontineWebhookUC(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	reference := "TONTINE:abc12345:1:xyz67890"
	pendingPayment := createTontinePayment("group-1", 1)
	testGroup := createTontineGroup(uuid.New().String(), 5)
	testShop := createTestShop()
	beneficiary := createTontineParticipant(testGroup.ID, 1)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByReferenceUnscopedForUpdate(gomock.Any(), reference).Return(pendingPayment, nil)
	groupRepo.EXPECT().FindByIDUnscoped(gomock.Any(), "group-1").Return(testGroup, nil)
	shopRepo.EXPECT().FindByID(gomock.Any(), gomock.Any()).Return(testShop, nil)
	paymentRepo.EXPECT().MarkDone(gomock.Any(), pendingPayment.ID, "txn-123").Return(nil)

	groupRepo.EXPECT().FindByID(gomock.Any(), testGroup.ID).Return(testGroup, nil)
	paymentRepo.EXPECT().CountDoneByGroupAndCycle(gomock.Any(), testGroup.ID, 1).Return(5, nil)
	participantRepo.EXPECT().FindByPosition(gomock.Any(), testGroup.ID, 1).Return(beneficiary, nil)
	voucherRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("duplicate voucher code"))

	err := uc.Execute(context.Background(), reference, "txn-123", entity.PaymentStatusSuccess)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save voucher")
}

func TestProcessWebhookUsecase_RecordWebhookError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl) // 🆕 AJOUTÉ
	mockDB := createMockDBExecutor(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)
	mockTontineUC := (*paymentusecase.ProcessTontineWebhookUsecase)(nil)

	uc := paymentusecase.NewProcessWebhookUsecase(
		mockPaymentRepo, mockRegistry, mockTxManager, mockDB, mockShopRepo, // 🆕 mockTxManager ajouté
		nil, mockTontineUC, nil, nil, nil,
	)

	ctx := context.Background()
	mockRegistry.EXPECT().Get(entity.ProviderYengaPay).Return(mockProvider, nil)
	event := createTestWebhookEvent("TXN-123", entity.PaymentStatusSuccess)
	mockProvider.EXPECT().ValidateWebhook(gomock.Any(), gomock.Any(), gomock.Any()).Return(event, nil)
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("database error")).AnyTimes()

	shopID := uuid.New()
	paymentEntity, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentEntity.MarkProcessing()

	testShop := &entity.Shop{ID: shopID, Name: "Test Shop", Slug: "test-shop"}

	mockPaymentRepo.EXPECT().FindByProviderRef(gomock.Any(), entity.ProviderYengaPay, "TXN-123").Return(paymentEntity, nil)
	mockShopRepo.EXPECT().FindByID(gomock.Any(), shopID).Return(testShop, nil)
	mockPaymentRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.Execute(ctx, entity.ProviderYengaPay, []byte("payload"), "signature")
	assert.NoError(t, err)
}
