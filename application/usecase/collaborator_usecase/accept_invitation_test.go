package collaboratorusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	collaboratorusecase "Goshop/application/usecase/collaborator_usecase"
	"Goshop/domain/entity"
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.12 : TESTS UNITAIRES - ACCEPT INVITATION USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestPlatformInvitation crée une invitation plateforme de test
func createTestPlatformInvitation(token string, status entity.InvitationStatus) *entity.CollaboratorInvitation {
	now := time.Now()
	return &entity.CollaboratorInvitation{
		ID:             uuid.New(),
		InvitationType: entity.InvitationTypePlatform,
		Email:          "test@example.com",
		Role:           string(entity.PlatformRoleTechAdmin),
		Permissions:    []byte(`{"can_manage_settings":true}`),
		Token:          token,
		InvitedBy:      "inviter-123",
		Status:         status,
		InvitedAt:      now,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// createTestShopInvitation crée une invitation boutique de test
func createTestShopInvitation(token string, shopID uuid.UUID, status entity.InvitationStatus) *entity.CollaboratorInvitation {
	now := time.Now()
	return &entity.CollaboratorInvitation{
		ID:             uuid.New(),
		InvitationType: entity.InvitationTypeShop,
		ShopID:         &shopID,
		Email:          "test@example.com",
		Role:           string(entity.ShopRoleSeller),
		Permissions:    []byte(`{"can_manage_products":true}`),
		Token:          token,
		InvitedBy:      "inviter-123",
		Status:         status,
		InvitedAt:      now,
		ExpiresAt:      now.Add(7 * 24 * time.Hour),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// createTestUser crée un user de test
func createTestUser(id string, email string) *userentity.UserEntity {
	return &userentity.UserEntity{
		ID:     id,
		Email:  email,
		Role:   "merchant",
		Active: true,
		Status: "active",
	}
}

// validToken crée un token valide de 64 caractères hex
func validToken() string {
	return "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
}

// ============================================================
// TESTS : AcceptInvitationUsecase - Validation du token
// ============================================================

func TestAcceptInvitationUsecase_EmptyToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "token is required")
}

func TestAcceptInvitationUsecase_TokenTooShort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: "short", // ❌ < 64 caractères
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "must be 64 characters")
}

func TestAcceptInvitationUsecase_TokenTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: "a" + validToken(), // ❌ 65 caractères
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "must be 64 characters")
}

func TestAcceptInvitationUsecase_TokenInvalidHex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", // ❌ Non hex
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "must be hexadecimal")
}

// ============================================================
// TESTS : AcceptInvitationUsecase - Invitation errors
// ============================================================

func TestAcceptInvitationUsecase_InvitationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	// Mock : Invitation non trouvée
	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(nil, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvitationNotFound, err)
}

func TestAcceptInvitationUsecase_InvitationAlreadyUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	// Invitation déjà acceptée
	acceptedInvitation := createTestPlatformInvitation(token, entity.InvitationStatusAccepted)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(acceptedInvitation, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvitationAlreadyUsed, err)
}

func TestAcceptInvitationUsecase_InvitationCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	// Invitation annulée
	cancelledInvitation := createTestPlatformInvitation(token, entity.InvitationStatusCancelled)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(cancelledInvitation, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvitationCancelled, err)
}

func TestAcceptInvitationUsecase_InvitationExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	// Invitation expirée
	expiredInvitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	expiredInvitation.ExpiresAt = time.Now().Add(-24 * time.Hour) // ❌ Expirée

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(expiredInvitation, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrInvitationExpired, err)
}

// ============================================================
// TESTS : AcceptInvitationUsecase - User errors
// ============================================================

func TestAcceptInvitationUsecase_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	invitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	// Mock : User non trouvé
	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(nil, userrepository.ErrUserNotFound)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not found")
}

// ============================================================
// TESTS : AcceptInvitationUsecase - Platform Happy path
// ============================================================

func TestAcceptInvitationUsecase_Platform_AlreadyCollaborator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	invitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	// Mock : User déjà collaborateur plateforme
	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(true, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrUserAlreadyCollaborator, err)
}

