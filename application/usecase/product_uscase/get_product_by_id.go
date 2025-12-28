// application/usecase/product_uscase/get_product_by_id_usecase.go
package productuscase

import (
	"context"
	"time"

	dto "Goshop/application/dto/product_dto"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type GetProductByIdUsecase struct {
	repo      repository.ProductRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewGetProductByIdUsecase(
	repo repository.ProductRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *GetProductByIdUsecase {
	return &GetProductByIdUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("get_product_by_id"),
	}
}

func (uc *GetProductByIdUsecase) Execute(ctx context.Context, id string) (*dto.ProductResponse, error) {
	logger := zerolog.Ctx(ctx)
	if id == "" {
		logger.Warn().
			Str("operation", "execute").
			Msg("Product ID is empty")
		return nil, utils.ErrProductNotFound
	}

	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Starting product retrieval by ID")

	// Récupération du produit
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Fetching product from repository")

	product, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		logger.Warn().
			Err(err).
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Str("product_id", id).
			Msg("Product not found in repository")
		return nil, utils.ErrProductNotFound
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Str("product_name", product.Name).
		Msg("Product retrieved from repository")

	// Construction de la réponse
	response := &dto.ProductResponse{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
		Stock:       product.Stock,
		CreatedAt:   product.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   product.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	// Log de succès
	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Str("product_name", product.Name).
		Dur("total_duration_ms", duration).
		Int("price_cents", int(product.PriceCents)).
		Int("stock", product.Stock).
		Msg("Product retrieved successfully")

	return response, nil
}
