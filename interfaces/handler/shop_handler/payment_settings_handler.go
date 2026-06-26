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

// UpdatePaymentSettings met à jour les settings de paiement
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

// GetPaymentSettings récupère les settings de paiement
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

// GetPaymentSettingsForCurrentShop récupère les settings pour le shop du contexte
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
