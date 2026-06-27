package order

import (
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

// getShopID extrait le shop_id du contexte (multi-tenant)
func (or *OrderPostgresInfra) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// CountByCustomerID compte les commandes d'un client dans le shop courant
func (or *OrderPostgresInfra) CountByCustomerID(ctx context.Context, customerID string) (int, error) {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM orders WHERE customer_id = $1 AND shop_id = $2`
	var count int
	err = or.queryRowContext(ctx, query, customerID, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders for customer %s: %w", customerID, err)
	}
	return count, nil
}

// CountAll compte les commandes du shop avec filtres
func (or *OrderPostgresInfra) CountAll(ctx context.Context, filter orderdto.OrderFilter) (int, error) {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	baseQuery := `SELECT COUNT(DISTINCT o.id) FROM orders o WHERE o.shop_id = $1`
	args := []interface{}{shopID}
	argPos := 2
	conditions := []string{}

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

	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	var total int
	err = or.queryRowContext(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	return total, nil
}

// FindAllWithPagination récupère les commandes du shop avec pagination
func (or *OrderPostgresInfra) FindAllWithPagination(ctx context.Context, limit, offset int, filter orderdto.OrderFilter) ([]*entity.Order, error) {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return nil, err
	}

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
		WHERE o.shop_id = $1
	`

	args := []interface{}{shopID}
	argPos := 2
	conditions := []string{}

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
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	query := baseQuery + whereClause + " ORDER BY o.created_at DESC LIMIT $" + strconv.Itoa(argPos) + " OFFSET $" + strconv.Itoa(argPos+1)
	args = append(args, limit, offset)

	rows, err := or.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch paginated orders: %w", err)
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

// Create crée une commande avec shop_id et tous les champs cash
func (or *OrderPostgresInfra) Create(ctx context.Context, order *entity.Order) (*entity.Order, error) {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	// 🆕 Payment method par défaut
	paymentMethod := order.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = string(entity.PaymentMethodMobileMoney)
	}

	query := `
		INSERT INTO orders (
			shop_id, customer_id, total_cents, status, 
			payment_method, reserved_until,
			created_at, updated_at
		) 
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, customer_id, total_cents, status, 
		          payment_method, reserved_until,
		          created_at, updated_at
	`

	err = or.queryRowContext(ctx, query,
		shopID,
		order.CustomerID,
		order.TotalCents,
		order.Status,
		paymentMethod,
		order.ReservedUntil,
	).Scan(
		&order.ID,
		&order.CustomerID,
		&order.TotalCents,
		&order.Status,
		&order.PaymentMethod,
		&order.ReservedUntil,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return order, nil
}

// FindByID trouve une commande par ID dans le shop courant (enrichi avec champs cash)
func (or *OrderPostgresInfra) FindByID(ctx context.Context, id string) (*entity.Order, error) {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, total_cents, status, created_at, updated_at,
		       payment_method, accepted_at, rejected_at, delivered_at, cancelled_at,
		       delivery_notes, amount_received_cents, reserved_until
		FROM orders 
		WHERE id = $1 AND shop_id = $2
	`

	order := &entity.Order{}
	err = or.queryRowContext(ctx, query, id, shopID).Scan(
		&order.ID,
		&order.CustomerID,
		&order.TotalCents,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.PaymentMethod,
		&order.AcceptedAt,
		&order.RejectedAt,
		&order.DeliveredAt,
		&order.CancelledAt,
		&order.DeliveryNotes,
		&order.AmountReceivedCents,
		&order.ReservedUntil,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order %s not found", id)
		}
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}

	// Récupérer les items de la commande
	queryItem := `SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents 
		FROM order_items
		WHERE order_id = $1`

	rows, err := or.queryContext(ctx, queryItem, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order items: %w", err)
	}
	defer rows.Close()

	order.Items = []*entity.OrderItem{}
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
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}

	return order, nil
}

// FindAll retourne toutes les commandes du shop courant
func (or *OrderPostgresInfra) FindAll(ctx context.Context) ([]*entity.Order, error) {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return nil, err
	}

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
		WHERE o.shop_id = $1
		ORDER BY o.created_at DESC
	`

	rows, err := or.queryContext(ctx, query, shopID)
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

	orders := make([]*entity.Order, 0, len(orderMap))
	for _, ord := range orderMap {
		orders = append(orders, ord)
	}

	return orders, nil
}

// UpdateStatus met à jour le statut d'une commande
func (r *OrderPostgresInfra) UpdateStatus(ctx context.Context, orderID uuid.UUID, status string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`
	result, err := r.execContext(ctx, query, status, orderID.String(), shopID)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order not found")
	}

	return nil
}

// UpdateOrder met à jour TOUS les champs de la commande (workflow cash)
func (or *OrderPostgresInfra) UpdateOrder(ctx context.Context, order *entity.Order) error {
	shopID, err := or.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE orders SET
			status = $1,
			payment_method = $2,
			accepted_at = $3,
			rejected_at = $4,
			delivered_at = $5,
			cancelled_at = $6,
			delivery_notes = $7,
			amount_received_cents = $8,
			reserved_until = $9,
			updated_at = NOW()
		WHERE id = $10 AND shop_id = $11
	`

	result, err := or.execContext(ctx, query,
		order.Status,
		order.PaymentMethod,
		order.AcceptedAt,
		order.RejectedAt,
		order.DeliveredAt,
		order.CancelledAt,
		order.DeliveryNotes,
		order.AmountReceivedCents,
		order.ReservedUntil,
		order.ID,
		shopID,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order %s not found", order.ID)
	}

	return nil
}

// FindCashPendingByShop retourne les commandes cash en attente de confirmation
func (or *OrderPostgresInfra) FindCashPendingByShop(ctx context.Context, shopID string) ([]*entity.Order, error) {
	query := `
		SELECT id, customer_id, total_cents, status, created_at, updated_at,
		       payment_method, accepted_at, rejected_at, delivered_at, cancelled_at,
		       delivery_notes, amount_received_cents, reserved_until
		FROM orders
		WHERE shop_id = $1 
		  AND payment_method = 'cash_on_delivery'
		  AND status = 'pending_confirmation'
		ORDER BY created_at ASC
	`

	rows, err := or.queryContext(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("find cash pending orders: %w", err)
	}
	defer rows.Close()

	var orders []*entity.Order
	for rows.Next() {
		order, err := or.scanOrderWithCashFields(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return orders, rows.Err()
}

// scanOrderWithCashFields scanne une commande avec tous les champs cash
func (or *OrderPostgresInfra) scanOrderWithCashFields(rows *sql.Rows) (*entity.Order, error) {
	order := &entity.Order{}
	err := rows.Scan(
		&order.ID,
		&order.CustomerID,
		&order.TotalCents,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.PaymentMethod,
		&order.AcceptedAt,
		&order.RejectedAt,
		&order.DeliveredAt,
		&order.CancelledAt,
		&order.DeliveryNotes,
		&order.AmountReceivedCents,
		&order.ReservedUntil,
	)
	if err != nil {
		return nil, fmt.Errorf("scan order with cash fields: %w", err)
	}
	return order, nil
}
