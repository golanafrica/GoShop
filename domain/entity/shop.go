package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// SHOP PLAN
// ============================================================

// ShopPlan représente le plan d'abonnement d'une boutique
type ShopPlan string

const (
	ShopPlanFree     ShopPlan = "free"
	ShopPlanPro      ShopPlan = "pro"
	ShopPlanBusiness ShopPlan = "business"
)

// ============================================================
// 🆕 v4.1.0 : KYC MARCHAND
// ============================================================

// ShopKYCStatus représente le statut KYC d'un marchand
type ShopKYCStatus string

const (
	ShopKYCStatusUnverified ShopKYCStatus = "unverified" // Initial (peut vendre, pas de retraits)
	ShopKYCStatusPending    ShopKYCStatus = "pending"    // Documents soumis, en attente
	ShopKYCStatusVerified   ShopKYCStatus = "verified"   // Approuvé par admin
	ShopKYCStatusRejected   ShopKYCStatus = "rejected"   // Rejeté par admin
)

// ShopKYCDocumentType représente le type de document KYC marchand
// 🆕 v4.1.0 : Préfixé "Shop" pour éviter conflit avec KYCDocumentType (customer)
type ShopKYCDocumentType string

const (
	ShopKYCDocIdentityCard     ShopKYCDocumentType = "identity_card"     // CNI
	ShopKYCDocPassport         ShopKYCDocumentType = "passport"          // Passeport
	ShopKYCDocBusinessRegistry ShopKYCDocumentType = "business_registry" // Registre de commerce
	ShopKYCDocTaxCertificate   ShopKYCDocumentType = "tax_certificate"   // Attestation fiscale
	ShopKYCDocBankStatement    ShopKYCDocumentType = "bank_statement"    // Relevé bancaire
	ShopKYCDocOther            ShopKYCDocumentType = "other"             // Autre
)

// Erreurs KYC marchand
var (
	ErrShopKYCAlreadyVerified  = errors.New("shop KYC already verified")
	ErrShopKYCNotPending       = errors.New("shop KYC not in pending status")
	ErrShopKYCInsufficientDocs = errors.New("minimum 2 documents required (identity + business registry)")
	ErrShopKYCCannotWithdraw   = errors.New("merchant KYC verification required for withdrawals")
	ErrShopKYCInvalidStatus    = errors.New("invalid shop KYC status")
	ErrShopKYCInvalidDocType   = errors.New("invalid shop KYC document type")
)

// IsValidShopKYCStatus vérifie si le statut KYC marchand est valide
func IsValidShopKYCStatus(status ShopKYCStatus) bool {
	switch status {
	case ShopKYCStatusUnverified, ShopKYCStatusPending, ShopKYCStatusVerified, ShopKYCStatusRejected:
		return true
	}
	return false
}

// IsValidShopKYCDocType vérifie si le type de document marchand est valide
func IsValidShopKYCDocType(docType ShopKYCDocumentType) bool {
	switch docType {
	case ShopKYCDocIdentityCard, ShopKYCDocPassport, ShopKYCDocBusinessRegistry,
		ShopKYCDocTaxCertificate, ShopKYCDocBankStatement, ShopKYCDocOther:
		return true
	}
	return false
}

// ============================================================
// SHOP ENTITY
// ============================================================

