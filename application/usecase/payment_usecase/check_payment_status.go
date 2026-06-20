package paymentusecase

import (
	"context"
	"fmt"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// CheckPaymentStatusUsecase vérifie le statut d'un paiement
type CheckPaymentStatusUsecase struct {
	paymentRepo repository.PaymentRepository
	registry    *payment.Registry
}

// NewCheckPaymentStatusUsecase crée une nouvelle instance
func NewCheckPaymentStatusUsecase(
	paymentRepo repository.PaymentRepository,
	registry *payment.Registry,
) *CheckPaymentStatusUsecase {
	return &CheckPaymentStatusUsecase{
		paymentRepo: paymentRepo,
		registry:    registry,
	}
}

// Execute vérifie le statut
func (uc *CheckPaymentStatusUsecase) Execute(ctx context.Context, paymentID string) (*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Vérifier le shop (multi-tenant)
	_, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Récupérer le paiement
	id, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_id: %w", err)
	}

	paymentEntity, err := uc.paymentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// 3. Si le paiement est dans un état non-terminal, vérifier auprès du provider
	if !paymentEntity.IsTerminal() && paymentEntity.ProviderRef != nil {
		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err == nil {
			status, err := provider.CheckStatus(ctx, *paymentEntity.ProviderRef)
			if err == nil && status.Status != paymentEntity.Status {
				// Mettre à jour le statut
				switch status.Status {
				case entity.PaymentStatusSuccess:
					if err := paymentEntity.MarkSuccess(*paymentEntity.ProviderRef); err == nil {
						_ = uc.paymentRepo.Update(ctx, paymentEntity)
					}
				case entity.PaymentStatusFailed:
					if err := paymentEntity.MarkFailed(status.FailureReason); err == nil {
						_ = uc.paymentRepo.Update(ctx, paymentEntity)
					}
				}
			}
		}
	}

	logger.Debug().
		Str("payment_id", paymentID).
		Str("status", string(paymentEntity.Status)).
		Msg("Payment status checked")

	// 4. Construire la réponse
	return mapPaymentToResponse(paymentEntity), nil
}

func mapPaymentToResponse(p *entity.Payment) *paymentdto.PaymentResponse {
	resp := &paymentdto.PaymentResponse{
		ID:          p.ID.String(),
		OrderID:     p.OrderID.String(),
		Provider:    p.Provider,
		AmountCents: p.AmountCents,
		Currency:    p.Currency,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if p.ProviderRef != nil {
		resp.ProviderRef = *p.ProviderRef
	}
	if p.CustomerPhone != nil {
		resp.CustomerPhone = *p.CustomerPhone
	}
	if p.Description != nil {
		resp.Description = *p.Description
	}
	if p.InitiatedAt != nil {
		resp.InitiatedAt = p.InitiatedAt.Format("2006-01-02 15:04:05")
	}
	if p.CompletedAt != nil {
		resp.CompletedAt = p.CompletedAt.Format("2006-01-02 15:04:05")
	}

	return resp
}
