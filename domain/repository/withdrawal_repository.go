package repository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_withdrawal_repository.go -package=repository . WithdrawalRepository

// WithdrawalRepository définit le contrat pour la persistance des retraits
type WithdrawalRepository interface {
	Create(ctx context.Context, withdrawal *entity.Withdrawal) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Withdrawal, error)
	FindByShopID(ctx context.Context, shopID uuid.UUID, limit, offset int) ([]*entity.Withdrawal, error)
	Update(ctx context.Context, withdrawal *entity.Withdrawal) error
	CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error)
}
