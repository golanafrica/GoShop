package shophandler

import (
	"encoding/json"
	"net/http"
	"time"

	"Goshop/application/metrics"
	shopusecase "Goshop/application/usecase/shop_usecase"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TontineSettingsHandler gère la configuration tontine par boutique
type TontineSettingsHandler struct {
	configureTontineUC *shopusecase.ConfigureTontineUsecase
	shopRepo           repository.ShopRepository
}

// NewTontineSettingsHandler crée une nouvelle instance
func NewTontineSettingsHandler(
	configureTontineUC *shopusecase.ConfigureTontineUsecase,
	shopRepo repository.ShopRepository,
) *TontineSettingsHandler {
	return &TontineSettingsHandler{
		configureTontineUC: configureTontineUC,
		shopRepo:           shopRepo,
	}
}

// TontineSettingsRequest représente la requête de configuration tontine
type TontineSettingsRequest struct {
	ProductID             string `json:"product_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	IsTontineEnabled      bool   `json:"is_tontine_enabled" example:"true"`
	AllowCommercialCircle bool   `json:"allow_commercial_circle" example:"true"`
	AllowCorporateCircle  bool   `json:"allow_corporate_circle" example:"true"`
	AllowFamilyCircle     bool   `json:"allow_family_circle" example:"true"`
	MinParticipants       int    `json:"min_participants" example:"4"`
	MaxParticipants       int    `json:"max_participants" example:"12"`
}

// @Summary Récupérer les paramètres de tontine d'un produit
// @Description Retourne la configuration de tontine actuelle pour un produit spécifique dans une boutique donnée.
// @Tags Tontine Settings
// @Accept json
// @Produce json
// @Param id path string true "ID de la boutique (UUID)"
// @Param product_id query string true "ID du produit (UUID)"
// @Success 200 {object} entity.ProductTontineSettings
// @Failure 400 {object} utils.AppError "ID de boutique invalide ou product_id manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (vous n'êtes pas le propriétaire)"
// @Failure 404 {object} utils.AppError "Boutique ou paramètres tontine introuvables"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{id}/tontine-settings [get]
func (h *TontineSettingsHandler) GetTontineSettings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	shopIDStr := chi.URLParam(r, "id")
	shopID, err := uuid.Parse(shopIDStr)
	if err != nil {
		return utils.NewAppError("INVALID_SHOP_ID", "invalid shop ID format", http.StatusBadRequest)
	}

	shop, err := h.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		return utils.NewAppError("SHOP_NOT_FOUND", "shop not found", http.StatusNotFound)
	}
	ctx = tenant.WithTenant(ctx, shop)

	productID := r.URL.Query().Get("product_id")
	if productID == "" {
		return utils.NewAppError("MISSING_PRODUCT_ID", "product_id query parameter is required", http.StatusBadRequest)
	}

	settings, err := h.configureTontineUC.GetTontineSettings(ctx, productID)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec get settings
		metrics.TontineSettingsOperationTotal.WithLabelValues("get", "error").Inc()
		metrics.TontineSettingsDuration.WithLabelValues("get").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("tontine_settings_get", "tontine_settings_handler").Inc()

		logger.Error().
			Err(err).
			Str("shop_id", shopID.String()).
			Str("product_id", productID).
			Float64("duration_seconds", duration).
			Msg("Failed to get tontine settings")

		return utils.NewAppError("GET_TONTINE_SETTINGS_FAILED", err.Error(), http.StatusNotFound)
	}

	// 📊 MÉTRIQUES : Succès get settings
	metrics.TontineSettingsOperationTotal.WithLabelValues("get", "success").Inc()
	metrics.TontineSettingsDuration.WithLabelValues("get").Observe(duration)

	logger.Info().
		Str("shop_id", shopID.String()).
		Str("product_id", productID).
		Float64("duration_seconds", duration).
		Msg("Tontine settings retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, settings)
	return nil
}

// @Summary Mettre à jour les paramètres de tontine d'un produit
// @Description Active ou désactive la tontine pour un produit et configure les paramètres (cercles autorisés, nombre de participants, etc.).
// @Tags Tontine Settings
// @Accept json
// @Produce json
// @Param id path string true "ID de la boutique (UUID)"
// @Param request body shophandler.TontineSettingsRequest true "Nouveaux paramètres de tontine"
// @Success 200 {object} entity.ProductTontineSettings
// @Failure 400 {object} utils.AppError "Payload invalide, ID invalide ou validation échouée"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (vous n'êtes pas le propriétaire)"
// @Failure 404 {object} utils.AppError "Boutique introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{id}/tontine-settings [put]
func (h *TontineSettingsHandler) UpdateTontineSettings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	shopIDStr := chi.URLParam(r, "id")
	if shopIDStr == "" {
		return utils.ErrInvalidPayload
	}

	shopID, err := uuid.Parse(shopIDStr)
	if err != nil {
		return utils.NewAppError("INVALID_SHOP_ID", "invalid shop ID format", http.StatusBadRequest)
	}

	shop, err := h.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		return utils.NewAppError("SHOP_NOT_FOUND", "shop not found", http.StatusNotFound)
	}
	ctx = tenant.WithTenant(ctx, shop)

	var req TontineSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	ucReq := &shopusecase.ConfigureTontineRequest{
		ProductID:             req.ProductID,
		ShopID:                shopID.String(),
		IsTontineEnabled:      req.IsTontineEnabled,
		AllowCommercialCircle: req.AllowCommercialCircle,
		AllowCorporateCircle:  req.AllowCorporateCircle,
		AllowFamilyCircle:     req.AllowFamilyCircle,
		MinParticipants:       req.MinParticipants,
		MaxParticipants:       req.MaxParticipants,
	}

	settings, err := h.configureTontineUC.Execute(ctx, ucReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec update settings
		metrics.TontineSettingsOperationTotal.WithLabelValues("update", "error").Inc()
		metrics.TontineSettingsDuration.WithLabelValues("update").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("tontine_settings_update", "tontine_settings_handler").Inc()

		logger.Error().
			Err(err).
			Str("shop_id", shopID.String()).
			Str("product_id", req.ProductID).
			Bool("tontine_enabled", req.IsTontineEnabled).
			Float64("duration_seconds", duration).
			Msg("Failed to update tontine settings")

		return utils.NewAppError("UPDATE_TONTINE_SETTINGS_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès update settings
	metrics.TontineSettingsOperationTotal.WithLabelValues("update", "success").Inc()
	metrics.TontineSettingsDuration.WithLabelValues("update").Observe(duration)

	logger.Info().
		Str("shop_id", shopID.String()).
		Str("product_id", req.ProductID).
		Bool("tontine_enabled", req.IsTontineEnabled).
		Int("min_participants", req.MinParticipants).
		Int("max_participants", req.MaxParticipants).
		Float64("duration_seconds", duration).
		Msg("Tontine settings updated successfully")

	utils.WriteJSON(w, http.StatusOK, settings)
	return nil
}
