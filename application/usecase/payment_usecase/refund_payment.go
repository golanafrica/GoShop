package paymentusecase

import (
	"context"
	"fmt"
	"strings"

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

	// 7. 🆕 B3 AUDIT : Idempotency check
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

	// 9. 🛡️ RÉSERVATION ATOMIQUE AVANT l'appel provider (anti-course)
	ok, err := uc.paymentRepo.AddRefundAmount(ctx, paymentID, refundAmount, reservedStatus)
	if err != nil {
		_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, err.Error())
		return nil, fmt.Errorf("reserve refund amount: %w", err)
	}
	if !ok {
		_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, "concurrent or insufficient amount")
		return nil, fmt.Errorf("refund amount (%d) exceeds remaining refundable amount (concurrent or insufficient)", refundAmount)
	}

	// 10. Appel provider (après réservation)
	requiresProviderCall := paymentRequiresProviderRefund(paymentEntity.Provider)
	if requiresProviderCall {
		if paymentEntity.ProviderRef == nil || *paymentEntity.ProviderRef == "" {
			if _, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount); revErr != nil {
				logger.Error().Err(revErr).Str("payment_id", paymentID.String()).Msg("CRITICAL: missing provider_ref and reverse reservation failed")
			}
			_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, "missing provider_ref")
			return nil, fmt.Errorf("cannot refund: missing provider reference for provider %s — manual reconciliation required", paymentEntity.Provider)
		}

		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err != nil {
			if _, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount); revErr != nil {
				logger.Error().Err(revErr).Msg("CRITICAL: failed to reverse refund reservation after provider not found")
			}
			_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, err.Error())
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
			if isDefinitiveRefundRejection(err) {
				if _, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount); revErr != nil {
					logger.Error().Err(revErr).Str("payment_id", paymentID.String()).Int64("amount", refundAmount).Msg("CRITICAL: definitive provider reject AND reverse failed")
				}
				_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundFailed, err.Error())
				return nil, fmt.Errorf("refund rejected by provider: %w", err)
			}

			logger.Error().Err(err).Str("payment_id", paymentID.String()).Int64("amount", refundAmount).Str("provider_ref", *paymentEntity.ProviderRef).Msg("CRITICAL: provider refund outcome UNKNOWN")
			_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundReconciliationRequired, err.Error())
			return nil, fmt.Errorf("refund outcome uncertain at provider (reservation kept, manual reconciliation required): %w", err)
		}
	}

	// 11. Metadata (best-effort) + relecture pour réponse cohérente
	paymentEntity, err = uc.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		logger.Error().Err(err).Msg("refund reserved and provider OK but reload failed")
		_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundReconciliationRequired, err.Error())
		return nil, fmt.Errorf("reload payment after refund: %w", err)
	}

	if paymentEntity.Metadata == nil {
		paymentEntity.Metadata = make(map[string]interface{})
	}
	paymentEntity.Metadata["last_refund_reason"] = req.Reason
	paymentEntity.Metadata["last_refund_amount"] = refundAmount
	paymentEntity.Metadata["total_refunded_amount"] = paymentEntity.RefundedAmountCents

	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		logger.Error().Err(err).Msg("refund OK but metadata update failed — cumul already reserved")
	}

	_ = uc.paymentRepo.UpdatePaymentRefundStatus(ctx, refundRow.ID, entity.PaymentRefundCompleted, "")

	logger.Info().Str("payment_id", paymentID.String()).Int64("refund_amount", refundAmount).Str("new_status", string(paymentEntity.Status)).Msg("Refund completed successfully (atomic reservation)")

	return mapPaymentToResponse(paymentEntity), nil
}

func paymentRequiresProviderRefund(p entity.PaymentProvider) bool {
	switch p {
	case entity.ProviderYengaPay, entity.ProviderOrangeMoney, entity.ProviderMoovMoney, entity.ProviderWave:
		return true
	default:
		return false
	}
}

func isDefinitiveRefundRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	definitiveHints := []string{"status 400", "status 401", "status 403", "status 404", "status 422", "invalid", "rejected", "not allowed", "insufficient", "already refunded", "does not support", "unsupported"}
	ambiguousHints := []string{"timeout", "deadline", "temporar", "unavailable", "status 500", "status 502", "status 503", "status 504", "connection", "reset by peer", "eof", "context canceled", "context deadline"}

	for _, h := range ambiguousHints {
		if strings.Contains(msg, h) {
			return false
		}
	}
	for _, h := range definitiveHints {
		if strings.Contains(msg, h) {
			return true
		}
	}
	return false
}
