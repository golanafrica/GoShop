package adminshopusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adminshopusecase "Goshop/application/usecase/admin_shop_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.11 : TESTS UNITAIRES - ACTIVATE SHOP USECASE
// ============================================================

// ============================================================
// TESTS : ActivateShopUsecase - Validation
// ============================================================

func TestActivateShopUsecase_EmptyShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestActivateShopUsecase_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: "invalid-uuid", // ❌ UUID invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

// ============================================================
// TESTS : ActivateShopUsecase - Repository errors
// ============================================================

func TestActivateShopUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	// Mock : Shop non trouvée
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, errors.New("shop not found"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find shop")
}

func TestActivateShopUsecase_ShopNotFound_Nil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	// Mock : Shop nil
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop not found")
}

func TestActivateShopUsecase_ShopNotSuspended(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	// Créer un shop actif (pas suspendu)
	activeShop := createTestShop("Active Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activeShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrShopNotSuspended, err)
}

func TestActivateShopUsecase_ActivateShopError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	// Créer un shop suspendu
	now := time.Now()
	previousAdmin := "admin-456"
	reason := "Previous reason"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		SuspendedAt:      &now,
		SuspendedBy:      &previousAdmin,
		SuspensionReason: &reason,
		HealthScore:      500,
		HealthLevel:      entity.ShopHealthWarning,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	// Mock : ActivateShop échoue
	mockShopRepo.EXPECT().
		ActivateShop(gomock.Any(), shopID, admin.AdminID).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "activate shop")
}

func TestActivateShopUsecase_ReloadShopError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	now := time.Now()
	previousAdmin := "admin-456"
	reason := "Previous reason"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		SuspendedAt:      &now,
		SuspendedBy:      &previousAdmin,
		SuspensionReason: &reason,
		HealthScore:      500,
		HealthLevel:      entity.ShopHealthWarning,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	mockShopRepo.EXPECT().
		ActivateShop(gomock.Any(), shopID, admin.AdminID).
		Return(nil)

	// Mock : Reload échoue
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, errors.New("reload error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "reload shop")
}

// ============================================================
// TESTS : ActivateShopUsecase - Non-blocking errors
// ============================================================

func TestActivateShopUsecase_UpdateHealthScoreError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	now := time.Now()
	previousAdmin := "admin-456"
	reason := "Previous reason"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		SuspendedAt:      &now,
		SuspendedBy:      &previousAdmin,
		SuspensionReason: &reason,
		HealthScore:      500,
		HealthLevel:      entity.ShopHealthWarning,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	mockShopRepo.EXPECT().
		ActivateShop(gomock.Any(), shopID, admin.AdminID).
		Return(nil)

	// Reload avec shop activé
	activatedShop := createTestShop("Suspended Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activatedShop.ID = shopID
	activatedShop.HealthScore = 1000
	activatedShop.HealthLevel = entity.ShopHealthExcellent

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activatedShop, nil)

	// Mock : UpdateHealthScore échoue (non bloquant)
	mockShopRepo.EXPECT().
		UpdateHealthScore(gomock.Any(), shopID, gomock.Any(), gomock.Any()).
		Return(errors.New("health score error"))

	// Mock : actionRepo.Create réussit
	mockActionRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}

func TestActivateShopUsecase_ActionRepoCreateError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
	}

	now := time.Now()
	previousAdmin := "admin-456"
	reason := "Previous reason"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		SuspendedAt:      &now,
		SuspendedBy:      &previousAdmin,
		SuspensionReason: &reason,
		HealthScore:      500,
		HealthLevel:      entity.ShopHealthWarning,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	mockShopRepo.EXPECT().
		ActivateShop(gomock.Any(), shopID, admin.AdminID).
		Return(nil)

	activatedShop := createTestShop("Suspended Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activatedShop.ID = shopID
	activatedShop.HealthScore = 1000
	activatedShop.HealthLevel = entity.ShopHealthExcellent

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activatedShop, nil)

	mockShopRepo.EXPECT().
		UpdateHealthScore(gomock.Any(), shopID, gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : actionRepo.Create échoue (non bloquant)
	mockActionRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("audit log error"))

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}

// ============================================================
// TESTS : ActivateShopUsecase - Happy path
// ============================================================

func TestActivateShopUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewActivateShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID.String(),
		Reason: strPtr("Réactivation après correction"),
	}

	now := time.Now()
	previousAdmin := "admin-456"
	reason := "Previous reason"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		Slug:             "suspended-shop",
		OwnerID:          "owner-123",
		Plan:             entity.ShopPlanFree,
		SuspendedAt:      &now,
		SuspendedBy:      &previousAdmin,
		SuspensionReason: &reason,
		HealthScore:      500,
		HealthLevel:      entity.ShopHealthWarning,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	mockShopRepo.EXPECT().
		ActivateShop(gomock.Any(), shopID, admin.AdminID).
		Return(nil)

	activatedShop := createTestShop("Suspended Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activatedShop.ID = shopID
	activatedShop.Slug = "suspended-shop"
	activatedShop.OwnerID = "owner-123"
	activatedShop.HealthScore = 1000
	activatedShop.HealthLevel = entity.ShopHealthExcellent

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activatedShop, nil)

	mockShopRepo.EXPECT().
		UpdateHealthScore(gomock.Any(), shopID, gomock.Any(), gomock.Any()).
		Return(nil)

	mockActionRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Contains(t, response.Message, "Suspended Shop")
	assert.Equal(t, shopID.String(), response.Shop.ID)
	assert.Equal(t, "Suspended Shop", response.Shop.Name)
	assert.True(t, response.Shop.IsActive)
	assert.Equal(t, admin.AdminID, response.Reactivation.ReactivatedBy)
	assert.Equal(t, "Réactivation après correction", *response.Reactivation.Reason)
	assert.NotNil(t, response.Reactivation.PreviousSuspension)
	assert.Equal(t, previousAdmin, response.Reactivation.PreviousSuspension.SuspendedBy)
	assert.NotEmpty(t, response.AuditActionID)
}

// Helper pour créer des pointeurs de string
func strPtr(s string) *string {
	return &s
}
