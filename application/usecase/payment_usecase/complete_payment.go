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

// CompletePaymentUsecase complète un paiement TWO_STEP avec un OTP
type CompletePaymentUsecase struct {
	paymentRepo repository.PaymentRepository
	registry    *payment.Registry
}

// NewCompletePaymentUsecase crée une nouvelle instance
func NewCompletePaymentUsecase(
	paymentRepo repository.PaymentRepository,
	registry *payment.Registry,
) *CompletePaymentUsecase {
	return &CompletePaymentUsecase{
		paymentRepo: paymentRepo,
		registry:    registry,
	}
}

// Execute complète le paiement avec l'OTP
func (uc *CompletePaymentUsecase) Execute(ctx context.Context, req *paymentdto.CompletePaymentRequest) (*paymentdto.PaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("payment_id", req.PaymentID).
		Msg("Completing payment with OTP")

	// 2. ✅ Convertir le PaymentID (string) en UUID
	paymentUUID, err := uuid.Parse(req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_id format: %w", err)
	}

	// 3. Récupérer le paiement avec l'UUID
	paymentEntity, err := uc.paymentRepo.FindByID(ctx, paymentUUID)
	if err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// 4. Vérifier que le paiement appartient au shop courant
	if paymentEntity.ShopID != shop.ID {
		return nil, fmt.Errorf("payment does not belong to current shop")
	}

	// 5. Vérifier que le paiement est en statut PROCESSING
	if paymentEntity.Status != entity.PaymentStatusProcessing {
		return nil, fmt.Errorf("payment is not in processing state (current: %s)", paymentEntity.Status)
	}

	// 6. Récupérer le provider
	provider, err := uc.registry.GetAvailable(ctx, paymentEntity.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}

	// 7. Vérifier que le provider supporte la complétion
	completable, ok := provider.(payment.ProviderCompletable)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support payment completion", paymentEntity.Provider)
	}

	// 8. Récupérer les métadonnées nécessaires
	if paymentEntity.Metadata == nil {
		return nil, fmt.Errorf("payment has no metadata (operator info missing)")
	}

	operatorCode, _ := paymentEntity.Metadata["operator"].(string)
	if operatorCode == "" {
		return nil, fmt.Errorf("operator code not found in payment metadata")
	}

	customerPhone := ""
	if paymentEntity.CustomerPhone != nil {
		customerPhone = *paymentEntity.CustomerPhone
	}
	if customerPhone == "" {
		return nil, fmt.Errorf("customer phone not found")
	}

	// 9. Récupérer la référence provider
	providerRef := ""
	if paymentEntity.ProviderRef != nil {
		providerRef = *paymentEntity.ProviderRef
	}
	if providerRef == "" {
		return nil, fmt.Errorf("provider reference not found")
	}

	// 10. Appeler le provider pour compléter le paiement
	completeResp, err := completable.CompletePayment(ctx, providerRef, operatorCode, customerPhone, req.OTP)
	if err != nil {
		// Marquer le paiement comme échoué
		if markErr := paymentEntity.MarkFailed(err.Error()); markErr != nil {
			logger.Error().Err(markErr).Msg("Failed to mark payment as failed")
		}
		if updateErr := uc.paymentRepo.Update(ctx, paymentEntity); updateErr != nil {
			logger.Error().Err(updateErr).Msg("Failed to update payment")
		}
		return nil, fmt.Errorf("complete payment with provider: %w", err)
	}

	// 11. Mettre à jour le statut selon la réponse
	if completeResp.Status == "DONE" {
		if err := paymentEntity.MarkSuccess(completeResp.TransactionID); err != nil {
			return nil, fmt.Errorf("mark payment as success: %w", err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Str("transaction_id", completeResp.TransactionID).
			Msg("Payment completed successfully")
	} else {
		return nil, fmt.Errorf("unexpected status from provider: %s", completeResp.Status)
	}

	// 12. Sauvegarder
	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	// 13. Construire la réponse
	description := ""
	if paymentEntity.Description != nil {
		description = *paymentEntity.Description
	}

	return &paymentdto.PaymentResponse{
		ID:            paymentEntity.ID.String(),
		OrderID:       paymentEntity.OrderID.String(),
		Provider:      paymentEntity.Provider,
		ProviderRef:   providerRef,
		AmountCents:   paymentEntity.AmountCents,
		Currency:      paymentEntity.Currency,
		Status:        paymentEntity.Status,
		CustomerPhone: customerPhone,
		Description:   description,
		CreatedAt:     paymentEntity.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	}, nil
}
