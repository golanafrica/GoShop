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
// 🆕 v4.4.17 : TESTS UNITAIRES - COUVERTURE COMPLÈTE
// ============================================================

// ============================================================
// TESTS : RejectCreditUsecase.Execute() - Branches manquantes (63.6% → ~90%)
// ============================================================

func TestRejectCreditUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewRejectCreditUsecase(appRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "app-1",
		ReviewedBy:      "merchant-1",
		RejectionReason: "Score too low",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestRejectCreditUsecase_ShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewRejectCreditUsecase(appRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "app-1",
		ReviewedBy:      "merchant-1",
		RejectionReason: "Score too low",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Application d'un autre shop
	app := pendingCreditApplication("app-1", "customer-1", "product-1", "other-shop-id")

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "access denied")
}

func TestRejectCreditUsecase_ApplicationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewRejectCreditUsecase(appRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "app-1",
		ReviewedBy:      "merchant-1",
		RejectionReason: "Score too low",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "application not found")
}

// ============================================================
// TESTS : PayDownPaymentUsecase.Execute() - Branches manquantes (72.9% → ~85%)
// ============================================================

// ============================================================
// TESTS : CanApplyForCredit - Branches manquantes (85.7% → 100%)
// ============================================================

func TestApplyForCreditUsecase_CanApplyForCredit_ExistingApprovedApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, _, scoreRepo, _ := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()

	plan := validCreditPlan("product-1", shopID)
	plan.MinCreditScore = 500
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	score := validCreditScore("customer-1", shopID, 650)
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(score, nil)

	existingApp := &entity.CreditApplication{
		ID:         "existing-app",
		CustomerID: "customer-1",
		ProductID:  "product-1",
		Status:     entity.CreditApplicationApproved,
	}
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), "customer-1", "product-1").Return(existingApp, nil)

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.False(t, canApply)
	assert.Contains(t, message, "already have an approved application")
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Branches manquantes (86.7% → ~90%)
// ============================================================

func TestApplyForCreditUsecase_CreateApplicationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	score := validCreditScore(req.CustomerID, shopID, 650)
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo).AnyTimes()
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), req.CustomerID, shopID).Return(score, nil)

	// ❌ SUPPRIMER : scoreRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	// Car Create() échoue avant que Update() soit appelé

	// Create application échoue
	appRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to save credit application")
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Branches manquantes (87.7% → ~90%)
// ============================================================

func TestConfigureCreditPlanUsecase_CreatePlanError(t *testing.T) {
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
	planRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create credit plan")
	_ = walletRepo // Évite unused variable
	_ = shopID
}

func TestConfigureCreditPlanUsecase_UpdatePlanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, _, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validConfigurePlanRequest("product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

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
	planRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to update credit plan")
}

// ============================================================
// TESTS : SimulateCreditCalculation - Branches manquantes (91.7% → 100%)
// ============================================================

func TestApplyForCreditUsecase_SimulateCreditCalculation_PlanNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, productRepo, _, _ := newApplyForCreditUsecase(ctrl)
	ctx := context.Background()

	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(validProduct("product-1"), nil)

	// Plan n'existe pas
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	resp, err := uc.SimulateCreditCalculation(ctx, "product-1", 6)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credit not available")
}
