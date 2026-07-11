package creditusecase_test

import (
	"context"
	"errors"
	"testing"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.17 : TESTS UNITAIRES - HELPERS DES USECASES
// ============================================================

// ============================================================
// TESTS : ApproveCreditUsecase - Helpers (query methods)
// ============================================================

func TestApproveCreditUsecase_GetContract_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contract := &entity.CreditContract{
		ID:         "contract-1",
		CustomerID: "customer-1",
		ProductID:  "product-1",
		ShopID:     "shop-1",
		Status:     entity.CreditContractActive,
	}

	contractRepo.EXPECT().FindByID(gomock.Any(), "contract-1").Return(contract, nil)

	result, err := uc.GetContract(context.Background(), "contract-1")

	assert.NoError(t, err)
	assert.Equal(t, "contract-1", result.ID)
	assert.Equal(t, entity.CreditContractActive, result.Status)
}

func TestApproveCreditUsecase_GetContract_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contractRepo.EXPECT().FindByID(gomock.Any(), "contract-1").Return(nil, errors.New("not found"))

	result, err := uc.GetContract(context.Background(), "contract-1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestApproveCreditUsecase_GetContractInstallments_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	installments := []*entity.CreditInstallment{
		{ID: "inst-1", ContractID: "contract-1", InstallmentNumber: 1, AmountCents: 14000},
		{ID: "inst-2", ContractID: "contract-1", InstallmentNumber: 2, AmountCents: 14000},
	}

	installmentRepo.EXPECT().FindByContractID(gomock.Any(), "contract-1").Return(installments, nil)

	result, err := uc.GetContractInstallments(context.Background(), "contract-1")

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(14000), result[0].AmountCents)
}

func TestApproveCreditUsecase_GetPendingApplications_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	applications := []*entity.CreditApplication{
		{ID: "app-1", CustomerID: "customer-1", Status: entity.CreditApplicationPending},
		{ID: "app-2", CustomerID: "customer-2", Status: entity.CreditApplicationPending},
	}

	appRepo.EXPECT().FindPendingByShopID(gomock.Any(), "shop-1").Return(applications, nil)

	result, err := uc.GetPendingApplications(context.Background(), "shop-1")

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, entity.CreditApplicationPending, result[0].Status)
}

func TestApproveCreditUsecase_GetActiveContractsByShop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contracts := []*entity.CreditContract{
		{ID: "contract-1", ShopID: "shop-1", Status: entity.CreditContractActive},
	}

	contractRepo.EXPECT().FindActiveByShopID(gomock.Any(), "shop-1").Return(contracts, nil)

	result, err := uc.GetActiveContractsByShop(context.Background(), "shop-1")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestApproveCreditUsecase_GetActiveContractsByCustomer_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contracts := []*entity.CreditContract{
		{ID: "contract-1", CustomerID: "customer-1", Status: entity.CreditContractActive},
	}

	contractRepo.EXPECT().FindActiveByCustomerID(gomock.Any(), "customer-1").Return(contracts, nil)

	result, err := uc.GetActiveContractsByCustomer(context.Background(), "customer-1")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestApproveCreditUsecase_GetOverdueInstallments_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	installments := []*entity.CreditInstallment{
		{ID: "inst-1", Status: entity.InstallmentLate},
	}

	installmentRepo.EXPECT().FindOverdue(gomock.Any()).Return(installments, nil)

	result, err := uc.GetOverdueInstallments(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, entity.InstallmentLate, result[0].Status)
}

