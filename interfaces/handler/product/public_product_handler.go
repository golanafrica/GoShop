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

func (ph *PublicProductHandler) GetPublicProducts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("path", r.URL.Path).Str("query", r.URL.RawQuery).Msg("Fetching public product catalog with filters")

	// Parsing des paramètres pour le catalogue public
	// On utilise 'dto.' car c'est le nom du package défini dans application/dto/product_dto/
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
