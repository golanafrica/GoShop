package repository

//go:generate mockgen -destination=../../mocks/repository/mock_account_freeze_repository.go -package=repository . AccountFreezeRepository

import (
	"context"
	"time"

	"Goshop/domain/entity"
)

// ============================================================
// ACCOUNT FREEZE REPOSITORY
// ============================================================

// AccountFreezeRepository définit les opérations sur les gels de comptes marchands
type AccountFreezeRepository interface {
	// Create crée un nouveau gel de compte
	Create(ctx context.Context, freeze *entity.AccountFreeze) error

	// FindByID trouve un gel par ID
	FindByID(ctx context.Context, id string) (*entity.AccountFreeze, error)

	// FindByShopID trouve un gel par boutique
	FindByShopID(ctx context.Context, shopID string) (*entity.AccountFreeze, error)

	// FindActiveByShopID trouve le gel actif d'une boutique (non résolu)
	FindActiveByShopID(ctx context.Context, shopID string) (*entity.AccountFreeze, error)

	// FindAll retourne tous les gels
	FindAll(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindActive retourne les gels actifs (non résolus)
	FindActive(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindResolved retourne les gels résolus
	FindResolved(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindByReason retourne les gels par raison
	FindByReason(ctx context.Context, reason entity.FreezeReason) ([]*entity.AccountFreeze, error)

	// FindByResolution retourne les gels par résolution
	FindByResolution(ctx context.Context, resolution entity.FreezeResolution) ([]*entity.AccountFreeze, error)

	// FindGracePeriodExpired retourne les gels dont la période de grâce est expirée
	FindGracePeriodExpired(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindGracePeriodExpiringSoon retourne les gels dont la période de grâce expire bientôt
	// daysRemaining : nombre de jours restants ou moins
	FindGracePeriodExpiringSoon(ctx context.Context, daysRemaining int) ([]*entity.AccountFreeze, error)

	// FindNeedsReminder1 retourne les gels nécessitant le rappel J+1
	FindNeedsReminder1(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindNeedsReminder2 retourne les gels nécessitant le rappel J+3
	FindNeedsReminder2(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindNeedsReminder3 retourne les gels nécessitant le rappel J+6
	FindNeedsReminder3(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindNeedsSuspension retourne les gels nécessitant une suspension définitive
	FindNeedsSuspension(ctx context.Context) ([]*entity.AccountFreeze, error)

	// FindHighValue retourne les gels avec montant dû élevé
	// minAmountCents : montant minimum en centimes
	FindHighValue(ctx context.Context, minAmountCents int64) ([]*entity.AccountFreeze, error)

	// CountActive compte les gels actifs
	CountActive(ctx context.Context) (int, error)

	// CountResolved compte les gels résolus
	CountResolved(ctx context.Context) (int, error)

	// CountByReason compte les gels par raison
	CountByReason(ctx context.Context, reason entity.FreezeReason) (int, error)

	// CountByResolution compte les gels par résolution
	CountByResolution(ctx context.Context, resolution entity.FreezeResolution) (int, error)

	// CountGracePeriodExpired compte les gels avec période de grâce expirée
	CountGracePeriodExpired(ctx context.Context) (int, error)

	// SumAmountDueActive somme des montants dus pour les gels actifs
	SumAmountDueActive(ctx context.Context) (int64, error)

	// SumAmountDueByReason somme des montants dus par raison
	SumAmountDueByReason(ctx context.Context, reason entity.FreezeReason) (int64, error)

	// SumAmountDueResolved somme des montants dus pour les gels résolus
	SumAmountDueResolved(ctx context.Context) (int64, error)

	// SumTotalAmountDue somme totale des montants dus (tous gels)
	SumTotalAmountDue(ctx context.Context) (int64, error)

	// Update met à jour un gel
	Update(ctx context.Context, freeze *entity.AccountFreeze) error

	// UpdateResolution met à jour la résolution d'un gel
	UpdateResolution(ctx context.Context, id string, resolution entity.FreezeResolution, resolvedBy string) error

	// UpdateReminder1 met à jour la date du rappel J+1
	UpdateReminder1(ctx context.Context, id string, sentAt time.Time) error

	// UpdateReminder2 met à jour la date du rappel J+3
	UpdateReminder2(ctx context.Context, id string, sentAt time.Time) error

	// UpdateReminder3 met à jour la date du rappel J+6
	UpdateReminder3(ctx context.Context, id string, sentAt time.Time) error

	// ResolvePaid résout un gel par paiement
	ResolvePaid(ctx context.Context, id string, resolvedBy string) error

	// ResolveSuspended résout un gel par suspension
	ResolveSuspended(ctx context.Context, id string, resolvedBy string) error

	// ResolveWaived résout un gel par annulation
	ResolveWaived(ctx context.Context, id string, resolvedBy string) error

	// ResolveEscalated résout un gel par escalade
	ResolveEscalated(ctx context.Context, id string, resolvedBy string) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) AccountFreezeRepository
}
