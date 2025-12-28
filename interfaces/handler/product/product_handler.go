// interfaces/handler/product/product_handler.go
package producthandler

import (
	dto "Goshop/application/dto/product_dto"
	productuscase "Goshop/application/usecase/product_uscase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type ProductHandler struct {
	createProductUsecase  *productuscase.CreateProductUsecase
	listProductUsecase    *productuscase.ListProductUsecase
	getProductByIdUsecase *productuscase.GetProductByIdUsecase
	updateProductUsecase  *productuscase.UpdateProductUsecase
	deleteProductUsecase  *productuscase.DeleteProductUsecase
	//logger                *setupLogging.Logger
}

func NewProductHandler(
	repo repository.ProductRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *ProductHandler {
	return &ProductHandler{
		createProductUsecase:  productuscase.NewCreateProductUsecase(repo, txManager),
		listProductUsecase:    productuscase.NewListProductUsecase(repo, txManager),
		getProductByIdUsecase: productuscase.NewGetProductByIdUsecase(repo, txManager),
		updateProductUsecase:  productuscase.NewUpdateProductUsecase(repo, txManager),
		deleteProductUsecase:  productuscase.NewDeleteProductUsecase(repo, txManager),
		//logger:                logger.WithComponent("product_handler"),
	}
}

func (ph *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	//logger := ph.logger.WithOperation("create_product")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Creating product")

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
		logger.Error().
			Err(err).
			Str("product_name", req.Name).
			Msg("Failed to create product")

		// Vérifier le type d'erreur
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrProductCreateFail
	}

	logger.Info().
		Str("product_id", product.ID).
		Str("product_name", product.Name).
		Dur("duration", time.Since(start)).
		Msg("Product created successfully")

	utils.WriteJSON(w, http.StatusCreated, product)
	return nil
}

func (ph *ProductHandler) GetAllProducts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	//logger := ph.logger.WithOperation("list_products")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("query", r.URL.RawQuery).
		Msg("Listing products")

	limit, offset := getPaginationParams(r, logger)

	logger.Debug().
		Int("limit", limit).
		Int("offset", offset).
		Msg("Pagination parameters")

	products, err := ph.listProductUsecase.Execute(ctx, limit, offset)
	if err != nil {
		logger.Error().
			Err(err).
			Int("limit", limit).
			Int("offset", offset).
			Msg("Failed to list products")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrInternalServer
	}

	logger.Info().
		Int("count", len(products)).
		Dur("duration", time.Since(start)).
		Msg("Products listed successfully")

	utils.WriteJSON(w, http.StatusOK, products)
	return nil
}

func getPaginationParams(r *http.Request, logger *zerolog.Logger) (limit, offset int) {
	limit = 50
	offset = 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		} else {
			logger.Warn().
				Str("limit", limitStr).
				Err(err).
				Msg("Invalid limit parameter, using default")
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		} else {
			logger.Warn().
				Str("offset", offsetStr).
				Err(err).
				Msg("Invalid offset parameter, using default")
		}
	}

	return limit, offset
}

func (ph *ProductHandler) GetProductById(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	//logger := ph.logger.WithOperation("get_product_by_id")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("product_id", id).
		Msg("Getting product by ID")

	product, err := ph.getProductByIdUsecase.Execute(ctx, id)
	if err != nil {
		// Vérifier si c'est une erreur "produit non trouvé"
		var appErr *utils.AppError
		if errors.As(err, &appErr) && appErr.Code == "PRODUCT_NOT_FOUND" {
			logger.Warn().
				Err(err).
				Msg("Product not found")
			return utils.ErrProductNotFound
		}

		logger.Error().
			Err(err).
			Msg("Failed to get product")

		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrInternalServer
	}

	logger.Info().
		Str("product_name", product.Name).
		Dur("duration", time.Since(time.Now())).
		Msg("Product retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, product)
	return nil
}

func (ph *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	//logger := ph.logger.WithOperation("update_product")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("product_id", id).
		Msg("Updating product")

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
		// Vérifier si c'est une erreur "produit non trouvé"
		var appErr *utils.AppError
		if errors.As(err, &appErr) && appErr.Code == "PRODUCT_NOT_FOUND" {
			logger.Warn().
				Err(err).
				Msg("Product not found for update")
			return utils.ErrProductNotFound
		}

		logger.Error().
			Err(err).
			Str("product_name", req.Name).
			Msg("Failed to update product")

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

	logger.Info().
		Str("product_name", response.Name).
		Dur("duration", time.Since(time.Now())).
		Msg("Product updated successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

func (ph *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	//logger := ph.logger.WithOperation("delete_product")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("product_id", id).
		Msg("Deleting product")

	if err := ph.deleteProductUsecase.Execute(ctx, id); err != nil {
		// Vérifier si c'est une erreur "produit non trouvé"
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			switch appErr.Code {
			case "PRODUCT_NOT_FOUND":
				logger.Warn().
					Err(err).
					Msg("Product not found for deletion")
				return utils.ErrProductNotFound
			case "PRODUCT_DELETE_FAILED":
				logger.Error().
					Err(err).
					Msg("Failed to delete product")
				return utils.ErrProductDeleteFail
			default:
				return appErr
			}
		}

		logger.Error().
			Err(err).
			Msg("Failed to delete product")
		return utils.ErrProductDeleteFail
	}

	logger.Info().Msg("Product deleted successfully")

	w.WriteHeader(http.StatusNoContent)
	return nil
}