func TestAcceptInvitationUsecase_Platform_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	invitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	// Mock : User n'est pas encore collaborateur
	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	// Mock : Create réussit
	mockPlatformRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : MarkAccepted réussit
	mockInvitationRepo.EXPECT().
		MarkAccepted(gomock.Any(), token).
		Return(nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "platform", response.InvitationType)
	assert.Equal(t, "tech_admin", response.Role)
	assert.NotNil(t, response.PlatformCollaboratorID)
}

// ============================================================
// TESTS : AcceptInvitationUsecase - Shop Happy path
// ============================================================

func TestAcceptInvitationUsecase_Shop_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	// Mock : Shop non trouvée
	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop not found")
}

func TestAcceptInvitationUsecase_Shop_ShopInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	// Shop inactive
	inactiveShop := &entity.Shop{
		ID:       shopID,
		Name:     "Inactive Shop",
		IsActive: false,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(inactiveShop, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop is not active")
}

func TestAcceptInvitationUsecase_Shop_AlreadyCollaborator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	activeShop := &entity.Shop{
		ID:       shopID,
		Name:     "Active Shop",
		IsActive: true,
	}

	// Collaborateur existant actif
	existingCollab := &entity.ShopCollaborator{
		ID:       uuid.New(),
		ShopID:   shopID,
		UserID:   user.ID,
		Role:     entity.ShopRoleSeller,
		IsActive: true,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	// Mock : User déjà collaborateur de cette boutique
	mockShopRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(existingCollab, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrUserAlreadyCollaborator, err)
}

func TestAcceptInvitationUsecase_Shop_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	activeShop := &entity.Shop{
		ID:       shopID,
		Name:     "Active Shop",
		IsActive: true,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	// Mock : User n'est pas encore collaborateur
	mockShopRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	// Mock : Create réussit
	mockShopRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : MarkAccepted réussit
	mockInvitationRepo.EXPECT().
		MarkAccepted(gomock.Any(), token).
		Return(nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "shop", response.InvitationType)
	assert.Equal(t, "seller", response.Role)
	assert.NotNil(t, response.ShopCollaboratorID)
	assert.NotNil(t, response.ShopID)
	assert.NotNil(t, response.ShopName)
	assert.Equal(t, "Active Shop", *response.ShopName)
}

// ============================================================
// TESTS : AcceptInvitationUsecase.GetInvitationPreview
// ============================================================

func TestGetInvitationPreview_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	preview, err := uc.GetInvitationPreview(ctx, "invalid")

	assert.Error(t, err)
	assert.Nil(t, preview)
	assert.Contains(t, err.Error(), "must be 64 characters")
}

func TestGetInvitationPreview_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()

	// Mock : Invitation non trouvée
	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(nil, nil)

	preview, err := uc.GetInvitationPreview(ctx, token)

	assert.Error(t, err)
	assert.Nil(t, preview)
	assert.Equal(t, entity.ErrInvitationNotFound, err)
}

func TestGetInvitationPreview_AlreadyUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()

	// Invitation déjà acceptée
	acceptedInvitation := createTestPlatformInvitation(token, entity.InvitationStatusAccepted)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(acceptedInvitation, nil)

	preview, err := uc.GetInvitationPreview(ctx, token)

	assert.Error(t, err)
	assert.Nil(t, preview)
	assert.Equal(t, entity.ErrInvitationAlreadyUsed, err)
}

func TestGetInvitationPreview_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()

	// Invitation expirée
	expiredInvitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	expiredInvitation.ExpiresAt = time.Now().Add(-24 * time.Hour)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(expiredInvitation, nil)

	preview, err := uc.GetInvitationPreview(ctx, token)

	assert.Error(t, err)
	assert.Nil(t, preview)
	assert.Equal(t, entity.ErrInvitationExpired, err)
}

