package adminshopusecase

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
// 🆕 v4.2.0 : GET SHOP DETAILS USECASE
// ============================================================
//
// 🎯 Objectif :
//   Récupérer les détails complets d'une boutique pour le dashboard admin.
//   Inspiré d'Amazon Seller Central > Shop Details.
//
// 📋 Informations retournées :
//   - Infos de base (ID, name, slug, plan, owner)
//   - Statut KYC + documents
//   - Health Score détaillé (score, level, emoji, description)
//   - Suspension (si applicable)
//   - Notes admin
//   - Historique revue
//
// 🔐 Sécurité :
//   - Réservé aux super_admin et admin
//   - Audit trail automatique
//
// ============================================================

// GetShopDetailsUsecase gère la récupération des détails d'une shop
type GetShopDetailsUsecase struct {
	shopRepo repository.ShopRepository
	kycRepo  repository.ShopKYCDocumentRepository
}

// NewGetShopDetailsUsecase crée une nouvelle instance
func NewGetShopDetailsUsecase(
	shopRepo repository.ShopRepository,
	kycRepo repository.ShopKYCDocumentRepository,
) *GetShopDetailsUsecase {
	return &GetShopDetailsUsecase{
		shopRepo: shopRepo,
		kycRepo:  kycRepo,
	}
}

// GetShopDetailsRequest représente la requête
type GetShopDetailsRequest struct {
	ShopID string `json:"shop_id"`
}

// GetShopDetailsResponse représente la réponse complète
type GetShopDetailsResponse struct {
	// Infos de base
	Shop *ShopDetails `json:"shop"`

	// KYC
	KYC *KYCDetails `json:"kyc,omitempty"`

	// Health Score
	Health *HealthDetails `json:"health"`

	// Suspension (si applicable)
	Suspension *SuspensionDetails `json:"suspension,omitempty"`

	// Notes admin
	AdminNotes *AdminNotesDetails `json:"admin_notes,omitempty"`

	// Revue admin
	Review *ReviewDetails `json:"review,omitempty"`
}

// ShopDetails contient les infos de base
type ShopDetails struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	OwnerID      string    `json:"owner_id"`
	Plan         string    `json:"plan"`
	IsActive     bool      `json:"is_active"`
	CustomDomain *string   `json:"custom_domain,omitempty"`
	LogoURL      *string   `json:"logo_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Statistiques basiques
	DaysSinceCreation int `json:"days_since_creation"`
}

// KYCDetails contient les infos KYC
type KYCDetails struct {
	Status            string             `json:"status"`
	Badge             string             `json:"badge"`
	CanWithdraw       bool               `json:"can_withdraw"`
	SubmittedAt       *time.Time         `json:"submitted_at,omitempty"`
	VerifiedAt        *time.Time         `json:"verified_at,omitempty"`
	VerifiedBy        *string            `json:"verified_by,omitempty"`
	RejectionReason   *string            `json:"rejection_reason,omitempty"`
	SubmissionsCount  int                `json:"submissions_count"`
	LastSubmissionAt  *time.Time         `json:"last_submission_at,omitempty"`
	Documents         []*KYCDocumentItem `json:"documents,omitempty"`
	DocumentsCount    int                `json:"documents_count"`
	DocumentsPending  int                `json:"documents_pending"`
	DocumentsApproved int                `json:"documents_approved"`
	DocumentsRejected int                `json:"documents_rejected"`
}

// KYCDocumentItem représente un document KYC
type KYCDocumentItem struct {
	ID              string     `json:"id"`
	DocumentType    string     `json:"document_type"`
	FileName        string     `json:"file_name"`
	FileSizeBytes   int64      `json:"file_size_bytes"`
	MimeType        string     `json:"mime_type"`
	Status          string     `json:"status"`
	ReviewedBy      *string    `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// HealthDetails contient les infos de santé
