package repository

import (
	"context"
	"time"

	"Goshop/domain/entity"
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

// ProductFilter contient les critères de recherche et de pagination
type ProductFilter struct {
	Search        string
	MinPriceCents int64
	MaxPriceCents int64
	Limit         int
	Offset        int
}

type ProductRepository interface {
	Create(ctx context.Context, product *entity.Product) error
	FindByID(ctx context.Context, id string) (*entity.Product, error)

	// List remplace FindAll pour supporter la recherche FTS et les filtres
	List(ctx context.Context, filter ProductFilter) ([]*entity.Product, error)

	Update(ctx context.Context, product *entity.Product) (*entity.Product, error)
	Delete(ctx context.Context, id string) error

	// FindPublicProducts mis à jour pour accepter les filtres
	FindPublicProducts(ctx context.Context, filter ProductFilter) ([]*PublicProduct, error)

	WithTX(tx Tx) ProductRepository
}
