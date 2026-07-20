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
}

func NewListProductUsecase(
	repo repository.ProductRepository,
	txManager repository.TxManager,
) *ListProductUsecase {
	return &ListProductUsecase{
		repo:      repo,
		txManager: txManager,
	}
}

func (pruc *ListProductUsecase) Execute(ctx context.Context, req *dto.ListProductsRequest) ([]*dto.ProductResponse, error) {
	logger := zerolog.Ctx(ctx)

	// Mapping DTO -> Domain Filter
	filter := repository.ProductFilter{
		Search:        req.Search,
		MinPriceCents: req.MinPriceCents,
		MaxPriceCents: req.MaxPriceCents,
		Limit:         req.Limit,
		Offset:        req.Offset,
	}

	logger.Debug().
		Str("search", filter.Search).
		Int64("min_price", filter.MinPriceCents).
		Int64("max_price", filter.MaxPriceCents).
		Int("limit", filter.Limit).
		Int("offset", filter.Offset).
		Msg("Executing list products use case with filters")

	products, err := pruc.repo.List(ctx, filter)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to retrieve products from repository")
		return nil, err
	}

	if len(products) == 0 {
		logger.Info().Msg("No products found matching criteria")
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

	logger.Info().
		Int("count", len(responses)).
		Msg("Products listed successfully")

	return responses, nil
}
