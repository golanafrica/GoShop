package customerusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
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

// UploadKYCDocumentUsecase permet à un client d'uploader un document KYC
type UploadKYCDocumentUsecase struct {
	kycDocRepo   repository.CustomerKYCRepository
	customerRepo repository.CustomerRepositoryInterface
	txManager    repository.TxManager
}

// NewUploadKYCDocumentUsecase crée une nouvelle instance
func NewUploadKYCDocumentUsecase(
	kycDocRepo repository.CustomerKYCRepository,
	customerRepo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
) *UploadKYCDocumentUsecase {
	return &UploadKYCDocumentUsecase{
		kycDocRepo:   kycDocRepo,
		customerRepo: customerRepo,
		txManager:    txManager,
	}
}

// UploadKYCRequest représente la requête d'upload KYC
type UploadKYCRequest struct {
	CustomerID    string                 `json:"customer_id"`
	DocumentType  entity.KYCDocumentType `json:"document_type"`
	FilePath      string                 `json:"file_path"`
	FileSizeBytes int64                  `json:"file_size_bytes"`
	MimeType      string                 `json:"mime_type"`
}

// Validate valide la requête
func (r *UploadKYCRequest) Validate() error {
	if r.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if !r.DocumentType.IsValid() {
		return fmt.Errorf("invalid document type: %s (must be cni, passport or other)", r.DocumentType)
	}
	if r.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}
	if r.FileSizeBytes <= 0 {
		return fmt.Errorf("file_size_bytes must be positive")
	}
	if r.FileSizeBytes > MaxFileSizeBytes {
		return fmt.Errorf("file size exceeds maximum (%d MB)", MaxFileSizeBytes/(1024*1024))
	}
	if r.MimeType == "" {
		return fmt.Errorf("mime_type is required")
	}
	// Vérifier les types MIME autorisés
	allowedMimeTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"application/pdf": true,
	}
	if !allowedMimeTypes[r.MimeType] {
		return fmt.Errorf("mime type not allowed: %s (must be image/jpeg, image/png or application/pdf)", r.MimeType)
	}
	return nil
}

// Execute uploade un document KYC pour un client
func (uc *UploadKYCDocumentUsecase) Execute(ctx context.Context, req *UploadKYCRequest) (*entity.CustomerKYCDocument, error) {
	// 1. Récupérer le shop du contexte (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 4. Attacher les repositories à la transaction
	kycDocRepoTx := uc.kycDocRepo.WithTX(tx)
	customerRepoTx := uc.customerRepo.WithTX(tx)

	// 5. Vérifier que le client existe et appartient à la boutique
	customer, err := customerRepoTx.FindByCustomerID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// 6. Vérifier que le client n'a pas déjà atteint la limite de documents
	count, err := kycDocRepoTx.CountByCustomer(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}
	if count >= MaxDocumentsPerCustomer {
		return nil, fmt.Errorf("maximum number of documents reached (%d)", MaxDocumentsPerCustomer)
	}

	// 7. Vérifier qu'il n'y a pas déjà un document en attente pour ce type
	pendingDocs, err := kycDocRepoTx.FindPendingByCustomer(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to check pending documents: %w", err)
	}
	for _, doc := range pendingDocs {
		if doc.DocumentType == req.DocumentType {
			return nil, fmt.Errorf("a document of type %s is already pending review", req.DocumentType)
		}
	}

	// 8. Créer le document KYC
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

	// 9. Mettre à jour le statut KYC du client à "pending"
	customer.MarkKYCPending()
	if _, err := customerRepoTx.UpdateCustomer(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to update customer KYC status: %w", err)
	}

	// 10. Commit
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return doc, nil
}

// ============================================================
// Usecase : Récupérer le statut KYC d'un client
// ============================================================

// GetKYCStatusUsecase récupère le statut KYC d'un client
type GetKYCStatusUsecase struct {
	customerRepo repository.CustomerRepositoryInterface
	kycDocRepo   repository.CustomerKYCRepository
}

// NewGetKYCStatusUsecase crée une nouvelle instance
func NewGetKYCStatusUsecase(
	customerRepo repository.CustomerRepositoryInterface,
	kycDocRepo repository.CustomerKYCRepository,
) *GetKYCStatusUsecase {
	return &GetKYCStatusUsecase{
		customerRepo: customerRepo,
		kycDocRepo:   kycDocRepo,
	}
}

// KYCStatusResponse représente la réponse du statut KYC
type KYCStatusResponse struct {
	CustomerID string                        `json:"customer_id"`
	KYCLevel   entity.KYCLevel               `json:"kyc_level"`
	Documents  []*entity.CustomerKYCDocument `json:"documents"`
}

// Execute récupère le statut KYC d'un client
func (uc *GetKYCStatusUsecase) Execute(ctx context.Context, customerID string) (*KYCStatusResponse, error) {
	// 1. Vérifier le multi-tenant
	_, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Récupérer le client
	customer, err := uc.customerRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// 3. Récupérer les documents KYC
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
// Utilitaire : Générer un path de fichier KYC
// ============================================================

// GenerateKYCFilePath génère un path standardisé pour un document KYC
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