func TestGetInvitationPreview_Success_ShopInvitation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	shopID := uuid.New()

	// Invitation boutique valide
	shopInvitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)

	// Shop active
	activeShop := &entity.Shop{
		ID:       shopID,
		Name:     "Test Shop",
		Slug:     "test-shop",
		IsActive: true,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(shopInvitation, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	preview, err := uc.GetInvitationPreview(ctx, token)

	assert.NoError(t, err)
	assert.NotNil(t, preview)
	assert.Equal(t, "shop", preview.InvitationType)
	assert.Equal(t, shopInvitation.Email, preview.Email)
	assert.Equal(t, shopInvitation.Role, preview.Role)
	assert.NotNil(t, preview.ShopName)
	assert.Equal(t, "Test Shop", *preview.ShopName)
	assert.NotNil(t, preview.ShopSlug)
	assert.Equal(t, "test-shop", *preview.ShopSlug)
}

// ============================================================
// TESTS : AcceptInvitationUsecase - acceptPlatformInvitation (branches manquantes)
// ============================================================

func TestAcceptInvitationUsecase_Platform_IsPlatformCollaboratorError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	invitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	// Mock : IsPlatformCollaborator échoue
	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, errors.New("database error"))

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "check collaborator")
}

func TestAcceptInvitationUsecase_Platform_InvalidPermissionsJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	// Invitation avec permissions JSON invalides
	invitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	invitation.Permissions = []byte(`{invalid json}`) // ❌ JSON invalide

	user := createTestUser("user-123", invitation.Email)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "get permissions")
}

func TestAcceptInvitationUsecase_Platform_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	invitation := createTestPlatformInvitation(token, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockPlatformRepo.EXPECT().
		IsPlatformCollaborator(gomock.Any(), user.ID).
		Return(false, nil)

	// Mock : Create échoue
	mockPlatformRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "save collaborator")
}

// ============================================================
// TESTS : AcceptInvitationUsecase - acceptShopInvitation (branches manquantes)
// ============================================================

func TestAcceptInvitationUsecase_Shop_FindByShopIDAndUserIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	activeShop := &entity.Shop{
		ID:       shopID,
		Name:     "Active Shop",
		IsActive: true,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	// Mock : FindByShopIDAndUserID échoue
	mockShopRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, errors.New("database error"))

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "check collaborator")
}

func TestAcceptInvitationUsecase_Shop_InvalidPermissionsJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	// Invitation avec permissions JSON invalides
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	invitation.Permissions = []byte(`{invalid json}`) // ❌ JSON invalide

	user := createTestUser("user-123", invitation.Email)

	activeShop := &entity.Shop{
		ID:       shopID,
		Name:     "Active Shop",
		IsActive: true,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "get permissions")
}

func TestAcceptInvitationUsecase_Shop_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInvitationRepo := mockrepo.NewMockCollaboratorInvitationRepository(ctrl)
	mockPlatformRepo := mockrepo.NewMockPlatformCollaboratorRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	mockShopRepository := mockrepo.NewMockShopRepository(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := collaboratorusecase.NewAcceptInvitationUsecase(
		mockInvitationRepo,
		mockPlatformRepo,
		mockShopRepo,
		mockShopRepository,
		mockUserRepo,
	)

	ctx := context.Background()
	token := validToken()
	req := &collaboratorusecase.AcceptInvitationRequest{
		Token: token,
	}

	shopID := uuid.New()
	invitation := createTestShopInvitation(token, shopID, entity.InvitationStatusPending)
	user := createTestUser("user-123", invitation.Email)

	activeShop := &entity.Shop{
		ID:       shopID,
		Name:     "Active Shop",
		IsActive: true,
	}

	mockInvitationRepo.EXPECT().
		FindByToken(gomock.Any(), token).
		Return(invitation, nil)

	mockUserRepo.EXPECT().
		FindUserByEmail(invitation.Email).
		Return(user, nil)

	mockShopRepository.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(activeShop, nil)

	mockShopRepo.EXPECT().
		FindByShopIDAndUserID(gomock.Any(), shopID, user.ID).
		Return(nil, nil)

	// Mock : Create échoue
	mockShopRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "save collaborator")
}
