package tontineusecase_test

import (
	"context"
	"errors"
	"testing"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

func validPayCycleRequest() *tontineusecase.PayCycleRequest {
	return &tontineusecase.PayCycleRequest{
		GroupID:     "group-0000000001",
		CustomerID:  "customer-1",
		Operator:    "orange_money",
		PhoneNumber: "+22670123456",
		Flow:        "indirect",
	}
}

func activeTontineGroup(shopID string) *entity.TontineGroup {
	return &entity.TontineGroup{
		ID:                  "group-0000000001",
		ShopID:              shopID,
		Status:              entity.TontineStatusActive,
		TotalCycles:         5,
		CurrentCycle:        2,
		AmountPerCycleCents: 20000,
	}
}

func activeTontineParticipant() *entity.TontineParticipant {
	return &entity.TontineParticipant{
		ID:         "participant-1",
		GroupID:    "group-1",
		CustomerID: "customer-1",
		Status:     entity.ParticipantStatusActive,
	}
}

func validShopPaymentSettings() *entity.ShopPaymentSettings {
	return &entity.ShopPaymentSettings{
		TontineEnabled:        true,
		TontineCommissionRate: 250,
	}
}

// ============================================================
// TESTS : Validate()
// ============================================================

func TestPayCycleRequest_Validate_Success(t *testing.T) {
	req := validPayCycleRequest()
	assert.NoError(t, req.Validate())
}

func TestPayCycleRequest_Validate_EmptyGroupID(t *testing.T) {
	req := validPayCycleRequest()
	req.GroupID = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group_id")
}

func TestPayCycleRequest_Validate_EmptyCustomerID(t *testing.T) {
	req := validPayCycleRequest()
	req.CustomerID = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id")
}

func TestPayCycleRequest_Validate_EmptyOperator(t *testing.T) {
	req := validPayCycleRequest()
	req.Operator = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator")
}

func TestPayCycleRequest_Validate_EmptyPhoneNumber(t *testing.T) {
	req := validPayCycleRequest()
	req.PhoneNumber = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "phone_number")
}

func TestPayCycleRequest_Validate_DefaultFlow(t *testing.T) {
	req := validPayCycleRequest()
	req.Flow = ""
	err := req.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "indirect", req.Flow)
}

func TestPayCycleRequest_Validate_InvalidFlow(t *testing.T) {
	req := validPayCycleRequest()
	req.Flow = "bogus"
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flow must be")
}

// ============================================================
// TESTS : PayCycleUsecase.Execute()
// ============================================================

func newPayCycleUsecase(ctrl *gomock.Controller) (
	*tontineusecase.PayCycleUsecase,
	*mockrepo.MockTontineGroupRepository,
	*mockrepo.MockTontineParticipantRepository,
	*mockrepo.MockTontinePaymentRepository,
	*mockrepo.MockShopRepository,
	*mockrepo.MockTxManager,
) {
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	shopRepo := mockrepo.NewMockShopRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewPayCycleUsecase(groupRepo, participantRepo, paymentRepo, shopRepo, txManager)
	return uc, groupRepo, participantRepo, paymentRepo, shopRepo, txManager
}

func TestPayCycleUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _ := newPayCycleUsecase(ctrl)

	ctx := context.Background()
	req := validPayCycleRequest()

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestPayCycleUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _ := newPayCycleUsecase(ctrl)

	ctx, _ := createTestContextForTontine()
	req := validPayCycleRequest()
	req.GroupID = ""

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestPayCycleUsecase_GroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, _, _, _, _ := newPayCycleUsecase(ctrl)

	ctx, _ := createTestContextForTontine()
	req := validPayCycleRequest()

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "group not found")
}

func TestPayCycleUsecase_ShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, _, _, _, _ := newPayCycleUsecase(ctrl)

	ctx, _ := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup("other-shop-id")
	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestPayCycleUsecase_GroupNotActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, _, _, _, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	group.Status = entity.TontineStatusPendingMembers
	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not active")
}

func TestPayCycleUsecase_ParticipantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, _, _, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not a participant")
}

func TestPayCycleUsecase_ParticipantNotActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, _, _, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	participant.Status = entity.ParticipantStatusSuspended

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not active")
}

func TestPayCycleUsecase_AlreadyPaidForCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, _, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	existingPayment := &entity.TontinePayment{ID: "payment-1", Status: entity.TontinePaymentDone}

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(existingPayment, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already paid")
}

func TestPayCycleUsecase_GetShopSettingsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, shopRepo, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shopID).Return(nil, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get shop settings")
}

func TestPayCycleUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, shopRepo, txManager := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shopID).Return(validShopPaymentSettings(), nil)
	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db down"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestPayCycleUsecase_SaveError_RollbackCalled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, shopRepo, txManager := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shopID).Return(validShopPaymentSettings(), nil)
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to save payment")
}

func TestPayCycleUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, shopRepo, txManager := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shopID).Return(validShopPaymentSettings(), nil)
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestPayCycleUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, shopRepo, txManager := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shopID).Return(validShopPaymentSettings(), nil)
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, group.CurrentCycle, resp.CycleNumber)
	assert.Equal(t, group.AmountPerCycleCents, resp.AmountCents)
	assert.Equal(t, entity.TontinePaymentProcessing, resp.Status)
	assert.NotEmpty(t, resp.ProviderRef)
}

func TestPayCycleUsecase_Success_ExistingPaymentNotDone(t *testing.T) {
	// Un paiement existant mais pas DONE (ex: FAILED) ne doit pas bloquer une nouvelle tentative
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, shopRepo, txManager := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	failedPayment := &entity.TontinePayment{ID: "payment-old", Status: entity.TontinePaymentFailed}
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)
	paymentRepo.EXPECT().FindByParticipantAndCycle(gomock.Any(), participant.ID, group.CurrentCycle).Return(failedPayment, nil)
	shopRepo.EXPECT().GetPaymentSettings(gomock.Any(), shopID).Return(validShopPaymentSettings(), nil)
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// ============================================================
// TESTS : ListCustomerPaymentsUsecase
// ============================================================

func TestListCustomerPaymentsUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	uc := tontineusecase.NewListCustomerPaymentsUsecase(paymentRepo, groupRepo)

	ctx := context.Background()

	payments, err := uc.Execute(ctx, "group-1", "customer-1")

	assert.Error(t, err)
	assert.Nil(t, payments)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestListCustomerPaymentsUsecase_GroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	uc := tontineusecase.NewListCustomerPaymentsUsecase(paymentRepo, groupRepo)

	ctx, _ := createTestContextForTontine()

	groupRepo.EXPECT().FindByID(gomock.Any(), "group-1").Return(nil, errors.New("not found"))

	payments, err := uc.Execute(ctx, "group-1", "customer-1")

	assert.Error(t, err)
	assert.Nil(t, payments)
	assert.Contains(t, err.Error(), "group not found")
}

func TestListCustomerPaymentsUsecase_ShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	uc := tontineusecase.NewListCustomerPaymentsUsecase(paymentRepo, groupRepo)

	ctx, _ := createTestContextForTontine()
	group := activeTontineGroup("other-shop-id")

	groupRepo.EXPECT().FindByID(gomock.Any(), "group-1").Return(group, nil)

	payments, err := uc.Execute(ctx, "group-1", "customer-1")

	assert.Error(t, err)
	assert.Nil(t, payments)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestListCustomerPaymentsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	uc := tontineusecase.NewListCustomerPaymentsUsecase(paymentRepo, groupRepo)

	ctx, shopID := createTestContextForTontine()
	group := activeTontineGroup(shopID.String())

	groupRepo.EXPECT().FindByID(gomock.Any(), "group-1").Return(group, nil)
	paymentRepo.EXPECT().FindByCustomerAndGroup(gomock.Any(), "customer-1", "group-1").Return(nil, errors.New("db error"))

	payments, err := uc.Execute(ctx, "group-1", "customer-1")

	assert.Error(t, err)
	assert.Nil(t, payments)
	assert.Contains(t, err.Error(), "failed to fetch payments")
}

func TestListCustomerPaymentsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	uc := tontineusecase.NewListCustomerPaymentsUsecase(paymentRepo, groupRepo)

	ctx, shopID := createTestContextForTontine()
	group := activeTontineGroup(shopID.String())
	expected := []*entity.TontinePayment{
		{ID: "payment-1"},
		{ID: "payment-2"},
	}

	groupRepo.EXPECT().FindByID(gomock.Any(), "group-1").Return(group, nil)
	paymentRepo.EXPECT().FindByCustomerAndGroup(gomock.Any(), "customer-1", "group-1").Return(expected, nil)

	payments, err := uc.Execute(ctx, "group-1", "customer-1")

	assert.NoError(t, err)
	assert.Len(t, payments, 2)
}

// ============================================================
// TESTS : GenerateTontineReference / ParseTontineReference
// ============================================================

func TestGenerateTontineReference_ValidUUIDs(t *testing.T) {
	groupID := uuid.New().String()
	participantID := uuid.New().String()

	ref := tontineusecase.GenerateTontineReference(groupID, 3, participantID)

	assert.Contains(t, ref, "TONTINE:")
	assert.Contains(t, ref, ":3:")
}

func TestGenerateTontineReference_InvalidUUIDs_Fallback(t *testing.T) {
	ref := tontineusecase.GenerateTontineReference("short-id", 2, "another-id")

	assert.Contains(t, ref, "TONTINE:")
	assert.Contains(t, ref, ":2:")
}

func TestParseTontineReference_Success(t *testing.T) {
	groupPrefix, cycle, participantPrefix, err := tontineusecase.ParseTontineReference("TONTINE:abcd1234:5:efgh5678")

	assert.NoError(t, err)
	assert.Equal(t, "abcd1234", groupPrefix)
	assert.Equal(t, 5, cycle)
	assert.Equal(t, "efgh5678", participantPrefix)
}

func TestParseTontineReference_InvalidFormat(t *testing.T) {
	_, _, _, err := tontineusecase.ParseTontineReference("NOT-A-VALID-REF")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tontine reference format")
}
