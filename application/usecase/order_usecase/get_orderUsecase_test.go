package orderusecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/postgres/customer"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/product"
	txmanager "Goshop/infrastructure/postgres/tx_manager"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// testShopAll est le shop utilisé pour ce test
var testShopAll = &entity.Shop{
	ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
	Name:     "Test Shop",
	Slug:     "test-shop",
	IsActive: true,
}

func TestGetAllOrderUsecase_Integration(t *testing.T) {
	// Setup
	db := setupTestDB_Get()
	defer db.Close()

	// ✅ Contexte avec tenant (au lieu de context.Background())
	ctx := tenant.WithTenant(context.Background(), testShopAll)

	// --- Initialisation des repositories ---
	productRepo := product.NewProductRepositoryInfrastructure(db)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(db)
	orderRepo := order.NewOrderPostgresInfra(db)
	orderItemRepo := order.NewOrderItemPostgresInfra(db)
	txManager := txmanager.NewTxManagerPostgresInfra(db)

	// --- Usecases ---
	createUsecase := orderusecase.NewCreateOrderUsecase(
		txManager,
		productRepo,
		customerRepo,
		orderItemRepo,
		orderRepo,
	)

	getAllUsecase := orderusecase.NewGetAllOrderUsecase(
		orderRepo,
		txManager,
	)

	// --- 1. Créer un customer ---
	customerEntity := &entity.Customer{
		FirstName: "Integration2",
		LastName:  "Test2",
		Email:     fmt.Sprintf("integration2_%d@test.com", time.Now().UnixNano()),
	}

	createdCustomer, err := customerRepo.Create(ctx, customerEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCustomer.ID)

	// --- 2. Créer un produit ---
	productEntity := &entity.Product{
		Name:        "Table artisanale",
		Description: "Fabriquée à la main",
		PriceCents:  20000,
		Stock:       10,
	}

	err = productRepo.Create(ctx, productEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, productEntity.ID)

	// --- 3. Créer 2 commandes pour tester le listing ---
	for i := 0; i < 2; i++ {
		orderEntity := &entity.Order{
			CustomerID: createdCustomer.ID,
			Items: []*entity.OrderItem{
				{ProductID: productEntity.ID, Quantity: 1},
			},
		}

		_, err := createUsecase.Execute(ctx, orderEntity)
		assert.NoError(t, err)
	}

	// --- 4. Récupérer toutes les commandes ---
	orders, err := getAllUsecase.Execute(ctx)
	assert.NoError(t, err)
	assert.True(t, len(orders) >= 2, "on doit avoir au moins 2 commandes")

	// --- Vérification basique ---
	for _, o := range orders {
		assert.NotEmpty(t, o.ID)
		assert.NotEmpty(t, o.CustomerID)
		assert.Equal(t, "PENDING", o.Status)
		assert.True(t, o.TotalCents > 0)
		assert.True(t, len(o.Items) > 0)
	}

	fmt.Println("✅ GetAllOrderUsecase fonctionne, commandes trouvées :", len(orders))
}
