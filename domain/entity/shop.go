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
	ShopKYCStatusUnverified ShopKYCStatus = "unverified"
	ShopKYCStatusPending    ShopKYCStatus = "pending"
	ShopKYCStatusVerified   ShopKYCStatus = "verified"
	ShopKYCStatusRejected   ShopKYCStatus = "rejected"
)

// ShopKYCDocumentType représente le type de document KYC marchand
type ShopKYCDocumentType string

const (
	ShopKYCDocIdentityCard     ShopKYCDocumentType = "identity_card"
	ShopKYCDocPassport         ShopKYCDocumentType = "passport"
	ShopKYCDocBusinessRegistry ShopKYCDocumentType = "business_registry"
	ShopKYCDocTaxCertificate   ShopKYCDocumentType = "tax_certificate"
	ShopKYCDocBankStatement    ShopKYCDocumentType = "bank_statement"
	ShopKYCDocOther            ShopKYCDocumentType = "other"
)

// Erreurs KYC
var (
	ErrShopKYCAlreadyVerified  = errors.New("shop KYC already verified")
	ErrShopKYCNotPending       = errors.New("shop KYC not in pending status")
	ErrShopKYCInsufficientDocs = errors.New("minimum 2 documents required (identity + business registry)")
	ErrShopKYCCannotWithdraw   = errors.New("merchant KYC verification required for withdrawals")
	ErrShopKYCInvalidStatus    = errors.New("invalid shop KYC status")
	ErrShopKYCInvalidDocType   = errors.New("invalid shop KYC document type")
)

// IsValidKYCStatus vérifie si le statut KYC est valide
func IsValidKYCStatus(status ShopKYCStatus) bool {
	switch status {
	case ShopKYCStatusUnverified, ShopKYCStatusPending, ShopKYCStatusVerified, ShopKYCStatusRejected:
		return true
	}
	return false
}

// IsValidShopKYCDocType vérifie si le type de document marchand est valide
// 🆕 v4.1.0 : Préfixé "Shop" pour cohérence avec ShopKYCStatus, ShopKYCDocumentType
func IsValidShopKYCDocType(docType ShopKYCDocumentType) bool {
	switch docType {
	case ShopKYCDocIdentityCard, ShopKYCDocPassport, ShopKYCDocBusinessRegistry,
		ShopKYCDocTaxCertificate, ShopKYCDocBankStatement, ShopKYCDocOther:
		return true
	}
	return false
}

// ============================================================
// 🆕 v4.2.0 : ADMIN SHOP MANAGEMENT
// ============================================================

// ShopHealthLevel représente le niveau de santé d'une boutique
type ShopHealthLevel string

const (
	ShopHealthExcellent ShopHealthLevel = "excellent" // 800-1000 🟢
	ShopHealthGood      ShopHealthLevel = "good"      // 600-799  🔵
	ShopHealthWarning   ShopHealthLevel = "warning"   // 400-599  🟡
	ShopHealthCritical  ShopHealthLevel = "critical"  // 0-399    🔴
)

// ShopAdminActionType représente le type d'action admin
type ShopAdminActionType string

const (
	ShopActionSuspend      ShopAdminActionType = "suspend"
	ShopActionActivate     ShopAdminActionType = "activate"
	ShopActionChangePlan   ShopAdminActionType = "change_plan"
	ShopActionUpdateHealth ShopAdminActionType = "update_health"
	ShopActionAddNote      ShopAdminActionType = "add_note"
	ShopActionReview       ShopAdminActionType = "review"
	ShopActionExportData   ShopAdminActionType = "export_data"
)

// Erreurs Admin
var (
	ErrShopAlreadySuspended     = errors.New("shop is already suspended")
	ErrShopNotSuspended         = errors.New("shop is not suspended")
	ErrShopAlreadyActive        = errors.New("shop is already active")
	ErrSuspensionReasonRequired = errors.New("suspension reason is required")
	ErrInvalidHealthScore       = errors.New("health score must be between 0 and 1000")
	ErrInvalidHealthLevel       = errors.New("invalid health level")
	ErrInvalidAdminAction       = errors.New("invalid admin action type")
)

// IsValidHealthLevel vérifie si le niveau de santé est valide
func IsValidHealthLevel(level ShopHealthLevel) bool {
	switch level {
	case ShopHealthExcellent, ShopHealthGood, ShopHealthWarning, ShopHealthCritical:
		return true
	}
	return false
}

