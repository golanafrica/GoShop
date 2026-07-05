package collaboratorusecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : ACCEPT INVITATION USECASE
// ============================================================

// AcceptInvitationUsecase gère l'acceptation d'une invitation
type AcceptInvitationUsecase struct {
	invitationRepo     repository.CollaboratorInvitationRepository
	platformCollabRepo repository.PlatformCollaboratorRepository
	shopCollabRepo     repository.ShopCollaboratorRepository
	shopRepo           repository.ShopRepository
	userRepo           userrepository.UserRepository
}

// NewAcceptInvitationUsecase crée une nouvelle instance
func NewAcceptInvitationUsecase(
	invitationRepo repository.CollaboratorInvitationRepository,
	platformCollabRepo repository.PlatformCollaboratorRepository,
	shopCollabRepo repository.ShopCollaboratorRepository,
	shopRepo repository.ShopRepository,
	userRepo userrepository.UserRepository,
) *AcceptInvitationUsecase {
	return &AcceptInvitationUsecase{
		invitationRepo:     invitationRepo,
		platformCollabRepo: platformCollabRepo,
		shopCollabRepo:     shopCollabRepo,
		shopRepo:           shopRepo,
		userRepo:           userRepo,
	}
}

// AcceptInvitationRequest représente la requête
type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

// AcceptInvitationResponse représente la réponse
type AcceptInvitationResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	InvitationType string `json:"invitation_type"`
	Role           string `json:"role"`

	// Pour invitation plateforme
	PlatformCollaboratorID *string `json:"platform_collaborator_id,omitempty"`

	// Pour invitation boutique
	ShopCollaboratorID *string `json:"shop_collaborator_id,omitempty"`
	ShopID             *string `json:"shop_id,omitempty"`
	ShopName           *string `json:"shop_name,omitempty"`
}

// Execute accepte une invitation
func (uc *AcceptInvitationUsecase) Execute(
	ctx context.Context,
	req *AcceptInvitationRequest,
) (*AcceptInvitationResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation du token
	if err := uc.validateToken(req.Token); err != nil {
		logger.Warn().Err(err).Msg("❌ Token invalide")
		return nil, err
	}

	// Log partiel du token (8 premiers caractères) pour debug
	tokenPreview := req.Token[:8] + "..."
	logger.Info().
		Str("token_preview", tokenPreview).
		Msg("🎫 Tentative d'acceptation d'invitation")

	// 2. Récupérer l'invitation par token
	invitation, err := uc.invitationRepo.FindByToken(ctx, req.Token)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération invitation")
		return nil, fmt.Errorf("find invitation: %w", err)
	}
	if invitation == nil {
		logger.Warn().
			Str("token_preview", tokenPreview).
			Msg("❌ Invitation non trouvée")
		return nil, entity.ErrInvitationNotFound
	}

	// 3. Vérifier que l'invitation est valide
	if !invitation.IsPending() {
		logger.Warn().
			Str("invitation_id", invitation.ID.String()).
			Str("status", string(invitation.Status)).
			Msg("❌ Invitation non pending")

		switch invitation.Status {
		case entity.InvitationStatusAccepted:
			return nil, entity.ErrInvitationAlreadyUsed
		case entity.InvitationStatusCancelled:
			return nil, entity.ErrInvitationCancelled
		}
	}

	if invitation.IsExpired() {
		logger.Warn().
			Str("invitation_id", invitation.ID.String()).
			Time("expires_at", invitation.ExpiresAt).
			Msg("❌ Invitation expirée")
		return nil, entity.ErrInvitationExpired
	}

	logger.Info().
		Str("invitation_id", invitation.ID.String()).
		Str("email", invitation.Email).
		Str("role", invitation.Role).
		Str("type", string(invitation.InvitationType)).
		Msg("📧 Invitation valide trouvée")

	// 4. Vérifier que l'user existe (via email)
	user, err := uc.userRepo.FindUserByEmail(invitation.Email)
	if err != nil {
		if errors.Is(err, userrepository.ErrUserNotFound) {
			logger.Warn().
				Str("email", invitation.Email).
				Msg("❌ User non trouvé")
			return nil, fmt.Errorf("user with email %s not found. Please register first", invitation.Email)
		}
		logger.Error().Err(err).Msg("❌ Erreur récupération user")
		return nil, fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user with email %s not found. Please register first", invitation.Email)
	}

	logger.Info().
		Str("user_id", user.ID).
		Str("email", user.Email).
		Msg("✅ User trouvé")

	// 5. Créer le collaborateur (selon type)
	var response *AcceptInvitationResponse

	switch invitation.InvitationType {
	case entity.InvitationTypePlatform:
		response, err = uc.acceptPlatformInvitation(ctx, invitation, user.ID)
		if err != nil {
			return nil, err
		}

	case entity.InvitationTypeShop:
		if invitation.ShopID == nil {
			logger.Error().Msg("❌ ShopID manquant pour invitation boutique")
			return nil, errors.New("shop_id is required for shop invitation")
		}
		response, err = uc.acceptShopInvitation(ctx, invitation, user.ID)
		if err != nil {
			return nil, err
		}

	default:
		logger.Error().
			Str("type", string(invitation.InvitationType)).
			Msg("❌ Type d'invitation inconnu")
		return nil, fmt.Errorf("unknown invitation type: %s", invitation.InvitationType)
	}

	// 6. Marquer l'invitation comme acceptée
	if err := uc.invitationRepo.MarkAccepted(ctx, req.Token); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur marquage invitation acceptée")
		// Non bloquant : le collaborateur est déjà créé
	}

	logger.Info().
		Str("invitation_id", invitation.ID.String()).
		Str("user_id", user.ID).
		Str("type", string(invitation.InvitationType)).
		Msg("✅ Invitation acceptée avec succès")

	return response, nil
}

