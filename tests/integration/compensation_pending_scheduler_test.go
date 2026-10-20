package integration

import (
	"testing"

	"Goshop/application/scheduler"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	"Goshop/infrastructure/postgres/wallet"
	withdrawalpostgres "Goshop/infrastructure/withdrawal"
	"Goshop/tests/testutils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompensationScheduler_IdempotenceOnCrash simule un crash après le crédit wallet
// mais avant la mise à jour du statut retrait, puis vérifie qu'un retry
// ne double-crédite pas (index uq_wallet_txn_ref_completed).
func TestCompensationScheduler_IdempotenceOnCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := testutils.NewTestServer(t)
	logger := zerolog.Nop()

	walletRepo := wallet.NewMerchantWalletRepositoryInfrastructure(server.DB)
	walletTxnRepo := wallet.NewWalletTransactionRepositoryInfrastructure(server.DB)
	withdrawalRepo := withdrawalpostgres.NewWithdrawalRepositoryPostgres(server.DB)
	// Obligatoire : CreditWalletUsecase.BeginTx → nil pointer si omis
	txManager := txmanager.NewTxManagerPostgresInfra(server.DB)

	creditWalletUC := walletusecase.NewCreditWalletUsecase(
		walletRepo,
		walletTxnRepo,
		nil, // escrowRepo optionnel pour withdrawal_reversal
		txManager,
	)

	compSched := scheduler.NewCompensationPendingScheduler(
		server.DB,
		withdrawalRepo,
		creditWalletUC,
		nil, // notifService optionnel
		logger,
	)

	// --- Seed : user → shop (owner_id NOT NULL) → wallet → withdrawal ---
	ownerID := uuid.New().String()
	ownerEmail := "comp-owner-" + uuid.New().String()[:8] + "@example.com"

	_, err := server.DB.Exec(`
		INSERT INTO users (id, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, ownerID, ownerEmail, "hashed-password-not-used")
	require.NoError(t, err, "seed user")

	shopID := uuid.New().String()
	shopName := "Test Shop Compensation"
	shopSlug := "test-shop-comp-" + uuid.New().String()[:8]

	_, err = server.DB.Exec(`
		INSERT INTO shops (
			id, name, slug, owner_id,
			plan, is_active, kyc_status,
			health_score, health_level,
			created_at, updated_at
		) VALUES (
			$1::uuid, $2, $3, $4,
			'free', true, 'verified',
			1000, 'excellent',
			NOW(), NOW()
		)
	`, shopID, shopName, shopSlug, ownerID)
	require.NoError(t, err, "seed shop (owner_id requis)")

	initialBalance := int64(10_000_000) // 100_000 FCFA
	_, err = server.DB.Exec(`
		INSERT INTO merchant_wallets (
			shop_id, balance_cents, held_cents, debt_cents,
			created_at, updated_at
		) VALUES ($1::uuid, $2, 0, 0, NOW(), NOW())
		ON CONFLICT (shop_id) DO UPDATE
		SET balance_cents = EXCLUDED.balance_cents,
		    held_cents = 0,
		    debt_cents = 0,
		    updated_at = NOW()
	`, shopID, initialBalance)
	require.NoError(t, err, "seed wallet")

	withdrawalAmount := int64(5_000_000) // 50_000 FCFA
	withdrawalID := uuid.New().String()

	// Colonnes DB : amount / fees / net_amount (pas amount_cents)
	// last_compensation_at ancien → évite tooRecent (minAgeSeconds=60)
	_, err = server.DB.Exec(`
		INSERT INTO withdrawals (
			id, shop_id, provider, amount, currency, fees, net_amount,
			payment_method, destination_number,
			status, failure_reason,
			compensation_attempts, last_compensation_at,
			created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, 'yenga_pay', $3, 'XOF', 0, $3,
			'ORANGE_MONEY', '70000000',
			'compensation_pending', 'provider_timeout',
			1, NOW() - INTERVAL '5 minutes',
			NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '5 minutes'
		)
	`, withdrawalID, shopID, withdrawalAmount)
	require.NoError(t, err, "seed withdrawal compensation_pending")

	// --- Run 1 : crédit reverse ---
	t.Log("🔄 Exécution 1 du scheduler (crédit réussi)")
	compSched.Run()

	var balanceAfterFirstRun int64
	err = server.DB.QueryRow(
		`SELECT balance_cents FROM merchant_wallets WHERE shop_id = $1::uuid`,
		shopID,
	).Scan(&balanceAfterFirstRun)
	require.NoError(t, err)
	assert.Equal(t, initialBalance+withdrawalAmount, balanceAfterFirstRun,
		"Le wallet doit être crédité après la première exécution")

	var txnCount int
	err = server.DB.QueryRow(`
		SELECT COUNT(*) FROM wallet_transactions
		WHERE reference_type = 'withdrawal_reversal'
		  AND reference_id = $1
		  AND status = 'completed'
	`, withdrawalID).Scan(&txnCount)
	require.NoError(t, err)
	assert.Equal(t, 1, txnCount, "Une seule transaction de compensation doit exister")

	var statusAfterFirst string
	err = server.DB.QueryRow(
		`SELECT status FROM withdrawals WHERE id = $1::uuid`,
		withdrawalID,
	).Scan(&statusAfterFirst)
	require.NoError(t, err)
	assert.Equal(t, "failed", statusAfterFirst, "Après reverse réussi, statut = failed")

	// --- Crash simulé : crédit déjà en base, statut pas encore failed ---
	t.Log("💥 Simulation de crash : reset statut → compensation_pending")
	_, err = server.DB.Exec(`
		UPDATE withdrawals
		SET status = 'compensation_pending',
		    last_compensation_at = NOW() - INTERVAL '5 minutes',
		    updated_at = NOW()
		WHERE id = $1::uuid
	`, withdrawalID)
	require.NoError(t, err)

	// --- Run 2 : retry idempotent ---
	t.Log("🔄 Exécution 2 du scheduler (retry après crash)")
	compSched.Run()

	var balanceAfterSecondRun int64
	err = server.DB.QueryRow(
		`SELECT balance_cents FROM merchant_wallets WHERE shop_id = $1::uuid`,
		shopID,
	).Scan(&balanceAfterSecondRun)
	require.NoError(t, err)
	assert.Equal(t, initialBalance+withdrawalAmount, balanceAfterSecondRun,
		"Le wallet ne doit PAS être crédité deux fois")

	err = server.DB.QueryRow(`
		SELECT COUNT(*) FROM wallet_transactions
		WHERE reference_type = 'withdrawal_reversal'
		  AND reference_id = $1
		  AND status = 'completed'
	`, withdrawalID).Scan(&txnCount)
	require.NoError(t, err)
	assert.Equal(t, 1, txnCount, "Contrainte UNIQUE : une seule ligne completed")

	var finalStatus string
	err = server.DB.QueryRow(
		`SELECT status FROM withdrawals WHERE id = $1::uuid`,
		withdrawalID,
	).Scan(&finalStatus)
	require.NoError(t, err)
	assert.Equal(t, "failed", finalStatus,
		"Après retry idempotent, retrait marqué failed")

	t.Log("✅ Idempotence compensation OK — pas de double crédit, état final cohérent")
}
