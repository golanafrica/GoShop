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

	// 🆕 PR 2 : Mise à jour atomique de plusieurs paramètres dans une seule transaction
	UpdateMany(ctx context.Context, settings map[string]json.RawMessage, updatedBy *string) error
}
