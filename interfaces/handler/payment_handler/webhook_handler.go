package paymenthandler

import (
	"context"
	"errors"
	"io"
	"net/http"

	paymentdto "Goshop/application/dto/payment_dto"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ProcessWebhookUseCaseInterface définit le contrat
type ProcessWebhookUseCaseInterface interface {
	Execute(ctx context.Context, providerCode entity.PaymentProvider, payload []byte, signature string) error
}

// WebhookHandler gère les webhooks des providers
type WebhookHandler struct {
	processUC ProcessWebhookUseCaseInterface
}

// NewWebhookHandler crée une nouvelle instance
func NewWebhookHandler(processUC ProcessWebhookUseCaseInterface) *WebhookHandler {
	return &WebhookHandler{
		processUC: processUC,
	}
}

// HandleWebhook traite un webhook d'un provider
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// Récupérer le provider depuis l'URL
	providerCode := chi.URLParam(r, "provider")
	if providerCode == "" {
		return utils.ErrInvalidPayload
	}

	// Valider le provider
	provider := entity.PaymentProvider(providerCode)
	validProviders := map[entity.PaymentProvider]bool{
		entity.ProviderOrangeMoney: true,
		entity.ProviderMoovMoney:   true,
		entity.ProviderWave:        true,
		entity.ProviderMock:        true,
	}
	if !validProviders[provider] {
		return utils.NewAppError("INVALID_PROVIDER", "unknown provider: "+providerCode, http.StatusBadRequest)
	}

	// Lire le payload
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read webhook payload")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// Récupérer la signature (header ou query param)
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		signature = r.URL.Query().Get("signature")
	}

	logger.Info().
		Str("provider", providerCode).
		Int("payload_size", len(payload)).
		Bool("has_signature", signature != "").
		Msg("Processing webhook")

	// Traiter le webhook
	err = h.processUC.Execute(ctx, provider, payload, signature)

	// ✅ DISTINGUER LES ERREURS
	if err != nil {
		// Erreur de validation (signature invalide, payload malformé) → 400
		if errors.Is(err, paymentusecase.ErrWebhookValidation) {
			logger.Warn().Err(err).Msg("Webhook validation failed")
			return utils.NewAppError(
				"WEBHOOK_VALIDATION_FAILED",
				err.Error(),
				http.StatusBadRequest,
			)
		}

		// Erreur de traitement (paiement non trouvé, etc.) → 200
		// On log mais on retourne 200 pour éviter les retries du provider
		if errors.Is(err, paymentusecase.ErrWebhookProcessing) {
			logger.Warn().Err(err).Msg("Webhook processing failed (returning 200)")
		} else {
			// Autres erreurs → 200 (pour éviter les retries)
			logger.Error().Err(err).Msg("Webhook processing error (returning 200)")
		}
	}

	// Retourner 200 pour que le provider ne renvoie pas le webhook
	response := paymentdto.WebhookResponse{
		Status:  "received",
		Message: "webhook processed",
	}
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}
