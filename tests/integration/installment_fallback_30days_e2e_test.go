package integration

import (
	"context"
	"fmt"
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

func TestInstallmentFallback_30DaysNoDelivery(t *testing.T) {
	setupInstallmentE2E(t)

	shopID := createE2EShop(t)
	customerID := createE2ECustomer(t, shopID)
	createE2EWallet(t, shopID)

	totalOrderCents := int64(3000000)
	trancheAmount := totalOrderCents / 3

	// 🆕 Code de zone court (moins de 20 caractères)
	zoneUrbanID := uuid.New().String()
	zoneUrbanCode := fmt.Sprintf("TF-%s", uuid.New().String()[:8])
	_, err := sharedDB.Exec(`
		INSERT INTO delivery_zones (id, zone_code, zone_name, country, zone_type, installment_release_delay_days, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Zone Urbaine Fallback', 'Burkina Faso', 'urban', 5, true, NOW(), NOW())
	`, zoneUrbanID, zoneUrbanCode)
	require.NoError(t, err)

	now := time.Now().UTC()
	createdAt := now.AddDate(0, 0, -31)

	orderID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, delivery_zone_id, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'confirmed', 'mobile_money', $5, NULL, $6, NOW())
	`, orderID, customerID, shopID, totalOrderCents, zoneUrbanID, createdAt)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		instID := uuid.New().String()
		_, err = sharedDB.Exec(`
			INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, paid_at, created_at)
			VALUES ($1, $2, $3, $4, NOW(), 'paid', NOW(), NOW())
		`, instID, orderID, i, trancheAmount)
		require.NoError(t, err)
	}

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{ID: uuid.MustParse(shopID)})

	orderRepo := orderinfra.NewOrderPostgresInfra(sharedDB)
	installmentRepo := installmentinfra.NewOrderInstallmentRepository(sharedDB)
	deliveryZoneRepo := deliveryzoneinfra.NewDeliveryZoneRepository(sharedDB)

	dashboardUC := installmentusecase.NewGetMerchantDashboardUsecase(orderRepo, installmentRepo, deliveryZoneRepo)

	_, summaries, err := dashboardUC.Execute(testCtx, shopID)
	require.NoError(t, err)
	require.Len(t, summaries, 1, "Doit retourner 1 commande")

	summary := summaries[0]

	assert.Equal(t, "complete", summary.Status, "Le statut doit être 'complete' car toutes les tranches sont payées")
	assert.Equal(t, 5, summary.ReleaseDelayDays, "Le délai de la zone est bien de 5 jours")

	expectedFallbackRelease := createdAt.AddDate(0, 0, 30)

	require.NotNil(t, summary.ExpectedReleaseDate, "ExpectedReleaseDate ne doit pas être nil")

	assert.WithinDuration(t, expectedFallbackRelease, *summary.ExpectedReleaseDate, time.Minute,
		"La date de libération doit être created_at + 30 jours (fallback), car delivered_at est NULL")

	assert.True(t, summary.ExpectedReleaseDate.Before(now),
		"La date de fallback (31 jours après création) est dans le passé, le scheduler doit la libérer")

	t.Log("✅ Test E2E Fallback 30 jours validé : Le système protège les fonds en utilisant created_at + 30j quand delivered_at est NULL !")
}