// IsValidAdminAction vérifie si l'action admin est valide
func IsValidAdminAction(action ShopAdminActionType) bool {
	switch action {
	case ShopActionSuspend, ShopActionActivate, ShopActionChangePlan,
		ShopActionUpdateHealth, ShopActionAddNote, ShopActionReview, ShopActionExportData:
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
	OwnerID      string
	LogoURL      *string
	Theme        map[string]interface{}
	Plan         ShopPlan
	DBSchema     *string
	IsActive     bool

	// 🆕 v4.1.0 : KYC Marchand
	KYCStatus           ShopKYCStatus `json:"kyc_status" db:"kyc_status"`
	KYCSubmittedAt      *time.Time    `json:"kyc_submitted_at,omitempty" db:"kyc_submitted_at"`
	KYCVerifiedAt       *time.Time    `json:"kyc_verified_at,omitempty" db:"kyc_verified_at"`
	KYCVerifiedBy       *string       `json:"kyc_verified_by,omitempty" db:"kyc_verified_by"`
	KYCRejectionReason  *string       `json:"kyc_rejection_reason,omitempty" db:"kyc_rejection_reason"`
	KYCSubmissionsCount int           `json:"kyc_submissions_count" db:"kyc_submissions_count"`
	KYCLastSubmissionAt *time.Time    `json:"kyc_last_submission_at,omitempty" db:"kyc_last_submission_at"`

	// 🆕 v4.2.0 : Admin Shop Management
	SuspendedAt      *time.Time `json:"suspended_at,omitempty" db:"suspended_at"`
	SuspendedBy      *string    `json:"suspended_by,omitempty" db:"suspended_by"`
	SuspensionReason *string    `json:"suspension_reason,omitempty" db:"suspension_reason"`

	// 🆕 v4.2.0 : Account Health Score (inspiré d'Amazon)
	HealthScore     int             `json:"health_score" db:"health_score"`
	HealthLevel     ShopHealthLevel `json:"health_level" db:"health_level"`
	HealthUpdatedAt *time.Time      `json:"health_updated_at,omitempty" db:"health_updated_at"`

	// 🆕 v4.2.0 : Notes et revue admin
	AdminNotes     *string    `json:"admin_notes,omitempty" db:"admin_notes"`
	LastReviewedAt *time.Time `json:"last_reviewed_at,omitempty" db:"last_reviewed_at"`
	LastReviewedBy *string    `json:"last_reviewed_by,omitempty" db:"last_reviewed_by"`

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
		// 🆕 v4.2.0 : Health Score initial
		HealthScore: 1000,
		HealthLevel: ShopHealthExcellent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// ============================================================
// MÉTHODES BASIQUES
// ============================================================

func (s *Shop) SetCustomDomain(domain string) error {
	if domain == "" {
		s.CustomDomain = nil
		return nil
	}
	s.CustomDomain = &domain
	s.UpdatedAt = time.Now()
	return nil
}

func (s *Shop) UpdateTheme(theme map[string]interface{}) {
	s.Theme = theme
	s.UpdatedAt = time.Now()
}

func (s *Shop) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

func (s *Shop) Activate() {
	s.IsActive = true
	s.UpdatedAt = time.Now()
}

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

func (s *Shop) IsVerified() bool {
	return s.KYCStatus == ShopKYCStatusVerified
}

func (s *Shop) IsPendingKYC() bool {
	return s.KYCStatus == ShopKYCStatusPending
}

func (s *Shop) IsUnverifiedKYC() bool {
	return s.KYCStatus == ShopKYCStatusUnverified
}

func (s *Shop) IsRejectedKYC() bool {
	return s.KYCStatus == ShopKYCStatusRejected
}

func (s *Shop) CanWithdraw() bool {
	return s.KYCStatus == ShopKYCStatusVerified
}

func (s *Shop) CanSell() bool {
	return s.IsActive
}

func (s *Shop) SubmitKYC() error {
	if s.KYCStatus == ShopKYCStatusVerified {
		return ErrShopKYCAlreadyVerified
	}

	now := time.Now()
	s.KYCStatus = ShopKYCStatusPending
	s.KYCSubmittedAt = &now
	s.KYCLastSubmissionAt = &now
	s.KYCSubmissionsCount++
	s.KYCRejectionReason = nil
	s.UpdatedAt = now

	return nil
}

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

func (s *Shop) ResetKYC() error {
	if s.KYCStatus == ShopKYCStatusVerified {
		return ErrShopKYCAlreadyVerified
	}

	s.KYCStatus = ShopKYCStatusUnverified
	s.KYCRejectionReason = nil
	s.UpdatedAt = time.Now()

	return nil
}

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
// 🆕 v4.2.0 : MÉTHODES ADMIN SHOP MANAGEMENT
// ============================================================

// IsSuspended vérifie si la boutique est suspendue
func (s *Shop) IsSuspended() bool {
	return s.SuspendedAt != nil
}

// Suspend suspend la boutique (admin)
func (s *Shop) Suspend(adminID, reason string) error {
	if s.IsSuspended() {
		return ErrShopAlreadySuspended
	}
	if reason == "" {
		return ErrSuspensionReasonRequired
	}

	now := time.Now()
	s.SuspendedAt = &now
	s.SuspendedBy = &adminID
	s.SuspensionReason = &reason
	s.IsActive = false // Désactiver automatiquement
	s.UpdatedAt = now

	return nil
}

// Activate réactive la boutique (admin)
func (s *Shop) ActivateByAdmin(adminID string) error {
	if !s.IsSuspended() {
		return ErrShopNotSuspended
	}

	now := time.Now()
	s.SuspendedAt = nil
	s.SuspendedBy = nil
	s.SuspensionReason = nil
	s.IsActive = true
	s.UpdatedAt = now

	return nil
}

// ChangePlan change le plan d'abonnement (super_admin)
func (s *Shop) ChangePlan(newPlan ShopPlan, adminID string) error {
	if !s.IsValidPlan() {
		return errors.New("invalid plan")
	}

	s.Plan = newPlan
	s.UpdatedAt = time.Now()

	return nil
}

// AddNote ajoute une note admin
func (s *Shop) AddNote(note string, adminID string) error {
	if note == "" {
		return errors.New("note cannot be empty")
	}

	now := time.Now()
	if s.AdminNotes == nil {
		s.AdminNotes = &note
	} else {
		updatedNotes := *s.AdminNotes + "\n\n[" + now.Format("2006-01-02 15:04") + " - " + adminID + "]\n" + note
		s.AdminNotes = &updatedNotes
	}
	s.UpdatedAt = now

	return nil
}

// MarkReviewed marque la boutique comme revue par un admin
func (s *Shop) MarkReviewed(adminID string) {
	now := time.Now()
	s.LastReviewedAt = &now
	s.LastReviewedBy = &adminID
	s.UpdatedAt = now
}

// ============================================================
// 🆕 v4.2.0 : ACCOUNT HEALTH SCORE (inspiré d'Amazon)
// ============================================================

// CalculateHealthScore calcule le score de santé (0-1000)
func (s *Shop) CalculateHealthScore() int {
	score := 1000

	// KYC non vérifié : -200
	if s.KYCStatus != ShopKYCStatusVerified {
		score -= 200
	}

	// Shop inactif : -300
	if !s.IsActive {
		score -= 300
	}

	// Shop suspendu : -500
	if s.IsSuspended() {
		score -= 500
	}

	// Ancienneté > 6 mois : +100
	if time.Since(s.CreatedAt) > 6*30*24*time.Hour {
		score += 100
	}

	// Limiter entre 0 et 1000
	if score < 0 {
		score = 0
	}
	if score > 1000 {
		score = 1000
	}

	return score
}

// UpdateHealthScore met à jour le score et le niveau
func (s *Shop) UpdateHealthScore() {
	s.HealthScore = s.CalculateHealthScore()
	s.HealthLevel = s.GetHealthLevelFromScore(s.HealthScore)
	now := time.Now()
	s.HealthUpdatedAt = &now
	s.UpdatedAt = now
}

// GetHealthLevelFromScore retourne le niveau basé sur le score
func (s *Shop) GetHealthLevelFromScore(score int) ShopHealthLevel {
	switch {
	case score >= 800:
		return ShopHealthExcellent
	case score >= 600:
		return ShopHealthGood
	case score >= 400:
		return ShopHealthWarning
	default:
		return ShopHealthCritical
	}
}

// GetHealthEmoji retourne l'emoji du niveau de santé
func (s *Shop) GetHealthEmoji() string {
	switch s.HealthLevel {
	case ShopHealthExcellent:
		return "🟢"
	case ShopHealthGood:
		return "🔵"
	case ShopHealthWarning:
		return "🟡"
	case ShopHealthCritical:
		return "🔴"
	default:
		return "⚪"
	}
}

// GetHealthDescription retourne la description du niveau
func (s *Shop) GetHealthDescription() string {
	switch s.HealthLevel {
	case ShopHealthExcellent:
		return "Excellent - Aucune action requise"
	case ShopHealthGood:
		return "Bon - Surveillance recommandée"
	case ShopHealthWarning:
		return "Attention - Actions correctives recommandées"
	case ShopHealthCritical:
		return "Critique - Action immédiate requise"
	default:
		return "Inconnu"
	}
}

// NeedsImmediateAction vérifie si une action immédiate est requise
func (s *Shop) NeedsImmediateAction() bool {
	return s.HealthLevel == ShopHealthCritical || s.IsSuspended()
}

// ============================================================
// CONFIGURATION PAIEMENT
// ============================================================

type YengaPayShopSettings struct {
	Enabled        bool     `json:"enabled"`
	APIKey         string   `json:"api_key,omitempty"`
	OrganizationID string   `json:"organization_id,omitempty"`
	ProjectID      string   `json:"project_id,omitempty"`
	WebhookSecret  string   `json:"webhook_secret,omitempty"`
	Operators      []string `json:"operators"`
	Env            string   `json:"env"`
}

type ShopPaymentSettings struct {
	ShopID      uuid.UUID            `json:"shop_id"`
	OrangeMoney bool                 `json:"orange_money_enabled"`
	MoovMoney   bool                 `json:"moov_money_enabled"`
	Wave        bool                 `json:"wave_enabled"`
	YengaPay    YengaPayShopSettings `json:"yenga_pay"`

	CashOnDeliveryEnabled bool `json:"cash_on_delivery_enabled"`
	CashCommissionRate    int  `json:"cash_commission_rate"`

	TontineEnabled        bool `json:"tontine_enabled"`
	TontineCommissionRate int  `json:"tontine_commission_rate"`
}

func (s *ShopPaymentSettings) IsYengaPayEnabled() bool {
	return s.YengaPay.Enabled
}

func (s *YengaPayShopSettings) IsOperatorEnabled(operator string) bool {
	for _, op := range s.Operators {
		if op == operator {
			return true
		}
	}
	return false
}

func (s *ShopPaymentSettings) IsCashOnDeliveryEnabled() bool {
	return s.CashOnDeliveryEnabled
}

func (s *ShopPaymentSettings) GetCashCommissionRate() int {
	if s.CashCommissionRate <= 0 || s.CashCommissionRate > 10000 {
		return 250
	}
	return s.CashCommissionRate
}

func (s *ShopPaymentSettings) IsTontineEnabled() bool {
	return s.TontineEnabled
}

func (s *ShopPaymentSettings) GetTontineCommissionRate() int {
	if s.TontineCommissionRate <= 0 || s.TontineCommissionRate > 1500 {
		return 250
	}
	return s.TontineCommissionRate
}

// ============================================================
// 🆕 v4.1.0 : SHOP KYC DOCUMENT
// ============================================================

type ShopKYCDocument struct {
	ID              uuid.UUID           `json:"id" db:"id"`
	ShopID          uuid.UUID           `json:"shop_id" db:"shop_id"`
	DocumentType    ShopKYCDocumentType `json:"document_type" db:"document_type"`
	FilePath        string              `json:"file_path" db:"file_path"`
	FileName        string              `json:"file_name" db:"file_name"`
	FileSizeBytes   int64               `json:"file_size_bytes" db:"file_size_bytes"`
	MimeType        string              `json:"mime_type" db:"mime_type"`
	Status          string              `json:"status" db:"status"`
	ReviewedBy      *string             `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt      *time.Time          `json:"reviewed_at,omitempty" db:"reviewed_at"`
	RejectionReason *string             `json:"rejection_reason,omitempty" db:"rejection_reason"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at" db:"updated_at"`
}

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

func (d *ShopKYCDocument) Approve(adminID string) {
	now := time.Now()
	d.Status = "approved"
	d.ReviewedBy = &adminID
	d.ReviewedAt = &now
	d.RejectionReason = nil
	d.UpdatedAt = now
}

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
// 🆕 v4.2.0 : SHOP ADMIN ACTION (audit trail)
// ============================================================

type ShopAdminAction struct {
	ID         uuid.UUID           `json:"id" db:"id"`
	ShopID     uuid.UUID           `json:"shop_id" db:"shop_id"`
	AdminID    string              `json:"admin_id" db:"admin_id"`
	AdminEmail string              `json:"admin_email" db:"admin_email"`
	AdminRole  string              `json:"admin_role" db:"admin_role"`
	ActionType ShopAdminActionType `json:"action_type" db:"action_type"`
	OldValue   interface{}         `json:"old_value,omitempty" db:"old_value"`
	NewValue   interface{}         `json:"new_value,omitempty" db:"new_value"`
	Reason     string              `json:"reason,omitempty" db:"reason"`
	IPAddress  string              `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  string              `json:"user_agent,omitempty" db:"user_agent"`
	RequestID  string              `json:"request_id,omitempty" db:"request_id"`
	CreatedAt  time.Time           `json:"created_at" db:"created_at"`
}

// NewShopAdminAction crée une nouvelle action admin
func NewShopAdminAction(
	shopID uuid.UUID,
	adminID, adminEmail, adminRole string,
	actionType ShopAdminActionType,
	reason string,
) (*ShopAdminAction, error) {
	if !IsValidAdminAction(actionType) {
		return nil, ErrInvalidAdminAction
	}

	return &ShopAdminAction{
		ID:         uuid.New(),
		ShopID:     shopID,
		AdminID:    adminID,
		AdminEmail: adminEmail,
		AdminRole:  adminRole,
		ActionType: actionType,
		Reason:     reason,
		CreatedAt:  time.Now(),
	}, nil
}
