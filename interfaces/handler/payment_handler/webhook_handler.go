package paymenthandler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	paymentdto "Goshop/application/dto/payment_dto"
	"Goshop/application/metrics"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	withdrawalusecase "Goshop/application/usecase/withdrawal_usecase"
	"Goshop/domain/entity"
	paymentinfra "Goshop/infrastructure/payment"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

const maxWebhookSize = 1 << 20 // 1 MB

type ProcessWebhookUseCaseInterface interface {
	Execute(ctx context.Context, providerCode entity.PaymentProvider, payload []byte, signature string) error
}

// WebhookHandler gère les webhooks des providers
type WebhookHandler struct {
	processUC       ProcessWebhookUseCaseInterface
	processPayoutUC *withdrawalusecase.ProcessPayoutWebhookUsecase
	registry        *paymentinfra.Registry
}

// NewWebhookHandler crée une nouvelle instance
func NewWebhookHandler(
	processUC ProcessWebhookUseCaseInterface,
	processPayoutUC *withdrawalusecase.ProcessPayoutWebhookUsecase,
	registry *paymentinfra.Registry,
) *WebhookHandler {
	return &WebhookHandler{
		processUC:       processUC,
		processPayoutUC: processPayoutUC,
		registry:        registry,
	}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	providerCode := chi.URLParam(r, "provider")
	if providerCode == "" {
		return utils.ErrInvalidPayload
	}

	provider := entity.PaymentProvider(providerCode)
	validProviders := map[entity.PaymentProvider]bool{
		entity.ProviderOrangeMoney: true,
		entity.ProviderMoovMoney:   true,
		entity.ProviderWave:        true,
		entity.ProviderYengaPay:    true,
	}
	if !validProviders[provider] {
		return utils.NewAppError("INVALID_PROVIDER", "unknown provider: "+providerCode, http.StatusBadRequest)
	}

	limitedBody := io.LimitReader(r.Body, maxWebhookSize+1)
	payload, err := io.ReadAll(limitedBody)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read webhook payload")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	if int64(len(payload)) > maxWebhookSize {
		logger.Warn().Str("provider", providerCode).Int64("size", int64(len(payload))).Msg("Webhook payload too large")
		return utils.NewAppError("PAYLOAD_TOO_LARGE", "webhook payload exceeds 1MB limit", http.StatusRequestEntityTooLarge)
	}

	signature := r.Header.Get("x-webhook-hash")
	if signature == "" {
		signature = r.Header.Get("X-Webhook-Hash")
	}
	if signature == "" {
		signature = r.Header.Get("X-Signature")
	}
	if signature == "" {
		signature = r.URL.Query().Get("signature")
	}

	eventType := r.Header.Get("x-yengapay-event")
	if eventType == "" {
		eventType = r.Header.Get("X-Yengapay-Event")
	}
	if eventType == "" {
		eventType = "unknown"
	}

	metrics.WebhookReceivedTotal.WithLabelValues(providerCode, eventType).Inc()

	logger.Info().
		Str("provider", providerCode).
		Str("event_type", eventType).
		Int("payload_size", len(payload)).
		Bool("has_signature", signature != "").
		Msg("Processing webhook")

	// 🆕 ROUTAGE : Si c'est un événement de PAYOUT YengaPay, on le dirige vers le usecase dédié
	if provider == entity.ProviderYengaPay && strings.HasPrefix(eventType, "payout.") {
		err = h.handlePayoutWebhook(ctx, provider, payload, signature, eventType)
	} else {
		// Sinon, on traite comme un paiement entrant classique
		err = h.processUC.Execute(ctx, provider, payload, signature)
	}

	duration := time.Since(start).Seconds()
	metrics.WebhookProcessingDuration.WithLabelValues(providerCode).Observe(duration)

	if err != nil {
		metrics.WebhookProcessedTotal.WithLabelValues(providerCode, "error").Inc()
		metrics.ApplicationErrorsTotal.WithLabelValues("webhook_processing", "webhook_handler").Inc()

		if errors.Is(err, paymentusecase.ErrWebhookValidation) {
			logger.Warn().Err(err).Msg("Webhook validation failed")
			return utils.NewAppError("WEBHOOK_VALIDATION_FAILED", err.Error(), http.StatusBadRequest)
		}

		if errors.Is(err, paymentusecase.ErrWebhookProcessing) {
			logger.Warn().Err(err).Msg("Webhook processing failed (returning 200)")
		} else if errors.Is(err, paymentusecase.ErrWebhookAlreadyProcessed) {
			logger.Info().Msg("Webhook already processed (idempotent, returning 200)")
			metrics.WebhookProcessedTotal.WithLabelValues(providerCode, "duplicate").Inc()
		} else {
			logger.Error().Err(err).Msg("Webhook processing error (returning 200)")
		}
	} else {
		metrics.WebhookProcessedTotal.WithLabelValues(providerCode, "success").Inc()
	}

	response := paymentdto.WebhookResponse{
		Status:  "received",
		Message: "webhook processed",
	}
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// 🆕 handlePayoutWebhook traite spécifiquement les webhooks de retrait (Cash-Out)
func (h *WebhookHandler) handlePayoutWebhook(
	ctx context.Context,
	provider entity.PaymentProvider,
	payload []byte,
	signature string,
	eventType string,
) error {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le provider spécifique depuis le registry
	providerInstance, err := h.registry.Get(provider)
	if err != nil {
		logger.Error().Err(err).Str("provider", string(provider)).Msg("Provider not found in registry for payout webhook")
		return err
	}

	// 2. Valider le webhook via l'instance du provider (qui possède la méthode ValidateWebhook)
	event, err := providerInstance.ValidateWebhook(ctx, payload, signature)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid payout webhook signature")
		return err
	}

	// 3. Extraire la référence (selon la structure de ton event YengaPay)
	providerRef := event.ProviderRef
	if providerRef == "" {
		// Fallback si la ref est dans le JSON brut (ex: "id" ou "transactionId")
		if id, ok := event.Metadata["id"].(string); ok {
			providerRef = id
		}
	}

	if providerRef == "" {
		logger.Warn().Msg("Missing provider reference in payout webhook")
		return nil // Ignorer silencieusement pour éviter les boucles de retry
	}

	// 4. Appeler le usecase de traitement de payout
	err = h.processPayoutUC.Execute(ctx, providerRef, eventType, event.Metadata)
	if err != nil {
		logger.Error().Err(err).Str("provider_ref", providerRef).Msg("Failed to process payout webhook")
		return err
	}

	logger.Info().Str("provider_ref", providerRef).Str("event_type", eventType).Msg("✅ Payout webhook processed successfully")
	return nil
}
