package apikey

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ============================================================
// 🆕 v4.4.3 : API KEY REPOSITORY - POSTGRES IMPLEMENTATION
// ============================================================

// APIKeyRepositoryImpl implémente repository.APIKeyRepository
type APIKeyRepositoryImpl struct {
	db *sql.DB
}

// NewAPIKeyRepository crée une nouvelle instance du repository
func NewAPIKeyRepository(db *sql.DB) *APIKeyRepositoryImpl {
	return &APIKeyRepositoryImpl{
		db: db,
	}
}

// ============================================================
// HELPERS
// ============================================================

// scopesToStrings convertit []APIKeyScope en []string
func scopesToStrings(scopes []entity.APIKeyScope) []string {
	result := make([]string, len(scopes))
	for i, s := range scopes {
		result[i] = string(s)
	}
	return result
}

// stringsToScopes convertit []string en []APIKeyScope
func stringsToScopes(strs []string) []entity.APIKeyScope {
	result := make([]entity.APIKeyScope, len(strs))
	for i, s := range strs {
		result[i] = entity.APIKeyScope(s)
	}
	return result
}

// ============================================================
// MÉTHODES CRUD
// ============================================================

// Create crée une nouvelle clé API
func (r *APIKeyRepositoryImpl) Create(ctx context.Context, apiKey *entity.APIKey) error {
	if apiKey == nil {
		return repository.ErrAPIKeyInvalidData
	}

	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO api_keys (
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			created_at, updated_at,
			created_ip, created_user_agent
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		id,
		apiKey.UserID,
		apiKey.Name,
		apiKey.Description,
		apiKey.KeyPrefix,
		apiKey.KeyHash,
		pq.Array(scopesToStrings(apiKey.Scopes)),
		apiKey.ExpiresAt,
		apiKey.IsActive,
		apiKey.RateLimitPerMinute,
		apiKey.RateLimitPerDay,
		now,
		now,
		apiKey.CreatedIP,
		apiKey.CreatedUserAgent,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return repository.ErrAPIKeyAlreadyExists
		}
		return fmt.Errorf("create api_key: %w", err)
	}

	apiKey.ID = id
	apiKey.CreatedAt = now
	apiKey.UpdatedAt = now

	return nil
}

// FindByID récupère une clé API par son ID
func (r *APIKeyRepositoryImpl) FindByID(ctx context.Context, id string) (*entity.APIKey, error) {
	if id == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE id = $1
	`

	return r.scanAPIKey(r.db.QueryRowContext(ctx, query, id))
}

// FindByHash récupère une clé API par son hash (authentification)
func (r *APIKeyRepositoryImpl) FindByHash(ctx context.Context, keyHash string) (*entity.APIKey, error) {
	if keyHash == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE key_hash = $1
	`

	return r.scanAPIKey(r.db.QueryRowContext(ctx, query, keyHash))
}

// Update met à jour une clé API
func (r *APIKeyRepositoryImpl) Update(ctx context.Context, apiKey *entity.APIKey) error {
	if apiKey == nil {
		return repository.ErrAPIKeyInvalidData
	}

	query := `
		UPDATE api_keys SET
			name = $1,
			description = $2,
			scopes = $3,
			rate_limit_per_minute = $4,
			rate_limit_per_day = $5,
			updated_at = $6
		WHERE id = $7
	`

	result, err := r.db.ExecContext(ctx, query,
		apiKey.Name,
		apiKey.Description,
		pq.Array(scopesToStrings(apiKey.Scopes)),
		apiKey.RateLimitPerMinute,
		apiKey.RateLimitPerDay,
		time.Now(),
		apiKey.ID,
	)

	if err != nil {
		return fmt.Errorf("update api_key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrAPIKeyNotFound
	}

	return nil
}

// Delete supprime complètement une clé API
func (r *APIKeyRepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return repository.ErrAPIKeyInvalidData
	}

	query := `DELETE FROM api_keys WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete api_key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrAPIKeyNotFound
	}

	return nil
}

// Exists vérifie si une clé API existe
func (r *APIKeyRepositoryImpl) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, repository.ErrAPIKeyInvalidData
	}

	query := `SELECT EXISTS(SELECT 1 FROM api_keys WHERE id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check api_key exists: %w", err)
	}

	return exists, nil
}

