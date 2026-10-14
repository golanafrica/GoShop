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
//   Le marchand soumet ses documents KYC pour vérification.
//   Après soumission, son statut passe à "pending".
//
// Règles :
//   - Minimum 2 documents : identity_card|passport + business_registry
//   - Taille max 5 Mo, MIME JPEG/PNG/PDF
//   - Magic number sur contenu disque (J2)
//   - Path 100% serveur scopé : uploads/kyc/merchant/{shop_id}/<file> (J3)
//   - Shop actif, KYC non déjà vérifié
//   - Atomicité transactionnelle (J1)
//
// ============================================================

// SubmitMerchantKYCUsecase gère la soumission KYC par un marchand
type SubmitMerchantKYCUsecase struct {
	shopRepo  repository.ShopRepository
	kycRepo   repository.ShopKYCDocumentRepository
	txManager repository.TxManager
}

// NewSubmitMerchantKYCUsecase crée une nouvelle instance
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

// SubmitMerchantKYCRequest représente la requête de soumission KYC
type SubmitMerchantKYCRequest struct {
	Documents []DocumentInput `json:"documents"`
}

// DocumentInput représente un document soumis
type DocumentInput struct {
	DocumentType  string `json:"document_type"`
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

// Execute soumet les documents KYC avec transaction atomique
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
		Msg("Début soumission KYC marchand")

	// 2. Vérifications métier préliminaires
	if !shop.IsActive {
		logger.Warn().Str("shop_id", shop.ID.String()).Msg("Shop inactif")
		return nil, errors.New("shop is not active")
	}

	if shop.IsVerified() {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Str("current_status", string(shop.KYCStatus)).
			Msg("KYC déjà vérifié")
		return nil, entity.ErrShopKYCAlreadyVerified
	}

	if err := uc.validateDocuments(req.Documents); err != nil {
		logger.Warn().Str("shop_id", shop.ID.String()).Err(err).Msg("Validation documents échouée")
		return nil, err
	}

	// 3. 🛡️ Validation approfondie du contenu et du path (J2 & J3)
	allowedTypes := []utils.FileType{
		utils.FileTypeJPEG,
		utils.FileTypePNG,
		utils.FileTypePDF,
	}
	const maxContentBytes int64 = 5 * 1024 * 1024

	resolvedPaths := make([]string, len(req.Documents))

	for i, docInput := range req.Documents {
		// J3 : Vérifier que le path est strictement scopé au shop actuel
		cleanPath, err := assertMerchantKYCPath(shop.ID.String(), docInput.FilePath)
		if err != nil {
			logger.Error().
				Err(err).
				Str("file_path", docInput.FilePath).
				Int("document_index", i+1).
				Msg("Chemin non autorisé (J3 path serveur)")
			return nil, fmt.Errorf("document %d: %w", i+1, err)
		}
		resolvedPaths[i] = cleanPath

		// Vérifier l'existence physique et le type (fichier, pas dossier)
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
			return nil, fmt.Errorf("document %d: path is a directory, not a file", i+1)
		}

		// J2 : Ouvrir le fichier pour valider le magic number
		f, err := os.Open(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("document %d: cannot open file: %w", i+1, err)
		}

		contentResult, err := utils.ValidateFileContent(f, maxContentBytes, allowedTypes)
		_ = f.Close() // Fermer immédiatement après lecture du header

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
				Msg("Contenu fichier rejeté (magic number)")
			return nil, fmt.Errorf("document %d: content validation failed: %s", i+1, msg)
		}

		// J2 : Vérifier la cohérence entre l'extension/MIME déclaré et le type détecté
		if err := utils.ValidateFileMetadata(docInput.FileName, docInput.MimeType, contentResult.FileType); err != nil {
			logger.Warn().
				Err(err).
				Int("document_index", i+1).
				Str("file_name", docInput.FileName).
				Str("mime_type", docInput.MimeType).
				Str("detected", string(contentResult.FileType)).
				Msg("Métadonnées incohérentes avec le contenu")
			return nil, fmt.Errorf("document %d: %w", i+1, err)
		}
	}

	// 4. 🛡️ DÉBUT TRANSACTION ATOMIQUE (J1)
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Impossible de démarrer la transaction")
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback automatique en cas de panic ou d'erreur non gérée

	txShopRepo := uc.shopRepo.WithTX(tx)
	txKYCRepo := uc.kycRepo.WithTX(tx)

	// 4.1 Supprimer les anciens documents (si re-soumission)
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

	// 4.2 Créer les nouveaux documents avec le path canonique sécurisé
	for i, docInput := range req.Documents {
		doc, err := entity.NewShopKYCDocument(
			shop.ID,
			entity.ShopKYCDocumentType(docInput.DocumentType),
			resolvedPaths[i], // Path validé et nettoyé, pas le path brut du client
			docInput.FileName,
			docInput.FileSizeBytes,
			docInput.MimeType,
		)
		if err != nil {
			logger.Error().
				Err(err).
				Int("document_index", i+1).
				Str("document_type", docInput.DocumentType).
				Msg("Erreur création document")
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
			Str("file_path", resolvedPaths[i]).
			Msg("Document KYC créé (dans transaction)")
	}

	// 4.3 Mettre à jour le statut KYC du shop
	if err := txShopRepo.UpdateKYCStatus(
		ctx,
		shop.ID,
		entity.ShopKYCStatusPending,
		"",
		nil,
	); err != nil {
		logger.Error().Err(err).Msg("Erreur mise à jour statut KYC")
		return nil, fmt.Errorf("update kyc status: %w", err)
	}

	// 4.4 Récupérer le shop mis à jour pour la réponse
	updatedShop, err := txShopRepo.FindByID(ctx, shop.ID)
	if err != nil {
		logger.Error().Err(err).Msg("Erreur récupération shop mis à jour")
		return nil, fmt.Errorf("find updated shop: %w", err)
	}

	// 4.5 🛡️ COMMIT TRANSACTION (tout a réussi)
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Erreur commit transaction")
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Msg("Transaction KYC commitée avec succès")

	// 5. Comptage final (hors transaction, non bloquant)
	documentsCount, err := uc.kycRepo.CountByShopID(ctx, shop.ID)
	if err != nil {
		logger.Warn().Err(err).Msg("Erreur comptage documents (non bloquant)")
		documentsCount = len(req.Documents)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Str("kyc_status", string(updatedShop.KYCStatus)).
		Int("documents_count", documentsCount).
		Int("submissions_count", updatedShop.KYCSubmissionsCount).
		Msg("Soumission KYC réussie (transactionnelle)")

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
// PATH SERVEUR (J3 - Anti Path Traversal & Anti IDOR Storage)
// ============================================================

// GenerateMerchantKYCFilePath génère un path canonique serveur sécurisé.
// Format: uploads/kyc/merchant/{shop_id}/{doc_type}_{timestamp}_{rand}.{ext}
func GenerateMerchantKYCFilePath(shopID string, documentType string, mimeType string) string {
	timestamp := time.Now().UTC().Format("20060102_150405")
	ext := "jpg"
	switch mimeType {
	case "image/png":
		ext = "png"
	case "application/pdf":
		ext = "pdf"
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	return fmt.Sprintf(
		"uploads/kyc/merchant/%s/%s_%s_%s.%s",
		shopID,
		documentType,
		timestamp,
		suffix,
		ext,
	)
}

// assertMerchantKYCPath refuse tout path hors de uploads/kyc/merchant/{shopID}/fichier
func assertMerchantKYCPath(shopID, filePath string) (string, error) {
	if shopID == "" {
		return "", errors.New("shop_id is required for path validation")
	}

	// Nettoyage et normalisation cross-platform
	clean := filepath.Clean(filePath)
	clean = strings.ReplaceAll(clean, "\\", "/")
	clean = strings.TrimPrefix(clean, "/")

	// Protection explicite contre le path traversal
	if strings.Contains(clean, "..") {
		return "", errors.New("file path must not contain '..'")
	}

	// Vérification du scope strict au shop actuel
	prefix := "uploads/kyc/merchant/" + shopID + "/"
	if !strings.HasPrefix(clean, prefix) {
		return "", fmt.Errorf("file path must be under %s", prefix)
	}

	// Vérification qu'il s'agit bien d'un fichier direct, pas d'un sous-dossier
	rest := strings.TrimPrefix(clean, prefix)
	if rest == "" {
		return "", errors.New("file path must include a file name")
	}
	if strings.Contains(rest, "/") {
		return "", errors.New("file path must be a single file under shop KYC directory")
	}

	return clean, nil
}

// ============================================================
// VALIDATION MÉTIER
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
