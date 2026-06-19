package shophandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	shopdto "Goshop/application/dto/shop_dto"
	shopusecase "Goshop/application/usecase/shop_usecase"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ShopHandler gère les requêtes HTTP pour les boutiques
type ShopHandler struct {
	createUsecase *shopusecase.CreateShopUsecase
	listUsecase   *shopusecase.ListShopsUsecase
	updateUsecase *shopusecase.UpdateShopUsecase
}

// NewShopHandler crée une nouvelle instance du handler
func NewShopHandler(
	createUsecase *shopusecase.CreateShopUsecase,
	listUsecase *shopusecase.ListShopsUsecase,
	updateUsecase *shopusecase.UpdateShopUsecase,
) *ShopHandler {
	return &ShopHandler{
		createUsecase: createUsecase,
		listUsecase:   listUsecase,
		updateUsecase: updateUsecase,
	}
}

// CreateShop crée une nouvelle boutique
func (h *ShopHandler) CreateShop(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	var req shopdto.CreateShopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.ErrValidationFailed
	}

	shop, err := h.createUsecase.Execute(ctx, req.Name, req.Slug, req.CustomDomain)
	if err != nil {
		logger.Error().Err(err).
			Str("shop_slug", req.Slug).
			Msg("Failed to create shop")

		// Détecter le type d'erreur
		errMsg := err.Error()
		if strings.Contains(errMsg, "already taken") && strings.Contains(errMsg, "slug") {
			return utils.ErrShopSlugTaken
		}
		if strings.Contains(errMsg, "already taken") && strings.Contains(errMsg, "domain") {
			return utils.ErrShopDomainTaken
		}

		return utils.ErrShopCreateFail
	}

	response := shopdto.ShopResponse{
		ID:        shop.ID.String(),
		Name:      shop.Name,
		Slug:      shop.Slug,
		OwnerID:   shop.OwnerID,
		Plan:      string(shop.Plan),
		IsActive:  shop.IsActive,
		CreatedAt: shop.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: shop.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	if shop.CustomDomain != nil {
		response.CustomDomain = *shop.CustomDomain
	}

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// ListShops liste les boutiques de l'utilisateur authentifié
func (h *ShopHandler) ListShops(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shops, err := h.listUsecase.Execute(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list shops")
		return utils.ErrInternalServer
	}

	response := make([]shopdto.ShopResponse, 0, len(shops))
	for _, shop := range shops {
		sr := shopdto.ShopResponse{
			ID:        shop.ID.String(),
			Name:      shop.Name,
			Slug:      shop.Slug,
			OwnerID:   shop.OwnerID,
			Plan:      string(shop.Plan),
			IsActive:  shop.IsActive,
			CreatedAt: shop.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: shop.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		if shop.CustomDomain != nil {
			sr.CustomDomain = *shop.CustomDomain
		}
		response = append(response, sr)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// UpdateShop met à jour une boutique
func (h *ShopHandler) UpdateShop(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.ErrShopInvalidID
	}

	var req shopdto.UpdateShopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload for update")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Update validation failed")
		return utils.ErrValidationFailed
	}

	shop, err := h.updateUsecase.Execute(ctx, shopID, req.Name, req.CustomDomain, req.Plan, req.IsActive)
	if err != nil {
		logger.Error().Err(err).
			Str("shop_id", shopID).
			Msg("Failed to update shop")

		// Détecter le type d'erreur
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}

		errMsg := err.Error()
		if errMsg == "shop not found" || errMsg == "invalid shop ID" {
			return utils.ErrShopNotFound
		}
		if errMsg == "you are not the owner of this shop" {
			return utils.ErrShopNotOwner
		}
		if strings.Contains(errMsg, "already taken") {
			return utils.ErrShopDomainTaken
		}

		return utils.ErrShopUpdateFail
	}

	response := shopdto.ShopResponse{
		ID:        shop.ID.String(),
		Name:      shop.Name,
		Slug:      shop.Slug,
		OwnerID:   shop.OwnerID,
		Plan:      string(shop.Plan),
		IsActive:  shop.IsActive,
		CreatedAt: shop.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: shop.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if shop.CustomDomain != nil {
		response.CustomDomain = *shop.CustomDomain
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}
