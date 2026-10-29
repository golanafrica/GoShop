package paymentusecase

import (
	"context"
	"errors"
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

	// 5. Vérifier que le paiement est remboursable
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

	// 7. Idempotency check
	idemKey := req.IdempotencyKey
	if idemKey == "" {
		idemKey = uuid.New().String()
	}

	existing, err := uc.paymentRepo.FindPaymentRefundByIdempotency(ctx, shop.ID, idemKey)
	if err != nil {
		return nil, fmt.Errorf("lookup refund idempotency: %w", err)
	}
	if existing != nil {
		switch existing.Status {
		case entity.PaymentRefundCompleted:
			p, err := uc.paymentRepo.FindByID(ctx, paymentID)
			if err != nil {
				return nil, err
			}
			return mapPaymentToResponse(p), nil
		case entity.PaymentRefundPending, entity.PaymentRefundReconciliationRequired:
			return nil, fmt.Errorf("refund already in progress or awaiting reconciliation (key=%s)", idemKey)
		case entity.PaymentRefundFailed:
			return nil, fmt.Errorf("refund previously failed for this idempotency key — use a new key")
		}
	}

	refundRow := &entity.PaymentRefund{
		ID:             uuid.New(),
		PaymentID:      paymentID,
		ShopID:         shop.ID,
		AmountCents:    refundAmount,
		Status:         entity.PaymentRefundPending,
		Reason:         req.Reason,
		IdempotencyKey: idemKey,
		ProviderRef:    paymentEntity.ProviderRef,
	}
	if err := uc.paymentRepo.CreatePaymentRefund(ctx, refundRow); err != nil {
		return nil, fmt.Errorf("create payment refund: %w", err)
	}

	// 8. Déterminer le nouveau statut après ce refund
	var reservedStatus entity.PaymentStatus
	if refundAmount == remainingRefundable {
		reservedStatus = entity.PaymentStatusRefunded
	} else {
		reservedStatus = entity.PaymentStatusPartiallyRefunded
	}

	// 9. RÉSERVATION ATOMIQUE AVANT l'appel provider (anti-course)
	ok, err := uc.paymentRepo.AddRefundAmount(ctx, paymentID, refundAmount, reservedStatus)
	if err != nil {
		if updateErr := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, err.Error()); updateErr != nil {
			logger.Error().Err(updateErr).Msg("CRITICAL: Failed to update refund status to Failed after reserve error")
		}
		return nil, fmt.Errorf("reserve refund amount: %w", err)
	}
	if !ok {
		if updateErr := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, "concurrent or insufficient amount"); updateErr != nil {
			logger.Error().Err(updateErr).Msg("CRITICAL: Failed to update refund status to Failed after reserve rejection")
		}
		return nil, fmt.Errorf("refund amount (%d) exceeds remaining refundable amount (concurrent or insufficient)", refundAmount)
	}

	// 10. Appel provider (après réservation)
	requiresProviderCall := paymentRequiresProviderRefund(paymentEntity.Provider)
	if requiresProviderCall {
		if paymentEntity.ProviderRef == nil || *paymentEntity.ProviderRef == "" {
			if revErr := uc.reverseRefundAndLog(ctx, paymentID, refundAmount, refundRow.ID, "missing provider_ref", logger); revErr != nil {
				logger.Error().Err(revErr).Msg("CRITICAL: Reverse failed, manual DB intervention required")
			}
			return nil, fmt.Errorf("cannot refund: missing provider reference for provider %s", paymentEntity.Provider)
		}

		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err != nil {
			if revErr := uc.reverseRefundAndLog(ctx, paymentID, refundAmount, refundRow.ID, "provider not found", logger); revErr != nil {
				logger.Error().Err(revErr).Msg("CRITICAL: Reverse failed, manual DB intervention required")
			}
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
			var definitive *payment.ErrCashOutDefinitive
			if errors.As(err, &definitive) {
				if revErr := uc.reverseRefundAndLog(ctx, paymentID, refundAmount, refundRow.ID, "provider definitive rejection", logger); revErr != nil {
					logger.Error().Err(revErr).Msg("CRITICAL: Definitive reject BUT reverse failed. Manual DB intervention required.")
				}
				if updateErr := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, err.Error()); updateErr != nil {
					logger.Error().Err(updateErr).Msg("CRITICAL: Failed to update refund status to Failed after definitive rejection")
				}
				return nil, fmt.Errorf("refund rejected by provider: %w", err)
			}

			// UNKNOWN ERROR: DO NOT REVERSE. Keep reservation.
			logger.Error().Err(err).Str("payment_id", paymentID.String()).Int64("amount", refundAmount).
				Msg("CRITICAL: provider refund outcome UNKNOWN. Reservation kept.")
			if updateErr := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundReconciliationRequired, err.Error()); updateErr != nil {
				logger.Error().Err(updateErr).Msg("CRITICAL: Failed to set refund to ReconciliationRequired")
			}
			return nil, fmt.Errorf("refund outcome uncertain at provider (reservation kept, manual reconciliation required): %w", err)
		}
	}

	// 11. Provider succeeded. Now we MUST update our DB.
	// Re-read payment to get latest state
	paymentEntity, err = uc.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		logger.Error().Err(err).Msg("CRITICAL: provider refund OK but reload failed")
		if updateErr := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundReconciliationRequired, err.Error()); updateErr != nil {
			logger.Error().Err(updateErr).Msg("CRITICAL: Failed to set refund to ReconciliationRequired after reload fail")
		}
		return nil, fmt.Errorf("reload payment after refund: %w", err)
	}

	if paymentEntity.Metadata == nil {
		paymentEntity.Metadata = make(map[string]interface{})
	}
	paymentEntity.Metadata["last_refund_reason"] = req.Reason
	paymentEntity.Metadata["last_refund_amount"] = refundAmount
	paymentEntity.Metadata["total_refunded_amount"] = paymentEntity.RefundedAmountCents

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		logger.Error().Err(err).Msg("CRITICAL: refund OK at provider but metadata update failed")
		if updateErr := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundReconciliationRequired, err.Error()); updateErr != nil {
			logger.Error().Err(updateErr).Msg("CRITICAL: Failed to set refund to ReconciliationRequired after metadata update fail")
		}
		return nil, fmt.Errorf("update payment metadata after refund: %w", err)
	}

	// Final step: Mark refund as completed
	if err := uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundCompleted, ""); err != nil {
		logger.Error().Err(err).Msg("CRITICAL: Refund completed at provider and DB, but final status update failed. Manual reconciliation required.")
		// We don't return error here because the refund IS done, but we flag it for ops.
	}

	logger.Info().Str("payment_id", paymentID.String()).Int64("refund_amount", refundAmount).Str("new_status", string(paymentEntity.Status)).Msg("Refund completed successfully (atomic reservation)")

	return mapPaymentToResponse(paymentEntity), nil
}

