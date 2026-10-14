package merchantkycusecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// SUBMIT MERCHANT KYC USECASE
// ============================================================
//
// Objectif :
//   Le marchand soumet ses documents KYC pour verification.
//   Apres soumission, son statut passe a "pending".
//
// Regles :
//   - Minimum 2 documents requis :
//     * identity_card OU passport
//     * business_registry
//   - Taille max par document : 5 Mo
//   - Types MIME autorises : JPEG, PNG, PDF
//   - Magic number verifie sur le contenu disque (anti-spoof)
//   - Le shop doit etre actif
//   - Le shop ne doit pas deja etre verifie
//
// Workflow :
//   1. Verifier multi-tenant + shop actif
//   2. Valider metadata documents (type, taille, MIME declare)
//   3. Existence physique + anti-path traversal
//   4. ValidateFileContent + ValidateFileMetadata
//   5. BEGIN TRANSACTION
//   6. Supprimer anciens docs si re-soumission
//   7. Creer nouveaux docs + statut pending
//   8. COMMIT
//
// ============================================================

// SubmitMerchantKYCUsecase gere la soumission KYC par un marchand
type SubmitMerchantKYCUsecase struct {
	shopRepo  repository.ShopRepository
	kycRepo   repository.ShopKYCDocumentRepository
	txManager repository.TxManager
}

// NewSubmitMerchantKYCUsecase cree une nouvelle instance
func NewSubmitMerchantKYCUsecase(
	shopRepo repository.ShopRepository,
	kycRepo repository.ShopKYCDocumentRepository,
	txManager repository.TxManager,
) *SubmitMerchantKYCUsecase {
	return &SubmitMerchantKYCUsecase{
		shopRepo:  shopRepo,
		kycRepo:   kycRepo,
		txManager: txManager,
	}
}

// SubmitMerchantKYCRequest represente la requete de soumission KYC
type SubmitMerchantKYCRequest struct {
	Documents []DocumentInput `json:"documents"`
}

