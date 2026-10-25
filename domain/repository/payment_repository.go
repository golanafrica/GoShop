package repository

//go:generate mockgen -destination=../../mocks/repository/mock_payment_repository.go -package=repository . PaymentRepository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

// PaymentRepository définit les opérations sur les paiements
type PaymentRepository interface {
	Create(ctx context.Context, payment *entity.Payment) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)
	FindByOrderID(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error)
	FindByProviderRef(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error)
	FindByShop(ctx context.Context, shopID uuid.UUID, filters PaymentFilters) ([]*entity.Payment, error)
	Update(ctx context.Context, payment *entity.Payment) error
	FindCompletedWithoutCommission(ctx context.Context, limit int) ([]*entity.Payment, error)
	UpdateCommissionStatus(ctx context.Context, paymentID string, status string, commissionCents int64) error
	FindByProviderRefForUpdate(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error)
	FindByOrderIDUnscoped(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error)
	WithTX(tx Tx) PaymentRepository

	AddRefundAmount(ctx context.Context, paymentID uuid.UUID, refundAmountCents int64, newStatus entity.PaymentStatus) (bool, error)
	ReverseRefundAmount(ctx context.Context, paymentID uuid.UUID, refundAmountCents int64, newStatus entity.PaymentStatus) (bool, error)
	ReverseRefundAmountAndRecompute(ctx context.Context, paymentID uuid.UUID, refundAmountCents int64) (bool, error)
	FindStuckNonTerminalPayments(ctx context.Context, minAgeMinutes int, limit int) ([]*entity.Payment, error)

	// 🆕 B3 AUDIT : Gestion durable des demandes de remboursement
	CreatePaymentRefund(ctx context.Context, refund *entity.PaymentRefund) error
	UpdatePaymentRefundStatus(ctx context.Context, id uuid.UUID, status entity.PaymentRefundStatus, providerErr string) error
	FindPaymentRefundByIdempotency(ctx context.Context, shopID uuid.UUID, key string) (*entity.PaymentRefund, error)
}

// PaymentFilters représente les filtres pour la recherche de paiements
type PaymentFilters struct {
	Status   *entity.PaymentStatus
	Provider *entity.PaymentProvider
	Limit    int
	Offset   int
}
