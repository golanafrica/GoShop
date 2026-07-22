package walletusecase_test

import (
	"context"
	"testing"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.16 : TESTS DES MÉTHODES HELPERS WALLET
// ============================================================

// ============================================================
// HELPERS COMMUNS
// ============================================================

func createWalletTestContext() (context.Context, string) {
	shop := &entity.Shop{
		ID:       uuid.New(),
		Name:     "Test Shop",
		IsActive: true,
	}
	return tenant.WithTenant(context.Background(), shop), shop.ID.String()
}

func setupCreditWalletMocks(ctrl *gomock.Controller) (
	*walletusecase.CreditWalletUsecase,
	*mockrepo.MockMerchantWalletRepository,
	*mockrepo.MockWalletTransactionRepository,
	*mockrepo.MockTxManager,
) {
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	txnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewCreditWalletUsecase(walletRepo, txnRepo, txManager)
	return uc, walletRepo, txnRepo, txManager
}

func setupDebitWalletMocks(ctrl *gomock.Controller) (
	*walletusecase.DebitWalletUsecase,
	*mockrepo.MockMerchantWalletRepository,
	*mockrepo.MockWalletTransactionRepository,
	*mockrepo.MockTxManager,
) {
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	txnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewDebitWalletUsecase(walletRepo, txnRepo, txManager)
	return uc, walletRepo, txnRepo, txManager
}

// simulateSuccessfulCredit simule un crédit réussi
func simulateSuccessfulCredit(
	walletRepo *mockrepo.MockMerchantWalletRepository,
	txnRepo *mockrepo.MockWalletTransactionRepository,
	txManager *mockrepo.MockTxManager,
	mockTx *mockrepo.MockTx,
	shopID string,
) {
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	txnRepo.EXPECT().WithTX(mockTx).Return(txnRepo).AnyTimes()

	wallet := entity.NewMerchantWallet(shopID)
	wallet.BalanceCents = 50000

	// ✅ CORRECTION : Utilisation de FindByShopIDForUpdate
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
}

// simulateSuccessfulDebit simule un débit réussi
func simulateSuccessfulDebit(
	walletRepo *mockrepo.MockMerchantWalletRepository,
	txnRepo *mockrepo.MockWalletTransactionRepository,
	txManager *mockrepo.MockTxManager,
	mockTx *mockrepo.MockTx,
	shopID string,
) {
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	txnRepo.EXPECT().WithTX(mockTx).Return(txnRepo).AnyTimes()

	wallet := entity.NewMerchantWallet(shopID)
	wallet.BalanceCents = 100000

	// ✅ CORRECTION : Utilisation de FindByShopIDForUpdate
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
}

// ============================================================
// TESTS : CreditWalletUsecase - CreditFromSale
// ============================================================

func TestCreditWalletUsecase_CreditFromSale_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupCreditWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulCredit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.CreditFromSale(ctx, shopID, 10000, "order-123")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxSaleCredit, resp.TransactionType)
	assert.Equal(t, int64(10000), resp.AmountCents)
}

// ============================================================
// TESTS : CreditWalletUsecase - CreditFromCOD
// ============================================================

func TestCreditWalletUsecase_CreditFromCOD_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupCreditWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulCredit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.CreditFromCOD(ctx, shopID, 10000, "order-456")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxCOD, resp.TransactionType)
}

// ============================================================
// TESTS : CreditWalletUsecase - CreditFromTontine
// ============================================================

func TestCreditWalletUsecase_CreditFromTontine_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupCreditWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulCredit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.CreditFromTontine(ctx, shopID, 10000, "group-789")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxSaleTontine, resp.TransactionType)
}

// ============================================================
// TESTS : CreditWalletUsecase - CreditFromCreditPlan
// ============================================================

func TestCreditWalletUsecase_CreditFromCreditPlan_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupCreditWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulCredit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.CreditFromCreditPlan(ctx, shopID, 10000, "contract-999")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxSaleCreditPlan, resp.TransactionType)
}

// ============================================================
// TESTS : CreditWalletUsecase - CreditFromDeposit
// ============================================================

func TestCreditWalletUsecase_CreditFromDeposit_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupCreditWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulCredit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.CreditFromDeposit(ctx, shopID, 10000, "Manual deposit")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxDeposit, resp.TransactionType)
}

// ============================================================
// TESTS : CreditWalletUsecase - CreditFromUnfreeze
// ============================================================

func TestCreditWalletUsecase_CreditFromUnfreeze_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupCreditWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulCredit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.CreditFromUnfreeze(ctx, shopID, 10000)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxUnfreezeDeposit, resp.TransactionType)
}

