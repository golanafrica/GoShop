package integration

import (
	"context"
	"fmt"
	"testing"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	creditinfra "Goshop/infrastructure/postgres/credit"
	productinfra "Goshop/infrastructure/postgres/product"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	walletinfra "Goshop/infrastructure/postgres/wallet"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// SETUP CREDIT
// ============================================================

func setupCreditTest(t *testing.T) (
	*creditusecase.ConfigureCreditPlanUsecase,
	*creditusecase.ApplyForCreditUsecase,
	*creditusecase.ApproveCreditUsecase,
	*creditusecase.RejectCreditUsecase,
) {
	t.Helper()
	require.NotNil(t, sharedDB, "Shared DB not initialized")

	// ✅ Repositories Credit
	planRepo := creditinfra.NewCreditPlanRepositoryInfrastructure(sharedDB)
	appRepo := creditinfra.NewCreditApplicationRepositoryInfrastructure(sharedDB)
	contractRepo := creditinfra.NewCreditContractRepositoryInfrastructure(sharedDB)
	installmentRepo := creditinfra.NewCreditInstallmentRepositoryInfrastructure(sharedDB)
	scoreRepo := creditinfra.NewCreditScoreRepositoryInfrastructure(sharedDB)
	txManager := txmanager.NewTxManagerPostgresInfra(sharedDB)

	// ✅ NOUVEAU : Repositories Product + Wallet (au lieu de nil)
	productRepo := productinfra.NewProductRepositoryInfrastructure(sharedDB)
	walletRepo := walletinfra.NewMerchantWalletRepositoryInfrastructure(sharedDB)

	// Usecases
	configureUC := creditusecase.NewConfigureCreditPlanUsecase(
		planRepo, productRepo, walletRepo, txManager, // ✅ Vrais repos
	)
	applyUC := creditusecase.NewApplyForCreditUsecase(
		appRepo, planRepo, productRepo, scoreRepo, txManager,
	)
	approveUC := creditusecase.NewApproveCreditUsecase(
		appRepo, contractRepo, installmentRepo, scoreRepo, txManager,
	)
	rejectUC := creditusecase.NewRejectCreditUsecase(
		appRepo, scoreRepo, txManager,
	)

	// Cleanup
	t.Cleanup(func() {
		tables := []string{
			"credit_installments",
			"credit_contracts",
			"credit_applications",
			"credit_scores",
			"credit_plans",
		}
		for _, table := range tables {
			sharedDB.Exec(fmt.Sprintf("DELETE FROM %s", table))
		}
	})

	return configureUC, applyUC, approveUC, rejectUC
}

// createTestProduct crée un produit de test
func createTestProduct(t *testing.T, shopID string) string {
	t.Helper()

	productID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO products (id, name, description, price_cents, stock, shop_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, productID, "Moto Test Credit", "Moto pour test crédit", 5000000, 10, shopID)
	require.NoError(t, err)

	return productID
}

// createTestCustomer crée un customer de test
func createTestCustomer(t *testing.T, shopID string) string {
	t.Helper()

	customerID := uuid.New().String()
	email := fmt.Sprintf("customer-%s@example.com", customerID[:8])

	_, err := sharedDB.Exec(`
		INSERT INTO customers (id, first_name, last_name, email, shop_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, customerID, "Jean", "Dupont", email, shopID)
	require.NoError(t, err)

	return customerID
}

// ============================================================
// TESTS : CREDIT PLANS
// ============================================================

func TestCreditPlan_Configure(t *testing.T) {
	configureUC, _, _, _ := setupCreditTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       1000, // 10%
		PenaltyRateBps:        500,  // 5%
		MinCreditScore:        300,
	}

	resp, err := configureUC.Execute(testCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "created", resp.Action)
	assert.Equal(t, productID, resp.ProductID)
	assert.True(t, resp.IsEnabled)
	assert.Equal(t, 1000, resp.InterestRateBps)
}

func TestCreditPlan_Configure_InvalidRate(t *testing.T) {
	configureUC, _, _, _ := setupCreditTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// Taux > 15% (limite BCEAO)
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       2000, // ❌ 20% > 15% BCEAO
		PenaltyRateBps:        500,
		MinCreditScore:        300,
	}

	_, err := configureUC.Execute(testCtx, req)

	assert.Error(t, err)
	// ✅ CORRECTION : Le message d'erreur contient "interest_rate_bps"
	assert.Contains(t, err.Error(), "interest_rate_bps")
}

// ============================================================
// TESTS : CREDIT APPLICATION
// ============================================================

func TestCreditApplication_Apply(t *testing.T) {
	configureUC, applyUC, _, _ := setupCreditTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// Configurer le plan d'abord
	planReq := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       1000,
		PenaltyRateBps:        500,
		MinCreditScore:        300,
	}
	_, err := configureUC.Execute(testCtx, planReq)
	require.NoError(t, err)

	// Demander un crédit
	applyReq := &creditusecase.ApplyForCreditRequest{
		CustomerID:              customerID,
		ProductID:               productID,
		RequestedDurationMonths: 12,
	}

	resp, err := applyUC.Execute(testCtx, applyReq)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.ApplicationID)
	assert.Equal(t, customerID, resp.CustomerID)
	assert.Equal(t, int64(5000000), resp.ProductPriceCents)
	assert.Equal(t, int64(1000000), resp.DownPaymentCents)   // 20%
	assert.Equal(t, int64(400000), resp.InterestAmountCents) // 10% de 4M
	assert.Equal(t, int64(366666), resp.MonthlyPaymentCents)
}

// ============================================================
// TESTS : CREDIT APPROVAL
// ============================================================

func TestCreditApproval_Approve(t *testing.T) {
	configureUC, applyUC, approveUC, _ := setupCreditTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// Configurer le plan
	planReq := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       1000,
		PenaltyRateBps:        500,
		MinCreditScore:        300,
	}
	_, err := configureUC.Execute(testCtx, planReq)
	require.NoError(t, err)

	// Demander un crédit
	applyReq := &creditusecase.ApplyForCreditRequest{
		CustomerID:              customerID,
		ProductID:               productID,
		RequestedDurationMonths: 12,
	}
	applyResp, err := applyUC.Execute(testCtx, applyReq)
	require.NoError(t, err)

	// ✅ CORRECTION : Utiliser un UUID valide pour ReviewedBy
	approveReq := &creditusecase.ApproveCreditRequest{
		ApplicationID: applyResp.ApplicationID,
		ReviewedBy:    uuid.New().String(), // ✅ UUID au lieu de "merchant-xxx"
	}

	approveResp, err := approveUC.Execute(testCtx, approveReq)

	require.NoError(t, err)
	assert.NotNil(t, approveResp)
	assert.NotEmpty(t, approveResp.ContractID)
	assert.Equal(t, 12, approveResp.InstallmentsCount)
	assert.Equal(t, entity.CreditApplicationApproved, approveResp.Status)
}

func TestCreditApproval_Reject(t *testing.T) {
	configureUC, applyUC, _, rejectUC := setupCreditTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// Configurer + demander
	planReq := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       1000,
		PenaltyRateBps:        500,
		MinCreditScore:        300,
	}
	_, err := configureUC.Execute(testCtx, planReq)
	require.NoError(t, err)

	applyReq := &creditusecase.ApplyForCreditRequest{
		CustomerID:              customerID,
		ProductID:               productID,
		RequestedDurationMonths: 12,
	}
	applyResp, err := applyUC.Execute(testCtx, applyReq)
	require.NoError(t, err)

	// ✅ CORRECTION : Utiliser un UUID valide pour ReviewedBy
	rejectReq := &creditusecase.RejectCreditRequest{
		ApplicationID:   applyResp.ApplicationID,
		ReviewedBy:      uuid.New().String(), // ✅ UUID au lieu de "merchant-xxx"
		RejectionReason: "Credit score too low",
	}

	rejectResp, err := rejectUC.Execute(testCtx, rejectReq)

	require.NoError(t, err)
	assert.NotNil(t, rejectResp)
	assert.Equal(t, entity.CreditApplicationRejected, rejectResp.Status)
	assert.Equal(t, "Credit score too low", rejectResp.RejectionReason)
}
