package adminshopusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.2.0 : ADMIN CONTEXT
// ============================================================

// AdminContext contient les informations de l'admin qui effectue l'action
type AdminContext struct {
	AdminID    string `json:"admin_id"`
	AdminEmail string `json:"admin_email"`
	AdminRole  string `json:"admin_role"`
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	RequestID  string `json:"request_id"`
}

// ============================================================
// 🆕 v4.2.0 : LIST SHOPS USECASE
// ============================================================
//
// 🎯 Objectif :
//   Lister toutes les shops de la plateforme (cross-tenant) avec filtres avancés.
//   Inspiré d'Amazon Seller Central > All Shops.
//
// 📋 Fonctionnalités :
//   - Pagination (limit/offset)
//   - Filtres : KYC status, plan, is_active, health_level, search
//   - Tri : created_at, health_score, name
//   - Statistiques globales incluses
//
// 🔐 Sécurité :
//   - Réservé aux super_admin et admin
//   - Audit trail automatique
//
// ============================================================

// ListShopsUsecase gère la liste cross-tenant des shops
type ListShopsUsecase struct {
	shopRepo repository.ShopRepository
}

// NewListShopsUsecase crée une nouvelle instance
func NewListShopsUsecase(shopRepo repository.ShopRepository) *ListShopsUsecase {
	return &ListShopsUsecase{shopRepo: shopRepo}
}

// ListShopsRequest représente la requête
type ListShopsRequest struct {
	// Filtres
	KYCStatus   *entity.ShopKYCStatus   `json:"kyc_status,omitempty"`
	Plan        *entity.ShopPlan        `json:"plan,omitempty"`
	IsActive    *bool                   `json:"is_active,omitempty"`
	IsSuspended *bool                   `json:"is_suspended,omitempty"`
	HealthLevel *entity.ShopHealthLevel `json:"health_level,omitempty"`
	Search      string                  `json:"search,omitempty"`

	// Pagination
	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	// Tri
	SortBy    string `json:"sort_by"`    // created_at, health_score, name
	SortOrder string `json:"sort_order"` // asc, desc
}

// ListShopsResponse représente la réponse
type ListShopsResponse struct {
	Shops  []*ShopListItem             `json:"shops"`
	Total  int                         `json:"total"`
	Limit  int                         `json:"limit"`
	Offset int                         `json:"offset"`
	Stats  *repository.ShopHealthStats `json:"stats"`
}

// ShopListItem représente une shop dans la liste
type ShopListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	OwnerID     string    `json:"owner_id"`
	Plan        string    `json:"plan"`
	IsActive    bool      `json:"is_active"`
	IsSuspended bool      `json:"is_suspended"`
	KYCStatus   string    `json:"kyc_status"`
	HealthScore int       `json:"health_score"`
	HealthLevel string    `json:"health_level"`
	HealthEmoji string    `json:"health_emoji"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Execute liste toutes les shops
func (uc *ListShopsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *ListShopsRequest,
) (*ListShopsResponse, error) {
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Int("limit", req.Limit).
		Int("offset", req.Offset).
		Msg("📋 Liste cross-tenant des shops")

	// 1. Validation des paramètres
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
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

	// 2. Construire les filtres
	filters := &repository.ShopAdminFilters{
		KYCStatus:   req.KYCStatus,
		Plan:        req.Plan,
		IsActive:    req.IsActive,
		IsSuspended: req.IsSuspended,
		HealthLevel: req.HealthLevel,
		Search:      req.Search,
		Limit:       req.Limit,
		Offset:      req.Offset,
		SortBy:      req.SortBy,
		SortOrder:   req.SortOrder,
	}

	// 3. Récupérer les shops
	shops, total, err := uc.shopRepo.FindAllShopsAdmin(ctx, filters)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération shops")
		return nil, fmt.Errorf("find all shops: %w", err)
	}

	// 4. Récupérer les statistiques
	stats, err := uc.shopRepo.GetHealthStats(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur récupération stats (non bloquant)")
		stats = nil
	}

	// 5. Convertir en ShopListItem
	items := make([]*ShopListItem, len(shops))
	for i, shop := range shops {
		items[i] = &ShopListItem{
			ID:          shop.ID.String(),
			Name:        shop.Name,
			Slug:        shop.Slug,
			OwnerID:     shop.OwnerID,
			Plan:        string(shop.Plan),
			IsActive:    shop.IsActive,
			IsSuspended: shop.IsSuspended(),
			KYCStatus:   string(shop.KYCStatus),
			HealthScore: shop.HealthScore,
			HealthLevel: string(shop.HealthLevel),
			HealthEmoji: shop.GetHealthEmoji(),
			CreatedAt:   shop.CreatedAt,
			UpdatedAt:   shop.UpdatedAt,
		}
	}

	logger.Info().
		Int("shops_count", len(items)).
		Int("total", total).
		Msg("✅ Liste des shops récupérée")

	return &ListShopsResponse{
		Shops:  items,
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
		Stats:  stats,
	}, nil
}
