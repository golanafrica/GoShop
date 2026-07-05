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

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : INVITE SHOP COLLABORATOR USECASE
// 🆕 v4.3.2 : + Intégration EmailService pour envoi automatique
// ============================================================

// InviteShopCollaboratorUsecase gère l'invitation d'un collaborateur boutique
type InviteShopCollaboratorUsecase struct {
	shopCollabRepo repository.ShopCollaboratorRepository
	invitationRepo repository.CollaboratorInvitationRepository
	shopRepo       repository.ShopRepository
	userRepo       userrepository.UserRepository
	emailService   service.EmailService // 🆕 v4.3.2
}

// NewInviteShopCollaboratorUsecase crée une nouvelle instance
func NewInviteShopCollaboratorUsecase(
	shopCollabRepo repository.ShopCollaboratorRepository,
	invitationRepo repository.CollaboratorInvitationRepository,
	shopRepo repository.ShopRepository,
	userRepo userrepository.UserRepository,
	emailService service.EmailService, // 🆕 v4.3.2
) *InviteShopCollaboratorUsecase {
	return &InviteShopCollaboratorUsecase{
		shopCollabRepo: shopCollabRepo,
		invitationRepo: invitationRepo,
		shopRepo:       shopRepo,
		userRepo:       userRepo,
		emailService:   emailService, // 🆕 v4.3.2
	}
}

// InviteShopCollaboratorRequest représente la requête
type InviteShopCollaboratorRequest struct {
	ShopID      string                  `json:"shop_id"`
	Email       string                  `json:"email"`
	Role        entity.ShopRole         `json:"role"`
	Permissions *entity.ShopPermissions `json:"permissions,omitempty"`
	Message     *string                 `json:"message,omitempty"`
}

// InviteShopCollaboratorResponse représente la réponse
type InviteShopCollaboratorResponse struct {
	Success       bool                   `json:"success"`
	Message       string                 `json:"message"`
	Invitation    *ShopInvitationDetails `json:"invitation"`
	InvitationURL string                 `json:"invitation_url"`
	EmailSent     bool                   `json:"email_sent"`     // 🆕 v4.3.2
	EmailProvider string                 `json:"email_provider"` // 🆕 v4.3.2
	EmailStatus   string                 `json:"email_status"`   // 🆕 v4.3.2 : "sent", "debug_logged", "failed", "not_configured"
}

