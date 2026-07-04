package merchantkycusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ============================================================
// GET MERCHANT KYC STATUS USECASE
// ============================================================
//
// 🎯 Objectif :
//   Le marchand consulte son statut KYC et ses documents.
//
// ============================================================

// GetMerchantKYCStatusUsecase récupère le statut KYC du marchand
type GetMerchantKYCStatusUsecase struct {
	shopRepo repository.ShopRepository
	kycRepo  repository.ShopKYCDocumentRepository
}

// NewGetMerchantKYCStatusUsecase crée une nouvelle instance
func NewGetMerchantKYCStatusUsecase(
	shopRepo repository.ShopRepository,
	kycRepo repository.ShopKYCDocumentRepository,
) *GetMerchantKYCStatusUsecase {
	return &GetMerchantKYCStatusUsecase{
		shopRepo: shopRepo,
		kycRepo:  kycRepo,
	}
}

// GetKYCStatusResponse représente la réponse
type GetKYCStatusResponse struct {
	ShopID              string           `json:"shop_id"`
	ShopName            string           `json:"shop_name"`
	KYCStatus           string           `json:"kyc_status"`
	KYCBadge            string           `json:"kyc_badge"`
	CanWithdraw         bool             `json:"can_withdraw"`
	KYCSubmittedAt      *time.Time       `json:"kyc_submitted_at,omitempty"`
	KYCVerifiedAt       *time.Time       `json:"kyc_verified_at,omitempty"`
	KYCRejectionReason  *string          `json:"kyc_rejection_reason,omitempty"`
	KYCSubmissionsCount int              `json:"kyc_submissions_count"`
	Documents           []DocumentItem   `json:"documents"`
	DocumentsSummary    DocumentsSummary `json:"documents_summary"`
}

// DocumentItem représente un document KYC
type DocumentItem struct {
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

// DocumentsSummary représente le résumé des documents
type DocumentsSummary struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
}

// Execute récupère le statut KYC
func (uc *GetMerchantKYCStatusUsecase) Execute(ctx context.Context) (*GetKYCStatusResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant context required: %w", err)
	}

	logger.Debug().
		Str("shop_id", shop.ID.String()).
		Str("kyc_status", string(shop.KYCStatus)).
		Msg("📊 Récupération statut KYC")

	// 2. Récupérer les documents
	documents, err := uc.kycRepo.FindByShopID(ctx, shop.ID)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("❌ Erreur récupération documents")
		return nil, fmt.Errorf("find documents: %w", err)
	}

	// 3. Construire la réponse
	items := make([]DocumentItem, 0, len(documents))
	summary := DocumentsSummary{}

	for _, doc := range documents {
		items = append(items, DocumentItem{
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
		})

		summary.Total++
		switch doc.Status {
		case "pending":
			summary.Pending++
		case "approved":
			summary.Approved++
		case "rejected":
			summary.Rejected++
		}
	}

	return &GetKYCStatusResponse{
		ShopID:              shop.ID.String(),
		ShopName:            shop.Name,
		KYCStatus:           string(shop.KYCStatus),
		KYCBadge:            shop.GetKYCBadge(),
		CanWithdraw:         shop.CanWithdraw(),
		KYCSubmittedAt:      shop.KYCSubmittedAt,
		KYCVerifiedAt:       shop.KYCVerifiedAt,
		KYCRejectionReason:  shop.KYCRejectionReason,
		KYCSubmissionsCount: shop.KYCSubmissionsCount,
		Documents:           items,
		DocumentsSummary:    summary,
	}, nil
}
