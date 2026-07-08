package paymentusecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/rs/zerolog"
)

// Erreurs typées pour le traitement des webhooks
var (
	ErrWebhookValidation = errors.New("webhook validation failed")
	ErrWebhookProcessing = errors.New("webhook processing failed")
)

// ProcessWebhookUsecase traite les webhooks reçus des providers
type ProcessWebhookUsecase struct {
	paymentRepo      repository.PaymentRepository
	registry         PaymentRegistry // ✅ Interface (pas *payment.Registry)
	db               repository.DBExecutor
	shopRepo         repository.ShopRepository
	tontineWebhookUC *ProcessTontineWebhookUsecase
}

// NewProcessWebhookUsecase crée une nouvelle instance
func NewProcessWebhookUsecase(
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry, // ✅ Interface
	db repository.DBExecutor,
	shopRepo repository.ShopRepository,
	tontineWebhookUC *ProcessTontineWebhookUsecase,
) *ProcessWebhookUsecase {
	return &ProcessWebhookUsecase{
		paymentRepo:      paymentRepo,
		registry:         registry,
		db:               db,
		shopRepo:         shopRepo,
		tontineWebhookUC: tontineWebhookUC,
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
		uc.recordWebhook(ctx, providerCode, nil, payload, signature, false, err.Error())
		return fmt.Errorf("%w: %v", ErrWebhookValidation, err)
	}

	// 3. Enregistrer le webhook (signature valide)
	uc.recordWebhook(ctx, providerCode, event, payload, signature, true, "")

	// 🆕 3b. v2.9.0 : Détecter si c'est un webhook tontine
	// La référence tontine est dans event.Metadata["reference"] (format: TONTINE:{groupID}:{cycle}:{participantID})
	if reference, ok := event.Metadata["reference"].(string); ok && IsTontineReference(reference) {
		logger.Info().
			Str("reference", reference).
			Str("transaction_id", event.ExternalID).
			Str("status", string(event.Status)).
			Msg("🎯 Tontine webhook detected, delegating to ProcessTontineWebhookUsecase")

		if uc.tontineWebhookUC == nil {
			return fmt.Errorf("%w: tontine webhook handler not configured", ErrWebhookProcessing)
		}

		err := uc.tontineWebhookUC.Execute(ctx, reference, event.ExternalID, event.Status)
		if err != nil {
			logger.Error().Err(err).Msg("Tontine webhook processing failed")
			return fmt.Errorf("%w: tontine webhook: %v", ErrWebhookProcessing, err)
		}

		logger.Info().
			Str("reference", reference).
			Msg("✅ Tontine webhook processed successfully")
		return nil
	}

	// 4. Trouver le paiement associé (flux standard)
	if event.ProviderRef == "" {
		return fmt.Errorf("%w: missing provider_ref", ErrWebhookValidation)
	}

	paymentEntity, err := uc.paymentRepo.FindByProviderRef(ctx, providerCode, event.ProviderRef)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("provider_ref", event.ProviderRef).
			Msg("Payment not found for webhook")
		return fmt.Errorf("%w: payment not found for provider_ref %s", ErrWebhookProcessing, event.ProviderRef)
	}

	// 4b. Injecter le shop du paiement dans le contexte
	shop, err := uc.shopRepo.FindByID(ctx, paymentEntity.ShopID)
	if err != nil {
		return fmt.Errorf("%w: shop not found for payment: %v", ErrWebhookProcessing, err)
	}
	ctx = tenant.WithTenant(ctx, shop)

	logger.Debug().
		Str("shop_id", shop.ID.String()).
		Str("shop_slug", shop.Slug).
		Msg("Tenant context injected for webhook processing")

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
			return fmt.Errorf("%w: mark success: %v", ErrWebhookProcessing, err)
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
			return fmt.Errorf("%w: mark failed: %v", ErrWebhookProcessing, err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Str("reason", reason).
			Msg("Payment marked as FAILED")

	case entity.PaymentStatusCancelled:
		if err := paymentEntity.MarkCancelled(); err != nil {
			return fmt.Errorf("%w: mark cancelled: %v", ErrWebhookProcessing, err)
		}
		logger.Info().
			Str("payment_id", paymentEntity.ID.String()).
			Msg("Payment marked as CANCELLED")

	case entity.PaymentStatusRefunded:
		if err := paymentEntity.MarkRefunded(); err != nil {
			return fmt.Errorf("%w: mark refunded: %v", ErrWebhookProcessing, err)
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
		return fmt.Errorf("%w: update payment: %v", ErrWebhookProcessing, err)
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
