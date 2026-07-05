package collaboratorusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : REMOVE COLLABORATOR USECASE
// ============================================================
//
// 🎯 Objectif :
//   Supprimer (soft delete) un collaborateur avec raison obligatoire,
//   vérification des permissions et audit trail.
//
// 🔐 Sécurité :
//   - Super admin : peut supprimer tous les collaborateurs
//   - Admin : peut supprimer les collaborateurs plateforme
//   - Shop admin : peut supprimer les collaborateurs de SA boutique
//   - Interdit de se supprimer soi-même
//   - Interdit de supprimer le dernier shop_admin d'une boutique
//   - Raison obligatoire (min 10 caractères)
//
// 🐛 v4.3.1 : Bug fix
//   - Suppression du FindByID après Deactivate (retournait nil)
//   - Utilisation de time.Now() pour le timestamp
//
// ============================================================

// RemoveCollaboratorUsecase gère la suppression d'un collaborateur
type RemoveCollaboratorUsecase struct {
	platformCollabRepo repository.PlatformCollaboratorRepository
	shopCollabRepo     repository.ShopCollaboratorRepository
	shopRepo           repository.ShopRepository
}

// NewRemoveCollaboratorUsecase crée une nouvelle instance
func NewRemoveCollaboratorUsecase(
	platformCollabRepo repository.PlatformCollaboratorRepository,
	shopCollabRepo repository.ShopCollaboratorRepository,
	shopRepo repository.ShopRepository,
) *RemoveCollaboratorUsecase {
	return &RemoveCollaboratorUsecase{
		platformCollabRepo: platformCollabRepo,
		shopCollabRepo:     shopCollabRepo,
		shopRepo:           shopRepo,
	}
}

// RemoveCollaboratorRequest représente la requête
type RemoveCollaboratorRequest struct {
	CollaboratorID string `json:"collaborator_id"`
	Type           string `json:"type"`              // "platform" ou "shop"
	Reason         string `json:"reason"`            // Obligatoire, min 10 caractères
	ShopID         string `json:"shop_id,omitempty"` // Requis si type = "shop"
}

// RemoveCollaboratorResponse représente la réponse
type RemoveCollaboratorResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	Type           string `json:"type"`
	CollaboratorID string `json:"collaborator_id"`
	UserID         string `json:"user_id"`
	OldRole        string `json:"old_role"`
	Reason         string `json:"reason"`
	DeletedAt      string `json:"deleted_at"`
	DeletedBy      string `json:"deleted_by"`
}

