package paymentusecase

import (
	"context"
	"fmt"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// RefundPaymentUsecase gère les remboursements
type RefundPaymentUsecase struct {
	paymentRepo repository.PaymentRepository
	registry    PaymentRegistry
}

// NewRefundPaymentUsecase crée une nouvelle instance
func NewRefundPaymentUsecase(
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry,
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

	// 5. 🛡️ PR 5 AUDIT : Vérifier que le paiement est remboursable
	// Vérification explicite du statut refunded pour un message d'erreur clair
	if paymentEntity.Status == entity.PaymentStatusRefunded {
		return nil, fmt.Errorf("payment has already been fully refunded")
	}
	if paymentEntity.Status != entity.PaymentStatusSuccess && paymentEntity.Status != entity.PaymentStatusPartiallyRefunded {
		return nil, fmt.Errorf("only successful or partially refunded payments can be refunded (current status: %s)", paymentEntity.Status)
	}

	// Calculer le montant restant remboursable
	alreadyRefunded := paymentEntity.RefundedAmountCents
	remainingRefundable := paymentEntity.AmountCents - alreadyRefunded

	if remainingRefundable <= 0 {
		return nil, fmt.Errorf("payment has already been fully refunded")
	}

	// 6. Déterminer le montant du remboursement
	refundAmount := req.AmountCents
	if refundAmount == 0 {
		// Si 0, rembourser automatiquement le reste
		refundAmount = remainingRefundable
	}

	if refundAmount > remainingRefundable {
		return nil, fmt.Errorf("refund amount (%d) exceeds remaining refundable amount (%d)", refundAmount, remainingRefundable)
	}

	logger.Info().
		Str("payment_id", paymentID.String()).
		Int64("refund_amount", refundAmount).
		Int64("remaining_refundable", remainingRefundable).
		Str("reason", req.Reason).
		Msg("Initiating refund")

	// 7. Appeler le provider pour le remboursement
	if paymentEntity.ProviderRef != nil {
		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err != nil {
			return nil, fmt.Errorf("provider not found: %w", err)
		}

		customerPhone := ""
		if paymentEntity.CustomerPhone != nil {
			customerPhone = *paymentEntity.CustomerPhone
		}

		operator := ""
		if paymentEntity.Metadata != nil {
			if op, ok := paymentEntity.Metadata["operator"].(string); ok {
				operator = op
			}
		}

		if err := provider.Refund(ctx, *paymentEntity.ProviderRef, refundAmount, customerPhone, operator); err != nil {
			return nil, fmt.Errorf("refund failed at provider: %w", err)
		}
	}

	// 8. 🛡️ PR 5 AUDIT : Mettre à jour le statut et le cumul de façon conditionnelle
	if paymentEntity.Metadata == nil {
		paymentEntity.Metadata = make(map[string]interface{})
	}
	paymentEntity.Metadata["last_refund_reason"] = req.Reason
	paymentEntity.Metadata["last_refund_amount"] = refundAmount
	paymentEntity.Metadata["total_refunded_amount"] = alreadyRefunded + refundAmount

	if refundAmount == remainingRefundable {
		// Remboursement total (soit d'un coup, soit le dernier d'une série de partiels)
		if err := paymentEntity.MarkRefunded(); err != nil {
			return nil, fmt.Errorf("mark refunded: %w", err)
		}
	} else {
		// Remboursement partiel
		if err := paymentEntity.MarkPartiallyRefunded(refundAmount); err != nil {
			return nil, fmt.Errorf("mark partially refunded: %w", err)
		}
	}

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	logger.Info().
		Str("payment_id", paymentID.String()).
		Int64("refund_amount", refundAmount).
		Str("new_status", string(paymentEntity.Status)).
		Msg("Refund completed successfully")

	// 9. Retourner la réponse (mapPaymentToResponse est déjà défini dans check_payment_status.go)
	return mapPaymentToResponse(paymentEntity), nil
}
