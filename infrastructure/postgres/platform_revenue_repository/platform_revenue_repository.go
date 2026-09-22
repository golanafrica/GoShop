package platformrevenuerepository

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/repository"

	"github.com/google/uuid"
)

type PlatformRevenueRepositoryPostgres struct {
	db *sql.DB
}

func NewPlatformRevenueRepositoryPostgres(db *sql.DB) repository.PlatformRevenueRepository {
	return &PlatformRevenueRepositoryPostgres{db: db}
}

// CreditRevenue crédite le solde plateforme + ligne d'audit, en une seule TX.
// Schéma réel:
//
//	platform_revenue_accounts(id, balance_cents, total_collected_cents, updated_at)
//	platform_revenue_transactions(id, transaction_type, amount_cents, reference_type, reference_id, description, created_at)
//
// referenceType : type métier (ex: "escrow_auto_release")
// referenceID   : UUID (ex: proof_id)
func (r *PlatformRevenueRepositoryPostgres) CreditRevenue(
	ctx context.Context,
	amountCents int64,
	referenceType string,
	referenceID string,
) error {
	if amountCents <= 0 {
		return nil
	}
	if referenceType == "" {
		referenceType = "escrow_auto_release"
	}
	if referenceID == "" {
		return fmt.Errorf("referenceID is required for platform revenue credit")
	}
	// reference_id est uuid en base
	if _, err := uuid.Parse(referenceID); err != nil {
		return fmt.Errorf("referenceID must be a valid UUID: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Compte global (pas de created_at sur cette table)
	var accountID string
	err = tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM platform_revenue_accounts
		ORDER BY updated_at ASC
		LIMIT 1
		FOR UPDATE
	`).Scan(&accountID)

	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO platform_revenue_accounts (balance_cents, total_collected_cents, updated_at)
			VALUES (0, 0, NOW())
			RETURNING id::text
		`).Scan(&accountID)
		if err != nil {
			return fmt.Errorf("seed platform_revenue_accounts: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("lock platform_revenue_accounts: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform_revenue_transactions (
			transaction_type, amount_cents, reference_type, reference_id, description, created_at
		) VALUES (
			'commission_collected', $1, $2, $3::uuid,
			'Commission collected from escrow auto-release', NOW()
		)
	`, amountCents, referenceType, referenceID)
	if err != nil {
		return fmt.Errorf("insert platform_revenue_transactions: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE platform_revenue_accounts
		SET balance_cents = balance_cents + $1,
		    total_collected_cents = total_collected_cents + $1,
		    updated_at = NOW()
		WHERE id = $2::uuid
	`, amountCents, accountID)
	if err != nil {
		return fmt.Errorf("update platform_revenue_accounts: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("platform_revenue_accounts update matched 0 rows (id=%s)", accountID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit platform revenue: %w", err)
	}
	return nil
}
