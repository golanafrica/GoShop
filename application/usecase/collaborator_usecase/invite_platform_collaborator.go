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
	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : INVITE PLATFORM COLLABORATOR USECASE
// 🆕 v4.3.2 : + Intégration EmailService pour envoi automatique
// ============================================================

// InvitePlatformCollaboratorUsecase gère l'invitation d'un collaborateur plateforme
type InvitePlatformCollaboratorUsecase struct {
	platformCollabRepo repository.PlatformCollaboratorRepository
	invitationRepo     repository.CollaboratorInvitationRepository
	userRepo           userrepository.UserRepository
	emailService       service.EmailService // 🆕 v4.3.2
}

// NewInvitePlatformCollaboratorUsecase crée une nouvelle instance
func NewInvitePlatformCollaboratorUsecase(
	platformCollabRepo repository.PlatformCollaboratorRepository,
	invitationRepo repository.CollaboratorInvitationRepository,
	userRepo userrepository.UserRepository,
	emailService service.EmailService, // 🆕 v4.3.2
) *InvitePlatformCollaboratorUsecase {
	return &InvitePlatformCollaboratorUsecase{
		platformCollabRepo: platformCollabRepo,
		invitationRepo:     invitationRepo,
		userRepo:           userRepo,
		emailService:       emailService, // 🆕 v4.3.2
	}
}

// InvitePlatformCollaboratorRequest représente la requête
type InvitePlatformCollaboratorRequest struct {
	Email       string                      `json:"email"`
	Role        entity.PlatformRole         `json:"role"`
	Permissions *entity.PlatformPermissions `json:"permissions,omitempty"`
	Message     *string                     `json:"message,omitempty"`
}

