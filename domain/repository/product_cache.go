// domain/repository/product_cache.go
package repository

import (
	"Goshop/domain/entity"
	"context"
)

//go:generate mockgen -destination=../../mocks/repository/mock_product_cache.go -package=repository . ProductCache

// ProductCache définit les opérations de cache pour les produits
type ProductCache interface {
	// Get récupère un produit du cache par ID
	// Retourne le produit ou une erreur (ex: cache miss, timeout, etc.)
	Get(ctx context.Context, id string) (*entity.Product, error)

	// Set stocke un produit dans le cache avec une durée d'expiration
	Set(ctx context.Context, product *entity.Product, expirationSeconds int) error

	// Delete supprime un produit du cache
	Delete(ctx context.Context, id string) error
}
