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

	// 7. Déterminer le nouveau statut après ce refund
	var reservedStatus entity.PaymentStatus
	if refundAmount == remainingRefundable {
		reservedStatus = entity.PaymentStatusRefunded
	} else {
		reservedStatus = entity.PaymentStatusPartiallyRefunded
	}

	// 8. 🛡️ RÉSERVATION ATOMIQUE AVANT l'appel provider (anti-course)
	ok, err := uc.paymentRepo.AddRefundAmount(ctx, paymentID, refundAmount, reservedStatus)
	if err != nil {
		return nil, fmt.Errorf("reserve refund amount: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("refund amount (%d) exceeds remaining refundable amount (concurrent or insufficient)", refundAmount)
	}

	// 9. Appel provider (après réservation)
	// B1 : pas de succès silencieux si le prestataire réel exige une référence.
	requiresProviderCall := paymentRequiresProviderRefund(paymentEntity.Provider)
	if requiresProviderCall {
		if paymentEntity.ProviderRef == nil || *paymentEntity.ProviderRef == "" {
			if _, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount); revErr != nil {
				logger.Error().Err(revErr).
					Str("payment_id", paymentID.String()).
					Msg("CRITICAL: missing provider_ref and reverse reservation failed")
			}
			return nil, fmt.Errorf("cannot refund: missing provider reference for provider %s — manual reconciliation required", paymentEntity.Provider)
		}

		provider, err := uc.registry.Get(paymentEntity.Provider)
		if err != nil {
			if _, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount); revErr != nil {
				logger.Error().Err(revErr).Msg("CRITICAL: failed to reverse refund reservation after provider not found")
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
			// B2 : reverse UNIQUEMENT si refus définitif ; sinon garder la réservation.
			if isDefinitiveRefundRejection(err) {
				if _, revErr := uc.paymentRepo.ReverseRefundAmountAndRecompute(ctx, paymentID, refundAmount); revErr != nil {
					logger.Error().Err(revErr).
						Str("payment_id", paymentID.String()).
						Int64("amount", refundAmount).
						Msg("CRITICAL: definitive provider reject AND reverse failed — manual reconciliation required")
				}
				return nil, fmt.Errorf("refund rejected by provider: %w", err)
			}

			logger.Error().Err(err).
				Str("payment_id", paymentID.String()).
				Int64("amount", refundAmount).
				Str("provider_ref", *paymentEntity.ProviderRef).
				Msg("CRITICAL: provider refund outcome UNKNOWN — reservation kept, do NOT retry blindly")
			return nil, fmt.Errorf(
				"refund outcome uncertain at provider (reservation kept, manual reconciliation required): %w",
				err,
			)
		}
	}

	// 10. Metadata (best-effort) + relecture pour réponse cohérente
	paymentEntity, err = uc.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		logger.Error().Err(err).Msg("refund reserved and provider OK but reload failed")
		// Réservation + provider OK : on ne reverse pas
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
		// Ne pas échouer durement si le cumul est déjà correct
	}

	logger.Info().
		Str("payment_id", paymentID.String()).
		Int64("refund_amount", refundAmount).
		Str("new_status", string(paymentEntity.Status)).
		Msg("Refund completed successfully (atomic reservation)")

	return mapPaymentToResponse(paymentEntity), nil
}

// paymentRequiresProviderRefund détermine si le prestataire nécessite un appel API pour le remboursement
func paymentRequiresProviderRefund(p entity.PaymentProvider) bool {
	switch p {
	case entity.ProviderYengaPay, entity.ProviderOrangeMoney, entity.ProviderMoovMoney, entity.ProviderWave:
		return true
	default:
		// cash / mock : remboursement purement local
		return false
	}
}

// isDefinitiveRefundRejection : refus métier clair uniquement.
// Timeout / 5xx / réseau → false (on conserve la réservation).
func isDefinitiveRefundRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// indices de refus métier (à resserrer quand le client provider sera typé)
	definitiveHints := []string{
		"status 400", "status 401", "status 403", "status 404", "status 422",
		"invalid", "rejected", "not allowed", "insufficient", "already refunded",
		"does not support", "unsupported",
	}

	// indices d'erreurs incertaines (réseau, timeout, serveur)
	ambiguousHints := []string{
		"timeout", "deadline", "temporar", "unavailable", "status 500",
		"status 502", "status 503", "status 504", "connection", "reset by peer",
		"eof", "context canceled", "context deadline",
	}

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

	// Par défaut : inconnu → ne pas reverse (sécurité financière)
	return false
}
