package productuscase

import (
	"context"

	productdto "Goshop/application/dto/product_dto"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type ListPublicProductsUsecase struct {
	repo repository.ProductRepository
}

func NewListPublicProductsUsecase(repo repository.ProductRepository) *ListPublicProductsUsecase {
	return &ListPublicProductsUsecase{
		repo: repo,
	}
}

func (uc *ListPublicProductsUsecase) Execute(ctx context.Context, req *productdto.ListProductsRequest) ([]*productdto.PublicProductResponse, error) {
	logger := zerolog.Ctx(ctx)

	// Mapping DTO -> Domain Filter (pas de filtre de prix pour le public par défaut, mais on le supporte)
	filter := repository.ProductFilter{
		Search: req.Search,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	logger.Debug().
		Str("search", filter.Search).
		Int("limit", filter.Limit).
		Int("offset", filter.Offset).
		Msg("Executing list public products use case")

	publicProducts, err := uc.repo.FindPublicProducts(ctx, filter)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to retrieve public products from repository")
		return nil, err
	}

	if len(publicProducts) == 0 {
		return []*productdto.PublicProductResponse{}, nil
	}

	responses := make([]*productdto.PublicProductResponse, 0, len(publicProducts))
	for _, p := range publicProducts {
		responses = append(responses, &productdto.PublicProductResponse{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			PriceCents:  p.PriceCents,
			Stock:       p.Stock,
			ShopID:      p.ShopID,
			ShopName:    p.ShopName,
			ShopSlug:    p.ShopSlug,
			CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   p.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	logger.Info().Int("count", len(responses)).Msg("Public products listed successfully")
	return responses, nil
}
