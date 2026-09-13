package integration

import (
	"context"
	"database/sql"
	"testing"

	installmentusecase "Goshop/application/usecase/installment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/postgres/customer"
	customerreliabilityscoreinfra "Goshop/infrastructure/postgres/customer_reliability_score"
	installmentpostgres "Goshop/infrastructure/postgres/installment"
	"Goshop/infrastructure/postgres/order"
	txmanagerinfra "Goshop/infrastructure/postgres/tx_manager"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallmentScoring_BlockExcessiveInstallments(t *testing.T) {
	setupInstallmentE2E(t)

	// 1. Créer les entités de base
	shopID := createE2EShop(t)
	customerID := createE2ECustomer(t, shopID)
	productID := createE2EProduct(t, shopID)

	// 2. Créer un plan de tranches pour le produit (permet jusqu'à 10 tranches)
	planID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO installment_plans (id, product_id, shop_id, nb_tranches, delai_jours, delivery_zone_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 10, 30, NULL, true, NOW(), NOW())
	`, planID, productID, shopID)
	require.NoError(t, err)

	// 3. Créer un score de fiabilité BRONZE pour le client (limite = 5 tranches)
	scoreID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO customer_reliability_scores (id, customer_id, score, tier, last_calculated_at, created_at, updated_at)
		VALUES ($1, $2, 400, 'BRONZE', NOW(), NOW(), NOW())
	`, scoreID, customerID)
	require.NoError(t, err)

	// 4. Initialiser les repositories et usecases
	txManager := txmanagerinfra.NewTxManagerPostgresInfra(sharedDB)
	orderRepo := createE2EOrderRepository(sharedDB)
	planRepo := installmentpostgres.NewInstallmentPlanRepository(sharedDB)
	customerRepo := createE2ECustomerRepository(sharedDB)
	scoreRepo := customerreliabilityscoreinfra.NewCustomerReliabilityScoreRepository(sharedDB)
	installmentRepo := installmentpostgres.NewOrderInstallmentRepository(sharedDB)

	createOrderUC := installmentusecase.NewCreateInstallmentOrderUsecase(
		txManager,
		orderRepo,
		planRepo,
		customerRepo,
		scoreRepo,
		installmentRepo,
	)

	// ✅ CORRECTION : Injecter le tenant dans le contexte pour satisfaire le OrderRepository
	shopUUID := uuid.MustParse(shopID)
	shop := &entity.Shop{ID: shopUUID}
	ctx := tenant.WithTenant(context.Background(), shop)

	// 5. Scénario 1 : Client Bronze (400) avec plan de 10 tranches → DOIT ÊTRE BLOQUÉ
	items := []*entity.OrderItem{
		{
			ID:        uuid.New().String(),
			ProductID: productID,
			Quantity:  1,
		},
	}

	_, err = createOrderUC.Execute(ctx, shopID, customerID, 1000000, items)

	// Vérifier que l'erreur de scoring est bien retournée
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accès refusé")
	assert.Contains(t, err.Error(), "BRONZE")
	assert.Contains(t, err.Error(), "5 tranches maximum")
	t.Log("✅ Test 1 validé : Client Bronze bloqué pour 10 tranches (limite = 5)")

	// 6. Scénario 2 : Upgrader en SILVER (650) avec plan de 10 tranches → DOIT ÊTRE BLOQUÉ
	_, err = sharedDB.Exec(`UPDATE customer_reliability_scores SET score = 650, tier = 'SILVER' WHERE id = $1`, scoreID)
	require.NoError(t, err)

	_, err = createOrderUC.Execute(ctx, shopID, customerID, 1000000, items)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accès refusé")
	assert.Contains(t, err.Error(), "SILVER")
	assert.Contains(t, err.Error(), "5 tranches maximum")
	t.Log("✅ Test 2 validé : Client Silver bloqué pour 10 tranches (limite = 5)")

	// 7. Scénario 3 : Upgrader en GOLD (850) avec plan de 10 tranches → DOIT RÉUSSIR
	_, err = sharedDB.Exec(`UPDATE customer_reliability_scores SET score = 850, tier = 'GOLD' WHERE id = $1`, scoreID)
	require.NoError(t, err)

	orderResult, err := createOrderUC.Execute(ctx, shopID, customerID, 1000000, items)
	require.NoError(t, err)
	require.NotNil(t, orderResult)
	t.Log("✅ Test 3 validé : Client Gold autorisé pour 10 tranches (illimité)")

	// 8. Vérifier que les 10 tranches ont bien été créées en base
	var installmentCount int
	err = sharedDB.QueryRow(`SELECT COUNT(*) FROM order_installments WHERE order_id = $1`, orderResult.ID).Scan(&installmentCount)
	require.NoError(t, err)
	assert.Equal(t, 10, installmentCount, "Doit avoir créé exactement 10 tranches pour le client Gold")
	t.Log("✅ Test 4 validé : 10 tranches créées en base de données pour le client Gold")

	// 9. Scénario 4 : Client Bronze avec plan de 3 tranches → DOIT RÉUSSIR
	_, err = sharedDB.Exec(`UPDATE customer_reliability_scores SET score = 400, tier = 'BRONZE' WHERE id = $1`, scoreID)
	require.NoError(t, err)

	_, err = sharedDB.Exec(`UPDATE installment_plans SET nb_tranches = 3 WHERE id = $1`, planID)
	require.NoError(t, err)

	orderResult2, err := createOrderUC.Execute(ctx, shopID, customerID, 1000000, items)
	require.NoError(t, err)
	require.NotNil(t, orderResult2)
	t.Log("✅ Test 5 validé : Client Bronze autorisé pour 3 tranches (dans la limite)")

	// 10. Vérifier que les 3 tranches ont bien été créées
	var installmentCount2 int
	err = sharedDB.QueryRow(`SELECT COUNT(*) FROM order_installments WHERE order_id = $1`, orderResult2.ID).Scan(&installmentCount2)
	require.NoError(t, err)
	assert.Equal(t, 3, installmentCount2, "Doit avoir créé exactement 3 tranches pour le client Bronze")
	t.Log("✅ Test 6 validé : 3 tranches créées en base de données pour le client Bronze")

	t.Log("🎉 Tous les tests E2E de scoring avec tranches sont validés !")
}

// Helper pour créer un repository Order
func createE2EOrderRepository(db *sql.DB) repository.OrderRepository {
	return order.NewOrderPostgresInfra(db)
}

// Helper pour créer un repository Customer
func createE2ECustomerRepository(db *sql.DB) repository.CustomerRepositoryInterface {
	return customer.NewCustomerRepoInfrastructurePostgres(db)
}
