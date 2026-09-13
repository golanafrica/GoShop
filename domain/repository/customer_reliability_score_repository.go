package repository

import (
	"Goshop/domain/entity"
	"context"
)

// CustomerReliabilityScoreRepository gère la persistance des scores de fiabilité
type CustomerReliabilityScoreRepository interface {
	// Avec transaction
	WithTX(tx Tx) CustomerReliabilityScoreRepository

	// CRUD de base
	Create(ctx context.Context, score *entity.CustomerReliabilityScore) error
	Update(ctx context.Context, score *entity.CustomerReliabilityScore) error
	FindByCustomerID(ctx context.Context, customerID string) (*entity.CustomerReliabilityScore, error)

	// Méthodes métier
	Upsert(ctx context.Context, score *entity.CustomerReliabilityScore) error // Create or Update

	// Statistiques
	CountByTier(ctx context.Context, tier entity.ReliabilityTier) (int, error)
	GetAverageScore(ctx context.Context) (float64, error)

	// Requêtes pour le calcul automatique
	GetCustomersWithOverdueInstallments(ctx context.Context) ([]string, error) // customer_ids
	GetCustomersWithCompletedTontineCycles(ctx context.Context, sinceDays int) ([]string, error)
	GetCustomersWithSuccessfulCODOrders(ctx context.Context, sinceDays int) ([]string, error)
}
