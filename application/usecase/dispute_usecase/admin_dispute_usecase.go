package disputeusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

type AdminDisputeUsecase struct {
	disputeRepo repository.DisputeRepository
}

func NewAdminDisputeUsecase(disputeRepo repository.DisputeRepository) *AdminDisputeUsecase {
	return &AdminDisputeUsecase{
		disputeRepo: disputeRepo,
	}
}

// GetAllDisputesRequest représente la requête pour lister les litiges
type GetAllDisputesRequest struct {
	Status string // "pending", "under_review", "resolved_merchant", "resolved_customer", "cancelled" ou "" pour tout
	Limit  int
	Offset int
}

// GetAllDisputes retourne la liste paginée des litiges
func (uc *AdminDisputeUsecase) GetAllDisputes(ctx context.Context, req *GetAllDisputesRequest) ([]*entity.Dispute, int, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	return uc.disputeRepo.FindAll(ctx, req.Status, limit, offset)
}

// GetDisputeByID retourne les détails d'un litige spécifique
func (uc *AdminDisputeUsecase) GetDisputeByID(ctx context.Context, id string) (*entity.Dispute, error) {
	disputeUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid dispute ID: %w", err)
	}

	dispute, err := uc.disputeRepo.FindByID(ctx, disputeUUID)
	if err != nil {
		return nil, fmt.Errorf("dispute not found: %w", err)
	}

	return dispute, nil
}
