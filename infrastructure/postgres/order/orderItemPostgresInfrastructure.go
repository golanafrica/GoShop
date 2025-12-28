package order

import (
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"context"
	"database/sql"
	"fmt"
)

type OrderItemPostgresInfra struct {
	db *sql.DB
	tx repository.Tx
}

func NewOrderItemPostgresInfra(db *sql.DB) *OrderItemPostgresInfra {
	return &OrderItemPostgresInfra{db: db}
}

func (ori *OrderItemPostgresInfra) WithTX(tx repository.Tx) repository.OrderItemRepository {
	return &OrderItemPostgresInfra{db: ori.db, tx: tx}
}

func (ori *OrderItemPostgresInfra) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if ori.tx != nil {
		return ori.tx.QueryRowContext(ctx, query, args...)
	}
	return ori.db.QueryRowContext(ctx, query, args...)
}

func (ori *OrderItemPostgresInfra) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if ori.tx != nil {
		return ori.tx.QueryContext(ctx, query, args...)
	}
	return ori.db.QueryContext(ctx, query, args...)
}
func (ori *OrderItemPostgresInfra) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if ori.tx != nil {
		return ori.tx.ExecContext(ctx, query, args...)
	}
	return ori.db.ExecContext(ctx, query, args...)
}

// ✅ Create un article de commande
func (ori *OrderItemPostgresInfra) Create(ctx context.Context, orderItem *entity.OrderItem) (*entity.OrderItem, error) {
	query := `
	INSERT INTO order_items (order_id, product_id, quantity, price_cents, subtotal_cents)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id, order_id, product_id, quantity, price_cents, subtotal_cents
	`

	err := ori.queryRowContext(ctx, query,
		orderItem.OrderID,
		orderItem.ProductID,
		orderItem.Quantity,
		orderItem.PriceCents,
		orderItem.SubTotal_Cents,
	).Scan(
		&orderItem.ID,
		&orderItem.OrderID,
		&orderItem.ProductID,
		&orderItem.Quantity,
		&orderItem.PriceCents,
		&orderItem.SubTotal_Cents,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create order item: %w", err)
	}

	return orderItem, nil
}

// ✅ Trouver un article par son ID
func (ori *OrderItemPostgresInfra) FindByID(ctx context.Context, id string) (*entity.OrderItem, error) {
	query := `
	SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents
	FROM order_items
	WHERE id = $1
	`

	orderItem := &entity.OrderItem{}

	err := ori.queryRowContext(ctx, query, id).Scan(
		&orderItem.ID,
		&orderItem.OrderID,
		&orderItem.ProductID,
		&orderItem.Quantity,
		&orderItem.PriceCents,
		&orderItem.SubTotal_Cents,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order item not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find order item: %w", err)
	}

	return orderItem, nil
}

// ✅ Récupérer tous les articles
func (ori *OrderItemPostgresInfra) FindAll(ctx context.Context) ([]*entity.OrderItem, error) {
	query := `
	SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents
	FROM order_items
	ORDER BY order_id DESC
	`

	rows, err := ori.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order items: %w", err)
	}
	defer rows.Close()

	var orderItems []*entity.OrderItem

	for rows.Next() {
		orderItem := &entity.OrderItem{}
		if err := rows.Scan(
			&orderItem.ID,
			&orderItem.OrderID,
			&orderItem.ProductID,
			&orderItem.Quantity,
			&orderItem.PriceCents,
			&orderItem.SubTotal_Cents,
		); err != nil {
			return nil, err
		}
		orderItems = append(orderItems, orderItem)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading order items: %w", err)
	}

	return orderItems, nil
}
