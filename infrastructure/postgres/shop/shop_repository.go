package shop

import (
	"context"
	"database/sql"
	"encoding/json"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

//go:generate mockgen -destination=../../../mocks/repository/mock_shop_repository.go -package=repository . ShopRepository

type ShopRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewShopRepositoryInfrastructure(db *sql.DB) repository.ShopRepository {
	return &ShopRepositoryInfrastructure{db: db}
}

func (r *ShopRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ShopRepository {
	return &ShopRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *ShopRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *ShopRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *ShopRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *ShopRepositoryInfrastructure) Create(ctx context.Context, shop *entity.Shop) error {
	query := `
		INSERT INTO shops (id, name, slug, custom_domain, owner_id, logo_url, theme, plan, db_schema, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at
	`

	themeJSON, err := json.Marshal(shop.Theme)
	if err != nil {
		return err
	}

	err = r.queryRowContext(ctx, query,
		shop.ID,
		shop.Name,
		shop.Slug,
		shop.CustomDomain,
		shop.OwnerID,
		shop.LogoURL,
		themeJSON,
		shop.Plan,
		shop.DBSchema,
		shop.IsActive,
		shop.CreatedAt,
		shop.UpdatedAt,
	).Scan(&shop.CreatedAt, &shop.UpdatedAt)

	return err
}

func (r *ShopRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan, 
		       db_schema, is_active, created_at, updated_at
		FROM shops
		WHERE id = $1
	`
	return r.scanShop(r.queryRowContext(ctx, query, id))
}

func (r *ShopRepositoryInfrastructure) FindBySlug(ctx context.Context, slug string) (*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active, created_at, updated_at
		FROM shops
		WHERE slug = $1
	`
	return r.scanShop(r.queryRowContext(ctx, query, slug))
}

func (r *ShopRepositoryInfrastructure) FindByCustomDomain(ctx context.Context, domain string) (*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active, created_at, updated_at
		FROM shops
		WHERE custom_domain = $1
	`
	return r.scanShop(r.queryRowContext(ctx, query, domain))
}

func (r *ShopRepositoryInfrastructure) FindByOwnerID(ctx context.Context, ownerID string) ([]*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active, created_at, updated_at
		FROM shops
		WHERE owner_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFromRows(rows)
		if err != nil {
			return nil, err
		}
		shops = append(shops, shop)
	}

	return shops, rows.Err()
}

func (r *ShopRepositoryInfrastructure) Update(ctx context.Context, shop *entity.Shop) error {
	query := `
		UPDATE shops
		SET name = $2, slug = $3, custom_domain = $4, logo_url = $5,
		    theme = $6, plan = $7, db_schema = $8, is_active = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	themeJSON, err := json.Marshal(shop.Theme)
	if err != nil {
		return err
	}

	err = r.queryRowContext(ctx, query,
		shop.ID,
		shop.Name,
		shop.Slug,
		shop.CustomDomain,
		shop.LogoURL,
		themeJSON,
		shop.Plan,
		shop.DBSchema,
		shop.IsActive,
	).Scan(&shop.UpdatedAt)

	return err
}

func (r *ShopRepositoryInfrastructure) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE shops SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.execContext(ctx, query, id)
	return err
}

// scanShop scanne une ligne depuis sql.Row
func (r *ShopRepositoryInfrastructure) scanShop(row *sql.Row) (*entity.Shop, error) {
	shop := &entity.Shop{}
	var themeJSON []byte

	err := row.Scan(
		&shop.ID,
		&shop.Name,
		&shop.Slug,
		&shop.CustomDomain,
		&shop.OwnerID,
		&shop.LogoURL,
		&themeJSON,
		&shop.Plan,
		&shop.DBSchema,
		&shop.IsActive,
		&shop.CreatedAt,
		&shop.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(themeJSON) > 0 {
		if err := json.Unmarshal(themeJSON, &shop.Theme); err != nil {
			return nil, err
		}
	} else {
		shop.Theme = make(map[string]interface{})
	}

	return shop, nil
}

// scanShopFromRows scanne une ligne depuis sql.Rows
func (r *ShopRepositoryInfrastructure) scanShopFromRows(rows *sql.Rows) (*entity.Shop, error) {
	shop := &entity.Shop{}
	var themeJSON []byte

	err := rows.Scan(
		&shop.ID,
		&shop.Name,
		&shop.Slug,
		&shop.CustomDomain,
		&shop.OwnerID,
		&shop.LogoURL,
		&themeJSON,
		&shop.Plan,
		&shop.DBSchema,
		&shop.IsActive,
		&shop.CreatedAt,
		&shop.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if len(themeJSON) > 0 {
		if err := json.Unmarshal(themeJSON, &shop.Theme); err != nil {
			return nil, err
		}
	} else {
		shop.Theme = make(map[string]interface{})
	}

	return shop, nil
}
