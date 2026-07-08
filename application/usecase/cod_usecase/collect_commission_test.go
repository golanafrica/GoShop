package codusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	codusecase "Goshop/application/usecase/cod_usecase"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.5 : TESTS UNITAIRES - COLLECT COMMISSION USECASE
// ============================================================
//
// 🎯 Stratégie (Option B) :
//   - Tests des cas où walletUC et freezeUC ne sont PAS appelés
//   - Validation de la requête
//   - Erreurs d'infrastructure (multi-tenant, transaction, repository)
//   - Validation métier (proof incomplet, incohérent, déjà collectée)
//
// ⚠️ Limitation :
//   - Les cas avec walletUC/freezeUC nécessitent d'extraire des interfaces (Option A)
//   - Test ForceCollect supprimé car nécessite walletUC non-nil
//
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createMockTx crée un mock de transaction
func createMockTx(ctrl *gomock.Controller) *mockrepo.MockTx {
	return mockrepo.NewMockTx(ctrl)
}

// createTestContextAndProof crée un contexte tenant ET une preuve synchronisée
func createTestContextAndProof() (context.Context, *entity.CODProof) {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
	}
	ctx = tenant.WithTenant(ctx, shop)

	proof, _ := entity.NewCODProof("order-123", shopID.String(), "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")

	return ctx, proof
}

// createTestContext crée un contexte tenant seul
func createTestContext() context.Context {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:   shopID,
		Name: "Test Shop",
	}
	return tenant.WithTenant(ctx, shop)
}

// ============================================================
// TESTS : CollectCommissionUsecase - Validation
// ============================================================

func TestCollectCommissionUsecase_Validation_EmptyOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()
	req := &codusecase.CollectCommissionRequest{
		OrderID:     "", // Vide
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "validation error")
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestCollectCommissionUsecase_Validation_EmptyCollectedBy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()
	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "", // Vide
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "validation error")
	assert.Contains(t, err.Error(), "collected_by is required")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Multi-tenant
// ============================================================

func TestCollectCommissionUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	// Contexte SANS tenant
	ctx := context.Background()
	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Transaction
// ============================================================

func TestCollectCommissionUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()

	// Mock : BeginTx échoue
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(nil, errors.New("database connection failed"))

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Proof non trouvé
// ============================================================

func TestCollectCommissionUsecase_ProofNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()

	// Mock : BeginTx réussit
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(mockTx, nil)

	// Mock : Rollback appelé (defer)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// Mock : WithTX retourne le même repo
	mockCODProofRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockCODProofRepo)

	// Mock : FindByOrderID retourne une erreur
	mockCODProofRepo.EXPECT().
		FindByOrderID(gomock.Any(), "order-123").
		Return(nil, errors.New("proof not found"))

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "COD proof not found")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Proof incomplet
// ============================================================

func TestCollectCommissionUsecase_ProofIncomplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()
	shop, _ := tenant.FromContext(ctx)

	// Créer une preuve incomplète (seulement client) avec le bon ShopID
	incompleteProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	incompleteProof.SubmitClientProof(
		"https://example.com/client.jpg",
		50000,
		time.Now().Add(-1*time.Hour),
		"",
		"",
	)

	// Mock : BeginTx réussit
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(mockTx, nil)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockCODProofRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockCODProofRepo)

	mockCODProofRepo.EXPECT().
		FindByOrderID(gomock.Any(), "order-123").
		Return(incompleteProof, nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "both client and merchant proofs are required")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Proof incohérent
// ============================================================

func TestCollectCommissionUsecase_ProofIncoherent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()
	shop, _ := tenant.FromContext(ctx)

	// Créer une preuve incohérente avec le bon ShopID
	incoherentProof, _ := entity.NewCODProof("order-123", shop.ID.String(), "customer-123", 50000)
	paymentDate := time.Now().Add(-1 * time.Hour)
	incoherentProof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	incoherentProof.SubmitMerchantProof("https://example.com/merchant.jpg", 40000, paymentDate, "")

	// Mock : BeginTx réussit
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(mockTx, nil)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockCODProofRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockCODProofRepo)

	mockCODProofRepo.EXPECT().
		FindByOrderID(gomock.Any(), "order-123").
		Return(incoherentProof, nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:      "order-123",
		CollectedBy:  "merchant-123",
		ForceCollect: false, // Pas de force
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "proofs are incoherent")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Commission déjà collectée
// ============================================================

