package tontineusecase_test

import (
	"context"
	"errors"
	"testing"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/entity"
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
		CircleType:          entity.TontineCircleFamily,
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

func validCommissionRate() *entity.CommissionRate {
	return &entity.CommissionRate{
		RateBps: 150,
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
	*mockrepo.MockCommissionRateRepository,
	*mockrepo.MockTxManager,
	*mockusecase.MockPaymentRegistry,
) {
	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	paymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	rateRepo := mockrepo.NewMockCommissionRateRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	paymentRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := tontineusecase.NewPayCycleUsecase(groupRepo, participantRepo, paymentRepo, rateRepo, txManager, paymentRegistry)
	return uc, groupRepo, participantRepo, paymentRepo, rateRepo, txManager, paymentRegistry
}

func TestPayCycleUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _, _ := newPayCycleUsecase(ctrl)

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

	uc, _, _, _, _, _, _ := newPayCycleUsecase(ctrl)

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

	uc, groupRepo, _, _, _, _, _ := newPayCycleUsecase(ctrl)

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

	uc, groupRepo, _, _, _, _, _ := newPayCycleUsecase(ctrl)

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

	uc, groupRepo, _, _, _, _, _ := newPayCycleUsecase(ctrl)

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

	uc, groupRepo, participantRepo, _, _, _, _ := newPayCycleUsecase(ctrl)

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

	uc, groupRepo, participantRepo, _, _, _, _ := newPayCycleUsecase(ctrl)

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

// 🆕 v4.11.0 : Test avec FOR UPDATE - déjà payé pour ce cycle
func TestPayCycleUsecase_AlreadyPaidForCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, _, txManager, _ := newPayCycleUsecase(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	existingPayment := &entity.TontinePayment{ID: "payment-1", Status: entity.TontinePaymentDone}

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	// 🛡️ v4.11.0 : Maintenant la transaction est démarrée AVANT la vérification
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByParticipantAndCycleForUpdate(gomock.Any(), participant.ID, group.CurrentCycle).Return(existingPayment, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already paid")
}

// 🆕 v4.11.0 : Test avec FOR UPDATE - erreur GetRate
func TestPayCycleUsecase_GetRateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, rateRepo, txManager, _ := newPayCycleUsecase(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	// 🛡️ v4.11.0 : Transaction + FOR UPDATE avant la récupération du taux
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	paymentRepo.EXPECT().WithTX(mockTx).Return(paymentRepo)
	paymentRepo.EXPECT().FindByParticipantAndCycleForUpdate(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	rateRepo.EXPECT().GetDefaultRate(gomock.Any(), shopID.String(), entity.TransactionTypeTontineFamily).Return(nil, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get commission rate")
}

// 🆕 v4.11.0 : Test avec FOR UPDATE - erreur BeginTx (nouvelle position)
func TestPayCycleUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, _, _, txManager, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	// 🛡️ v4.11.0 : BeginTx est maintenant appelé AVANT le FindForUpdate
	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db down"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestPayCycleUsecase_SaveError_RollbackCalled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, rateRepo, txManager, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	// 🛡️ v4.11.0 : BeginTx AVANT la vérification FOR UPDATE
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().FindByParticipantAndCycleForUpdate(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	rateRepo.EXPECT().GetDefaultRate(gomock.Any(), shopID.String(), entity.TransactionTypeTontineFamily).Return(validCommissionRate(), nil)
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

	uc, groupRepo, participantRepo, paymentRepo, rateRepo, txManager, _ := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // ✅ FIX : Accepter Rollback après Commit
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().FindByParticipantAndCycleForUpdate(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	rateRepo.EXPECT().GetDefaultRate(gomock.Any(), shopID.String(), entity.TransactionTypeTontineFamily).Return(validCommissionRate(), nil)
	txPaymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestPayCycleUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, rateRepo, txManager, paymentRegistry := newPayCycleUsecase(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)
	mockProvider := mockusecase.NewMockProvider(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // ✅ FIX : Accepter Rollback après Commit
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().FindByParticipantAndCycleForUpdate(gomock.Any(), participant.ID, group.CurrentCycle).Return(nil, nil)
	rateRepo.EXPECT().GetDefaultRate(gomock.Any(), shopID.String(), entity.TransactionTypeTontineFamily).Return(validCommissionRate(), nil)
	txPaymentRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)

	// ✅ FIX : Mock SetProviderIntentID appelé après InitiatePayment
	paymentRepo.EXPECT().SetProviderIntentID(gomock.Any(), gomock.Any(), "prov-ref-123").Return(nil)

	paymentRegistry.EXPECT().GetAvailable(gomock.Any(), entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().InitiatePayment(gomock.Any(), gomock.Any()).Return(&payment.PaymentResponse{
		ProviderRef: "prov-ref-123",
		USSDCode:    "*144*123#",
	}, nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, group.CurrentCycle, resp.CycleNumber)
	assert.Equal(t, group.AmountPerCycleCents, resp.AmountCents)
	assert.Equal(t, entity.TontinePaymentProcessing, resp.Status)
	assert.NotEmpty(t, resp.ProviderRef)
}

// 🆕 v4.11.0 : Test avec FOR UPDATE - paiement existant réutilisé
func TestPayCycleUsecase_Success_ExistingPaymentNotDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, groupRepo, participantRepo, paymentRepo, _, txManager, paymentRegistry := newPayCycleUsecase(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	txPaymentRepo := mockrepo.NewMockTontinePaymentRepository(ctrl)

	ctx, shopID := createTestContextForTontine()
	req := validPayCycleRequest()

	group := activeTontineGroup(shopID.String())
	participant := activeTontineParticipant()
	failedPayment := &entity.TontinePayment{ID: "payment-old", Status: entity.TontinePaymentFailed}
	mockProvider := mockusecase.NewMockProvider(ctrl)

	groupRepo.EXPECT().FindByID(gomock.Any(), req.GroupID).Return(group, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(participant, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // ✅ FIX : Accepter Rollback après Commit
	paymentRepo.EXPECT().WithTX(mockTx).Return(txPaymentRepo)
	txPaymentRepo.EXPECT().FindByParticipantAndCycleForUpdate(gomock.Any(), participant.ID, group.CurrentCycle).Return(failedPayment, nil)
	txPaymentRepo.EXPECT().UpdateStatus(gomock.Any(), "payment-old", entity.TontinePaymentProcessing).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)

	// ✅ FIX : Mock SetProviderIntentID appelé après InitiatePayment
	paymentRepo.EXPECT().SetProviderIntentID(gomock.Any(), "payment-old", "prov-ref-123").Return(nil)

	paymentRegistry.EXPECT().GetAvailable(gomock.Any(), entity.ProviderYengaPay).Return(mockProvider, nil)
	mockProvider.EXPECT().InitiatePayment(gomock.Any(), gomock.Any()).Return(&payment.PaymentResponse{
		ProviderRef: "prov-ref-123",
	}, nil)

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
	assert.Contains(t, err.Error(), "db error")
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
