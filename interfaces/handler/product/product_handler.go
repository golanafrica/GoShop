package producthandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	dto "Goshop/application/dto/product_dto"
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type ProductHandler struct {
	createProductUsecase  *productuscase.CreateProductUsecase
	listProductUsecase    *productuscase.ListProductUsecase
	getProductByIdUsecase *productuscase.GetProductByIdUsecase
	updateProductUsecase  *productuscase.UpdateProductUsecase
	deleteProductUsecase  *productuscase.DeleteProductUsecase
}

func NewProductHandler(
	repo repository.ProductRepository,
	txManager repository.TxManager,
) *ProductHandler {
	return &ProductHandler{
		createProductUsecase:  productuscase.NewCreateProductUsecase(repo, txManager),
		listProductUsecase:    productuscase.NewListProductUsecase(repo, txManager),
		getProductByIdUsecase: productuscase.NewGetProductByIdUsecase(repo, txManager),
		updateProductUsecase:  productuscase.NewUpdateProductUsecase(repo, txManager),
		deleteProductUsecase:  productuscase.NewDeleteProductUsecase(repo, txManager),
	}
}

func (ph *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("method", r.Method).Str("path", r.URL.Path).Msg("Creating product")

	var req dto.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Validation failed")
		return utils.ErrValidationFailed
	}

	product, err := ph.createProductUsecase.Execute(ctx, req)
	if err != nil {
		logger.Error().Err(err).Str("product_name", req.Name).Msg("Failed to create product")
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrProductCreateFail
	}

	logger.Info().Str("product_id", product.ID).Str("product_name", product.Name).Dur("duration", time.Since(start)).Msg("Product created successfully")
	utils.WriteJSON(w, http.StatusCreated, product)
	return nil
}

func (ph *ProductHandler) GetAllProducts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("method", r.Method).Str("path", r.URL.Path).Str("query", r.URL.RawQuery).Msg("Listing products with filters")

	// Parsing des paramètres de requête pour la recherche FTS et les filtres
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

	if minStr := r.URL.Query().Get("min_price"); minStr != "" {
		if val, err := strconv.ParseInt(minStr, 10, 64); err == nil {
			req.MinPriceCents = val
		}
	}

	if maxStr := r.URL.Query().Get("max_price"); maxStr != "" {
		if val, err := strconv.ParseInt(maxStr, 10, 64); err == nil {
			req.MaxPriceCents = val
		}
	}

	logger.Debug().
		Str("search", req.Search).
		Int64("min_price", req.MinPriceCents).
		Int64("max_price", req.MaxPriceCents).
		Int("limit", req.Limit).
		Int("offset", req.Offset).
		Msg("Filter parameters parsed")

	products, err := ph.listProductUsecase.Execute(ctx, req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list products")
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrInternalServer
	}

	logger.Info().Int("count", len(products)).Dur("duration", time.Since(start)).Msg("Products listed successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(products),
		"data":    products,
	})
	return nil
}

func (ph *ProductHandler) GetProductById(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("product_id", id).Msg("Getting product by ID")

	product, err := ph.getProductByIdUsecase.Execute(ctx, id)
	if err != nil {
		var appErr *utils.AppError
		if errors.As(err, &appErr) && appErr.Code == "PRODUCT_NOT_FOUND" {
			logger.Warn().Err(err).Msg("Product not found")
			return utils.ErrProductNotFound
		}
		logger.Error().Err(err).Msg("Failed to get product")
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrInternalServer
	}

	logger.Info().Str("product_name", product.Name).Msg("Product retrieved successfully")
	utils.WriteJSON(w, http.StatusOK, product)
	return nil
}

func (ph *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("product_id", id).Msg("Updating product")

	var req dto.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON payload for update")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("Update validation failed")
		return utils.ErrValidationFailed
	}

	product := &entity.Product{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Stock:       req.Stock,
	}

	updated, err := ph.updateProductUsecase.Execute(ctx, product)
	if err != nil {
		var appErr *utils.AppError
		if errors.As(err, &appErr) && appErr.Code == "PRODUCT_NOT_FOUND" {
			logger.Warn().Err(err).Msg("Product not found for update")
			return utils.ErrProductNotFound
		}
		logger.Error().Err(err).Str("product_name", req.Name).Msg("Failed to update product")
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrProductUpdateFail
	}

	response := dto.ProductResponse{
		ID:          updated.ID,
		Name:        updated.Name,
		Description: updated.Description,
		PriceCents:  updated.PriceCents,
		Stock:       updated.Stock,
		CreatedAt:   updated.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   updated.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	logger.Info().Str("product_name", response.Name).Msg("Product updated successfully")
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

func (ph *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	logger := zerolog.Ctx(ctx)

	logger.Info().Str("product_id", id).Msg("Deleting product")

	if err := ph.deleteProductUsecase.Execute(ctx, id); err != nil {
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			switch appErr.Code {
			case "PRODUCT_NOT_FOUND":
				logger.Warn().Err(err).Msg("Product not found for deletion")
				return utils.ErrProductNotFound
			case "PRODUCT_DELETE_FAILED":
				logger.Error().Err(err).Msg("Failed to delete product")
				return utils.ErrProductDeleteFail
			default:
				return appErr
			}
		}
		logger.Error().Err(err).Msg("Failed to delete product")
		return utils.ErrProductDeleteFail
	}

	logger.Info().Msg("Product deleted successfully")
	w.WriteHeader(http.StatusNoContent)
	return nil
}
