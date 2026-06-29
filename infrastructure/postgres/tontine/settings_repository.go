package tontine

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// ProductTontineSettingsRepositoryInfrastructure implémente repository.ProductTontineSettingsRepository
type ProductTontineSettingsRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewProductTontineSettingsRepositoryInfrastructure crée une nouvelle instance
func NewProductTontineSettingsRepositoryInfrastructure(db *sql.DB) repository.ProductTontineSettingsRepository {
	return &ProductTontineSettingsRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *ProductTontineSettingsRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ProductTontineSettingsRepository {
	return &ProductTontineSettingsRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers
// ============================================================

func (r *ProductTontineSettingsRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *ProductTontineSettingsRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *ProductTontineSettingsRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *ProductTontineSettingsRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// ============================================================
// Implémentation
// ============================================================

// FindByProductID retourne la configuration tontine d'un produit
func (r *ProductTontineSettingsRepositoryInfrastructure) FindByProductID(ctx context.Context, productID string) (*entity.ProductTontineSettings, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT product_id, shop_id, is_tontine_enabled,
		       allow_commercial_circle, allow_corporate_circle, allow_family_circle,
		       min_participants, max_participants,
		       created_at, updated_at
		FROM product_tontine_settings
		WHERE product_id = $1 AND shop_id = $2
	`

	settings := &entity.ProductTontineSettings{}
	err = r.queryRowContext(ctx, query, productID, shopID).Scan(
		&settings.ProductID,
		&settings.ShopID,
		&settings.IsTontineEnabled,
		&settings.AllowCommercialCircle,
		&settings.AllowCorporateCircle,
		&settings.AllowFamilyCircle,
		&settings.MinParticipants,
		&settings.MaxParticipants,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("product tontine settings not found")
		}
		return nil, fmt.Errorf("failed to find product tontine settings: %w", err)
	}

	return settings, nil
}

// FindByShopID retourne toutes les configurations tontine d'une boutique
func (r *ProductTontineSettingsRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.ProductTontineSettings, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if currentShopID != shopID {
		return nil, fmt.Errorf("access denied: shop_id mismatch")
	}

	query := `
		SELECT product_id, shop_id, is_tontine_enabled,
		       allow_commercial_circle, allow_corporate_circle, allow_family_circle,
		       min_participants, max_participants,
		       created_at, updated_at
		FROM product_tontine_settings
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to query product tontine settings: %w", err)
	}
	defer rows.Close()

	var settings []*entity.ProductTontineSettings
	for rows.Next() {
		s := &entity.ProductTontineSettings{}
		err := rows.Scan(
			&s.ProductID, &s.ShopID, &s.IsTontineEnabled,
			&s.AllowCommercialCircle, &s.AllowCorporateCircle, &s.AllowFamilyCircle,
			&s.MinParticipants, &s.MaxParticipants,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		settings = append(settings, s)
	}

	if settings == nil {
		settings = []*entity.ProductTontineSettings{}
	}
	return settings, rows.Err()
}

// Upsert crée ou met à jour la configuration tontine d'un produit
func (r *ProductTontineSettingsRepositoryInfrastructure) Upsert(ctx context.Context, settings *entity.ProductTontineSettings) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	if settings.ShopID != shopID {
		return fmt.Errorf("settings shop_id does not match tenant shop_id")
	}

	query := `
		INSERT INTO product_tontine_settings (
			product_id, shop_id, is_tontine_enabled,
			allow_commercial_circle, allow_corporate_circle, allow_family_circle,
			min_participants, max_participants,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (product_id) DO UPDATE SET
			is_tontine_enabled = EXCLUDED.is_tontine_enabled,
			allow_commercial_circle = EXCLUDED.allow_commercial_circle,
			allow_corporate_circle = EXCLUDED.allow_corporate_circle,
			allow_family_circle = EXCLUDED.allow_family_circle,
			min_participants = EXCLUDED.min_participants,
			max_participants = EXCLUDED.max_participants,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		settings.ProductID,
		settings.ShopID,
		settings.IsTontineEnabled,
		settings.AllowCommercialCircle,
		settings.AllowCorporateCircle,
		settings.AllowFamilyCircle,
		settings.MinParticipants,
		settings.MaxParticipants,
	).Scan(&settings.CreatedAt, &settings.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert product tontine settings: %w", err)
	}

	return nil
}
