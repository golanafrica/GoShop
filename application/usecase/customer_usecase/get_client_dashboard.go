package customerusecase

import (
	"context"
	"fmt"

	credit_dto "Goshop/application/dto/credit_dto"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

type GetClientDashboardUsecase struct {
	customerRepo repository.CustomerRepositoryInterface
}

func NewGetClientDashboardUsecase(customerRepo repository.CustomerRepositoryInterface) *GetClientDashboardUsecase {
	return &GetClientDashboardUsecase{
		customerRepo: customerRepo,
	}
}

func (uc *GetClientDashboardUsecase) Execute(ctx context.Context, customerID string) (*credit_dto.ClientDashboardResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Vérification multi-tenant
	if _, err := tenant.FromContext(ctx); err != nil {
		logger.Error().Err(err).Msg("Multi-tenant context missing")
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Exécution de la requête optimisée unique
	dashboard, err := uc.customerRepo.GetClientDashboard(ctx, customerID)
	if err != nil {
		logger.Error().Err(err).Str("customer_id", customerID).Msg("Failed to get client dashboard")
		return nil, fmt.Errorf("failed to retrieve dashboard: %w", err)
	}

	logger.Info().
		Str("customer_id", customerID).
		Int("score", dashboard.CreditScore.Score).
		Int("active_contracts", len(dashboard.ActiveContracts)).
		Int("upcoming_installments", len(dashboard.UpcomingInstallments)).
		Msg("Client dashboard retrieved successfully")

	return dashboard, nil
}
