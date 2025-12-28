package repository

import (
	"Goshop/domain/entity"
	"context"
)

//go:generate mockgen -destination=../../mocks/repository/mock_product_repository.go -package=repository . ProductRepository

type ProductRepository interface {
	Create(ctx context.Context, product *entity.Product) error
	FindByID(ctx context.Context, id string) (*entity.Product, error)
	FindAll(ctx context.Context, limit, offset int) ([]*entity.Product, error)
	Update(ctx context.Context, product *entity.Product) (*entity.Product, error)
	Delete(ctx context.Context, id string) error

	WithTX(tx Tx) ProductRepository
}
