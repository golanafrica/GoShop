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

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : INVITE SHOP COLLABORATOR USECASE
// ============================================================
//
// 🎯 Objectif :
//   Un shop_admin invite un collaborateur boutique (seller, support, accountant)
//   pour sa propre boutique.
//
// 📋 Workflow :
//   1. Valider la requête (email, rôle, shopID)
//   2. Vérifier que l'admin est shop_admin de la boutique
//   3. Vérifier que la boutique existe et est active
//   4. Vérifier que l'user existe dans users
//   5. Vérifier que l'user n'est pas déjà collaborateur de CETTE boutique
//   6. Vérifier qu'il n'y a pas déjà une invitation pending pour cet email + shop
//   7. Créer l'invitation avec token sécurisé
//   8. Sauvegarder l'invitation
//   9. (Future) Envoyer l'email d'invitation
//   10. Retourner le token et les détails
//
// 🔐 Sécurité :
//   - Réservé aux shop_admin de la boutique
//   - Vérification multi-boutique (user peut être collab dans plusieurs boutiques)
//   - Token sécurisé (64 caractères hex)
//   - Expiration 7 jours
//
// ============================================================

// InviteShopCollaboratorUsecase gère l'invitation d'un collaborateur boutique
type InviteShopCollaboratorUsecase struct {
	shopCollabRepo repository.ShopCollaboratorRepository
	invitationRepo repository.CollaboratorInvitationRepository
	shopRepo       repository.ShopRepository
	userRepo       userrepository.UserRepository
}

// NewInviteShopCollaboratorUsecase crée une nouvelle instance
func NewInviteShopCollaboratorUsecase(
	shopCollabRepo repository.ShopCollaboratorRepository,
	invitationRepo repository.CollaboratorInvitationRepository,
	shopRepo repository.ShopRepository,
	userRepo userrepository.UserRepository,
) *InviteShopCollaboratorUsecase {
	return &InviteShopCollaboratorUsecase{
		shopCollabRepo: shopCollabRepo,
		invitationRepo: invitationRepo,
		shopRepo:       shopRepo,
		userRepo:       userRepo,
	}
}

// InviteShopCollaboratorRequest représente la requête
type InviteShopCollaboratorRequest struct {
	ShopID      string                  `json:"shop_id"`
	Email       string                  `json:"email"`
	Role        entity.ShopRole         `json:"role"`
	Permissions *entity.ShopPermissions `json:"permissions,omitempty"` // Optionnel (défaut par rôle)
	Message     *string                 `json:"message,omitempty"`
}

// InviteShopCollaboratorResponse représente la réponse
type InviteShopCollaboratorResponse struct {
	Success       bool                   `json:"success"`
	Message       string                 `json:"message"`
	Invitation    *ShopInvitationDetails `json:"invitation"`
	InvitationURL string                 `json:"invitation_url"`
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

	// Parser le shopID
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
		// Fallback : vérifier si super_admin (peut inviter dans toutes les boutiques)
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

	// 5. Vérifier que l'user n'est pas déjà collaborateur de CETTE boutique
	// Note: user.ID est une string (pas uuid.UUID)
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

	// 10. (Future) Envoyer l'email d'invitation
	// TODO: Intégrer avec NotificationService
	logger.Info().
		Str("invitation_id", invitation.ID.String()).
		Str("token", invitation.Token[:8]+"...").
		Str("email", req.Email).
		Str("shop_id", shopID.String()).
		Msg("✅ Invitation créée (email à envoyer)")

	// 11. Construire la réponse
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

func (uc *InviteShopCollaboratorUsecase) validateRequest(req *InviteShopCollaboratorRequest) error {
	if req.ShopID == "" {
		return errors.New("shop_id is required")
	}

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
	if !entity.IsValidShopRole(req.Role) {
		return entity.ErrInvalidShopRole
	}

	// Validation message (optionnel, max 500 caractères)
	if req.Message != nil && len(*req.Message) > 500 {
		return errors.New("message must be at most 500 characters")
	}

	return nil
}
