package withdrawaldto

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// CreateWithdrawalRequest représente une demande de création de retrait
type CreateWithdrawalRequest struct {
	AmountCents       int64  `json:"amount_cents"`
	PaymentMethod     string `json:"payment_method"`
	DestinationNumber string `json:"destination_number"`
	DestinationName   string `json:"destination_name,omitempty"`
	DestinationEmail  string `json:"destination_email,omitempty"`
	Description       string `json:"description,omitempty"`
}

// Validate valide la requête
func (r *CreateWithdrawalRequest) Validate() error {
	if r.AmountCents <= 0 {
		return errors.New("amount_cents must be positive")
	}
	if r.AmountCents < 10000 { // Minimum 100 XOF = 10000 centimes
		return errors.New("minimum withdrawal amount is 100 XOF")
	}
	if r.PaymentMethod == "" {
		return errors.New("payment_method is required")
	}

	// Normaliser en majuscules
	validMethods := map[string]bool{
		"ORANGE_MONEY":  true,
		"MOOV_MONEY":    true,
		"TELECEL_MONEY": true,
		"CORIS_MONEY":   true,
		"SANK_MONEY":    true,
		"MTN":           true,
	}
	method := strings.ToUpper(r.PaymentMethod)
	if !validMethods[method] {
		return errors.New("invalid payment_method: " + r.PaymentMethod)
	}
	r.PaymentMethod = method

	if r.DestinationNumber == "" {
		return errors.New("destination_number is required")
	}
	// Validation basique du numéro (format Burkina Faso)
	if len(r.DestinationNumber) < 8 {
		return errors.New("destination_number is too short")
	}
	return nil
}

// WithdrawalResponse représente la réponse d'un retrait
type WithdrawalResponse struct {
	ID                    string `json:"id"`
	ShopID                string `json:"shop_id"`
	Provider              string `json:"provider"`
	ProviderRef           string `json:"provider_ref,omitempty"`
	AmountCents           int64  `json:"amount_cents"`
	Currency              string `json:"currency"`
	FeesCents             int64  `json:"fees_cents"`
	NetAmountCents        int64  `json:"net_amount_cents"`
	Status                string `json:"status"`
	PaymentMethod         string `json:"payment_method"`
	DestinationNumber     string `json:"destination_number"`
	DestinationName       string `json:"destination_name,omitempty"`
	DestinationEmail      string `json:"destination_email,omitempty"`
	Description           string `json:"description,omitempty"`
	ErrorMessage          string `json:"error_message,omitempty"`
	OperatorTransactionID string `json:"operator_transaction_id,omitempty"`
	ProcessedAt           string `json:"processed_at,omitempty"`
	CreatedAt             string `json:"created_at"`
}

// ListWithdrawalsRequest représente une demande de liste
type ListWithdrawalsRequest struct {
	ShopID uuid.UUID
	Limit  int
	Offset int
}
