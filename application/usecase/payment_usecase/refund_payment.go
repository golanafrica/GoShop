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

// RefundPaymentUsecase gère les remboursements
type RefundPaymentUsecase struct {
	paymentRepo repository.PaymentRepository
	registry    *payment.Registry
}

// NewRefundPaymentUsecase crée une nouvelle instance
func NewRefundPaymentUsecase(
	paymentRepo repository.PaymentRepository,
	registry *payment.Registry,
) *RefundPaymentUsecase {
	return &RefundPaymentUsecase{
		paymentRepo: paymentRepo,
		registry:    registry,
	}
}

// Execute effectue le remboursement
func (uc *RefundPaymentUsecase) Execute(ctx context.Context, req *paymentdto.RefundPaymentRequest) (*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Vérifier le shop (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 3. Récupérer le paiement
	paymentID, err := uuid.Parse(req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_id: %w", err)
	}

	paymentEntity, err := uc.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// 4. Vérifier que le paiement appartient au shop courant
	if paymentEntity.ShopID != shop.ID {
		return nil, fmt.Errorf("payment does not belong to current shop")
	}

	// 5. Vérifier que le paiement est remboursable
	if paymentEntity.Status != entity.PaymentStatusSuccess {
		return nil, fmt.Errorf("only successful payments can be refunded (current status: %s)", paymentEntity.Status)
	}

	// 6. Déterminer le montant du remboursement
	refundAmount := req.AmountCents
	if refundAmount == 0 {
		// Remboursement complet
		refundAmount = paymentEntity.AmountCents
	}

	if refundAmount > paymentEntity.AmountCents {
		return nil, fmt.Errorf("refund amount (%d) exceeds original payment (%d)", refundAmount, paymentEntity.AmountCents)
	}

	logger.Info().
		Str("payment_id", paymentID.String()).
		Int64("refund_amount", refundAmount).
		Str("reason", req.Reason).
		Msg("Initiating refund")

	// 7. Appeler le provider pour le remboursement
	if paymentEntity.ProviderRef != nil {
		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err != nil {
			return nil, fmt.Errorf("provider not found: %w", err)
		}

		if err := provider.Refund(ctx, *paymentEntity.ProviderRef, refundAmount); err != nil {
			return nil, fmt.Errorf("refund failed at provider: %w", err)
		}
	}

	// 8. Mettre à jour le statut du paiement
	if err := paymentEntity.MarkRefunded(); err != nil {
		return nil, fmt.Errorf("mark refunded: %w", err)
	}

	// Ajouter la raison du remboursement dans les métadonnées
	if paymentEntity.Metadata == nil {
		paymentEntity.Metadata = make(map[string]interface{})
	}
	paymentEntity.Metadata["refund_reason"] = req.Reason
	paymentEntity.Metadata["refund_amount"] = refundAmount

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	logger.Info().
		Str("payment_id", paymentID.String()).
		Int64("refund_amount", refundAmount).
		Msg("Refund completed successfully")

	// 9. Retourner la réponse
	return mapPaymentToResponse(paymentEntity), nil
}
