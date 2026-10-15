package integration

import (
	"context"
	"testing"

	tontineusecase "Goshop/application/usecase/tontine_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/postgres/customer"
	customerreliabilityscoreinfra "Goshop/infrastructure/postgres/customer_reliability_score"
	"Goshop/infrastructure/postgres/product"
	"Goshop/infrastructure/postgres/tontine"
	txmanagerinfra "Goshop/infrastructure/postgres/tx_manager"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTontineScoring_BlockCommercialCreation(t *testing.T) {
	shopID := createTestShop(t)
	productID := createTestProduct(t, shopID)
	customerID := createTestCustomer(t, shopID)

	_, err := sharedDB.Exec(`
		INSERT INTO product_tontine_settings (product_id, shop_id, is_tontine_enabled, allow_commercial_circle, allow_corporate_circle, allow_family_circle, min_participants, max_participants)
		VALUES ($1, $2, true, true, true, true, 2, 10)
		ON CONFLICT (product_id) DO UPDATE SET
			is_tontine_enabled = true, allow_commercial_circle = true, allow_corporate_circle = true, allow_family_circle = true
	`, productID, shopID)
	require.NoError(t, err)

	scoreID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO customer_reliability_scores (id, customer_id, score, tier, last_calculated_at, created_at, updated_at)
		VALUES ($1, $2, 400, 'BRONZE', NOW(), NOW(), NOW())
		ON CONFLICT (customer_id) DO UPDATE SET score = 400, tier = 'BRONZE'
	`, scoreID, customerID)
	require.NoError(t, err)

	txManager := txmanagerinfra.NewTxManagerPostgresInfra(sharedDB)
	groupRepo := tontine.NewTontineGroupRepositoryInfrastructure(sharedDB)
	participantRepo := tontine.NewTontineParticipantRepositoryInfrastructure(sharedDB)
	settingsRepo := tontine.NewProductTontineSettingsRepositoryInfrastructure(sharedDB)
	productRepo := product.NewProductRepositoryInfrastructure(sharedDB)
	customerRepo := customer.NewCustomerRepoInfrastructurePostgres(sharedDB)
	scoreRepo := customerreliabilityscoreinfra.NewCustomerReliabilityScoreRepository(sharedDB)

	createGroupUC := tontineusecase.NewCreateTontineGroupUsecase(
		groupRepo,
		participantRepo,
		settingsRepo,
		productRepo,
		customerRepo,
		scoreRepo,
		txManager,
	)

	shopUUID := uuid.MustParse(shopID)
	shop := &entity.Shop{ID: shopUUID}
	ctx := tenant.WithTenant(context.Background(), shop)

	// Scénario 1 : Bronze + COMMERCIAL → échec
	reqCommercial := &tontineusecase.CreateGroupRequest{
		Name:              "Cercle Business Interdit",
		ProductID:         productID,
		CreatorCustomerID: customerID,
		CircleType:        entity.TontineCircleCommercial,
		TotalCycles:       5,
	}

	_, err = createGroupUC.Execute(ctx, reqCommercial)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SCORE_INSUFFICIENT")
	assert.Contains(t, err.Error(), "BRONZE")
	assert.Contains(t, err.Error(), "COMMERCIAL")
	t.Log("✅ Test 1 validé : Client Bronze bloqué pour créer un cercle COMMERCIAL")

	// Scénario 2 : Bronze + FAMILY → OK
	reqFamily := &tontineusecase.CreateGroupRequest{
		Name:              "Cercle Famille Diallo",
		ProductID:         productID,
		CreatorCustomerID: customerID,
		CircleType:        entity.TontineCircleFamily,
		TotalCycles:       3,
	}

	group, err := createGroupUC.Execute(ctx, reqFamily)
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.Equal(t, "Cercle Famille Diallo", group.Name)
	t.Log("✅ Test 2 validé : Client Bronze autorisé à créer un cercle FAMILY")

	var groupName string
	err = sharedDB.QueryRow(`SELECT name FROM tontine_groups WHERE id = $1`, group.ID).Scan(&groupName)
	require.NoError(t, err)
	assert.Equal(t, "Cercle Famille Diallo", groupName)
	t.Log("✅ Test 3 validé : Nom du groupe persisté en base")

	t.Log("🎉 Tous les tests E2E de scoring Tontine sont validés !")
}
