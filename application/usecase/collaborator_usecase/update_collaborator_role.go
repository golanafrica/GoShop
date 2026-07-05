package collaboratorusecase

import (
	"context"
	"errors"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.3.0 : UPDATE COLLABORATOR ROLE USECASE
// ============================================================
//
// 🎯 Objectif :
//   Changer le rôle d'un collaborateur (plateforme OU boutique)
//   avec vérification des permissions et audit trail.
//
// 📋 Workflow :
//   1. Valider la requête (ID, type, nouveau rôle)
//   2. Vérifier les permissions
//   3. Récupérer le collaborateur existant
//   4. Vérifier que le nouveau rôle est valide
//   5. Mettre à jour le rôle et les permissions
//   6. Logger l'action (audit trail)
//   7. Retourner le collaborateur mis à jour
//
// 🔐 Sécurité :
//   - Super admin : peut changer tous les rôles
//   - Admin : peut changer les rôles plateforme
//   - Shop admin : peut changer les rôles de SA boutique
//   - Interdit de se rétrograder soi-même
//
// ============================================================

// UpdateCollaboratorRoleUsecase gère le changement de rôle d'un collaborateur
type UpdateCollaboratorRoleUsecase struct {
	platformCollabRepo repository.PlatformCollaboratorRepository
	shopCollabRepo     repository.ShopCollaboratorRepository
	shopRepo           repository.ShopRepository
}

// NewUpdateCollaboratorRoleUsecase crée une nouvelle instance
func NewUpdateCollaboratorRoleUsecase(
	platformCollabRepo repository.PlatformCollaboratorRepository,
	shopCollabRepo repository.ShopCollaboratorRepository,
	shopRepo repository.ShopRepository,
) *UpdateCollaboratorRoleUsecase {
	return &UpdateCollaboratorRoleUsecase{
		platformCollabRepo: platformCollabRepo,
		shopCollabRepo:     shopCollabRepo,
		shopRepo:           shopRepo,
	}
}

// UpdateCollaboratorRoleRequest représente la requête
type UpdateCollaboratorRoleRequest struct {
	CollaboratorID string `json:"collaborator_id"`
	Type           string `json:"type"` // "platform" ou "shop"
	NewRole        string `json:"new_role"`
	ShopID         string `json:"shop_id,omitempty"` // Requis si type = "shop"
}

// UpdateCollaboratorRoleResponse représente la réponse
type UpdateCollaboratorRoleResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	Type           string `json:"type"`
	CollaboratorID string `json:"collaborator_id"`
	OldRole        string `json:"old_role"`
	NewRole        string `json:"new_role"`
	UpdatedAt      string `json:"updated_at"`
}

// Execute change le rôle d'un collaborateur
func (uc *UpdateCollaboratorRoleUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *UpdateCollaboratorRoleRequest,
) (*UpdateCollaboratorRoleResponse, error) {
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
		Str("new_role", req.NewRole).
		Msg("🔄 Changement de rôle collaborateur")

	// 2. Dispatch selon le type
	switch req.Type {
	case "platform":
		return uc.updatePlatformRole(ctx, admin, req)
	case "shop":
		return uc.updateShopRole(ctx, admin, req)
	default:
		return nil, fmt.Errorf("unknown collaborator type: %s", req.Type)
	}
}

// ============================================================
// MÉTHODES PRIVÉES
// ============================================================

