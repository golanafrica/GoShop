package repository

import (
	"context"
	"time"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_shop_repository.go -package=repository . ShopRepository
//go:generate mockgen -destination=../../mocks/repository/mock_shop_kyc_document_repository.go -package=repository . ShopKYCDocumentRepository
//go:generate mockgen -destination=../../mocks/repository/mock_shop_admin_action_repository.go -package=repository . ShopAdminActionRepository

// ============================================================
// SHOP REPOSITORY (existant + extensions KYC + Admin)
// ============================================================

// ShopRepository définit les opérations sur les boutiques
type ShopRepository interface {
	// ============ MÉTHODES EXISTANTES (ne pas modifier) ============
	Create(ctx context.Context, shop *entity.Shop) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Shop, error)
	FindBySlug(ctx context.Context, slug string) (*entity.Shop, error)
	FindByCustomDomain(ctx context.Context, domain string) (*entity.Shop, error)
	FindByOwnerID(ctx context.Context, ownerID string) ([]*entity.Shop, error)
	Update(ctx context.Context, shop *entity.Shop) error
	Deactivate(ctx context.Context, id uuid.UUID) error

	// Méthodes pour configuration paiement par boutique
	GetPaymentSettings(ctx context.Context, shopID uuid.UUID) (*entity.ShopPaymentSettings, error)
	UpsertPaymentSettings(ctx context.Context, settings *entity.ShopPaymentSettings) error
	IsOwner(ctx context.Context, shopID uuid.UUID, userID uuid.UUID) (bool, error)

	// ============ 🆕 v4.1.0 : MÉTHODES KYC MARCHAND ============

	// FindByKYCStatus retourne les shops filtrés par statut KYC
	FindByKYCStatus(ctx context.Context, status entity.ShopKYCStatus, limit, offset int) ([]*entity.Shop, error)

	// CountByKYCStatus compte les shops par statut KYC
	CountByKYCStatus(ctx context.Context) (map[entity.ShopKYCStatus]int, error)

	// UpdateKYCStatus met à jour le statut KYC d'un shop
	UpdateKYCStatus(
		ctx context.Context,
		shopID uuid.UUID,
		status entity.ShopKYCStatus,
		adminID string,
		rejectionReason *string,
	) error

	// ============ 🆕 v4.2.0 : MÉTHODES ADMIN SHOP MANAGEMENT ============

	// FindAllShopsAdmin retourne tous les shops (cross-tenant) avec pagination et filtres
	// Utilisé par le dashboard admin pour gérer toutes les boutiques
	FindAllShopsAdmin(ctx context.Context, filters *ShopAdminFilters) ([]*entity.Shop, int, error)

	// UpdateShopStatus active ou désactive une boutique (admin)
	UpdateShopStatus(ctx context.Context, shopID uuid.UUID, isActive bool, adminID string) error

	// UpdateShopPlan change le plan d'abonnement (super_admin uniquement)
	UpdateShopPlan(ctx context.Context, shopID uuid.UUID, plan entity.ShopPlan, adminID string) error

	// SuspendShop suspend une boutique avec raison (admin)
	SuspendShop(ctx context.Context, shopID uuid.UUID, adminID, reason string) error

	// ActivateShop réactive une boutique suspendue (admin)
	ActivateShop(ctx context.Context, shopID uuid.UUID, adminID string) error

	// UpdateHealthScore met à jour le score de santé d'un shop
	UpdateHealthScore(ctx context.Context, shopID uuid.UUID, score int, level entity.ShopHealthLevel) error

	// AddAdminNote ajoute une note admin à un shop
	AddAdminNote(ctx context.Context, shopID uuid.UUID, note string, adminID string) error

	// MarkShopReviewed marque un shop comme revu par un admin
	MarkShopReviewed(ctx context.Context, shopID uuid.UUID, adminID string) error

	// ============ 🆕 v4.2.0 : MÉTHODES DASHBOARD ADMIN ============

	// GetHealthStats retourne les statistiques de santé globales
	GetHealthStats(ctx context.Context) (*ShopHealthStats, error)

	// GetSuspendedShops retourne la liste des shops suspendus
	GetSuspendedShops(ctx context.Context, limit, offset int) ([]*entity.Shop, int, error)

	// GetCriticalShops retourne la liste des shops critiques (score < 400)
	GetCriticalShops(ctx context.Context, limit, offset int) ([]*entity.Shop, int, error)

	// GetShopsByHealthLevel retourne les shops par niveau de santé
	GetShopsByHealthLevel(ctx context.Context, level entity.ShopHealthLevel, limit, offset int) ([]*entity.Shop, int, error)

	// SearchShopsAdmin recherche des shops par nom/slug/email
	SearchShopsAdmin(ctx context.Context, query string, limit, offset int) ([]*entity.Shop, int, error)

	WithTX(tx Tx) ShopRepository
}

