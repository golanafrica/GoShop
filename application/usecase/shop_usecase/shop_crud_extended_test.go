package shopusecase_test

import (
	"context"
	"fmt"
	"testing"

	shopusecase "Goshop/application/usecase/shop_usecase"
	"Goshop/domain/entity"
	"Goshop/interfaces/utils"
	mock_repository "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.18 : TESTS COMPLÉMENTAIRES - CRUD SHOP
// ============================================================

// ============================================================
// TESTS : CreateShopUsecase - Branches manquantes
// ============================================================

func TestCreateShopUsecase_FindBySlugError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	mockCollabRepo := mock_repository.NewMockShopCollaboratorRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo, mockCollabRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")

	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(nil, fmt.Errorf("database error"))

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "")

	assert.Error(t, err)
	assert.Nil(t, shop)
	assert.Contains(t, err.Error(), "failed to check slug availability")
}

func TestCreateShopUsecase_FindByCustomDomainError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	mockCollabRepo := mock_repository.NewMockShopCollaboratorRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo, mockCollabRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")

	// Slug disponible
	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(nil, nil)

	// Erreur lors de la vérification du custom domain
	mockRepo.EXPECT().
		FindByCustomDomain(ctx, "boutique.com").
		Return(nil, fmt.Errorf("database error"))

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "boutique.com")

	assert.Error(t, err)
	assert.Nil(t, shop)
	assert.Contains(t, err.Error(), "failed to check custom domain availability")
}

func TestCreateShopUsecase_Success_WithCustomDomain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	mockCollabRepo := mock_repository.NewMockShopCollaboratorRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo, mockCollabRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")

	// Slug disponible
	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(nil, nil)

	// Custom domain disponible
	mockRepo.EXPECT().
		FindByCustomDomain(ctx, "boutique.com").
		Return(nil, nil)

	// Création réussie de la boutique
	mockRepo.EXPECT().
		Create(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, shop *entity.Shop) error {
			assert.NotNil(t, shop.CustomDomain)
			assert.Equal(t, "boutique.com", *shop.CustomDomain)
			return nil
		})

	// ✅ AJOUT CRITIQUE : Création réussie du collaborateur propriétaire
	mockCollabRepo.EXPECT().
		Create(ctx, gomock.Any()).
		Return(nil)

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "boutique.com")

	assert.NoError(t, err)
	assert.NotNil(t, shop)
	assert.Equal(t, "Ma Boutique", shop.Name)
	assert.Equal(t, "ma-boutique", shop.Slug)
	assert.NotNil(t, shop.CustomDomain)
	assert.Equal(t, "boutique.com", *shop.CustomDomain)
}

// ============================================================
// TESTS : UpdateShopUsecase - Branches manquantes
// ============================================================

func TestUpdateShopUsecase_FindByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"

	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(nil, fmt.Errorf("database error"))

	newName := "Nouveau Nom"
	result, err := usecase.Execute(ctx, shopID, &newName, nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to find shop")
}

func TestUpdateShopUsecase_CustomDomainAlreadyTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	newDomain := "boutique.com"

	// Shop trouvé et appartient à l'utilisateur
	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	// Custom domain appartient à un autre shop
	otherShop := newTestShop("b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22", "Other Shop", "other-slug", "other-user")
	mockRepo.EXPECT().
		FindByCustomDomain(ctx, "boutique.com").
		Return(otherShop, nil)

	result, err := usecase.Execute(ctx, shopID, nil, &newDomain, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "already taken")
}

func TestUpdateShopUsecase_CustomDomainCheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	newDomain := "boutique.com"

	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	// Erreur lors de la vérification du custom domain
	mockRepo.EXPECT().
		FindByCustomDomain(ctx, "boutique.com").
		Return(nil, fmt.Errorf("database error"))

	result, err := usecase.Execute(ctx, shopID, nil, &newDomain, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to check custom domain")
}

func TestUpdateShopUsecase_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"

	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	mockRepo.EXPECT().
		Update(ctx, gomock.Any()).
		Return(fmt.Errorf("database error"))

	newName := "Nouveau Nom"
	result, err := usecase.Execute(ctx, shopID, &newName, nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update shop")
}

func TestUpdateShopUsecase_ActivateShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	isActive := true

	// Shop désactivé initialement
	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	shop.IsActive = false

	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	mockRepo.EXPECT().
		Update(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, s *entity.Shop) error {
			assert.True(t, s.IsActive)
			return nil
		})

	result, err := usecase.Execute(ctx, shopID, nil, nil, nil, &isActive)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsActive)
}

func TestUpdateShopUsecase_ClearCustomDomain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := utils.WithUserID(context.Background(), "user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	emptyDomain := ""

	// Shop avec custom domain existant
	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	existingDomain := "old-domain.com"
	shop.CustomDomain = &existingDomain

	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	mockRepo.EXPECT().
		Update(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, s *entity.Shop) error {
			assert.Nil(t, s.CustomDomain)
			return nil
		})

	result, err := usecase.Execute(ctx, shopID, nil, &emptyDomain, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.CustomDomain)
}
