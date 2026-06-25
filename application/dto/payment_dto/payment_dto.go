package paymentdto

import (
	"errors"

	"Goshop/domain/entity"
)

// InitiatePaymentRequest représente la requête pour initier un paiement
type InitiatePaymentRequest struct {
	OrderID       string                 `json:"order_id"`
	Provider      entity.PaymentProvider `json:"provider"`
	PhoneNumber   string                 `json:"phone_number"`
	CustomerEmail string                 `json:"customer_email,omitempty"`
	Description   string                 `json:"description,omitempty"`
	CallbackURL   string                 `json:"callback_url,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Validate valide la requête d'initiation
func (r *InitiatePaymentRequest) Validate() error {
	if r.OrderID == "" {
		return errors.New("order_id is required")
	}

	if r.Provider == "" {
		return errors.New("provider is required")
	}

	// Validation du provider
	validProviders := map[entity.PaymentProvider]bool{
		entity.ProviderOrangeMoney: true,
		entity.ProviderMoovMoney:   true,
		entity.ProviderYengaPay:    true,
		entity.ProviderWave:        true,
		entity.ProviderCash:        true,
		entity.ProviderMock:        true,
	}
	if !validProviders[r.Provider] {
		return errors.New("invalid provider")
	}

	if r.PhoneNumber == "" {
		return errors.New("phone_number is required")
	}

	if len(r.PhoneNumber) < 8 {
		return errors.New("phone_number is too short")
	}

	// 🆕 Validation spécifique à Yenga Pay (plus souple)
	if r.Provider == entity.ProviderYengaPay {
		// Metadata est optionnel, mais si présent, valider le flow
		if r.Metadata != nil {
			// Vérifier le flux (indirect par défaut)
			flow, _ := r.Metadata["flow"].(string)
			if flow == "" {
				flow = "indirect"
			}

			validFlows := map[string]bool{
				"indirect": true,
				"direct":   true,
			}
			if !validFlows[flow] {
				return errors.New("invalid flow: " + flow + " (must be 'indirect' or 'direct')")
			}

			// Pour le flux DIRECT uniquement, l'opérateur est obligatoire
			if flow == "direct" {
				operator, ok := r.Metadata["operator"].(string)
				if !ok || operator == "" {
					return errors.New("metadata.operator is required for direct flow (orange_money, moov_money, telecel, coris_money, sank_money, mtn)")
				}

				validOperators := map[string]bool{
					"orange_money": true,
					"moov_money":   true,
					"telecel":      true,
					"coris_money":  true,
					"sank_money":   true,
					"mtn":          true,
				}
				if !validOperators[operator] {
					return errors.New("invalid operator for Yenga Pay: " + operator)
				}
			}
			// Pour le flux INDIRECT, l'opérateur est OPTIONNEL
			// (choisi par le client sur la page checkout)
		}
	}

	return nil
}

// InitiatePaymentResponse représente la réponse d'initiation
type InitiatePaymentResponse struct {
	PaymentID   string                 `json:"payment_id"`
	ProviderRef string                 `json:"provider_ref"`
	Status      entity.PaymentStatus   `json:"status"`
	USSDCode    string                 `json:"ussd_code,omitempty"`
	RedirectURL string                 `json:"redirect_url,omitempty"`
	ExpiresAt   int64                  `json:"expires_at"`
	Message     string                 `json:"message"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PaymentResponse représente la réponse détaillée d'un paiement
type PaymentResponse struct {
	ID            string                 `json:"id"`
	OrderID       string                 `json:"order_id"`
	Provider      entity.PaymentProvider `json:"provider"`
	ProviderRef   string                 `json:"provider_ref,omitempty"`
	AmountCents   int64                  `json:"amount_cents"`
	Currency      entity.Currency        `json:"currency"`
	Status        entity.PaymentStatus   `json:"status"`
	CustomerPhone string                 `json:"customer_phone,omitempty"`
	Description   string                 `json:"description,omitempty"`
	InitiatedAt   string                 `json:"initiated_at,omitempty"`
	CompletedAt   string                 `json:"completed_at,omitempty"`
	CreatedAt     string                 `json:"created_at"`
}

// RefundPaymentRequest représente la requête de remboursement
type RefundPaymentRequest struct {
	PaymentID   string `json:"payment_id"`
	AmountCents int64  `json:"amount_cents,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// Validate valide la requête de remboursement
func (r *RefundPaymentRequest) Validate() error {
	if r.PaymentID == "" {
		return errors.New("payment_id is required")
	}
	if r.AmountCents < 0 {
		return errors.New("amount_cents must be positive")
	}
	return nil
}

// WebhookRequest représente un webhook reçu (pour debug)
type WebhookRequest struct {
	Provider  string `json:"provider"`
	Signature string `json:"signature"`
	Payload   string `json:"payload"`
}

// CompletePaymentRequest représente la requête pour compléter un paiement TWO_STEP
type CompletePaymentRequest struct {
	PaymentID string `json:"payment_id"`
	OTP       string `json:"otp"`
}

// Validate valide la requête
func (r *CompletePaymentRequest) Validate() error {
	if r.PaymentID == "" {
		return errors.New("payment_id is required")
	}
	if r.OTP == "" {
		return errors.New("otp is required")
	}
	if len(r.OTP) < 4 || len(r.OTP) > 10 {
		return errors.New("otp must be between 4 and 10 characters")
	}
	return nil
}