// DocumentInput represente un document soumis
type DocumentInput struct {
	DocumentType  string `json:"document_type"`
	FilePath      string `json:"file_path"`
	FileName      string `json:"file_name"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	MimeType      string `json:"mime_type"`
}

// SubmitMerchantKYCResponse represente la reponse
type SubmitMerchantKYCResponse struct {
	ShopID              string    `json:"shop_id"`
	ShopName            string    `json:"shop_name"`
	KYCStatus           string    `json:"kyc_status"`
	KYCSubmittedAt      time.Time `json:"kyc_submitted_at"`
	KYCSubmissionsCount int       `json:"kyc_submissions_count"`
	DocumentsCount      int       `json:"documents_count"`
	Message             string    `json:"message"`
}

// Execute soumet les documents KYC avec transaction atomique
func (uc *SubmitMerchantKYCUsecase) Execute(
	ctx context.Context,
	req *SubmitMerchantKYCRequest,
) (*SubmitMerchantKYCResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant context required: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("shop_name", shop.Name).
		Int("documents_count", len(req.Documents)).
		Msg("Debut soumission KYC marchand")

	// 2. Shop actif
	if !shop.IsActive {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Msg("Shop inactif")
		return nil, errors.New("shop is not active")
	}

	// 3. Pas deja verifie
	if shop.IsVerified() {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Str("current_status", string(shop.KYCStatus)).
			Msg("KYC deja verifie")
		return nil, entity.ErrShopKYCAlreadyVerified
	}

	// 4. Validation metier documents (AVANT TX)
	if err := uc.validateDocuments(req.Documents); err != nil {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Err(err).
			Msg("Validation documents echouee")
		return nil, err
	}

	// ============================================================
	// 4.5 Existence + path + magic number + coherence metadata
	// ============================================================
	allowedTypes := []utils.FileType{
		utils.FileTypeJPEG,
		utils.FileTypePNG,
		utils.FileTypePDF,
	}
	const maxContentBytes int64 = 5 * 1024 * 1024

	for i, docInput := range req.Documents {
		cleanPath := filepath.Clean(docInput.FilePath)
		cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

		if !strings.HasPrefix(cleanPath, "/uploads/") && !strings.HasPrefix(cleanPath, "uploads/") {
			logger.Error().
				Str("file_path", docInput.FilePath).
				Int("document_index", i+1).
				Msg("Chemin non autorise (doit etre dans /uploads/)")
			return nil, fmt.Errorf("document %d: file path must be in /uploads/ directory", i+1)
		}

		fileInfo, err := os.Stat(cleanPath)
		if os.IsNotExist(err) {
			logger.Error().
				Str("file_path", cleanPath).
				Int("document_index", i+1).
				Msg("Fichier introuvable sur disque")
			return nil, fmt.Errorf("document %d: file not found at %s", i+1, cleanPath)
		}
		if err != nil {
			return nil, fmt.Errorf("document %d: stat file: %w", i+1, err)
		}
		if fileInfo.IsDir() {
			logger.Error().
				Str("file_path", cleanPath).
				Int("document_index", i+1).
				Msg("Le chemin pointe vers un dossier, pas un fichier")
			return nil, fmt.Errorf("document %d: path is a directory, not a file", i+1)
		}

		f, err := os.Open(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("document %d: cannot open file: %w", i+1, err)
		}

		contentResult, err := utils.ValidateFileContent(f, maxContentBytes, allowedTypes)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("document %d: content validation error: %w", i+1, err)
		}
		if contentResult == nil || !contentResult.Valid {
			msg := "invalid file content"
			if contentResult != nil && contentResult.ErrorMessage != "" {
				msg = contentResult.ErrorMessage
			}
			logger.Warn().
				Int("document_index", i+1).
				Str("file_path", cleanPath).
				Str("reason", msg).
				Msg("Contenu fichier rejete (magic number)")
			return nil, fmt.Errorf("document %d: content validation failed: %s", i+1, msg)
		}

		if err := utils.ValidateFileMetadata(docInput.FileName, docInput.MimeType, contentResult.FileType); err != nil {
			logger.Warn().
				Err(err).
				Int("document_index", i+1).
				Str("file_name", docInput.FileName).
				Str("mime_type", docInput.MimeType).
				Str("detected", string(contentResult.FileType)).
				Msg("Metadata incoherente avec le contenu")
			return nil, fmt.Errorf("document %d: %w", i+1, err)
		}
	}

	// ============================================================
	// TRANSACTION ATOMIQUE
	// ============================================================
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Impossible de demarrer la transaction")
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	txShopRepo := uc.shopRepo.WithTX(tx)
	txKYCRepo := uc.kycRepo.WithTX(tx)

	// 5.1 Supprimer anciens documents (re-soumission)
	if shop.KYCSubmissionsCount > 0 {
		logger.Info().
			Str("shop_id", shop.ID.String()).
			Int("previous_submissions", shop.KYCSubmissionsCount).
			Msg("Suppression des anciens documents KYC")

		if err := txKYCRepo.DeleteByShopID(ctx, shop.ID); err != nil {
			logger.Error().Err(err).Msg("Erreur suppression anciens documents")
			return nil, fmt.Errorf("delete old documents: %w", err)
		}
	}

	// 5.2 Creer les nouveaux documents
	for i, docInput := range req.Documents {
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
				Int("document_index", i+1).
				Str("document_type", docInput.DocumentType).
				Msg("Erreur creation document")
			return nil, fmt.Errorf("create document %d: %w", i+1, err)
		}

		if err := txKYCRepo.Create(ctx, doc); err != nil {
			logger.Error().
				Err(err).
				Str("document_id", doc.ID.String()).
				Str("document_type", docInput.DocumentType).
				Msg("Erreur sauvegarde document")
			return nil, fmt.Errorf("save document %d: %w", i+1, err)
		}

		logger.Debug().
			Str("document_id", doc.ID.String()).
			Str("document_type", docInput.DocumentType).
			Msg("Document KYC cree (dans transaction)")
	}

	// 5.3 Statut KYC pending
	if err := txShopRepo.UpdateKYCStatus(
		ctx,
		shop.ID,
		entity.ShopKYCStatusPending,
		"",
		nil,
	); err != nil {
		logger.Error().Err(err).Msg("Erreur mise a jour statut KYC")
		return nil, fmt.Errorf("update kyc status: %w", err)
	}

	// 5.4 Shop a jour
	updatedShop, err := txShopRepo.FindByID(ctx, shop.ID)
	if err != nil {
		logger.Error().Err(err).Msg("Erreur recuperation shop mis a jour")
		return nil, fmt.Errorf("find updated shop: %w", err)
	}

	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Erreur commit transaction")
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Msg("Transaction KYC committee avec succes")

	documentsCount, err := uc.kycRepo.CountByShopID(ctx, shop.ID)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("Erreur comptage documents (non bloquant)")
		documentsCount = len(req.Documents)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("kyc_status", string(updatedShop.KYCStatus)).
		Int("documents_count", documentsCount).
		Int("submissions_count", updatedShop.KYCSubmissionsCount).
		Msg("Soumission KYC reussie (transactionnelle)")

	return &SubmitMerchantKYCResponse{
		ShopID:              updatedShop.ID.String(),
		ShopName:            updatedShop.Name,
		KYCStatus:           string(updatedShop.KYCStatus),
		KYCSubmittedAt:      *updatedShop.KYCSubmittedAt,
		KYCSubmissionsCount: updatedShop.KYCSubmissionsCount,
		DocumentsCount:      documentsCount,
		Message:             "Documents KYC soumis avec succes. En attente de verification par un administrateur.",
	}, nil
}

// ============================================================
// VALIDATION
// ============================================================

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
		if !entity.IsValidShopKYCDocType(entity.ShopKYCDocumentType(doc.DocumentType)) {
			return fmt.Errorf("document %d: invalid document type '%s'", i+1, doc.DocumentType)
		}

		if doc.FilePath == "" {
			return fmt.Errorf("document %d: file path is required", i+1)
		}

		if doc.FileName == "" {
			return fmt.Errorf("document %d: file name is required", i+1)
		}

		if doc.FileSizeBytes <= 0 {
			return fmt.Errorf("document %d: file size must be positive", i+1)
		}
		if doc.FileSizeBytes > 5*1024*1024 {
			return fmt.Errorf("document %d: file size exceeds maximum (5 MB)", i+1)
		}

		if !allowedMimeTypes[doc.MimeType] {
			return fmt.Errorf("document %d: mime type '%s' not allowed (allowed: JPEG, PNG, PDF)", i+1, doc.MimeType)
		}

		if doc.DocumentType == "identity_card" || doc.DocumentType == "passport" {
			hasIdentity = true
		}
		if doc.DocumentType == "business_registry" {
			hasBusinessRegistry = true
		}
	}

	if !hasIdentity {
		return errors.New("identity document required (identity_card or passport)")
	}
	if !hasBusinessRegistry {
		return errors.New("business registry document required")
	}

	return nil
}