// ============================================================
// 🆕 v4.2.0 : FILTRES ADMIN
// ============================================================

// ShopAdminFilters représente les filtres pour la recherche admin
type ShopAdminFilters struct {
	// Filtres simples
	KYCStatus   *entity.ShopKYCStatus
	Plan        *entity.ShopPlan
	IsActive    *bool
	IsSuspended *bool

	// Filtre santé
	HealthLevel    *entity.ShopHealthLevel
	MinHealthScore *int
	MaxHealthScore *int

	// Filtres date
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time

	// Recherche texte
	Search string // Recherche sur name/slug/owner_email

	// Pagination
	Limit  int
	Offset int

	// Tri
	SortBy    string // "created_at", "updated_at", "health_score", "name"
	SortOrder string // "asc", "desc"
}

// ============================================================
// 🆕 v4.2.0 : STATS SANTÉ
// ============================================================

// ShopHealthStats représente les statistiques de santé globales
type ShopHealthStats struct {
	TotalShops     int `json:"total_shops"`
	ActiveShops    int `json:"active_shops"`
	InactiveShops  int `json:"inactive_shops"`
	SuspendedShops int `json:"suspended_shops"`
	PendingKYC     int `json:"pending_kyc"`

	// Distribution par niveau de santé
	ExcellentCount int `json:"excellent_count"`
	GoodCount      int `json:"good_count"`
	WarningCount   int `json:"warning_count"`
	CriticalCount  int `json:"critical_count"`

	// Scores
	AverageScore float64 `json:"average_score"`
	MinScore     int     `json:"min_score"`
	MaxScore     int     `json:"max_score"`

	// Distribution par plan
	FreeCount     int `json:"free_count"`
	ProCount      int `json:"pro_count"`
	BusinessCount int `json:"business_count"`

	// Activité admin
	ActionsLast24h int `json:"actions_last_24h"`
	ActionsLast7d  int `json:"actions_last_7d"`
}

// ============================================================
// 🆕 v4.1.0 : SHOP KYC DOCUMENT REPOSITORY
// ============================================================