// Shop représente une boutique dans le système multi-tenant
type Shop struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	CustomDomain *string
	OwnerID      string // ⚠️ string pour être compatible avec User.ID
	LogoURL      *string
	Theme        map[string]interface{}
	Plan         ShopPlan
	DBSchema     *string // Pour migration Option A future
	IsActive     bool

	// 🆕 v4.1.0 : KYC Marchand
	KYCStatus           ShopKYCStatus `json:"kyc_status" db:"kyc_status"`
	KYCSubmittedAt      *time.Time    `json:"kyc_submitted_at,omitempty" db:"kyc_submitted_at"`
	KYCVerifiedAt       *time.Time    `json:"kyc_verified_at,omitempty" db:"kyc_verified_at"`
	KYCVerifiedBy       *string       `json:"kyc_verified_by,omitempty" db:"kyc_verified_by"`
	KYCRejectionReason  *string       `json:"kyc_rejection_reason,omitempty" db:"kyc_rejection_reason"`
	KYCSubmissionsCount int           `json:"kyc_submissions_count" db:"kyc_submissions_count"`
	KYCLastSubmissionAt *time.Time    `json:"kyc_last_submission_at,omitempty" db:"kyc_last_submission_at"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// NewShop crée une nouvelle boutique
func NewShop(name, slug string, ownerID string) (*Shop, error) {
	if name == "" {
		return nil, errors.New("shop name is required")
	}
	if slug == "" {
		return nil, errors.New("shop slug is required")
	}

	return &Shop{
		ID:       uuid.New(),
		Name:     name,
		Slug:     slug,
		OwnerID:  ownerID,
		Theme:    make(map[string]interface{}),
		Plan:     ShopPlanFree,
		IsActive: true,
		// 🆕 v4.1.0 : KYC initial
		KYCStatus:           ShopKYCStatusUnverified,
		KYCSubmissionsCount: 0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil
}

// ============================================================
// MÉTHODES BASIQUES
// ============================================================

// SetCustomDomain définit un domaine personnalisé
func (s *Shop) SetCustomDomain(domain string) error {
	if domain == "" {
		s.CustomDomain = nil
		return nil
	}
	s.CustomDomain = &domain
	s.UpdatedAt = time.Now()
	return nil
}

// UpdateTheme met à jour le thème de la boutique
func (s *Shop) UpdateTheme(theme map[string]interface{}) {
	s.Theme = theme
	s.UpdatedAt = time.Now()
}

// Deactivate désactive la boutique
func (s *Shop) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

// Activate active la boutique
func (s *Shop) Activate() {
	s.IsActive = true
	s.UpdatedAt = time.Now()
}

// IsValidPlan vérifie si le plan est valide
func (s *Shop) IsValidPlan() bool {
	switch s.Plan {
	case ShopPlanFree, ShopPlanPro, ShopPlanBusiness:
		return true
	default:
		return false
	}
}

// ============================================================
// 🆕 v4.1.0 : MÉTHODES KYC
// ============================================================

// IsVerified vérifie si le marchand est vérifié KYC
func (s *Shop) IsVerified() bool {
	return s.KYCStatus == ShopKYCStatusVerified
}

// IsPendingKYC vérifie si le KYC est en attente
func (s *Shop) IsPendingKYC() bool {
	return s.KYCStatus == ShopKYCStatusPending
}

// IsUnverifiedKYC vérifie si le KYC est non vérifié
func (s *Shop) IsUnverifiedKYC() bool {
	return s.KYCStatus == ShopKYCStatusUnverified
}

// IsRejectedKYC vérifie si le KYC a été rejeté
func (s *Shop) IsRejectedKYC() bool {
	return s.KYCStatus == ShopKYCStatusRejected
}

// CanWithdraw vérifie si le marchand peut faire des retraits
// Règle : Seulement si KYC verified
func (s *Shop) CanWithdraw() bool {
	return s.KYCStatus == ShopKYCStatusVerified
}

// CanSell vérifie si le marchand peut vendre
// Règle : Tous les marchands peuvent vendre (même unverified)
func (s *Shop) CanSell() bool {
	return s.IsActive
}

// SubmitKYC soumet les documents pour vérification
// Appelé quand le marchand upload ses documents
func (s *Shop) SubmitKYC() error {
	if s.KYCStatus == ShopKYCStatusVerified {
		return ErrShopKYCAlreadyVerified
	}

	now := time.Now()
	s.KYCStatus = ShopKYCStatusPending
	s.KYCSubmittedAt = &now
	s.KYCLastSubmissionAt = &now
	s.KYCSubmissionsCount++
	s.KYCRejectionReason = nil // Reset reason si re-soumission
	s.UpdatedAt = now

	return nil
}

// ApproveKYC approuve le KYC (admin)
func (s *Shop) ApproveKYC(adminID string) error {
	if s.KYCStatus != ShopKYCStatusPending {
		return ErrShopKYCNotPending
	}

	now := time.Now()
	s.KYCStatus = ShopKYCStatusVerified
	s.KYCVerifiedAt = &now
	s.KYCVerifiedBy = &adminID
	s.KYCRejectionReason = nil
	s.UpdatedAt = now

	return nil
}

// RejectKYC rejette le KYC avec une raison (admin)
func (s *Shop) RejectKYC(adminID, reason string) error {
	if reason == "" {
		return errors.New("rejection reason is required")
	}

	now := time.Now()
	s.KYCStatus = ShopKYCStatusRejected
	s.KYCVerifiedBy = &adminID
	s.KYCRejectionReason = &reason
	s.UpdatedAt = now

	return nil
}

// ResetKYC réinitialise le KYC (pour re-soumission après rejet)
func (s *Shop) ResetKYC() error {
	if s.KYCStatus == ShopKYCStatusVerified {
		return ErrShopKYCAlreadyVerified
	}

	s.KYCStatus = ShopKYCStatusUnverified
	s.KYCRejectionReason = nil
	s.UpdatedAt = time.Now()

	return nil
}

// GetKYCBadge retourne le badge à afficher
func (s *Shop) GetKYCBadge() string {
	switch s.KYCStatus {
	case ShopKYCStatusVerified:
		return "✅ Marchand vérifié"
	case ShopKYCStatusPending:
		return "⏳ KYC en cours"
	case ShopKYCStatusRejected:
		return "❌ KYC rejeté"
	default:
		return ""
	}
}

// ============================================================
// CONFIGURATION PAIEMENT
// ============================================================

// YengaPayShopSettings représente la configuration Yenga Pay d'une boutique
type YengaPayShopSettings struct {
	Enabled        bool     `json:"enabled"`
	APIKey         string   `json:"api_key,omitempty"`         // Déchiffré
	OrganizationID string   `json:"organization_id,omitempty"` // Déchiffré
	ProjectID      string   `json:"project_id,omitempty"`      // Déchiffré
	WebhookSecret  string   `json:"webhook_secret,omitempty"`  // Déchiffré
	Operators      []string `json:"operators"`
	Env            string   `json:"env"` // "test" ou "prod"
}

// ShopPaymentSettings représente tous les settings de paiement d'une boutique
type ShopPaymentSettings struct {
	ShopID      uuid.UUID            `json:"shop_id"`
	OrangeMoney bool                 `json:"orange_money_enabled"`
	MoovMoney   bool                 `json:"moov_money_enabled"`
	Wave        bool                 `json:"wave_enabled"`
	YengaPay    YengaPayShopSettings `json:"yenga_pay"`

	// Cash à la livraison
	CashOnDeliveryEnabled bool `json:"cash_on_delivery_enabled"`
	CashCommissionRate    int  `json:"cash_commission_rate"` // basis points (250 = 2.50%)

	// 🆕 Tontine (v2.9.0)
	TontineEnabled        bool `json:"tontine_enabled"`
	TontineCommissionRate int  `json:"tontine_commission_rate"` // basis points (250 = 2.50%, max 1500 = 15%)
}

// ============================================================
// 🆕 v4.1.0 : SHOP KYC DOCUMENT
// ============================================================

// ShopKYCDocument représente un document KYC soumis par un marchand
type ShopKYCDocument struct {
	ID              uuid.UUID           `json:"id" db:"id"`
	ShopID          uuid.UUID           `json:"shop_id" db:"shop_id"`
	DocumentType    ShopKYCDocumentType `json:"document_type" db:"document_type"`
	FilePath        string              `json:"file_path" db:"file_path"`
	FileName        string              `json:"file_name" db:"file_name"`
	FileSizeBytes   int64               `json:"file_size_bytes" db:"file_size_bytes"`
	MimeType        string              `json:"mime_type" db:"mime_type"`
	Status          string              `json:"status" db:"status"` // pending, approved, rejected
	ReviewedBy      *string             `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt      *time.Time          `json:"reviewed_at,omitempty" db:"reviewed_at"`
	RejectionReason *string             `json:"rejection_reason,omitempty" db:"rejection_reason"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at" db:"updated_at"`
}