// ============================================================
// MÉTHODES SPÉCIFIQUES API KEYS
// ============================================================

// FindByUserID récupère toutes les clés API d'un utilisateur
func (r *APIKeyRepositoryImpl) FindByUserID(ctx context.Context, userID string) ([]*entity.APIKey, error) {
	if userID == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	return r.scanAPIKeys(r.db.QueryContext(ctx, query, userID))
}

// FindActiveByUserID récupère uniquement les clés actives d'un user
func (r *APIKeyRepositoryImpl) FindActiveByUserID(ctx context.Context, userID string) ([]*entity.APIKey, error) {
	if userID == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE user_id = $1 
		AND is_active = true 
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY last_used_at DESC NULLS LAST
	`

	return r.scanAPIKeys(r.db.QueryContext(ctx, query, userID))
}

// CountByUserID compte le nombre de clés API d'un utilisateur
func (r *APIKeyRepositoryImpl) CountByUserID(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, repository.ErrAPIKeyInvalidData
	}

	query := `SELECT COUNT(*) FROM api_keys WHERE user_id = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count api_keys: %w", err)
	}

	return count, nil
}

// CountActiveByUserID compte les clés actives d'un utilisateur
func (r *APIKeyRepositoryImpl) CountActiveByUserID(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT COUNT(*) FROM api_keys 
		WHERE user_id = $1 
		AND is_active = true 
		AND (expires_at IS NULL OR expires_at > NOW())
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active api_keys: %w", err)
	}

	return count, nil
}

// ============================================================
// MÉTHODES DE SÉCURITÉ
// ============================================================

