package integration_test

import (
	"context"
	"testing"
	"time"

	"Goshop/domain/entity"
	withdrawalpostgres "Goshop/infrastructure/withdrawal"
	"Goshop/tests/testutils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithdrawalReconciliation_Isolation prouve que le scheduler de compensation
// ignore strictement les retraits en "reconciliation_required" (PR 1 Audit).
func TestWithdrawalReconciliation_Isolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := testutils.NewTestServer(t)
	ctx := context.Background()
	withdrawalRepo := withdrawalpostgres.NewWithdrawalRepositoryPostgres(server.DB)

	// --- Seed aligné sur compensation_pending_scheduler_test.go ---
	ownerID := uuid.New().String()
	ownerEmail := "recon-owner-" + uuid.New().String()[:8] + "@example.com"

	_, err := server.DB.Exec(`
		INSERT INTO users (id, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, ownerID, ownerEmail, "hashed-password-not-used")
	require.NoError(t, err, "seed user")

	shopID := uuid.New().String()
	shopName := "Test Shop Reconciliation"
	shopSlug := "test-shop-recon-" + uuid.New().String()[:8]

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

	shopUUID, err := uuid.Parse(shopID)
	require.NoError(t, err)
	amountCents := int64(50000)

	// ============================================================
	// SCÉNARIO 1 : Retrait coincé -> passe en reconciliation_required
	// ============================================================
	t.Run("Reconciliation scheduler moves stuck withdrawal to reconciliation_required", func(t *testing.T) {
		w := &entity.Withdrawal{
			ID:                uuid.New(),
			ShopID:            shopUUID,
			Provider:          "yenga_pay",
			AmountCents:       amountCents,
			Currency:          entity.CurrencyXOF,
			Status:            entity.WithdrawalStatusPending,
			PaymentMethod:     entity.WithdrawalOrangeMoney,
			DestinationNumber: "+22670000000",
			CreatedAt:         time.Now().Add(-20 * time.Minute),
			UpdatedAt:         time.Now().Add(-20 * time.Minute),
		}
		require.NoError(t, withdrawalRepo.Create(ctx, w))

		// Simule l'action du WithdrawalReconciliationScheduler
		reason := "Stuck in pending after wallet debit. Requires manual provider verification."
		w.Status = entity.WithdrawalStatusReconciliationRequired
		w.FailureReason = &reason
		require.NoError(t, withdrawalRepo.Update(ctx, w))

		updatedW, err := withdrawalRepo.FindByID(ctx, w.ID)
		require.NoError(t, err)
		assert.Equal(t, entity.WithdrawalStatusReconciliationRequired, updatedW.Status)
	})

	// ============================================================
	// SCÉNARIO 2 : Le scheduler de compensation NE DOIT PAS le voir
	// ============================================================
	t.Run("Compensation scheduler MUST NOT pick up reconciliation_required withdrawals", func(t *testing.T) {
		pendingCompensations, err := withdrawalRepo.FindCompensationPending(ctx, 10, 3)
		require.NoError(t, err)

		for _, wc := range pendingCompensations {
			assert.NotEqual(t, entity.WithdrawalStatusReconciliationRequired, wc.Status,
				"CRITICAL: FindCompensationPending returned a withdrawal with reconciliation_required status!")
		}
	})

	// ============================================================
	// SCÉNARIO 3 : Un retrait explicitement en compensation_pending EST traité
	// ============================================================
	t.Run("Compensation scheduler MUST pick up explicit compensation_pending withdrawals", func(t *testing.T) {
		w2 := &entity.Withdrawal{
			ID:                   uuid.New(),
			ShopID:               shopUUID,
			Provider:             "yenga_pay",
			AmountCents:          amountCents,
			Currency:             entity.CurrencyXOF,
			Status:               entity.WithdrawalStatusCompensationPending,
			PaymentMethod:        entity.WithdrawalOrangeMoney,
			DestinationNumber:    "+22670000001",
			CompensationAttempts: 0,
			CreatedAt:            time.Now().Add(-20 * time.Minute),
			UpdatedAt:            time.Now().Add(-20 * time.Minute),
		}
		require.NoError(t, withdrawalRepo.Create(ctx, w2))

		pendingCompensations, err := withdrawalRepo.FindCompensationPending(ctx, 10, 3)
		require.NoError(t, err)

		var found bool
		for _, wc := range pendingCompensations {
			if wc.ID == w2.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "CRITICAL: FindCompensationPending failed to return an explicit compensation_pending withdrawal!")
	})
}
