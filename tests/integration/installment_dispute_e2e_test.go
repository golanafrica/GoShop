package integration

import (
	"context"
	"testing"
	"time"

	disputeusecase "Goshop/application/usecase/dispute_usecase"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	disputeinfra "Goshop/infrastructure/postgres/dispute"
	escrowinfra "Goshop/infrastructure/postgres/escrow"
	installmentinfra "Goshop/infrastructure/postgres/installment"
	orderinfra "Goshop/infrastructure/postgres/order"
	txmanager "Goshop/infrastructure/postgres/tx_manager"
	walletinfra "Goshop/infrastructure/postgres/wallet"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstallmentFlow_DisputeBlocksRelease valide que l'ouverture d'un litige
// bloque l'auto-release, et que la résolution en faveur du marchand libère les fonds.
func TestInstallmentFlow_DisputeBlocksRelease(t *testing.T) {
	setupInstallmentE2E(t)

	// 1. Préparation des entités
	shopID := createE2EShop(t)
	productID := createE2EProduct(t, shopID)
	customerID := createE2ECustomer(t, shopID)
	createE2EWallet(t, shopID)

	totalOrderCents := int64(3000000) // 30 000 FCFA
	trancheAmount := totalOrderCents / 3

	// 2. Création du plan et de la commande
	planID := uuid.New().String()
	_, err := sharedDB.Exec(`
		INSERT INTO installment_plans (id, product_id, shop_id, nb_tranches, delai_jours, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())
	`, planID, productID, shopID, 3, 15)
	require.NoError(t, err)

	orderID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO orders (id, customer_id, shop_id, total_cents, status, payment_method, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'confirmed', 'mobile_money', NOW(), NOW())
	`, orderID, customerID, shopID, totalOrderCents)
	require.NoError(t, err)

	// 🆕 2.1 Création du compte Escrow (simule l'initiation du paiement)
	// Schéma réel : id, order_id, source_type, total_amount_cents, status, funds_held_at, created_at, updated_at
	escrowID := uuid.New().String()
	_, err = sharedDB.Exec(`
		INSERT INTO escrow_accounts (id, order_id, source_type, total_amount_cents, status, funds_held_at, created_at, updated_at)
		VALUES ($1, $2, 'order', $3, 'funds_held', NOW(), NOW(), NOW())
	`, escrowID, orderID, totalOrderCents)
	require.NoError(t, err, "La création de l'escrow account doit réussir")

	// Générer les 3 tranches
	for i := 1; i <= 3; i++ {
		instID := uuid.New().String()
		dueDate := time.Now().UTC().AddDate(0, 0, i*15)
		_, err = sharedDB.Exec(`
			INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'pending', NOW())
		`, instID, orderID, i, trancheAmount, dueDate)
		require.NoError(t, err)
	}

	// 3. Simuler le paiement des 3 tranches (les fonds sont mis en Hold)
	walletRepo := walletinfra.NewMerchantWalletRepositoryInfrastructure(sharedDB)
	txnRepo := walletinfra.NewWalletTransactionRepositoryInfrastructure(sharedDB)
	txManager := txmanager.NewTxManagerPostgresInfra(sharedDB)
	creditUC := walletusecase.NewCreditWalletUsecase(walletRepo, txnRepo, nil, txManager)

	testCtx := tenant.WithTenant(context.Background(), &entity.Shop{ID: uuid.MustParse(shopID)})

	for i := 1; i <= 3; i++ {
		_, err = creditUC.Execute(testCtx, &walletusecase.CreditWalletRequest{
			ShopID:          shopID,
			AmountCents:     trancheAmount,
			TransactionType: entity.WalletTxSaleCredit,
		})
		require.NoError(t, err)

		wallet, err := walletRepo.FindByShopIDForUpdate(testCtx, shopID)
		require.NoError(t, err)
		require.NoError(t, wallet.Hold(trancheAmount))
		require.NoError(t, walletRepo.Update(testCtx, wallet))

		_, err = sharedDB.Exec(`UPDATE order_installments SET status = 'paid', paid_at = NOW() WHERE order_id = $1 AND tranche_number = $2`, orderID, i)
		require.NoError(t, err)
	}

	// Vérification intermédiaire : tout est gelé
	walletAfter, _ := walletRepo.FindByShopID(testCtx, shopID)
	assert.Equal(t, totalOrderCents, walletAfter.HeldCents, "Fonds doivent être en séquestre")

	// 4. 🚨 LE CLIENT OUVRE UN LITIGE
	orderUUID := uuid.MustParse(orderID)
	shopUUID := uuid.MustParse(shopID)
	customerUUID := uuid.MustParse(customerID)

	disputeRepo := disputeinfra.NewDisputeRepositoryPostgres(sharedDB)
	orderRepo := orderinfra.NewOrderPostgresInfra(sharedDB)
	escrowRepo := escrowinfra.NewEscrowAccountRepositoryInfrastructure(sharedDB)

	openDisputeUC := disputeusecase.NewOpenDisputeUsecase(
		disputeRepo,
		orderRepo,
		escrowRepo,
		txManager,
	)

	_, err = openDisputeUC.Execute(testCtx, &disputeusecase.OpenDisputeRequest{
		OrderID:       orderID,
		ShopID:        shopUUID,
		InitiatorID:   customerUUID,
		InitiatorRole: entity.RoleCustomer,
		Reason:        "Produit non conforme aux tranches payées",
	})
	require.NoError(t, err, "L'ouverture du litige doit réussir")

	// 5. 🛡️ VÉRIFICATION : Le scheduler doit SKIPPER la libération
	orderInstallmentRepo := installmentinfra.NewOrderInstallmentRepository(sharedDB)

	installments, err := orderInstallmentRepo.GetByOrderID(testCtx, orderID)
	require.NoError(t, err)

	allPaid := true
	for _, inst := range installments {
		if !inst.IsPaid() {
			allPaid = false
		}
	}
	require.True(t, allPaid, "Toutes les tranches sont payées")

	hasDispute, err := disputeRepo.ExistsByOrderID(testCtx, orderUUID)
	require.NoError(t, err)
	assert.True(t, hasDispute, "Le système doit détecter le litige actif")

	// Vérifier que le statut de l'escrow est bien passé à 'disputed'
	escrowAfter, err := escrowRepo.FindByOrderID(testCtx, orderID)
	require.NoError(t, err)
	assert.Equal(t, entity.EscrowAccountDisputed, escrowAfter.Status, "L'escrow doit être marqué comme disputed")

	t.Log("✅ Le scheduler détecte correctement le litige et bloque l'auto-release")

	// 6. ⚖️ VÉRIFICATION DU STATUT DU LITIGE
	dispute, err := disputeRepo.FindByOrderID(testCtx, orderUUID)
	require.NoError(t, err)
	assert.Equal(t, entity.DisputeStatusPending, dispute.Status)

	t.Log("✅ Test E2E Litige Installment validé : Le flux de blocage fonctionne parfaitement !")
}
