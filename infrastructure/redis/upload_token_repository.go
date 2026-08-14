package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"
)

// RedisUploadTokenRepository implémente UploadTokenRepository avec Redis
type RedisUploadTokenRepository struct {
	ttl time.Duration
}

// NewRedisUploadTokenRepository crée une nouvelle instance
func NewRedisUploadTokenRepository() repository.UploadTokenRepository {
	return &RedisUploadTokenRepository{
		ttl: 1 * time.Hour,
	}
}

// Create crée un nouveau token dans Redis
func (r *RedisUploadTokenRepository) Create(ctx context.Context, token *entity.UploadToken) error {
	if utils.Rdb == nil {
		return fmt.Errorf("redis not initialized")
	}

	key := r.key(token.ID)
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Stocker avec TTL
	return utils.Rdb.Set(ctx, key, data, r.ttl).Err()
}

// FindByID récupère un token par son ID
func (r *RedisUploadTokenRepository) FindByID(ctx context.Context, tokenID string) (*entity.UploadToken, error) {
	if utils.Rdb == nil {
		return nil, fmt.Errorf("redis not initialized")
	}

	key := r.key(tokenID)
	data, err := utils.Rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	var token entity.UploadToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// MarkUsed marque un token comme utilisé
func (r *RedisUploadTokenRepository) MarkUsed(ctx context.Context, tokenID string) error {
	token, err := r.FindByID(ctx, tokenID)
	if err != nil {
		return err
	}

	token.MarkUsed()

	key := r.key(tokenID)
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Calculer TTL restant
	remainingTTL := time.Until(token.ExpiresAt)
	if remainingTTL <= 0 {
		return fmt.Errorf("token expired")
	}

	return utils.Rdb.Set(ctx, key, data, remainingTTL).Err()
}

// DeleteExpired supprime les tokens expirés (Redis gère ça automatiquement avec TTL)
func (r *RedisUploadTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// Redis supprime automatiquement les clés expirées
	return 0, nil
}

// DeleteByUser supprime tous les tokens d'un utilisateur
func (r *RedisUploadTokenRepository) DeleteByUser(ctx context.Context, userID string) error {
	// Implémentation future : scanner toutes les clés et filtrer par user_id
	// Pour l'instant, on ne fait rien (les tokens expirent naturellement)
	return nil
}

// key génère la clé Redis pour un token
func (r *RedisUploadTokenRepository) key(tokenID string) string {
	return fmt.Sprintf("upload_token:%s", tokenID)
}
