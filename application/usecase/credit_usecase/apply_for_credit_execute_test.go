package creditusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.17 : TESTS UNITAIRES - APPLY FOR CREDIT USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createCreditTestContext() (context.Context, string) {
	shop := &entity.Shop{
		ID:       uuid.New(),
		Name:     "Test Shop",
		IsActive: true,
	}
	return tenant.WithTenant(context.Background(), shop), shop.ID.String()
}

func validCreditPlan(productID, shopID string) *entity.CreditPlan {
	return &entity.CreditPlan{
		ID:                    uuid.New().String(),
		ProductID:             productID,
		ShopID:                shopID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       500, // 5%
		PenaltyRateBps:        500,
		MinCreditScore:        500,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

func validProduct(productID string) *entity.Product {
	return &entity.Product{
		ID:          productID,
		Name:        "Test Product",
		Description: "Test product description",
		PriceCents:  100000, // 1000 FCFA
		Stock:       10,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func validCreditScore(customerID, shopID string, score int) *entity.CreditScore {
	return &entity.CreditScore{
		ID:             uuid.New().String(),
		CustomerID:     customerID,
		ShopID:         shopID,
		Score:          score,
		TotalContracts: 0,
		LastUpdatedAt:  time.Now(),
	}
}

func validApplyRequest(customerID, productID string) *creditusecase.ApplyForCreditRequest {
	return &creditusecase.ApplyForCreditRequest{
		CustomerID:              customerID,
		ProductID:               productID,
		RequestedDurationMonths: 6,
	}
}

func newApplyForCreditUsecase(ctrl *gomock.Controller) (
	*creditusecase.ApplyForCreditUsecase,
	*mockrepo.MockCreditApplicationRepository,
	*mockrepo.MockCreditPlanRepository,
	*mockrepo.MockProductRepository,
	*mockrepo.MockCreditScoreRepository,
	*mockrepo.MockTxManager,
) {
	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	planRepo := mockrepo.NewMockCreditPlanRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApplyForCreditUsecase(appRepo, planRepo, productRepo, scoreRepo, txManager)
	return uc, appRepo, planRepo, productRepo, scoreRepo, txManager
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Validation & Multi-tenant
// ============================================================

func TestApplyForCreditUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _ := newApplyForCreditUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "", // ❌ Vide
		ProductID:               "product-1",
		RequestedDurationMonths: 6,
	}

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestApplyForCreditUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, _ := newApplyForCreditUsecase(ctrl)
	ctx := context.Background() // ❌ Sans tenant
	req := validApplyRequest("customer-1", "product-1")

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestApplyForCreditUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Product & Plan
// ============================================================

func TestApplyForCreditUsecase_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, productRepo, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "product not found")
}

func TestApplyForCreditUsecase_CreditPlanNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, productRepo, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, _ := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo)
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credit not available")
}

func TestApplyForCreditUsecase_CreditNotEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, productRepo, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	plan.IsEnabled = false // ❌ Désactivé

	planRepo.EXPECT().WithTX(mockTx).Return(planRepo)
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credit is not enabled")
}

func TestApplyForCreditUsecase_DurationExceedsPlanMax(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, productRepo, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	req.RequestedDurationMonths = 24 // ❌ > plan.MaxDurationMonths (12)
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo)
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Existing Applications
// ============================================================

func TestApplyForCreditUsecase_ExistingPendingApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo)
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	existingApp := &entity.CreditApplication{
		ID:         "existing-app",
		CustomerID: req.CustomerID,
		ProductID:  req.ProductID,
		ShopID:     shopID,
		Status:     entity.CreditApplicationPending,
	}

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo)
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(existingApp, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already have a pending application")
}

func TestApplyForCreditUsecase_ExistingApprovedApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, _, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo)
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	existingApp := &entity.CreditApplication{
		ID:         "existing-app",
		CustomerID: req.CustomerID,
		ProductID:  req.ProductID,
		ShopID:     shopID,
		Status:     entity.CreditApplicationApproved,
	}

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo)
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(existingApp, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already have an approved application")
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Credit Score
// ============================================================

func TestApplyForCreditUsecase_CreditScoreTooLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo)
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	plan.MinCreditScore = 600 // Score minimum requis : 600
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo)
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo)
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	// Score trop bas
	score := validCreditScore(req.CustomerID, shopID, 500) // ❌ 500 < 600
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo)
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), req.CustomerID, shopID).Return(score, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credit score too low")
}