// RevokeAPIKey révoque une clé API spécifique
func (r *APIKeyRepositoryImpl) RevokeAPIKey(ctx context.Context, id string, revokedBy, reason string) error {
	if id == "" {
		return repository.ErrAPIKeyInvalidData
	}

	query := `
		UPDATE api_keys 
		SET 
			is_active = false,
			revoked_at = NOW(),
			revoked_by = $2,
			revocation_reason = $3,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, revokedBy, reason)
	if err != nil {
		return fmt.Errorf("revoke api_key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrAPIKeyNotFound
	}

	return nil
}

// RevokeAllUserAPIKeys révoque TOUTES les clés API d'un utilisateur
func (r *APIKeyRepositoryImpl) RevokeAllUserAPIKeys(ctx context.Context, userID string, excludeKeyID string, revokedBy, reason string) error {
	if userID == "" {
		return repository.ErrAPIKeyInvalidData
	}

	query := `
		UPDATE api_keys 
		SET 
			is_active = false,
			revoked_at = NOW(),
			revoked_by = $2,
			revocation_reason = $3,
			updated_at = NOW()
		WHERE user_id = $1 
		AND is_active = true
		AND id != $4
	`

	_, err := r.db.ExecContext(ctx, query, userID, revokedBy, reason, excludeKeyID)
	if err != nil {
		return fmt.Errorf("revoke all api_keys: %w", err)
	}

	return nil
}

// ValidateKey vérifie si une clé est valide
func (r *APIKeyRepositoryImpl) ValidateKey(ctx context.Context, keyHash string) (*entity.APIKey, error) {
	apiKey, err := r.FindByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Vérifier si active
	if !apiKey.IsActive {
		return nil, entity.ErrAPIKeyRevoked
	}

	// Vérifier si expirée
	if apiKey.IsExpired() {
		return nil, entity.ErrAPIKeyExpired
	}

	return apiKey, nil
}

// CheckScope vérifie si une clé a le scope requis
func (r *APIKeyRepositoryImpl) CheckScope(ctx context.Context, keyHash string, scope entity.APIKeyScope) error {
	apiKey, err := r.ValidateKey(ctx, keyHash)
	if err != nil {
		return err
	}

	if !apiKey.HasScope(scope) {
		return entity.ErrAPIKeyInsufficientScope
	}

	return nil
}

// MarkKeyUsed enregistre l'utilisation d'une clé
func (r *APIKeyRepositoryImpl) MarkKeyUsed(ctx context.Context, keyHash string) error {
	if keyHash == "" {
		return repository.ErrAPIKeyInvalidData
	}

	query := `
		UPDATE api_keys 
		SET last_used_at = NOW()
		WHERE key_hash = $1
	`

	result, err := r.db.ExecContext(ctx, query, keyHash)
	if err != nil {
		return fmt.Errorf("mark key used: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrAPIKeyNotFound
	}

	return nil
}

// ExtendExpiration étend la date d'expiration
func (r *APIKeyRepositoryImpl) ExtendExpiration(ctx context.Context, id string, duration time.Duration) error {
	if id == "" {
		return repository.ErrAPIKeyInvalidData
	}

	if duration <= 0 {
		duration = entity.APIKeyDefaultLifetime
	}

	newExpires := time.Now().Add(duration)

	query := `
		UPDATE api_keys 
		SET 
			expires_at = $1,
			updated_at = NOW()
		WHERE id = $2 AND is_active = true
	`

	result, err := r.db.ExecContext(ctx, query, newExpires, id)
	if err != nil {
		return fmt.Errorf("extend expiration: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrAPIKeyNotFound
	}

	return nil
}

// ============================================================
// MÉTHODES D'AUDIT
// ============================================================

// LogUsage enregistre l'utilisation d'une clé API
func (r *APIKeyRepositoryImpl) LogUsage(ctx context.Context, log *entity.APIKeyUsageLog) error {
	if log == nil {
		return repository.ErrAPIKeyInvalidData
	}

	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO api_key_usage_logs (
			id, api_key_id, method, path, status_code,
			ip_address, user_agent, response_time_ms,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		id,
		log.APIKeyID,
		log.Method,
		log.Path,
		log.StatusCode,
		log.IPAddress,
		log.UserAgent,
		log.ResponseTime,
		now,
	)

	if err != nil {
		return fmt.Errorf("log usage: %w", err)
	}

	log.ID = id
	log.CreatedAt = now

	return nil
}

// GetUsageLogs récupère les logs d'utilisation d'une clé
func (r *APIKeyRepositoryImpl) GetUsageLogs(ctx context.Context, apiKeyID string, limit int) ([]*entity.APIKeyUsageLog, error) {
	if apiKeyID == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	query := `
		SELECT 
			id, api_key_id, method, path, status_code,
			ip_address, user_agent, response_time_ms,
			created_at
		FROM api_key_usage_logs
		WHERE api_key_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, apiKeyID, limit)
	if err != nil {
		return nil, fmt.Errorf("get usage logs: %w", err)
	}
	defer rows.Close()

	var logs []*entity.APIKeyUsageLog
	for rows.Next() {
		var log entity.APIKeyUsageLog
		var userAgent sql.NullString

		if err := rows.Scan(
			&log.ID,
			&log.APIKeyID,
			&log.Method,
			&log.Path,
			&log.StatusCode,
			&log.IPAddress,
			&userAgent,
			&log.ResponseTime,
			&log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage log: %w", err)
		}

		if userAgent.Valid {
			log.UserAgent = userAgent.String
		}

		logs = append(logs, &log)
	}

	return logs, nil
}

// GetRecentUsageLogs récupère les logs récentes (dernières 24h)
func (r *APIKeyRepositoryImpl) GetRecentUsageLogs(ctx context.Context, apiKeyID string) ([]*entity.APIKeyUsageLog, error) {
	if apiKeyID == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT 
			id, api_key_id, method, path, status_code,
			ip_address, user_agent, response_time_ms,
			created_at
		FROM api_key_usage_logs
		WHERE api_key_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("get recent usage logs: %w", err)
	}
	defer rows.Close()

	var logs []*entity.APIKeyUsageLog
	for rows.Next() {
		var log entity.APIKeyUsageLog
		var userAgent sql.NullString

		if err := rows.Scan(
			&log.ID,
			&log.APIKeyID,
			&log.Method,
			&log.Path,
			&log.StatusCode,
			&log.IPAddress,
			&userAgent,
			&log.ResponseTime,
			&log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage log: %w", err)
		}

		if userAgent.Valid {
			log.UserAgent = userAgent.String
		}

		logs = append(logs, &log)
	}

	return logs, nil
}

