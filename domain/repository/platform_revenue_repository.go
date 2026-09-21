package repository

import "context"

// PlatformRevenueRepository définit les opérations pour les revenus de la plateforme.
type PlatformRevenueRepository interface {
	// CreditRevenue crédite le compte de revenus de la plateforme et enregistre la transaction.
	CreditRevenue(ctx context.Context, amountCents int64, referenceType string, referenceID string) error
}