func TestApproveCreditUsecase_GetOverdueInstallmentsByContract_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	installments := []*entity.CreditInstallment{
		{ID: "inst-1", ContractID: "contract-1", Status: entity.InstallmentLate},
	}

	installmentRepo.EXPECT().FindOverdueByContractID(gomock.Any(), "contract-1").Return(installments, nil)

	result, err := uc.GetOverdueInstallmentsByContract(context.Background(), "contract-1")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestApproveCreditUsecase_GetContractStats_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contract := &entity.CreditContract{
		ID:     "contract-1",
		Status: entity.CreditContractActive,
	}

	installments := []*entity.CreditInstallment{
		{ID: "inst-1", Status: entity.InstallmentPaid, AmountCents: 14000},
		{ID: "inst-2", Status: entity.InstallmentPending, AmountCents: 14000},
		{ID: "inst-3", Status: entity.InstallmentLate, AmountCents: 14000},
		{ID: "inst-4", Status: entity.InstallmentPaid, AmountCents: 14000},
	}

	contractRepo.EXPECT().FindByID(gomock.Any(), "contract-1").Return(contract, nil)
	installmentRepo.EXPECT().FindByContractID(gomock.Any(), "contract-1").Return(installments, nil)

	stats, err := uc.GetContractStats(context.Background(), "contract-1")

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "contract-1", stats["contract_id"])
	assert.Equal(t, entity.CreditContractActive, stats["status"])
	assert.Equal(t, 4, stats["total_installments"])
	assert.Equal(t, 2, stats["paid_installments"])
	assert.Equal(t, 1, stats["pending_installments"])
	assert.Equal(t, 1, stats["late_installments"])
	assert.Equal(t, int64(28000), stats["total_paid_cents"])
	assert.Equal(t, int64(28000), stats["total_pending_cents"])
	assert.Equal(t, 50.0, stats["completion_percent"]) // 2/4 = 50%
}

func TestApproveCreditUsecase_GetContractStats_ContractNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contractRepo.EXPECT().FindByID(gomock.Any(), "contract-1").Return(nil, errors.New("not found"))

	stats, err := uc.GetContractStats(context.Background(), "contract-1")

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "contract not found")
}

func TestApproveCreditUsecase_GetContractStats_InstallmentsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appRepo := mockrepo.NewMockCreditApplicationRepository(ctrl)
	contractRepo := mockrepo.NewMockCreditContractRepository(ctrl)
	installmentRepo := mockrepo.NewMockCreditInstallmentRepository(ctrl)
	scoreRepo := mockrepo.NewMockCreditScoreRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewApproveCreditUsecase(appRepo, contractRepo, installmentRepo, scoreRepo, txManager)

	contract := &entity.CreditContract{
		ID:     "contract-1",
		Status: entity.CreditContractActive,
	}

	contractRepo.EXPECT().FindByID(gomock.Any(), "contract-1").Return(contract, nil)
	installmentRepo.EXPECT().FindByContractID(gomock.Any(), "contract-1").Return(nil, errors.New("db error"))

	stats, err := uc.GetContractStats(context.Background(), "contract-1")

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to get installments")
}

// ============================================================
// TESTS : ConfigureCreditPlanUsecase - ListEnabledCreditPlansByShop
// ============================================================

func TestConfigureCreditPlanUsecase_ListEnabledCreditPlansByShop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	planRepo := mockrepo.NewMockCreditPlanRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewConfigureCreditPlanUsecase(planRepo, productRepo, walletRepo, txManager)

	plans := []*entity.CreditPlan{
		{ID: uuid.New().String(), ProductID: "product-1", IsEnabled: true},
		{ID: uuid.New().String(), ProductID: "product-2", IsEnabled: true},
	}

	planRepo.EXPECT().FindEnabledByShopID(gomock.Any(), "shop-1").Return(plans, nil)

	result, err := uc.ListEnabledCreditPlansByShop(context.Background(), "shop-1")

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.True(t, result[0].IsEnabled)
	assert.True(t, result[1].IsEnabled)
}

func TestConfigureCreditPlanUsecase_ListEnabledCreditPlansByShop_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	planRepo := mockrepo.NewMockCreditPlanRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := creditusecase.NewConfigureCreditPlanUsecase(planRepo, productRepo, walletRepo, txManager)

	planRepo.EXPECT().FindEnabledByShopID(gomock.Any(), "shop-1").Return([]*entity.CreditPlan{}, nil)

	result, err := uc.ListEnabledCreditPlansByShop(context.Background(), "shop-1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}
