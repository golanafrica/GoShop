package payment

import (
	"context"
	"errors"

	"Goshop/domain/entity"
)

// Erreurs standards des providers
var (
	ErrProviderUnavailable = errors.New("payment provider is unavailable")
	ErrInvalidPhoneNumber  = errors.New("invalid phone number")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrPaymentDeclined     = errors.New("payment declined by provider")
	ErrInvalidWebhook      = errors.New("invalid webhook signature")
	ErrDuplicatePayment    = errors.New("duplicate payment reference")
)

// PaymentRequest représente une requête d'initiation de paiement
type PaymentRequest struct {
	PaymentID   string
	AmountCents int64
	Currency    entity.Currency
	PhoneNumber string
	CustomerRef string // Référence client (ex: user ID)
	Description string
	CallbackURL string // URL de retour après paiement
	WebhookURL  string // URL pour les webhooks
	Metadata    map[string]interface{}
}

// PaymentResponse représente la réponse d'initiation
type PaymentResponse struct {
	ProviderRef string // Référence côté provider
	Status      entity.PaymentStatus
	RedirectURL string // URL pour rediriger le client (si applicable)
	USSDCode    string // Code USSD à composer (si applicable)
	ExpiresAt   int64  // Timestamp d'expiration (Unix)
	Metadata    map[string]interface{}
}

// PaymentStatus représente le statut d'un paiement côté provider
type PaymentStatus struct {
	ProviderRef   string
	Status        entity.PaymentStatus
	AmountCents   int64
	CompletedAt   int64 // Timestamp (Unix), 0 si non complété
	FailureReason string
	Metadata      map[string]interface{}
}

// WebhookEvent représente un événement webhook reçu
type WebhookEvent struct {
	Provider    entity.PaymentProvider
	EventType   string // ex: "payment.success", "payment.failed"
	ProviderRef string
	ExternalID  string // ID du webhook côté provider
	Payload     []byte // Payload brut pour audit
	Signature   string // Signature pour validation
	Status      entity.PaymentStatus
	AmountCents int64
	Metadata    map[string]interface{}
}

// Provider définit le contrat pour un fournisseur de paiement
type Provider interface {
	// Code retourne le code unique du provider
	Code() entity.PaymentProvider

	// InitiatePayment initie un paiement auprès du provider
	InitiatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error)

	// CheckStatus vérifie le statut d'un paiement auprès du provider
	CheckStatus(ctx context.Context, providerRef string) (*PaymentStatus, error)

	// ValidateWebhook valide et parse un webhook reçu
	ValidateWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)

	// Refund initie un remboursement vers le client (via Cash-Out dynamique)
	Refund(ctx context.Context, providerRef string, amountCents int64, customerPhone string, operator string) error

	// IsAvailable vérifie si le provider est disponible
	IsAvailable(ctx context.Context) bool
}

// 🆕 ProviderCompletable est l'interface optionnelle pour les providers qui supportent la complétion
// (utilisé par Yenga Pay pour le flux TWO_STEP avec OTP)
type ProviderCompletable interface {
	CompletePayment(ctx context.Context, paymentIntentID, operatorCode, customerMSISDN, otp string) (*CompletePaymentResponse, error)
}

// CompletePaymentResponse représente la réponse de complétion d'un paiement
type CompletePaymentResponse struct {
	Status        string
	TransactionID string
	Amount        int64
	Fees          int64
	TotalAmount   int64
}
