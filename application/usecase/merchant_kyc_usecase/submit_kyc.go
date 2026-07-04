package merchantkycusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ============================================================
// SUBMIT MERCHANT KYC USECASE
// ============================================================
//
// 🎯 Objectif :
//   Le marchand soumet ses documents KYC pour vérification.
//   Après soumission, son statut passe à "pending".
//
// 📋 Règles :
//   - Minimum 2 documents requis :
//     * identity_card OU passport
//     * business_registry
//   - Taille max par document : 5 Mo
//   - Types MIME autorisés : JPEG, PNG, PDF
//   - Le shop doit être actif
//   - Le shop ne doit pas déjà être vérifié
//
// 🔄 Workflow :
//   1. Vérifier que le shop existe et est actif
//   2. Valider les documents soumis
//   3. Supprimer les anciens documents (si re-soumission)
//   4. Créer les nouveaux documents
//   5. Mettre à jour le statut KYC du shop à "pending"
//   6. Retourner le statut mis à jour
//
// ============================================================

// SubmitMerchantKYCUsecase gère la soumission KYC par un marchand
type SubmitMerchantKYCUsecase struct {
	shopRepo repository.ShopRepository
	kycRepo  repository.ShopKYCDocumentRepository
}

// NewSubmitMerchantKYCUsecase crée une nouvelle instance
func NewSubmitMerchantKYCUsecase(
	shopRepo repository.ShopRepository,
	kycRepo repository.ShopKYCDocumentRepository,
) *SubmitMerchantKYCUsecase {
	return &SubmitMerchantKYCUsecase{
		shopRepo: shopRepo,
		kycRepo:  kycRepo,
	}
}

// SubmitMerchantKYCRequest représente la requête de soumission KYC
type SubmitMerchantKYCRequest struct {
	Documents []DocumentInput `json:"documents"`
}