// InvitePlatformCollaboratorResponse représente la réponse
type InvitePlatformCollaboratorResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	Invitation    *InvitationDetails `json:"invitation"`
	InvitationURL string             `json:"invitation_url"`
	EmailSent     bool               `json:"email_sent"`     // 🆕 v4.3.2
	EmailProvider string             `json:"email_provider"` // 🆕 v4.3.2
	EmailStatus   string             `json:"email_status"`   // 🆕 v4.3.2 : "sent", "debug_logged", "failed", "not_configured"
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

	// 3. Vérifier que l'email existe dans users
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

	// 🆕 v4.3.2 : Envoyer l'email d'invitation (asynchrone, non-bloquant)
	emailSent := false
	emailProvider := ""
	emailStatus := "not_configured" // 🆕 v4.3.2

	if uc.emailService != nil && uc.emailService.IsConfigured() {
		customMessage := ""
		if req.Message != nil {
			customMessage = *req.Message
		}

		emailData := &service.PlatformInvitationData{
			RecipientEmail:  invitation.Email,
			RecipientName:   "",
			InvitationID:    invitation.ID.String(),
			Token:           invitation.Token,
			Role:            string(invitation.Role),
			RoleDisplayName: service.GetPlatformRoleDisplayName(string(invitation.Role)),
			InviterName:     admin.AdminEmail,
			InviterEmail:    admin.AdminEmail,
			InviterRole:     admin.AdminRole,
			PlatformName:    "GoShop",
			PlatformURL:     "http://localhost:8081",
			CustomMessage:   customMessage,
			ExpiresAt:       invitation.ExpiresAt,
			DaysUntilExpire: int(invitation.ExpiresAt.Sub(invitation.InvitedAt).Hours() / 24),
		}

		// Envoi asynchrone (ne bloque pas la réponse)
		uc.emailService.SendEmailAsync(&service.EmailMessage{
			To:       emailData.RecipientEmail,
			Subject:  fmt.Sprintf("🎉 Invitation à rejoindre GoShop en tant que %s", emailData.RoleDisplayName),
			HTMLBody: uc.buildPlatformInvitationHTML(emailData),
			TextBody: uc.buildPlatformInvitationText(emailData),
		})

		emailProvider = uc.emailService.GetProvider()

		// 🆕 v4.3.2 : email_sent = false en mode debug (plus honnête)
		if emailProvider == "debug" {
			emailSent = false
			emailStatus = "debug_logged"
		} else {
			emailSent = true
			emailStatus = "sent"
		}

		logger.Info().
			Str("to", invitation.Email).
			Str("provider", emailProvider).
			Str("status", emailStatus).
			Msg("📧 Email d'invitation traité (asynchrone)")
	} else {
		logger.Warn().Msg("⚠️ Email service non configuré - email non envoyé")
	}

	// 9. Construire la réponse
	daysUntilExpire := int(invitation.ExpiresAt.Sub(invitation.InvitedAt).Hours() / 24)

	return &InvitePlatformCollaboratorResponse{
		Success: true,
		Message: fmt.Sprintf("Invitation envoyée à %s pour le rôle %s. L'invitation expire dans %d jours.",
			req.Email, req.Role, daysUntilExpire),
		Invitation: &InvitationDetails{
			ID:              invitation.ID.String(),
			Email:           invitation.Email,
			Role:            string(invitation.Role),
			Token:           invitation.Token,
			InvitedAt:       invitation.InvitedAt.Format("2006-01-02 15:04:05"),
			ExpiresAt:       invitation.ExpiresAt.Format("2006-01-02 15:04:05"),
			DaysUntilExpire: daysUntilExpire,
		},
		InvitationURL: fmt.Sprintf("/collaborators/invitations/%s", invitation.Token),
		EmailSent:     emailSent,
		EmailProvider: emailProvider,
		EmailStatus:   emailStatus, // 🆕 v4.3.2
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *InvitePlatformCollaboratorUsecase) validateRequest(req *InvitePlatformCollaboratorRequest) error {
	if req.Email == "" {
		return errors.New("email is required")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email format")
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if !entity.IsValidPlatformRole(req.Role) {
		return entity.ErrInvalidPlatformRole
	}

	if req.Message != nil && len(*req.Message) > 500 {
		return errors.New("message must be at most 500 characters")
	}

	return nil
}

// ============================================================
// 🆕 v4.3.2 : EMAIL BUILDERS (inline pour simplicité)
// ============================================================

func (uc *InvitePlatformCollaboratorUsecase) buildPlatformInvitationHTML(data *service.PlatformInvitationData) string {
	acceptURL := fmt.Sprintf("http://localhost:8081/collaborators/invitations/%s", data.Token)

	customMessageHTML := ""
	if data.CustomMessage != "" {
		customMessageHTML = fmt.Sprintf(`
    <div style="background-color: #eff6ff; border-left: 4px solid #2563eb; padding: 15px; margin: 20px 0;">
        <p style="margin: 0;"><strong>Message de %s :</strong></p>
        <p style="margin: 10px 0 0 0; font-style: italic;">"%s"</p>
    </div>`, data.InviterName, data.CustomMessage)
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>Invitation GoShop</title></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; background-color: #f9fafb;">
    <div style="background-color: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <h1 style="color: #2563eb; margin-top: 0;">🎉 Vous êtes invité(e) !</h1>
        <p>Bonjour,</p>
        <p><strong>%s</strong> vous invite à rejoindre <strong>%s</strong> en tant que <strong>%s</strong>.</p>
        %s
        <div style="background-color: #f3f4f6; padding: 15px; border-radius: 6px; margin: 20px 0;">
            <p style="margin: 5px 0;"><strong>Rôle :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Expiration :</strong> %s (%d jours)</p>
        </div>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; padding: 14px 28px; background-color: #2563eb; color: white; text-decoration: none; border-radius: 6px; font-weight: bold;">
                Accepter l'invitation
            </a>
        </div>
        <p style="color: #6b7280; font-size: 12px; margin-top: 30px;">
            © 2026 GoShop. Tous droits réservés.
        </p>
    </div>
</body>
</html>`,
		data.InviterName, data.PlatformName, data.RoleDisplayName,
		customMessageHTML,
		data.RoleDisplayName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL,
	)
}

func (uc *InvitePlatformCollaboratorUsecase) buildPlatformInvitationText(data *service.PlatformInvitationData) string {
	acceptURL := fmt.Sprintf("http://localhost:8081/collaborators/invitations/%s", data.Token)
	return fmt.Sprintf(`Bonjour,

%s vous invite à rejoindre %s en tant que %s.

Rôle : %s
Expiration : %s (%d jours)

Pour accepter : %s

© 2026 GoShop.`,
		data.InviterName, data.PlatformName, data.RoleDisplayName,
		data.RoleDisplayName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL,
	)
}
