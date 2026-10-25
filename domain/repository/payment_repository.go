package repository

//go:generate mockgen -destination=../../mocks/repository/mock_payment_repository.go -package=repository . PaymentRepository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

// PaymentRepository définit les opérations sur les paiements
type PaymentRepository interface {
	// Create crée un nouveau paiement
	Create(ctx context.Context, payment *entity.Payment) error

	// FindByID trouve un paiement par ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)

	// FindByOrderID trouve tous les paiements d'une commande
	FindByOrderID(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error)

	// FindByProviderRef trouve un paiement par référence provider
	FindByProviderRef(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error)

	// FindByShop finds all payments for a shop with optional filters
	FindByShop(ctx context.Context, shopID uuid.UUID, filters PaymentFilters) ([]*entity.Payment, error)

	// Update met à jour un paiement
	Update(ctx context.Context, payment *entity.Payment) error

	// FindCompletedWithoutCommission récupère les paiements success sans commission collectée
	FindCompletedWithoutCommission(ctx context.Context, limit int) ([]*entity.Payment, error)

	// UpdateCommissionStatus met à jour le statut de commission
	UpdateCommissionStatus(ctx context.Context, paymentID string, status string, commissionCents int64) error

	// 🛡️ CORRECTION AUDIT : FindByProviderRefForUpdate verrouille la ligne atomiquement
	// Empêche les doubles traitements de webhooks concurrents (idempotence)
	FindByProviderRefForUpdate(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error)

	// ⚠️ À utiliser UNIQUEMENT dans le traitement des webhooks entrants où le tenant est inconnu.
	FindByOrderIDUnscoped(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) PaymentRepository

	// 🆕 PR 5 AUDIT : AddRefundAmount met à jour le cumul remboursé de manière atomique.
	// Retourne true si la mise à jour a réussi, false si le montant remboursé dépasserait le montant total.
	AddRefundAmount(ctx context.Context, paymentID uuid.UUID, refundAmountCents int64, newStatus entity.PaymentStatus) (bool, error)

	// ReverseRefundAmount annule une réservation de remboursement (provider a refusé).
	// Décrémente refunded_amount_cents et restaure newStatus (ex: success ou partially_refunded).
	// Retourne false si le cumul est insuffisant pour décrémenter.
	ReverseRefundAmount(ctx context.Context, paymentID uuid.UUID, refundAmountCents int64, newStatus entity.PaymentStatus) (bool, error)

	// FindStuckNonTerminalPayments retourne les paiements non terminaux
	// (pending/processing) plus vieux que minAgeMinutes, pour réconciliation ONE_STEP.
	FindStuckNonTerminalPayments(ctx context.Context, minAgeMinutes int, limit int) ([]*entity.Payment, error)

	// FindSuccessWithEscrowPending : success mais escrow pas encore créé (flag metadata).
	// Peut être implémenté plus tard via JSONB ; optionnel en V1.

	// ReverseRefundAmountAndRecompute décrémente le cumul et recalcule
	// status (success | partially_refunded | refunded) en SQL.
	// Retourne false si cumul insuffisant.
	ReverseRefundAmountAndRecompute(ctx context.Context, paymentID uuid.UUID, refundAmountCents int64) (bool, error)
}

// PaymentFilters représente les filtres pour la recherche de paiements
type PaymentFilters struct {
	Status   *entity.PaymentStatus
	Provider *entity.PaymentProvider
	Limit    int
	Offset   int
}
