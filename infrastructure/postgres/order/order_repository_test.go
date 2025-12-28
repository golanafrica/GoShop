package order_test

import (
	"Goshop/domain/entity"
	"Goshop/infrastructure/postgres/order"
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestOrderRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := order.NewOrderPostgresInfra(db)

	orderEntity := &entity.Order{
		CustomerID: "1234",
		TotalCents: 50000,
		Status:     "PENDING",
	}

	rows := sqlmock.NewRows([]string{
		"id", "customer_id", "total_cents", "status", "created_at", "updated_at",
	}).AddRow("order-1", "1234", 50000, "PENDING", time.Now(), time.Now())

	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO orders (customer_id, total_cents, status, created_at, updated_at) VALUES( $1,$2,$3,NOW(),NOW())
RETURNING id, customer_id, total_cents, status,created_at, updated_at`,
	)).
		WithArgs(orderEntity.CustomerID, orderEntity.TotalCents, orderEntity.Status).
		WillReturnRows(rows)

	result, err := repo.Create(context.Background(), orderEntity)

	assert.NoError(t, err)
	assert.Equal(t, "order-1", result.ID)
	assert.Equal(t, int64(50000), result.TotalCents)
	assert.Equal(t, "PENDING", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_FindByID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	repo := order.NewOrderPostgresInfra(db)

	// 1️⃣ Requête principale : orders
	orderRows := sqlmock.NewRows([]string{
		"id", "customer_id", "total_cents", "status", "created_at", "updated_at",
	}).AddRow("order-1", "cust-123", 100000, "PENDING", time.Now(), time.Now())

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, customer_id, total_cents, status, created_at, 
		updated_at
		FROM orders 
		WHERE id = $1`)).
		WithArgs("order-1").
		WillReturnRows(orderRows)

	// 2️⃣ Requête secondaire : order_items
	itemRows := sqlmock.NewRows([]string{
		"id", "order_id", "product_id", "quantity", "price_cents", "subtotal_cents",
	}).AddRow(
		"item-1", "order-1", "prod-99", int64(2), int64(50000), int64(100000),
	)

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents 
		FROM order_items
		WHERE order_id = $1`)).
		WithArgs("order-1").
		WillReturnRows(itemRows)

	// 3️⃣ Exécution
	result, err := repo.FindByID(context.Background(), "order-1")

	// 4️⃣ Assertions
	assert.NoError(t, err)
	assert.Equal(t, "cust-123", result.CustomerID)
	assert.Equal(t, int64(100000), result.TotalCents)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "prod-99", result.Items[0].ProductID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_FindAll(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	repo := order.NewOrderPostgresInfra(db)

	// ✅ On crée de vraies valeurs time.Time
	date1 := time.Now()
	date2 := time.Now().Add(-24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"order_id", "customer_id", "total_cents", "status", "created_at", "updated_at",
		"item_id", "product_id", "quantity", "price_cents", "subtotal_cents",
	}).
		AddRow("order-1", "cust-1", 200000, "PENDING", date1, date1,
			"item-1", "prod-1", 1, 100000, 100000).
		AddRow("order-1", "cust-1", 200000, "PENDING", date1, date1,
			"item-2", "prod-2", 1, 100000, 100000).
		AddRow("order-2", "cust-2", 50000, "PENDING", date2, date2,
			nil, nil, nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`
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
	`)).WillReturnRows(rows)

	results, err := repo.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	order1 := results[0]
	assert.Equal(t, "order-1", order1.ID)
	assert.Len(t, order1.Items, 2)

	order2 := results[1]
	assert.Equal(t, "order-2", order2.ID)
	assert.Len(t, order2.Items, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}
