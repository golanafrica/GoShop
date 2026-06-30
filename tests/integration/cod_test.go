package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	codusecase "Goshop/application/usecase/cod_usecase"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	codinfra "Goshop/infrastructure/postgres/cod"
	freezeinfra "Goshop/infrastructure/postgres/freeze"
	orderinfra "Goshop/infrastructure/postgres/order"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	walletinfra "Goshop/infrastructure/postgres/wallet"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// HELPER : Conversion string → *string
// ============================================================

func strPtr(s string) *string {
	return &s
}

// ============================================================
// SETUP COD
// ============================================================

func setupCODTest(t *testing.T) (
	*codusecase.SubmitClientProofUsecase,
	*codusecase.SubmitMerchantProofUsecase,
	*codusecase.CollectCommissionUsecase,
) {
	t.Helper()
	require.NotNil(t, sharedDB, "Shared DB not initialized")

	// Repositories COD
	codRepo := codinfra.NewCODProofRepositoryInfrastructure(sharedDB)
	orderRepo := orderinfra.NewOrderPostgresInfra(sharedDB)
	txManager := txmanager.NewTxManagerPostgresInfra(sharedDB)

	// Repositories Wallet
	walletRepo := walletinfra.NewMerchantWalletRepositoryInfrastructure(sharedDB)
	walletTxnRepo := walletinfra.NewWalletTransactionRepositoryInfrastructure(sharedDB)
	freezeRepo := freezeinfra.NewAccountFreezeRepositoryInfrastructure(sharedDB)

	// Usecases Wallet
	debitUC := walletusecase.NewDebitWalletUsecase(walletRepo, walletTxnRepo, txManager)
	freezeUC := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txManager)

	// Usecases COD
	submitClientUC := codusecase.NewSubmitClientProofUsecase(codRepo, orderRepo, txManager)
	submitMerchantUC := codusecase.NewSubmitMerchantProofUsecase(codRepo, orderRepo, txManager)
	collectCommissionUC := codusecase.NewCollectCommissionUsecase(
		codRepo, orderRepo, debitUC, freezeUC, txManager,
	)

	// Cleanup
	t.Cleanup(func() {
		tables := []string{
			"wallet_transactions",
			"account_freezes",
			"merchant_wallets",
			"cod_proofs",
			"order_items",
			"orders",
		}
		for _, table := range tables {
			sharedDB.Exec(fmt.Sprintf("DELETE FROM %s", table))
		}
	})

	return submitClientUC, submitMerchantUC, collectCommissionUC
}

