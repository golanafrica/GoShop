package repository

//go:generate mockgen -destination=../../mocks/repository/mock_idempotency_repository.go -package=repository . IdempotencyRepository

import (
	"context"

	"Goshop/domain/entity"
)

// ============================================================
// IDEMPOTENCY REPOSITORY
// ============================================================

// IdempotencyRepository définit le contrat pour la gestion des clés d'idempotence
type IdempotencyRepository interface {
	// FindByKey récupère une clé d'idempotence par sa clé unique
	// Retourne nil si non trouvée
	FindByKey(ctx context.Context, key string) (*entity.IdempotencyKey, error)

	// Create crée une nouvelle clé d'idempotence
	// Retourne une erreur si la clé existe déjà (conflit)
	Create(ctx context.Context, ik *entity.IdempotencyKey) error

	// UpdateResponse met à jour la réponse stockée pour une clé existante
	UpdateResponse(ctx context.Context, key string, status int, headers map[string]string, body map[string]interface{}) error

	// DeleteExpired supprime toutes les clés expirées
	// Retourne le nombre de clés supprimées
	DeleteExpired(ctx context.Context) (int64, error)

	// DeleteByKey supprime une clé spécifique
	DeleteByKey(ctx context.Context, key string) error

	// CountActive compte les clés d'idempotence actives (non expirées)
	CountActive(ctx context.Context) (int, error)

	// CountByUser compte les clés par utilisateur (pour audit)
	CountByUser(ctx context.Context, userID string) (int, error)

	// FindByUser retourne les clés d'un utilisateur (pour audit/admin)
	FindByUser(ctx context.Context, userID string, limit, offset int) ([]*entity.IdempotencyKey, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) IdempotencyRepository
}
