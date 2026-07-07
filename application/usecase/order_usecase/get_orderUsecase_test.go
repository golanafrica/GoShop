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

// testShopAll est le shop utilise pour ce test
var testShopAll = &entity.Shop{
	ID:       uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"),
	Name:     "Demo Shop",
	Slug:     "demo", // ? Meme slug que le shop existant
	IsActive: true,
}

func TestGetAllOrderUsecase_Integration(t *testing.T) {
	db := setupTestDB()
	defer db.Close()

	ctx := tenant.WithTenant(context.Background(), testShopAll)

	// --- Initialisation des repositories ---
	productRepo := product.NewProductRepositoryInfrastructure(db)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(db)
	orderRepo := order.NewOrderPostgresInfra(db)
	orderItemRepo := order.NewOrderItemPostgresInfra(db)
	txManager := txmanager.NewTxManagerPostgresInfra(db)

	// --- Usecases ---
	createUsecase := orderusecase.NewCreateOrderUsecase(txManager, productRepo, customerRepo, orderItemRepo, orderRepo, nil)

	getAllUsecase := orderusecase.NewGetAllOrderUsecase(
		orderRepo,
		txManager,
	)

	// --- 1. Cr�er un customer ---
	customerEntity := &entity.Customer{
		FirstName: "Integration2",
		LastName:  "Test2",
		Email:     fmt.Sprintf("integration2_%d@test.com", time.Now().UnixNano()),
	}

	createdCustomer, err := customerRepo.Create(ctx, customerEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCustomer.ID)

	// --- 2. Cr�er un produit ---
	productEntity := &entity.Product{
		Name:        "Table artisanale",
		Description: "Fabriqu�e � la main",
		PriceCents:  20000,
		Stock:       10,
	}

	err = productRepo.Create(ctx, productEntity)
	assert.NoError(t, err)
	assert.NotEmpty(t, productEntity.ID)

	// --- 3. Cr�er 2 commandes pour tester le listing ---
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

	// --- 4. R�cup�rer toutes les commandes ---
	allOrders, err := getAllUsecase.Execute(ctx)
	assert.NoError(t, err)
	assert.True(t, len(allOrders) >= 2, "on doit avoir au moins 2 commandes")

	// ? FIX : Filtrer uniquement les commandes cr��es par CE test
	// (celles qui appartiennent au customer du test)
	testOrders := make([]*entity.Order, 0)
	for _, o := range allOrders {
		if o.CustomerID == createdCustomer.ID {
			testOrders = append(testOrders, o)
		}
	}

	assert.Equal(t, 2, len(testOrders),
		"on doit avoir exactement 2 commandes pour ce customer")

	// --- V�rification basique sur les commandes du test uniquement ---
	for _, o := range testOrders {
		assert.NotEmpty(t, o.ID)
		assert.Equal(t, createdCustomer.ID, o.CustomerID,
			"la commande doit appartenir au customer du test")
		assert.Equal(t, "pending", o.Status,
			"la commande doit �tre en PENDING (pas encore pay�e)")
		assert.Equal(t, int64(20000), o.TotalCents,
			"le total doit �tre 20000")
		assert.True(t, len(o.Items) > 0)
	}

	fmt.Printf("? GetAllOrderUsecase fonctionne\n")
	fmt.Printf("   - Total commandes dans le shop : %d\n", len(allOrders))
	fmt.Printf("   - Commandes cr��es par ce test : %d\n", len(testOrders))
}