// ============================================================
// MÉTHODES PRIVÉES
// ============================================================

func (uc *AcceptInvitationUsecase) acceptPlatformInvitation(
	ctx context.Context,
	invitation *entity.CollaboratorInvitation,
	userID string,
) (*AcceptInvitationResponse, error) {
	logger := zerolog.Ctx(ctx)

	// Vérifier que l'user n'est pas déjà collaborateur plateforme
	isCollab, err := uc.platformCollabRepo.IsPlatformCollaborator(ctx, userID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur vérification collaborateur")
		return nil, fmt.Errorf("check collaborator: %w", err)
	}
	if isCollab {
		logger.Warn().
			Str("user_id", userID).
			Msg("❌ User déjà collaborateur plateforme")
		return nil, entity.ErrUserAlreadyCollaborator
	}

	// Récupérer les permissions
	permissions, err := invitation.GetPlatformPermissions()
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération permissions")
		return nil, fmt.Errorf("get permissions: %w", err)
	}

	// Créer le collaborateur plateforme
	collab, err := entity.NewPlatformCollaborator(
		userID,
		entity.PlatformRole(invitation.Role),
		permissions,
		invitation.InvitedBy,
	)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur création collaborateur")
		return nil, fmt.Errorf("create collaborator: %w", err)
	}

	// Marquer comme accepté
	collab.MarkAccepted()

	// Sauvegarder
	if err := uc.platformCollabRepo.Create(ctx, collab); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur sauvegarde collaborateur")
		return nil, fmt.Errorf("save collaborator: %w", err)
	}

	logger.Info().
		Str("collaborator_id", collab.ID.String()).
		Str("user_id", userID).
		Str("role", string(collab.Role)).
		Msg("✅ Collaborateur plateforme créé")

	return &AcceptInvitationResponse{
		Success:                true,
		Message:                fmt.Sprintf("Vous êtes maintenant collaborateur plateforme avec le rôle %s", collab.Role),
		InvitationType:         "platform",
		Role:                   string(collab.Role),
		PlatformCollaboratorID: strPtr(collab.ID.String()),
	}, nil
}