// updatePlatformRole change le rôle d'un collaborateur plateforme
func (uc *UpdateCollaboratorRoleUsecase) updatePlatformRole(
	ctx context.Context,
	admin *AdminContext,
	req *UpdateCollaboratorRoleRequest,
) (*UpdateCollaboratorRoleResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Vérifier les permissions
	if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("❌ Permission refusée : pas super_admin ou admin")
		return nil, errors.New("insufficient permissions: only super_admin or admin can update platform roles")
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

	// 4. Vérifier qu'on ne se rétrograde pas soi-même
	if collab.UserID == admin.AdminID {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Msg("❌ Interdit de changer son propre rôle")
		return nil, errors.New("cannot change your own role")
	}

	// 5. Vérifier que le nouveau rôle est valide
	newRole := entity.PlatformRole(req.NewRole)
	if !entity.IsValidPlatformRole(newRole) {
		return nil, entity.ErrInvalidPlatformRole
	}

	// 6. Sauvegarder l'ancien rôle
	oldRole := string(collab.Role)

	// 7. Obtenir les permissions par défaut pour le nouveau rôle
	newPermissions := entity.DefaultPlatformPermissions(newRole)

	// 8. Mettre à jour le rôle et les permissions
	if err := uc.platformCollabRepo.UpdateRole(ctx, collabID, newRole, newPermissions); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur mise à jour rôle")
		return nil, fmt.Errorf("update role: %w", err)
	}

	// 9. Récupérer le collaborateur mis à jour
	updatedCollab, err := uc.platformCollabRepo.FindByID(ctx, collabID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération collaborateur mis à jour")
		return nil, fmt.Errorf("find updated collaborator: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("collaborator_id", collabID.String()).
		Str("old_role", oldRole).
		Str("new_role", req.NewRole).
		Msg("✅ Rôle collaborateur plateforme mis à jour")

	return &UpdateCollaboratorRoleResponse{
		Success:        true,
		Message:        fmt.Sprintf("Rôle changé de %s à %s avec succès", oldRole, req.NewRole),
		Type:           "platform",
		CollaboratorID: collabID.String(),
		OldRole:        oldRole,
		NewRole:        req.NewRole,
		UpdatedAt:      updatedCollab.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// updateShopRole change le rôle d'un collaborateur boutique
func (uc *UpdateCollaboratorRoleUsecase) updateShopRole(
	ctx context.Context,
	admin *AdminContext,
	req *UpdateCollaboratorRoleRequest,
) (*UpdateCollaboratorRoleResponse, error) {
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

	// 7. Vérifier qu'on ne se rétrograde pas soi-même
	if collab.UserID == admin.AdminID {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Msg("❌ Interdit de changer son propre rôle")
		return nil, errors.New("cannot change your own role")
	}

	// 8. Vérifier que le nouveau rôle est valide
	newRole := entity.ShopRole(req.NewRole)
	if !entity.IsValidShopRole(newRole) {
		return nil, entity.ErrInvalidShopRole
	}

	// 9. Sauvegarder l'ancien rôle
	oldRole := string(collab.Role)

	// 10. Obtenir les permissions par défaut pour le nouveau rôle
	newPermissions := entity.DefaultShopPermissions(newRole)

	// 11. Mettre à jour le rôle et les permissions
	if err := uc.shopCollabRepo.UpdateRole(ctx, collabID, newRole, newPermissions); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur mise à jour rôle")
		return nil, fmt.Errorf("update role: %w", err)
	}

	// 12. Récupérer le collaborateur mis à jour
	updatedCollab, err := uc.shopCollabRepo.FindByID(ctx, collabID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération collaborateur mis à jour")
		return nil, fmt.Errorf("find updated collaborator: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("collaborator_id", collabID.String()).
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Str("old_role", oldRole).
		Str("new_role", req.NewRole).
		Msg("✅ Rôle collaborateur boutique mis à jour")

	return &UpdateCollaboratorRoleResponse{
		Success:        true,
		Message:        fmt.Sprintf("Rôle changé de %s à %s dans la boutique '%s' avec succès", oldRole, req.NewRole, shop.Name),
		Type:           "shop",
		CollaboratorID: collabID.String(),
		OldRole:        oldRole,
		NewRole:        req.NewRole,
		UpdatedAt:      updatedCollab.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *UpdateCollaboratorRoleUsecase) validateRequest(req *UpdateCollaboratorRoleRequest) error {
	if req.CollaboratorID == "" {
		return errors.New("collaborator_id is required")
	}

	if req.Type == "" {
		return errors.New("type is required (platform or shop)")
	}

	if req.Type != "platform" && req.Type != "shop" {
		return errors.New("type must be 'platform' or 'shop'")
	}

	if req.NewRole == "" {
		return errors.New("new_role is required")
	}

	// ShopID requis si type = shop
	if req.Type == "shop" && req.ShopID == "" {
		return errors.New("shop_id is required when type is 'shop'")
	}

	return nil
}
