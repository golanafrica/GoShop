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

// ============ HELPERS ============

// contextWithUser crée un contexte avec un user_id (comme le middleware Auth)
func contextWithUser(userID string) context.Context {
	return utils.WithUserID(context.Background(), userID)
}

// newTestShop crée un shop de test
func newTestShop(id, name, slug, ownerID string) *entity.Shop {
	return &entity.Shop{
		ID:       uuid.MustParse(id),
		Name:     name,
		Slug:     slug,
		OwnerID:  ownerID,
		Plan:     entity.ShopPlanFree,
		IsActive: true,
	}
}

// ============ TESTS CREATE SHOP ============

func TestCreateShopUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	// Le slug n'existe pas
	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(nil, nil)

	// Création réussie
	mockRepo.EXPECT().
		Create(ctx, gomock.Any()).
		Return(nil)

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "")

	assert.NoError(t, err)
	assert.NotNil(t, shop)
	assert.Equal(t, "Ma Boutique", shop.Name)
	assert.Equal(t, "ma-boutique", shop.Slug)
	assert.Equal(t, "user-123", shop.OwnerID)
}

func TestCreateShopUsecase_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo)

	ctx := context.Background() // Pas de user_id

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "")

	assert.Error(t, err)
	assert.Nil(t, shop)
	assert.Contains(t, err.Error(), "user not authenticated")
}

func TestCreateShopUsecase_SlugAlreadyTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	existingShop := newTestShop("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "Existing", "ma-boutique", "other-user")
	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(existingShop, nil)

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "")

	assert.Error(t, err)
	assert.Nil(t, shop)
	assert.Contains(t, err.Error(), "already taken")
}

func TestCreateShopUsecase_CustomDomainAlreadyTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(nil, nil)

	existingShop := newTestShop("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "Existing", "other-slug", "other-user")
	mockRepo.EXPECT().
		FindByCustomDomain(ctx, "boutique.com").
		Return(existingShop, nil)

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "boutique.com")

	assert.Error(t, err)
	assert.Nil(t, shop)
	assert.Contains(t, err.Error(), "already taken")
}

func TestCreateShopUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewCreateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	mockRepo.EXPECT().
		FindBySlug(ctx, "ma-boutique").
		Return(nil, nil)

	mockRepo.EXPECT().
		Create(ctx, gomock.Any()).
		Return(fmt.Errorf("database error"))

	shop, err := usecase.Execute(ctx, "Ma Boutique", "ma-boutique", "")

	assert.Error(t, err)
	assert.Nil(t, shop)
	assert.Contains(t, err.Error(), "failed to save shop")
}

// ============ TESTS LIST SHOPS ============

func TestListShopsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewListShopsUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	shops := []*entity.Shop{
		newTestShop("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "Shop 1", "shop-1", "user-123"),
		newTestShop("b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22", "Shop 2", "shop-2", "user-123"),
	}
	mockRepo.EXPECT().
		FindByOwnerID(ctx, "user-123").
		Return(shops, nil)

	result, err := usecase.Execute(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Shop 1", result[0].Name)
	assert.Equal(t, "Shop 2", result[1].Name)
}

func TestListShopsUsecase_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewListShopsUsecase(mockRepo)

	ctx := context.Background()

	result, err := usecase.Execute(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user not authenticated")
}

func TestListShopsUsecase_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewListShopsUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	mockRepo.EXPECT().
		FindByOwnerID(ctx, "user-123").
		Return([]*entity.Shop{}, nil)

	result, err := usecase.Execute(ctx)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestListShopsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewListShopsUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	mockRepo.EXPECT().
		FindByOwnerID(ctx, "user-123").
		Return(nil, fmt.Errorf("database error"))

	result, err := usecase.Execute(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// ============ TESTS UPDATE SHOP ============

func TestUpdateShopUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	newName := "Nouveau Nom"

	shop := newTestShop(shopID, "Ancien Nom", "shop-1", "user-123")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	mockRepo.EXPECT().
		Update(ctx, gomock.Any()).
		Return(nil)

	result, err := usecase.Execute(ctx, shopID, &newName, nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Nouveau Nom", result.Name)
}

func TestUpdateShopUsecase_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := context.Background()
	newName := "Nouveau Nom"

	result, err := usecase.Execute(ctx, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", &newName, nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user not authenticated")
}

func TestUpdateShopUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"

	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(nil, nil)

	newName := "Nouveau Nom"
	result, err := usecase.Execute(ctx, shopID, &newName, nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "shop not found")
}

func TestUpdateShopUsecase_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"

	// Le shop appartient à un autre utilisateur
	shop := newTestShop(shopID, "Shop Name", "shop-1", "other-user")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	newName := "Nouveau Nom"
	result, err := usecase.Execute(ctx, shopID, &newName, nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not the owner")
}

func TestUpdateShopUsecase_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")

	newName := "Nouveau Nom"
	result, err := usecase.Execute(ctx, "invalid-uuid", &newName, nil, nil, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid shop ID")
}

func TestUpdateShopUsecase_ChangePlan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	newPlan := "pro"

	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	mockRepo.EXPECT().
		Update(ctx, gomock.Any()).
		Return(nil)

	result, err := usecase.Execute(ctx, shopID, nil, nil, &newPlan, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, entity.ShopPlan("pro"), result.Plan)
}

func TestUpdateShopUsecase_DeactivateShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repository.NewMockShopRepository(ctrl)
	usecase := shopusecase.NewUpdateShopUsecase(mockRepo)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	isActive := false

	shop := newTestShop(shopID, "Shop Name", "shop-1", "user-123")
	mockRepo.EXPECT().
		FindByID(ctx, uuid.MustParse(shopID)).
		Return(shop, nil)

	mockRepo.EXPECT().
		Update(ctx, gomock.Any()).
		Return(nil)

	result, err := usecase.Execute(ctx, shopID, nil, nil, nil, &isActive)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsActive)
}
