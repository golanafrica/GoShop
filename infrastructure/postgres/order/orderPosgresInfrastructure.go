package order

import (
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type OrderPostgresInfra struct {
	db *sql.DB
	tx repository.Tx
}

func NewOrderPostgresInfra(db *sql.DB) *OrderPostgresInfra {
	return &OrderPostgresInfra{
		db: db,
	}
}

func (or *OrderPostgresInfra) WithTX(tx repository.Tx) repository.OrderRepository {
	return &OrderPostgresInfra{db: or.db, tx: tx}
}

func (or *OrderPostgresInfra) CountByCustomerID(ctx context.Context, customerID string) (int, error) {
	query := `SELECT COUNT(*) FROM orders WHERE customer_id = $1`
	var count int
	err := or.queryRowContext(ctx, query, customerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders for customer %s: %w", customerID, err)
	}
	return count, nil
}

func (or *OrderPostgresInfra) CountAll(ctx context.Context, filter orderdto.OrderFilter) (int, error) {
	baseQuery := `SELECT COUNT(DISTINCT o.id) FROM orders o`
	conditions := []string{}
	args := []interface{}{}
	argPos := 1

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("o.status = $%d", argPos))
		args = append(args, *filter.Status)
		argPos++
	}

	if filter.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("o.customer_id = $%d", argPos))
		args = append(args, *filter.CustomerID)
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := baseQuery + whereClause

	var total int
	err := or.queryRowContext(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	return total, nil
}

func (or *OrderPostgresInfra) FindAllWithPagination(ctx context.Context, limit, offset int, filter orderdto.OrderFilter) ([]*entity.Order, error) {
	// Base query
	baseQuery := `
		SELECT 
			o.id AS order_id,
			o.customer_id,
			o.total_cents,
			o.status,
			o.created_at,
			o.updated_at,
			oi.id AS item_id,
			oi.product_id,
			oi.quantity,
			oi.price_cents,
			oi.subtotal_cents
		FROM orders o
		LEFT JOIN order_items oi ON o.id = oi.order_id
	`

	// Conditions dynamiques
	conditions := []string{}
	args := []interface{}{}
	argPos := 1 // position des $1, $2, etc.

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("o.status = $%d", argPos))
		args = append(args, *filter.Status)
		argPos++
	}

	if filter.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("o.customer_id = $%d", argPos))
		args = append(args, *filter.CustomerID)
		argPos++
	}

	// Construire la clause WHERE
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Ajouter ORDER + LIMIT/OFFSET
	query := baseQuery + whereClause + " ORDER BY o.created_at DESC LIMIT $" + strconv.Itoa(argPos) + " OFFSET $" + strconv.Itoa(argPos+1)
	args = append(args, limit, offset)

	// Exécuter
	rows, err := or.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch paginated orders: %w", err)
	}
	defer rows.Close()

	// Mapper les résultats (inchangé)
	orderMap := make(map[string]*entity.Order)
	for rows.Next() {
		var (
			orderID       string
			customerID    string
			totalCents    int64
			status        string
			createdAt     time.Time
			updatedAt     time.Time
			itemID        sql.NullString
			productID     sql.NullString
			quantity      sql.NullInt64
			priceCents    sql.NullInt64
			subTotalCents sql.NullInt64
		)

		err := rows.Scan(
			&orderID,
			&customerID,
			&totalCents,
			&status,
			&createdAt,
			&updatedAt,
			&itemID,
			&productID,
			&quantity,
			&priceCents,
			&subTotalCents,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order row: %w", err)
		}

		order, exists := orderMap[orderID]
		if !exists {
			order = &entity.Order{
				ID:         orderID,
				CustomerID: customerID,
				TotalCents: totalCents,
				Status:     status,
				CreatedAt:  createdAt,
				UpdatedAt:  updatedAt,
				Items:      []*entity.OrderItem{},
			}
			orderMap[orderID] = order
		}

		if itemID.Valid {
			item := &entity.OrderItem{
				ID:             itemID.String,
				OrderID:        orderID,
				ProductID:      productID.String,
				Quantity:       int(quantity.Int64),
				PriceCents:     priceCents.Int64,
				SubTotal_Cents: subTotalCents.Int64,
			}
			order.Items = append(order.Items, item)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	orders := make([]*entity.Order, 0, len(orderMap))
	for _, order := range orderMap {
		orders = append(orders, order)
	}

	return orders, nil
}

func (or *OrderPostgresInfra) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if or.tx != nil {
		return or.tx.QueryRowContext(ctx, query, args...)
	}
	return or.db.QueryRowContext(ctx, query, args...)
}

