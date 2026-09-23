package platformrevenuerepository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/repository"
)

type PlatformRevenueRepositoryPostgres struct {
	db *sql.DB
}

func NewPlatformRevenueRepositoryPostgres(db *sql.DB) repository.PlatformRevenueRepository {
	return &PlatformRevenueRepositoryPostgres{db: db}
}

func (r *PlatformRevenueRepositoryPostgres) CreditRevenue(ctx context.Context, amountCents int64, referenceType string, referenceID string) error {
	query := `
		INSERT INTO platform_revenue_transactions (transaction_type, amount_cents, reference_type, reference_id, description, created_at)
		VALUES ('commission_collected', $1, $2, $3, 'Commission collectée auto-release', NOW());
		
		UPDATE platform_revenue_accounts 
		SET balance_cents = balance_cents + $1, 
		    total_commission_cents = total_commission_cents + $1,
		    updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, query, amountCents, referenceType, referenceID)
	if err != nil {
		return fmt.Errorf("failed to credit platform revenue: %w", err)
	}
	return nil
}

// GetBalance retourne le compte global plateforme.
func (r *PlatformRevenueRepositoryPostgres) GetBalance(ctx context.Context) (*repository.PlatformRevenueBalance, error) {
	var bal repository.PlatformRevenueBalance
	err := r.db.QueryRowContext(ctx, `
		SELECT balance_cents, total_collected_cents, updated_at
		FROM platform_revenue_accounts
		ORDER BY updated_at ASC
		LIMIT 1
	`).Scan(&bal.BalanceCents, &bal.TotalCollectedCents, &bal.UpdatedAt)

	if err == sql.ErrNoRows {
		return &repository.PlatformRevenueBalance{
			BalanceCents:        0,
			TotalCollectedCents: 0,
			UpdatedAt:           time.Now().UTC(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get platform revenue balance: %w", err)
	}
	return &bal, nil
}

// ListTransactions filtre optionnel sur created_at [from, to].
func (r *PlatformRevenueRepositoryPostgres) ListTransactions(
	ctx context.Context,
	from, to time.Time,
	limit, offset int,
) ([]*repository.PlatformRevenueTransaction, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id::text, transaction_type, amount_cents,
		       COALESCE(reference_type, ''), COALESCE(reference_id::text, ''),
		       COALESCE(description, ''), created_at
		FROM platform_revenue_transactions
		WHERE ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	var fromArg, toArg interface{}
	if !from.IsZero() {
		fromArg = from.UTC()
	}
	if !to.IsZero() {
		toArg = to.UTC()
	}

	rows, err := r.db.QueryContext(ctx, query, fromArg, toArg, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list platform revenue transactions: %w", err)
	}
	defer rows.Close()

	var out []*repository.PlatformRevenueTransaction
	for rows.Next() {
		var t repository.PlatformRevenueTransaction
		if err := rows.Scan(
			&t.ID, &t.TransactionType, &t.AmountCents,
			&t.ReferenceType, &t.ReferenceID, &t.Description, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan platform revenue transaction: %w", err)
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CountTransactions même filtre que ListTransactions.
func (r *PlatformRevenueRepositoryPostgres) CountTransactions(
	ctx context.Context,
	from, to time.Time,
) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM platform_revenue_transactions
		WHERE ($1::timestamptz IS NULL OR created_at >= $1)
		  AND ($2::timestamptz IS NULL OR created_at <= $2)
	`
	var fromArg, toArg interface{}
	if !from.IsZero() {
		fromArg = from.UTC()
	}
	if !to.IsZero() {
		toArg = to.UTC()
	}

	var n int
	if err := r.db.QueryRowContext(ctx, query, fromArg, toArg).Scan(&n); err != nil {
		return 0, fmt.Errorf("count platform revenue transactions: %w", err)
	}
	return n, nil
}
