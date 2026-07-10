package adminshopusecase_test

import (
	"context"
	"errors"
	"strings"
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
// 🆕 v4.4.11 : TESTS UNITAIRES - SUSPEND SHOP USECASE
// ============================================================

// ============================================================
// TESTS : SuspendShopUsecase - Validation
// ============================================================

func TestSuspendShopUsecase_EmptyShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: "", // ❌ Vide
		Reason: "Violation des conditions d'utilisation",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestSuspendShopUsecase_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: "invalid-uuid", // ❌ UUID invalide
		Reason: "Violation des conditions d'utilisation",
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

func TestSuspendShopUsecase_EmptyReason(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: uuid.New().String(),
		Reason: "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "reason is required")
}

func TestSuspendShopUsecase_ReasonTooShort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: uuid.New().String(),
		Reason: "short", // ❌ < 10 caractères
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at least 10 characters")
}

func TestSuspendShopUsecase_ReasonTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: uuid.New().String(),
		Reason: strings.Repeat("a", 1001), // ❌ > 1000 caractères
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at most 1000 characters")
}

// ============================================================
// TESTS : SuspendShopUsecase - Repository errors
// ============================================================

func TestSuspendShopUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
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

func TestSuspendShopUsecase_ShopNotFound_Nil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
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

func TestSuspendShopUsecase_ShopAlreadySuspended(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
	}

	// Créer un shop déjà suspendu
	now := time.Now()
	previousAdmin := "admin-456"
	reason := "Previous reason"
	alreadySuspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Already Suspended Shop",
		SuspendedAt:      &now,
		SuspendedBy:      &previousAdmin,
		SuspensionReason: &reason,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(alreadySuspendedShop, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrShopAlreadySuspended, err)
}

func TestSuspendShopUsecase_SuspendShopError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
	}

	activeShop := createTestShop("Active Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activeShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	// Mock : SuspendShop échoue
	mockShopRepo.EXPECT().
		SuspendShop(gomock.Any(), shopID, admin.AdminID, req.Reason).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "suspend shop")
}

func TestSuspendShopUsecase_ReloadShopError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
	}

	activeShop := createTestShop("Active Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activeShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopRepo.EXPECT().
		SuspendShop(gomock.Any(), shopID, admin.AdminID, req.Reason).
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
// TESTS : SuspendShopUsecase - Non-blocking errors
// ============================================================

func TestSuspendShopUsecase_UpdateHealthScoreError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
	}

	activeShop := createTestShop("Active Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activeShop.ID = shopID

	// Premier FindByID (vérification)
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopRepo.EXPECT().
		SuspendShop(gomock.Any(), shopID, admin.AdminID, req.Reason).
		Return(nil)

	// Deuxième FindByID (reload)
	suspendedShop := createTestShop("Active Shop", entity.ShopPlanFree, false, entity.ShopKYCStatusVerified)
	suspendedShop.ID = shopID
	now := time.Now()
	adminID := admin.AdminID
	reason := req.Reason
	suspendedShop.SuspendedAt = &now
	suspendedShop.SuspendedBy = &adminID
	suspendedShop.SuspensionReason = &reason

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	// Mock : UpdateHealthScore échoue (non bloquant)
	mockShopRepo.EXPECT().
		UpdateHealthScore(gomock.Any(), shopID, gomock.Any(), gomock.Any()).
		Return(errors.New("health score error"))

	// Mock : actionRepo.Create réussit
	mockActionRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante, la réponse est quand même retournée
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}

func TestSuspendShopUsecase_ActionRepoCreateError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
	}

	activeShop := createTestShop("Active Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activeShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopRepo.EXPECT().
		SuspendShop(gomock.Any(), shopID, admin.AdminID, req.Reason).
		Return(nil)

	suspendedShop := createTestShop("Active Shop", entity.ShopPlanFree, false, entity.ShopKYCStatusVerified)
	suspendedShop.ID = shopID
	now := time.Now()
	adminID := admin.AdminID
	reason := req.Reason
	suspendedShop.SuspendedAt = &now
	suspendedShop.SuspendedBy = &adminID
	suspendedShop.SuspensionReason = &reason

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

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
// TESTS : SuspendShopUsecase - Happy path
// ============================================================

func TestSuspendShopUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockActionRepo := mockrepo.NewMockShopAdminActionRepository(ctrl)

	uc := adminshopusecase.NewSuspendShopUsecase(mockShopRepo, mockActionRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID.String(),
		Reason: "Violation des conditions d'utilisation",
	}

	activeShop := createTestShop("Active Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	activeShop.ID = shopID
	activeShop.HealthScore = 1000
	activeShop.HealthLevel = entity.ShopHealthExcellent

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopRepo.EXPECT().
		SuspendShop(gomock.Any(), shopID, admin.AdminID, req.Reason).
		Return(nil)

	suspendedShop := createTestShop("Active Shop", entity.ShopPlanFree, false, entity.ShopKYCStatusVerified)
	suspendedShop.ID = shopID
	now := time.Now()
	adminID := admin.AdminID
	reason := req.Reason
	suspendedShop.SuspendedAt = &now
	suspendedShop.SuspendedBy = &adminID
	suspendedShop.SuspensionReason = &reason
	suspendedShop.HealthScore = 500
	suspendedShop.HealthLevel = entity.ShopHealthWarning

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

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
	assert.Contains(t, response.Message, "Active Shop")
	assert.Equal(t, shopID.String(), response.Shop.ID)
	assert.Equal(t, "Active Shop", response.Shop.Name)
	assert.False(t, response.Shop.IsActive)
	assert.Equal(t, admin.AdminID, response.Suspension.SuspendedBy)
	assert.Equal(t, req.Reason, response.Suspension.Reason)
	assert.Equal(t, 0, response.Suspension.DaysSuspended)
	assert.NotEmpty(t, response.AuditActionID)
}