// Execute supprime un collaborateur
func (uc *RemoveCollaboratorUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *RemoveCollaboratorRequest,
) (*RemoveCollaboratorResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if err := uc.validateRequest(req); err != nil {
		logger.Warn().Err(err).Msg("❌ Validation requête échouée")
		return nil, err
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("collaborator_id", req.CollaboratorID).
		Str("type", req.Type).
		Str("reason", req.Reason).
		Msg("🗑️ Suppression collaborateur")

	// 2. Dispatch selon le type
	switch req.Type {
	case "platform":
		return uc.removePlatformCollaborator(ctx, admin, req)
	case "shop":
		return uc.removeShopCollaborator(ctx, admin, req)
	default:
		return nil, fmt.Errorf("unknown collaborator type: %s", req.Type)
	}
}

// ============================================================
// MÉTHODES PRIVÉES
// ============================================================

// removePlatformCollaborator supprime un collaborateur plateforme
func (uc *RemoveCollaboratorUsecase) removePlatformCollaborator(
	ctx context.Context,
	admin *AdminContext,
	req *RemoveCollaboratorRequest,
) (*RemoveCollaboratorResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Vérifier les permissions
	if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("❌ Permission refusée : pas super_admin ou admin")
		return nil, errors.New("insufficient permissions: only super_admin or admin can remove platform collaborators")
	}

	// 2. Parser l'ID
	collabID, err := uuid.Parse(req.CollaboratorID)
	if err != nil {
		return nil, errors.New("invalid collaborator_id format")
	}

	// 3. Récupérer le collaborateur existant
	collab, err := uc.platformCollabRepo.FindByID(ctx, collabID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération collaborateur")
		return nil, fmt.Errorf("find collaborator: %w", err)
	}
	if collab == nil {
		return nil, entity.ErrCollaboratorNotFound
	}

	// 4. Vérifier que le collaborateur n'est pas déjà supprimé
	if !collab.IsActive || collab.DeletedAt != nil {
		logger.Warn().
			Str("collaborator_id", collabID.String()).
			Msg("❌ Collaborateur déjà supprimé")
		return nil, errors.New("collaborator is already deleted")
	}

	// 5. Vérifier qu'on ne se supprime pas soi-même
	if collab.UserID == admin.AdminID {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Msg("❌ Interdit de se supprimer soi-même")
		return nil, errors.New("cannot delete yourself")
	}

	// 6. Sauvegarder les infos avant suppression (pour la réponse)
	oldRole := string(collab.Role)
	userID := collab.UserID

	// 7. Soft delete avec raison
	if err := uc.platformCollabRepo.Deactivate(ctx, collabID, admin.AdminID, req.Reason); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur suppression collaborateur")
		return nil, fmt.Errorf("deactivate collaborator: %w", err)
	}

	// 🆕 v4.3.1 : Ne pas refaire FindByID après suppression (retourne nil à cause du WHERE deleted_at IS NULL)
	// On utilise time.Now() pour le timestamp de suppression
	now := time.Now()
	deletedAtStr := now.Format("2006-01-02 15:04:05")

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("collaborator_id", collabID.String()).
		Str("user_id", userID).
		Str("old_role", oldRole).
		Str("reason", req.Reason).
		Msg("✅ Collaborateur plateforme supprimé")

	return &RemoveCollaboratorResponse{
		Success:        true,
		Message:        fmt.Sprintf("Collaborateur plateforme avec le rôle %s supprimé avec succès", oldRole),
		Type:           "platform",
		CollaboratorID: collabID.String(),
		UserID:         userID,
		OldRole:        oldRole,
		Reason:         req.Reason,
		DeletedAt:      deletedAtStr,
		DeletedBy:      admin.AdminID,
	}, nil
}