// CountUsageToday compte le nombre d'appels aujourd'hui
func (r *APIKeyRepositoryImpl) CountUsageToday(ctx context.Context, apiKeyID string) (int, error) {
	if apiKeyID == "" {
		return 0, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT COUNT(*) FROM api_key_usage_logs 
		WHERE api_key_id = $1 AND created_at > CURRENT_DATE
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, apiKeyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count usage today: %w", err)
	}

	return count, nil
}

// CountUsageLast7Days compte les appels des 7 derniers jours
func (r *APIKeyRepositoryImpl) CountUsageLast7Days(ctx context.Context, apiKeyID string) (int, error) {
	if apiKeyID == "" {
		return 0, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT COUNT(*) FROM api_key_usage_logs 
		WHERE api_key_id = $1 AND created_at > NOW() - INTERVAL '7 days'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, apiKeyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count usage last 7 days: %w", err)
	}

	return count, nil
}

// ============================================================
// MÉTHODES DE NETTOYAGE
// ============================================================

// CleanupExpiredKeys supprime les clés expirées depuis > 30 jours
func (r *APIKeyRepositoryImpl) CleanupExpiredKeys(ctx context.Context) (int, error) {
	query := `
		DELETE FROM api_keys 
		WHERE expires_at < NOW() - INTERVAL '30 days'
		OR (is_active = false AND revoked_at < NOW() - INTERVAL '30 days')
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired keys: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// CleanupOldUsageLogs supprime les logs de plus de N jours
func (r *APIKeyRepositoryImpl) CleanupOldUsageLogs(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}

	// Utiliser la fonction SQL créée dans la migration
	query := `SELECT cleanup_old_api_key_logs($1)`

	var deletedCount int
	err := r.db.QueryRowContext(ctx, query, retentionDays).Scan(&deletedCount)
	if err != nil {
		return 0, fmt.Errorf("cleanup old usage logs: %w", err)
	}

	return deletedCount, nil
}

// CleanupAllInactive supprime toutes les clés inactives
func (r *APIKeyRepositoryImpl) CleanupAllInactive(ctx context.Context) (int, error) {
	// Supprimer les clés expirées/révoquées anciennes
	keysDeleted, err := r.CleanupExpiredKeys(ctx)
	if err != nil {
		return 0, err
	}

	// Supprimer les vieux logs
	logsDeleted, err := r.CleanupOldUsageLogs(ctx, 90)
	if err != nil {
		return keysDeleted, err
	}

	return keysDeleted + logsDeleted, nil
}

// ============================================================
// MÉTHODES DE STATISTIQUES
// ============================================================

// GetStatistics retourne les statistiques globales
func (r *APIKeyRepositoryImpl) GetStatistics(ctx context.Context) (*entity.APIKeyStatistics, error) {
	// Utiliser la vue créée dans la migration
	query := `
		SELECT 
			total_keys,
			active_keys,
			revoked_keys,
			expired_keys,
			unused_keys,
			unique_users,
			calls_today,
			calls_last_7_days,
			total_calls_all_time
		FROM v_api_key_statistics
	`

	var stats entity.APIKeyStatistics
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalKeys,
		&stats.ActiveKeys,
		&stats.RevokedKeys,
		&stats.ExpiredKeys,
		&stats.UnusedKeys,
		&stats.UniqueUsers,
		&stats.CallsToday,
		&stats.CallsLast7Days,
		&stats.TotalCallsAllTime,
	)

	if err != nil {
		return nil, fmt.Errorf("get api key statistics: %w", err)
	}

	return &stats, nil
}

// GetStatisticsByUser retourne les statistiques par utilisateur
func (r *APIKeyRepositoryImpl) GetStatisticsByUser(ctx context.Context, userID string) (*entity.APIKeyStatistics, error) {
	if userID == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT 
			COUNT(*) AS total_keys,
			COUNT(*) FILTER (WHERE ak.is_active = true AND (ak.expires_at IS NULL OR ak.expires_at > NOW())) AS active_keys,
			COUNT(*) FILTER (WHERE ak.is_active = false) AS revoked_keys,
			COUNT(*) FILTER (WHERE ak.expires_at < NOW()) AS expired_keys,
			COUNT(*) FILTER (WHERE ak.last_used_at IS NULL) AS unused_keys,
			1 AS unique_users,
			(SELECT COUNT(*) FROM api_key_usage_logs WHERE api_key_id IN (SELECT id FROM api_keys WHERE user_id = $1) AND created_at > CURRENT_DATE) AS calls_today,
			(SELECT COUNT(*) FROM api_key_usage_logs WHERE api_key_id IN (SELECT id FROM api_keys WHERE user_id = $1) AND created_at > NOW() - INTERVAL '7 days') AS calls_last_7_days,
			(SELECT COUNT(*) FROM api_key_usage_logs WHERE api_key_id IN (SELECT id FROM api_keys WHERE user_id = $1)) AS total_calls_all_time
		FROM api_keys ak
		WHERE ak.user_id = $1
	`

	var stats entity.APIKeyStatistics
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.TotalKeys,
		&stats.ActiveKeys,
		&stats.RevokedKeys,
		&stats.ExpiredKeys,
		&stats.UnusedKeys,
		&stats.UniqueUsers,
		&stats.CallsToday,
		&stats.CallsLast7Days,
		&stats.TotalCallsAllTime,
	)

	if err != nil {
		return nil, fmt.Errorf("get api key statistics by user: %w", err)
	}

	return &stats, nil
}

