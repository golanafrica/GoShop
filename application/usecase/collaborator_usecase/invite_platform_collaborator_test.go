package collaboratorusecase_test

import (
	"context"
	"errors"
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
// 🆕 v4.4.12 : TESTS UNITAIRES - INVITE PLATFORM COLLABORATOR USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestPlatformAdminContext crée un contexte admin plateforme
func createTestPlatformAdminContext() *collaboratorusecase.AdminContext {
	return &collaboratorusecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@goshop.com",
		AdminRole:  "super_admin",
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		RequestID:  "req-abc",
	}
}

// createTestUserEntity crée un user entity de test
func createTestUserEntity(id string, email string) *userentity.UserEntity {
	return &userentity.UserEntity{
		ID:     id,
		Email:  email,
		Role:   "merchant",
		Active: true,
		Status: "active",
	}
}

// createValidPlatformInviteRequest crée une requête d'invitation plateforme valide
func createValidPlatformInviteRequest() *collaboratorusecase.InvitePlatformCollaboratorRequest {
	return &collaboratorusecase.InvitePlatformCollaboratorRequest{
		Email: "newuser@example.com",
		Role:  entity.PlatformRoleTechAdmin,
	}
}

// ============================================================
// TESTS : InvitePlatformCollaboratorUsecase - Validation
// ============================================================

func TestInvitePlatformCollaboratorUsecase_EmptyEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := &collaboratorusecase.InvitePlatformCollaboratorRequest{
		Email: "", // ❌ Vide
		Role:  entity.PlatformRoleTechAdmin,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "email is required")
}

func TestInvitePlatformCollaboratorUsecase_InvalidEmailFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := &collaboratorusecase.InvitePlatformCollaboratorRequest{
		Email: "invalid-email", // ❌ Format invalide
		Role:  entity.PlatformRoleTechAdmin,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid email format")
}

func TestInvitePlatformCollaboratorUsecase_InvalidRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := &collaboratorusecase.InvitePlatformCollaboratorRequest{
		Email: "test@example.com",
		Role:  "invalid_role", // ❌ Rôle invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvalidPlatformRole, err)
}

func TestInvitePlatformCollaboratorUsecase_MessageTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	longMessage := strings.Repeat("a", 501) // ❌ > 500 caractères
	req := &collaboratorusecase.InvitePlatformCollaboratorRequest{
		Email:   "test@example.com",
		Role:    entity.PlatformRoleTechAdmin,
		Message: &longMessage,
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at most 500 characters")
}

// ============================================================
// TESTS : InvitePlatformCollaboratorUsecase - Permissions
// ============================================================

func TestInvitePlatformCollaboratorUsecase_InsufficientPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := &collaboratorusecase.AdminContext{
		AdminID:    "merchant-123",
		AdminEmail: "merchant@goshop.com",
		AdminRole:  "merchant", // ❌ Pas super_admin
	}
	req := createValidPlatformInviteRequest()

	// Mock : Pas de permission can_invite_collaborators
	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(false, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestInvitePlatformCollaboratorUsecase_SuperAdminBypass(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext() // super_admin
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	// Mock : Permission check (même si false, super_admin bypass)
	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(false, nil)

	// Mock : User existe
	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	// Mock : Pas déjà collaborateur
	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	// Mock : Pas d'invitation pending
	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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
// TESTS : InvitePlatformCollaboratorUsecase - Repository errors
// ============================================================

func TestInvitePlatformCollaboratorUsecase_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()

	// Mock : Permission OK
	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
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

func TestInvitePlatformCollaboratorUsecase_AlreadyCollaborator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	// Mock : Déjà collaborateur
	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(true, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrUserAlreadyCollaborator, err)
}

func TestInvitePlatformCollaboratorUsecase_PendingInvitationExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	// ✅ Mock : Invitation pending existante (avec ExpiresAt dans le futur)
	now := time.Now()
	pendingInvitation := &entity.CollaboratorInvitation{
		ID:             uuid.New(),
		InvitationType: entity.InvitationTypePlatform,
		Status:         entity.InvitationStatusPending,
		Email:          req.Email,
		InvitedAt:      now,
		ExpiresAt:      now.Add(7 * 24 * time.Hour), // ✅ Dans le futur
	}

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
		Return([]*entity.CollaboratorInvitation{pendingInvitation}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "pending invitation already exists")
}
func TestInvitePlatformCollaboratorUsecase_SaveInvitationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
		Return([]*entity.CollaboratorInvitation{}, nil)

	// Mock : Create échoue
	mockInvitationRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "save invitation")
}

// ============================================================
// TESTS : InvitePlatformCollaboratorUsecase - Email service
// ============================================================

func TestInvitePlatformCollaboratorUsecase_EmailServiceNotConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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
	assert.False(t, response.EmailSent)
	assert.Equal(t, "not_configured", response.EmailStatus)
}

func TestInvitePlatformCollaboratorUsecase_EmailServiceDebugMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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

func TestInvitePlatformCollaboratorUsecase_EmailServiceSent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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
// TESTS : InvitePlatformCollaboratorUsecase - Happy path
// ============================================================

func TestInvitePlatformCollaboratorUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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
	assert.NotNil(t, response.Invitation)
	assert.Equal(t, req.Email, response.Invitation.Email)
	assert.Equal(t, string(req.Role), response.Invitation.Role)
	assert.NotEmpty(t, response.Invitation.Token)
	assert.NotEmpty(t, response.InvitationURL)
	assert.Equal(t, 7, response.Invitation.DaysUntilExpire)
}

func TestInvitePlatformCollaboratorUsecase_Success_WithCustomMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)
	mockEmailService := mockservice.NewMockEmailService(ctrl)

	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		mockEmailService,
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	customMessage := "Bienvenue dans l'équipe !"
	req := &collaboratorusecase.InvitePlatformCollaboratorRequest{
		Email:   "newuser@example.com",
		Role:    entity.PlatformRoleTechAdmin,
		Message: &customMessage,
	}
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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
// TEST : InvitePlatformCollaboratorUsecase - Nil emailService
// ============================================================

func TestInvitePlatformCollaboratorUsecase_NilEmailService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	// ✅ Passer nil pour emailService
	uc := collaboratorusecase.NewInvitePlatformCollaboratorUsecase(
		mockPlatformRepo,
		mockInvitationRepo,
		mockUserRepo,
		nil, // ❌ EmailService nil
	)

	ctx := context.Background()
	admin := createTestPlatformAdminContext()
	req := createValidPlatformInviteRequest()
	user := createTestUserEntity("user-123", req.Email)

	mockPlatformRepo.EXPECT().
		HasPermission(gomock.Any(), admin.AdminID, "can_invite_collaborators").
		Return(true, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(req.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	mockInvitationRepo.EXPECT().
		FindByEmail(gomock.Any(), req.Email).
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
// TEST : Helper GetPlatformRoleDisplayName
// ============================================================

func TestGetPlatformRoleDisplayName(t *testing.T) {
	tests := []struct {
		role     string
		expected string
	}{
		{"finance_manager", "Responsable Finances"},
		{"support_manager", "Responsable Support"},
		{"kyc_reviewer", "Validateur KYC"},
		{"marketing_manager", "Responsable Marketing"},
		{"tech_admin", "Administrateur Technique"},
		{"unknown_role", "unknown_role"}, // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := service.GetPlatformRoleDisplayName(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}
