package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	walletinfra "Goshop/infrastructure/postgres/wallet"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// SETUP INSTALLMENT E2E
// ============================================================

func setupInstallmentE2E(t *testing.T) {
	t.Helper()
	require.NotNil(t, sharedDB, "Shared DB not initialized")

	t.Cleanup(func() {
		tables := []string{
			"wallet_transactions",
			"order_installments",
			"installment_plans",
			"orders",
			"order_items",
			"merchant_wallets",
			"customers",
			"shops",
			"users",
		}
		for _, table := range tables {
			sharedDB.Exec(fmt.Sprintf("DELETE FROM %s", table))
		}
	})
}

// ============================================================
// HELPERS DE DONNÉES
// ============================================================

func createE2EShop(t *testing.T) string {
	t.Helper()
	userID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO users (id, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, userID, fmt.Sprintf("test-e2e-%s@example.com", userID[:8]), "$2a$04$dummyhashfortesting000000000000000000000000")
	require.NoError(t, err)

	shopID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO shops (id, name, slug, owner_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, shopID, "E2E Test Shop", "e2e-shop-"+shopID[:8], userID)
	require.NoError(t, err)
	return shopID
}

func createE2EProduct(t *testing.T, shopID string) string {
	t.Helper()
	productID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO products (id, shop_id, name, price_cents, stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`, productID, shopID, "E2E Test Product", 3000000, 10)
	require.NoError(t, err)
	return productID
}

func createE2ECustomer(t *testing.T, shopID string) string {
	t.Helper()
	customerID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO customers (id, shop_id, first_name, last_name, email, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, customerID, shopID, "Test", "Customer", fmt.Sprintf("test-cust-%s@example.com", customerID[:8]), "+22670000000")
	require.NoError(t, err)
	return customerID
}

func createE2EWallet(t *testing.T, shopID string) {
	t.Helper()
	_, err := sharedDB.Exec(`
		INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, is_frozen, max_negative_balance_cents, created_at, updated_at)
		VALUES ($1, $2, $3, false, $4, NOW(), NOW())
	`, shopID, 0, 0, -10000000)
	require.NoError(t, err)
}

// ============================================================
// TEST E2E : WORKFLOW COMPLET PAIEMENT EN TRANCHES
// ============================================================

func TestInstallmentFlow_CompleteEscrowWorkflow(t *testing.T) {
	var dbName, dbUser, dbHost string
	err := sharedDB.QueryRow("SELECT current_database(), current_user, inet_server_addr()").Scan(&dbName, &dbUser, &dbHost)
	if err != nil {
		t.Logf("⚠️ Impossible de récupérer les infos DB: %v", err)
	} else {
		t.Logf("✅ Connecté à la DB: '%s' (User: %s, Host/IP: %s)", dbName, dbUser, dbHost)
	}

	setupInstallmentE2E(t)

	// 1. Préparation des entités
	shopID := createE2EShop(t)
	productID := createE2EProduct(t, shopID)
	customerID := createE2ECustomer(t, shopID)
	createE2EWallet(t, shopID)

	totalOrderCents := int64(3000000) // 30 000 FCFA
	commissionRateBps := int64(500)   // 5% de commission GoShop
	expectedCommissionCents := (totalOrderCents * commissionRateBps) / 10000
	expectedNetMerchantCents := totalOrderCents - expectedCommissionCents

	// 2. Plan de paiement en 3 tranches
	planID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO installment_plans (id, product_id, shop_id, nb_tranches, delai_jours, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())
	`, planID, productID, shopID, 3, 15)
	require.NoError(t, err)

	// 3. Commande du client
	orderID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending', 'mobile_money', NOW(), NOW())
	`, orderID, customerID, shopID, totalOrderCents)
	require.NoError(t, err)

	// Génération des 3 tranches
	trancheAmount := totalOrderCents / 3
	for i := 1; i <= 3; i++ {
		_, err = sharedDB.Exec(`
			INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'pending', NOW())
		`, uuid.New().String(), orderID, i, trancheAmount, time.Now().UTC().AddDate(0, 0, i*15))
		require.NoError(t, err)
	}

	// 4. Simulation des 3 paiements du client
	walletRepo := walletinfra.NewMerchantWalletRepositoryInfrastructure(sharedDB)
	txnRepo := walletinfra.NewWalletTransactionRepositoryInfrastructure(sharedDB)
	txManager := txmanager.NewTxManagerPostgresInfra(sharedDB)
	creditUC := walletusecase.NewCreditWalletUsecase(walletRepo, txnRepo, nil, txManager)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{
		ID: uuid.MustParse(shopID),
	})

	for i := 1; i <= 3; i++ {
		// a. Créditer le wallet (l'argent arrive)
		_, err = creditUC.Execute(testCtx, &walletusecase.CreditWalletRequest{
			ShopID:          shopID,
			AmountCents:     trancheAmount,
			TransactionType: entity.WalletTxSaleCredit,
		})
		require.NoError(t, err)

		// b. Geler les fonds en escrow
		wallet, err := walletRepo.FindByShopIDForUpdate(testCtx, shopID)
		require.NoError(t, err)
		require.NoError(t, wallet.Hold(trancheAmount))
		require.NoError(t, walletRepo.Update(testCtx, wallet))

		// c. Marquer la tranche payée
		_, err = sharedDB.Exec(`
			UPDATE order_installments SET status = 'paid', paid_at = NOW() 
			WHERE order_id = $1 AND tranche_number = $2
		`, orderID, i)
		require.NoError(t, err)
	}

	// 5. Vérification intermédiaire : tout est gelé, rien de disponible
	walletAfterPayments, err := walletRepo.FindByShopID(testCtx, shopID)
	require.NoError(t, err)
	assert.Equal(t, totalOrderCents, walletAfterPayments.HeldCents, "Tout l'argent doit être gelé en escrow")
	assert.Equal(t, int64(0), walletAfterPayments.AvailableCents(), "Le solde disponible doit être à 0 avant la libération")

	// 6. Libération du séquestre (après confirmation de réception)
	walletForRelease, err := walletRepo.FindByShopIDForUpdate(testCtx, shopID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, walletForRelease.HeldCents, totalOrderCents, "Fonds insuffisants en escrow")

	// Dégeler les fonds
	require.NoError(t, walletForRelease.ReleaseHeld(totalOrderCents))

	// CORRIGÉ : Soustraire la commission du solde total (pas ajouter le net)
	walletForRelease.BalanceCents -= expectedCommissionCents
	walletForRelease.TotalCommissionsCents += expectedCommissionCents
	walletForRelease.UpdatedAt = time.Now().UTC()
	require.NoError(t, walletRepo.Update(testCtx, walletForRelease))

	// Marquer la commande comme livrée
	_, err = sharedDB.Exec(`UPDATE orders SET status = 'delivered' WHERE id = $1`, orderID)
	require.NoError(t, err)

	// 7. Assertions Finales
	finalWallet, err := walletRepo.FindByShopID(testCtx, shopID)
	require.NoError(t, err)

	t.Logf("💰 Résultat Final : Total=%d, Commission GoShop=%d, Net Marchand=%d",
		totalOrderCents, expectedCommissionCents, expectedNetMerchantCents)
	t.Logf("📊 Wallet Final : Balance=%d, Held=%d, Available=%d, TotalCommissions=%d",
		finalWallet.BalanceCents, finalWallet.HeldCents, finalWallet.AvailableCents(), finalWallet.TotalCommissionsCents)

	assert.Equal(t, int64(0), finalWallet.HeldCents, "Le séquestre doit être entièrement vidé")
	assert.Equal(t, expectedNetMerchantCents, finalWallet.AvailableCents(), "Le marchand doit recevoir le montant net (Total - Commission)")
	assert.Equal(t, expectedCommissionCents, finalWallet.TotalCommissionsCents, "GoShop doit avoir enregistré sa commission")

	t.Log("✅ Workflow E2E de paiement en tranches avec séquestre validé avec succès !")
}
