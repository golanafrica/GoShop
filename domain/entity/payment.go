package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// PaymentStatus représente le statut d'un paiement
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusSuccess    PaymentStatus = "success"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusRefunded   PaymentStatus = "refunded"
	PaymentStatusCancelled  PaymentStatus = "cancelled"
	PaymentStatusExpired    PaymentStatus = "expired"
)

// PaymentProvider représente le fournisseur de paiement
type PaymentProvider string

const (
	ProviderOrangeMoney PaymentProvider = "orange_money"
	ProviderMoovMoney   PaymentProvider = "moov_money"
	ProviderWave        PaymentProvider = "wave"
	ProviderYengaPay    PaymentProvider = "yenga_pay"
	ProviderCash        PaymentProvider = "cash"
	ProviderMock        PaymentProvider = "mock"
)

// CommissionStatus
const (
	CommissionStatusPending   = "pending"
	CommissionStatusCollected = "collected"
	CommissionStatusFailed    = "failed"
)

// Currency représente la devise
type Currency string

const (
	CurrencyXOF Currency = "XOF"
	CurrencyXAF Currency = "XAF"
)

// Payment représente un paiement
type Payment struct {
	ID      uuid.UUID `json:"id" db:"id"`
	ShopID  uuid.UUID `json:"shop_id" db:"shop_id"`
	OrderID uuid.UUID `json:"order_id" db:"order_id"`

	ReferenceType *string `json:"reference_type,omitempty" db:"reference_type"`
	ReferenceID   *string `json:"reference_id,omitempty" db:"reference_id"`

	Provider      PaymentProvider        `json:"provider" db:"provider"`
	ProviderRef   *string                `json:"provider_ref,omitempty" db:"provider_ref"`
	AmountCents   int64                  `json:"amount_cents" db:"amount_cents"`
	Currency      Currency               `json:"currency" db:"currency"`
	CustomerPhone *string                `json:"customer_phone,omitempty" db:"customer_phone"`
	CustomerEmail *string                `json:"customer_email,omitempty" db:"customer_email"`
	Description   *string                `json:"description,omitempty" db:"description"`
	Status        PaymentStatus          `json:"status" db:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	InitiatedAt   *time.Time             `json:"initiated_at,omitempty" db:"initiated_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty" db:"expires_at"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`

	// 🛡️ CORRECTION AUDIT : Champ pour traçabilité d'idempotence webhook
	WebhookExternalID *string `json:"webhook_external_id,omitempty" db:"webhook_external_id"`

	// 🆕 CORRECTION DOUBLE PRÉLÈVEMENT : Frais prélevés par le PSP (YengaPay)
	ProviderFeesCents int64 `json:"provider_fees_cents" db:"provider_fees_cents"`

	// Commission
	CommissionRateBps     int        `json:"commission_rate_bps" db:"commission_rate_bps"`
	CommissionCents       int64      `json:"commission_cents" db:"commission_cents"`
	CommissionStatus      string     `json:"commission_status" db:"commission_status"`
	CommissionCollectedAt *time.Time `json:"commission_collected_at,omitempty" db:"commission_collected_at"`
}

// IsValidStatusTransition vérifie si une transition de statut est valide
func (p *Payment) IsValidStatusTransition(newStatus PaymentStatus) bool {
	transitions := map[PaymentStatus][]PaymentStatus{
		// 🆕 AJOUT CRITIQUE : PaymentStatusSuccess autorisé depuis Pending pour gérer le flux ONE_STEP (ex: Orange Money sandbox avec OTP immédiat)
		PaymentStatusPending:    {PaymentStatusProcessing, PaymentStatusSuccess, PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired},
		PaymentStatusProcessing: {PaymentStatusSuccess, PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired},
		PaymentStatusSuccess:    {PaymentStatusRefunded},
		PaymentStatusFailed:     {},
		PaymentStatusRefunded:   {},
		PaymentStatusCancelled:  {},
		PaymentStatusExpired:    {},
	}

	allowed, exists := transitions[p.Status]
	if !exists {
		return false
	}

	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}

// NewPayment crée un nouveau paiement en statut "pending"
func NewPayment(shopID, orderID uuid.UUID, provider PaymentProvider, amountCents int64) (*Payment, error) {
	if amountCents <= 0 {
		return nil, errors.New("payment amount must be positive")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Minute)

	return &Payment{
		ID:          uuid.New(),
		ShopID:      shopID,
		OrderID:     orderID,
		Provider:    provider,
		AmountCents: amountCents,
		Currency:    CurrencyXOF,
		Status:      PaymentStatusPending,
		Metadata:    make(map[string]interface{}),
		ExpiresAt:   &expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// MarkProcessing marque le paiement comme en cours de traitement
func (p *Payment) MarkProcessing() error {
	if !p.IsValidStatusTransition(PaymentStatusProcessing) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusProcessing
	p.InitiatedAt = &now
	p.UpdatedAt = now
	return nil
}

// MarkSuccess marque le paiement comme réussi
func (p *Payment) MarkSuccess(providerRef string) error {
	if !p.IsValidStatusTransition(PaymentStatusSuccess) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusSuccess
	p.ProviderRef = &providerRef
	p.CompletedAt = &now
	p.UpdatedAt = now
	return nil
}

// MarkSuccessWithWebhook marque le paiement avec traçabilité webhook
func (p *Payment) MarkSuccessWithWebhook(providerRef string, webhookExternalID string) error {
	if err := p.MarkSuccess(providerRef); err != nil {
		return err
	}
	if webhookExternalID != "" {
		p.WebhookExternalID = &webhookExternalID
	}
	return nil
}

// MarkFailed marque le paiement comme échoué
func (p *Payment) MarkFailed(reason string) error {
	if !p.IsValidStatusTransition(PaymentStatusFailed) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusFailed
	p.UpdatedAt = now
	if p.Metadata == nil {
		p.Metadata = make(map[string]interface{})
	}
	p.Metadata["failure_reason"] = reason
	return nil
}

// MarkRefunded marque le paiement comme remboursé
func (p *Payment) MarkRefunded() error {
	if !p.IsValidStatusTransition(PaymentStatusRefunded) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusRefunded
	p.CompletedAt = &now
	p.UpdatedAt = now
	return nil
}

// MarkCancelled marque le paiement comme annulé
func (p *Payment) MarkCancelled() error {
	if !p.IsValidStatusTransition(PaymentStatusCancelled) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusCancelled
	p.UpdatedAt = now
	return nil
}

// MarkExpired marque le paiement comme expiré
func (p *Payment) MarkExpired() error {
	if !p.IsValidStatusTransition(PaymentStatusExpired) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC()
	p.Status = PaymentStatusExpired
	p.UpdatedAt = now
	return nil
}

// IsTerminal retourne true si le paiement est dans un état terminal
func (p *Payment) IsTerminal() bool {
	switch p.Status {
	case PaymentStatusSuccess, PaymentStatusFailed, PaymentStatusRefunded,
		PaymentStatusCancelled, PaymentStatusExpired:
		return true
	default:
		return false
	}
}

// IsExpired vérifie si le paiement a expiré
func (p *Payment) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*p.ExpiresAt) && !p.IsTerminal()
}
