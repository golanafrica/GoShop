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
	ProductID             string `json:"product_id"`
	IsTontineEnabled      bool   `json:"is_tontine_enabled"`
	AllowCommercialCircle bool   `json:"allow_commercial_circle"`
	AllowCorporateCircle  bool   `json:"allow_corporate_circle"`
	AllowFamilyCircle     bool   `json:"allow_family_circle"`
	MinParticipants       int    `json:"min_participants"`
	MaxParticipants       int    `json:"max_participants"`
}

// GetTontineSettings gère GET /api/shops/{id}/tontine-settings?product_id=...
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

// UpdateTontineSettings gère PUT /api/shops/{id}/tontine-settings
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
