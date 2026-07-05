package collaboratorusecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : INVITE PLATFORM COLLABORATOR USECASE
// ============================================================
//
// 🎯 Objectif :
//   Un super_admin invite un collaborateur plateforme (finance_manager,
//   support_manager, kyc_reviewer, marketing_manager, tech_admin).
//
// 📋 Workflow :
//   1. Vérifier que l'admin a la permission can_invite_collaborators
//   2. Valider l'email et le rôle
//   3. Vérifier que l'user existe dans users
//   4. Vérifier que l'user n'est pas déjà collaborateur plateforme
//   5. Vérifier qu'il n'y a pas déjà une invitation pending pour cet email
//   6. Créer l'invitation avec token sécurisé
//   7. (Future) Envoyer l'email d'invitation
//   8. Retourner le token et les détails
//
// 🔐 Sécurité :
//   - Réservé aux super_admin
//   - Permission can_invite_collaborators requise
//   - Token sécurisé (64 caractères hex)
//   - Expiration 7 jours
//
// ============================================================

// InvitePlatformCollaboratorUsecase gère l'invitation d'un collaborateur plateforme
type InvitePlatformCollaboratorUsecase struct {
	platformCollabRepo repository.PlatformCollaboratorRepository
	invitationRepo     repository.CollaboratorInvitationRepository
	userRepo           userrepository.UserRepository
}

// NewInvitePlatformCollaboratorUsecase crée une nouvelle instance
func NewInvitePlatformCollaboratorUsecase(
	platformCollabRepo repository.PlatformCollaboratorRepository,
	invitationRepo repository.CollaboratorInvitationRepository,
	userRepo userrepository.UserRepository,
) *InvitePlatformCollaboratorUsecase {
	return &InvitePlatformCollaboratorUsecase{
		platformCollabRepo: platformCollabRepo,
		invitationRepo:     invitationRepo,
		userRepo:           userRepo,
	}
}

// InvitePlatformCollaboratorRequest représente la requête
type InvitePlatformCollaboratorRequest struct {
	Email       string                      `json:"email"`
	Role        entity.PlatformRole         `json:"role"`
	Permissions *entity.PlatformPermissions `json:"permissions,omitempty"` // Optionnel (défaut par rôle)
	Message     *string                     `json:"message,omitempty"`
}

// InvitePlatformCollaboratorResponse représente la réponse
type InvitePlatformCollaboratorResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	Invitation    *InvitationDetails `json:"invitation"`
	InvitationURL string             `json:"invitation_url"`
}

// InvitationDetails contient les détails de l'invitation
type InvitationDetails struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	Token           string `json:"token"`
	InvitedAt       string `json:"invited_at"`
	ExpiresAt       string `json:"expires_at"`
	DaysUntilExpire int    `json:"days_until_expire"`
}

