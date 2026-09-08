package repository

import (
	"context"

	"Goshop/domain/entity"
)

// ============================================================
// INSTALLMENT PLAN REPOSITORY
// ============================================================

// InstallmentPlanRepository définit les opérations de persistance pour les plans de paiement en tranches.
type InstallmentPlanRepository interface {
	// Create sauvegarde un nouveau plan de paiement en tranches.
	Create(ctx context.Context, plan *entity.InstallmentPlan) error

	// Update met à jour un plan existant (ex: désactivation).
	Update(ctx context.Context, plan *entity.InstallmentPlan) error

	// GetByProductID récupère le plan de paiement pour un produit spécifique.
	// Retourne nil, nil si aucun plan n'existe pour ce produit.
	GetByProductID(ctx context.Context, productID string) (*entity.InstallmentPlan, error)

	// ListByShopID récupère tous les plans configurés par une boutique.
	ListByShopID(ctx context.Context, shopID string) ([]*entity.InstallmentPlan, error)

	// Delete supprime définitivement un plan (généralement utilisé si le produit est supprimé).
	Delete(ctx context.Context, id string) error
}

// ============================================================
// ORDER INSTALLMENT REPOSITORY
// ============================================================

// OrderInstallmentRepository définit les opérations de persistance pour les tranches d'une commande.
type OrderInstallmentRepository interface {
	// CreateBatch crée plusieurs tranches en une seule transaction (lors de la création de la commande).
	CreateBatch(ctx context.Context, installments []*entity.OrderInstallment) error

	// GetByOrderID récupère toutes les tranches d'une commande, triées par numéro de tranche.
	GetByOrderID(ctx context.Context, orderID string) ([]*entity.OrderInstallment, error)

	// MarkAsPaid met à jour le statut d'une tranche spécifique vers 'paid' et enregistre la référence de paiement.
	MarkAsPaid(ctx context.Context, id string, paymentRef string) error

	// GetOverdueInstallments récupère toutes les tranches en retard (status = 'pending' et due_date < now).
	// Utilisé par le scheduler pour envoyer des notifications de relance.
	GetOverdueInstallments(ctx context.Context) ([]*entity.OrderInstallment, error)

	// MarkAsOverdue met à jour le statut d'une tranche vers 'overdue'.
	MarkAsOverdue(ctx context.Context, id string) error
}
