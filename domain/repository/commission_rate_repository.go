package repository

//go:generate mockgen -destination=../../mocks/repository/mock_commission_rate_repository.go -package=repository . CommissionRateRepository

import (
	"Goshop/domain/entity"
	"context"
)

// CommissionRateRepository gère la persistance des taux de commission
type CommissionRateRepository interface {
	// Avec transaction
	WithTX(tx Tx) CommissionRateRepository

	// CRUD
	Create(ctx context.Context, rate *entity.CommissionRate) error
	Update(ctx context.Context, rate *entity.CommissionRate) error
	FindByShopAndType(ctx context.Context, shopID, transactionType string) (*entity.CommissionRate, error)
	FindByShop(ctx context.Context, shopID string) ([]*entity.CommissionRate, error)

	// Requêtes métier
	GetDefaultRate(ctx context.Context, shopID, transactionType string) (*entity.CommissionRate, error)
}
