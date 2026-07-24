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

// 🛡️ CORRECTION AUDIT : Taille maximale du payload webhook (1 MB)
// Protection contre les attaques DoS par gros payloads
const maxWebhookSize = 1 << 20 // 1 MB

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

// @Summary Recevoir les webhooks de paiement
// @Description Endpoint public sécurisé pour recevoir les notifications de changement de statut de paiement des fournisseurs (Wave, Orange, Moov, YengaPay).
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param provider path string true "Code du fournisseur (wave, orange_money, moov_money, yenga_pay)"
// @Param x-webhook-hash header string false "Signature HMAC-SHA256 du payload (YengaPay)"
// @Param X-Signature header string false "Signature HMAC-SHA256 du payload (fallback)"
// @Success 200 {object} paymentdto.WebhookResponse "Webhook reçu et traité avec succès"
// @Failure 400 {object} utils.AppError "Fournisseur inconnu ou signature invalide"
// @Failure 413 {object} utils.AppError "Payload trop volumineux"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Router /webhooks/{provider} [post]
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
		entity.ProviderYengaPay:    true,
		// 🛡️ CORRECTION AUDIT : ProviderMock retiré en production
		// Les mocks ne doivent jamais accepter de webhooks réels
	}
	if !validProviders[provider] {
		return utils.NewAppError("INVALID_PROVIDER", "unknown provider: "+providerCode, http.StatusBadRequest)
	}

	// 🛡️ CORRECTION AUDIT : Limiter la taille du payload pour éviter les attaques DoS
	limitedBody := io.LimitReader(r.Body, maxWebhookSize+1)
	payload, err := io.ReadAll(limitedBody)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read webhook payload")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// 🛡️ Vérifier si le payload dépasse la limite
	if int64(len(payload)) > maxWebhookSize {
		logger.Warn().
			Str("provider", providerCode).
			Int64("size", int64(len(payload))).
			Msg("Webhook payload too large")
		return utils.NewAppError("PAYLOAD_TOO_LARGE", "webhook payload exceeds 1MB limit", http.StatusRequestEntityTooLarge)
	}

	// ✅ CORRECTION : Lire la signature depuis le bon header YengaPay
	// YengaPay utilise "x-webhook-hash" selon la documentation officielle
	signature := r.Header.Get("x-webhook-hash")
	if signature == "" {
		signature = r.Header.Get("X-Webhook-Hash") // Fallback avec majuscules
	}
	if signature == "" {
		signature = r.Header.Get("X-Signature") // Fallback générique
	}
	if signature == "" {
		signature = r.URL.Query().Get("signature") // Fallback query param
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
		} else if errors.Is(err, paymentusecase.ErrWebhookAlreadyProcessed) {
			// 🛡️ CORRECTION AUDIT : Idempotence - webhook déjà traité
			logger.Info().Msg("Webhook already processed (idempotent, returning 200)")
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
