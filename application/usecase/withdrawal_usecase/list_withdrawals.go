package withdrawalusecase

import (
	"context"
	"fmt"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
)

// ListWithdrawalsUsecase liste les retraits d'une boutique
type ListWithdrawalsUsecase struct {
	withdrawalRepo repository.WithdrawalRepository
}

func NewListWithdrawalsUsecase(withdrawalRepo repository.WithdrawalRepository) *ListWithdrawalsUsecase {
	return &ListWithdrawalsUsecase{withdrawalRepo: withdrawalRepo}
}

func (uc *ListWithdrawalsUsecase) Execute(ctx context.Context, limit, offset int) ([]*withdrawaldto.WithdrawalResponse, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	withdrawals, err := uc.withdrawalRepo.FindByShopID(ctx, shop.ID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find withdrawals: %w", err)
	}

	var responses []*withdrawaldto.WithdrawalResponse
	for _, w := range withdrawals {
		responses = append(responses, toResponse(w))
	}
	return responses, nil
}

// GetWithdrawal récupère un retrait par ID
func (uc *ListWithdrawalsUsecase) GetWithdrawal(ctx context.Context, id string) (*withdrawaldto.WithdrawalResponse, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	withdrawalUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid withdrawal id: %w", err)
	}

	withdrawal, err := uc.withdrawalRepo.FindByID(ctx, withdrawalUUID)
	if err != nil {
		return nil, fmt.Errorf("find withdrawal: %w", err)
	}
	if withdrawal == nil {
		return nil, fmt.Errorf("withdrawal not found")
	}

	// Vérifier que le retrait appartient au shop
	if withdrawal.ShopID != shop.ID {
		return nil, fmt.Errorf("withdrawal does not belong to this shop")
	}

	return toResponse(withdrawal), nil
}