// reverseRefundAndLog est un helper pour gérer proprement l'échec de l'annulation de réservation
func (uc *RefundPaymentUsecase) reverseRefundAndLog(ctx context.Context, paymentID uuid.UUID, refundAmount int64, refundID uuid.UUID, reason string, logger *zerolog.Logger) error {
	ok, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount)
	if revErr != nil {
		logger.Error().Err(revErr).Str("payment_id", paymentID.String()).Int64("amount", refundAmount).
			Msgf("CRITICAL: Failed to reverse refund reservation (%s)", reason)
		// Attempt to flag the refund row as needing reconciliation if the reverse failed
		_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundID, entity.PaymentRefundReconciliationRequired, fmt.Sprintf("reverse failed: %v", revErr))
		return revErr
	}
	if !ok {
		logger.Error().Str("payment_id", paymentID.String()).Int64("amount", refundAmount).
			Msgf("CRITICAL: Reverse refund reservation returned false (%s)", reason)
		_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundID, entity.PaymentRefundReconciliationRequired, fmt.Sprintf("reverse failed: ok=false, %s", reason))
		return fmt.Errorf("reverse refund failed: ok=false")
	}
	return nil
}

func paymentRequiresProviderRefund(p entity.PaymentProvider) bool {
	switch p {
	case entity.ProviderYengaPay, entity.ProviderOrangeMoney, entity.ProviderMoovMoney, entity.ProviderWave:
		return true
	default:
		return false
	}
}
