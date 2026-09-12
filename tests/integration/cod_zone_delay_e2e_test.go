package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"Goshop/domain/entity"
	codinfra "Goshop/infrastructure/postgres/cod"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCODZoneDelay_E2E(t *testing.T) {
	setupInstallmentE2E(t)

	shopID := createE2EShop(t)
	customerID := createE2ECustomer(t, shopID)

	// 1. Créer 2 zones avec des délais de confirmation COD différents et des codes UNIQUES
	zoneUrbanID := uuid.New().String()
	zoneUrbanCode := fmt.Sprintf("TCU-%s", uuid.New().String()[:8]) // TCU = Test COD Urban
	_, err := sharedDB.Exec(`
		INSERT INTO delivery_zones (id, zone_code, zone_name, country, zone_type, cod_confirmation_delay_days, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Zone Urbaine COD', 'Burkina Faso', 'urban', 3, true, NOW(), NOW())
	`, zoneUrbanID, zoneUrbanCode)
	require.NoError(t, err)

	zoneRuralID := uuid.New().String()
	zoneRuralCode := fmt.Sprintf("TCR-%s", uuid.New().String()[:8]) // TCR = Test COD Rural
	_, err = sharedDB.Exec(`
		INSERT INTO delivery_zones (id, zone_code, zone_name, country, zone_type, cod_confirmation_delay_days, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Zone Rurale COD', 'Burkina Faso', 'rural', 10, true, NOW(), NOW())
	`, zoneRuralID, zoneRuralCode)
	require.NoError(t, err)

	now := time.Now().UTC()

	// 2. Scénario Urbain : Créer d'abord la commande (pour respecter la foreign key)
	orderUrbanID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, shop_id, customer_id, total_cents, status, payment_method, delivery_zone_id, created_at, updated_at)
		VALUES ($1, $2, $3, 5000000, 'delivered', 'cash_on_delivery', $4, NOW(), NOW())
	`, orderUrbanID, shopID, customerID, zoneUrbanID)
	require.NoError(t, err)

	// Créer la preuve COD Urbaine (il y a 4 jours, donc PRÊTE car délai = 3 jours)
	proofUrbanID := uuid.New().String()
	createdAtUrban := now.AddDate(0, 0, -4)
	_, err = sharedDB.Exec(`
		INSERT INTO cod_proofs (id, order_id, shop_id, customer_id, delivery_zone_id, status, commission_status, commission_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'confirmed', 'pending', 125000, $6, $6)
	`, proofUrbanID, orderUrbanID, shopID, customerID, zoneUrbanID, createdAtUrban)
	require.NoError(t, err)

	// 3. Scénario Rural : Créer d'abord la commande
	orderRuralID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, shop_id, customer_id, total_cents, status, payment_method, delivery_zone_id, created_at, updated_at)
		VALUES ($1, $2, $3, 5000000, 'delivered', 'cash_on_delivery', $4, NOW(), NOW())
	`, orderRuralID, shopID, customerID, zoneRuralID)
	require.NoError(t, err)

	// Créer la preuve COD Rurale (il y a 4 jours, donc PAS PRÊTE car délai = 10 jours)
	proofRuralID := uuid.New().String()
	createdAtRural := now.AddDate(0, 0, -4)
	_, err = sharedDB.Exec(`
		INSERT INTO cod_proofs (id, order_id, shop_id, customer_id, delivery_zone_id, status, commission_status, commission_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'confirmed', 'pending', 125000, $6, $6)
	`, proofRuralID, orderRuralID, shopID, customerID, zoneRuralID, createdAtRural)
	require.NoError(t, err)

	// 4. Appeler la méthode du repository
	codRepo := codinfra.NewCODProofRepositoryInfrastructure(sharedDB)

	proofs, err := codRepo.FindProofsReadyForCollection(context.Background(), 10)
	require.NoError(t, err)

	// 5. Assertions
	require.Len(t, proofs, 1, "Seule la preuve urbaine (délai 3j, créée il y a 4j) doit être prête")
	assert.Equal(t, proofUrbanID, proofs[0].ID, "La preuve récupérée doit être celle de la zone urbaine")
	assert.Equal(t, entity.CODProofConfirmed, proofs[0].Status)
	assert.Equal(t, entity.CODCommissionPending, proofs[0].CommissionStatus)

	t.Log("✅ Test E2E Délai Zone COD validé : Le scheduler respecte bien les délais dynamiques par zone !")
}