// removeShopCollaborator supprime un collaborateur boutique
func (uc *RemoveCollaboratorUsecase) removeShopCollaborator(
	ctx context.Context,
	admin *AdminContext,
	req *RemoveCollaboratorRequest,
) (*RemoveCollaboratorResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Parser le shopID
	if req.ShopID == "" {
		return nil, errors.New("shop_id is required for shop collaborator")
	}

	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		return nil, errors.New("invalid shop_id format")
	}

	// 2. Vérifier les permissions
	if admin.AdminRole != "super_admin" {
		// Vérifier que l'admin est shop_admin de cette boutique
		isAdmin, err := uc.shopCollabRepo.IsShopAdmin(ctx, admin.AdminID, shopID)
		if err != nil {
			logger.Error().Err(err).Msg("❌ Erreur vérification shop_admin")
			return nil, fmt.Errorf("check shop admin: %w", err)
		}
		if !isAdmin {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("shop_id", shopID.String()).
				Msg("❌ Permission refusée : pas shop_admin de cette boutique")
			return nil, errors.New("insufficient permissions: not shop_admin of this shop")
		}
	}

	// 3. Vérifier que la boutique existe
	shop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération boutique")
		return nil, fmt.Errorf("find shop: %w", err)
	}
	if shop == nil {
		return nil, errors.New("shop not found")
	}

	// 4. Parser l'ID du collaborateur
	collabID, err := uuid.Parse(req.CollaboratorID)
	if err != nil {
		return nil, errors.New("invalid collaborator_id format")
	}

	// 5. Récupérer le collaborateur existant
	collab, err := uc.shopCollabRepo.FindByID(ctx, collabID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération collaborateur")
		return nil, fmt.Errorf("find collaborator: %w", err)
	}
	if collab == nil {
		return nil, entity.ErrCollaboratorNotFound
	}

	// 6. Vérifier que le collaborateur appartient bien à cette boutique
	if collab.ShopID != shopID {
		logger.Warn().
			Str("collaborator_id", collabID.String()).
			Str("collaborator_shop_id", collab.ShopID.String()).
			Str("requested_shop_id", shopID.String()).
			Msg("❌ Collaborateur n'appartient pas à cette boutique")
		return nil, errors.New("collaborator does not belong to this shop")
	}

	// 7. Vérifier que le collaborateur n'est pas déjà supprimé
	if !collab.IsActive || collab.DeletedAt != nil {
		logger.Warn().
			Str("collaborator_id", collabID.String()).
			Msg("❌ Collaborateur déjà supprimé")
		return nil, errors.New("collaborator is already deleted")
	}

	// 8. Vérifier qu'on ne se supprime pas soi-même
	if collab.UserID == admin.AdminID {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Msg("❌ Interdit de se supprimer soi-même")
		return nil, errors.New("cannot delete yourself")
	}

	// 9. Vérifier qu'on ne supprime pas le dernier shop_admin
	if collab.Role == entity.ShopRoleShopAdmin {
		// Compter les shop_admin actifs de cette boutique
		shopAdmins, err := uc.shopCollabRepo.FindByRole(ctx, shopID, entity.ShopRoleShopAdmin)
		if err != nil {
			logger.Error().Err(err).Msg("❌ Erreur comptage shop_admin")
			return nil, fmt.Errorf("count shop admins: %w", err)
		}

		// Filtrer pour ne compter que les actifs et non supprimés
		activeShopAdmins := 0
		for _, admin := range shopAdmins {
			if admin.IsActive && admin.DeletedAt == nil {
				activeShopAdmins++
			}
		}

		if activeShopAdmins <= 1 {
			logger.Warn().
				Str("collaborator_id", collabID.String()).
				Str("shop_id", shopID.String()).
				Msg("❌ Impossible de supprimer le dernier shop_admin")
			return nil, errors.New("cannot delete the last shop_admin of a shop. Promote another collaborator to shop_admin first")
		}
	}

	// 10. Sauvegarder les infos avant suppression (pour la réponse)
	oldRole := string(collab.Role)
	userID := collab.UserID

	// 11. Soft delete avec raison
	if err := uc.shopCollabRepo.Deactivate(ctx, collabID, admin.AdminID, req.Reason); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur suppression collaborateur")
		return nil, fmt.Errorf("deactivate collaborator: %w", err)
	}

	// 🆕 v4.3.1 : Ne pas refaire FindByID après suppression (retourne nil à cause du WHERE deleted_at IS NULL)
	now := time.Now()
	deletedAtStr := now.Format("2006-01-02 15:04:05")

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("collaborator_id", collabID.String()).
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Str("user_id", userID).
		Str("old_role", oldRole).
		Str("reason", req.Reason).
		Msg("✅ Collaborateur boutique supprimé")

	return &RemoveCollaboratorResponse{
		Success:        true,
		Message:        fmt.Sprintf("Collaborateur avec le rôle %s supprimé de la boutique '%s' avec succès", oldRole, shop.Name),
		Type:           "shop",
		CollaboratorID: collabID.String(),
		UserID:         userID,
		OldRole:        oldRole,
		Reason:         req.Reason,
		DeletedAt:      deletedAtStr,
		DeletedBy:      admin.AdminID,
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *RemoveCollaboratorUsecase) validateRequest(req *RemoveCollaboratorRequest) error {
	if req.CollaboratorID == "" {
		return errors.New("collaborator_id is required")
	}

	if req.Type == "" {
		return errors.New("type is required (platform or shop)")
	}

	if req.Type != "platform" && req.Type != "shop" {
		return errors.New("type must be 'platform' or 'shop'")
	}

	if req.Reason == "" {
		return errors.New("reason is required")
	}

	// Raison minimum 10 caractères
	if len(req.Reason) < 10 {
		return fmt.Errorf("reason must be at least 10 characters (current: %d)", len(req.Reason))
	}

	// Raison maximum 1000 caractères
	if len(req.Reason) > 1000 {
		return fmt.Errorf("reason must be at most 1000 characters (current: %d)", len(req.Reason))
	}

	// ShopID requis si type = shop
	if req.Type == "shop" && req.ShopID == "" {
		return errors.New("shop_id is required when type is 'shop'")
	}

	return nil
}
