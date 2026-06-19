package shopusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// CreateShopUsecase crée une nouvelle boutique
type CreateShopUsecase struct {
	shopRepo repository.ShopRepository
}

func NewCreateShopUsecase(shopRepo repository.ShopRepository) *CreateShopUsecase {
	return &CreateShopUsecase{shopRepo: shopRepo}
}

func (uc *CreateShopUsecase) Execute(ctx context.Context, name, slug, customDomain string) (*entity.Shop, error) {
	logger := zerolog.Ctx(ctx)

	// ✅ Récupérer l'user_id via utils (bonne clé de contexte)
	userID, ok := utils.GetUserID(ctx)
	if !ok || userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	logger.Info().
		Str("operation", "create_shop").
		Str("user_id", userID).
		Str("shop_name", name).
		Str("shop_slug", slug).
		Msg("Starting shop creation")

	start := time.Now()

	// Vérifier que le slug n'existe pas déjà
	existingShop, err := uc.shopRepo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to check slug availability: %w", err)
	}
	if existingShop != nil {
		return nil, fmt.Errorf("slug '%s' is already taken", slug)
	}

	// Vérifier que le custom_domain n'existe pas déjà
	if customDomain != "" {
		existingShop, err = uc.shopRepo.FindByCustomDomain(ctx, customDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to check custom domain availability: %w", err)
		}
		if existingShop != nil {
			return nil, fmt.Errorf("custom domain '%s' is already taken", customDomain)
		}
	}

	// Créer la boutique
	shop, err := entity.NewShop(name, slug, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create shop entity: %w", err)
	}

	if customDomain != "" {
		if err := shop.SetCustomDomain(customDomain); err != nil {
			return nil, fmt.Errorf("failed to set custom domain: %w", err)
		}
	}

	if err := uc.shopRepo.Create(ctx, shop); err != nil {
		return nil, fmt.Errorf("failed to save shop: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("shop_slug", shop.Slug).
		Dur("duration_ms", time.Since(start)).
		Msg("Shop created successfully")

	return shop, nil
}

// ListShopsUsecase liste les boutiques d'un utilisateur
type ListShopsUsecase struct {
	shopRepo repository.ShopRepository
}

func NewListShopsUsecase(shopRepo repository.ShopRepository) *ListShopsUsecase {
	return &ListShopsUsecase{shopRepo: shopRepo}
}

func (uc *ListShopsUsecase) Execute(ctx context.Context) ([]*entity.Shop, error) {
	logger := zerolog.Ctx(ctx)

	// ✅ Récupérer l'user_id via utils (bonne clé de contexte)
	userID, ok := utils.GetUserID(ctx)
	if !ok || userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	shops, err := uc.shopRepo.FindByOwnerID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shops: %w", err)
	}

	logger.Debug().Int("shops_count", len(shops)).Msg("Shops listed successfully")
	return shops, nil
}

// UpdateShopUsecase met à jour une boutique
type UpdateShopUsecase struct {
	shopRepo repository.ShopRepository
}

func NewUpdateShopUsecase(shopRepo repository.ShopRepository) *UpdateShopUsecase {
	return &UpdateShopUsecase{shopRepo: shopRepo}
}

func (uc *UpdateShopUsecase) Execute(
	ctx context.Context,
	shopID string,
	name *string,
	customDomain *string,
	plan *string,
	isActive *bool,
) (*entity.Shop, error) {
	logger := zerolog.Ctx(ctx)

	// ✅ Récupérer l'user_id via utils (bonne clé de contexte)
	userID, ok := utils.GetUserID(ctx)
	if !ok || userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	id, err := uuid.Parse(shopID)
	if err != nil {
		return nil, fmt.Errorf("invalid shop ID")
	}

	shop, err := uc.shopRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find shop: %w", err)
	}
	if shop == nil {
		return nil, fmt.Errorf("shop not found")
	}

	// Vérifier que l'utilisateur est le propriétaire
	if shop.OwnerID != userID {
		return nil, fmt.Errorf("you are not the owner of this shop")
	}

	// Appliquer les modifications
	if name != nil {
		shop.Name = *name
	}

	if customDomain != nil {
		if *customDomain != "" {
			existingShop, err := uc.shopRepo.FindByCustomDomain(ctx, *customDomain)
			if err != nil {
				return nil, fmt.Errorf("failed to check custom domain: %w", err)
			}
			if existingShop != nil && existingShop.ID != shop.ID {
				return nil, fmt.Errorf("custom domain '%s' is already taken", *customDomain)
			}
			shop.CustomDomain = customDomain
		} else {
			shop.CustomDomain = nil
		}
	}

	if plan != nil {
		shop.Plan = entity.ShopPlan(*plan)
	}

	if isActive != nil {
		if *isActive {
			shop.Activate()
		} else {
			shop.Deactivate()
		}
	}

	if err := uc.shopRepo.Update(ctx, shop); err != nil {
		return nil, fmt.Errorf("failed to update shop: %w", err)
	}

	logger.Info().Str("shop_id", shop.ID.String()).Msg("Shop updated successfully")
	return shop, nil
}
