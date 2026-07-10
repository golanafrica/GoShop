package collaboratorusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	collaboratorusecase "Goshop/application/usecase/collaborator_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.12 : TESTS UNITAIRES - LIST COLLABORATORS USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestAdminContext crée un contexte admin pour les tests
func createTestAdminContext() *collaboratorusecase.AdminContext {
	return &collaboratorusecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@goshop.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		RequestID:  "req-abc",
	}
}

// createTestPlatformCollaborator crée un collaborateur plateforme de test
func createTestPlatformCollaborator(userID string, role entity.PlatformRole) *entity.PlatformCollaborator {
	collab, _ := entity.NewPlatformCollaborator(
		userID,
		role,
		entity.DefaultPlatformPermissions(role),
		"inviter-123",
	)
	return collab
}

// createTestShopCollaborator crée un collaborateur boutique de test
func createTestShopCollaborator(shopID uuid.UUID, userID string, role entity.ShopRole) *entity.ShopCollaborator {
	collab, _ := entity.NewShopCollaborator(
		shopID,
		userID,
		role,
		entity.DefaultShopPermissions(role),
		"inviter-123",
	)
	return collab
}

// createTestShop crée une boutique de test
func createTestShop(name string) *entity.Shop {
	return &entity.Shop{
		ID:        uuid.New(),
		Name:      name,
		Slug:      "test-shop",
		OwnerID:   "owner-123",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - Validation
// ============================================================

func TestListCollaboratorsUsecase_EmptyType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "type is required")
}

func TestListCollaboratorsUsecase_InvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "invalid", // ❌ Invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "type must be 'platform' or 'shop'")
}

func TestListCollaboratorsUsecase_ShopWithoutShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestListCollaboratorsUsecase_InvalidSortOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:      "platform",
		SortOrder: "invalid", // ❌ Invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "sort_order must be 'asc' or 'desc'")
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - Permissions
// ============================================================

func TestListCollaboratorsUsecase_InsufficientPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "merchant-123",
		AdminEmail: "merchant@goshop.com",
		AdminRole:  "merchant", // ❌ Pas super_admin ou admin
	}
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform", // ❌ Merchant ne peut pas lister platform
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestListCollaboratorsUsecase_MerchantNotShopAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	shopID := uuid.New()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "merchant-123",
		AdminEmail: "merchant@goshop.com",
		AdminRole:  "merchant",
	}
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: shopID.String(),
	}

	// Mock : Merchant n'est pas shop_admin
	mockShopRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(false, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not shop_admin")
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - Platform Happy paths
// ============================================================

func TestListCollaboratorsUsecase_Platform_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
	}

	// Mock : Liste vide
	mockPlatformRepo.EXPECT().
		FindAll(gomock.Any(), 20, 0).
		Return([]*entity.PlatformCollaborator{}, 0, nil)

	mockPlatformRepo.EXPECT().
		CountByRole(gomock.Any()).
		Return(map[entity.PlatformRole]int{}, nil)

	mockPlatformRepo.EXPECT().
		CountActive(gomock.Any()).
		Return(0, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "platform", response.Type)
	assert.Empty(t, response.Collaborators)
	assert.Equal(t, 0, response.Total)
}

func TestListCollaboratorsUsecase_Platform_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
	}

	// Créer 2 collaborateurs
	collab1 := createTestPlatformCollaborator("user-1", entity.PlatformRoleFinanceManager)
	collab2 := createTestPlatformCollaborator("user-2", entity.PlatformRoleTechAdmin)

	collaborators := []*entity.PlatformCollaborator{collab1, collab2}

	// Mock : Liste avec 2 collaborateurs
	mockPlatformRepo.EXPECT().
		FindAll(gomock.Any(), 20, 0).
		Return(collaborators, 2, nil)

	mockPlatformRepo.EXPECT().
		CountByRole(gomock.Any()).
		Return(map[entity.PlatformRole]int{
			entity.PlatformRoleFinanceManager: 1,
			entity.PlatformRoleTechAdmin:      1,
		}, nil)

	mockPlatformRepo.EXPECT().
		CountActive(gomock.Any()).
		Return(2, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "platform", response.Type)
	assert.Len(t, response.Collaborators, 2)
	assert.Equal(t, 2, response.Total)
	assert.NotNil(t, response.Stats)
	assert.Equal(t, 2, response.Stats.TotalActive)
}

func TestListCollaboratorsUsecase_Platform_WithRoleFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
		Role: "tech_admin", // ✅ Filtre par rôle
	}

	collab := createTestPlatformCollaborator("user-1", entity.PlatformRoleTechAdmin)

	// Mock : FindByRole au lieu de FindAll
	mockPlatformRepo.EXPECT().
		FindByRole(gomock.Any(), entity.PlatformRoleTechAdmin, 20, 0).
		Return([]*entity.PlatformCollaborator{collab}, 1, nil)

	mockPlatformRepo.EXPECT().
		CountByRole(gomock.Any()).
		Return(map[entity.PlatformRole]int{
			entity.PlatformRoleTechAdmin: 1,
		}, nil)

	mockPlatformRepo.EXPECT().
		CountActive(gomock.Any()).
		Return(1, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Collaborators, 1)
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - Shop Happy paths
// ============================================================

func TestListCollaboratorsUsecase_Shop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	shopID := uuid.New()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	collab := createTestShopCollaborator(shopID, "user-1", entity.ShopRoleShopAdmin)

	// Mock : Shop existe
	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Mock : Liste avec 1 collaborateur
	mockShopRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID, 20, 0).
		Return([]*entity.ShopCollaborator{collab}, 1, nil)

	mockShopRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(1, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "shop", response.Type)
	assert.Len(t, response.Collaborators, 1)
	assert.Equal(t, shopID.String(), *response.Collaborators[0].ShopID)
	assert.Equal(t, "Test Shop", *response.Collaborators[0].ShopName)
}

