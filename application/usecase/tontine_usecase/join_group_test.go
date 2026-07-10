package tontineusecase_test

import (
	"context"
	"errors"
	"testing"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

func validJoinGroupRequest() *tontineusecase.JoinGroupRequest {
	return &tontineusecase.JoinGroupRequest{
		InviteCode: "ABCD1234",
		CustomerID: "customer-1",
	}
}

func pendingTontineGroup(shopID string) *entity.TontineGroup {
	return &entity.TontineGroup{
		ID:          "group-1",
		ShopID:      shopID,
		Status:      entity.TontineStatusPendingMembers,
		TotalCycles: 5,
		InviteCode:  "ABCD1234",
	}
}

// ============================================================
// TESTS : Validate()
// ============================================================

func TestJoinGroupRequest_Validate_Success(t *testing.T) {
	req := validJoinGroupRequest()
	assert.NoError(t, req.Validate())
}

func TestJoinGroupRequest_Validate_EmptyInviteCode(t *testing.T) {
	req := validJoinGroupRequest()
	req.InviteCode = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invite_code")
}

func TestJoinGroupRequest_Validate_EmptyCustomerID(t *testing.T) {
	req := validJoinGroupRequest()
	req.CustomerID = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id")
}

// ============================================================
// TESTS : Execute()
// ============================================================

func TestJoinTontineGroupUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx := context.Background()
	req := validJoinGroupRequest()

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestJoinTontineGroupUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, _ := createTestContextForTontine()
	req := validJoinGroupRequest()
	req.InviteCode = ""

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestJoinTontineGroupUsecase_GroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, _ := createTestContextForTontine()
	req := validJoinGroupRequest()

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invitation invalide")
}

func TestJoinTontineGroupUsecase_ShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, _ := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup("other-shop-id")
	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "n'existe pas dans cette boutique")
}

func TestJoinTontineGroupUsecase_GroupNotPendingMembers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	group.Status = entity.TontineStatusActive

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "n'accepte plus")
}

func TestJoinTontineGroupUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "client introuvable")
}

func TestJoinTontineGroupUsecase_CustomerKYCNotVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelPending}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "impossible de rejoindre")
}

func TestJoinTontineGroupUsecase_AlreadyMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}
	existing := &entity.TontineParticipant{ID: "participant-existing"}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(existing, nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "déjà membre")
}

func TestJoinTontineGroupUsecase_CountByGroupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, nil)
	participantRepo.EXPECT().CountByGroup(gomock.Any(), group.ID).Return(0, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "erreur vérification membres")
}

func TestJoinTontineGroupUsecase_GroupFull(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	group.TotalCycles = 3
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, nil)
	participantRepo.EXPECT().CountByGroup(gomock.Any(), group.ID).Return(3, nil) // déjà complet

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "complet")
}

func TestJoinTontineGroupUsecase_AddParticipantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	group.TotalCycles = 5
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, nil)
	participantRepo.EXPECT().CountByGroup(gomock.Any(), group.ID).Return(1, nil)
	participantRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "erreur ajout participant")
}

func TestJoinTontineGroupUsecase_Success_NotFull(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	group.TotalCycles = 5
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, nil)
	participantRepo.EXPECT().CountByGroup(gomock.Any(), group.ID).Return(1, nil) // 1/5, pas encore complet
	participantRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)
	// Pas de Start attendu car groupe pas encore complet

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.GroupReady)
	assert.Equal(t, 2, resp.Participant.PayoutPosition) // currentCount(1) + 1
}

func TestJoinTontineGroupUsecase_Success_GroupBecomesFull_StartsGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	group.TotalCycles = 3
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, nil)
	participantRepo.EXPECT().CountByGroup(gomock.Any(), group.ID).Return(2, nil) // 2/3 -> devient 3/3
	participantRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)
	groupRepo.EXPECT().Start(gomock.Any(), group.ID).Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.GroupReady)
	assert.Equal(t, entity.TontineStatusActive, resp.Group.Status)
}

func TestJoinTontineGroupUsecase_StartGroupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := tontineusecase.NewJoinTontineGroupUsecase(groupRepo, participantRepo, customerRepo)

	ctx, shopID := createTestContextForTontine()
	req := validJoinGroupRequest()

	group := pendingTontineGroup(shopID.String())
	group.TotalCycles = 3
	customer := &entity.Customer{ID: req.CustomerID, KYCLevel: entity.KYCLevelVerified}

	groupRepo.EXPECT().FindByInviteCode(gomock.Any(), req.InviteCode).Return(group, nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), req.CustomerID).Return(customer, nil)
	participantRepo.EXPECT().FindByGroupAndCustomer(gomock.Any(), group.ID, req.CustomerID).Return(nil, nil)
	participantRepo.EXPECT().CountByGroup(gomock.Any(), group.ID).Return(2, nil)
	participantRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)
	groupRepo.EXPECT().Start(gomock.Any(), group.ID).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "erreur démarrage groupe")
}
