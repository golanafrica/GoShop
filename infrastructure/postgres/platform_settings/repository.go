package platformsettings

import (
	"context"
	"database/sql"
	"encoding/json"

	"Goshop/domain/repository"
)

type PlatformSettingsRepository struct {
	db *sql.DB
}

func NewPlatformSettingsRepository(db *sql.DB) repository.PlatformSettingsRepository {
	return &PlatformSettingsRepository{db: db}
}

func (r *PlatformSettingsRepository) Get(ctx context.Context, key string) (json.RawMessage, error) {
	var value json.RawMessage
	err := r.db.QueryRowContext(ctx, "SELECT value FROM platform_settings WHERE key = $1", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

func (r *PlatformSettingsRepository) Set(ctx context.Context, key string, value json.RawMessage, updatedBy *string) error {
	query := `
		INSERT INTO platform_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_by = $3, updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, query, key, value, updatedBy)
	return err
}

func (r *PlatformSettingsRepository) GetAll(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT key, value FROM platform_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]json.RawMessage)
	for rows.Next() {
		var key string
		var value json.RawMessage
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}

	// 🛡️ CORRECTION : Vérification de l'erreur après la boucle Next()
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