// ============================================================
// TESTS : DebitWalletUsecase - DebitCommission
// ============================================================

func TestDebitWalletUsecase_DebitCommission_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupDebitWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulDebit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.DebitCommission(ctx, shopID, 10000, "order", "order-123")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxCommissionDebit, resp.TransactionType)
}

// ============================================================
// TESTS : DebitWalletUsecase - DebitRefund
// ============================================================

func TestDebitWalletUsecase_DebitRefund_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupDebitWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulDebit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.DebitRefund(ctx, shopID, 10000, "order-456")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxRefund, resp.TransactionType)
}

// ============================================================
// TESTS : DebitWalletUsecase - DebitPayout
// ============================================================

func TestDebitWalletUsecase_DebitPayout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupDebitWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulDebit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.DebitPayout(ctx, shopID, 10000, "payout-789")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxPayout, resp.TransactionType)
}

// ============================================================
// TESTS : DebitWalletUsecase - DebitPenalty
// ============================================================

func TestDebitWalletUsecase_DebitPenalty_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := setupDebitWalletMocks(ctrl)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	simulateSuccessfulDebit(walletRepo, txnRepo, txManager, mockTx, shopID)

	resp, err := uc.DebitPenalty(ctx, shopID, 10000, "Violation of terms")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxFreezePenalty, resp.TransactionType)
}

// ============================================================
// TESTS : FreezeAccountUsecase - FreezeForNegativeBalance
// ============================================================

func TestFreezeAccountUsecase_FreezeForNegativeBalance_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	freezeRepo := mockrepo.NewMockAccountFreezeRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txManager)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	freezeRepo.EXPECT().WithTX(mockTx).Return(freezeRepo).AnyTimes()

	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), shopID).Return(nil, nil)

	wallet := entity.NewMerchantWallet(shopID)
	// ✅ CORRECTION : Utilisation de FindByShopIDForUpdate
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, freeze *entity.AccountFreeze) error {
			freeze.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.FreezeForNegativeBalance(ctx, shopID, 50000)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.FreezeReasonNegativeBalance, resp.Reason)
}

// ============================================================
// TESTS : FreezeAccountUsecase - FreezeForUnpaidCommission
// ============================================================

func TestFreezeAccountUsecase_FreezeForUnpaidCommission_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	freezeRepo := mockrepo.NewMockAccountFreezeRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txManager)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	freezeRepo.EXPECT().WithTX(mockTx).Return(freezeRepo).AnyTimes()

	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), shopID).Return(nil, nil)

	wallet := entity.NewMerchantWallet(shopID)
	// ✅ CORRECTION : Utilisation de FindByShopIDForUpdate
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, freeze *entity.AccountFreeze) error {
			freeze.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.FreezeForUnpaidCommission(ctx, shopID, 50000, "order-123")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.FreezeReasonUnpaidCommission, resp.Reason)
}

// ============================================================
// TESTS : FreezeAccountUsecase - FreezeForFraud
// ============================================================

func TestFreezeAccountUsecase_FreezeForFraud_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	freezeRepo := mockrepo.NewMockAccountFreezeRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txManager)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	freezeRepo.EXPECT().WithTX(mockTx).Return(freezeRepo).AnyTimes()

	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), shopID).Return(nil, nil)

	wallet := entity.NewMerchantWallet(shopID)
	// ✅ CORRECTION : Utilisation de FindByShopIDForUpdate
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, freeze *entity.AccountFreeze) error {
			freeze.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.FreezeForFraud(ctx, shopID, 50000, "Suspicious activity detected")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.FreezeReasonFraudSuspected, resp.Reason)
}

// ============================================================
// TESTS : FreezeAccountUsecase - FreezeByAdmin
// ============================================================

func TestFreezeAccountUsecase_FreezeByAdmin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	freezeRepo := mockrepo.NewMockAccountFreezeRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txManager)
	ctx, shopID := createWalletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	freezeRepo.EXPECT().WithTX(mockTx).Return(freezeRepo).AnyTimes()

	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), shopID).Return(nil, nil)

	wallet := entity.NewMerchantWallet(shopID)
	// ✅ CORRECTION : Utilisation de FindByShopIDForUpdate
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, freeze *entity.AccountFreeze) error {
			freeze.ID = uuid.New().String()
			return nil
		},
	)

	resp, err := uc.FreezeByAdmin(ctx, shopID, 50000, "Admin decision: policy violation")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.FreezeReasonAdminDecision, resp.Reason)
}