type HealthDetails struct {
	Score           int        `json:"score"`
	Level           string     `json:"level"`
	Emoji           string     `json:"emoji"`
	Description     string     `json:"description"`
	NeedsAction     bool       `json:"needs_action"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	DaysSinceUpdate int        `json:"days_since_update"`
}

// SuspensionDetails contient les infos de suspension
type SuspensionDetails struct {
	IsSuspended   bool       `json:"is_suspended"`
	SuspendedAt   *time.Time `json:"suspended_at,omitempty"`
	SuspendedBy   *string    `json:"suspended_by,omitempty"`
	Reason        *string    `json:"reason,omitempty"`
	DaysSuspended int        `json:"days_suspended"`
}

// AdminNotesDetails contient les notes admin
type AdminNotesDetails struct {
	Notes      *string `json:"notes,omitempty"`
	HasNotes   bool    `json:"has_notes"`
	NotesCount int     `json:"notes_count"` // Nombre de notes (approximatif)
}

// ReviewDetails contient les infos de revue
type ReviewDetails struct {
	LastReviewedAt  *time.Time `json:"last_reviewed_at,omitempty"`
	LastReviewedBy  *string    `json:"last_reviewed_by,omitempty"`
	DaysSinceReview int        `json:"days_since_review"`
	NeedsReview     bool       `json:"needs_review"` // true si jamais revu ou > 30 jours
}

// Execute récupère les détails d'une shop
func (uc *GetShopDetailsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *GetShopDetailsRequest,
) (*GetShopDetailsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider l'ID
	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Msg("❌ ID shop invalide")
		return nil, errors.New("invalid shop_id format")
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("shop_id", shopID.String()).
		Msg("🔍 Récupération détails shop")

	// 2. Récupérer la shop
	shop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération shop")
		return nil, fmt.Errorf("find shop: %w", err)
	}
	if shop == nil {
		logger.Warn().
			Str("shop_id", shopID.String()).
			Msg("❌ Shop non trouvée")
		return nil, errors.New("shop not found")
	}

	// 3. Récupérer les documents KYC
	documents, err := uc.kycRepo.FindByShopID(ctx, shopID)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur récupération documents KYC (non bloquant)")
		documents = nil
	}

	// 4. Construire la réponse
	response := &GetShopDetailsResponse{
		Shop:       uc.buildShopDetails(shop),
		KYC:        uc.buildKYCDetails(shop, documents),
		Health:     uc.buildHealthDetails(shop),
		Suspension: uc.buildSuspensionDetails(shop),
		AdminNotes: uc.buildAdminNotesDetails(shop),
		Review:     uc.buildReviewDetails(shop),
	}

	logger.Info().
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Str("health_level", string(shop.HealthLevel)).
		Msg("✅ Détails shop récupérés")

	return response, nil
}

// ============================================================
// BUILDERS
// ============================================================

func (uc *GetShopDetailsUsecase) buildShopDetails(shop *entity.Shop) *ShopDetails {
	return &ShopDetails{
		ID:                shop.ID.String(),
		Name:              shop.Name,
		Slug:              shop.Slug,
		OwnerID:           shop.OwnerID,
		Plan:              string(shop.Plan),
		IsActive:          shop.IsActive,
		CustomDomain:      shop.CustomDomain,
		LogoURL:           shop.LogoURL,
		CreatedAt:         shop.CreatedAt,
		UpdatedAt:         shop.UpdatedAt,
		DaysSinceCreation: int(time.Since(shop.CreatedAt).Hours() / 24),
	}
}

func (uc *GetShopDetailsUsecase) buildKYCDetails(
	shop *entity.Shop,
	documents []*entity.ShopKYCDocument,
) *KYCDetails {
	kyc := &KYCDetails{
		Status:           string(shop.KYCStatus),
		Badge:            shop.GetKYCBadge(),
		CanWithdraw:      shop.CanWithdraw(),
		SubmittedAt:      shop.KYCSubmittedAt,
		VerifiedAt:       shop.KYCVerifiedAt,
		VerifiedBy:       shop.KYCVerifiedBy,
		RejectionReason:  shop.KYCRejectionReason,
		SubmissionsCount: shop.KYCSubmissionsCount,
		LastSubmissionAt: shop.KYCLastSubmissionAt,
	}

	if len(documents) > 0 {
		kyc.Documents = make([]*KYCDocumentItem, len(documents))
		for i, doc := range documents {
			kyc.Documents[i] = &KYCDocumentItem{
				ID:              doc.ID.String(),
				DocumentType:    string(doc.DocumentType),
				FileName:        doc.FileName,
				FileSizeBytes:   doc.FileSizeBytes,
				MimeType:        doc.MimeType,
				Status:          doc.Status,
				ReviewedBy:      doc.ReviewedBy,
				ReviewedAt:      doc.ReviewedAt,
				RejectionReason: doc.RejectionReason,
				CreatedAt:       doc.CreatedAt,
			}

			// Compter par statut
			kyc.DocumentsCount++
			switch doc.Status {
			case "pending":
				kyc.DocumentsPending++
			case "approved":
				kyc.DocumentsApproved++
			case "rejected":
				kyc.DocumentsRejected++
			}
		}
	}

	return kyc
}

func (uc *GetShopDetailsUsecase) buildHealthDetails(shop *entity.Shop) *HealthDetails {
	daysSinceUpdate := 0
	if shop.HealthUpdatedAt != nil {
		daysSinceUpdate = int(time.Since(*shop.HealthUpdatedAt).Hours() / 24)
	}

	return &HealthDetails{
		Score:           shop.HealthScore,
		Level:           string(shop.HealthLevel),
		Emoji:           shop.GetHealthEmoji(),
		Description:     shop.GetHealthDescription(),
		NeedsAction:     shop.NeedsImmediateAction(),
		UpdatedAt:       shop.HealthUpdatedAt,
		DaysSinceUpdate: daysSinceUpdate,
	}
}

func (uc *GetShopDetailsUsecase) buildSuspensionDetails(shop *entity.Shop) *SuspensionDetails {
	if !shop.IsSuspended() {
		return nil
	}

	daysSuspended := 0
	if shop.SuspendedAt != nil {
		daysSuspended = int(time.Since(*shop.SuspendedAt).Hours() / 24)
	}

	return &SuspensionDetails{
		IsSuspended:   true,
		SuspendedAt:   shop.SuspendedAt,
		SuspendedBy:   shop.SuspendedBy,
		Reason:        shop.SuspensionReason,
		DaysSuspended: daysSuspended,
	}
}

func (uc *GetShopDetailsUsecase) buildAdminNotesDetails(shop *entity.Shop) *AdminNotesDetails {
	if shop.AdminNotes == nil || *shop.AdminNotes == "" {
		return &AdminNotesDetails{
			HasNotes:   false,
			NotesCount: 0,
		}
	}

	// Compter approximativement le nombre de notes (basé sur les timestamps)
	notesCount := 1
	for i := 0; i < len(*shop.AdminNotes)-1; i++ {
		if (*shop.AdminNotes)[i] == '[' && (*shop.AdminNotes)[i+1] == '2' {
			notesCount++
		}
	}

	return &AdminNotesDetails{
		Notes:      shop.AdminNotes,
		HasNotes:   true,
		NotesCount: notesCount,
	}
}

func (uc *GetShopDetailsUsecase) buildReviewDetails(shop *entity.Shop) *ReviewDetails {
	daysSinceReview := -1
	needsReview := true

	if shop.LastReviewedAt != nil {
		daysSinceReview = int(time.Since(*shop.LastReviewedAt).Hours() / 24)
		needsReview = daysSinceReview > 30 // Revue requise si > 30 jours
	}

	return &ReviewDetails{
		LastReviewedAt:  shop.LastReviewedAt,
		LastReviewedBy:  shop.LastReviewedBy,
		DaysSinceReview: daysSinceReview,
		NeedsReview:     needsReview,
	}
}
