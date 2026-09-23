package repository

import (
	"context"
	"time"
)

// PlatformRevenueTransaction est une ligne d'audit des commissions plateforme.
type PlatformRevenueTransaction struct {
	ID              string
	TransactionType string
	AmountCents     int64
	ReferenceType   string
	ReferenceID     string
	Description     string
	CreatedAt       time.Time
}

// PlatformRevenueBalance solde du compte global plateforme.
type PlatformRevenueBalance struct {
	BalanceCents        int64
	TotalCollectedCents int64
	UpdatedAt           time.Time
}

// PlatformRevenueRepository opérations trésorerie plateforme.
type PlatformRevenueRepository interface {
	// CreditRevenue crédite le compte et enregistre la transaction (atomique).
	CreditRevenue(ctx context.Context, amountCents int64, referenceType string, referenceID string) error

	// GetBalance retourne le solde du compte global (0 si aucun compte).
	GetBalance(ctx context.Context) (*PlatformRevenueBalance, error)

	// ListTransactions liste les commissions (filtre optionnel from/to UTC, pagination).
	// from / to zero = pas de borne de ce côté.
	ListTransactions(ctx context.Context, from, to time.Time, limit, offset int) ([]*PlatformRevenueTransaction, error)

	// CountTransactions compte les lignes pour la même fenêtre.
	CountTransactions(ctx context.Context, from, to time.Time) (int, error)
}
