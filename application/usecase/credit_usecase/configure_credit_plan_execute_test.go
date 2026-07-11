package creditusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.17 : TESTS UNITAIRES - CONFIGURE CREDIT PLAN USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func validConfigurePlanRequest(productID string) *creditusecase.ConfigureCreditPlanRequest {
	return &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       500,
		PenaltyRateBps:        500,
		MinCreditScore:        500,
	}
}

func newConfigureCreditPlanUsecase(ctrl *gomock.Controller) (
	*creditusecase.ConfigureCreditPlanUsecase,
	*mockrepo.MockCreditPlanRepository,
	*mockrepo.MockProductRepository,
	*mockrepo.MockMerchantWalletRepository,
	*mockrepo.MockTxManager,
) {
	planRepo := mockrepo.NewMockCreditPlanRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewConfigureCreditPlanUsecase(planRepo, productRepo, walletRepo, txManager)
	return uc, planRepo, productRepo, walletRepo, txManager
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Validation & Multi-tenant
// ============================================================

func TestConfigureCreditPlanUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID: "", // ❌ Vide
	}

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestConfigureCreditPlanUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx := context.Background() // ❌ Sans tenant
	req := validConfigurePlanRequest("product-1")

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestConfigureCreditPlanUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Product validation
// ============================================================

func TestConfigureCreditPlanUsecase_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, productRepo, _, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "product not found")
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Create new plan
// ============================================================

func TestConfigureCreditPlanUsecase_CreateNewPlan_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, walletRepo, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))
	planRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, plan *entity.CreditPlan) error {
			plan.ID = uuid.New().String()
			return nil
		},
	)

	// Wallet creation (car IsEnabled = true)
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletRepo.EXPECT().FindByShopID(gomock.Any(), shopID).Return(nil, errors.New("not found"))
	walletRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "created", resp.Action)
	assert.True(t, resp.IsEnabled)
	assert.Equal(t, req.ProductID, resp.ProductID)
	assert.Equal(t, shopID, resp.ShopID)
	assert.Equal(t, 5.0, resp.InterestRatePercent) // 500 bps = 5%
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Update existing plan
// ============================================================

func TestConfigureCreditPlanUsecase_UpdateExistingPlan_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, walletRepo, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	// Plan existant
	existingPlan := &entity.CreditPlan{
		ID:                    uuid.New().String(),
		ProductID:             req.ProductID,
		ShopID:                shopID,
		IsEnabled:             false,
		MinDownPaymentPercent: 10,
		MaxDurationMonths:     6,
		InterestRateBps:       300,
		PenaltyRateBps:        300,
		MinCreditScore:        400,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(existingPlan, nil)
	planRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// ✅ AJOUTER : Mocks walletRepo (car IsEnabled = true)
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletRepo.EXPECT().FindByShopID(gomock.Any(), shopID).Return(entity.NewMerchantWallet(shopID), nil)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "updated", resp.Action)
	assert.True(t, resp.IsEnabled)
	assert.Equal(t, 20, resp.MinDownPaymentPercent)
	assert.Equal(t, 12, resp.MaxDurationMonths)
	assert.Equal(t, 500, resp.InterestRateBps)
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Wallet frozen
// ============================================================

func TestConfigureCreditPlanUsecase_WalletFrozen_ContinuesAnyway(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, walletRepo, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))
	planRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, plan *entity.CreditPlan) error {
			plan.ID = uuid.New().String()
			return nil
		},
	)

	// Wallet gelé
	frozenWallet := entity.NewMerchantWallet(shopID)
	frozenWallet.IsFrozen = true

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletRepo.EXPECT().FindByShopID(gomock.Any(), shopID).Return(frozenWallet, nil)
	// Pas de Create car wallet existe

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err) // ✅ Continue même si wallet gelé
	assert.NotNil(t, resp)
	assert.True(t, resp.IsEnabled)
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Commit error
// ============================================================

func TestConfigureCreditPlanUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, walletRepo, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))
	planRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletRepo.EXPECT().FindByShopID(gomock.Any(), shopID).Return(nil, errors.New("not found"))
	walletRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase - Helpers
// ============================================================

func TestConfigureCreditPlanUsecase_EnableCreditPlan_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, walletRepo, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(validProduct("product-1"), nil)

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))
	planRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, plan *entity.CreditPlan) error {
			plan.ID = uuid.New().String()
			return nil
		},
	)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	walletRepo.EXPECT().FindByShopID(gomock.Any(), shopID).Return(nil, errors.New("not found"))
	walletRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := uc.EnableCreditPlan(ctx, "product-1", 500, 12)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsEnabled)
	assert.Equal(t, "created", resp.Action)
}

func TestConfigureCreditPlanUsecase_DisableCreditPlan_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, _, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	// Plan existant à désactiver
	existingPlan := &entity.CreditPlan{
		ID:                    uuid.New().String(),
		ProductID:             "product-1",
		ShopID:                shopID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       500,
		PenaltyRateBps:        500,
		MinCreditScore:        500,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(existingPlan, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(validProduct("product-1"), nil)

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(existingPlan, nil)
	planRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := uc.DisableCreditPlan(ctx, "product-1")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsEnabled)
	assert.Equal(t, "updated", resp.Action)
}

func TestConfigureCreditPlanUsecase_DisableCreditPlan_PlanNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx := context.Background()

	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	resp, err := uc.DisableCreditPlan(ctx, "product-1")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credit plan not found")
}

func TestConfigureCreditPlanUsecase_GetCreditPlan_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx := context.Background()

	plan := &entity.CreditPlan{
		ID:        uuid.New().String(),
		ProductID: "product-1",
		IsEnabled: true,
	}

	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	result, err := uc.GetCreditPlan(ctx, "product-1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsEnabled)
}

func TestConfigureCreditPlanUsecase_IsCreditEnabled_True(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx := context.Background()

	plan := &entity.CreditPlan{
		ProductID: "product-1",
		IsEnabled: true,
	}

	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	enabled, err := uc.IsCreditEnabled(ctx, "product-1")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestConfigureCreditPlanUsecase_IsCreditEnabled_False_PlanNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx := context.Background()

	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	enabled, err := uc.IsCreditEnabled(ctx, "product-1")
	assert.NoError(t, err) // ✅ Pas d'erreur, juste false
	assert.False(t, enabled)
}

func TestConfigureCreditPlanUsecase_ListCreditPlansByShop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, _, _, _ := newConfigureCreditPlanUsecase(ctrl)
	ctx := context.Background()

	plans := []*entity.CreditPlan{
		{ID: "plan-1", ProductID: "product-1"},
		{ID: "plan-2", ProductID: "product-2"},
	}

	planRepo.EXPECT().FindByShopID(gomock.Any(), "shop-1").Return(plans, nil)

	result, err := uc.ListCreditPlansByShop(ctx, "shop-1")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}
