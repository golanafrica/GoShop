package orderusecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/domain/entity"
	"Goshop/infrastructure/postgres/customer"
	"Goshop/infrastructure/postgres/order"
	"Goshop/infrastructure/postgres/product"
	txmanager "Goshop/infrastructure/postgres/tx_manager"

	"github.com/stretchr/testify/assert"
)

func TestGetAllOrderUsecase_Integration(t *testing.T) {
	// Setup - utilisez la fonction existante setupTestDB_Get()
	// qui est déjà définie dans get_order_by_id_integration_test.go
	db := setupTestDB_Get()
	defer db.Close()
	ctx := context.Background()

	// --- Initialisation des repositories ---
	productRepo := product.NewProductRepositoryInfrastructure(db)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(db)
	orderRepo := order.NewOrderPostgresInfra(db)
	orderItemRepo := order.NewOrderItemPostgresInfra(db)
	txManager := txmanager.NewTxManagerPostgresInfra(db)

	// --- Usecases ---
	createUsecase := orderusecase.NewCreateOrderUsecase(
		// 1. db
		txManager,     // 2. txManager
		productRepo,   // 3. productRepo
		customerRepo,  // 4. customerRepo
		orderItemRepo, // 5. orderItemRepo
		orderRepo,     // 6. orderRepo

	)

	getAllUsecase := orderusecase.NewGetAllOrderUsecase(
		orderRepo, // 1. repo
		txManager, // 2. txManager

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
