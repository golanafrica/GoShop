package adminshopusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adminshopusecase "Goshop/application/usecase/admin_shop_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.11 : TESTS UNITAIRES - LIST SHOPS USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestAdminContext crée un contexte admin pour les tests
func createTestAdminContext() *adminshopusecase.AdminContext {
	return &adminshopusecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@goshop.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		RequestID:  "req-abc",
	}
}

// createTestShop crée un shop de test
func createTestShop(name string, plan entity.ShopPlan, isActive bool, kycStatus entity.ShopKYCStatus) *entity.Shop {
	return &entity.Shop{
		ID:          uuid.New(),
		Name:        name,
		Slug:        name,
		OwnerID:     "owner-123",
		Plan:        plan,
		IsActive:    isActive,
		KYCStatus:   kycStatus,
		HealthScore: 1000,
		HealthLevel: entity.ShopHealthExcellent,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now(),
	}
}

// ============================================================
// TESTS : ListShopsUsecase - Repository errors
// ============================================================

func TestListShopsUsecase_FindAllError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  10,
		Offset: 0,
	}

	// Mock : FindAllShopsAdmin échoue
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		Return(nil, 0, errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find all shops")
}

func TestListShopsUsecase_GetHealthStatsError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  10,
		Offset: 0,
	}

	shop := createTestShop("Test Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)

	// Mock : FindAllShopsAdmin réussit
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		Return([]*entity.Shop{shop}, 1, nil)

	// Mock : GetHealthStats échoue (non bloquant)
	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(nil, errors.New("stats error"))

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante, la réponse est quand même retournée
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Shops, 1)
	assert.Nil(t, response.Stats)
}

// ============================================================
// TESTS : ListShopsUsecase - Happy paths
// ============================================================

func TestListShopsUsecase_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  10,
		Offset: 0,
	}

	// Mock : Liste vide
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		Return([]*entity.Shop{}, 0, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   0,
			AverageScore: 0,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Empty(t, response.Shops)
	assert.Equal(t, 0, response.Total)
	assert.Equal(t, 10, response.Limit)
	assert.Equal(t, 0, response.Offset)
}

func TestListShopsUsecase_SuccessWithMultipleShops(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  10,
		Offset: 0,
	}

	// Créer 3 shops avec différents états
	shop1 := createTestShop("Shop 1", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	shop2 := createTestShop("Shop 2", entity.ShopPlanPro, true, entity.ShopKYCStatusPending)
	shop3 := createTestShop("Shop 3", entity.ShopPlanBusiness, false, entity.ShopKYCStatusRejected)

	shops := []*entity.Shop{shop1, shop2, shop3}

	// Mock : Liste avec 3 shops
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		Return(shops, 3, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   3,
			AverageScore: 800,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Shops, 3)
	assert.Equal(t, 3, response.Total)

	// Vérifier le mapping DTO
	assert.Equal(t, shop1.ID.String(), response.Shops[0].ID)
	assert.Equal(t, "Shop 1", response.Shops[0].Name)
	assert.Equal(t, "free", response.Shops[0].Plan)
	assert.True(t, response.Shops[0].IsActive)
	assert.Equal(t, "verified", response.Shops[0].KYCStatus)

	assert.Equal(t, "Shop 2", response.Shops[1].Name)
	assert.Equal(t, "pro", response.Shops[1].Plan)
	assert.Equal(t, "pending", response.Shops[1].KYCStatus)

	assert.Equal(t, "Shop 3", response.Shops[2].Name)
	assert.Equal(t, "business", response.Shops[2].Plan)
	assert.False(t, response.Shops[2].IsActive)
	assert.Equal(t, "rejected", response.Shops[2].KYCStatus)
}

// ============================================================
// TESTS : ListShopsUsecase - Pagination auto-correction
// ============================================================

func TestListShopsUsecase_InvalidLimit_AutoCorrected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  0, // ❌ Invalide (doit être > 0)
		Offset: 0,
	}

	// Mock : Limit corrigé à 20 par défaut
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, filters *repository.ShopAdminFilters) ([]*entity.Shop, int, error) {
			// ✅ Vérifier que le limit a été corrigé
			assert.Equal(t, 20, filters.Limit)
			return []*entity.Shop{}, 0, nil
		})

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 20, response.Limit) // ✅ Limit corrigé
}

func TestListShopsUsecase_LimitTooHigh_AutoCorrected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  500, // ❌ Trop élevé (max 100)
		Offset: 0,
	}

	// Mock : Limit corrigé à 20 par défaut
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, filters *repository.ShopAdminFilters) ([]*entity.Shop, int, error) {
			assert.Equal(t, 20, filters.Limit)
			return []*entity.Shop{}, 0, nil
		})

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 20, response.Limit)
}

func TestListShopsUsecase_NegativeOffset_AutoCorrected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:  10,
		Offset: -5, // ❌ Négatif
	}

	// Mock : Offset corrigé à 0
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, filters *repository.ShopAdminFilters) ([]*entity.Shop, int, error) {
			assert.Equal(t, 0, filters.Offset)
			return []*entity.Shop{}, 0, nil
		})

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.Offset) // ✅ Offset corrigé
}

// ============================================================
// TESTS : ListShopsUsecase - Sort defaults
// ============================================================

func TestListShopsUsecase_DefaultSort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewListShopsUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ListShopsRequest{
		Limit:     10,
		Offset:    0,
		SortBy:    "", // ❌ Vide
		SortOrder: "", // ❌ Vide
	}

	// Mock : Sort corrigé aux valeurs par défaut
	mockShopRepo.EXPECT().
		FindAllShopsAdmin(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, filters *repository.ShopAdminFilters) ([]*entity.Shop, int, error) {
			assert.Equal(t, "created_at", filters.SortBy)
			assert.Equal(t, "desc", filters.SortOrder)
			return []*entity.Shop{}, 0, nil
		})

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
}