func TestApplyForCreditUsecase_CreateDefaultScore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// ✅ AJOUTER .AnyTimes() sur TOUS les WithTX
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	plan.MinCreditScore = 300 // Score minimum bas
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes() // ✅ AnyTimes
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	// Score n'existe pas → créer par défaut (500)
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo).AnyTimes()
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), req.CustomerID, shopID).Return(nil, errors.New("not found"))
	scoreRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, score *entity.CreditScore) error {
			score.ID = uuid.New().String()
			return nil
		},
	)
	scoreRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	appRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, app *entity.CreditApplication) error {
			app.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.CreditApplicationPending, resp.Status)
	assert.Equal(t, 500, resp.CreditScoreAtApplication) // Score par défaut
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Success
// ============================================================

func TestApplyForCreditUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	// ✅ AJOUTER .AnyTimes() sur TOUS les WithTX
	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes() // ✅ AnyTimes
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	// Score existant et suffisant
	score := validCreditScore(req.CustomerID, shopID, 650)
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo).AnyTimes()
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), req.CustomerID, shopID).Return(score, nil)
	scoreRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	appRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, app *entity.CreditApplication) error {
			app.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.CreditApplicationPending, resp.Status)
	assert.Equal(t, 650, resp.CreditScoreAtApplication)
	assert.Equal(t, "good", resp.CreditScoreLevel)
	assert.NotEmpty(t, resp.ApplicationID)
	assert.Equal(t, req.CustomerID, resp.CustomerID)
	assert.Equal(t, req.ProductID, resp.ProductID)
	assert.Equal(t, shopID, resp.ShopID)
	assert.Equal(t, int64(100000), resp.ProductPriceCents)
	assert.Equal(t, int64(20000), resp.DownPaymentCents) // 20% de 100000
	assert.Equal(t, req.RequestedDurationMonths, resp.RequestedDurationMonths)
}

// ============================================================
// TESTS : ApplyForCreditUsecase - Helpers
// ============================================================

func TestApplyForCreditUsecase_SimulateCreditCalculation_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, productRepo, _, _ := newApplyForCreditUsecase(ctrl)
	ctx := context.Background()

	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(validProduct("product-1"), nil)

	plan := &entity.CreditPlan{
		ProductID:             "product-1",
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		InterestRateBps:       500,
	}
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	resp, err := uc.SimulateCreditCalculation(ctx, "product-1", 6)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "product-1", resp.ProductID)
	assert.Equal(t, int64(100000), resp.ProductPriceCents)
	assert.Equal(t, int64(20000), resp.DownPaymentCents)
	assert.Equal(t, int64(80000), resp.FinancedAmountCents)
	assert.Equal(t, int64(4000), resp.InterestAmountCents) // 5% de 80000
	assert.Equal(t, int64(84000), resp.TotalAmountCents)
	assert.Equal(t, int64(14000), resp.MonthlyPaymentCents) // 84000 / 6
}

func TestApplyForCreditUsecase_SimulateCreditCalculation_ProductNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, productRepo, _, _ := newApplyForCreditUsecase(ctrl)
	ctx := context.Background()

	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	resp, err := uc.SimulateCreditCalculation(ctx, "product-1", 6)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "product not found")
}

func TestApplyForCreditUsecase_GetCustomerCreditScore_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, scoreRepo, _ := newApplyForCreditUsecase(ctrl)
	ctx := context.Background()

	score := validCreditScore("customer-1", "shop-1", 650)
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", "shop-1").Return(score, nil)

	result, err := uc.GetCustomerCreditScore(ctx, "customer-1", "shop-1")
	assert.NoError(t, err)
	assert.Equal(t, 650, result.Score)
}

func TestApplyForCreditUsecase_CanApplyForCredit_Eligible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, _, scoreRepo, _ := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()

	plan := validCreditPlan("product-1", shopID)
	plan.MinCreditScore = 500
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	score := validCreditScore("customer-1", shopID, 650)
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(score, nil)

	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), "customer-1", "product-1").Return(nil, errors.New("not found"))

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.True(t, canApply)
	assert.Equal(t, "eligible", message)
}

func TestApplyForCreditUsecase_CanApplyForCredit_ScoreTooLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, _, scoreRepo, _ := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()

	plan := validCreditPlan("product-1", shopID)
	plan.MinCreditScore = 700
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	score := validCreditScore("customer-1", shopID, 500) // ❌ 500 < 700
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(score, nil)

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.False(t, canApply)
	assert.Contains(t, message, "credit score too low")
}

func TestApplyForCreditUsecase_CanApplyForCredit_CreditNotEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, _, _, _ := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()

	plan := validCreditPlan("product-1", shopID)
	plan.IsEnabled = false
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.False(t, canApply)
	assert.Contains(t, message, "credit is not enabled")
}

func TestApplyForCreditUsecase_CanApplyForCredit_NoCreditHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, _, scoreRepo, _ := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()

	plan := validCreditPlan("product-1", shopID)
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(nil, errors.New("not found"))

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.False(t, canApply)
	assert.Contains(t, message, "no credit history")
}
