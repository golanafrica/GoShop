package repository

import (
	"context"

	credit_dto "Goshop/application/dto/credit_dto"
	dto "Goshop/application/dto/customer_dto"
	"Goshop/domain/entity"
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

	// GetClientDashboard récupère toutes les infos financières du client en une seule requête optimisée
	GetClientDashboard(ctx context.Context, customerID string) (*credit_dto.ClientDashboardResponse, error)

	// permet d'utiliser txmanager
	WithTX(tx Tx) CustomerRepositoryInterface
}
