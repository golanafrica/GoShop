package collaboratorusecase_test

import (
	"context"
	"errors"
	"strings"
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
// 🆕 v4.4.12 : TESTS UNITAIRES - REMOVE COLLABORATOR USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestPlatformCollab crée un collaborateur plateforme de test
func createTestPlatformCollab(userID string, role entity.PlatformRole) *entity.PlatformCollaborator {
	return &entity.PlatformCollaborator{
		ID:          uuid.New(),
		UserID:      userID,
		Role:        role,
		Permissions: entity.DefaultPlatformPermissions(role),
		InvitedBy:   "inviter-123",
		InvitedAt:   time.Now(),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// createTestShopCollab crée un collaborateur boutique de test
func createTestShopCollab(shopID uuid.UUID, userID string, role entity.ShopRole) *entity.ShopCollaborator {
	return &entity.ShopCollaborator{
		ID:          uuid.New(),
		ShopID:      shopID,
		UserID:      userID,
		Role:        role,
		Permissions: entity.DefaultShopPermissions(role),
		InvitedBy:   "inviter-123",
		InvitedAt:   time.Now(),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// ============================================================
// TESTS : RemoveCollaboratorUsecase - Validation
// ============================================================

func TestRemoveCollaboratorUsecase_EmptyCollaboratorID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: "", // ❌ Vide
		Type:           "platform",
		Reason:         "Reason with enough characters",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "collaborator_id is required")
}

func TestRemoveCollaboratorUsecase_EmptyType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "", // ❌ Vide
		Reason:         "Reason with enough characters",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "type is required")
}

func TestRemoveCollaboratorUsecase_InvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "invalid", // ❌ Invalide
		Reason:         "Reason with enough characters",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "type must be 'platform' or 'shop'")
}

func TestRemoveCollaboratorUsecase_EmptyReason(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "platform",
		Reason:         "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "reason is required")
}

func TestRemoveCollaboratorUsecase_ReasonTooShort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "platform",
		Reason:         "short", // ❌ < 10 caractères
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at least 10 characters")
}

func TestRemoveCollaboratorUsecase_ReasonTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "platform",
		Reason:         strings.Repeat("a", 1001), // ❌ > 1000 caractères
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at most 1000 characters")
}

func TestRemoveCollaboratorUsecase_ShopWithoutShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
		ShopID:         "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

// ============================================================
// TESTS : RemoveCollaboratorUsecase - Platform
// ============================================================

func TestRemoveCollaboratorUsecase_Platform_InsufficientPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
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
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "platform",
		Reason:         "Reason with enough characters",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestRemoveCollaboratorUsecase_Platform_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		Reason:         "Reason with enough characters",
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

func TestRemoveCollaboratorUsecase_Platform_AlreadyDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		Reason:         "Reason with enough characters",
	}

	// Créer un collaborateur déjà supprimé
	now := time.Now()
	deletedBy := "admin-456"
	reason := "Previous reason"
	alreadyDeletedCollab := &entity.PlatformCollaborator{
		ID:             collabID,
		UserID:         "user-123",
		Role:           entity.PlatformRoleTechAdmin,
		IsActive:       false,
		DeletedAt:      &now,
		DeletedBy:      &deletedBy,
		DeletionReason: &reason,
	}

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(alreadyDeletedCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "already deleted")
}

func TestRemoveCollaboratorUsecase_Platform_SelfDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext() // AdminID = "admin-123"
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		Reason:         "Reason with enough characters",
	}

	// Créer un collaborateur avec le même UserID que l'admin
	selfCollab := createTestPlatformCollab(admin.AdminID, entity.PlatformRoleTechAdmin)
	selfCollab.ID = collabID

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(selfCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot delete yourself")
}

func TestRemoveCollaboratorUsecase_Platform_DeactivateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		Reason:         "Reason with enough characters",
	}

	collab := createTestPlatformCollab("user-123", entity.PlatformRoleTechAdmin)
	collab.ID = collabID

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	// Mock : Deactivate échoue
	mockPlatformRepo.EXPECT().
		Deactivate(gomock.Any(), collabID, admin.AdminID, req.Reason).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "deactivate collaborator")
}

