package repository

import (
	"Goshop/domain/entity"
	"context"
	"time"
)

//go:generate mockgen -destination=../../mocks/repository/mock_product_repository.go -package=repository . ProductRepository

// PublicProduct est une structure de lecture pour le catalogue public (JOIN products + shops)
type PublicProduct struct {
	ID          string
	Name        string
	Description string
	PriceCents  int64
	Stock       int
	ShopID      string
	ShopName    string
	ShopSlug    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProductRepository interface {
	Create(ctx context.Context, product *entity.Product) error
	FindByID(ctx context.Context, id string) (*entity.Product, error)
	FindAll(ctx context.Context, limit, offset int) ([]*entity.Product, error)
	Update(ctx context.Context, product *entity.Product) (*entity.Product, error)
	Delete(ctx context.Context, id string) error

	// 🆕 Méthode pour le catalogue public (sans contrainte de tenant)
	FindPublicProducts(ctx context.Context, limit, offset int) ([]*PublicProduct, error)

	WithTX(tx Tx) ProductRepository
}
