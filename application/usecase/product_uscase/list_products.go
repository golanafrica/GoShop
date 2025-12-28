// application/usecase/product_uscase/list_product_usecase.go
package productuscase

import (
	"context"

	dto "Goshop/application/dto/product_dto"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type ListProductUsecase struct {
	repo      repository.ProductRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewListProductUsecase(
	repo repository.ProductRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *ListProductUsecase {
	return &ListProductUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("list_product_usecase"),
	}
}

func (pruc *ListProductUsecase) Execute(ctx context.Context, limit, offset int) ([]*dto.ProductResponse, error) {
	logger := zerolog.Ctx(ctx)
	// ✅ Utilise pruc.logger directement — pas de .With().Logger()
	logger.Debug().
		Str("operation", "execute").
		Int("limit", limit).
		Int("offset", offset).
		Msg("Executing list products use case")

	products, err := pruc.repo.FindAll(ctx, limit, offset)
	if err != nil {
		logger.Error().
			Err(err).
			Str("operation", "execute").
			Int("limit", limit).
			Int("offset", offset).
			Msg("Failed to retrieve products from repository")
		return nil, err
	}

	logger.Debug().
		Str("operation", "execute").
		Int("limit", limit).
		Int("offset", offset).
		Int("product_count", len(products)).
		Msg("Products retrieved from repository")

	if len(products) == 0 {
		logger.Info().
			Str("operation", "execute").
			Int("limit", limit).
			Int("offset", offset).
			Msg("No products found")
		return []*dto.ProductResponse{}, nil
	}

	responses := make([]*dto.ProductResponse, 0, len(products))
	for _, product := range products {
		responses = append(responses, &dto.ProductResponse{
			ID:          product.ID,
			Name:        product.Name,
			Description: product.Description,
			PriceCents:  product.PriceCents,
			Stock:       product.Stock,
			CreatedAt:   product.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   product.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	logger.Debug().
		Str("operation", "execute").
		Int("limit", limit).
		Int("offset", offset).
		Int("response_count", len(responses)).
		Msg("Products converted to DTO")

	return responses, nil
}