// ShopKYCDocumentRepository définit les opérations sur les documents KYC marchands
type ShopKYCDocumentRepository interface {
	// Create crée un nouveau document KYC
	Create(ctx context.Context, doc *entity.ShopKYCDocument) error

	// FindByID retourne un document par son ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.ShopKYCDocument, error)

	// FindByShopID retourne tous les documents d'un shop
	FindByShopID(ctx context.Context, shopID uuid.UUID) ([]*entity.ShopKYCDocument, error)

	// FindPendingByShopID retourne les documents en attente d'un shop
	FindPendingByShopID(ctx context.Context, shopID uuid.UUID) ([]*entity.ShopKYCDocument, error)

	// FindByTypeAndShop retourne un document spécifique par type et shop
	FindByTypeAndShop(ctx context.Context, shopID uuid.UUID, docType entity.ShopKYCDocumentType) (*entity.ShopKYCDocument, error)

	// Update met à jour un document (statut, review, etc.)
	Update(ctx context.Context, doc *entity.ShopKYCDocument) error

	// Delete supprime un document
	Delete(ctx context.Context, id uuid.UUID) error

	// DeleteByShopID supprime tous les documents d'un shop (utilisé lors de re-soumission)
	DeleteByShopID(ctx context.Context, shopID uuid.UUID) error

	// CountByShopID compte les documents d'un shop
	CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error)

	// CountPendingByShopID compte les documents en attente d'un shop
	CountPendingByShopID(ctx context.Context, shopID uuid.UUID) (int, error)

	// FindAllPending retourne tous les documents en attente (dashboard admin)
	FindAllPending(ctx context.Context, limit, offset int) ([]*entity.ShopKYCDocument, error)

	WithTX(tx Tx) ShopKYCDocumentRepository
}

// ============================================================
// 🆕 v4.2.0 : SHOP ADMIN ACTION REPOSITORY (audit trail)
// ============================================================

// ShopAdminActionRepository définit les opérations sur les actions admin
type ShopAdminActionRepository interface {
	// Create crée une nouvelle action admin
	Create(ctx context.Context, action *entity.ShopAdminAction) error

	// FindByID retourne une action par son ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.ShopAdminAction, error)

	// FindByShopID retourne toutes les actions d'un shop
	FindByShopID(ctx context.Context, shopID uuid.UUID, limit, offset int) ([]*entity.ShopAdminAction, error)

	// FindByAdminID retourne toutes les actions d'un admin
	FindByAdminID(ctx context.Context, adminID string, limit, offset int) ([]*entity.ShopAdminAction, error)

	// FindByActionType retourne les actions par type
	FindByActionType(ctx context.Context, actionType entity.ShopAdminActionType, limit, offset int) ([]*entity.ShopAdminAction, error)

	// FindRecent retourne les actions récentes (tous shops)
	FindRecent(ctx context.Context, limit, offset int) ([]*entity.ShopAdminAction, error)

	// CountByShopID compte les actions d'un shop
	CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error)

	// CountByAdminID compte les actions d'un admin
	CountByAdminID(ctx context.Context, adminID string) (int, error)

	// CountRecent compte les actions récentes
	CountRecent(ctx context.Context, since time.Time) (int, error)

	// DeleteByShopID supprime toutes les actions d'un shop
	DeleteByShopID(ctx context.Context, shopID uuid.UUID) error

	WithTX(tx Tx) ShopAdminActionRepository
}

// ============================================================
// 🆕 v4.1.0 : DTOs KYC
// ============================================================

// SubmitShopKYCRequest représente la requête de soumission KYC
type SubmitShopKYCRequest struct {
	ShopID      uuid.UUID
	Documents   []DocumentUpload
	SubmittedBy string // ID du marchand
}

// DocumentUpload représente un document uploadé
type DocumentUpload struct {
	DocumentType  entity.ShopKYCDocumentType
	FilePath      string
	FileName      string
	FileSizeBytes int64
	MimeType      string
}

// ReviewShopKYCRequest représente la requête de revue KYC (admin)
type ReviewShopKYCRequest struct {
	ShopID          uuid.UUID
	Action          string // "approve" ou "reject"
	AdminID         string
	RejectionReason *string // Obligatoire si action = "reject"
	DocumentReviews []DocumentReview
}

// DocumentReview représente la revue d'un document individuel
type DocumentReview struct {
	DocumentID      uuid.UUID
	Action          string // "approve" ou "reject"
	RejectionReason *string
}

// ShopKYCStats représente les statistiques KYC
type ShopKYCStats struct {
	TotalShops      int
	UnverifiedCount int
	PendingCount    int
	VerifiedCount   int
	RejectedCount   int
}
