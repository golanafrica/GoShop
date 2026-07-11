package creditusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.17 : TESTS UNITAIRES - APPROVE/REJECT/PAY DOWN PAYMENT
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func pendingCreditApplication(appID, customerID, productID, shopID string) *entity.CreditApplication {
	return &entity.CreditApplication{
		ID:                       appID,
		CustomerID:               customerID,
		ProductID:                productID,
		ShopID:                   shopID,
		RequestedDurationMonths:  6,
		CreditScoreAtApplication: 650,
		ProductPriceCents:        100000,
		DownPaymentCents:         20000,
		FinancedAmountCents:      80000,
		InterestAmountCents:      4000,
		TotalAmountCents:         84000,
		MonthlyPaymentCents:      14000,
		Status:                   entity.CreditApplicationPending,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
}

func activeCreditContract(contractID, customerID, productID, shopID string) *entity.CreditContract {
	return &entity.CreditContract{
		ID:                  contractID,
		ApplicationID:       "app-1",
		CustomerID:          customerID,
		ProductID:           productID,
		ShopID:              shopID,
		ProductPriceCents:   100000,
		DownPaymentCents:    20000,
		FinancedAmountCents: 80000,
		InterestAmountCents: 4000,
		TotalAmountCents:    84000,
		MonthlyPaymentCents: 14000,
		DurationMonths:      6,
		StartDate:           time.Now(),
		EndDate:             time.Now().AddDate(0, 6, 0),
		Status:              entity.CreditContractActive,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// ============================================================
// TESTS : ApproveCreditUsecase.Execute() - Validation & Multi-tenant
// ============================================================

func TestApproveCreditUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "", // ❌ Vide
		ReviewedBy:    "merchant-1",
	}

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestApproveCreditUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx := context.Background() // ❌ Sans tenant
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestApproveCreditUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

// ============================================================
// TESTS : ApproveCreditUsecase.Execute() - Application validation
// ============================================================

func TestApproveCreditUsecase_ApplicationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
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

func TestApproveCreditUsecase_ShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
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

func TestApproveCreditUsecase_ApplicationNotPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Application déjà approuvée
	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)
	app.Status = entity.CreditApplicationApproved

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not in pending status")
}

// ============================================================
// TESTS : ApproveCreditUsecase.Execute() - Repository errors
// ============================================================

func TestApproveCreditUsecase_UpdateApplicationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
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

func TestApproveCreditUsecase_SaveContractError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to save credit contract")
}

func TestApproveCreditUsecase_SaveInstallmentsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	installmentRepo.EXPECT().WithTX(mockTx).Return(installmentRepo).AnyTimes()
	installmentRepo.EXPECT().CreateBatch(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to save installments")
}

func TestApproveCreditUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	installmentRepo.EXPECT().WithTX(mockTx).Return(installmentRepo).AnyTimes()
	installmentRepo.EXPECT().CreateBatch(gomock.Any(), gomock.Any()).Return(nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

// ============================================================
// TESTS : ApproveCreditUsecase.Execute() - Success
// ============================================================

func TestApproveCreditUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, shopID := createCreditTestContext()
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-1",
		ReviewedBy:    "merchant-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	installmentRepo.EXPECT().WithTX(mockTx).Return(installmentRepo).AnyTimes()
	installmentRepo.EXPECT().CreateBatch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, installments []*entity.CreditInstallment) error {
			assert.Equal(t, 6, len(installments)) // 6 mois
			return nil
		},
	)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.CreditApplicationApproved, resp.Status)
	assert.NotEmpty(t, resp.ContractID)
	assert.Equal(t, 6, resp.InstallmentsCount)
	assert.Equal(t, int64(20000), resp.DownPaymentCents)
	assert.Equal(t, int64(80000), resp.FinancedAmountCents)
	assert.Equal(t, int64(4000), resp.InterestAmountCents)
	assert.Equal(t, int64(84000), resp.TotalAmountCents)
	assert.Equal(t, int64(14000), resp.MonthlyPaymentCents)
	assert.Contains(t, resp.Message, "approved")
}

// ============================================================
// TESTS : RejectCreditUsecase.Execute()
// ============================================================

func TestRejectCreditUsecase_ValidationError(t *testing.T) {
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
		RejectionReason: "", // ❌ Vide
	}

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestRejectCreditUsecase_Success(t *testing.T) {
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
		RejectionReason: "Credit score too low",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	app := pendingCreditApplication("app-1", "customer-1", "product-1", shopID)

	appRepo.EXPECT().WithTX(mockTx).Return(appRepo).AnyTimes()
	appRepo.EXPECT().FindByID(gomock.Any(), req.ApplicationID).Return(app, nil)
	appRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.CreditApplicationRejected, resp.Status)
	assert.Equal(t, "Credit score too low", resp.RejectionReason)
	assert.NotNil(t, resp.ReviewedAt)
}

// ============================================================
// TESTS : PayDownPaymentUsecase.Execute()
// ============================================================

func TestPayDownPaymentUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewPayDownPaymentUsecase(contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "", // ❌ Vide
		PaymentID:  "payment-1",
	}

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestPayDownPaymentUsecase_ContractNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewPayDownPaymentUsecase(contractRepo, installmentRepo, scoreRepo, txManager)

	ctx, _ := createCreditTestContext()
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "contract-1",
		PaymentID:  "payment-1",
	}
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().FindByID(gomock.Any(), req.ContractID).Return(nil, errors.New("not found"))

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "contract not found")
}

func TestPayDownPaymentUsecase_ContractNotActive(t *testing.T) {
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

	// Contrat complété (pas actif)
	contract := activeCreditContract("contract-1", "customer-1", "product-1", shopID)
	contract.Status = entity.CreditContractCompleted

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().FindByID(gomock.Any(), req.ContractID).Return(contract, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not in active status")
}

func TestPayDownPaymentUsecase_DownPaymentAlreadyPaid(t *testing.T) {
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

	// Apport déjà payé
	contract := activeCreditContract("contract-1", "customer-1", "product-1", shopID)
	now := time.Now()
	contract.DownPaymentPaidAt = &now

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().FindByID(gomock.Any(), req.ContractID).Return(contract, nil)

	resp, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "down payment already paid")
}

func TestPayDownPaymentUsecase_Success(t *testing.T) {
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
	mockTx.EXPECT().Commit().Return(nil)

	contract := activeCreditContract("contract-1", "customer-1", "product-1", shopID)

	contractRepo.EXPECT().WithTX(mockTx).Return(contractRepo).AnyTimes()
	contractRepo.EXPECT().FindByID(gomock.Any(), req.ContractID).Return(contract, nil)
	contractRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	score := validCreditScore("customer-1", shopID, 650)
	scoreRepo.EXPECT().WithTX(mockTx).Return(scoreRepo).AnyTimes()
	scoreRepo.EXPECT().FindByCustomerAndShop(gomock.Any(), "customer-1", shopID).Return(score, nil)

	installmentRepo.EXPECT().WithTX(mockTx).Return(installmentRepo).AnyTimes()
	installmentRepo.EXPECT().FindByContractID(gomock.Any(), "contract-1").Return([]*entity.CreditInstallment{}, nil)

	resp, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.DownPaymentPaidAt)
	assert.Equal(t, 650, resp.CreditScoreBefore)
	assert.Contains(t, resp.Message, "Down payment received")
}
