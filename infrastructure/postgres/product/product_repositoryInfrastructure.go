package product

import (
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"context"
	"database/sql"
	"fmt"
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

func (pr *ProductRepositoryInfrastructure) Create(ctx context.Context, product *entity.Product) error {
	query := `INSERT INTO products (name, description, price_cents, stock)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at, updated_at;`
	return pr.queryRowContext(ctx, query, product.Name, product.Description, product.PriceCents, product.Stock).Scan(&product.ID, &product.CreatedAt, &product.UpdatedAt)

}

func (pr *ProductRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.Product, error) {

	query := `SELECT id, name, description, price_cents, stock, created_at, updated_at
	FROM products WHERE id= $1;`
	product := &entity.Product{}
	err := pr.queryRowContext(ctx, query, id).Scan(&product.ID, &product.Name, &product.Description, &product.PriceCents, &product.Stock, &product.CreatedAt, &product.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("product with id %s not found", id)
	}

	return product, nil
}

// infrastructure/postgres/product/product_repository_infrastructure.go

func (pr *ProductRepositoryInfrastructure) FindAll(ctx context.Context, limit, offset int) ([]*entity.Product, error) {
	// ⚠️ AJOUTE CES LOGS ⚠️
	fmt.Printf("🚨 [DEBUG] Repository.FindAll appelé avec limit=%d, offset=%d\n", limit, offset)

	if limit <= 0 {
		limit = 50
		fmt.Printf("⚠️ [DEBUG] Repository: Limit corrigé à %d\n", limit)
	}
	if limit > 100 {
		limit = 100
		fmt.Printf("⚠️ [DEBUG] Repository: Limit limité à %d\n", limit)
	}
	if offset < 0 {
		offset = 0
		fmt.Printf("⚠️ [DEBUG] Repository: Offset corrigé à %d\n", offset)
	}

	query := `SELECT id, name, description, price_cents, stock, created_at, updated_at 
              FROM products 
              ORDER BY created_at DESC 
              LIMIT $1 OFFSET $2`

	// ⚠️ AJOUTE CES LOGS ⚠️
	fmt.Printf("📋 [DEBUG] Repository: Requête SQL: %s\n", query)
	fmt.Printf("📋 [DEBUG] Repository: Paramètres SQL: limit=%d, offset=%d\n", limit, offset)

	rows, err := pr.queryContext(ctx, query, limit, offset)
	if err != nil {
		fmt.Printf("❌ [DEBUG] Repository: Erreur queryContext: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	var products []*entity.Product
	count := 0
	for rows.Next() {
		count++
		p := &entity.Product{}
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents,
			&p.Stock, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			fmt.Printf("❌ [DEBUG] Repository: Erreur Scan ligne %d: %v\n", count, err)
			return nil, err
		}
		products = append(products, p)
	}

	fmt.Printf("✅ [DEBUG] Repository: Retourne %d produits (lues %d lignes)\n", len(products), count)
	return products, nil
}

func (pr *ProductRepositoryInfrastructure) Update(ctx context.Context, product *entity.Product) (*entity.Product, error) {
	query := `
	UPDATE products
	SET name = $1, description = $2, price_cents = $3, stock = $4, updated_at = NOW()
	WHERE id=$5
	RETURNING id, name, description, price_cents, stock, created_at, updated_at;`

	updated := &entity.Product{}
	err := pr.queryRowContext(ctx, query, product.Name, product.Description, product.PriceCents, product.Stock, product.ID).
		Scan(&updated.ID, &updated.Name, &updated.Description, &updated.PriceCents, &updated.Stock, &updated.CreatedAt, &updated.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (pr *ProductRepositoryInfrastructure) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM products WHERE id =$1`
	_, err := pr.execContext(ctx, query, id)
	return err
}
