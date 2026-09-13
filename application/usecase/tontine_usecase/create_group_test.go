package tontineusecase_test

import (
	"context"
	"errors"
	"testing"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

func createTestContextForTontine() (context.Context, uuid.UUID) {
	shopID := uuid.New()
	shop := &entity.Shop{ID: shopID, Name: "Test Shop", IsActive: true}
	return tenant.WithTenant(context.Background(), shop), shopID
}

func validTontineSettings(shopID string) *entity.ProductTontineSettings {
	return &entity.ProductTontineSettings{
		ProductID:             "product-1",
		ShopID:                shopID,
		IsTontineEnabled:      true,
		AllowCommercialCircle: true,
		AllowCorporateCircle:  true,
		AllowFamilyCircle:     true,
		MinParticipants:       2,
		MaxParticipants:       10,
	}
}

func validProduct() *entity.Product {
	return &entity.Product{
		ID:         "product-1",
		Name:       "Test Product",
		PriceCents: 10000,
		Stock:      5,
	}
}

func validCreateGroupRequest() *tontineusecase.CreateGroupRequest {
	return &tontineusecase.CreateGroupRequest{
		Name:        "Test Group", // 🆕 AJOUTÉ
		ProductID:   "product-1",
		CircleType:  entity.TontineCircleCommercial,
		TotalCycles: 5,
	}
}

// ============================================================
// TESTS : Validate()
// ============================================================

func TestCreateGroupRequest_Validate_Success(t *testing.T) {
	req := validCreateGroupRequest()
	assert.NoError(t, req.Validate())
}

func TestCreateGroupRequest_Validate_EmptyName(t *testing.T) {
	req := validCreateGroupRequest()
	req.Name = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestCreateGroupRequest_Validate_EmptyProductID(t *testing.T) {
	req := validCreateGroupRequest()
	req.ProductID = ""
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product_id")
}

func TestCreateGroupRequest_Validate_InvalidCircleType(t *testing.T) {
	req := validCreateGroupRequest()
	req.CircleType = "INVALID"
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circle type")
}

func TestCreateGroupRequest_Validate_TooFewCycles(t *testing.T) {
	req := validCreateGroupRequest()
	req.TotalCycles = 1
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2")
}

func TestCreateGroupRequest_Validate_TooManyCycles(t *testing.T) {
	req := validCreateGroupRequest()
	req.TotalCycles = 51
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceed 50")
}

// ============================================================
// TESTS : Execute()
// ============================================================

func TestCreateTontineGroupUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx := context.Background()
	req := validCreateGroupRequest()

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestCreateTontineGroupUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()
	req.ProductID = ""

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "validation error")
	_ = shopID
}

func TestCreateTontineGroupUsecase_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "product not found")
	_ = shopID
}

func TestCreateTontineGroupUsecase_SettingsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "tontine settings not found")
	_ = shopID
}

func TestCreateTontineGroupUsecase_TontineDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()

	settings := validTontineSettings(shopID.String())
	settings.IsTontineEnabled = false

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(settings, nil)

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestCreateTontineGroupUsecase_CircleTypeNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()

	settings := validTontineSettings(shopID.String())
	settings.AllowCommercialCircle = false

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(settings, nil)

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestCreateTontineGroupUsecase_InvalidParticipantCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()
	req.TotalCycles = 3

	settings := validTontineSettings(shopID.String())
	settings.MinParticipants = 5

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(settings, nil)

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "invalid participant count")
}

func TestCreateTontineGroupUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()
	req.CreatorCustomerID = "customer-1"

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(validTontineSettings(shopID.String()), nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "customer-1").Return(nil, errors.New("not found"))

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "customer not found")
}

func TestCreateTontineGroupUsecase_CustomerKYCNotVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()
	req.CreatorCustomerID = "customer-1"

	customer := &entity.Customer{ID: "customer-1", KYCLevel: entity.KYCLevelPending}

	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(validTontineSettings(shopID.String()), nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "customer-1").Return(customer, nil)

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "cannot participate")
}

func TestCreateTontineGroupUsecase_SaveGroupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()

	// ✅ FIX : FindByID peut être appelé 2 fois (validation + création)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil).AnyTimes()
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(validTontineSettings(shopID.String()), nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	groupRepo.EXPECT().WithTX(mockTx).Return(groupRepo)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "failed to save group")
}

func TestCreateTontineGroupUsecase_Success_MerchantCreator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()

	// ✅ FIX : FindByID peut être appelé 2 fois
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil).AnyTimes()
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(validTontineSettings(shopID.String()), nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	groupRepo.EXPECT().WithTX(mockTx).Return(groupRepo)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)

	group, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, entity.TontineCreatorMerchant, group.CreatorType)
	assert.Nil(t, group.CreatorCustomerID)
}

func TestCreateTontineGroupUsecase_Success_CustomerCreator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()
	req.CreatorCustomerID = "customer-1"

	customer := &entity.Customer{ID: "customer-1", KYCLevel: entity.KYCLevelVerified}
	// 🆕 Mock du score pour qu'il soit Silver et puisse créer
	score := entity.NewCustomerReliabilityScoreWithKYC("customer-1")
	scoreRepo.EXPECT().FindByCustomerID(gomock.Any(), "customer-1").Return(score, nil)

	// ✅ FIX : FindByID peut être appelé 2 fois
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil).AnyTimes()
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(validTontineSettings(shopID.String()), nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "customer-1").Return(customer, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	groupRepo.EXPECT().WithTX(mockTx).Return(groupRepo)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, g *entity.TontineGroup) error {
			g.ID = "group-uuid-123"
			return nil
		},
	)
	participantRepo.EXPECT().WithTX(mockTx).Return(participantRepo)
	participantRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)

	group, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, entity.TontineCreatorCustomer, group.CreatorType)
	assert.NotNil(t, group.CreatorCustomerID)
	assert.Equal(t, "customer-1", *group.CreatorCustomerID)
}

func TestCreateTontineGroupUsecase_AddParticipantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mockrepo.NewMockTontineGroupRepository(ctrl)
	participantRepo := mockrepo.NewMockTontineParticipantRepository(ctrl)
	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	scoreRepo := mockrepo.NewMockCustomerReliabilityScoreRepository(ctrl) // 🆕 AJOUTÉ
	txManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	uc := tontineusecase.NewCreateTontineGroupUsecase(groupRepo, participantRepo, settingsRepo, productRepo, customerRepo, scoreRepo, txManager)

	ctx, shopID := createTestContextForTontine()
	req := validCreateGroupRequest()
	req.CreatorCustomerID = "customer-1"

	customer := &entity.Customer{ID: "customer-1", KYCLevel: entity.KYCLevelVerified}
	// 🆕 Mock du score
	score := entity.NewCustomerReliabilityScoreWithKYC("customer-1")
	scoreRepo.EXPECT().FindByCustomerID(gomock.Any(), "customer-1").Return(score, nil)

	// ✅ FIX : FindByID peut être appelé 2 fois
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(), nil).AnyTimes()
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(validTontineSettings(shopID.String()), nil)
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "customer-1").Return(customer, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	groupRepo.EXPECT().WithTX(mockTx).Return(groupRepo)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, g *entity.TontineGroup) error {
			g.ID = "group-uuid-123"
			return nil
		},
	)
	participantRepo.EXPECT().WithTX(mockTx).Return(participantRepo)
	participantRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	group, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, group)
	assert.Contains(t, err.Error(), "failed to add creator as participant")
}