// ListActiveKeys liste les clés actives avec pagination
func (r *APIKeyRepositoryImpl) ListActiveKeys(ctx context.Context, limit, offset int) ([]*entity.APIKey, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `
		SELECT COUNT(*) FROM api_keys 
		WHERE is_active = true 
		AND (expires_at IS NULL OR expires_at > NOW())
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count active keys: %w", err)
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE is_active = true 
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY last_used_at DESC NULLS LAST
		LIMIT $1 OFFSET $2
	`

	keys, err := r.scanAPIKeys(r.db.QueryContext(ctx, query, limit, offset))
	if err != nil {
		return nil, 0, err
	}

	return keys, total, nil
}

// ListActiveKeysByUser liste les clés actives d'un user
func (r *APIKeyRepositoryImpl) ListActiveKeysByUser(ctx context.Context, userID string, limit, offset int) ([]*entity.APIKey, int, error) {
	if userID == "" {
		return nil, 0, repository.ErrAPIKeyInvalidData
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `
		SELECT COUNT(*) FROM api_keys 
		WHERE user_id = $1 
		AND is_active = true 
		AND (expires_at IS NULL OR expires_at > NOW())
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count active keys by user: %w", err)
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE user_id = $1 
		AND is_active = true 
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY last_used_at DESC NULLS LAST
		LIMIT $2 OFFSET $3
	`

	keys, err := r.scanAPIKeys(r.db.QueryContext(ctx, query, userID, limit, offset))
	if err != nil {
		return nil, 0, err
	}

	return keys, total, nil
}

// GetMostUsedKeys retourne les clés les plus utilisées
func (r *APIKeyRepositoryImpl) GetMostUsedKeys(ctx context.Context, limit int) ([]repository.APIKeyUsageCount, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			ak.id AS api_key_id,
			ak.key_prefix,
			ak.name,
			ak.user_id,
			u.email AS user_email,
			COUNT(l.id) AS usage_count,
			COALESCE(MAX(l.created_at), ak.created_at) AS last_used_at
		FROM api_keys ak
		JOIN users u ON ak.user_id = u.id
		LEFT JOIN api_key_usage_logs l ON ak.id = l.api_key_id
		GROUP BY ak.id, ak.key_prefix, ak.name, ak.user_id, u.email, ak.created_at
		ORDER BY usage_count DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get most used keys: %w", err)
	}
	defer rows.Close()

	var counts []repository.APIKeyUsageCount
	for rows.Next() {
		var c repository.APIKeyUsageCount
		if err := rows.Scan(
			&c.APIKeyID,
			&c.KeyPrefix,
			&c.Name,
			&c.UserID,
			&c.UserEmail,
			&c.UsageCount,
			&c.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage count: %w", err)
		}
		counts = append(counts, c)
	}

	return counts, nil
}

