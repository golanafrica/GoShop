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
// 🆕 v4.3.0 : LIST COLLABORATORS USECASE
// ============================================================

// ListCollaboratorsUsecase gère la liste des collaborateurs
type ListCollaboratorsUsecase struct {
	platformCollabRepo repository.PlatformCollaboratorRepository
	shopCollabRepo     repository.ShopCollaboratorRepository
	shopRepo           repository.ShopRepository
}

// NewListCollaboratorsUsecase crée une nouvelle instance
func NewListCollaboratorsUsecase(
	platformCollabRepo repository.PlatformCollaboratorRepository,
	shopCollabRepo repository.ShopCollaboratorRepository,
	shopRepo repository.ShopRepository,
) *ListCollaboratorsUsecase {
	return &ListCollaboratorsUsecase{
		platformCollabRepo: platformCollabRepo,
		shopCollabRepo:     shopCollabRepo,
		shopRepo:           shopRepo,
	}
}

// ListCollaboratorsRequest représente la requête
type ListCollaboratorsRequest struct {
	Type      string `json:"type"`
	ShopID    string `json:"shop_id"`
	Role      string `json:"role"`
	IsActive  *bool  `json:"is_active"`
	Search    string `json:"search"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"`
}

// ListCollaboratorsResponse représente la réponse
type ListCollaboratorsResponse struct {
	Success       bool                   `json:"success"`
	Type          string                 `json:"type"`
	Collaborators []CollaboratorListItem `json:"collaborators"`
	Total         int                    `json:"total"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
	Stats         *CollaboratorStats     `json:"stats,omitempty"`
}

// CollaboratorListItem représente un collaborateur dans la liste
type CollaboratorListItem struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Email       string  `json:"email,omitempty"`
	Role        string  `json:"role"`
	Permissions string  `json:"permissions"`
	IsActive    bool    `json:"is_active"`
	InvitedBy   string  `json:"invited_by"`
	InvitedAt   string  `json:"invited_at"`
	AcceptedAt  *string `json:"accepted_at,omitempty"`
	LastLoginAt *string `json:"last_login_at,omitempty"`
	CreatedAt   string  `json:"created_at"`

	// Pour shop collaborator
	ShopID   *string `json:"shop_id,omitempty"`
	ShopName *string `json:"shop_name,omitempty"`
}

// CollaboratorStats contient les statistiques
type CollaboratorStats struct {
	TotalActive   int            `json:"total_active"`
	TotalInactive int            `json:"total_inactive"`
	ByRole        map[string]int `json:"by_role"`

	// Pour shop collaborator
	ShopID   *string `json:"shop_id,omitempty"`
	ShopName *string `json:"shop_name,omitempty"`
}

// Execute liste les collaborateurs
func (uc *ListCollaboratorsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *ListCollaboratorsRequest,
) (*ListCollaboratorsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if err := uc.validateRequest(req); err != nil {
		logger.Warn().Err(err).Msg("❌ Validation requête échouée")
		return nil, err
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("type", req.Type).
		Str("shop_id", req.ShopID).
		Int("limit", req.Limit).
		Int("offset", req.Offset).
		Msg("📋 Liste des collaborateurs")

	// 2. Vérifier les permissions
	if err := uc.checkPermissions(ctx, admin, req); err != nil {
		logger.Warn().Err(err).Msg("❌ Permission refusée")
		return nil, err
	}

	// 3. Dispatch selon le type
	switch req.Type {
	case "platform":
		return uc.listPlatformCollaborators(ctx, admin, req)
	case "shop":
		return uc.listShopCollaborators(ctx, admin, req)
	default:
		return nil, fmt.Errorf("unknown collaborator type: %s", req.Type)
	}
}

// ============================================================
// MÉTHODES PRIVÉES
// ============================================================

func (uc *ListCollaboratorsUsecase) listPlatformCollaborators(
	ctx context.Context,
	admin *AdminContext,
	req *ListCollaboratorsRequest,
) (*ListCollaboratorsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 🆕 v4.3.0 : Log de début avec admin pour traçabilité
	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Msg("📋 Début liste collaborateurs plateforme")

	var collaborators []*entity.PlatformCollaborator
	var total int
	var err error

	if req.Role != "" {
		role := entity.PlatformRole(req.Role)
		collaborators, total, err = uc.platformCollabRepo.FindByRole(ctx, role, req.Limit, req.Offset)
	} else {
		collaborators, total, err = uc.platformCollabRepo.FindAll(ctx, req.Limit, req.Offset)
	}

	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération collaborateurs plateforme")
		return nil, fmt.Errorf("find platform collaborators: %w", err)
	}

	countByRole, err := uc.platformCollabRepo.CountByRole(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur comptage par rôle")
		countByRole = make(map[entity.PlatformRole]int)
	}

	totalActive, err := uc.platformCollabRepo.CountActive(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur comptage actifs")
		totalActive = 0
	}

	items := make([]CollaboratorListItem, len(collaborators))
	for i, collab := range collaborators {
		permJSON, err := collab.Permissions.ToJSON()
		if err != nil {
			logger.Warn().Err(err).Msg("⚠️ Erreur sérialisation permissions")
			permJSON = []byte("{}")
		}

		item := CollaboratorListItem{
			ID:          collab.ID.String(),
			UserID:      collab.UserID,
			Role:        string(collab.Role),
			Permissions: string(permJSON),
			IsActive:    collab.IsActive,
			InvitedBy:   collab.InvitedBy,
			InvitedAt:   collab.InvitedAt.Format("2006-01-02 15:04:05"),
			CreatedAt:   collab.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		if collab.AcceptedAt != nil {
			acceptedStr := collab.AcceptedAt.Format("2006-01-02 15:04:05")
			item.AcceptedAt = &acceptedStr
		}

		if collab.LastLoginAt != nil {
			lastLoginStr := collab.LastLoginAt.Format("2006-01-02 15:04:05")
			item.LastLoginAt = &lastLoginStr
		}

		items[i] = item
	}

	statsByRole := make(map[string]int)
	for role, count := range countByRole {
		statsByRole[string(role)] = count
	}

	stats := &CollaboratorStats{
		TotalActive:   totalActive,
		TotalInactive: total - totalActive,
		ByRole:        statsByRole,
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Int("total", total).
		Int("count", len(items)).
		Msg("✅ Liste collaborateurs plateforme récupérée")

	return &ListCollaboratorsResponse{
		Success:       true,
		Type:          "platform",
		Collaborators: items,
		Total:         total,
		Limit:         req.Limit,
		Offset:        req.Offset,
		Stats:         stats,
	}, nil
}

func (uc *ListCollaboratorsUsecase) listShopCollaborators(
	ctx context.Context,
	admin *AdminContext,
	req *ListCollaboratorsRequest,
) (*ListCollaboratorsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 🆕 v4.3.0 : Log de début avec admin pour traçabilité
	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("shop_id", req.ShopID).
		Msg("📋 Début liste collaborateurs boutique")

	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		return nil, errors.New("invalid shop_id format")
	}

	shop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération boutique")
		return nil, fmt.Errorf("find shop: %w", err)
	}
	if shop == nil {
		return nil, errors.New("shop not found")
	}

	var collaborators []*entity.ShopCollaborator
	var total int

	if req.Role != "" {
		role := entity.ShopRole(req.Role)
		collaborators, err = uc.shopCollabRepo.FindByRole(ctx, shopID, role)
		if err != nil {
			return nil, fmt.Errorf("find by role: %w", err)
		}
		total = len(collaborators)

		if req.Offset >= len(collaborators) {
			collaborators = []*entity.ShopCollaborator{}
		} else {
			end := req.Offset + req.Limit
			if end > len(collaborators) {
				end = len(collaborators)
			}
			collaborators = collaborators[req.Offset:end]
		}
	} else {
		collaborators, total, err = uc.shopCollabRepo.FindByShopID(ctx, shopID, req.Limit, req.Offset)
		if err != nil {
			return nil, fmt.Errorf("find by shop: %w", err)
		}
	}

	totalActive, err := uc.shopCollabRepo.CountByShopID(ctx, shopID)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur comptage actifs")
		totalActive = 0
	}

	items := make([]CollaboratorListItem, len(collaborators))
	for i, collab := range collaborators {
		permJSON, err := collab.Permissions.ToJSON()
		if err != nil {
			logger.Warn().Err(err).Msg("⚠️ Erreur sérialisation permissions")
			permJSON = []byte("{}")
		}

		item := CollaboratorListItem{
			ID:          collab.ID.String(),
			UserID:      collab.UserID,
			Role:        string(collab.Role),
			Permissions: string(permJSON),
			IsActive:    collab.IsActive,
			InvitedBy:   collab.InvitedBy,
			InvitedAt:   collab.InvitedAt.Format("2006-01-02 15:04:05"),
			CreatedAt:   collab.CreatedAt.Format("2006-01-02 15:04:05"),
			ShopID:      strPtr(collab.ShopID.String()),
			ShopName:    strPtr(shop.Name),
		}

		if collab.AcceptedAt != nil {
			acceptedStr := collab.AcceptedAt.Format("2006-01-02 15:04:05")
			item.AcceptedAt = &acceptedStr
		}

		if collab.LastLoginAt != nil {
			lastLoginStr := collab.LastLoginAt.Format("2006-01-02 15:04:05")
			item.LastLoginAt = &lastLoginStr
		}

		items[i] = item
	}

	stats := &CollaboratorStats{
		TotalActive:   totalActive,
		TotalInactive: total - totalActive,
		ByRole:        make(map[string]int),
		ShopID:        strPtr(shopID.String()),
		ShopName:      strPtr(shop.Name),
	}

	for _, collab := range collaborators {
		stats.ByRole[string(collab.Role)]++
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Int("total", total).
		Int("count", len(items)).
		Msg("✅ Liste collaborateurs boutique récupérée")

	return &ListCollaboratorsResponse{
		Success:       true,
		Type:          "shop",
		Collaborators: items,
		Total:         total,
		Limit:         req.Limit,
		Offset:        req.Offset,
		Stats:         stats,
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *ListCollaboratorsUsecase) validateRequest(req *ListCollaboratorsRequest) error {
	if req.Type == "" {
		return errors.New("type is required (platform or shop)")
	}

	if req.Type != "platform" && req.Type != "shop" {
		return errors.New("type must be 'platform' or 'shop'")
	}

	if req.Type == "shop" && req.ShopID == "" {
		return errors.New("shop_id is required when type is 'shop'")
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	if req.SortOrder != "asc" && req.SortOrder != "desc" {
		return errors.New("sort_order must be 'asc' or 'desc'")
	}

	return nil
}

// ============================================================
// PERMISSIONS
// ============================================================

func (uc *ListCollaboratorsUsecase) checkPermissions(
	ctx context.Context,
	admin *AdminContext,
	req *ListCollaboratorsRequest,
) error {
	if admin.AdminRole == "super_admin" {
		return nil
	}

	if admin.AdminRole == "admin" && req.Type == "platform" {
		return nil
	}

	if admin.AdminRole == "merchant" && req.Type == "shop" {
		shopID, err := uuid.Parse(req.ShopID)
		if err != nil {
			return errors.New("invalid shop_id format")
		}

		isAdmin, err := uc.shopCollabRepo.IsShopAdmin(ctx, admin.AdminID, shopID)
		if err != nil {
			return fmt.Errorf("check shop admin: %w", err)
		}
		if !isAdmin {
			return errors.New("insufficient permissions: not shop_admin of this shop")
		}
		return nil
	}

	return errors.New("insufficient permissions")
}
