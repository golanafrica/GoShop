package integration

import (
	"context"
	"testing"

	"Goshop/application/scheduler"
	"Goshop/infrastructure/postgres/escrow"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/shop"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	"Goshop/infrastructure/postgres/wallet"
	"Goshop/tests/testutils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscrowAutoReleaseScheduler_AtomicCreditAndIdempotentRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := testutils.NewTestServer(t)
	ctx := context.Background()
	logger := zerolog.Nop()

	txmanagerRepo := txmanager.NewTxManagerPostgresInfra(server.DB)
	shopRepo := shop.NewShopRepositoryInfrastructure(server.DB)
	walletRepo := wallet.NewMerchantWalletRepositoryInfrastructure(server.DB)
	walletTxnRepo := wallet.NewWalletTransactionRepositoryInfrastructure(server.DB)
	escrowRepo := escrow.NewEscrowAccountRepositoryInfrastructure(server.DB)
	orderRepo := order.NewOrderPostgresInfra(server.DB)
	deliveryProofRepo := escrow.NewDeliveryProofRepositoryInfrastructure(server.DB)

	sched := scheduler.NewEscrowAutoReleaseScheduler(
		deliveryProofRepo,
		escrowRepo,
		orderRepo,
		nil, // tontineVoucherRepo
		nil, // tontineGroupRepo
		shopRepo,
		walletRepo,
		walletTxnRepo,
		nil, // platformRevenueRepo
		nil, // creditWalletUC
		txmanagerRepo,
		logger,
	)

	ownerID := uuid.New().String()
	shopID := uuid.New().String()
	customerID := uuid.New().String()
	orderID := uuid.New().String()
	escrowID := uuid.New().String()
	proofID := uuid.New().String()
	initialBalance := int64(10_000_000)
	releaseAmount := int64(5_000_000)

	_, err := server.DB.Exec(`
		INSERT INTO users (id, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, ownerID, "escrow-owner-"+ownerID[:8]+"@example.com", "hashed-password-not-used")
	require.NoError(t, err)

	_, err = server.DB.Exec(`
		INSERT INTO shops (id, name, slug, owner_id, is_active, created_at, updated_at)
		VALUES ($1, 'Escrow Test Shop', $2, $3, true, NOW(), NOW())
	`, shopID, "escrow-"+shopID[:8], ownerID)
	require.NoError(t, err)

	_, err = server.DB.Exec(`
		INSERT INTO merchant_wallets (shop_id, balance_cents, held_cents, debt_cents, created_at, updated_at)
		VALUES ($1, $2, 0, 0, NOW(), NOW())
	`, shopID, initialBalance)
	require.NoError(t, err)

	_, err = server.DB.Exec(`
		INSERT INTO customers (id, shop_id, first_name, last_name, phone, email, kyc_level, created_at, updated_at)
		VALUES ($1, $2, 'Test', 'Customer', '+22670000001', $3, 'verified', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, customerID, shopID, "cust-"+customerID[:8]+"@example.com")
	require.NoError(t, err)

	_, err = server.DB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending_confirmation', 'cash_on_delivery', NOW(), NOW())
	`, orderID, customerID, shopID, releaseAmount)
	require.NoError(t, err)

	// Pas de shop_id sur escrow_accounts — lien via order_id
	_, err = server.DB.Exec(`
		INSERT INTO escrow_accounts (
			id, order_id, source_type,
			total_amount_cents, released_amount_cents, commission_cents,
			status, funds_held_at, created_at, updated_at
		) VALUES (
			$1, $2, 'order',
			$3, 0, 0,
			'funds_held', NOW(), NOW(), NOW()
		)
	`, escrowID, orderID, releaseAmount)
	require.NoError(t, err)

	// FindAutoReleaseEligible : escrow_status='delivered' + delivery_date assez vieux
	_, err = server.DB.Exec(`
		INSERT INTO delivery_proofs (
			id, order_id, escrow_status, delivery_date, created_at, updated_at
		) VALUES (
			$1, $2, 'delivered', NOW() - INTERVAL '4 days', NOW(), NOW()
		)
	`, proofID, orderID)
	require.NoError(t, err)

	// Run 1
	err = sched.RunAutoRelease(ctx)
	require.NoError(t, err)

	var bal int64
	err = server.DB.QueryRow(`SELECT balance_cents FROM merchant_wallets WHERE shop_id = $1`, shopID).Scan(&bal)
	require.NoError(t, err)
	assert.Equal(t, initialBalance+releaseAmount, bal)

	var txnCount int
	err = server.DB.QueryRow(`
		SELECT COUNT(*) FROM wallet_transactions
		WHERE reference_type = 'escrow_auto_release' AND reference_id = $1
	`, proofID).Scan(&txnCount)
	require.NoError(t, err)
	assert.Equal(t, 1, txnCount)

	// Run 2 — idempotent
	err = sched.RunAutoRelease(ctx)
	require.NoError(t, err)

	err = server.DB.QueryRow(`SELECT balance_cents FROM merchant_wallets WHERE shop_id = $1`, shopID).Scan(&bal)
	require.NoError(t, err)
	assert.Equal(t, initialBalance+releaseAmount, bal)

	err = server.DB.QueryRow(`
		SELECT COUNT(*) FROM wallet_transactions
		WHERE reference_type = 'escrow_auto_release' AND reference_id = $1
	`, proofID).Scan(&txnCount)
	require.NoError(t, err)
	assert.Equal(t, 1, txnCount)

	t.Log("✅ Crédit atomique + retry idempotent (pas de double ledger/solde)")
}
