package collaboratorusecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	collaboratorusecase "Goshop/application/usecase/collaborator_usecase"
	"Goshop/domain/entity"
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/domain/service"
	mockrepo "Goshop/mocks/repository"
	mockservice "Goshop/mocks/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.12 : TESTS UNITAIRES - INVITE SHOP COLLABORATOR USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestShopAdminContext crée un contexte admin shop
func createTestShopAdminContext() *collaboratorusecase.AdminContext {
	return &collaboratorusecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@goshop.com",
		AdminRole:  "merchant",
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		RequestID:  "req-abc",
	}
}

// createTestActiveShop crée une boutique active de test
func createTestActiveShop(id uuid.UUID, name string) *entity.Shop {
	return &entity.Shop{
		ID:       id,
		Name:     name,
		Slug:     "test-shop",
		OwnerID:  "owner-123",
		IsActive: true,
	}
}

// createTestUserForShop crée un user entity de test
func createTestUserForShop(id string, email string) *userentity.UserEntity {
	return &userentity.UserEntity{
		ID:     id,
		Email:  email,
		Role:   "merchant",
		Active: true,
		Status: "active",
	}
}

// createValidShopInviteRequest crée une requête d'invitation boutique valide
func createValidShopInviteRequest(shopID uuid.UUID) *collaboratorusecase.InviteShopCollaboratorRequest {
	return &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID: shopID.String(),
		Email:  "newuser@example.com",
		Role:   entity.ShopRoleSeller,
	}
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - Validation
// ============================================================

func TestInviteShopCollaboratorUsecase_EmptyShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID: "", // ❌ Vide
		Email:  "test@example.com",
		Role:   entity.ShopRoleSeller,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestInviteShopCollaboratorUsecase_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID: "invalid-uuid", // ❌ UUID invalide
		Email:  "test@example.com",
		Role:   entity.ShopRoleSeller,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

func TestInviteShopCollaboratorUsecase_EmptyEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID: shopID.String(),
		Email:  "", // ❌ Vide
		Role:   entity.ShopRoleSeller,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "email is required")
}

func TestInviteShopCollaboratorUsecase_InvalidEmailFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID: shopID.String(),
		Email:  "invalid-email", // ❌ Format invalide
		Role:   entity.ShopRoleSeller,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid email format")
}

func TestInviteShopCollaboratorUsecase_InvalidRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID: shopID.String(),
		Email:  "test@example.com",
		Role:   "invalid_role", // ❌ Rôle invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvalidShopRole, err)
}

func TestInviteShopCollaboratorUsecase_MessageTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	longMessage := strings.Repeat("a", 501) // ❌ > 500 caractères
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID:  shopID.String(),
		Email:   "test@example.com",
		Role:    entity.ShopRoleSeller,
		Message: &longMessage,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at most 500 characters")
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - Shop validation
// ============================================================

func TestInviteShopCollaboratorUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)

	// Mock : Shop non trouvée
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop not found")
}

func TestInviteShopCollaboratorUsecase_ShopInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)

	// Shop inactive
	inactiveShop := &entity.Shop{
		ID:       shopID,
		Name:     "Inactive Shop",
		IsActive: false,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(inactiveShop, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop is not active")
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - Permissions
// ============================================================

func TestInviteShopCollaboratorUsecase_NotShopAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext() // merchant (pas super_admin)
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	// Mock : Pas shop_admin
	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(false, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestInviteShopCollaboratorUsecase_SuperAdminBypass(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "super-admin-123",
		AdminEmail: "super@goshop.com",
		AdminRole:  "super_admin", // ✅ Super admin
	}
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	// Mock : Pas shop_admin (mais super_admin bypass)
	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(false, nil)

	// Mock : User existe
	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	// Mock : Pas déjà collaborateur
	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	// Mock : Pas d'invitation pending
	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{}, nil)

	// Mock : Create réussit
	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : Email service non configuré
	mockEmailService.EXPECT().
		IsConfigured().
		Return(false)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - User errors
// ============================================================

func TestInviteShopCollaboratorUsecase_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	// Mock : User non trouvé
	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(nil, userrepository.ErrUserNotFound)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not found")
}

func TestInviteShopCollaboratorUsecase_AlreadyCollaborator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	// Collaborateur existant actif
	existingCollab := &entity.ShopCollaborator{
		ID:       uuid.New(),
		ShopID:   shopID,
		UserID:   user.ID,
		Role:     entity.ShopRoleSeller,
		IsActive: true,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	// Mock : Déjà collaborateur
	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(existingCollab, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrUserAlreadyCollaborator, err)
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - Pending invitation
// ============================================================

func TestInviteShopCollaboratorUsecase_PendingInvitationExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	// Mock : Invitation pending existante
	now := time.Now()
	pendingInvitation := &entity.CollaboratorInvitation{
		ID:             uuid.New(),
		InvitationType: entity.InvitationTypeShop,
		ShopID:         &shopID,
		Email:          req.Email,
		Status:         entity.InvitationStatusPending,
		InvitedAt:      now,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
	}

	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{pendingInvitation}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "pending invitation already exists")
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - Email service
// ============================================================

func TestInviteShopCollaboratorUsecase_EmailServiceDebugMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{}, nil)

	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : Email service configuré en mode debug
	mockEmailService.EXPECT().
		IsConfigured().
		Return(true)

	mockEmailService.EXPECT().
		SendEmailAsync(gomock.Any())

	mockEmailService.EXPECT().
		GetProvider().
		Return("debug")

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.False(t, response.EmailSent) // ❌ False en mode debug
	assert.Equal(t, "debug", response.EmailProvider)
	assert.Equal(t, "debug_logged", response.EmailStatus)
}