func (or *OrderPostgresInfra) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if or.tx != nil {
		return or.tx.QueryContext(ctx, query, args...)
	}
	return or.db.QueryContext(ctx, query, args...)

}

func (or *OrderPostgresInfra) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {

	if or.tx != nil {
		return or.tx.ExecContext(ctx, query, args...)
	}
	return or.db.ExecContext(ctx, query, args...)

}

func (or *OrderPostgresInfra) Create(ctx context.Context, order *entity.Order) (*entity.Order, error) {
	query := `INSERT INTO orders (customer_id, total_cents, status, created_at, updated_at) VALUES( $1,$2,$3,NOW(),NOW())
	RETURNING id, customer_id, total_cents, status,created_at, updated_at  `

	err := or.queryRowContext(ctx, query,
		order.CustomerID,
		order.TotalCents,
		order.Status,
	).Scan(
		&order.ID,
		&order.CustomerID,
		&order.TotalCents,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("faille to create order %w", err)
	}

	return order, nil
}

func (or *OrderPostgresInfra) FindByID(ctx context.Context, id string) (*entity.Order, error) {

	query := `SELECT id, customer_id, total_cents, status, created_at, 
	updated_at
	FROM orders 
	WHERE id = $1`

	order := &entity.Order{}
	err := or.queryRowContext(ctx, query, id).Scan(
		&order.ID,
		&order.CustomerID,
		&order.TotalCents,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order %s not found", id)
		}
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}

	queryItem := `SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents FROM order_items
	WHERE order_id = $1`

	rows, err := or.queryContext(ctx, queryItem, id)
	if err != nil {
		return nil, fmt.Errorf("falled order item %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		item := &entity.OrderItem{}
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductID,
			&item.Quantity,
			&item.PriceCents,
			&item.SubTotal_Cents,
		)
		if err != nil {
			return nil, fmt.Errorf("failled scan order item %w", err)
		}
		order.Items = append(order.Items, item)

	}

	return order, nil
}

func (or *OrderPostgresInfra) FindAll(ctx context.Context) ([]*entity.Order, error) {

	query := `
		SELECT 
			o.id AS order_id,
			o.customer_id,
			o.total_cents,
			o.status,
			o.created_at,
			o.updated_at,
			oi.id AS item_id,
			oi.product_id,
			oi.quantity,
			oi.price_cents,
			oi.subtotal_cents
		FROM orders o
		LEFT JOIN order_items oi ON o.id = oi.order_id
		ORDER BY o.created_at DESC
	`

	rows, err := or.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}
	defer rows.Close()

	orderMap := make(map[string]*entity.Order)

	for rows.Next() {

		var (
			orderID       string
			customerID    string
			totalCents    int64
			status        string
			createdAt     time.Time
			updatedAt     time.Time
			itemID        sql.NullString
			productID     sql.NullString
			quantity      sql.NullInt64
			priceCents    sql.NullInt64
			subTotalCents sql.NullInt64
		)

		err := rows.Scan(
			&orderID,
			&customerID,
			&totalCents,
			&status,
			&createdAt,
			&updatedAt,
			&itemID,
			&productID,
			&quantity,
			&priceCents,
			&subTotalCents,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		// Récupérer l'ordre existant ou le créer
		order, exists := orderMap[orderID]
		if !exists {
			order = &entity.Order{
				ID:         orderID,
				CustomerID: customerID,
				TotalCents: totalCents,
				Status:     status,
				CreatedAt:  createdAt,
				UpdatedAt:  updatedAt,
				Items:      []*entity.OrderItem{},
			}
			orderMap[orderID] = order
		}

		// Ajouter l'item si valable
		if itemID.Valid {
			item := &entity.OrderItem{
				ID:             itemID.String,
				OrderID:        orderID,
				ProductID:      productID.String,
				Quantity:       int(quantity.Int64),
				PriceCents:     priceCents.Int64,
				SubTotal_Cents: subTotalCents.Int64,
			}
			order.Items = append(order.Items, item)
		}
	}

	// Convertir map → slice
	orders := make([]*entity.Order, 0, len(orderMap))
	for _, ord := range orderMap {
		orders = append(orders, ord)
	}

	return orders, nil
}
