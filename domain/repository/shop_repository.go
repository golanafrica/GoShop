package repository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_shop_repository.go -package=repository . ShopRepository

// ShopRepository définit les opérations sur les boutiques
type ShopRepository interface {
	Create(ctx context.Context, shop *entity.Shop) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Shop, error)
	FindBySlug(ctx context.Context, slug string) (*entity.Shop, error)
	FindByCustomDomain(ctx context.Context, domain string) (*entity.Shop, error)
	FindByOwnerID(ctx context.Context, ownerID string) ([]*entity.Shop, error)
	Update(ctx context.Context, shop *entity.Shop) error
	Deactivate(ctx context.Context, id uuid.UUID) error

	WithTX(tx Tx) ShopRepository
}
