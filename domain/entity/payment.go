package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// PaymentStatus représente le statut d'un paiement
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"    // Créé, en attente
	PaymentStatusProcessing PaymentStatus = "processing" // En cours de traitement
	PaymentStatusSuccess    PaymentStatus = "success"    // Réussi
	PaymentStatusFailed     PaymentStatus = "failed"     // Échoué
	PaymentStatusRefunded   PaymentStatus = "refunded"   // Remboursé
	PaymentStatusCancelled  PaymentStatus = "cancelled"  // Annulé
	PaymentStatusExpired    PaymentStatus = "expired"    // Expiré
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
	CurrencyXOF Currency = "XOF" // Franc CFA (UEMOA)
	CurrencyXAF Currency = "XAF" // Franc CFA (CEMAC)
)

// Payment représente un paiement
type Payment struct {
	ID      uuid.UUID `json:"id" db:"id"`
	ShopID  uuid.UUID `json:"shop_id" db:"shop_id"`
	OrderID uuid.UUID `json:"order_id" db:"order_id"`

	// 🆕 Champs polymorphes pour supporter les échéances de crédit, tontines, etc.
	ReferenceType *string `json:"reference_type,omitempty" db:"reference_type"` // ex: "order", "credit_installment"
	ReferenceID   *string `json:"reference_id,omitempty" db:"reference_id"`     // UUID en format string

	Provider      PaymentProvider        `json:"provider" db:"provider"`
	ProviderRef   *string                `json:"provider_ref,omitempty" db:"provider_ref"` // Référence côté provider (peut être nil avant initiation)
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

	// Commission
	CommissionRateBps     int        `json:"commission_rate_bps" db:"commission_rate_bps"`
	CommissionCents       int64      `json:"commission_cents" db:"commission_cents"`
	CommissionStatus      string     `json:"commission_status" db:"commission_status"` // pending, collected, failed
	CommissionCollectedAt *time.Time `json:"commission_collected_at,omitempty" db:"commission_collected_at"`
}

// IsValidStatusTransition vérifie si une transition de statut est valide
func (p *Payment) IsValidStatusTransition(newStatus PaymentStatus) bool {
	transitions := map[PaymentStatus][]PaymentStatus{
		PaymentStatusPending:    {PaymentStatusProcessing, PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired},
		PaymentStatusProcessing: {PaymentStatusSuccess, PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired},
		PaymentStatusSuccess:    {PaymentStatusRefunded},
		PaymentStatusFailed:     {}, // Terminal
		PaymentStatusRefunded:   {}, // Terminal
		PaymentStatusCancelled:  {}, // Terminal
		PaymentStatusExpired:    {}, // Terminal
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

	now := time.Now().UTC()                // ✅ UTC explicite
	expiresAt := now.Add(30 * time.Minute) // Expiration par défaut : 30 min

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
	now := time.Now().UTC() // ✅ UTC explicite
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
	now := time.Now().UTC() // ✅ UTC explicite
	p.Status = PaymentStatusSuccess
	p.ProviderRef = &providerRef
	p.CompletedAt = &now
	p.UpdatedAt = now
	return nil
}

// MarkFailed marque le paiement comme échoué
func (p *Payment) MarkFailed(reason string) error {
	if !p.IsValidStatusTransition(PaymentStatusFailed) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC() // ✅ UTC explicite
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
	now := time.Now().UTC() // ✅ UTC explicite
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
	now := time.Now().UTC() // ✅ UTC explicite
	p.Status = PaymentStatusCancelled
	p.UpdatedAt = now
	return nil
}

// MarkExpired marque le paiement comme expiré
func (p *Payment) MarkExpired() error {
	if !p.IsValidStatusTransition(PaymentStatusExpired) {
		return errors.New("invalid status transition from " + string(p.Status))
	}
	now := time.Now().UTC() // ✅ UTC explicite
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
	return time.Now().UTC().After(*p.ExpiresAt) && !p.IsTerminal() // ✅ UTC explicite
}
