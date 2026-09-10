package integration

import (
	"context"
	"testing"
	"time"

	installmentusecase "Goshop/application/usecase/installment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	deliveryzoneinfra "Goshop/infrastructure/postgres/delivery_zone"
	installmentinfra "Goshop/infrastructure/postgres/installment"
	orderinfra "Goshop/infrastructure/postgres/order"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallmentDashboard_CalculatesStatsCorrectly(t *testing.T) {
	setupInstallmentE2E(t)

	// 1. Préparation des entités
	shopID := createE2EShop(t)
	_ = createE2EProduct(t, shopID) // 🆕 Ignoré car non utilisé dans ce test spécifique
	customerID := createE2ECustomer(t, shopID)
	createE2EWallet(t, shopID)

	totalOrderCents := int64(3000000) // 30 000 FCFA
	trancheAmount := totalOrderCents / 3

	// 2. Créer la commande
	orderID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'confirmed', 'mobile_money', NOW(), NOW())
	`, orderID, customerID, shopID, totalOrderCents)
	require.NoError(t, err)

	// 3. Créer 3 tranches : 1 payée, 1 en retard, 1 future
	now := time.Now().UTC()

	// Tranche 1 : Payée (Due il y a 10 jours)
	inst1ID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, paid_at, created_at)
		VALUES ($1, $2, 1, $3, $4, 'paid', NOW(), NOW())
	`, inst1ID, orderID, trancheAmount, now.AddDate(0, 0, -10))
	require.NoError(t, err)

	// Tranche 2 : En retard / Overdue (Due il y a 5 jours)
	inst2ID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, created_at)
		VALUES ($1, $2, 2, $3, $4, 'pending', NOW())
	`, inst2ID, orderID, trancheAmount, now.AddDate(0, 0, -5))
	require.NoError(t, err)

	// Tranche 3 : Future (Due dans 15 jours)
	inst3ID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, created_at)
		VALUES ($1, $2, 3, $3, $4, 'pending', NOW())
	`, inst3ID, orderID, trancheAmount, now.AddDate(0, 0, 15))
	require.NoError(t, err)

	// 4. Exécuter le Usecase
	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{ID: uuid.MustParse(shopID)})

	orderRepo := orderinfra.NewOrderPostgresInfra(sharedDB)
	installmentRepo := installmentinfra.NewOrderInstallmentRepository(sharedDB)
	deliveryZoneRepo := deliveryzoneinfra.NewDeliveryZoneRepository(sharedDB)

	dashboardUC := installmentusecase.NewGetMerchantDashboardUsecase(orderRepo, installmentRepo, deliveryZoneRepo)

	dashboard, summaries, err := dashboardUC.Execute(testCtx, shopID)
	require.NoError(t, err)

	// 5. Assertions pour valider la logique métier
	assert.Equal(t, 1, dashboard.TotalPendingOrders, "Doit avoir 1 commande en cours")
	assert.Equal(t, trancheAmount*2, dashboard.TotalAmountPending, "Le montant restant doit être de 2 tranches (20 000 FCFA)")
	assert.Equal(t, trancheAmount, dashboard.TotalHeldAmount, "Le montant en séquestre doit être de 1 tranche payée (10 000 FCFA)")
	assert.Equal(t, 1, dashboard.TotalOverdueOrders, "Doit avoir 1 commande avec au moins une tranche en retard")

	require.Len(t, summaries, 1, "Doit retourner 1 résumé de commande")
	assert.Equal(t, "overdue", summaries[0].Status, "Le statut de la commande doit être 'overdue'")
	assert.Equal(t, trancheAmount, summaries[0].PaidAmount)
	assert.Equal(t, trancheAmount*2, summaries[0].RemainingAmount)
	assert.Len(t, summaries[0].Installments, 3, "Doit contenir les 3 tranches")

	t.Log("✅ Test E2E Dashboard Installment validé : Les statistiques sont correctement calculées et le cache est prêt !")
}
