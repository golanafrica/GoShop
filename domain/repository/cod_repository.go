package repository

import (
	"context"

	"Goshop/domain/entity"
)

//go:generate mockgen -destination=../../mocks/repository/mock_cod_proof_repository.go -package=repository . CODProofRepository

// ============================================================
// COD PROOF REPOSITORY
// ============================================================

// CODProofRepository définit les opérations sur les preuves de paiement à la livraison
type CODProofRepository interface {
	// Create crée une nouvelle preuve COD
	Create(ctx context.Context, proof *entity.CODProof) error

	// FindByID trouve une preuve par ID
	FindByID(ctx context.Context, id string) (*entity.CODProof, error)

	// FindByOrderID trouve une preuve par commande
	FindByOrderID(ctx context.Context, orderID string) (*entity.CODProof, error)

	// FindByShopID retourne toutes les preuves COD d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// FindByCustomerID retourne toutes les preuves COD d'un client
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CODProof, error)

	// FindByStatus retourne les preuves par statut
	FindByStatus(ctx context.Context, status entity.CODProofStatus) ([]*entity.CODProof, error)

	// FindByShopIDAndStatus retourne les preuves d'une boutique par statut
	FindByShopIDAndStatus(ctx context.Context, shopID string, status entity.CODProofStatus) ([]*entity.CODProof, error)

	// FindPendingProofs retourne les preuves en attente de soumission
	FindPendingProofs(ctx context.Context) ([]*entity.CODProof, error)

	// FindPendingProofsByShopID retourne les preuves en attente d'une boutique
	FindPendingProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// FindConfirmedProofs retourne les preuves confirmées (prêtes pour collecte commission)
	FindConfirmedProofs(ctx context.Context) ([]*entity.CODProof, error)

	// FindConfirmedProofsByShopID retourne les preuves confirmées d'une boutique
	FindConfirmedProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// FindDisputedProofs retourne les preuves en litige
	FindDisputedProofs(ctx context.Context) ([]*entity.CODProof, error)

	// FindDisputedProofsByShopID retourne les preuves en litige d'une boutique
	FindDisputedProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// FindPastDeadlineProofs retourne les preuves dont la date limite est dépassée
	FindPastDeadlineProofs(ctx context.Context) ([]*entity.CODProof, error)

	// FindPastDeadlineProofsByShopID retourne les preuves en retard d'une boutique
	FindPastDeadlineProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// FindIncoherentProofs retourne les preuves incohérentes (montants/dates ne correspondent pas)
	FindIncoherentProofs(ctx context.Context) ([]*entity.CODProof, error)

	// FindByCommissionStatus retourne les preuves par statut de commission
	FindByCommissionStatus(ctx context.Context, status entity.CODCommissionStatus) ([]*entity.CODProof, error)

	// FindCommissionDue retourne les preuves avec commission due (wallet négatif)
	FindCommissionDue(ctx context.Context) ([]*entity.CODProof, error)

	// FindCommissionDueByShopID retourne les commissions dues d'une boutique
	FindCommissionDueByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// FindCompletedProofs retourne les preuves terminées (commission collectée)
	FindCompletedProofs(ctx context.Context) ([]*entity.CODProof, error)

	// FindCompletedProofsByShopID retourne les preuves terminées d'une boutique
	FindCompletedProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error)

	// CountByStatusByShopID compte les preuves par statut pour une boutique
	CountByStatusByShopID(ctx context.Context, shopID string, status entity.CODProofStatus) (int, error)

	// CountPendingByShopID compte les preuves en attente pour une boutique
	CountPendingByShopID(ctx context.Context, shopID string) (int, error)

	// CountConfirmedByShopID compte les preuves confirmées pour une boutique
	CountConfirmedByShopID(ctx context.Context, shopID string) (int, error)

	// CountDisputedByShopID compte les preuves en litige pour une boutique
	CountDisputedByShopID(ctx context.Context, shopID string) (int, error)

	// SumCommissionPendingByShopID somme des commissions en attente pour une boutique
	SumCommissionPendingByShopID(ctx context.Context, shopID string) (int64, error)

	// SumCommissionDueByShopID somme des commissions dues pour une boutique
	SumCommissionDueByShopID(ctx context.Context, shopID string) (int64, error)

	// SumCommissionCollectedByShopID somme des commissions collectées pour une boutique
	SumCommissionCollectedByShopID(ctx context.Context, shopID string) (int64, error)

	// SumTotalCommissionPending somme totale des commissions en attente (toutes boutiques)
	SumTotalCommissionPending(ctx context.Context) (int64, error)

	// SumTotalCommissionDue somme totale des commissions dues (toutes boutiques)
	SumTotalCommissionDue(ctx context.Context) (int64, error)

	// SumTotalCommissionCollected somme totale des commissions collectées (toutes boutiques)
	SumTotalCommissionCollected(ctx context.Context) (int64, error)

	// Update met à jour une preuve
	Update(ctx context.Context, proof *entity.CODProof) error

	// UpdateStatus met à jour uniquement le statut
	UpdateStatus(ctx context.Context, id string, status entity.CODProofStatus) error

	// UpdateCommissionStatus met à jour uniquement le statut de commission
	UpdateCommissionStatus(ctx context.Context, id string, status entity.CODCommissionStatus) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CODProofRepository
}
