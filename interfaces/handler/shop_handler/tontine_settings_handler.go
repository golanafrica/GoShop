package shophandler

import (
	"encoding/json"
	"net/http"

	shopusecase "Goshop/application/usecase/shop_usecase"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TontineSettingsHandler gère la configuration tontine par boutique
type TontineSettingsHandler struct {
	configureTontineUC *shopusecase.ConfigureTontineUsecase
	shopRepo           repository.ShopRepository // 🆕 Ajouté pour injecter le tenant
}

// NewTontineSettingsHandler crée une nouvelle instance
func NewTontineSettingsHandler(
	configureTontineUC *shopusecase.ConfigureTontineUsecase,
	shopRepo repository.ShopRepository, // 🆕 Ajouté
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

	shopIDStr := chi.URLParam(r, "id")
	shopID, err := uuid.Parse(shopIDStr)
	if err != nil {
		return utils.NewAppError("INVALID_SHOP_ID", "invalid shop ID format", http.StatusBadRequest)
	}

	// 🆕 Récupérer le shop et injecter le tenant
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
	if err != nil {
		return utils.NewAppError("GET_TONTINE_SETTINGS_FAILED", err.Error(), http.StatusNotFound)
	}

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

	shopIDStr := chi.URLParam(r, "id")
	if shopIDStr == "" {
		return utils.ErrInvalidPayload
	}

	shopID, err := uuid.Parse(shopIDStr)
	if err != nil {
		return utils.NewAppError("INVALID_SHOP_ID", "invalid shop ID format", http.StatusBadRequest)
	}

	// 🆕 Récupérer le shop et injecter le tenant
	shop, err := h.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		return utils.NewAppError("SHOP_NOT_FOUND", "shop not found", http.StatusNotFound)
	}
	ctx = tenant.WithTenant(ctx, shop) // 🆕 Injection du tenant

	var req TontineSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	// Convertir en ConfigureTontineRequest avec ShopID explicite
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
	if err != nil {
		return utils.NewAppError("UPDATE_TONTINE_SETTINGS_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, settings)
	return nil
}
