package entity

import (
	"errors"
	"time"
)

// ============================================================
// Niveaux KYC
// ============================================================

// KYCLevel représente le niveau de vérification d'identité d'un client
type KYCLevel string

const (
	KYCLevelNone     KYCLevel = "none"     // Non vérifié
	KYCLevelPending  KYCLevel = "pending"  // Documents uploadés, en attente de validation
	KYCLevelVerified KYCLevel = "verified" // Validé par le marchand
	KYCLevelRejected KYCLevel = "rejected" // Documents rejetés
)

// IsValid vérifie si le niveau KYC est valide
func (k KYCLevel) IsValid() bool {
	switch k {
	case KYCLevelNone, KYCLevelPending, KYCLevelVerified, KYCLevelRejected:
		return true
	}
	return false
}

// String retourne la représentation string du niveau KYC
func (k KYCLevel) String() string {
	return string(k)
}

// ============================================================
// Types de documents KYC
// ============================================================

// KYCDocumentType représente le type de document KYC
type KYCDocumentType string

const (
	KYCDocumentCNI      KYCDocumentType = "cni"      // Carte Nationale d'Identité
	KYCDocumentPassport KYCDocumentType = "passport" // Passeport
	KYCDocumentOther    KYCDocumentType = "other"    // Autre document
)

// IsValid vérifie si le type de document est valide
func (t KYCDocumentType) IsValid() bool {
	switch t {
	case KYCDocumentCNI, KYCDocumentPassport, KYCDocumentOther:
		return true
	}
	return false
}

// String retourne la représentation string du type de document
func (t KYCDocumentType) String() string {
	return string(t)
}

// ============================================================
// Statuts de document KYC
// ============================================================

// KYCDocumentStatus représente le statut d'un document KYC
type KYCDocumentStatus string

const (
	KYCDocumentStatusPending  KYCDocumentStatus = "pending"  // En attente de revue
	KYCDocumentStatusApproved KYCDocumentStatus = "approved" // Approuvé
	KYCDocumentStatusRejected KYCDocumentStatus = "rejected" // Rejeté
)

// IsValid vérifie si le statut est valide
func (s KYCDocumentStatus) IsValid() bool {
	switch s {
	case KYCDocumentStatusPending, KYCDocumentStatusApproved, KYCDocumentStatusRejected:
		return true
	}
	return false
}

// ============================================================
// CustomerKYCDocument — document uploadé par un client
// ============================================================

// CustomerKYCDocument représente un document KYC uploadé par un client
type CustomerKYCDocument struct {
	ID              string            `json:"id" db:"id"`
	CustomerID      string            `json:"customer_id" db:"customer_id"`
	ShopID          string            `json:"shop_id" db:"shop_id"`
	DocumentType    KYCDocumentType   `json:"document_type" db:"document_type"`
	FilePath        string            `json:"file_path" db:"file_path"`
	FileSizeBytes   int64             `json:"file_size_bytes" db:"file_size_bytes"`
	MimeType        string            `json:"mime_type" db:"mime_type"`
	Status          KYCDocumentStatus `json:"status" db:"status"`
	ReviewedBy      *string           `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt      *time.Time        `json:"reviewed_at,omitempty" db:"reviewed_at"`
	RejectionReason *string           `json:"rejection_reason,omitempty" db:"rejection_reason"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at" db:"updated_at"`
}

// NewCustomerKYCDocument crée un nouveau document KYC
func NewCustomerKYCDocument(
	customerID, shopID string,
	documentType KYCDocumentType,
	filePath string,
	fileSizeBytes int64,
	mimeType string,
) (*CustomerKYCDocument, error) {
	// Validations
	if customerID == "" {
		return nil, errors.New("customer_id is required")
	}
	if shopID == "" {
		return nil, errors.New("shop_id is required")
	}
	if !documentType.IsValid() {
		return nil, errors.New("invalid document type")
	}
	if filePath == "" {
		return nil, errors.New("file path is required")
	}
	if fileSizeBytes <= 0 {
		return nil, errors.New("file size must be positive")
	}
	if fileSizeBytes > 5*1024*1024 { // 5 Mo max
		return nil, errors.New("file size exceeds maximum (5 MB)")
	}
	if mimeType == "" {
		return nil, errors.New("mime type is required")
	}

	now := time.Now().UTC()
	return &CustomerKYCDocument{
		CustomerID:    customerID,
		ShopID:        shopID,
		DocumentType:  documentType,
		FilePath:      filePath,
		FileSizeBytes: fileSizeBytes,
		MimeType:      mimeType,
		Status:        KYCDocumentStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// Approve approuve le document
func (d *CustomerKYCDocument) Approve(reviewerID string) error {
	if d.Status != KYCDocumentStatusPending {
		return errors.New("document must be pending to be approved")
	}
	if reviewerID == "" {
		return errors.New("reviewer_id is required")
	}
	now := time.Now().UTC()
	d.Status = KYCDocumentStatusApproved
	d.ReviewedBy = &reviewerID
	d.ReviewedAt = &now
	d.UpdatedAt = now
	return nil
}

// Reject rejette le document avec une raison
func (d *CustomerKYCDocument) Reject(reviewerID, reason string) error {
	if d.Status != KYCDocumentStatusPending {
		return errors.New("document must be pending to be rejected")
	}
	if reviewerID == "" {
		return errors.New("reviewer_id is required")
	}
	if reason == "" {
		return errors.New("rejection reason is required")
	}
	now := time.Now().UTC()
	d.Status = KYCDocumentStatusRejected
	d.ReviewedBy = &reviewerID
	d.ReviewedAt = &now
	d.RejectionReason = &reason
	d.UpdatedAt = now
	return nil
}

// IsPending vérifie si le document est en attente
func (d *CustomerKYCDocument) IsPending() bool {
	return d.Status == KYCDocumentStatusPending
}

// IsApproved vérifie si le document est approuvé
func (d *CustomerKYCDocument) IsApproved() bool {
	return d.Status == KYCDocumentStatusApproved
}

// IsRejected vérifie si le document est rejeté
func (d *CustomerKYCDocument) IsRejected() bool {
	return d.Status == KYCDocumentStatusRejected
}