func TestRemoveCollaboratorUsecase_Platform_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "platform",
		Reason:         "Reason with enough characters",
	}

	collab := createTestPlatformCollab("user-123", entity.PlatformRoleTechAdmin)
	collab.ID = collabID

	mockPlatformRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(collab, nil)

	mockPlatformRepo.EXPECT().
		Deactivate(gomock.Any(), collabID, admin.AdminID, req.Reason).
		Return(nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "platform", response.Type)
	assert.Equal(t, collabID.String(), response.CollaboratorID)
	assert.Equal(t, "user-123", response.UserID)
	assert.Equal(t, "tech_admin", response.OldRole)
	assert.Equal(t, req.Reason, response.Reason)
	assert.Equal(t, admin.AdminID, response.DeletedBy)
	assert.NotEmpty(t, response.DeletedAt)
}

// ============================================================
// TESTS : RemoveCollaboratorUsecase - Shop
// ============================================================

func TestRemoveCollaboratorUsecase_Shop_InsufficientPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
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
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: uuid.New().String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
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

func TestRemoveCollaboratorUsecase_Shop_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
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

func TestRemoveCollaboratorUsecase_Shop_CollabNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
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

func TestRemoveCollaboratorUsecase_Shop_CollabShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	otherShopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
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

func TestRemoveCollaboratorUsecase_Shop_SelfDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext() // AdminID = "admin-123"
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
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
	assert.Contains(t, err.Error(), "cannot delete yourself")
}

func TestRemoveCollaboratorUsecase_Shop_LastShopAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Le dernier shop_admin
	lastAdmin := createTestShopCollab(shopID, "user-123", entity.ShopRoleShopAdmin)
	lastAdmin.ID = collabID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(lastAdmin, nil)

	// Mock : Un seul shop_admin actif
	mockShopRepo.EXPECT().
		FindByRole(gomock.Any(), shopID, entity.ShopRoleShopAdmin).
		Return([]*entity.ShopCollaborator{lastAdmin}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot delete the last shop_admin")
}

func TestRemoveCollaboratorUsecase_Shop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Collaborateur à supprimer (seller, pas shop_admin)
	sellerCollab := createTestShopCollab(shopID, "user-123", entity.ShopRoleSeller)
	sellerCollab.ID = collabID

	// Un autre shop_admin existe
	otherAdmin := createTestShopCollab(shopID, "admin-456", entity.ShopRoleShopAdmin)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(sellerCollab, nil)

	mockShopRepo.EXPECT().
		Deactivate(gomock.Any(), collabID, admin.AdminID, req.Reason).
		Return(nil)

	_ = otherAdmin // Pour éviter unused variable

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "shop", response.Type)
	assert.Equal(t, collabID.String(), response.CollaboratorID)
	assert.Equal(t, "user-123", response.UserID)
	assert.Equal(t, "seller", response.OldRole)
	assert.Equal(t, req.Reason, response.Reason)
	assert.Equal(t, admin.AdminID, response.DeletedBy)
	assert.NotEmpty(t, response.DeletedAt)
}

// ============================================================
// TESTS : RemoveCollaboratorUsecase - removeShopCollaborator (branches manquantes)
// ============================================================

func TestRemoveCollaboratorUsecase_Shop_AlreadyDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	// Collaborateur déjà supprimé
	now := time.Now()
	deletedBy := "admin-456"
	reason := "Previous reason"
	alreadyDeletedCollab := &entity.ShopCollaborator{
		ID:             collabID,
		ShopID:         shopID,
		UserID:         "user-123",
		Role:           entity.ShopRoleSeller,
		IsActive:       false,
		DeletedAt:      &now,
		DeletedBy:      &deletedBy,
		DeletionReason: &reason,
	}

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(alreadyDeletedCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "already deleted")
}

func TestRemoveCollaboratorUsecase_Shop_DeactivateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)

	uc := collaboratorusecase.NewRemoveCollaboratorUsecase(
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
	)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	collabID := uuid.New()
	req := &collaboratorusecase.RemoveCollaboratorRequest{
		CollaboratorID: collabID.String(),
		Type:           "shop",
		Reason:         "Reason with enough characters",
		ShopID:         shopID.String(),
	}

	testShop := createTestShop("Test Shop")
	testShop.ID = shopID

	sellerCollab := createTestShopCollab(shopID, "user-123", entity.ShopRoleSeller)
	sellerCollab.ID = collabID

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), collabID).
		Return(sellerCollab, nil)

	// Mock : Deactivate échoue
	mockShopRepo.EXPECT().
		Deactivate(gomock.Any(), collabID, admin.AdminID, req.Reason).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "deactivate collaborator")
}
