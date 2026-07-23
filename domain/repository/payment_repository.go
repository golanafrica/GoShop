package repository

//go:generate mockgen -destination=../../mocks/repository/mock_payment_repository.go -package=repository . PaymentRepository

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../mocks/repository/mock_payment_repository.go -package=repository . PaymentRepository

// PaymentRepository définit les opérations sur les paiements
type PaymentRepository interface {
	// Create crée un nouveau paiement
	Create(ctx context.Context, payment *entity.Payment) error

	// FindByID trouve un paiement par ID
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)

	// FindByOrderID trouve tous les paiements d'une commande
	FindByOrderID(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error)

	// FindByProviderRef trouve un paiement par référence provider
	FindByProviderRef(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error)

	// FindByShop finds all payments for a shop with optional filters
	FindByShop(ctx context.Context, shopID uuid.UUID, filters PaymentFilters) ([]*entity.Payment, error)

	// Update met à jour un paiement
	Update(ctx context.Context, payment *entity.Payment) error

	// Ajoute à l'interface PaymentRepository :
	FindCompletedWithoutCommission(ctx context.Context, limit int) ([]*entity.Payment, error)
	UpdateCommissionStatus(ctx context.Context, paymentID string, status string, commissionCents int64) error

	// 🛡️ CORRECTION AUDIT : FindByProviderRefForUpdate verrouille la ligne atomiquement
	// Empêche les doubles traitements de webhooks concurrents (idempotence)
	FindByProviderRefForUpdate(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) PaymentRepository
}

// PaymentFilters représente les filtres pour la recherche de paiements
type PaymentFilters struct {
	Status   *entity.PaymentStatus
	Provider *entity.PaymentProvider
	Limit    int
	Offset   int
}
