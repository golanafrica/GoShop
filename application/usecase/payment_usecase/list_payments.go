package paymentusecase

import (
	"context"
	"fmt"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ListPaymentsUsecase liste les paiements d'un shop
type ListPaymentsUsecase struct {
	paymentRepo repository.PaymentRepository
}

// NewListPaymentsUsecase crée une nouvelle instance
func NewListPaymentsUsecase(paymentRepo repository.PaymentRepository) *ListPaymentsUsecase {
	return &ListPaymentsUsecase{
		paymentRepo: paymentRepo,
	}
}

// ListPaymentsRequest représente les paramètres de filtrage
type ListPaymentsRequest struct {
	Status   *entity.PaymentStatus
	Provider *entity.PaymentProvider
	Limit    int
	Offset   int
}

// Execute liste les paiements
func (uc *ListPaymentsUsecase) Execute(ctx context.Context, req *ListPaymentsRequest) ([]*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Construire les filtres
	filters := repository.PaymentFilters{
		Status:   req.Status,
		Provider: req.Provider,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	// 3. Récupérer les paiements
	payments, err := uc.paymentRepo.FindByShop(ctx, shop.ID, filters)
	if err != nil {
		return nil, fmt.Errorf("find payments: %w", err)
	}

	logger.Debug().
		Int("count", len(payments)).
		Msg("Payments listed")

	// 4. Mapper vers les DTOs
	responses := make([]*paymentdto.PaymentResponse, 0, len(payments))
	for _, p := range payments {
		responses = append(responses, mapPaymentToResponse(p))
	}

	return responses, nil
}

// Helper pour parser un UUID (utilisé par plusieurs usecases)
//func parseUUIDSafe(s string) (uuid.UUID, error) {
//return uuid.Parse(s)
//}
