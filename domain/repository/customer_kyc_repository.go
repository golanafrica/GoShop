package repository

import (
	"context"

	"Goshop/domain/entity"
)

//go:generate mockgen -destination=../../mocks/repository/mock_customer_kyc_repository.go -package=repository . CustomerKYCRepository

// CustomerKYCRepository définit les opérations sur les documents KYC des clients
type CustomerKYCRepository interface {
	// Create crée un nouveau document KYC
	Create(ctx context.Context, doc *entity.CustomerKYCDocument) error

	// FindByID trouve un document KYC par son ID
	FindByID(ctx context.Context, id string) (*entity.CustomerKYCDocument, error)

	// FindByCustomerID retourne tous les documents KYC d'un client
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CustomerKYCDocument, error)

	// FindPendingByCustomer retourne les documents KYC en attente de validation pour un client
	FindPendingByCustomer(ctx context.Context, customerID string) ([]*entity.CustomerKYCDocument, error)

	// FindPendingByShop retourne tous les documents KYC en attente pour une boutique
	// Utilisé par le marchand pour voir sa liste de validations en attente
	FindPendingByShop(ctx context.Context, shopID string) ([]*entity.CustomerKYCDocument, error)

	// CountByCustomer compte le nombre de documents KYC d'un client
	// Utilisé pour limiter à 3 documents max par client
	CountByCustomer(ctx context.Context, customerID string) (int, error)

	// Update met à jour un document KYC (ex: après approbation/rejet)
	Update(ctx context.Context, doc *entity.CustomerKYCDocument) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) CustomerKYCRepository
}
