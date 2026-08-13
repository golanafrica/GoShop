package idempotency

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

// ============================================================
// IDEMPOTENCY POSTGRES REPOSITORY
// ============================================================

// PostgresIdempotencyRepository implémente IdempotencyRepository avec PostgreSQL
type PostgresIdempotencyRepository struct {
	db *sql.DB
	tx repository.Tx
}

// NewPostgresIdempotencyRepository crée une nouvelle instance
func NewPostgresIdempotencyRepository(db *sql.DB) repository.IdempotencyRepository {
	return &PostgresIdempotencyRepository{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *PostgresIdempotencyRepository) WithTX(tx repository.Tx) repository.IdempotencyRepository {
	return &PostgresIdempotencyRepository{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *PostgresIdempotencyRepository) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresIdempotencyRepository) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *PostgresIdempotencyRepository) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// MÉTHODES PRINCIPALES
// ============================================================

// FindByKey récupère une clé d'idempotence par sa clé unique
func (r *PostgresIdempotencyRepository) FindByKey(ctx context.Context, key string) (*entity.IdempotencyKey, error) {
	query := `
		SELECT 
			idempotency_key, user_id, endpoint, request_hash,
			response_status, response_headers, response_body,
			created_at, expires_at
		FROM idempotency_keys
		WHERE idempotency_key = $1
	`

	var ik entity.IdempotencyKey
	var headersJSON, bodyJSON []byte

	err := r.queryRowContext(ctx, query, key).Scan(
		&ik.IdempotencyKey,
		&ik.UserID,
		&ik.Endpoint,
		&ik.RequestHash,
		&ik.ResponseStatus,
		&headersJSON,
		&bodyJSON,
		&ik.CreatedAt,
		&ik.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Non trouvée
		}
		return nil, err
	}

	// Désérialiser les JSON
	if headersJSON != nil {
		if err := json.Unmarshal(headersJSON, &ik.ResponseHeaders); err != nil {
			return nil, err
		}
	} else {
		ik.ResponseHeaders = make(map[string]string)
	}

	if bodyJSON != nil {
		if err := json.Unmarshal(bodyJSON, &ik.ResponseBody); err != nil {
			return nil, err
		}
	} else {
		ik.ResponseBody = make(map[string]interface{})
	}

	return &ik, nil
}

// Create crée une nouvelle clé d'idempotence
func (r *PostgresIdempotencyRepository) Create(ctx context.Context, ik *entity.IdempotencyKey) error {
	query := `
		INSERT INTO idempotency_keys (
			idempotency_key, user_id, endpoint, request_hash,
			response_status, response_headers, response_body,
			created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	headersJSON, err := json.Marshal(ik.ResponseHeaders)
	if err != nil {
		return err
	}

	bodyJSON, err := json.Marshal(ik.ResponseBody)
	if err != nil {
		return err
	}

	_, err = r.execContext(ctx, query,
		ik.IdempotencyKey,
		ik.UserID,
		ik.Endpoint,
		ik.RequestHash,
		ik.ResponseStatus,
		headersJSON,
		bodyJSON,
		ik.CreatedAt,
		ik.ExpiresAt,
	)

	return err
}

// UpdateResponse met à jour la réponse stockée
func (r *PostgresIdempotencyRepository) UpdateResponse(ctx context.Context, key string, status int, headers map[string]string, body map[string]interface{}) error {
	query := `
		UPDATE idempotency_keys
		SET response_status = $1, response_headers = $2, response_body = $3
		WHERE idempotency_key = $4
	`

	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return err
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	_, err = r.execContext(ctx, query, status, headersJSON, bodyJSON, key)
	return err
}

// DeleteExpired supprime toutes les clés expirées
func (r *PostgresIdempotencyRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM idempotency_keys WHERE expires_at < $1`
	result, err := r.execContext(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteByKey supprime une clé spécifique
func (r *PostgresIdempotencyRepository) DeleteByKey(ctx context.Context, key string) error {
	query := `DELETE FROM idempotency_keys WHERE idempotency_key = $1`
	_, err := r.execContext(ctx, query, key)
	return err
}

// CountActive compte les clés non expirées
func (r *PostgresIdempotencyRepository) CountActive(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM idempotency_keys WHERE expires_at > $1`
	var count int
	err := r.queryRowContext(ctx, query, time.Now()).Scan(&count)
	return count, err
}

// CountByUser compte les clés par utilisateur
func (r *PostgresIdempotencyRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM idempotency_keys WHERE user_id = $1`
	var count int
	err := r.queryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

// FindByUser retourne les clés d'un utilisateur (pour audit)
func (r *PostgresIdempotencyRepository) FindByUser(ctx context.Context, userID string, limit, offset int) ([]*entity.IdempotencyKey, error) {
	query := `
		SELECT 
			idempotency_key, user_id, endpoint, request_hash,
			response_status, response_headers, response_body,
			created_at, expires_at
		FROM idempotency_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.queryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*entity.IdempotencyKey
	for rows.Next() {
		var ik entity.IdempotencyKey
		var headersJSON, bodyJSON []byte

		err := rows.Scan(
			&ik.IdempotencyKey,
			&ik.UserID,
			&ik.Endpoint,
			&ik.RequestHash,
			&ik.ResponseStatus,
			&headersJSON,
			&bodyJSON,
			&ik.CreatedAt,
			&ik.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}

		// Désérialiser JSON
		if headersJSON != nil {
			_ = json.Unmarshal(headersJSON, &ik.ResponseHeaders)
		} else {
			ik.ResponseHeaders = make(map[string]string)
		}
		if bodyJSON != nil {
			_ = json.Unmarshal(bodyJSON, &ik.ResponseBody)
		} else {
			ik.ResponseBody = make(map[string]interface{})
		}

		keys = append(keys, &ik)
	}

	return keys, rows.Err()
}
