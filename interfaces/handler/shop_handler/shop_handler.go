package shophandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	shopdto "Goshop/application/dto/shop_dto"
	"Goshop/domain/entity"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============ INTERFACES POUR LES USECASES ============

// CreateShopUseCaseInterface définit le contrat pour la création de shop
type CreateShopUseCaseInterface interface {
	Execute(ctx context.Context, name, slug, customDomain string) (*entity.Shop, error)
}

// ListShopsUseCaseInterface définit le contrat pour la liste des shops
type ListShopsUseCaseInterface interface {
	Execute(ctx context.Context) ([]*entity.Shop, error)
}

// UpdateShopUseCaseInterface définit le contrat pour la mise à jour de shop
type UpdateShopUseCaseInterface interface {
	Execute(ctx context.Context, shopID string, name *string, customDomain *string, plan *string, isActive *bool) (*entity.Shop, error)
}

// ============ HANDLER ============

// ShopHandler gère les requêtes HTTP pour les boutiques
type ShopHandler struct {
	createUsecase CreateShopUseCaseInterface
	listUsecase   ListShopsUseCaseInterface
	updateUsecase UpdateShopUseCaseInterface
}

// NewShopHandler crée une nouvelle instance du handler
func NewShopHandler(
	createUsecase CreateShopUseCaseInterface,
	listUsecase ListShopsUseCaseInterface,
	updateUsecase UpdateShopUseCaseInterface,
) *ShopHandler {
	return &ShopHandler{
		createUsecase: createUsecase,
		listUsecase:   listUsecase,
		updateUsecase: updateUsecase,
	}
}

// @Summary Créer une nouvelle boutique
// @Description Crée une nouvelle boutique pour l'utilisateur authentifié.
// @Tags Shops
// @Accept json
// @Produce json
// @Param request body shopdto.CreateShopRequest true "Détails de la boutique à créer"
// @Success 201 {object} shopdto.ShopResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou validation échouée"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 409 {object} utils.AppError "Le slug ou le domaine est déjà utilisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops [post]
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

// @Summary Lister mes boutiques
// @Description Retourne la liste des boutiques dont l'utilisateur authentifié est propriétaire.
// @Tags Shops
// @Accept json
// @Produce json
// @Success 200 {array} shopdto.ShopResponse
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops [get]
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

// @Summary Mettre à jour une boutique
// @Description Met à jour les informations d'une boutique existante (nom, domaine, plan, statut).
// @Tags Shops
// @Accept json
// @Produce json
// @Param id path string true "ID de la boutique (UUID)"
// @Param request body shopdto.UpdateShopRequest true "Nouvelles données de la boutique"
// @Success 200 {object} shopdto.ShopResponse
// @Failure 400 {object} utils.AppError "Payload invalide ou ID manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Interdit (vous n'êtes pas le propriétaire)"
// @Failure 404 {object} utils.AppError "Boutique introuvable"
// @Failure 409 {object} utils.AppError "Le domaine est déjà utilisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/shops/{id} [put]
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
