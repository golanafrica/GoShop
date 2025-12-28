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

var db *sql.DB
var ctx = context.Background()

// âœ… Initialisation de la base de test
func setupTestDB() *sql.DB {
	if err := godotenv.Load("../../../.env"); err != nil {
		log.Println("fichier non trouver")
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
		log.Fatalf("erreur de connexion la base de test : %v", err)
	}

	log.Println("âœ… Connexion PostgreSQL rÃ©ussie")
	return db
}

// ðŸ”§ Setup global avant tous les tests
func TestMain(m *testing.M) {
	db = setupTestDB()
	defer db.Close()
	os.Exit(m.Run())
}

// ðŸš€ Test d'intÃ©gration complet du usecase CreateOrderUsecase
func TestCreateOrderUsecase_Integration(t *testing.T) {
	// --- Initialisation des repositories ---
	productRepo := product.NewProductRepositoryInfrastructure(db)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(db)
	orderRepo := order.NewOrderPostgresInfra(db)
	orderItemRepo := order.NewOrderItemPostgresInfra(db)
	txManager := txmanager.NewTxManagerPostgresInfra(db)

	// --- Initialisation du usecase ---
	usecase := orderusecase.NewCreateOrderUsecase(

		txManager, // ✓ au lieu de db
		productRepo,
		customerRepo,
		orderItemRepo,
		orderRepo,
	)

	// --- Ã‰tape 1 : CrÃ©er un client ---
	// --- Ã‰tape 1 : CrÃ©er un customer ---
	customerEntity := &entity.Customer{
		FirstName: "Integration",
		LastName:  "Test",
		Email:     fmt.Sprintf("integration_%d@test.com", time.Now().UnixNano()),
	}
	createdCustomer, err := customerRepo.Create(ctx, customerEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCustomer.ID)

	// --- Ã‰tape 2 : CrÃ©er un produit ---
	productEntity := &entity.Product{
		Name:        "Chaise en bois",
		Description: "FabriquÃ©e artisanalement",
		PriceCents:  15000,
		Stock:       5,
	}

	err = productRepo.Create(ctx, productEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, productEntity.ID)

	// --- Ã‰tape 3 : CrÃ©er une commande ---
	orderEntity := &entity.Order{
		CustomerID: createdCustomer.ID,
		Items: []*entity.OrderItem{
			{
				ProductID: productEntity.ID,
				Quantity:  2,
			},
		},
	}

	createdOrder, err := usecase.Execute(ctx, orderEntity)
	assert.NoError(t, err, "la commande doit Ãªtre crÃ©Ã©e sans erreur")
	assert.NotEmpty(t, createdOrder.ID, "un ID de commande doit Ãªtre gÃ©nÃ©rÃ©")
	assert.Equal(t, "PENDING", createdOrder.Status)
	assert.Equal(t, int64(30000), createdOrder.TotalCents)

	// --- Ã‰tape 4 : VÃ©rifier le stock mis Ã  jour ---
	updatedProduct, err := productRepo.FindByID(ctx, productEntity.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, updatedProduct.Stock, "le stock doit avoir diminuÃ© de 2 unitÃ©s")

	fmt.Printf("âœ… Commande crÃ©Ã©e avec succÃ¨s : %+v\n", createdOrder)
}