func (uc *AcceptInvitationUsecase) acceptShopInvitation(
	ctx context.Context,
	invitation *entity.CollaboratorInvitation,
	userID string,
) (*AcceptInvitationResponse, error) {
	logger := zerolog.Ctx(ctx)

	shopID := *invitation.ShopID

	// Vérifier que la boutique existe et est active
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

	// Vérifier que l'user n'est pas déjà collaborateur de cette boutique
	existingCollab, err := uc.shopCollabRepo.FindByShopIDAndUserID(ctx, shopID, userID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur vérification collaborateur")
		return nil, fmt.Errorf("check collaborator: %w", err)
	}
	if existingCollab != nil && existingCollab.IsActive {
		logger.Warn().
			Str("user_id", userID).
			Str("shop_id", shopID.String()).
			Msg("❌ User déjà collaborateur de cette boutique")
		return nil, entity.ErrUserAlreadyCollaborator
	}

	// Récupérer les permissions
	permissions, err := invitation.GetShopPermissions()
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération permissions")
		return nil, fmt.Errorf("get permissions: %w", err)
	}

	// Créer le collaborateur boutique
	collab, err := entity.NewShopCollaborator(
		shopID,
		userID,
		entity.ShopRole(invitation.Role),
		permissions,
		invitation.InvitedBy,
	)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur création collaborateur")
		return nil, fmt.Errorf("create collaborator: %w", err)
	}

	// Marquer comme accepté
	collab.MarkAccepted()

	// Sauvegarder
	if err := uc.shopCollabRepo.Create(ctx, collab); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur sauvegarde collaborateur")
		return nil, fmt.Errorf("save collaborator: %w", err)
	}

	logger.Info().
		Str("collaborator_id", collab.ID.String()).
		Str("user_id", userID).
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Str("role", string(collab.Role)).
		Msg("✅ Collaborateur boutique créé")

	return &AcceptInvitationResponse{
		Success:            true,
		Message:            fmt.Sprintf("Vous êtes maintenant collaborateur de la boutique '%s' avec le rôle %s", shop.Name, collab.Role),
		InvitationType:     "shop",
		Role:               string(collab.Role),
		ShopCollaboratorID: strPtr(collab.ID.String()),
		ShopID:             strPtr(shopID.String()),
		ShopName:           strPtr(shop.Name),
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *AcceptInvitationUsecase) validateToken(token string) error {
	if token == "" {
		return errors.New("token is required")
	}

	// Token doit être 64 caractères hex (32 bytes)
	if len(token) != 64 {
		return errors.New("invalid token format: must be 64 characters")
	}

	// Vérifier que c'est bien de l'hex
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return errors.New("invalid token format: must be hexadecimal")
		}
	}

	return nil
}

// ============================================================
// MÉTHODES PUBLIQUES SUPPLÉMENTAIRES
// ============================================================

// GetInvitationPreview retourne un aperçu de l'invitation (pour affichage avant acceptation)
func (uc *AcceptInvitationUsecase) GetInvitationPreview(
	ctx context.Context,
	token string,
) (*InvitationPreview, error) {
	logger := zerolog.Ctx(ctx)

	// Validation
	if err := uc.validateToken(token); err != nil {
		return nil, err
	}

	// Récupérer l'invitation
	invitation, err := uc.invitationRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("find invitation: %w", err)
	}
	if invitation == nil {
		return nil, entity.ErrInvitationNotFound
	}

	// Vérifier statut
	if !invitation.IsPending() {
		switch invitation.Status {
		case entity.InvitationStatusAccepted:
			return nil, entity.ErrInvitationAlreadyUsed
		case entity.InvitationStatusCancelled:
			return nil, entity.ErrInvitationCancelled
		}
	}
	if invitation.IsExpired() {
		return nil, entity.ErrInvitationExpired
	}

	// Construire l'aperçu
	preview := &InvitationPreview{
		InvitationType: string(invitation.InvitationType),
		Email:          invitation.Email,
		Role:           invitation.Role,
		InvitedAt:      invitation.InvitedAt.Format("2006-01-02 15:04:05"),
		ExpiresAt:      invitation.ExpiresAt.Format("2006-01-02 15:04:05"),
		Message:        invitation.Message,
	}

	// Ajouter infos boutique si type = shop
	if invitation.InvitationType == entity.InvitationTypeShop && invitation.ShopID != nil {
		shop, err := uc.shopRepo.FindByID(ctx, *invitation.ShopID)
		if err == nil && shop != nil {
			preview.ShopName = &shop.Name
			preview.ShopSlug = &shop.Slug
		}
	}

	logger.Info().
		Str("invitation_type", preview.InvitationType).
		Str("email", preview.Email).
		Str("role", preview.Role).
		Msg("👁️ Aperçu invitation généré")

	return preview, nil
}

// InvitationPreview représente l'aperçu d'une invitation
type InvitationPreview struct {
	InvitationType string  `json:"invitation_type"`
	Email          string  `json:"email"`
	Role           string  `json:"role"`
	InvitedAt      string  `json:"invited_at"`
	ExpiresAt      string  `json:"expires_at"`
	Message        *string `json:"message,omitempty"`

	// Pour invitation boutique
	ShopName *string `json:"shop_name,omitempty"`
	ShopSlug *string `json:"shop_slug,omitempty"`
}

// NormalizeToken normalise un token (trim + lowercase)
func NormalizeToken(token string) string {
	return strings.ToLower(strings.TrimSpace(token))
}