func TestCollectCommissionUsecase_CommissionAlreadyCollected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx, collectedProof := createTestContextAndProof()
	collectedProof.CommissionStatus = entity.CODCommissionCollected

	// Mock : BeginTx réussit
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(mockTx, nil)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockCODProofRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockCODProofRepo)

	mockCODProofRepo.EXPECT().
		FindByOrderID(gomock.Any(), "order-123").
		Return(collectedProof, nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "commission already collected")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Multi-tenant mismatch
// ============================================================

func TestCollectCommissionUsecase_ProofShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		nil, // walletUC
		nil, // freezeUC
		mockTxManager,
	)

	ctx := createTestContext()

	// Créer une preuve pour un AUTRE shop
	wrongShopProof, _ := entity.NewCODProof("order-123", "other-shop-456", "customer-123", 50000)
	paymentDate := time.Now().Add(-1 * time.Hour)
	wrongShopProof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	wrongShopProof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")

	// Mock : BeginTx réussit
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(mockTx, nil)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockCODProofRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockCODProofRepo)

	mockCODProofRepo.EXPECT().
		FindByOrderID(gomock.Any(), "order-123").
		Return(wrongShopProof, nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "access denied")
	assert.Contains(t, err.Error(), "does not belong to tenant shop")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Happy Path
// ============================================================

func TestCollectCommissionUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	// ✅ NOUVEAU : Mocks pour les interfaces
	mockWalletUC := mockusecase.NewMockWalletDebiter(ctrl)
	mockFreezeUC := mockusecase.NewMockAccountFreezer(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockWalletUC,
		mockFreezeUC,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProof()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().
		BeginTx(gomock.Any()).
		Return(mockTx, nil)

	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	mockCODProofRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockCODProofRepo).
		AnyTimes()

	mockCODProofRepo.EXPECT().
		FindByOrderID(gomock.Any(), "order-123").
		Return(validProof, nil)

	// ✅ Mock : GetWallet (wallet positif)
	mockWalletUC.EXPECT().
		GetWallet(gomock.Any(), shop.ID.String()).
		Return(&entity.MerchantWallet{
			ShopID:       shop.ID.String(),
			BalanceCents: 100000, // 1000 FCFA (positif)
		}, nil)

	// ✅ Mock : Execute (débit réussi)
	mockWalletUC.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(&walletusecase.DebitWalletResponse{
			TransactionID:     "txn-123",
			BalanceAfterCents: 98750, // 100000 - 1250 (commission)
			ShouldFreeze:      false, // Wallet reste positif
		}, nil)

	// Mock : Update (sauvegarde preuve)
	mockCODProofRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODCommissionCollected, response.CommissionStatus)
	assert.Equal(t, "txn-123", *response.WalletTransactionID)
	assert.Equal(t, int64(98750), response.WalletBalanceAfter)
	assert.False(t, response.AccountFrozen)
	assert.Equal(t, "Commission collected successfully", response.Message)
}

func TestCollectCommissionUsecase_SuccessWithAutoFreeze(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	mockWalletUC := mockusecase.NewMockWalletDebiter(ctrl)
	mockFreezeUC := mockusecase.NewMockAccountFreezer(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockWalletUC,
		mockFreezeUC,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProof()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : GetWallet (wallet faible)
	mockWalletUC.EXPECT().
		GetWallet(gomock.Any(), shop.ID.String()).
		Return(&entity.MerchantWallet{
			ShopID:       shop.ID.String(),
			BalanceCents: 1000, // 10 FCFA (faible)
		}, nil)

	// Mock : Execute (débit réussi mais wallet passe en négatif)
	mockWalletUC.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(&walletusecase.DebitWalletResponse{
			TransactionID:     "txn-456",
			BalanceAfterCents: -250, // Négatif !
			ShouldFreeze:      true, // Déclenche le gel
		}, nil)

	// ✅ Mock : FreezeForNegativeBalance
	mockFreezeUC.EXPECT().
		FreezeForNegativeBalance(gomock.Any(), shop.ID.String(), int64(250)).
		Return(&walletusecase.FreezeAccountResponse{
			FreezeID:       "freeze-123",
			AmountDueCents: 250,
		}, nil)

	// Mock : Update
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODCommissionCollected, response.CommissionStatus)
	assert.True(t, response.AccountFrozen)
	assert.Equal(t, "freeze-123", response.FreezeID)
	assert.Equal(t, int64(250), response.AmountDueCents)
	assert.Equal(t, "Commission collected, account frozen due to negative balance", response.Message)
}