// NewShopKYCDocument crée un nouveau document KYC
func NewShopKYCDocument(
	shopID uuid.UUID,
	docType ShopKYCDocumentType,
	filePath, fileName string,
	fileSizeBytes int64,
	mimeType string,
) (*ShopKYCDocument, error) {
	if !IsValidShopKYCDocType(docType) {
		return nil, ErrShopKYCInvalidDocType
	}

	if filePath == "" || fileName == "" {
		return nil, errors.New("file path and name are required")
	}

	if fileSizeBytes <= 0 {
		return nil, errors.New("file size must be positive")
	}

	// Limite de taille : 5 Mo
	if fileSizeBytes > 5*1024*1024 {
		return nil, errors.New("file size exceeds maximum (5 MB)")
	}

	now := time.Now()
	return &ShopKYCDocument{
		ID:            uuid.New(),
		ShopID:        shopID,
		DocumentType:  docType,
		FilePath:      filePath,
		FileName:      fileName,
		FileSizeBytes: fileSizeBytes,
		MimeType:      mimeType,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// Approve approuve le document (admin)
func (d *ShopKYCDocument) Approve(adminID string) {
	now := time.Now()
	d.Status = "approved"
	d.ReviewedBy = &adminID
	d.ReviewedAt = &now
	d.RejectionReason = nil
	d.UpdatedAt = now
}

// Reject rejette le document avec une raison (admin)
func (d *ShopKYCDocument) Reject(adminID, reason string) error {
	if reason == "" {
		return errors.New("rejection reason is required")
	}
	now := time.Now()
	d.Status = "rejected"
	d.ReviewedBy = &adminID
	d.ReviewedAt = &now
	d.RejectionReason = &reason
	d.UpdatedAt = now
	return nil
}

// ============================================================
// MÉTHODES SHOP PAYMENT SETTINGS
// ============================================================

// IsYengaPayEnabled vérifie si Yenga Pay est activé pour cette boutique
func (s *ShopPaymentSettings) IsYengaPayEnabled() bool {
	return s.YengaPay.Enabled
}

// IsOperatorEnabled vérifie si un opérateur Yenga Pay est activé
func (s *YengaPayShopSettings) IsOperatorEnabled(operator string) bool {
	for _, op := range s.Operators {
		if op == operator {
			return true
		}
	}
	return false
}

// IsCashOnDeliveryEnabled vérifie si le cash à la livraison est activé
func (s *ShopPaymentSettings) IsCashOnDeliveryEnabled() bool {
	return s.CashOnDeliveryEnabled
}

// GetCashCommissionRate retourne le taux de commission cash en basis points
// Retourne 250 (2.50%) par défaut si la valeur est invalide
func (s *ShopPaymentSettings) GetCashCommissionRate() int {
	if s.CashCommissionRate <= 0 || s.CashCommissionRate > 10000 {
		return 250 // Défaut : 2.50%
	}
	return s.CashCommissionRate
}

// IsTontineEnabled vérifie si la tontine est activée pour cette boutique
func (s *ShopPaymentSettings) IsTontineEnabled() bool {
	return s.TontineEnabled
}

// GetTontineCommissionRate retourne le taux de commission tontine en basis points
// Retourne 250 (2.50%) par défaut si la valeur est 0 ou invalide
// Max autorisé : 1500 (15%)
func (s *ShopPaymentSettings) GetTontineCommissionRate() int {
	if s.TontineCommissionRate <= 0 || s.TontineCommissionRate > 1500 {
		return 250 // Défaut : 2.50%
	}
	return s.TontineCommissionRate
}
