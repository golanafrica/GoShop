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
// 🆕 v4.4.17 : TESTS UNITAIRES - BRANCHES MANQUANTES
// ============================================================

// ============================================================
// TESTS : determineScoreLevel (40% → 100%)
// ============================================================

func TestDetermineScoreLevel_Excellent(t *testing.T) {
	// Score >= 700 = "excellent"
	// UpdateScore() = 500 + (OnTimePayments * 10) - (LatePayments * 20) - (Defaults * 100)
	// Pour 750 : 500 + (25 * 10) = 750
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	plan.MinCreditScore = 300
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	// Score avec 25 paiements à temps → UpdateScore() = 500 + (25*10) = 750
	score := &entity.CreditScore{
		ID:             uuid.New().String(),
		CustomerID:     req.CustomerID,
		ShopID:         shopID,
		Score:          500, // Initial (sera recalculé)
		OnTimePayments: 25,  // ✅ 25 * 10 = 250 → 500 + 250 = 750
		LatePayments:   0,
		Defaults:       0,
		TotalContracts: 0,
		LastUpdatedAt:  time.Now(),
	}
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
	assert.Equal(t, "excellent", resp.CreditScoreLevel)
}

func TestDetermineScoreLevel_Medium(t *testing.T) {
	// Score entre 300 et 500 = "medium"
	// Pour 400 : 500 - (5 * 20) = 400
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	plan.MinCreditScore = 300
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	// Score avec 5 paiements en retard → UpdateScore() = 500 - (5*20) = 400
	score := &entity.CreditScore{
		ID:             uuid.New().String(),
		CustomerID:     req.CustomerID,
		ShopID:         shopID,
		Score:          500, // Initial (sera recalculé)
		OnTimePayments: 0,
		LatePayments:   5, // ✅ 5 * 20 = 100 → 500 - 100 = 400
		Defaults:       0,
		TotalContracts: 0,
		LastUpdatedAt:  time.Now(),
	}
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
	assert.Equal(t, "medium", resp.CreditScoreLevel)
}

func TestDetermineScoreLevel_Bad(t *testing.T) {
	// Score < 300 = "bad"
	// Pour 200 : 500 - (3 * 100) = 200
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	plan := validCreditPlan(req.ProductID, shopID)
	plan.MinCreditScore = 200 // Min très bas pour accepter le score
	planRepo.EXPECT().WithTX(mockTx).Return(planRepo).AnyTimes()
	planRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(plan, nil)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), req.CustomerID, req.ProductID).Return(nil, errors.New("not found"))

	// Score avec 3 défauts → UpdateScore() = 500 - (3*100) = 200
	score := &entity.CreditScore{
		ID:             uuid.New().String(),
		CustomerID:     req.CustomerID,
		ShopID:         shopID,
		Score:          500, // Initial (sera recalculé)
		OnTimePayments: 0,
		LatePayments:   0,
		Defaults:       3, // ✅ 3 * 100 = 300 → 500 - 300 = 200
		TotalContracts: 0,
		LastUpdatedAt:  time.Now(),
	}
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
	assert.Equal(t, "bad", resp.CreditScoreLevel)
}

// ============================================================
// TESTS : RejectCreditUsecase.Execute() - Error paths (57.6% → ~85%)
// ============================================================

// ❌ SUPPRIMÉ : TestRejectCreditUsecase_ApplicationRejectError
// Ce test est inutile car le code source vérifie le statut avant d'appeler Reject()
// Le cas est déjà couvert par TestApproveCreditUsecase_ApplicationNotPending

func TestRejectCreditUsecase_UpdateApplicationError(t *testing.T) {
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
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to update application")
}

// ============================================================
// TESTS : PayDownPaymentUsecase.Execute() - Error paths (62.5% → ~85%)
// ============================================================

func TestPayDownPaymentUsecase_ScoreCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewPayDownPaymentUsecase(contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "contract-1",
		PaymentID:  "payment-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	contract := activeCreditContract("contract-1", "customer-1", "product-1", shopID)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().FindByID(gomock.Any(), req.ContractID).Return(contract, nil)

	// Score n'existe pas et création échoue
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo).AnyTimes()
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(nil, errors.New("not found"))
	scoreRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create credit score")
}

