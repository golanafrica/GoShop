package entity

import (
	"time"

	"github.com/google/uuid"
)

type PaymentRefundStatus string

const (
	PaymentRefundPending                PaymentRefundStatus = "pending"
	PaymentRefundCompleted              PaymentRefundStatus = "completed"
	PaymentRefundFailed                 PaymentRefundStatus = "failed"
	PaymentRefundReconciliationRequired PaymentRefundStatus = "reconciliation_required"
)

type PaymentRefund struct {
	ID             uuid.UUID
	PaymentID      uuid.UUID
	ShopID         uuid.UUID
	AmountCents    int64
	Status         PaymentRefundStatus
	Reason         string
	IdempotencyKey string
	ProviderRef    *string
	ProviderError  *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
