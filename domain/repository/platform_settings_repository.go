package repository

import (
	"context"
	"encoding/json"
)

// PlatformSettingsRepository définit les opérations sur la configuration globale
type PlatformSettingsRepository interface {
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Set(ctx context.Context, key string, value json.RawMessage, updatedBy *string) error
	GetAll(ctx context.Context) (map[string]json.RawMessage, error)
}
