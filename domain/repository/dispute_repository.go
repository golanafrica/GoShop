package repository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_dispute_repository.go -package=repository . DisputeRepository

// ============================================================
// 🆕 DISPUTE REPOSITORY - Interface pour la gestion des litiges
// ============================================================
// Cette interface définit les opérations CRUD et de recherche
// pour les litiges (disputes) liés aux commandes et paiements.
// L'implémentation Postgres se trouve dans :
// infrastructure/postgres/dispute/dispute_repository.go
// ============================================================

// DisputeStatusFilter représente un filtre par statut de litige
type DisputeStatusFilter string

const (
	DisputeFilterAll              DisputeStatusFilter = ""
	DisputeFilterPending          DisputeStatusFilter = "pending"
	DisputeFilterUnderReview      DisputeStatusFilter = "under_review"
	DisputeFilterResolvedMerchant DisputeStatusFilter = "resolved_merchant"
	DisputeFilterResolvedCustomer DisputeStatusFilter = "resolved_customer"
	DisputeFilterCancelled        DisputeStatusFilter = "cancelled"
)

// DisputeFilters regroupe tous les filtres possibles pour la liste des litiges
type DisputeFilters struct {
	Status        DisputeStatusFilter  // Filtrer par statut
	InitiatorRole entity.InitiatorRole // Filtrer par rôle de l'initiateur (customer, merchant, admin)
	ShopID        *uuid.UUID           // Filtrer par boutique (utile pour l'admin)
	Limit         int                  // Nombre max de résultats (pagination)
	Offset        int                  // Décalage pour la pagination
}

// DisputeRepository définit les opérations sur les litiges
type DisputeRepository interface {
	// Create sauvegarde un nouveau litige en base de données
	// Retourne une erreur si le litige existe déjà (contrainte UNIQUE sur order_id)
	Create(ctx context.Context, dispute *entity.Dispute) error

	// FindByID récupère un litige par son identifiant unique
	// Retourne ErrDisputeNotFound si aucun litige n'est trouvé
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Dispute, error)

	// FindByOrderID récupère le litige associé à une commande
	// Retourne nil si aucun litige n'existe pour cette commande
	FindByOrderID(ctx context.Context, orderID uuid.UUID) (*entity.Dispute, error)

	// FindByPaymentID récupère le litige associé à un paiement
	// Retourne nil si aucun litige n'existe pour ce paiement
	FindByPaymentID(ctx context.Context, paymentID uuid.UUID) (*entity.Dispute, error)

	// ExistsByOrderID vérifie si un litige (non annulé) existe déjà pour une commande
	// Utilisé pour empêcher l'ouverture de litiges multiples sur la même commande
	ExistsByOrderID(ctx context.Context, orderID uuid.UUID) (bool, error)

	// Update met à jour un litige existant (statut, notes de résolution, etc.)
	// Retourne une erreur si le litige n'existe pas
	Update(ctx context.Context, dispute *entity.Dispute) error

	// List retourne une liste de litiges selon les filtres appliqués
	// Utilisé principalement par l'interface admin pour la gestion des litiges
	List(ctx context.Context, filters DisputeFilters) ([]*entity.Dispute, error)

	// Count retourne le nombre total de litiges correspondant aux filtres
	// Utilisé pour la pagination côté client
	Count(ctx context.Context, filters DisputeFilters) (int, error)

	WithTX(tx Tx) DisputeRepository
}
