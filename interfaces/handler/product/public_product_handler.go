package producthandler

import (
	"net/http"
	"strconv"
	"time"

	dto "Goshop/application/dto/product_dto"
	"Goshop/application/metrics"
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
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("path", r.URL.Path).Str("query", r.URL.RawQuery).Msg("Fetching public product catalog with filters")

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
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec catalogue public
		metrics.ProductsOperationDuration.WithLabelValues("public_list").Observe(duration)
		metrics.ProductsOperationErrors.WithLabelValues("public_list", "query_error").Inc()
		metrics.ApplicationErrorsTotal.WithLabelValues("public_product_list", "public_product_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Failed to list public products")

		return utils.ErrInternalServer
	}

	// 📊 MÉTRIQUES : Succès catalogue public
	metrics.PublicProductsListedTotal.Inc()
	metrics.PublicProductsListedCount.Observe(float64(len(products)))
	metrics.ProductsOperationDuration.WithLabelValues("public_list").Observe(duration)

	logger.Info().
		Int("count", len(products)).
		Int("limit", req.Limit).
		Int("offset", req.Offset).
		Str("search", req.Search).
		Float64("duration_seconds", duration).
		Msg("Public products listed successfully")

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
