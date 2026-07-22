package producthandler

import (
	"net/http"
	"strconv"

	dto "Goshop/application/dto/product_dto"
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type PublicProductHandler struct {
	listPublicProductsUsecase *productuscase.ListPublicProductsUsecase
}

func NewPublicProductHandler(uc *productuscase.ListPublicProductsUsecase) *PublicProductHandler {
	return &PublicProductHandler{
		listPublicProductsUsecase: uc,
	}
}

// @Summary Obtenir le catalogue public des produits
// @Description Retourne la liste paginée et filtrée des produits disponibles publiquement (sans authentification requise).
// @Tags Public Products
// @Accept json
// @Produce json
// @Param search query string false "Terme de recherche (Full Text Search)"
// @Param limit query int false "Nombre de résultats (défaut: 50, max: 100)"
// @Param offset query int false "Décalage (défaut: 0)"
// @Param min_price query int false "Prix minimum en centimes"
// @Param max_price query int false "Prix maximum en centimes"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Router /api/public/products [get]
func (ph *PublicProductHandler) GetPublicProducts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("path", r.URL.Path).Str("query", r.URL.RawQuery).Msg("Fetching public product catalog with filters")

	// Parsing des paramètres pour le catalogue public
	req := &dto.ListProductsRequest{
		Search: r.URL.Query().Get("search"),
		Limit:  50,
		Offset: 0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			req.Limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			req.Offset = o
		}
	}

	products, err := ph.listPublicProductsUsecase.Execute(ctx, req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list public products")
		return utils.ErrInternalServer
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    products,
		"meta": map[string]interface{}{
			"limit":  req.Limit,
			"offset": req.Offset,
			"count":  len(products),
			"search": req.Search,
		},
	})
	return nil
}