func TestInviteShopCollaboratorUsecase_EmailServiceSent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{}, nil)

	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : Email service configuré (SMTP)
	mockEmailService.EXPECT().
		IsConfigured().
		Return(true)

	mockEmailService.EXPECT().
		SendEmailAsync(gomock.Any())

	mockEmailService.EXPECT().
		GetProvider().
		Return("smtp")

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.True(t, response.EmailSent) // ✅ True en mode SMTP
	assert.Equal(t, "smtp", response.EmailProvider)
	assert.Equal(t, "sent", response.EmailStatus)
}

// ============================================================
// TESTS : InviteShopCollaboratorUsecase - Happy paths
// ============================================================

func TestInviteShopCollaboratorUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{}, nil)

	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : Email service non configuré
	mockEmailService.EXPECT().
		IsConfigured().
		Return(false)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Contains(t, response.Message, req.Email)
	assert.Contains(t, response.Message, "Active Shop")
	assert.NotNil(t, response.Invitation)
	assert.Equal(t, req.Email, response.Invitation.Email)
	assert.Equal(t, string(req.Role), response.Invitation.Role)
	assert.Equal(t, shopID.String(), response.Invitation.ShopID)
	assert.Equal(t, "Active Shop", response.Invitation.ShopName)
	assert.NotEmpty(t, response.Invitation.Token)
	assert.NotEmpty(t, response.InvitationURL)
	assert.Equal(t, 7, response.Invitation.DaysUntilExpire)
	assert.False(t, response.EmailSent)
	assert.Equal(t, "not_configured", response.EmailStatus)
}

func TestInviteShopCollaboratorUsecase_Success_WithCustomMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	customMessage := "Bienvenue dans l'équipe !"
	req := &collaboratorusecase.InviteShopCollaboratorRequest{
		ShopID:  shopID.String(),
		Email:   "newuser@example.com",
		Role:    entity.ShopRoleSeller,
		Message: &customMessage,
	}
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{}, nil)

	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : Email service configuré
	mockEmailService.EXPECT().
		IsConfigured().
		Return(true)

	mockEmailService.EXPECT().
		SendEmailAsync(gomock.Any())

	mockEmailService.EXPECT().
		GetProvider().
		Return("smtp")

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.True(t, response.EmailSent)
}

// ============================================================
// TEST : InviteShopCollaboratorUsecase - Nil emailService
// ============================================================

func TestInviteShopCollaboratorUsecase_NilEmailService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	// ✅ Passer nil pour emailService
	uc := collaboratorusecase.NewInviteShopCollaboratorUsecase(
		mockShopCollabRepo,
		mockInvitationRepo,
		mockShopRepo,
		mockUserRepo,
		nil, // ❌ EmailService nil
	)

	ctx := context.Background()
	admin := createTestShopAdminContext()
	shopID := uuid.New()
	req := createValidShopInviteRequest(shopID)
	user := createTestUserForShop("user-123", req.Email)

	activeShop := createTestActiveShop(shopID, "Active Shop")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopCollabRepo.EXPECT().
		IsShopAdmin(gomock.Any(), admin.AdminID, shopID).
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockShopCollabRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	mockInvitationRepo.EXPECT().
		FindPendingByShopID(gomock.Any(), shopID).
		Return([]*entity.CollaboratorInvitation{}, nil)

	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.False(t, response.EmailSent)
	assert.Equal(t, "not_configured", response.EmailStatus)
}

// ============================================================
// TEST : Helper GetShopRoleDisplayName
// ============================================================

func TestGetShopRoleDisplayName(t *testing.T) {
	tests := []struct {
		role     string
		expected string
	}{
		{"shop_admin", "Administrateur Boutique"},
		{"seller", "Vendeur"},
		{"support", "Support Client"},
		{"accountant", "Comptable"},
		{"unknown_role", "unknown_role"}, // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := service.GetShopRoleDisplayName(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}
