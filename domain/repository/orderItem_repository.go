package repository

import (
	"Goshop/domain/entity"
	"context"
)

//go:generate mockgen -destination=../../mocks/repository/mock_orderitem_repository.go -package=repository . OrderItemRepository

type OrderItemRepository interface {
	Create(ctx context.Context, orderItem *entity.OrderItem) (*entity.OrderItem, error)
	FindByID(ctx context.Context, id string) (*entity.OrderItem, error)
	FindAll(ctx context.Context) ([]*entity.OrderItem, error)

	WithTX(tx Tx) OrderItemRepository
}