func TestListCollaboratorsUsecase_Shop_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	shopID := uuid.New()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: shopID.String(),
	}

	// Mock : Shop non trouvée
	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop not found")
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - listShopCollaborators (branches manquantes)
// ============================================================

func TestListCollaboratorsUsecase_Shop_WithRoleFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: shopID.String(),
		Role:   "seller", // ✅ Filtre par rôle
		Limit:  20,
		Offset: 0,
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Créer 2 collaborateurs avec rôle seller
	collab1 := createTestShopCollab(shopID, "user-1", entity.ShopRoleSeller)
	collab2 := createTestShopCollab(shopID, "user-2", entity.ShopRoleSeller)
	collaborators := []*entity.ShopCollaborator{collab1, collab2}

	// Mock : Shop existe
	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Mock : FindByRole (pas FindByShopID)
	mockShopRepo.EXPECT().
		FindByRole(gomock.Any(), shopID, entity.ShopRoleSeller).
		Return(collaborators, nil)

	// Mock : CountByShopID
	mockShopRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(2, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "shop", response.Type)
	assert.Len(t, response.Collaborators, 2)
	assert.Equal(t, 2, response.Total)
	assert.Equal(t, "seller", response.Collaborators[0].Role)
}

func TestListCollaboratorsUsecase_Shop_OffsetGreaterThanTotal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: shopID.String(),
		Role:   "seller",
		Limit:  20,
		Offset: 100, // ❌ Offset > total (seulement 2 collaborateurs)
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Seulement 2 collaborateurs
	collab1 := createTestShopCollab(shopID, "user-1", entity.ShopRoleSeller)
	collab2 := createTestShopCollab(shopID, "user-2", entity.ShopRoleSeller)
	collaborators := []*entity.ShopCollaborator{collab1, collab2}

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByRole(gomock.Any(), shopID, entity.ShopRoleSeller).
		Return(collaborators, nil)

	mockShopRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(2, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "shop", response.Type)
	assert.Empty(t, response.Collaborators) // ✅ Liste vide car offset > total
	assert.Equal(t, 2, response.Total)      // Total reste 2
}

func TestListCollaboratorsUsecase_Shop_CountByShopIDError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type:   "shop",
		ShopID: shopID.String(),
		Limit:  20,
		Offset: 0,
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	collab := createTestShopCollab(shopID, "user-1", entity.ShopRoleSeller)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID, 20, 0).
		Return([]*entity.ShopCollaborator{collab}, 1, nil)

	// Mock : CountByShopID échoue (non bloquant)
	mockShopRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(0, errors.New("count error"))

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante, la réponse est quand même retournée
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Collaborators, 1)
	assert.NotNil(t, response.Stats)
	assert.Equal(t, 0, response.Stats.TotalActive) // ✅ 0 car erreur
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - listPlatformCollaborators (branches manquantes)
// ============================================================

func TestListCollaboratorsUsecase_Platform_CountByRoleError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
	}

	collab := createTestPlatformCollab("user-1", entity.PlatformRoleTechAdmin)

	mockPlatformRepo.EXPECT().
		FindAll(gomock.Any(), 20, 0).
		Return([]*entity.PlatformCollaborator{collab}, 1, nil)

	// Mock : CountByRole échoue (non bloquant)
	mockPlatformRepo.EXPECT().
		CountByRole(gomock.Any()).
		Return(nil, errors.New("count error"))

	mockPlatformRepo.EXPECT().
		CountActive(gomock.Any()).
		Return(1, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Collaborators, 1)
	assert.NotNil(t, response.Stats)
	assert.Empty(t, response.Stats.ByRole) // ✅ Map vide car erreur
}

func TestListCollaboratorsUsecase_Platform_CountActiveError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
	}

	collab := createTestPlatformCollab("user-1", entity.PlatformRoleTechAdmin)

	mockPlatformRepo.EXPECT().
		FindAll(gomock.Any(), 20, 0).
		Return([]*entity.PlatformCollaborator{collab}, 1, nil)

	mockPlatformRepo.EXPECT().
		CountByRole(gomock.Any()).
		Return(map[entity.PlatformRole]int{entity.PlatformRoleTechAdmin: 1}, nil)

	// Mock : CountActive échoue (non bloquant)
	mockPlatformRepo.EXPECT().
		CountActive(gomock.Any()).
		Return(0, errors.New("count error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Stats)
	assert.Equal(t, 0, response.Stats.TotalActive) // ✅ 0 car erreur
}

// ============================================================
// TESTS : ListCollaboratorsUsecase - checkPermissions (tous rôles)
// ============================================================

func TestListCollaboratorsUsecase_AdminCanListPlatform(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "admin-456",
		AdminEmail: "admin@goshop.com",
		AdminRole:  "admin", // ✅ Admin peut lister platform
	}
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
	}

	mockPlatformRepo.EXPECT().
		FindAll(gomock.Any(), 20, 0).
		Return([]*entity.PlatformCollaborator{}, 0, nil)

	mockPlatformRepo.EXPECT().
		CountByRole(gomock.Any()).
		Return(map[entity.PlatformRole]int{}, nil)

	mockPlatformRepo.EXPECT().
		CountActive(gomock.Any()).
		Return(0, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "platform", response.Type)
}

func TestListCollaboratorsUsecase_MerchantCannotListPlatform(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewListCollaboratorsUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "merchant-123",
		AdminEmail: "merchant@goshop.com",
		AdminRole:  "merchant", // ❌ Merchant ne peut pas lister platform
	}
	req := &collaboratorusecase.ListCollaboratorsRequest{
		Type: "platform",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}
