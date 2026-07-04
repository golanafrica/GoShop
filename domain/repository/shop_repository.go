package repository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_shop_repository.go -package=repository . ShopRepository
//go:generate mockgen -destination=../../mocks/repository/mock_shop_kyc_document_repository.go -package=repository . ShopKYCDocumentRepository

// ============================================================
// SHOP REPOSITORY (existant + extensions KYC)
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
	// Utilisé par le dashboard admin pour voir les shops en attente
	FindByKYCStatus(ctx context.Context, status entity.ShopKYCStatus, limit, offset int) ([]*entity.Shop, error)

	// CountByKYCStatus compte les shops par statut KYC
	// Utilisé pour les statistiques du dashboard admin
	CountByKYCStatus(ctx context.Context) (map[entity.ShopKYCStatus]int, error)

	// UpdateKYCStatus met à jour le statut KYC d'un shop
	// Utilisé par ApproveKYC et RejectKYC
	UpdateKYCStatus(
		ctx context.Context,
		shopID uuid.UUID,
		status entity.ShopKYCStatus,
		adminID string,
		rejectionReason *string,
	) error

	// FindAllShopsAdmin retourne tous les shops (cross-tenant) avec pagination
	// Utilisé par le dashboard admin pour gérer toutes les boutiques
	FindAllShopsAdmin(ctx context.Context, limit, offset int, filters *ShopAdminFilters) ([]*entity.Shop, int, error)

	// UpdateShopStatus active ou désactive une boutique (admin)
	UpdateShopStatus(ctx context.Context, shopID uuid.UUID, isActive bool, adminID string) error

	// UpdateShopPlan change le plan d'abonnement (super_admin uniquement)
	UpdateShopPlan(ctx context.Context, shopID uuid.UUID, plan entity.ShopPlan, adminID string) error

	WithTX(tx Tx) ShopRepository
}

// ============================================================
// 🆕 v4.1.0 : FILTRES ADMIN
// ============================================================

// ShopAdminFilters représente les filtres pour la recherche admin
type ShopAdminFilters struct {
	KYCStatus *entity.ShopKYCStatus
	Plan      *entity.ShopPlan
	IsActive  *bool
	Search    string // Recherche sur name/slug
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