// ============================================================
// MÉTHODES D'ANALYSE
// ============================================================

// FindKeysByIP trouve toutes les clés utilisées depuis une IP
func (r *APIKeyRepositoryImpl) FindKeysByIP(ctx context.Context, ipAddress string) ([]*entity.APIKey, error) {
	if ipAddress == "" {
		return nil, repository.ErrAPIKeyInvalidData
	}

	query := `
		SELECT DISTINCT
			ak.id, ak.user_id, ak.name, ak.description,
			ak.key_prefix, ak.key_hash, ak.scopes,
			ak.expires_at, ak.last_used_at, ak.is_active,
			ak.rate_limit_per_minute, ak.rate_limit_per_day,
			ak.revoked_at, ak.revoked_by, ak.revocation_reason,
			ak.created_at, ak.updated_at,
			ak.created_ip, ak.created_user_agent
		FROM api_keys ak
		JOIN api_key_usage_logs l ON ak.id = l.api_key_id
		WHERE l.ip_address = $1
		ORDER BY ak.last_used_at DESC NULLS LAST
	`

	return r.scanAPIKeys(r.db.QueryContext(ctx, query, ipAddress))
}

// FindDuplicateUsers trouve les users avec plusieurs clés actives
func (r *APIKeyRepositoryImpl) FindDuplicateUsers(ctx context.Context, maxKeysPerUser int) ([]*entity.APIKey, error) {
	if maxKeysPerUser <= 0 {
		maxKeysPerUser = 3
	}

	query := `
		SELECT 
			ak.id, ak.user_id, ak.name, ak.description,
			ak.key_prefix, ak.key_hash, ak.scopes,
			ak.expires_at, ak.last_used_at, ak.is_active,
			ak.rate_limit_per_minute, ak.rate_limit_per_day,
			ak.revoked_at, ak.revoked_by, ak.revocation_reason,
			ak.created_at, ak.updated_at,
			ak.created_ip, ak.created_user_agent
		FROM api_keys ak
		INNER JOIN (
			SELECT user_id 
			FROM api_keys 
			WHERE is_active = true AND (expires_at IS NULL OR expires_at > NOW())
			GROUP BY user_id 
			HAVING COUNT(*) > $1
		) dup ON ak.user_id = dup.user_id
		WHERE ak.is_active = true 
		AND (ak.expires_at IS NULL OR ak.expires_at > NOW())
		ORDER BY ak.user_id, ak.last_used_at DESC NULLS LAST
	`

	return r.scanAPIKeys(r.db.QueryContext(ctx, query, maxKeysPerUser))
}

// GetExpiringKeys retourne les clés qui expirent bientôt
func (r *APIKeyRepositoryImpl) GetExpiringKeys(ctx context.Context, withinDays int) ([]*entity.APIKey, error) {
	if withinDays <= 0 {
		withinDays = 30
	}

	query := `
		SELECT 
			id, user_id, name, description,
			key_prefix, key_hash, scopes,
			expires_at, last_used_at, is_active,
			rate_limit_per_minute, rate_limit_per_day,
			revoked_at, revoked_by, revocation_reason,
			created_at, updated_at,
			created_ip, created_user_agent
		FROM api_keys
		WHERE is_active = true 
		AND expires_at IS NOT NULL
		AND expires_at > NOW()
		AND expires_at < NOW() + ($1 || ' days')::INTERVAL
		ORDER BY expires_at ASC
	`

	return r.scanAPIKeys(r.db.QueryContext(ctx, query, withinDays))
}

// ============================================================
// HELPERS DE SCAN
// ============================================================