// ShopInvitationDetails contient les détails de l'invitation boutique
type ShopInvitationDetails struct {
	ID              string `json:"id"`
	ShopID          string `json:"shop_id"`
	ShopName        string `json:"shop_name"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	Token           string `json:"token"`
	InvitedAt       string `json:"invited_at"`
	ExpiresAt       string `json:"expires_at"`
	DaysUntilExpire int    `json:"days_until_expire"`
}

// Execute invite un collaborateur boutique
func (uc *InviteShopCollaboratorUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *InviteShopCollaboratorRequest,
) (*InviteShopCollaboratorResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if err := uc.validateRequest(req); err != nil {
		logger.Warn().Err(err).Msg("❌ Validation requête échouée")
		return nil, err
	}

	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		return nil, errors.New("invalid shop_id format")
	}

	// 2. Vérifier que la boutique existe et est active
	shop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération boutique")
		return nil, fmt.Errorf("find shop: %w", err)
	}
	if shop == nil {
		return nil, errors.New("shop not found")
	}
	if !shop.IsActive {
		return nil, errors.New("shop is not active")
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Str("email", req.Email).
		Str("role", string(req.Role)).
		Msg("📧 Invitation collaborateur boutique")

	// 3. Vérifier que l'admin est shop_admin de cette boutique
	isAdmin, err := uc.shopCollabRepo.IsShopAdmin(ctx, admin.AdminID, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur vérification shop_admin")
		return nil, fmt.Errorf("check shop admin: %w", err)
	}
	if !isAdmin {
		if admin.AdminRole != "super_admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("shop_id", shopID.String()).
				Msg("❌ Permission refusée : pas shop_admin")
			return nil, errors.New("insufficient permissions: must be shop_admin of this shop")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("ℹ️ Super admin peut inviter dans toutes les boutiques")
	}

	// 4. Vérifier que l'email existe dans users
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

	// 5. Vérifier que l'user n'est pas déjà collaborateur de CETTE boutique
	existingCollab, err := uc.shopCollabRepo.FindByShopIDAndUserID(ctx, shopID, user.ID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur vérification collaborateur existant")
		return nil, fmt.Errorf("check existing collaborator: %w", err)
	}
	if existingCollab != nil && existingCollab.IsActive {
		logger.Warn().
			Str("user_id", user.ID).
			Str("shop_id", shopID.String()).
			Str("existing_role", string(existingCollab.Role)).
			Msg("❌ User déjà collaborateur de cette boutique")
		return nil, entity.ErrUserAlreadyCollaborator
	}

	// 6. Vérifier qu'il n'y a pas déjà une invitation pending pour cet email + shop
	pendingInvitations, err := uc.invitationRepo.FindPendingByShopID(ctx, shopID)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur vérification invitations existantes")
	} else {
		for _, inv := range pendingInvitations {
			if strings.EqualFold(inv.Email, req.Email) {
				logger.Warn().
					Str("email", req.Email).
					Str("shop_id", shopID.String()).
					Str("existing_invitation_id", inv.ID.String()).
					Msg("❌ Invitation déjà en attente pour cette boutique")
				return nil, fmt.Errorf("pending invitation already exists for email %s in this shop", req.Email)
			}
		}
	}

	// 7. Déterminer les permissions (custom ou par défaut)
	var permissions entity.ShopPermissions
	if req.Permissions != nil {
		permissions = *req.Permissions
	} else {
		permissions = entity.DefaultShopPermissions(req.Role)
	}

	// 8. Créer l'invitation
	invitation, err := entity.NewShopInvitation(
		shopID,
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

	// 9. Sauvegarder l'invitation
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

		emailData := &service.ShopInvitationData{
			RecipientEmail:  invitation.Email,
			RecipientName:   "",
			InvitationID:    invitation.ID.String(),
			Token:           invitation.Token,
			Role:            string(invitation.Role),
			RoleDisplayName: service.GetShopRoleDisplayName(string(invitation.Role)),
			ShopID:          shopID.String(),
			ShopName:        shop.Name,
			ShopSlug:        shop.Slug,
			InviterName:     admin.AdminEmail,
			InviterEmail:    admin.AdminEmail,
			InviterRole:     admin.AdminRole,
			PlatformName:    "GoShop",
			PlatformURL:     "http://localhost:8081",
			CustomMessage:   customMessage,
			ExpiresAt:       invitation.ExpiresAt,
			DaysUntilExpire: int(invitation.ExpiresAt.Sub(invitation.InvitedAt).Hours() / 24),
		}

		// Envoi asynchrone
		uc.emailService.SendEmailAsync(&service.EmailMessage{
			To:       emailData.RecipientEmail,
			Subject:  fmt.Sprintf("🏪 Invitation à rejoindre '%s' en tant que %s", shop.Name, emailData.RoleDisplayName),
			HTMLBody: uc.buildShopInvitationHTML(emailData),
			TextBody: uc.buildShopInvitationText(emailData),
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
			Str("shop", shop.Name).
			Str("provider", emailProvider).
			Str("status", emailStatus).
			Msg("📧 Email d'invitation boutique traité (asynchrone)")
	} else {
		logger.Warn().Msg("⚠️ Email service non configuré - email non envoyé")
	}

	// 10. Construire la réponse
	daysUntilExpire := int(invitation.ExpiresAt.Sub(invitation.InvitedAt).Hours() / 24)

	return &InviteShopCollaboratorResponse{
		Success: true,
		Message: fmt.Sprintf("Invitation envoyée à %s pour le rôle %s dans la boutique '%s'. L'invitation expire dans %d jours.",
			req.Email, req.Role, shop.Name, daysUntilExpire),
		Invitation: &ShopInvitationDetails{
			ID:              invitation.ID.String(),
			ShopID:          shopID.String(),
			ShopName:        shop.Name,
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

func (uc *InviteShopCollaboratorUsecase) validateRequest(req *InviteShopCollaboratorRequest) error {
	if req.ShopID == "" {
		return errors.New("shop_id is required")
	}

	if req.Email == "" {
		return errors.New("email is required")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email format")
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if !entity.IsValidShopRole(req.Role) {
		return entity.ErrInvalidShopRole
	}

	if req.Message != nil && len(*req.Message) > 500 {
		return errors.New("message must be at most 500 characters")
	}

	return nil
}

// ============================================================
// 🆕 v4.3.2 : EMAIL BUILDERS (inline pour simplicité)
// ============================================================

func (uc *InviteShopCollaboratorUsecase) buildShopInvitationHTML(data *service.ShopInvitationData) string {
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
<head><meta charset="UTF-8"><title>Invitation %s</title></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; background-color: #f9fafb;">
    <div style="background-color: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
        <h1 style="color: #10b981; margin-top: 0;">🏪 Vous êtes invité(e) !</h1>
        <p>Bonjour,</p>
        <p><strong>%s</strong> vous invite à rejoindre la boutique <strong>%s</strong> en tant que <strong>%s</strong>.</p>
        %s
        <div style="background-color: #f3f4f6; padding: 15px; border-radius: 6px; margin: 20px 0;">
            <p style="margin: 5px 0;"><strong>Rôle :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Boutique :</strong> %s</p>
            <p style="margin: 5px 0;"><strong>Expiration :</strong> %s (%d jours)</p>
        </div>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; padding: 14px 28px; background-color: #10b981; color: white; text-decoration: none; border-radius: 6px; font-weight: bold;">
                Accepter l'invitation
            </a>
        </div>
        <p style="color: #6b7280; font-size: 12px; margin-top: 30px;">
            © 2026 GoShop. Tous droits réservés.
        </p>
    </div>
</body>
</html>`,
		data.ShopName,
		data.InviterName, data.ShopName, data.RoleDisplayName,
		customMessageHTML,
		data.RoleDisplayName, data.ShopName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL,
	)
}

func (uc *InviteShopCollaboratorUsecase) buildShopInvitationText(data *service.ShopInvitationData) string {
	acceptURL := fmt.Sprintf("http://localhost:8081/collaborators/invitations/%s", data.Token)
	return fmt.Sprintf(`Bonjour,

%s vous invite à rejoindre la boutique %s en tant que %s.

Rôle : %s
Boutique : %s
Expiration : %s (%d jours)

Pour accepter : %s

© 2026 GoShop.`,
		data.InviterName, data.ShopName, data.RoleDisplayName,
		data.RoleDisplayName, data.ShopName,
		data.ExpiresAt.Format("02/01/2006 à 15:04"), data.DaysUntilExpire,
		acceptURL,
	)
}
