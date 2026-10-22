package repository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_withdrawal_repository.go -package=repository . WithdrawalRepository

// WithdrawalRepository definit le contrat pour la persistance des retraits
type WithdrawalRepository interface {
	Create(ctx context.Context, withdrawal *entity.Withdrawal) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Withdrawal, error)
	FindByShopID(ctx context.Context, shopID uuid.UUID, limit, offset int) ([]*entity.Withdrawal, error)
	FindByProviderRef(ctx context.Context, providerRef string) (*entity.Withdrawal, error)
	Update(ctx context.Context, withdrawal *entity.Withdrawal) error
	CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error)

	// FindCompensationPending retourne les retraits a retenter (FIFO).
	// maxAttempts > 0 exclut les rows deja au plafond ; 0 = pas de filtre attempts.
	FindCompensationPending(ctx context.Context, limit int, maxAttempts int) ([]*entity.Withdrawal, error)

	FindStuckPendingWithdrawals(ctx context.Context, limit, minAgeMinutes int) ([]*entity.Withdrawal, error)

	// TransitionToReconciliationRequired passe atomiquement pending|processing → reconciliation_required.
	// Retourne true si 1 ligne a été mise à jour (transition réussie).
	// Retourne false si le statut a déjà changé (webhook concurrent, autre job, etc.).
	TransitionToReconciliationRequired(ctx context.Context, id uuid.UUID, reason string) (bool, error)

	// WithTX retourne le repository attache a une transaction
	WithTX(tx Tx) WithdrawalRepository
}
