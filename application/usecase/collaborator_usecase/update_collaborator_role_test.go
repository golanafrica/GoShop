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
// 🆕 v4.4.12 : TESTS UNITAIRES - UPDATE COLLABORATOR ROLE USECASE
// ============================================================

// ============================================================
// TESTS : UpdateCollaboratorRoleUsecase - Validation
// ============================================================

func TestUpdateCollaboratorRoleUsecase_EmptyCollaboratorID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: "", // ❌ Vide
		Type:           "platform",
		NewRole:        "tech_admin",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "collaborator_id is required")
}

func TestUpdateCollaboratorRoleUsecase_EmptyType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "", // ❌ Vide
		NewRole:        "tech_admin",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "type is required")
}

func TestUpdateCollaboratorRoleUsecase_InvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "invalid", // ❌ Invalide
		NewRole:        "tech_admin",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "type must be 'platform' or 'shop'")
}

func TestUpdateCollaboratorRoleUsecase_EmptyNewRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "platform",
		NewRole:        "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "new_role is required")
}

func TestUpdateCollaboratorRoleUsecase_ShopWithoutShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

// ============================================================
// TESTS : UpdateCollaboratorRoleUsecase - Platform
// ============================================================

func TestUpdateCollaboratorRoleUsecase_Platform_InsufficientPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
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
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "platform",
		NewRole:        "tech_admin",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestUpdateCollaboratorRoleUsecase_Platform_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		NewRole:        "tech_admin",
	}

	// Mock : Collaborateur non trouvé
	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrCollaboratorNotFound, err)
}

func TestUpdateCollaboratorRoleUsecase_Platform_SelfChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext() // AdminID = "admin-123"
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		NewRole:        "tech_admin",
	}

	// Collaborateur avec le même UserID que l'admin
	selfCollab := createTestPlatformCollab(admin.AdminID, entity.PlatformRoleFinanceManager)
	selfCollab.ID = collabID

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(selfCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot change your own role")
}

func TestUpdateCollaboratorRoleUsecase_Platform_InvalidNewRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		NewRole:        "invalid_role", // ❌ Invalide
	}

	collab := createTestPlatformCollab("user-123", entity.PlatformRoleFinanceManager)
	collab.ID = collabID

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvalidPlatformRole, err)
}

func TestUpdateCollaboratorRoleUsecase_Platform_UpdateRoleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		NewRole:        "tech_admin",
	}

	collab := createTestPlatformCollab("user-123", entity.PlatformRoleFinanceManager)
	collab.ID = collabID

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	// Mock : UpdateRole échoue
	mockPlatformRepo.EXPECT().
		UpdateRole(gomock.Any(), collabID, entity.PlatformRoleTechAdmin, gomock.Any()).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update role")
}

func TestUpdateCollaboratorRoleUsecase_Platform_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		NewRole:        "tech_admin",
	}

	collab := createTestPlatformCollab("user-123", entity.PlatformRoleFinanceManager)
	collab.ID = collabID

	// Collaborateur mis à jour
	updatedCollab := createTestPlatformCollab("user-123", entity.PlatformRoleTechAdmin)
	updatedCollab.ID = collabID
	updatedCollab.UpdatedAt = time.Now()

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	mockPlatformRepo.EXPECT().
		UpdateRole(gomock.Any(), collabID, entity.PlatformRoleTechAdmin, gomock.Any()).
		Return(nil)

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(updatedCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "platform", response.Type)
	assert.Equal(t, collabID.String(), response.CollaboratorID)
	assert.Equal(t, "finance_manager", response.OldRole)
	assert.Equal(t, "tech_admin", response.NewRole)
	assert.NotEmpty(t, response.UpdatedAt)
}

// ============================================================
// TESTS : UpdateCollaboratorRoleUsecase - Shop
// ============================================================

func TestUpdateCollaboratorRoleUsecase_Shop_InsufficientPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	shopID := uuid.New()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "merchant-123",
		AdminEmail: "merchant@goshop.com",
		AdminRole:  "merchant", // ❌ Pas super_admin
	}
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
	}

	// Mock : Merchant n'est pas shop_admin
	mockShopRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(false, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestUpdateCollaboratorRoleUsecase_Shop_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
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

func TestUpdateCollaboratorRoleUsecase_Shop_CollabNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Mock : Collaborateur non trouvé
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrCollaboratorNotFound, err)
}

func TestUpdateCollaboratorRoleUsecase_Shop_CollabShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	otherShopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Collaborateur d'une AUTRE boutique
	wrongShopCollab := createTestShopCollab(otherShopID, "user-123", entity.ShopRoleSeller)
	wrongShopCollab.ID = collabID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(wrongShopCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "does not belong to this shop")
}

func TestUpdateCollaboratorRoleUsecase_Shop_SelfChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext() // AdminID = "admin-123"
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Collaborateur avec le même UserID que l'admin
	selfCollab := createTestShopCollab(shopID, admin.AdminID, entity.ShopRoleShopAdmin)
	selfCollab.ID = collabID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(selfCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot change your own role")
}

func TestUpdateCollaboratorRoleUsecase_Shop_InvalidNewRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "invalid_role", // ❌ Invalide
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	collab := createTestShopCollab(shopID, "user-123", entity.ShopRoleSeller)
	collab.ID = collabID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvalidShopRole, err)
}

func TestUpdateCollaboratorRoleUsecase_Shop_UpdateRoleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	collab := createTestShopCollab(shopID, "user-123", entity.ShopRoleShopAdmin)
	collab.ID = collabID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	// Mock : UpdateRole échoue
	mockShopRepo.EXPECT().
		UpdateRole(gomock.Any(), collabID, entity.ShopRoleSeller, gomock.Any()).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update role")
}

func TestUpdateCollaboratorRoleUsecase_Shop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewUpdateCollaboratorRoleUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.UpdateCollaboratorRoleRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		NewRole:        "seller",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	collab := createTestShopCollab(shopID, "user-123", entity.ShopRoleShopAdmin)
	collab.ID = collabID

	// Collaborateur mis à jour
	updatedCollab := createTestShopCollab(shopID, "user-123", entity.ShopRoleSeller)
	updatedCollab.ID = collabID
	updatedCollab.UpdatedAt = time.Now()

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	mockShopRepo.EXPECT().
		UpdateRole(gomock.Any(), collabID, entity.ShopRoleSeller, gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(updatedCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "shop", response.Type)
	assert.Equal(t, collabID.String(), response.CollaboratorID)
	assert.Equal(t, "shop_admin", response.OldRole)
	assert.Equal(t, "seller", response.NewRole)
	assert.NotEmpty(t, response.UpdatedAt)
}
