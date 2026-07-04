package merchantkycusecase

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
// REVIEW MERCHANT KYC USECASE (Admin)
// ============================================================
//
// 🎯 Objectif :
//   Un admin (super_admin, admin, collaborator, support_agent)
//   approuve ou rejette le KYC d'un marchand.
//
// 📋 Règles :
//   - Le shop doit être en statut "pending"
//   - L'action doit être "approve" ou "reject"
//   - Si "reject", une raison est obligatoire
//   - Audit trail : admin_id, timestamp, raison
//
// ============================================================

// ReviewMerchantKYCUsecase gère l'approbation/rejet KYC par admin
type ReviewMerchantKYCUsecase struct {
	shopRepo repository.ShopRepository
	kycRepo  repository.ShopKYCDocumentRepository
}

// NewReviewMerchantKYCUsecase crée une nouvelle instance
func NewReviewMerchantKYCUsecase(
	shopRepo repository.ShopRepository,
	kycRepo repository.ShopKYCDocumentRepository,
) *ReviewMerchantKYCUsecase {
	return &ReviewMerchantKYCUsecase{
		shopRepo: shopRepo,
		kycRepo:  kycRepo,
	}
}

// ReviewMerchantKYCRequest représente la requête de revue KYC
type ReviewMerchantKYCRequest struct {
	ShopID          string           `json:"shop_id"`
	Action          string           `json:"action"` // "approve" ou "reject"
	AdminID         string           `json:"admin_id"`
	RejectionReason *string          `json:"rejection_reason,omitempty"`
	DocumentReviews []DocumentReview `json:"document_reviews,omitempty"`
}

// DocumentReview représente la revue d'un document individuel
type DocumentReview struct {
	DocumentID      string  `json:"document_id"`
	Action          string  `json:"action"` // "approve" ou "reject"
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// ReviewMerchantKYCResponse représente la réponse
type ReviewMerchantKYCResponse struct {
	ShopID         string     `json:"shop_id"`
	ShopName       string     `json:"shop_name"`
	KYCStatus      string     `json:"kyc_status"`
	KYCVerifiedAt  *time.Time `json:"kyc_verified_at,omitempty"`
	KYCVerifiedBy  string     `json:"kyc_verified_by"`
	DocumentsCount int        `json:"documents_count"`
	Message        string     `json:"message"`
}

// Execute approuve ou rejette le KYC
func (uc *ReviewMerchantKYCUsecase) Execute(
	ctx context.Context,
	req *ReviewMerchantKYCRequest,
) (*ReviewMerchantKYCResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if err := uc.validateRequest(req); err != nil {
		return nil, err
	}

	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		return nil, errors.New("invalid shop_id format")
	}

	logger.Info().
		Str("shop_id", shopID.String()).
		Str("action", req.Action).
		Str("admin_id", req.AdminID).
		Msg("🔍 Début revue KYC marchand")

	// 2. Récupérer le shop
	shop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().
			Err(err).
			Str("shop_id", shopID.String()).
			Msg("❌ Shop non trouvé")
		return nil, fmt.Errorf("shop not found: %w", err)
	}
	if shop == nil {
		return nil, errors.New("shop not found")
	}

	// 3. Vérifier que le shop est en statut "pending"
	if !shop.IsPendingKYC() {
		logger.Warn().
			Str("shop_id", shopID.String()).
			Str("current_status", string(shop.KYCStatus)).
			Msg("❌ Shop n'est pas en statut pending")
		return nil, fmt.Errorf("shop KYC is not pending (current: %s)", shop.KYCStatus)
	}

	// 4. Déterminer le nouveau statut
	var newStatus entity.ShopKYCStatus
	var message string

	switch req.Action {
	case "approve":
		newStatus = entity.ShopKYCStatusVerified
		message = "KYC approuvé. Le marchand peut maintenant effectuer des retraits."
	case "reject":
		newStatus = entity.ShopKYCStatusRejected
		message = fmt.Sprintf("KYC rejeté. Raison: %s", *req.RejectionReason)
	}

	// 5. Mettre à jour le statut KYC du shop
	if err := uc.shopRepo.UpdateKYCStatus(
		ctx,
		shopID,
		newStatus,
		req.AdminID,
		req.RejectionReason,
	); err != nil {
		logger.Error().
			Err(err).
			Msg("❌ Erreur mise à jour statut KYC")
		return nil, fmt.Errorf("update kyc status: %w", err)
	}

	// 6. Revoir les documents individuels (si fournis)
	if len(req.DocumentReviews) > 0 {
		for _, docReview := range req.DocumentReviews {
			docID, err := uuid.Parse(docReview.DocumentID)
			if err != nil {
				logger.Warn().
					Err(err).
					Str("document_id", docReview.DocumentID).
					Msg("⚠️ ID document invalide, ignoré")
				continue
			}

			doc, err := uc.kycRepo.FindByID(ctx, docID)
			if err != nil || doc == nil {
				logger.Warn().
					Err(err).
					Str("document_id", docID.String()).
					Msg("⚠️ Document non trouvé, ignoré")
				continue
			}

			switch docReview.Action {
			case "approve":
				doc.Approve(req.AdminID)
			case "reject":
				if docReview.RejectionReason != nil {
					if err := doc.Reject(req.AdminID, *docReview.RejectionReason); err != nil {
						logger.Warn().Err(err).Msg("⚠️ Erreur rejet document")
						continue
					}
				}
			}

			if err := uc.kycRepo.Update(ctx, doc); err != nil {
				logger.Warn().
					Err(err).
					Str("document_id", docID.String()).
					Msg("⚠️ Erreur mise à jour document")
			}
		}
	}

	// 7. Récupérer le shop mis à jour
	updatedShop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("find updated shop: %w", err)
	}

	// 8. Compter les documents
	documentsCount, _ := uc.kycRepo.CountByShopID(ctx, shopID)

	logger.Info().
		Str("shop_id", shopID.String()).
		Str("new_status", string(updatedShop.KYCStatus)).
		Str("admin_id", req.AdminID).
		Msg("✅ Revue KYC terminée")

	return &ReviewMerchantKYCResponse{
		ShopID:         updatedShop.ID.String(),
		ShopName:       updatedShop.Name,
		KYCStatus:      string(updatedShop.KYCStatus),
		KYCVerifiedAt:  updatedShop.KYCVerifiedAt,
		KYCVerifiedBy:  req.AdminID,
		DocumentsCount: documentsCount,
		Message:        message,
	}, nil
}

