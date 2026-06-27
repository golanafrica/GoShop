package repository

import (
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/domain/entity"
	"context"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_order_repository.go -package=repository . OrderRepository

type OrderRepository interface {
	Create(ctx context.Context, order *entity.Order) (*entity.Order, error)
	FindByID(ctx context.Context, id string) (*entity.Order, error)
	FindAll(ctx context.Context) ([]*entity.Order, error)
	FindAllWithPagination(ctx context.Context, limit, offset int, filter orderdto.OrderFilter) ([]*entity.Order, error)
	CountAll(ctx context.Context, filter orderdto.OrderFilter) (int, error)
	CountByCustomerID(ctx context.Context, customerID string) (int, error)

	// 🆕 UpdateStatus met à jour uniquement le statut (rétro-compatible)
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status string) error

	// 🆕 UpdateOrder met à jour TOUS les champs de la commande (pour workflow cash)
	UpdateOrder(ctx context.Context, order *entity.Order) error

	// 🆕 FindCashPendingByShop retourne les commandes cash en attente de confirmation
	FindCashPendingByShop(ctx context.Context, shopID string) ([]*entity.Order, error)

	WithTX(tx Tx) OrderRepository
}
