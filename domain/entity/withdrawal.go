package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// WithdrawalStatus représente le statut d'un retrait
type WithdrawalStatus string

const (
	WithdrawalStatusPending    WithdrawalStatus = "pending"
	WithdrawalStatusProcessing WithdrawalStatus = "processing"
	WithdrawalStatusSuccess    WithdrawalStatus = "success"
	WithdrawalStatusFailed     WithdrawalStatus = "failed"
	WithdrawalStatusCancelled  WithdrawalStatus = "cancelled"
)

// WithdrawalPaymentMethod représente la méthode de paiement du retrait
type WithdrawalPaymentMethod string

const (
	WithdrawalOrangeMoney  WithdrawalPaymentMethod = "ORANGE_MONEY"
	WithdrawalMoovMoney    WithdrawalPaymentMethod = "MOOV_MONEY"
	WithdrawalTelecelMoney WithdrawalPaymentMethod = "TELECEL_MONEY"
	WithdrawalCorisMoney   WithdrawalPaymentMethod = "CORIS_MONEY"
	WithdrawalSankMoney    WithdrawalPaymentMethod = "SANK_MONEY"
	WithdrawalMTN          WithdrawalPaymentMethod = "MTN"
)

// Withdrawal représente un retrait (cash-out) vers Mobile Money
type Withdrawal struct {
	ID                    uuid.UUID               `json:"id" db:"id"`
	ShopID                uuid.UUID               `json:"shop_id" db:"shop_id"`
	Provider              string                  `json:"provider" db:"provider"`
	ProviderRef           *string                 `json:"provider_ref,omitempty" db:"provider_ref"`
	AmountCents           int64                   `json:"amount_cents" db:"amount"`
	Currency              Currency                `json:"currency" db:"currency"`
	FeesCents             int64                   `json:"fees_cents" db:"fees"`
	NetAmountCents        int64                   `json:"net_amount_cents" db:"net_amount"`
	Status                WithdrawalStatus        `json:"status" db:"status"`
	PaymentMethod         WithdrawalPaymentMethod `json:"payment_method" db:"payment_method"`
	DestinationNumber     string                  `json:"destination_number" db:"destination_number"`
	DestinationName       *string                 `json:"destination_name,omitempty" db:"destination_name"`
	DestinationEmail      *string                 `json:"destination_email,omitempty" db:"destination_email"`
	Description           *string                 `json:"description,omitempty" db:"description"`
	ErrorMessage          *string                 `json:"error_message,omitempty" db:"error_message"`
	OperatorTransactionID *string                 `json:"operator_transaction_id,omitempty" db:"operator_transaction_id"`
	ProcessedAt           *time.Time              `json:"processed_at,omitempty" db:"processed_at"`
	CreatedAt             time.Time               `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at" db:"updated_at"`
}

// NewWithdrawal crée un nouveau retrait
func NewWithdrawal(
	shopID uuid.UUID,
	amountCents int64,
	paymentMethod WithdrawalPaymentMethod,
	destinationNumber string,
) (*Withdrawal, error) {
	if amountCents <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if destinationNumber == "" {
		return nil, errors.New("destination number is required")
	}

	return &Withdrawal{
		ID:                uuid.New(),
		ShopID:            shopID,
		Provider:          "yenga_pay",
		AmountCents:       amountCents,
		Currency:          CurrencyXOF,
		Status:            WithdrawalStatusPending,
		PaymentMethod:     paymentMethod,
		DestinationNumber: destinationNumber,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}, nil
}

// MarkProcessing marque le retrait comme en cours de traitement
func (w *Withdrawal) MarkProcessing(providerRef string) error {
	if w.Status != WithdrawalStatusPending {
		return errors.New("withdrawal must be pending to mark as processing")
	}
	w.Status = WithdrawalStatusProcessing
	w.ProviderRef = &providerRef
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkSuccess marque le retrait comme réussi
func (w *Withdrawal) MarkSuccess(feesCents, netAmountCents int64, operatorTxID string) error {
	if w.Status != WithdrawalStatusPending && w.Status != WithdrawalStatusProcessing {
		return errors.New("withdrawal must be pending or processing to mark as success")
	}
	now := time.Now().UTC()
	w.Status = WithdrawalStatusSuccess
	w.FeesCents = feesCents
	w.NetAmountCents = netAmountCents
	w.ProcessedAt = &now
	if operatorTxID != "" {
		w.OperatorTransactionID = &operatorTxID
	}
	w.UpdatedAt = now
	return nil
}

// MarkFailed marque le retrait comme échoué
func (w *Withdrawal) MarkFailed(errorMsg string) error {
	if w.Status != WithdrawalStatusPending && w.Status != WithdrawalStatusProcessing {
		return errors.New("withdrawal must be pending or processing to mark as failed")
	}
	now := time.Now().UTC()
	w.Status = WithdrawalStatusFailed
	w.ErrorMessage = &errorMsg
	w.ProcessedAt = &now
	w.UpdatedAt = now
	return nil
}