// createCODOrder crée une commande COD de test et retourne son ID
func createCODOrder(t *testing.T, shopID, customerID, productID string) string {
	t.Helper()

	orderID := uuid.New().String()

	_, err := sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending_confirmation', 'cash_on_delivery', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, orderID, customerID, shopID, 5000000)
	require.NoError(t, err)

	_, err = sharedDB.Exec(`
		INSERT INTO order_items (id, order_id, product_id, quantity, price_cents, subtotal_cents)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`, uuid.New().String(), orderID, productID, 1, 5000000, 5000000)
	require.NoError(t, err)

	commissionCents := int64(5000000 * 250 / 10000)
	_, err = sharedDB.Exec(`
		INSERT INTO cod_proofs (id, order_id, shop_id, customer_id, commission_cents, commission_status, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', 'pending_proofs', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, uuid.New().String(), orderID, shopID, customerID, commissionCents)
	require.NoError(t, err)

	return orderID
}

// ✅ NOUVEAU : Helper pour créer le wallet d'un shop
func createTestWallet(t *testing.T, shopID string, balanceCents int64) {
	t.Helper()

	_, err := sharedDB.Exec(`
		INSERT INTO merchant_wallets (shop_id, balance_cents, is_frozen, max_negative_balance_cents, created_at, updated_at)
		VALUES ($1, $2, false, $3, NOW(), NOW())
		ON CONFLICT (shop_id) DO UPDATE SET balance_cents = $2
	`, shopID, balanceCents, -10000000)
	require.NoError(t, err, "Failed to create test wallet")
}

// ============================================================
// TESTS : SUBMIT CLIENT PROOF
// ============================================================

func TestCODSubmitClientProof_Success(t *testing.T) {
	submitClientUC, _, _ := setupCODTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	orderID := createCODOrder(t, shopID, customerID, productID)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	req := &codusecase.SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      "https://example.com/client-proof.jpg",
		AmountCents:   5000000,
		PaymentDate:   time.Now().UTC(),
		ReceiptNumber: strPtr("REC-001"),
		Notes:         strPtr("Paiement reçu en espèces"),
	}

	resp, err := submitClientUC.Execute(testCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.ProofID)
	assert.Equal(t, entity.CODProofClientProofSent, resp.Status)
}

func TestCODSubmitClientProof_InvalidOrder(t *testing.T) {
	submitClientUC, _, _ := setupCODTest(t)

	shopID := createTestShop(t)
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	req := &codusecase.SubmitClientProofRequest{
		OrderID:     uuid.New().String(),
		CustomerID:  uuid.New().String(),
		ProofURL:    "https://example.com/client-proof.jpg",
		AmountCents: 5000000,
		PaymentDate: time.Now().UTC(),
	}

	_, err := submitClientUC.Execute(testCtx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================
// TESTS : SUBMIT MERCHANT PROOF
// ============================================================

func TestCODSubmitMerchantProof_Success(t *testing.T) {
	submitClientUC, submitMerchantUC, _ := setupCODTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	orderID := createCODOrder(t, shopID, customerID, productID)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	clientReq := &codusecase.SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      "https://example.com/client-proof.jpg",
		AmountCents:   5000000,
		PaymentDate:   time.Now().UTC(),
		ReceiptNumber: strPtr("REC-001"),
	}
	_, err := submitClientUC.Execute(testCtx, clientReq)
	require.NoError(t, err)

	merchantReq := &codusecase.SubmitMerchantProofRequest{
		OrderID:     orderID,
		ProofURL:    "https://example.com/merchant-proof.jpg",
		AmountCents: 5000000,
		ReceiptDate: time.Now().UTC(),
		Notes:       strPtr("Reçu par le marchand"),
	}

	resp, err := submitMerchantUC.Execute(testCtx, merchantReq)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsCoherent)
	assert.Equal(t, entity.CODProofConfirmed, resp.Status)
}

func TestCODSubmitMerchantProof_IncoherentAmount(t *testing.T) {
	submitClientUC, submitMerchantUC, _ := setupCODTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	orderID := createCODOrder(t, shopID, customerID, productID)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	clientReq := &codusecase.SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      "https://example.com/client-proof.jpg",
		AmountCents:   5000000,
		PaymentDate:   time.Now().UTC(),
		ReceiptNumber: strPtr("REC-001"),
	}
	_, err := submitClientUC.Execute(testCtx, clientReq)
	require.NoError(t, err)

	merchantReq := &codusecase.SubmitMerchantProofRequest{
		OrderID:     orderID,
		ProofURL:    "https://example.com/merchant-proof.jpg",
		AmountCents: 4500000,
		ReceiptDate: time.Now().UTC(),
	}

	resp, err := submitMerchantUC.Execute(testCtx, merchantReq)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsCoherent)
	assert.Equal(t, entity.CODProofDisputed, resp.Status)
}

// ============================================================
// TESTS : COLLECT COMMISSION
// ============================================================

func TestCODCollectCommission_Success(t *testing.T) {
	submitClientUC, submitMerchantUC, collectUC := setupCODTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	orderID := createCODOrder(t, shopID, customerID, productID)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// ✅ CORRECTION : 2000 FCFA (assez pour 1250 FCFA de commission)
	createTestWallet(t, shopID, 200000)

	clientReq := &codusecase.SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      "https://example.com/client-proof.jpg",
		AmountCents:   5000000,
		PaymentDate:   time.Now().UTC(),
		ReceiptNumber: strPtr("REC-001"),
	}
	_, err := submitClientUC.Execute(testCtx, clientReq)
	require.NoError(t, err)

	merchantReq := &codusecase.SubmitMerchantProofRequest{
		OrderID:     orderID,
		ProofURL:    "https://example.com/merchant-proof.jpg",
		AmountCents: 5000000,
		ReceiptDate: time.Now().UTC(),
	}
	_, err = submitMerchantUC.Execute(testCtx, merchantReq)
	require.NoError(t, err)

	collectReq := &codusecase.CollectCommissionRequest{
		OrderID:      orderID,
		ForceCollect: false,
		CollectedBy:  uuid.New().String(),
	}

	resp, err := collectUC.Execute(testCtx, collectReq)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(125000), resp.CommissionCents)
	assert.Equal(t, entity.CODCommissionCollected, resp.CommissionStatus)
	assert.False(t, resp.AccountFrozen)
}

func TestCODCollectCommission_IncoherentProofs(t *testing.T) {
	submitClientUC, submitMerchantUC, collectUC := setupCODTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	orderID := createCODOrder(t, shopID, customerID, productID)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// ✅ CORRECTION : 2000 FCFA
	createTestWallet(t, shopID, 200000)

	clientReq := &codusecase.SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      "https://example.com/client-proof.jpg",
		AmountCents:   5000000,
		PaymentDate:   time.Now().UTC(),
		ReceiptNumber: strPtr("REC-001"),
	}
	_, err := submitClientUC.Execute(testCtx, clientReq)
	require.NoError(t, err)

	merchantReq := &codusecase.SubmitMerchantProofRequest{
		OrderID:     orderID,
		ProofURL:    "https://example.com/merchant-proof.jpg",
		AmountCents: 4500000,
		ReceiptDate: time.Now().UTC(),
	}
	_, err = submitMerchantUC.Execute(testCtx, merchantReq)
	require.NoError(t, err)

	collectReq := &codusecase.CollectCommissionRequest{
		OrderID:      orderID,
		ForceCollect: false,
		CollectedBy:  uuid.New().String(),
	}

	_, err = collectUC.Execute(testCtx, collectReq)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incoherent")
}

// ============================================================
// TESTS : WORKFLOW COMPLET
// ============================================================

func TestCODWorkflow_Complete(t *testing.T) {
	submitClientUC, submitMerchantUC, collectUC := setupCODTest(t)

	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)
	orderID := createCODOrder(t, shopID, customerID, productID)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	// ✅ CORRECTION : 2000 FCFA
	createTestWallet(t, shopID, 200000)

	clientReq := &codusecase.SubmitClientProofRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		ProofURL:      "https://example.com/client-proof.jpg",
		AmountCents:   5000000,
		PaymentDate:   time.Now().UTC(),
		ReceiptNumber: strPtr("REC-001"),
		Notes:         strPtr("Paiement reçu"),
	}
	clientResp, err := submitClientUC.Execute(testCtx, clientReq)
	require.NoError(t, err)
	assert.Equal(t, entity.CODProofClientProofSent, clientResp.Status)

	merchantReq := &codusecase.SubmitMerchantProofRequest{
		OrderID:     orderID,
		ProofURL:    "https://example.com/merchant-proof.jpg",
		AmountCents: 5000000,
		ReceiptDate: time.Now().UTC(),
		Notes:       strPtr("Reçu par le marchand"),
	}
	merchantResp, err := submitMerchantUC.Execute(testCtx, merchantReq)
	require.NoError(t, err)
	assert.Equal(t, entity.CODProofConfirmed, merchantResp.Status)
	assert.True(t, merchantResp.IsCoherent)

	collectReq := &codusecase.CollectCommissionRequest{
		OrderID:      orderID,
		ForceCollect: false,
		CollectedBy:  uuid.New().String(),
	}
	collectResp, err := collectUC.Execute(testCtx, collectReq)
	require.NoError(t, err)
	assert.Equal(t, entity.CODCommissionCollected, collectResp.CommissionStatus)
	assert.Equal(t, int64(125000), collectResp.CommissionCents)

	t.Logf("✅ Workflow COD complet validé : Order %s → Commission %d FCFA collectée",
		orderID, collectResp.CommissionCents/100)
}
