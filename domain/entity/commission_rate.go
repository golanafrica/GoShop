package entity

import "time"

// CommissionRate représente un taux de commission configurable par boutique
type CommissionRate struct {
	ID                 string    `json:"id" db:"id"`
	ShopID             string    `json:"shop_id" db:"shop_id"`
	TransactionType    string    `json:"transaction_type" db:"transaction_type"` // online_payment, cod, tontine_solo, tontine_group, credit
	RateBps            int       `json:"rate_bps" db:"rate_bps"`                 // 250 = 2.5%
	MinCommissionCents int64     `json:"min_commission_cents" db:"min_commission_cents"`
	MaxCommissionCents int64     `json:"max_commission_cents" db:"max_commission_cents"`
	IsActive           bool      `json:"is_active" db:"is_active"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy          *string   `json:"created_by,omitempty" db:"created_by"`
}

// Types de transactions
const (
	TransactionTypeOnlinePayment = "online_payment"
	TransactionTypeCOD           = "cod"
	TransactionTypeTontineSolo   = "tontine_solo"
	TransactionTypeTontineGroup  = "tontine_group"
	TransactionTypeCredit        = "credit"
)

// Taux par défaut (en basis points)
const (
	DefaultRateOnlinePayment = 250 // 2.5%
	DefaultRateCOD           = 250 // 2.5%
	DefaultRateTontineSolo   = 200 // 2%
	DefaultRateTontineGroup  = 150 // 1.5%
	DefaultRateCredit        = 50  // 0.5%
)

// CalculateCommission calcule la commission pour un montant donné
func (cr *CommissionRate) CalculateCommission(amountCents int64) int64 {
	if !cr.IsActive || amountCents <= 0 {
		return 0
	}

	// Calcul : amount * rate / 10000
	commission := amountCents * int64(cr.RateBps) / 10000

	// Appliquer les limites
	if commission < cr.MinCommissionCents {
		commission = cr.MinCommissionCents
	}
	if cr.MaxCommissionCents > 0 && commission > cr.MaxCommissionCents {
		commission = cr.MaxCommissionCents
	}

	return commission
}

// Validate valide le taux de commission
func (cr *CommissionRate) Validate() error {
	if cr.ShopID == "" {
		return ErrInvalidShopID
	}
	if cr.TransactionType == "" {
		return ErrInvalidTransactionType
	}
	if cr.RateBps < 0 || cr.RateBps > 1500 { // Max 15% (BCEAO)
		return ErrInvalidCommissionRate
	}
	if cr.MinCommissionCents < 0 {
		return ErrInvalidMinCommission
	}
	if cr.MaxCommissionCents < 0 {
		return ErrInvalidMaxCommission
	}
	if cr.MaxCommissionCents > 0 && cr.MinCommissionCents > cr.MaxCommissionCents {
		return ErrMinGreaterThanMax
	}
	return nil
}
