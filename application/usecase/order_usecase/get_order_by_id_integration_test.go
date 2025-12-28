package orderusecase_test

import (
	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/infrastructure/postgres"
	"Goshop/infrastructure/postgres/customer"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/product"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

// --- Initialisation DB ---
func setupTestDB_Get() *sql.DB {
	if err := godotenv.Load("../../../.env"); err != nil {
		log.Println("⚠️ .env non trouvé (mode défaut)")
	}

	connStr := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_SSLMODE"),
	)

	db, err := postgres.Connect(connStr)
	if err != nil {
		log.Fatalf("❌ Connexion DB échouée : %v", err)
	}

	log.Println("✅ Connexion PostgreSQL OK")
	return db
}

func TestGetOrderByIdUsecase_Integration(t *testing.T) {
	// Setup
	db := setupTestDB_Get()
	defer db.Close()
	ctx := context.Background()

	// --- Repos
	productRepo := product.NewProductRepositoryInfrastructure(db)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(db)
	orderRepo := order.NewOrderPostgresInfra(db)
	orderItemRepo := order.NewOrderItemPostgresInfra(db)
	txManager := txmanager.NewTxManagerPostgresInfra(db)

	// --- Usecases
	createOrderUsecase := orderusecase.NewCreateOrderUsecase(

		txManager,
		productRepo,
		customerRepo,
		orderItemRepo,
		orderRepo,
	)

	getOrderUsecase := orderusecase.NewGetOrderByIdUsecase(
		orderRepo,
		txManager,
	)

	// --------------------------
	// 1️⃣ Création Customer
	// --------------------------
	customerEntity := &entity.Customer{
		FirstName: "Client",
		LastName:  "Test",
		Email:     fmt.Sprintf("client_%d@test.com", time.Now().UnixNano()),
	}

	createdCustomer, err := customerRepo.Create(ctx, customerEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCustomer.ID)

	// --------------------------
	// 2️⃣ Création Produit
	// --------------------------
	productEntity := &entity.Product{
		Name:        "Produit A",
		Description: "Description test",
		PriceCents:  10000,
		Stock:       10,
	}

	err = productRepo.Create(ctx, productEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, productEntity.ID)

	// --------------------------
	// 3️⃣ Création Commande
	// --------------------------
	orderEntity := &entity.Order{
		CustomerID: createdCustomer.ID,
		Items: []*entity.OrderItem{
			{ProductID: productEntity.ID, Quantity: 2},
		},
	}

	createdOrder, err := createOrderUsecase.Execute(ctx, orderEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdOrder.ID)

	// --------------------------
	// 4️⃣ Test GetOrderByID
	// --------------------------
	orderFromDB, err := getOrderUsecase.Execute(ctx, createdOrder.ID)
	assert.NoError(t, err)
	assert.NotNil(t, orderFromDB)

	// --------------------------
	// 5️⃣ Vérifications
	// --------------------------
	assert.Equal(t, createdOrder.ID, orderFromDB.ID)
	assert.Equal(t, createdCustomer.ID, orderFromDB.CustomerID)
	assert.Equal(t, int64(20000), orderFromDB.TotalCents) // 2 * 10000
	assert.Equal(t, "PENDING", orderFromDB.Status)
	assert.Len(t, orderFromDB.Items, 1)

	item := orderFromDB.Items[0]
	assert.Equal(t, productEntity.ID, item.ProductID)
	assert.Equal(t, int64(10000), item.PriceCents)
	assert.Equal(t, int64(20000), item.SubTotal_Cents)

	fmt.Printf("✅ Order récupéré avec GetOrderById : %+v\n", orderFromDB)
}