// DocumentInput représente un document soumis
type DocumentInput struct {
	DocumentType  string `json:"document_type"` // identity_card, passport, business_registry, etc.
	FilePath      string `json:"file_path"`
	FileName      string `json:"file_name"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	MimeType      string `json:"mime_type"`
}

// SubmitMerchantKYCResponse représente la réponse
type SubmitMerchantKYCResponse struct {
	ShopID              string    `json:"shop_id"`
	ShopName            string    `json:"shop_name"`
	KYCStatus           string    `json:"kyc_status"`
	KYCSubmittedAt      time.Time `json:"kyc_submitted_at"`
	KYCSubmissionsCount int       `json:"kyc_submissions_count"`
	DocumentsCount      int       `json:"documents_count"`
	Message             string    `json:"message"`
}

// Execute soumet les documents KYC
func (uc *SubmitMerchantKYCUsecase) Execute(
	ctx context.Context,
	req *SubmitMerchantKYCRequest,
) (*SubmitMerchantKYCResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant context required: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("shop_name", shop.Name).
		Int("documents_count", len(req.Documents)).
		Msg("📝 Début soumission KYC marchand")

	// 2. Vérifier que le shop est actif
	if !shop.IsActive {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Msg("❌ Shop inactif")
		return nil, errors.New("shop is not active")
	}

	// 3. Vérifier que le KYC n'est pas déjà vérifié
	if shop.IsVerified() {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Str("current_status", string(shop.KYCStatus)).
			Msg("❌ KYC déjà vérifié")
		return nil, entity.ErrShopKYCAlreadyVerified
	}

	// 4. Valider les documents soumis
	if err := uc.validateDocuments(req.Documents); err != nil {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Err(err).
			Msg("❌ Validation documents échouée")
		return nil, err
	}

	// 5. Transaction : supprimer anciens docs + créer nouveaux + update statut
	txRepo := uc.shopRepo.WithTX(nil) // TODO: Implémenter transaction réelle
	kycTxRepo := uc.kycRepo.WithTX(nil)

	// 5.1 Supprimer les anciens documents (si re-soumission)
	if shop.KYCSubmissionsCount > 0 {
		logger.Info().
			Str("shop_id", shop.ID.String()).
			Int("previous_submissions", shop.KYCSubmissionsCount).
			Msg("🗑️ Suppression des anciens documents KYC")

		if err := kycTxRepo.DeleteByShopID(ctx, shop.ID); err != nil {
			logger.Error().
				Err(err).
				Msg("❌ Erreur suppression anciens documents")
			return nil, fmt.Errorf("delete old documents: %w", err)
		}
	}

	// 5.2 Créer les nouveaux documents
	for _, docInput := range req.Documents {
		doc, err := entity.NewShopKYCDocument(
			shop.ID,
			entity.ShopKYCDocumentType(docInput.DocumentType),
			docInput.FilePath,
			docInput.FileName,
			docInput.FileSizeBytes,
			docInput.MimeType,
		)
		if err != nil {
			logger.Error().
				Err(err).
				Str("document_type", docInput.DocumentType).
				Msg("❌ Erreur création document")
			return nil, fmt.Errorf("create document: %w", err)
		}

		if err := kycTxRepo.Create(ctx, doc); err != nil {
			logger.Error().
				Err(err).
				Str("document_id", doc.ID.String()).
				Msg("❌ Erreur sauvegarde document")
			return nil, fmt.Errorf("save document: %w", err)
		}

		logger.Debug().
			Str("document_id", doc.ID.String()).
			Str("document_type", docInput.DocumentType).
			Msg("✅ Document KYC créé")
	}

	// 5.3 Mettre à jour le statut KYC du shop
	if err := txRepo.UpdateKYCStatus(
		ctx,
		shop.ID,
		entity.ShopKYCStatusPending,
		"", // Pas d'admin, c'est le marchand qui soumet
		nil,
	); err != nil {
		logger.Error().
			Err(err).
			Msg("❌ Erreur mise à jour statut KYC")
		return nil, fmt.Errorf("update kyc status: %w", err)
	}

	// 6. Récupérer le shop mis à jour
	updatedShop, err := txRepo.FindByID(ctx, shop.ID)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("❌ Erreur récupération shop mis à jour")
		return nil, fmt.Errorf("find updated shop: %w", err)
	}

	// 7. Compter les documents créés
	documentsCount, err := uc.kycRepo.CountByShopID(ctx, shop.ID)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("⚠️ Erreur comptage documents (non bloquant)")
		documentsCount = len(req.Documents)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("kyc_status", string(updatedShop.KYCStatus)).
		Int("documents_count", documentsCount).
		Int("submissions_count", updatedShop.KYCSubmissionsCount).
		Msg("✅ Soumission KYC réussie")

	return &SubmitMerchantKYCResponse{
		ShopID:              updatedShop.ID.String(),
		ShopName:            updatedShop.Name,
		KYCStatus:           string(updatedShop.KYCStatus),
		KYCSubmittedAt:      *updatedShop.KYCSubmittedAt,
		KYCSubmissionsCount: updatedShop.KYCSubmissionsCount,
		DocumentsCount:      documentsCount,
		Message:             "Documents KYC soumis avec succès. En attente de vérification par un administrateur.",
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

// validateDocuments valide les documents soumis
func (uc *SubmitMerchantKYCUsecase) validateDocuments(docs []DocumentInput) error {
	if len(docs) == 0 {
		return errors.New("at least one document is required")
	}

	if len(docs) > 10 {
		return errors.New("maximum 10 documents allowed")
	}

	hasIdentity := false
	hasBusinessRegistry := false

	allowedMimeTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"application/pdf": true,
	}

	for i, doc := range docs {
		// Validation type
		if !entity.IsValidShopKYCDocType(entity.ShopKYCDocumentType(doc.DocumentType)) {
			return fmt.Errorf("document %d: invalid document type '%s'", i+1, doc.DocumentType)
		}

		// Validation chemin fichier
		if doc.FilePath == "" {
			return fmt.Errorf("document %d: file path is required", i+1)
		}

		// Validation nom fichier
		if doc.FileName == "" {
			return fmt.Errorf("document %d: file name is required", i+1)
		}

		// Validation taille
		if doc.FileSizeBytes <= 0 {
			return fmt.Errorf("document %d: file size must be positive", i+1)
		}
		if doc.FileSizeBytes > 5*1024*1024 {
			return fmt.Errorf("document %d: file size exceeds maximum (5 MB)", i+1)
		}

		// Validation MIME type
		if !allowedMimeTypes[doc.MimeType] {
			return fmt.Errorf("document %d: mime type '%s' not allowed (allowed: JPEG, PNG, PDF)", i+1, doc.MimeType)
		}

		// Tracking documents requis
		if doc.DocumentType == "identity_card" || doc.DocumentType == "passport" {
			hasIdentity = true
		}
		if doc.DocumentType == "business_registry" {
			hasBusinessRegistry = true
		}
	}

	// Vérifier documents requis
	if !hasIdentity {
		return errors.New("identity document required (identity_card or passport)")
	}
	if !hasBusinessRegistry {
		return errors.New("business registry document required")
	}

	return nil
}
