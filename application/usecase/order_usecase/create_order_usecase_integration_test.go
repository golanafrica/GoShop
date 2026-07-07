package orderusecase_test

import (
	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
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

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

var db *sql.DB

// testShop est le shop utilisé pour tous les tests d'intégration
var testShop = &entity.Shop{
	ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
	Name:     "Test Shop",
	Slug:     "test-shop",
	IsActive: true,
}

// ctx est le contexte global avec le tenant pour tous les tests
var ctx = tenant.WithTenant(context.Background(), testShop)

// ✅ Initialisation de la base de test
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

	log.Println("✅ Connexion PostgreSQL réussie")
	return db
}

// 🔧 Setup global avant tous les tests
func TestMain(m *testing.M) {
	db = setupTestDB()

	// S'assurer que le shop de test existe en base
	ensureTestShopExists()

	defer db.Close()
	os.Exit(m.Run())
}

// ensureTestShopExists crée le shop de test s'il n'existe pas
func ensureTestShopExists() {
	// Créer un utilisateur de test
	_, _ = db.Exec(`
		INSERT INTO users (id, email, password, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000001', 'test-integration@golanafrica.com', 'dummy', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`)

	// Créer le shop de test
	_, err := db.Exec(`
		INSERT INTO shops (id, name, slug, owner_id, plan, is_active)
		VALUES ($1, $2, $3, '00000000-0000-0000-0000-000000000001', 'free', true)
		ON CONFLICT (id) DO NOTHING
	`, testShop.ID, testShop.Name, testShop.Slug)
	if err != nil {
		log.Printf("⚠️  Warning creating test shop: %v", err)
	}

	// Créer les settings du shop
	_, _ = db.Exec(`
		INSERT INTO shop_payment_settings (shop_id)
		VALUES ($1)
		ON CONFLICT (shop_id) DO NOTHING
	`, testShop.ID)
}

// 🚀 Test d'intégration complet du usecase CreateOrderUsecase
func TestCreateOrderUsecase_Integration(t *testing.T) {
	// --- Initialisation des repositories ---
	productRepo := product.NewProductRepositoryInfrastructure(db)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(db)
	orderRepo := order.NewOrderPostgresInfra(db)
	orderItemRepo := order.NewOrderItemPostgresInfra(db)
	txManager := txmanager.NewTxManagerPostgresInfra(db)

	// --- Initialisation du usecase ---
	usecase := orderusecase.NewCreateOrderUsecase(txManager, productRepo, customerRepo, orderItemRepo, orderRepo, nil)

	// --- Étape 1 : Créer un customer ---
	customerEntity := &entity.Customer{
		FirstName: "Integration",
		LastName:  "Test",
		Email:     fmt.Sprintf("integration_%d@test.com", time.Now().UnixNano()),
	}
	createdCustomer, err := customerRepo.Create(ctx, customerEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCustomer.ID)

	// --- Étape 2 : Créer un produit ---
	productEntity := &entity.Product{
		Name:        "Chaise en bois",
		Description: "Fabriquée artisanalement",
		PriceCents:  15000,
		Stock:       5,
	}

	err = productRepo.Create(ctx, productEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, productEntity.ID)

	// --- Étape 3 : Créer une commande ---
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
	assert.NoError(t, err, "la commande doit être créée sans erreur")
	assert.NotEmpty(t, createdOrder.ID, "un ID de commande doit être généré")
	assert.Equal(t, "pending", createdOrder.Status)
	assert.Equal(t, int64(30000), createdOrder.TotalCents)

	// --- Étape 4 : Vérifier le stock mis à jour ---
	updatedProduct, err := productRepo.FindByID(ctx, productEntity.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, updatedProduct.Stock, "le stock doit avoir diminué de 2 unités")

	fmt.Printf("✅ Commande créée avec succès : %+v\n", createdOrder)
}