func TestPayDownPaymentUsecase_FindInstallmentsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewPayDownPaymentUsecase(contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "contract-1",
		PaymentID:  "payment-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	contract := activeCreditContract("contract-1", "customer-1", "product-1", shopID)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().FindByID(gomock.Any(), req.ContractID).Return(contract, nil)
	contractRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	score := validCreditScore("customer-1", shopID, 650)
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo).AnyTimes()
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(score, nil)

	// FindByContractID échoue
	installmentRepo.EXPECT().WithTX(mockTx).Return(installmentRepo).AnyTimes()
	installmentRepo.EXPECT().FindByContractID(gomock.Any(), "contract-1").Return(nil, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to find installments")
}

// ============================================================
// TESTS : CanApplyForCredit - Branches manquantes (71.4% → ~90%)
// ============================================================

func TestApplyForCreditUsecase_CanApplyForCredit_PlanNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, _, _, _ := newApplyForCreditUsecase(ctrl)
	ctx, _ := createCreditTestContext()

	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.False(t, canApply)
	assert.Contains(t, message, "credit not available")
}

func TestApplyForCreditUsecase_CanApplyForCredit_ExistingPendingApp(t *testing.T) {
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
		Status:     entity.CreditApplicationPending,
	}
	appRepo.EXPECT().FindByCustomerAndProduct(gomock.Any(), "customer-1", "product-1").Return(existingApp, nil)

	canApply, message, err := uc.CanApplyForCredit(ctx, "customer-1", "product-1")
	assert.NoError(t, err)
	assert.False(t, canApply)
	assert.Contains(t, message, "already have a pending application")
}

// ============================================================
// TESTS : SimulateCreditCalculation - Credit not enabled (83.3% → 100%)
// ============================================================

func TestApplyForCreditUsecase_SimulateCreditCalculation_CreditNotEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, planRepo, productRepo, _, _ := newApplyForCreditUsecase(ctrl)
	ctx := context.Background()

	productRepo.EXPECT().FindByID(gomock.Any(), "product-1").Return(validProduct("product-1"), nil)

	plan := &entity.CreditPlan{
		ProductID: "product-1",
		IsEnabled: false, // ❌ Désactivé
	}
	planRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(plan, nil)

	resp, err := uc.SimulateCreditCalculation(ctx, "product-1", 6)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "credit is not enabled")
}

// ============================================================
// TESTS : ApplyForCreditUsecase.Execute() - Score update error (85% → ~90%)
// ============================================================

func TestApplyForCreditUsecase_ScoreUpdateError_ContinuesAnyway(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, appRepo, planRepo, productRepo, scoreRepo, txManager := newApplyForCreditUsecase(ctrl)
	ctx, shopID := createCreditTestContext()
	req := validApplyRequest("customer-1", "product-1")
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

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

	// Score update échoue mais on continue
	scoreRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	appRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, app *entity.CreditApplication) error {
			app.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err) // ✅ Continue malgré l'erreur
	assert.NotNil(t, resp)
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase.Execute() - Plan validation error (84.6% → ~90%)
// ============================================================

func TestConfigureCreditPlanUsecase_PlanValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, planRepo, productRepo, _, txManager := newConfigureCreditPlanUsecase(ctrl)
	ctx, shopID := createCreditTestContext()

	// Requête avec des valeurs invalides pour le plan
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-1",
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       500,
		PenaltyRateBps:        500,
		MinCreditScore:        500,
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	productRepo.EXPECT().WithTX(mockTx).Return(productRepo).AnyTimes()
	productRepo.EXPECT().FindByID(gomock.Any(), req.ProductID).Return(validProduct(req.ProductID), nil)

	// Plan existant avec ShopID vide → validation échoue
	existingPlan := &entity.CreditPlan{
		ID:                    uuid.New().String(),
		ProductID:             req.ProductID,
		ShopID:                "", // ❌ Vide → validation échoue
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

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
	_ = shopID // Évite unused variable
}
