package shophandler

import (
	"context"
	"encoding/json"
	"net/http"

	shopdto "Goshop/application/dto/shop_dto"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ConfigurePaymentUseCaseInterface définit le contrat
type ConfigurePaymentUseCaseInterface interface {
	Execute(ctx context.Context, req *shopdto.UpdatePaymentSettingsRequest, userID uuid.UUID) (*shopdto.PaymentSettingsResponse, error)
	GetPaymentSettings(ctx context.Context, shopID string, userID uuid.UUID) (*shopdto.PaymentSettingsResponse, error)
}

// PaymentSettingsHandler gère les settings de paiement
type PaymentSettingsHandler struct {
	configureUC ConfigurePaymentUseCaseInterface
}

// NewPaymentSettingsHandler crée une nouvelle instance
func NewPaymentSettingsHandler(configureUC ConfigurePaymentUseCaseInterface) *PaymentSettingsHandler {
	return &PaymentSettingsHandler{
		configureUC: configureUC,
	}
}

// @Summary Mettre à jour les paramètres de paiement d'une boutique
// @Description Met à jour les configurations de paiement (Mobile Money, COD, Tontine, etc.) pour une boutique spécifique.
// @Tags Shop Payment Settings
// @Accept json
// @Produce json
// @Param id path string true "ID de la boutique (UUID)"
// @Param request body shopdto.UpdatePaymentSettingsRequest true "Nouvelles configurations de paiement"
// @Success 200 {object} shopdto.PaymentSettingsResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou validation échouée"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (vous n'êtes pas le propriétaire)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{id}/payment-settings [put]
func (h *PaymentSettingsHandler) UpdatePaymentSettings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.ErrInvalidPayload
	}

	// ✅ CORRECTION : Utiliser utils.GetUserID() au lieu de chercher directement dans le contexte
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to get user ID")
		return utils.ErrUnauthorized
	}

	var req shopdto.UpdatePaymentSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	req.ShopID = shopID

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.NewAppError("VALIDATION_FAILED", err.Error(), http.StatusBadRequest)
	}

	resp, err := h.configureUC.Execute(ctx, &req, userID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to update payment settings")

		errMsg := err.Error()
		switch {
		case errMsg == "user is not the owner of this shop":
			return utils.ErrForbidden
		default:
			return utils.NewAppError("UPDATE_FAILED", errMsg, http.StatusBadRequest)
		}
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// @Summary Récupérer les paramètres de paiement d'une boutique
// @Description Retourne les configurations de paiement actuelles d'une boutique spécifique.
// @Tags Shop Payment Settings
// @Accept json
// @Produce json
// @Param id path string true "ID de la boutique (UUID)"
// @Success 200 {object} shopdto.PaymentSettingsResponse
// @Failure 400 {object} utils.AppError "ID de boutique manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (vous n'êtes pas le propriétaire)"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{id}/payment-settings [get]
func (h *PaymentSettingsHandler) GetPaymentSettings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.ErrInvalidPayload
	}

	// ✅ CORRECTION : Utiliser utils.GetUserID()
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Str("shop_id", shopID).Msg("Failed to get user ID")
		return utils.ErrUnauthorized
	}

	resp, err := h.configureUC.GetPaymentSettings(ctx, shopID, userID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get payment settings")

		errMsg := err.Error()
		switch {
		case errMsg == "user is not the owner of this shop":
			return utils.ErrForbidden
		default:
			return utils.NewAppError("GET_FAILED", errMsg, http.StatusBadRequest)
		}
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// @Summary Récupérer les paramètres de paiement de la boutique active
// @Description Retourne les configurations de paiement de la boutique identifiée par le contexte multi-tenant (header X-Shop-Slug).
// @Tags Shop Payment Settings
// @Accept json
// @Produce json
// @Success 200 {object} shopdto.PaymentSettingsResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Contexte multi-tenant manquant ou interdit"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/payment-settings [get]
func (h *PaymentSettingsHandler) GetPaymentSettingsForCurrentShop(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return utils.ErrForbidden
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get user ID")
		return utils.ErrUnauthorized
	}

	resp, err := h.configureUC.GetPaymentSettings(ctx, shop.ID.String(), userID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get payment settings")
		return utils.ErrInternalServer
	}

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// getUserIDFromContext extrait l'ID utilisateur du contexte JWT
// ✅ CORRECTION : Utilise utils.GetUserID() qui est la bonne méthode
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	// Utiliser la fonction utilitaire qui correspond au middleware
	userIDStr, ok := utils.GetUserID(ctx)
	if !ok || userIDStr == "" {
		return uuid.Nil, utils.ErrUnauthorized
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, utils.ErrUnauthorized
	}

	return userID, nil
}
