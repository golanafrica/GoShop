package repository

import (
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/domain/entity"
	"context"
)

//go:generate mockgen -destination=../../mocks/repository/mock_order_repository.go -package=repository . OrderRepository

type OrderRepository interface {
	Create(ctx context.Context, order *entity.Order) (*entity.Order, error)
	FindByID(ctx context.Context, id string) (*entity.Order, error)
	FindAll(ctx context.Context) ([]*entity.Order, error)
	FindAllWithPagination(ctx context.Context, limit, offset int, filter orderdto.OrderFilter) ([]*entity.Order, error)
	CountAll(ctx context.Context, filter orderdto.OrderFilter) (int, error)
	CountByCustomerID(ctx context.Context, customerID string) (int, error)

	WithTX(tx Tx) OrderRepository
}
