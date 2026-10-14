package customerusecase

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
// Constantes KYC
// ============================================================

const (
	// MaxFileSizeBytes = 5 Mo
	MaxFileSizeBytes = 5 * 1024 * 1024
	// MaxDocumentsPerCustomer = 3 documents max
	MaxDocumentsPerCustomer = 3
)

// ============================================================
// Usecase
// ============================================================

// UploadKYCDocumentUsecase permet a un client d'uploader un document KYC
type UploadKYCDocumentUsecase struct {
	kycDocRepo   repository.CustomerKYCRepository
	customerRepo repository.CustomerRepositoryInterface
	txManager    repository.TxManager
	tokenRepo    repository.UploadTokenRepository
}

// NewUploadKYCDocumentUsecase cree une nouvelle instance
func NewUploadKYCDocumentUsecase(
	kycDocRepo repository.CustomerKYCRepository,
	customerRepo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
	tokenRepo repository.UploadTokenRepository,
) *UploadKYCDocumentUsecase {
	return &UploadKYCDocumentUsecase{
		kycDocRepo:   kycDocRepo,
		customerRepo: customerRepo,
		txManager:    txManager,
		tokenRepo:    tokenRepo,
	}
}

// UploadKYCRequest represente la requete d'upload KYC
type UploadKYCRequest struct {
	CustomerID    string                 `json:"customer_id"`
	DocumentType  entity.KYCDocumentType `json:"document_type"`
	FilePath      string                 `json:"file_path"`
	FileSizeBytes int64                  `json:"file_size_bytes"`
	MimeType      string                 `json:"mime_type"`
	Token         string                 `json:"token"`
}

// Validate valide la requete
func (r *UploadKYCRequest) Validate() error {
	if r.CustomerID == "" {
		return errors.New("customer_id is required")
	}
	if !r.DocumentType.IsValid() {
		return fmt.Errorf("invalid document type: %s (must be cni, passport or other)", r.DocumentType)
	}
	if r.FilePath == "" {
		return errors.New("file_path is required")
	}
	if r.FileSizeBytes <= 0 {
		return errors.New("file_size_bytes must be positive")
	}
	if r.FileSizeBytes > MaxFileSizeBytes {
		return fmt.Errorf("file size exceeds maximum (%d MB)", MaxFileSizeBytes/(1024*1024))
	}
	if r.MimeType == "" {
		return errors.New("mime_type is required")
	}
	allowedMimeTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"application/pdf": true,
	}
	if !allowedMimeTypes[r.MimeType] {
		return fmt.Errorf("mime type not allowed: %s (must be image/jpeg, image/png or application/pdf)", r.MimeType)
	}
	if r.Token == "" {
		return errors.New("upload token is required")
	}
	return nil
}

