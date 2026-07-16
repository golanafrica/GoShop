package order_test

import (
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/postgres/order"
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// testShop pour les tests
var testShop = &entity.Shop{
	ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
	Name:     "Test Shop",
	Slug:     "test-shop",
	IsActive: true,
}

// contextWithTenant retourne un contexte avec le shop de test
func contextWithTenant() context.Context {
	return tenant.WithTenant(context.Background(), testShop)
}

func TestOrderRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := order.NewOrderPostgresInfra(db)

	orderEntity := &entity.Order{
		CustomerID:    "1234",
		TotalCents:    50000,
		Status:        "PENDING",
		PaymentMethod: "mobile_money", // Ajouté pour correspondre à la logique par défaut
	}

	// ✅ Lignes retournées correspondant à la nouvelle requête
	rows := sqlmock.NewRows([]string{
		"id", "customer_id", "total_cents", "status", "payment_method", "reserved_until", "created_at", "updated_at",
	}).AddRow("order-1", "1234", 50000, "PENDING", "mobile_money", nil, time.Now(), time.Now())

	// ✅ Utilisation d'une regex robuste qui ignore les espaces et sauts de ligne
	// Cela évite les échecs dus aux différences de formatage entre le code et le test
	mock.ExpectQuery(`(?i)INSERT INTO orders\s*\(.*?\)\s*VALUES\s*\(.*?\)\s*RETURNING.*`).
		WithArgs(testShop.ID.String(), orderEntity.CustomerID, orderEntity.TotalCents, orderEntity.Status, "mobile_money", nil).
		WillReturnRows(rows)

	result, err := repo.Create(contextWithTenant(), orderEntity)

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

	// 1️⃣ Requête principale : orders (avec shop_id)
	orderRows := sqlmock.NewRows([]string{
		"id", "customer_id", "total_cents", "status", "created_at", "updated_at",
		"payment_method", "accepted_at", "rejected_at", "delivered_at", "cancelled_at",
		"delivery_notes", "amount_received_cents", "reserved_until",
	}).AddRow("order-1", "cust-123", 100000, "PENDING", time.Now(), time.Now(),
		"mobile_money", nil, nil, nil, nil, nil, int64(0), nil)

	mock.ExpectQuery(`(?i)SELECT id, customer_id, total_cents, status, created_at, updated_at,.*payment_method.*FROM orders\s+WHERE id = \$1 AND shop_id = \$2`).
		WithArgs("order-1", testShop.ID.String()).
		WillReturnRows(orderRows)

	// 2️⃣ Requête secondaire : order_items
	itemRows := sqlmock.NewRows([]string{
		"id", "order_id", "product_id", "quantity", "price_cents", "subtotal_cents",
	}).AddRow(
		"item-1", "order-1", "prod-99", int64(2), int64(50000), int64(100000),
	)

	mock.ExpectQuery(`(?i)SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents\s+FROM order_items\s+WHERE order_id = \$1`).
		WithArgs("order-1").
		WillReturnRows(itemRows)

	// 3️⃣ Exécution
	result, err := repo.FindByID(contextWithTenant(), "order-1")

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

	mock.ExpectQuery(`(?i)SELECT\s+o\.id AS order_id.*FROM orders o\s+LEFT JOIN order_items oi ON o\.id = oi\.order_id\s+WHERE o\.shop_id = \$1\s+ORDER BY o\.created_at DESC`).
		WithArgs(testShop.ID.String()).
		WillReturnRows(rows)

	results, err := repo.FindAll(contextWithTenant())
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
