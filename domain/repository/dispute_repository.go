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

type DisputeStatusFilter string

const (
	DisputeFilterAll              DisputeStatusFilter = ""
	DisputeFilterPending          DisputeStatusFilter = "pending"
	DisputeFilterUnderReview      DisputeStatusFilter = "under_review"
	DisputeFilterResolvedMerchant DisputeStatusFilter = "resolved_merchant"
	DisputeFilterResolvedCustomer DisputeStatusFilter = "resolved_customer"
	DisputeFilterCancelled        DisputeStatusFilter = "cancelled"
)

type DisputeFilters struct {
	Status        DisputeStatusFilter
	InitiatorRole entity.InitiatorRole
	ShopID        *uuid.UUID
	Limit         int
	Offset        int
}

// DisputeRepository définit les opérations sur les litiges
type DisputeRepository interface {
	Create(ctx context.Context, dispute *entity.Dispute) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Dispute, error)
	FindByOrderID(ctx context.Context, orderID uuid.UUID) (*entity.Dispute, error)
	FindByPaymentID(ctx context.Context, paymentID uuid.UUID) (*entity.Dispute, error)
	ExistsByOrderID(ctx context.Context, orderID uuid.UUID) (bool, error)
	Update(ctx context.Context, dispute *entity.Dispute) error

	// 🆕 AJOUT : Méthode pour le dashboard admin (retourne les données + le total pour la pagination)
	FindAll(ctx context.Context, status string, limit, offset int) ([]*entity.Dispute, int, error)

	List(ctx context.Context, filters DisputeFilters) ([]*entity.Dispute, error)
	Count(ctx context.Context, filters DisputeFilters) (int, error)

	WithTX(tx Tx) DisputeRepository
}