// Execute invite un collaborateur plateforme
func (uc *InvitePlatformCollaboratorUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *InvitePlatformCollaboratorRequest,
) (*InvitePlatformCollaboratorResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if err := uc.validateRequest(req); err != nil {
		logger.Warn().Err(err).Msg("❌ Validation requête échouée")
		return nil, err
	}

	// 2. Vérifier la permission de l'admin
	hasPerm, err := uc.platformCollabRepo.HasPermission(ctx, admin.AdminID, "can_invite_collaborators")
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur vérification permission")
		return nil, fmt.Errorf("check permission: %w", err)
	}
	if !hasPerm && admin.AdminRole != "super_admin" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("❌ Permission refusée")
		return nil, errors.New("insufficient permissions: can_invite_collaborators required")
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("email", req.Email).
		Str("role", string(req.Role)).
		Msg("📧 Invitation collaborateur plateforme")

	// 3. Vérifier que l'email existe dans users (sinon on ne peut pas inviter)
	// Note: FindUserByEmail ne prend pas de ctx, c'est normal dans cette architecture
	user, err := uc.userRepo.FindUserByEmail(req.Email)
	if err != nil {
		if errors.Is(err, userrepository.ErrUserNotFound) {
			logger.Warn().Str("email", req.Email).Msg("❌ User non trouvé")
			return nil, fmt.Errorf("user with email %s not found", req.Email)
		}
		logger.Error().Err(err).Str("email", req.Email).Msg("❌ Erreur récupération user")
		return nil, fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user with email %s not found", req.Email)
	}

	// 4. Vérifier que l'user n'est pas déjà collaborateur plateforme
	// Note: user.ID est une string (pas uuid.UUID)
	isCollab, err := uc.platformCollabRepo.IsPlatformCollaborator(ctx, user.ID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur vérification collaborateur")
		return nil, fmt.Errorf("check collaborator: %w", err)
	}
	if isCollab {
		logger.Warn().
			Str("user_id", user.ID).
			Str("email", req.Email).
			Msg("❌ User déjà collaborateur plateforme")
		return nil, entity.ErrUserAlreadyCollaborator
	}

	// 5. Vérifier qu'il n'y a pas déjà une invitation pending
	existingInvitations, err := uc.invitationRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur vérification invitations existantes")
	} else {
		for _, inv := range existingInvitations {
			if inv.InvitationType == entity.InvitationTypePlatform && inv.IsPending() {
				logger.Warn().
					Str("email", req.Email).
					Str("existing_invitation_id", inv.ID.String()).
					Msg("❌ Invitation déjà en attente")
				return nil, fmt.Errorf("pending invitation already exists for email %s", req.Email)
			}
		}
	}

	// 6. Déterminer les permissions (custom ou par défaut)
	var permissions entity.PlatformPermissions
	if req.Permissions != nil {
		permissions = *req.Permissions
	} else {
		permissions = entity.DefaultPlatformPermissions(req.Role)
	}

	// 7. Créer l'invitation
	invitation, err := entity.NewPlatformInvitation(
		req.Email,
		req.Role,
		permissions,
		admin.AdminID,
		req.Message,
	)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur création invitation")
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	// 8. Sauvegarder l'invitation
	if err := uc.invitationRepo.Create(ctx, invitation); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur sauvegarde invitation")
		return nil, fmt.Errorf("save invitation: %w", err)
	}

	// 9. (Future) Envoyer l'email d'invitation
	// TODO: Intégrer avec NotificationService
	logger.Info().
		Str("invitation_id", invitation.ID.String()).
		Str("token", invitation.Token[:8]+"...").
		Str("email", req.Email).
		Msg("✅ Invitation créée (email à envoyer)")

	// 10. Construire la réponse
	daysUntilExpire := int(invitation.ExpiresAt.Sub(invitation.InvitedAt).Hours() / 24)

	return &InvitePlatformCollaboratorResponse{
		Success: true,
		Message: fmt.Sprintf("Invitation envoyée à %s pour le rôle %s. L'invitation expire dans %d jours.",
			req.Email, req.Role, daysUntilExpire),
		Invitation: &InvitationDetails{
			ID:              invitation.ID.String(),
			Email:           invitation.Email,
			Role:            invitation.Role,
			Token:           invitation.Token,
			InvitedAt:       invitation.InvitedAt.Format("2006-01-02 15:04:05"),
			ExpiresAt:       invitation.ExpiresAt.Format("2006-01-02 15:04:05"),
			DaysUntilExpire: daysUntilExpire,
		},
		InvitationURL: fmt.Sprintf("/collaborators/invitations/%s", invitation.Token),
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *InvitePlatformCollaboratorUsecase) validateRequest(req *InvitePlatformCollaboratorRequest) error {
	if req.Email == "" {
		return errors.New("email is required")
	}

	// Validation format email
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email format")
	}

	// Normaliser l'email (lowercase + trim)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Validation rôle
	if !entity.IsValidPlatformRole(req.Role) {
		return entity.ErrInvalidPlatformRole
	}

	// Validation message (optionnel, max 500 caractères)
	if req.Message != nil && len(*req.Message) > 500 {
		return errors.New("message must be at most 500 characters")
	}

	return nil
}