// validateRequest valide la requête de revue
func (uc *ReviewMerchantKYCUsecase) validateRequest(req *ReviewMerchantKYCRequest) error {
	if req.ShopID == "" {
		return errors.New("shop_id is required")
	}
	if req.AdminID == "" {
		return errors.New("admin_id is required")
	}
	if req.Action != "approve" && req.Action != "reject" {
		return errors.New("action must be 'approve' or 'reject'")
	}
	if req.Action == "reject" {
		if req.RejectionReason == nil || *req.RejectionReason == "" {
			return errors.New("rejection_reason is required when action is 'reject'")
		}
	}
	return nil
}

// ============================================================
// LIST PENDING KYC USECASE (Admin)
// ============================================================

// ListPendingMerchantKYCUsecase liste les shops en attente de vérification
type ListPendingMerchantKYCUsecase struct {
	shopRepo repository.ShopRepository
	kycRepo  repository.ShopKYCDocumentRepository
}

// NewListPendingMerchantKYCUsecase crée une nouvelle instance
func NewListPendingMerchantKYCUsecase(
	shopRepo repository.ShopRepository,
	kycRepo repository.ShopKYCDocumentRepository,
) *ListPendingMerchantKYCUsecase {
	return &ListPendingMerchantKYCUsecase{
		shopRepo: shopRepo,
		kycRepo:  kycRepo,
	}
}

// ListPendingRequest représente la requête
type ListPendingRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// PendingShopItem représente un shop en attente
type PendingShopItem struct {
	ShopID              string     `json:"shop_id"`
	ShopName            string     `json:"shop_name"`
	ShopSlug            string     `json:"shop_slug"`
	OwnerID             string     `json:"owner_id"`
	KYCStatus           string     `json:"kyc_status"`
	KYCSubmittedAt      *time.Time `json:"kyc_submitted_at"`
	KYCSubmissionsCount int        `json:"kyc_submissions_count"`
	DaysWaiting         int        `json:"days_waiting"`
	DocumentsCount      int        `json:"documents_count"`
}

// ListPendingResponse représente la réponse
type ListPendingResponse struct {
	Shops      []PendingShopItem `json:"shops"`
	TotalCount int               `json:"total_count"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
}

// Execute liste les shops en attente
func (uc *ListPendingMerchantKYCUsecase) Execute(
	ctx context.Context,
	req *ListPendingRequest,
) (*ListPendingResponse, error) {
	logger := zerolog.Ctx(ctx)

	// Valeurs par défaut
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	logger.Debug().
		Int("limit", req.Limit).
		Int("offset", req.Offset).
		Msg("📋 Liste des shops KYC en attente")

	// 1. Récupérer les shops en statut "pending"
	shops, err := uc.shopRepo.FindByKYCStatus(
		ctx,
		entity.ShopKYCStatusPending,
		req.Limit,
		req.Offset,
	)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("❌ Erreur récupération shops pending")
		return nil, fmt.Errorf("find pending shops: %w", err)
	}

	// 2. Compter le total
	counts, err := uc.shopRepo.CountByKYCStatus(ctx)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("⚠️ Erreur comptage (non bloquant)")
	}
	totalCount := counts[entity.ShopKYCStatusPending]

	// 3. Construire la réponse
	items := make([]PendingShopItem, 0, len(shops))
	for _, shop := range shops {
		docsCount, _ := uc.kycRepo.CountByShopID(ctx, shop.ID)

		daysWaiting := 0
		if shop.KYCSubmittedAt != nil {
			daysWaiting = int(time.Since(*shop.KYCSubmittedAt).Hours() / 24)
		}

		items = append(items, PendingShopItem{
			ShopID:              shop.ID.String(),
			ShopName:            shop.Name,
			ShopSlug:            shop.Slug,
			OwnerID:             shop.OwnerID,
			KYCStatus:           string(shop.KYCStatus),
			KYCSubmittedAt:      shop.KYCSubmittedAt,
			KYCSubmissionsCount: shop.KYCSubmissionsCount,
			DaysWaiting:         daysWaiting,
			DocumentsCount:      docsCount,
		})
	}

	logger.Info().
		Int("shops_count", len(items)).
		Int("total_pending", totalCount).
		Msg("✅ Liste des shops pending récupérée")

	return &ListPendingResponse{
		Shops:      items,
		TotalCount: totalCount,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}, nil
}