// scanAPIKey scanne une seule clé API
func (r *APIKeyRepositoryImpl) scanAPIKey(row *sql.Row) (*entity.APIKey, error) {
	var apiKey entity.APIKey
	var scopes []string
	var expiresAt, lastUsedAt, revokedAt sql.NullTime
	var description, revokedBy, revocationReason, createdIP, createdUserAgent sql.NullString

	err := row.Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&description,
		&apiKey.KeyPrefix,
		&apiKey.KeyHash,
		pq.Array(&scopes),
		&expiresAt,
		&lastUsedAt,
		&apiKey.IsActive,
		&apiKey.RateLimitPerMinute,
		&apiKey.RateLimitPerDay,
		&revokedAt,
		&revokedBy,
		&revocationReason,
		&apiKey.CreatedAt,
		&apiKey.UpdatedAt,
		&createdIP,
		&createdUserAgent,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("scan api_key: %w", err)
	}

	// Convertir les scopes
	apiKey.Scopes = stringsToScopes(scopes)

	// Gérer les valeurs NULL
	if description.Valid {
		apiKey.Description = description.String
	}
	if expiresAt.Valid {
		apiKey.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		apiKey.LastUsedAt = &lastUsedAt.Time
	}
	if revokedAt.Valid {
		apiKey.RevokedAt = &revokedAt.Time
	}
	if revokedBy.Valid {
		apiKey.RevokedBy = revokedBy.String
	}
	if revocationReason.Valid {
		apiKey.RevocationReason = revocationReason.String
	}
	if createdIP.Valid {
		apiKey.CreatedIP = createdIP.String
	}
	if createdUserAgent.Valid {
		apiKey.CreatedUserAgent = createdUserAgent.String
	}

	return &apiKey, nil
}

// scanAPIKeys scanne plusieurs clés API
func (r *APIKeyRepositoryImpl) scanAPIKeys(rows *sql.Rows, err error) ([]*entity.APIKey, error) {
	if err != nil {
		return nil, fmt.Errorf("query api_keys: %w", err)
	}
	defer rows.Close()

	var keys []*entity.APIKey
	for rows.Next() {
		var apiKey entity.APIKey
		var scopes []string
		var expiresAt, lastUsedAt, revokedAt sql.NullTime
		var description, revokedBy, revocationReason, createdIP, createdUserAgent sql.NullString

		if err := rows.Scan(
			&apiKey.ID,
			&apiKey.UserID,
			&apiKey.Name,
			&description,
			&apiKey.KeyPrefix,
			&apiKey.KeyHash,
			pq.Array(&scopes),
			&expiresAt,
			&lastUsedAt,
			&apiKey.IsActive,
			&apiKey.RateLimitPerMinute,
			&apiKey.RateLimitPerDay,
			&revokedAt,
			&revokedBy,
			&revocationReason,
			&apiKey.CreatedAt,
			&apiKey.UpdatedAt,
			&createdIP,
			&createdUserAgent,
		); err != nil {
			return nil, fmt.Errorf("scan api_key row: %w", err)
		}

		apiKey.Scopes = stringsToScopes(scopes)

		if description.Valid {
			apiKey.Description = description.String
		}
		if expiresAt.Valid {
			apiKey.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			apiKey.LastUsedAt = &lastUsedAt.Time
		}
		if revokedAt.Valid {
			apiKey.RevokedAt = &revokedAt.Time
		}
		if revokedBy.Valid {
			apiKey.RevokedBy = revokedBy.String
		}
		if revocationReason.Valid {
			apiKey.RevocationReason = revocationReason.String
		}
		if createdIP.Valid {
			apiKey.CreatedIP = createdIP.String
		}
		if createdUserAgent.Valid {
			apiKey.CreatedUserAgent = createdUserAgent.String
		}

		keys = append(keys, &apiKey)
	}

	return keys, nil
}

// isUniqueViolation vérifie si l'erreur est une violation d'unicité
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "unique constraint") || contains(errMsg, "duplicate key")
}

// contains vérifie si une chaîne contient une sous-chaîne
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Assurer que json est utilisé
var _ = json.Marshal
