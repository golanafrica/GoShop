package repository

import "context"

// PlatformRevenueRepository définit les opérations sur les revenus de la plateforme
type PlatformRevenueRepository interface {
	CreditRevenue(ctx context.Context, amountCents int64, referenceID string, escrowID string) error
}
