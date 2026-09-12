package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	installment_dto "Goshop/application/dto/installment_dto"
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

func TestInstallmentZone_DeliveryDelayCalculation(t *testing.T) {
	setupInstallmentE2E(t)

	shopID := createE2EShop(t)
	customerID := createE2ECustomer(t, shopID)
	createE2EWallet(t, shopID)

	totalOrderCents := int64(3000000)
	trancheAmount := totalOrderCents / 3

	// 🆕 Codes de zone courts (moins de 20 caractères pour respecter VARCHAR(20))
	zoneUrbanID := uuid.New().String()
	zoneUrbanCode := fmt.Sprintf("TU-%s", uuid.New().String()[:8])
	_, err := sharedDB.Exec(`
		INSERT INTO delivery_zones (id, zone_code, zone_name, country, zone_type, installment_release_delay_days, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Zone Urbaine Test', 'Burkina Faso', 'urban', 5, true, NOW(), NOW())
	`, zoneUrbanID, zoneUrbanCode)
	require.NoError(t, err)

	zoneRuralID := uuid.New().String()
	zoneRuralCode := fmt.Sprintf("TR-%s", uuid.New().String()[:8])
	_, err = sharedDB.Exec(`
		INSERT INTO delivery_zones (id, zone_code, zone_name, country, zone_type, installment_release_delay_days, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Zone Rurale Test', 'Burkina Faso', 'rural', 10, true, NOW(), NOW())
	`, zoneRuralID, zoneRuralCode)
	require.NoError(t, err)

	now := time.Now().UTC()

	orderUrbanID := uuid.New().String()
	deliveredAtUrban := now.AddDate(0, 0, -6)
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, delivery_zone_id, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'delivered', 'mobile_money', $5, $6, NOW(), NOW())
	`, orderUrbanID, customerID, shopID, totalOrderCents, zoneUrbanID, deliveredAtUrban)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		instID := uuid.New().String()
		_, err = sharedDB.Exec(`
			INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, paid_at, created_at)
			VALUES ($1, $2, $3, $4, NOW(), 'paid', NOW(), NOW())
		`, instID, orderUrbanID, i, trancheAmount)
		require.NoError(t, err)
	}

	orderRuralID := uuid.New().String()
	deliveredAtRural := now.AddDate(0, 0, -6)
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, delivery_zone_id, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'delivered', 'mobile_money', $5, $6, NOW(), NOW())
	`, orderRuralID, customerID, shopID, totalOrderCents, zoneRuralID, deliveredAtRural)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		instID := uuid.New().String()
		_, err = sharedDB.Exec(`
			INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, paid_at, created_at)
			VALUES ($1, $2, $3, $4, NOW(), 'paid', NOW(), NOW())
		`, instID, orderRuralID, i, trancheAmount)
		require.NoError(t, err)
	}

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{ID: uuid.MustParse(shopID)})

	orderRepo := orderinfra.NewOrderPostgresInfra(sharedDB)
	installmentRepo := installmentinfra.NewOrderInstallmentRepository(sharedDB)
	deliveryZoneRepo := deliveryzoneinfra.NewDeliveryZoneRepository(sharedDB)

	dashboardUC := installmentusecase.NewGetMerchantDashboardUsecase(orderRepo, installmentRepo, deliveryZoneRepo)

	_, summaries, err := dashboardUC.Execute(testCtx, shopID)
	require.NoError(t, err)
	require.Len(t, summaries, 2, "Doit retourner 2 commandes")

	var urbanSummary, ruralSummary *installment_dto.InstallmentOrderSummary

	for _, summary := range summaries {
		switch summary.OrderID {
		case orderUrbanID:
			urbanSummary = summary
		case orderRuralID:
			ruralSummary = summary
		}
	}

	require.NotNil(t, urbanSummary, "La commande urbaine doit être trouvée")
	require.NotNil(t, ruralSummary, "La commande rurale doit être trouvée")

	assert.Equal(t, 5, urbanSummary.ReleaseDelayDays, "La zone urbaine doit avoir un délai de 5 jours")
	assert.Equal(t, 10, ruralSummary.ReleaseDelayDays, "La zone rurale doit avoir un délai de 10 jours")

	assert.True(t, urbanSummary.ExpectedReleaseDate.Before(now) || urbanSummary.ExpectedReleaseDate.Equal(now),
		"La commande urbaine (livrée il y a 6j + 5j de délai) devrait être prête à être libérée")

	assert.True(t, ruralSummary.ExpectedReleaseDate.After(now),
		"La commande rurale (livrée il y a 6j + 10j de délai) ne devrait PAS encore être prête à être libérée")

	t.Log("✅ Test E2E Zones validé : Les délais dynamiques (5j vs 10j) sont correctement appliqués !")
}