func TestCollectCommissionUsecase_DebitFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)

	mockWalletUC := mockusecase.NewMockWalletDebiter(ctrl)
	mockFreezeUC := mockusecase.NewMockAccountFreezer(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockWalletUC,
		mockFreezeUC,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProof()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : GetWallet
	mockWalletUC.EXPECT().
		GetWallet(gomock.Any(), shop.ID.String()).
		Return(&entity.MerchantWallet{
			ShopID:       shop.ID.String(),
			BalanceCents: 100000,
		}, nil)

	// ✅ Mock : Execute échoue (wallet gelé)
	mockWalletUC.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("wallet is frozen"))

	// Mock : Update
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err) // Pas d'erreur, commission marquée "due"
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODCommissionDue, response.CommissionStatus)
	assert.Contains(t, response.Message, "Commission could not be collected")
	assert.Contains(t, response.Message, "wallet is frozen")
}

// ============================================================
// TESTS : CollectCommissionUsecase - Cas limites
// ============================================================

func TestCollectCommissionUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)
	mockWalletUC := mockusecase.NewMockWalletDebiter(ctrl)
	mockFreezeUC := mockusecase.NewMockAccountFreezer(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockWalletUC,
		mockFreezeUC,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProof()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Wallet
	mockWalletUC.EXPECT().GetWallet(gomock.Any(), shop.ID.String()).Return(&entity.MerchantWallet{
		ShopID:       shop.ID.String(),
		BalanceCents: 100000,
	}, nil)

	mockWalletUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(&walletusecase.DebitWalletResponse{
		TransactionID:     "txn-123",
		BalanceAfterCents: 98750,
		ShouldFreeze:      false,
	}, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// ✅ Mock : Commit ÉCHOUE
	mockTx.EXPECT().Commit().Return(errors.New("database commit failed"))

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestCollectCommissionUsecase_FreezeFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)
	mockWalletUC := mockusecase.NewMockWalletDebiter(ctrl)
	mockFreezeUC := mockusecase.NewMockAccountFreezer(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockWalletUC,
		mockFreezeUC,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProof()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Wallet faible
	mockWalletUC.EXPECT().GetWallet(gomock.Any(), shop.ID.String()).Return(&entity.MerchantWallet{
		ShopID:       shop.ID.String(),
		BalanceCents: 1000,
	}, nil)

	// Mock : Débit réussi mais wallet négatif
	mockWalletUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(&walletusecase.DebitWalletResponse{
		TransactionID:     "txn-456",
		BalanceAfterCents: -250,
		ShouldFreeze:      true,
	}, nil)

	// ✅ Mock : Freeze ÉCHOUE
	mockFreezeUC.EXPECT().
		FreezeForNegativeBalance(gomock.Any(), shop.ID.String(), int64(250)).
		Return(nil, errors.New("freeze service unavailable"))

	// Mock : Update
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.NoError(t, err) // Pas d'erreur propagée
	assert.NotNil(t, response)
	assert.Equal(t, entity.CODCommissionCollected, response.CommissionStatus)
	assert.False(t, response.AccountFrozen) // Freeze échoué
	assert.Contains(t, response.Message, "freeze failed")
}

func TestCollectCommissionUsecase_RollbackCalledOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCODProofRepo := mockrepo.NewMockCODProofRepository(ctrl)
	mockOrderRepo := mockrepo.NewMockOrderRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := createMockTx(ctrl)
	mockWalletUC := mockusecase.NewMockWalletDebiter(ctrl)
	mockFreezeUC := mockusecase.NewMockAccountFreezer(ctrl)

	uc := codusecase.NewCollectCommissionUsecase(
		mockCODProofRepo,
		mockOrderRepo,
		mockWalletUC,
		mockFreezeUC,
		mockTxManager,
	)

	ctx, validProof := createTestContextAndProof()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Transaction
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// ✅ Vérifie que Rollback est appelé exactement 1 fois
	mockTx.EXPECT().Rollback().Return(nil).Times(1)

	mockCODProofRepo.EXPECT().WithTX(gomock.Any()).Return(mockCODProofRepo).AnyTimes()
	mockCODProofRepo.EXPECT().FindByOrderID(gomock.Any(), "order-123").Return(validProof, nil)

	// Mock : Wallet
	mockWalletUC.EXPECT().GetWallet(gomock.Any(), shop.ID.String()).Return(&entity.MerchantWallet{
		ShopID:       shop.ID.String(),
		BalanceCents: 100000,
	}, nil)

	mockWalletUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(&walletusecase.DebitWalletResponse{
		TransactionID:     "txn-123",
		BalanceAfterCents: 98750,
		ShouldFreeze:      false,
	}, nil)

	// Mock : Update réussit
	mockCODProofRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// Mock : Commit échoue (déclenche le rollback)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	response, err := uc.Execute(ctx, req)

	// ✅ Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	// Le test échouera si Rollback n'est pas appelé exactement 1 fois
}
