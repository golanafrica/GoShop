package repository

import (
	dto "Goshop/application/dto/customer_dto"
	"Goshop/domain/entity"

	"context"
)

//go:generate mockgen -destination=../../mocks/repository/mock_customer_repository.go -package=repository . CustomerRepositoryInterface

type CustomerRepositoryInterface interface {
	Create(ctx context.Context, customer *entity.Customer) (*entity.Customer, error)
	FindByCustomerID(ctx context.Context, id string) (*entity.Customer, error)
	FindByEmail(ctx context.Context, email string) (*entity.Customer, error)
	FindAllCustomers(ctx context.Context) ([]*entity.Customer, error)
	UpdateCustomer(ctx context.Context, customer *entity.Customer) (*entity.Customer, error)
	DeleteCustomer(ctx context.Context, id string) error

	// ✅ Nouvelles méthodes pour pagination, tri et comptage
	FindAllCustomersWithPagination(ctx context.Context, limit, offset int, filter dto.CustomerFilter) ([]*entity.Customer, error)
	CountAllCustomers(ctx context.Context, filter dto.CustomerFilter) (int, error)
	FindAllCustomersWithSorting(ctx context.Context, sortBy, order string) ([]*entity.Customer, error)

	// permet d'utiliser txmanager

	WithTX(tx Tx) CustomerRepositoryInterface
}
