package producthandler

import (
	"net/http"
	"strconv"

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

func (ph *PublicProductHandler) GetPublicProducts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("path", r.URL.Path).Msg("Fetching public product catalog")

	// Récupération des paramètres de pagination (défaut: 50, max: 100)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	products, err := ph.listPublicProductsUsecase.Execute(ctx, limit, offset)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list public products")
		return utils.ErrInternalServer
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    products,
		"meta": map[string]int{
			"limit":  limit,
			"offset": offset,
			"count":  len(products),
		},
	})
	return nil
}
