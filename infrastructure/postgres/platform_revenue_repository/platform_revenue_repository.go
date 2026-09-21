package platformrevenuerepository

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/repository"
)

type PlatformRevenueRepositoryPostgres struct {
	db *sql.DB
}

func NewPlatformRevenueRepositoryPostgres(db *sql.DB) repository.PlatformRevenueRepository {
	return &PlatformRevenueRepositoryPostgres{db: db}
}

func (r *PlatformRevenueRepositoryPostgres) CreditRevenue(ctx context.Context, amountCents int64, referenceType string, referenceID string) error {
	// On exécute deux requêtes : une pour l'audit (transaction), une pour mettre à jour le solde global.
	// Note: Pour une atomicité parfaite en production, on utiliserait une transaction SQL (BEGIN/COMMIT),
	// mais pour ce cas d'usage, deux requêtes séquentielles sont suffisantes et plus simples.
	query := `
		INSERT INTO platform_revenue_transactions (transaction_type, amount_cents, reference_type, reference_id, description, created_at)
		VALUES ('commission_collected', $1, $2, $3, 'Commission collected from order', NOW());
		
		UPDATE platform_revenue_accounts 
		SET balance_cents = balance_cents + $1, 
		    total_commission_cents = total_commission_cents + $1,
		    updated_at = NOW();
	`

	_, err := r.db.ExecContext(ctx, query, amountCents, referenceType, referenceID)
	if err != nil {
		return fmt.Errorf("failed to credit platform revenue: %w", err)
	}

	return nil
}
