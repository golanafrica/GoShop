package product

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

type ProductRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewProductRepositoryInfrastructure(db *sql.DB) repository.ProductRepository {
	return &ProductRepositoryInfrastructure{db: db}
}

func (pr *ProductRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ProductRepository {
	return &ProductRepositoryInfrastructure{tx: tx, db: pr.db}
}

func (pr *ProductRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if pr.tx != nil {
		return pr.tx.QueryRowContext(ctx, query, args...)
	}
	return pr.db.QueryRowContext(ctx, query, args...)
}

func (pr *ProductRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if pr.tx != nil {
		return pr.tx.QueryContext(ctx, query, args...)
	}
	return pr.db.QueryContext(ctx, query, args...)
}

func (pr *ProductRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if pr.tx != nil {
		return pr.tx.ExecContext(ctx, query, args...)
	}
	return pr.db.ExecContext(ctx, query, args...)
}

// getShopID extrait le shop_id du contexte (multi-tenant)
func (pr *ProductRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// Create crée un produit avec le shop_id du contexte
func (pr *ProductRepositoryInfrastructure) Create(ctx context.Context, product *entity.Product) error {
	shopID, err := pr.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `INSERT INTO products (shop_id, name, description, price_cents, stock)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id, created_at, updated_at;`

	return pr.queryRowContext(ctx, query, shopID, product.Name, product.Description, product.PriceCents, product.Stock).
		Scan(&product.ID, &product.CreatedAt, &product.UpdatedAt)
}

// FindByID trouve un produit par ID en respectant le tenant
func (pr *ProductRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.Product, error) {
	shopID, err := pr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, description, price_cents, stock, created_at, updated_at
	FROM products WHERE id = $1 AND shop_id = $2;`

	product := &entity.Product{}
	err = pr.queryRowContext(ctx, query, id, shopID).Scan(
		&product.ID, &product.Name, &product.Description,
		&product.PriceCents, &product.Stock,
		&product.CreatedAt, &product.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("product with id %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	return product, nil
}

// FindAll retourne les produits du shop courant
func (pr *ProductRepositoryInfrastructure) FindAll(ctx context.Context, limit, offset int) ([]*entity.Product, error) {
	shopID, err := pr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name, description, price_cents, stock, created_at, updated_at 
              FROM products 
              WHERE shop_id = $1
              ORDER BY created_at DESC 
              LIMIT $2 OFFSET $3`

	rows, err := pr.queryContext(ctx, query, shopID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*entity.Product
	for rows.Next() {
		p := &entity.Product{}
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents,
			&p.Stock, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}

	return products, rows.Err()
}

// Update met à jour un produit en respectant le tenant
func (pr *ProductRepositoryInfrastructure) Update(ctx context.Context, product *entity.Product) (*entity.Product, error) {
	shopID, err := pr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
	UPDATE products
	SET name = $1, description = $2, price_cents = $3, stock = $4, updated_at = NOW()
	WHERE id = $5 AND shop_id = $6
	RETURNING id, name, description, price_cents, stock, created_at, updated_at;`

	updated := &entity.Product{}
	err = pr.queryRowContext(ctx, query,
		product.Name, product.Description, product.PriceCents, product.Stock,
		product.ID, shopID,
	).Scan(&updated.ID, &updated.Name, &updated.Description, &updated.PriceCents,
		&updated.Stock, &updated.CreatedAt, &updated.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("product not found or access denied")
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete supprime un produit en respectant le tenant
func (pr *ProductRepositoryInfrastructure) Delete(ctx context.Context, id string) error {
	shopID, err := pr.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `DELETE FROM products WHERE id = $1 AND shop_id = $2`
	result, err := pr.execContext(ctx, query, id, shopID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("product not found or access denied")
	}
	return nil
}
