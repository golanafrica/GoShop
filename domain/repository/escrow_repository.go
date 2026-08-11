package repository

//go:generate mockgen -destination=../../mocks/repository/mock_escrow_account_repository.go -package=repository . EscrowAccountRepository
//go:generate mockgen -destination=../../mocks/repository/mock_delivery_proof_repository.go -package=repository . DeliveryProofRepository

import (
	"context"

	"Goshop/domain/entity"
)

// ============================================================
// ESCROW ACCOUNT REPOSITORY
// ============================================================

// EscrowAccountRepository définit les opérations sur les comptes séquestres
type EscrowAccountRepository interface {
	// Create crée un nouveau compte séquestre
	Create(ctx context.Context, account *entity.EscrowAccount) error

	// FindByID trouve un compte séquestre par ID
	FindByID(ctx context.Context, id string) (*entity.EscrowAccount, error)

	// FindByOrderID trouve un compte séquestre par commande
	FindByOrderID(ctx context.Context, orderID string) (*entity.EscrowAccount, error)

	// FindByCreditContractID trouve un compte séquestre par contrat de crédit
	FindByCreditContractID(ctx context.Context, contractID string) (*entity.EscrowAccount, error)

	// FindByTontineGroupID trouve un compte séquestre par groupe tontine
	FindByTontineGroupID(ctx context.Context, groupID string) (*entity.EscrowAccount, error)

	// FindByReferenceID trouve un compte séquestre par référence (polymorphique)
	FindByReferenceID(ctx context.Context, sourceType entity.EscrowSourceType, referenceID string) (*entity.EscrowAccount, error)

	// FindByStatus retourne les comptes séquestres par statut
	FindByStatus(ctx context.Context, status entity.EscrowAccountStatus) ([]*entity.EscrowAccount, error)

	// FindHeldByShopID retourne les comptes séquestres bloqués d'une boutique
	FindHeldByShopID(ctx context.Context, shopID string) ([]*entity.EscrowAccount, error)

	// FindDisputedByShopID retourne les comptes séquestres en litige d'une boutique
	FindDisputedByShopID(ctx context.Context, shopID string) ([]*entity.EscrowAccount, error)

	// SumHeldAmountByShopID somme des montants bloqués par boutique
	SumHeldAmountByShopID(ctx context.Context, shopID string) (int64, error)

	// SumTotalHeldAmount somme totale des montants bloqués (toutes boutiques)
	SumTotalHeldAmount(ctx context.Context) (int64, error)

	// Update met à jour un compte séquestre
	Update(ctx context.Context, account *entity.EscrowAccount) error

	// UpdateStatus met à jour uniquement le statut
	UpdateStatus(ctx context.Context, id string, status entity.EscrowAccountStatus) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) EscrowAccountRepository
}

// ============================================================
// DELIVERY PROOF REPOSITORY
// ============================================================

// DeliveryProofRepository définit les opérations sur les preuves de livraison
type DeliveryProofRepository interface {
	// Create crée une nouvelle preuve de livraison
	Create(ctx context.Context, proof *entity.DeliveryProof) error

	// FindByID trouve une preuve par ID
	FindByID(ctx context.Context, id string) (*entity.DeliveryProof, error)

	// FindByOrderID trouve une preuve par commande
	FindByOrderID(ctx context.Context, orderID string) (*entity.DeliveryProof, error)

	// FindByCreditContractID trouve une preuve par contrat de crédit
	FindByCreditContractID(ctx context.Context, contractID string) (*entity.DeliveryProof, error)

	// FindByTontineVoucherID trouve une preuve par voucher tontine
	FindByTontineVoucherID(ctx context.Context, voucherID string) (*entity.DeliveryProof, error)

	// FindByReferenceID trouve une preuve par référence (polymorphique)
	FindByReferenceID(ctx context.Context, referenceType string, referenceID string) (*entity.DeliveryProof, error)

	// FindByStatus retourne les preuves par statut escrow
	FindByStatus(ctx context.Context, status entity.EscrowStatus) ([]*entity.DeliveryProof, error)

	// FindPendingShipmentByShopID retourne les preuves en attente d'envoi d'une boutique
	FindPendingShipmentByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error)

	// FindShippedByShopID retourne les preuves expédiées d'une boutique
	FindShippedByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error)

	// FindDeliveredByShopID retourne les preuves livrées d'une boutique
	FindDeliveredByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error)

	// FindDisputedByShopID retourne les preuves en litige d'une boutique
	FindDisputedByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error)

	// FindAutoReleaseEligible retourne les preuves éligibles au déblocage automatique
	FindAutoReleaseEligible(ctx context.Context) ([]*entity.DeliveryProof, error)

	// 🆕 v4.8.4 : Force le delivery_date pour tests/admin (bypass tenant)
	ForceDeliveryDate(ctx context.Context, orderID string, daysAgo int) error

	// FindDisputeDeadlineExpired retourne les preuves dont le délai de litige est expiré
	FindDisputeDeadlineExpired(ctx context.Context) ([]*entity.DeliveryProof, error)

	// CountByStatusByShopID compte les preuves par statut pour une boutique
	CountByStatusByShopID(ctx context.Context, shopID string, status entity.EscrowStatus) (int, error)

	// Update met à jour une preuve
	Update(ctx context.Context, proof *entity.DeliveryProof) error

	// FindAutoReleaseEligibleForUpdate retourne les proofs éligibles avec verrouillage pessimiste
	// Utilise FOR UPDATE SKIP LOCKED pour éviter les race conditions entre instances du scheduler
	FindAutoReleaseEligibleForUpdate(ctx context.Context) ([]*entity.DeliveryProof, error)

	// UpdateEscrowStatus met à jour uniquement le statut escrow
	UpdateEscrowStatus(ctx context.Context, id string, status entity.EscrowStatus) error

	// BeginTx démarre une nouvelle transaction SQL pour opérations atomiques
	// Utilisé par le scheduler d'auto-release pour garantir l'atomicité des opérations
	BeginTx(ctx context.Context) (Tx, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) DeliveryProofRepository
}

// ============================================================
// DELIVERY PROOF EVENT REPOSITORY
// ============================================================

// DeliveryProofEventRepository définit les opérations sur les événements d'audit escrow
type DeliveryProofEventRepository interface {
	// Create crée un nouvel événement
	Create(ctx context.Context, event *entity.DeliveryProofEvent) error

	// FindByProofID retourne tous les événements d'une preuve (triés par date)
	FindByProofID(ctx context.Context, proofID string) ([]*entity.DeliveryProofEvent, error)

	// FindByProofIDAndType retourne les événements d'une preuve par type
	FindByProofIDAndType(ctx context.Context, proofID string, eventType entity.ProofEventType) ([]*entity.DeliveryProofEvent, error)

	// FindByPerformedBy retourne les événements effectués par un utilisateur
	FindByPerformedBy(ctx context.Context, userID string) ([]*entity.DeliveryProofEvent, error)

	// FindRecentByProofID retourne les N derniers événements d'une preuve
	FindRecentByProofID(ctx context.Context, proofID string, limit int) ([]*entity.DeliveryProofEvent, error)

	// CountByProofID compte les événements d'une preuve
	CountByProofID(ctx context.Context, proofID string) (int, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) DeliveryProofEventRepository
}
