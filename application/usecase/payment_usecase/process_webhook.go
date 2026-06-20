package paymentusecase

import (
	"context"
	"encoding/json"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/infrastructure/payment"

	"github.com/rs/zerolog"
)

// ProcessWebhookUsecase traite les webhooks reçus des providers
type ProcessWebhookUsecase struct {
	paymentRepo repository.PaymentRepository
	registry    *payment.Registry
	db          repository.DBExecutor // Pour enregistrer les webhooks bruts
}

// NewProcessWebhookUsecase crée une nouvelle instance
func NewProcessWebhookUsecase(
	paymentRepo repository.PaymentRepository,
	registry *payment.Registry,
	db repository.DBExecutor,
) *ProcessWebhookUsecase {
	return &ProcessWebhookUsecase{
		paymentRepo: paymentRepo,
		registry:    registry,
		db:          db,
	}
}

// Execute traite un webhook
func (uc *ProcessWebhookUsecase) Execute(ctx context.Context, providerCode entity.PaymentProvider, payload []byte, signature string) error {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("provider", string(providerCode)).
		Int("payload_size", len(payload)).
		Msg("Processing webhook")

	// 1. Récupérer le provider
	provider, err := uc.registry.Get(providerCode)
	if err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}

	// 2. Valider et parser le webhook
	event, err := provider.ValidateWebhook(ctx, payload, signature)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid webhook")
		// Enregistrer quand même le webhook pour audit (signature invalide)
		uc.recordWebhook(ctx, providerCode, event, payload, signature, false, err.Error())
		return fmt.Errorf("invalid webhook: %w", err)
	}

	// 3. Enregistrer le webhook (signature valide)
	uc.recordWebhook(ctx, providerCode, event, payload, signature, true, "")

	// 4. Trouver le paiement associé
	if event.ProviderRef == "" {
		return fmt.Errorf("webhook missing provider_ref")
	}

	paymentEntity, err := uc.paymentRepo.FindByProviderRef(ctx, providerCode, event.ProviderRef)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("provider_ref", event.ProviderRef).
			Msg("Payment not found for webhook")
		return fmt.Errorf("payment not found: %w", err)
	}

	// 5. Vérifier si le paiement est déjà dans un état terminal
	if paymentEntity.IsTerminal() {
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Str("status", string(paymentEntity.Status)).
			Msg("Payment already in terminal state, ignoring webhook")
		return nil
	}

	// 6. Transition de statut selon l'événement
	switch event.Status {
	case entity.PaymentStatusSuccess:
		if err := paymentEntity.MarkSuccess(event.ProviderRef); err != nil {
			return fmt.Errorf("mark success: %w", err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Msg("Payment marked as SUCCESS")

	case entity.PaymentStatusFailed:
		reason := "unknown"
		if r, ok := event.Metadata["failure_reason"]; ok {
			if s, ok := r.(string); ok {
				reason = s
			}
		}
		if err := paymentEntity.MarkFailed(reason); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Str("reason", reason).
			Msg("Payment marked as FAILED")

	case entity.PaymentStatusCancelled:
		if err := paymentEntity.MarkCancelled(); err != nil {
			return fmt.Errorf("mark cancelled: %w", err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Msg("Payment marked as CANCELLED")

	case entity.PaymentStatusRefunded:
		if err := paymentEntity.MarkRefunded(); err != nil {
			return fmt.Errorf("mark refunded: %w", err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Msg("Payment marked as REFUNDED")

	default:
		logger.Warn().
			Str("status", string(event.Status)).
			Msg("Unknown webhook status, ignoring")
		return nil
	}

	// 7. Sauvegarder les changements
	if err := uc.paymentRepo.Update(ctx, paymentEntity); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	logger.Info().
		Str("payment_id", paymentEntity.ID.String()).
		Str("status", string(paymentEntity.Status)).
		Msg("Webhook processed successfully")

	return nil
}

// recordWebhook enregistre le webhook dans la table d'audit
func (uc *ProcessWebhookUsecase) recordWebhook(
	ctx context.Context,
	provider entity.PaymentProvider,
	event *payment.WebhookEvent,
	payload []byte,
	signature string,
	signatureValid bool,
	processingError string,
) {
	if uc.db == nil {
		return
	}

	// ✅ FIX : Suppression de providerRef (non utilisé dans l'INSERT)
	var eventType, externalID string
	if event != nil {
		eventType = event.EventType
		externalID = event.ExternalID
	}

	payloadJSON, _ := json.Marshal(map[string]interface{}{
		"raw": string(payload),
	})

	query := `
		INSERT INTO payment_webhooks (
			provider, event_type, external_id, payload, signature,
			signature_validated, processing_error, processed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	processed := signatureValid && processingError == ""

	_, err := uc.db.ExecContext(ctx, query,
		provider,
		eventType,
		externalID,
		payloadJSON,
		signature,
		signatureValid,
		processingError,
		processed,
	)

	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to record webhook")
	}
}
