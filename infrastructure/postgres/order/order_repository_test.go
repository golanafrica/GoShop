package order_test

import (
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/postgres/order"
	"context"
	"regexp"
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
		CustomerID: "1234",
		TotalCents: 50000,
		Status:     "PENDING",
	}

	expectedQuery := `
		INSERT INTO orders (
			shop_id, customer_id, total_cents, status, 
			payment_method, reserved_until,
			created_at, updated_at
		) 
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, shop_id, customer_id, total_cents, status, payment_method, reserved_until, created_at, updated_at
	`

	rows := sqlmock.NewRows([]string{
		"id", "shop_id", "customer_id", "total_cents", "status", "payment_method", "reserved_until", "created_at", "updated_at",
	}).AddRow("order-1", testShop.ID.String(), "1234", 50000, "PENDING", "mobile_money", nil, time.Now(), time.Now())

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs(testShop.ID.String(), orderEntity.CustomerID, orderEntity.TotalCents, orderEntity.Status, "mobile_money", nil).
		WillReturnRows(rows)

	result, err := repo.Create(contextWithTenant(), orderEntity)

	assert.NoError(t, err)
	assert.Equal(t, "order-1", result.ID)
	assert.Equal(t, testShop.ID.String(), result.ShopID)
	assert.Equal(t, int64(50000), result.TotalCents)
	assert.Equal(t, "PENDING", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_FindByID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	repo := order.NewOrderPostgresInfra(db)

	// 1️⃣ Requête principale : orders (avec shop_id)
	// Il faut fournir TOUTES les colonnes que le code réel essaie de scanner (15 colonnes)
	orderRows := sqlmock.NewRows([]string{
		"id", "shop_id", "customer_id", "total_cents", "status", "created_at", "updated_at",
		"payment_method", "accepted_at", "rejected_at", "delivered_at", "cancelled_at",
		"delivery_notes", "amount_received_cents", "reserved_until",
	}).AddRow("order-1", testShop.ID.String(), "cust-123", 100000, "PENDING", time.Now(), time.Now(),
		"mobile_money", nil, nil, nil, nil, "", int64(0), nil)

	expectedQuery := `
		SELECT id, shop_id, customer_id, total_cents, status, created_at, updated_at,
		       payment_method, accepted_at, rejected_at, delivered_at, cancelled_at,
		       delivery_notes, amount_received_cents, reserved_until
		FROM orders 
		WHERE id = $1 AND shop_id = $2
	`

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs("order-1", testShop.ID.String()).
		WillReturnRows(orderRows)

	// 2️⃣ Requête secondaire : order_items
	itemRows := sqlmock.NewRows([]string{
		"id", "order_id", "product_id", "quantity", "price_cents", "subtotal_cents",
	}).AddRow(
		"item-1", "order-1", "prod-99", int64(2), int64(50000), int64(100000),
	)

	expectedItemQuery := `SELECT id, order_id, product_id, quantity, price_cents, subtotal_cents 
		FROM order_items
		WHERE order_id = $1`

	mock.ExpectQuery(regexp.QuoteMeta(expectedItemQuery)).
		WithArgs("order-1").
		WillReturnRows(itemRows)

	// 3️⃣ Exécution
	result, err := repo.FindByID(contextWithTenant(), "order-1")

	// 4️⃣ Assertions
	assert.NoError(t, err)
	assert.Equal(t, testShop.ID.String(), result.ShopID)
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
		"order_id", "shop_id", "customer_id", "total_cents", "status", "created_at", "updated_at",
		"payment_method", "accepted_at", "rejected_at", "delivered_at", "cancelled_at",
		"delivery_notes", "amount_received_cents", "reserved_until", "delivery_zone_id",
		"item_id", "product_id", "quantity", "price_cents", "subtotal_cents",
	}).
		AddRow("order-1", testShop.ID.String(), "cust-1", 200000, "PENDING", date1, date1,
			"mobile_money", nil, nil, nil, nil, "", int64(0), nil, nil,
			"item-1", "prod-1", 1, 100000, 100000).
		AddRow("order-1", testShop.ID.String(), "cust-1", 200000, "PENDING", date1, date1,
			"mobile_money", nil, nil, nil, nil, "", int64(0), nil, nil,
			"item-2", "prod-2", 1, 100000, 100000).
		AddRow("order-2", testShop.ID.String(), "cust-2", 50000, "PENDING", date2, date2,
			"mobile_money", nil, nil, nil, nil, "", int64(0), nil, nil,
			nil, nil, nil, nil, nil)

	expectedQuery := `
		SELECT 
			o.id AS order_id,
			o.shop_id,
			o.customer_id,
			o.total_cents,
			o.status,
			o.created_at,
			o.updated_at,
			o.payment_method,
			o.accepted_at,
			o.rejected_at,
			o.delivered_at,
			o.cancelled_at,
			o.delivery_notes,
			o.amount_received_cents,
			o.reserved_until,
			o.delivery_zone_id,
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

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs(testShop.ID.String()).
		WillReturnRows(rows)

	results, err := repo.FindAll(contextWithTenant())
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// ✅ CORRECTION PR-G : Chercher par ID au lieu d'indices fixes
	// car l'itération sur un map en Go est aléatoire.
	// Utilisation d'un tagged switch pour satisfaire staticcheck QF1003.
	var order1, order2 *entity.Order
	for _, o := range results {
		switch o.ID {
		case "order-1":
			order1 = o
		case "order-2":
			order2 = o
		}
	}

	assert.NotNil(t, order1, "order-1 should exist")
	assert.NotNil(t, order2, "order-2 should exist")
	assert.Len(t, order1.Items, 2)
	assert.Len(t, order2.Items, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}