// Execute uploade un document KYC pour un client
func (uc *UploadKYCDocumentUsecase) Execute(ctx context.Context, req *UploadKYCRequest) (*entity.CustomerKYCDocument, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Valider la requete
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 3. Token d'upload (AVANT la transaction)
	token, err := uc.tokenRepo.FindByID(ctx, req.Token)
	if err != nil {
		logger.Warn().Err(err).Str("token", req.Token).Msg("Token introuvable")
		return nil, fmt.Errorf("invalid upload token: %w", err)
	}

	if !token.IsValid() {
		logger.Warn().
			Str("token", req.Token).
			Bool("used", token.Used).
			Bool("expired", token.IsExpired()).
			Msg("Token invalide (expire ou deja utilise)")
		return nil, errors.New("upload token is expired or already used")
	}

	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || !token.MatchesUserID(authUserID) {
		logger.Warn().
			Str("token_user", token.UserID).
			Str("auth_user", authUserID).
			Msg("Token n'appartient pas a l'utilisateur authentifie")
		return nil, errors.New("upload token does not belong to authenticated user")
	}

	if !token.MatchesFilePath(req.FilePath) {
		logger.Warn().
			Str("token_path", token.FilePath).
			Str("request_path", req.FilePath).
			Msg("FilePath ne correspond pas au token")
		return nil, errors.New("file path does not match upload token")
	}

	// ============================================================
	// 3.5 Existence physique + anti-path traversal
	// ============================================================
	cleanPath := filepath.Clean(req.FilePath)
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	if !strings.HasPrefix(cleanPath, "/uploads/") && !strings.HasPrefix(cleanPath, "uploads/") {
		logger.Error().
			Str("file_path", req.FilePath).
			Msg("Chemin non autorise (doit etre dans /uploads/)")
		return nil, errors.New("file path must be in /uploads/ directory")
	}

	fileInfo, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		logger.Error().
			Str("file_path", cleanPath).
			Msg("Fichier introuvable sur disque")
		return nil, errors.New("file not found on disk")
	}
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if fileInfo.IsDir() {
		logger.Error().
			Str("file_path", cleanPath).
			Msg("Le chemin pointe vers un dossier, pas un fichier")
		return nil, errors.New("path is a directory, not a file")
	}

	// ============================================================
	// 3.6 Magic number + coherence extension / MIME declare
	// ============================================================
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}

	allowedTypes := []utils.FileType{
		utils.FileTypeJPEG,
		utils.FileTypePNG,
		utils.FileTypePDF,
	}
	contentResult, err := utils.ValidateFileContent(f, MaxFileSizeBytes, allowedTypes)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("content validation error: %w", err)
	}
	if contentResult == nil || !contentResult.Valid {
		msg := "invalid file content"
		if contentResult != nil && contentResult.ErrorMessage != "" {
			msg = contentResult.ErrorMessage
		}
		logger.Warn().
			Str("file_path", cleanPath).
			Str("reason", msg).
			Msg("Contenu fichier rejete (magic number)")
		return nil, fmt.Errorf("content validation failed: %s", msg)
	}

	// Nom de fichier : token ou basename du path
	fileName := filepath.Base(req.FilePath)
	if token.FileName != "" {
		fileName = token.FileName
	}
	if err := utils.ValidateFileMetadata(fileName, req.MimeType, contentResult.FileType); err != nil {
		return nil, fmt.Errorf("file metadata validation failed: %w", err)
	}

	if err := utils.ValidateFileMetadata(fileName, req.MimeType, contentResult.FileType); err != nil {
		logger.Warn().
			Err(err).
			Str("file_name", fileName).
			Str("mime_type", req.MimeType).
			Str("detected", string(contentResult.FileType)).
			Msg("Metadata incoherente avec le contenu")
		return nil, fmt.Errorf("file metadata validation failed: %w", err)
	}

	logger.Info().
		Str("token", req.Token).
		Str("user_id", authUserID).
		Str("file_path", req.FilePath).
		Str("detected_type", string(contentResult.FileType)).
		Msg("Token valide, fichier verifie (magic number OK)")

	// 4. Transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	kycDocRepoTx := uc.kycDocRepo.WithTX(tx)
	customerRepoTx := uc.customerRepo.WithTX(tx)

	customer, err := customerRepoTx.FindByCustomerID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	count, err := kycDocRepoTx.CountByCustomer(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}
	if count >= MaxDocumentsPerCustomer {
		return nil, fmt.Errorf("maximum number of documents reached (%d)", MaxDocumentsPerCustomer)
	}

	pendingDocs, err := kycDocRepoTx.FindPendingByCustomer(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to check pending documents: %w", err)
	}
	for _, doc := range pendingDocs {
		if doc.DocumentType == req.DocumentType {
			return nil, fmt.Errorf("a document of type %s is already pending review", req.DocumentType)
		}
	}

	doc, err := entity.NewCustomerKYCDocument(
		req.CustomerID,
		shop.ID.String(),
		req.DocumentType,
		req.FilePath,
		req.FileSizeBytes,
		req.MimeType,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create document entity: %w", err)
	}

	if err := kycDocRepoTx.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to save document: %w", err)
	}

	customer.MarkKYCPending()
	if _, err := customerRepoTx.UpdateCustomer(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to update customer KYC status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if err := uc.tokenRepo.MarkUsed(ctx, req.Token); err != nil {
		logger.Warn().
			Err(err).
			Str("token", req.Token).
			Msg("Impossible de marquer le token comme used (non bloquant)")
	}

	logger.Info().
		Str("customer_id", req.CustomerID).
		Str("document_type", string(req.DocumentType)).
		Str("file_path", req.FilePath).
		Msg("Document KYC soumis avec succes (token consomme)")

	return doc, nil
}

// ============================================================
// GetKYCStatusUsecase
// ============================================================

type GetKYCStatusUsecase struct {
	customerRepo repository.CustomerRepositoryInterface
	kycDocRepo   repository.CustomerKYCRepository
}

func NewGetKYCStatusUsecase(
	customerRepo repository.CustomerRepositoryInterface,
	kycDocRepo repository.CustomerKYCRepository,
) *GetKYCStatusUsecase {
	return &GetKYCStatusUsecase{
		customerRepo: customerRepo,
		kycDocRepo:   kycDocRepo,
	}
}

type KYCStatusResponse struct {
	CustomerID string                        `json:"customer_id"`
	KYCLevel   entity.KYCLevel               `json:"kyc_level"`
	Documents  []*entity.CustomerKYCDocument `json:"documents"`
}

func (uc *GetKYCStatusUsecase) Execute(ctx context.Context, customerID string) (*KYCStatusResponse, error) {
	_, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	customer, err := uc.customerRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	documents, err := uc.kycDocRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}

	return &KYCStatusResponse{
		CustomerID: customer.ID,
		KYCLevel:   customer.KYCLevel,
		Documents:  documents,
	}, nil
}

// ============================================================
// GenerateKYCFilePath
// ============================================================

// GenerateKYCFilePath genere un path standardise pour un document KYC
// Format : /uploads/kyc/{customer_id}/{document_type}_{timestamp}.ext
func GenerateKYCFilePath(customerID string, documentType entity.KYCDocumentType, mimeType string) string {
	timestamp := time.Now().UTC().Format("20060102_150405")
	ext := "jpg"
	switch mimeType {
	case "image/png":
		ext = "png"
	case "application/pdf":
		ext = "pdf"
	}
	return fmt.Sprintf("/uploads/kyc/%s/%s_%s.%s", customerID, documentType, timestamp, ext)
}
