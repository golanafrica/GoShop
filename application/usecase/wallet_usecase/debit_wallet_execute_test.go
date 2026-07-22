package walletusecase_test

import (
	"context"
	"errors"
	"testing"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

func debitWalletValidRequest(shopID string) *walletusecase.DebitWalletRequest {
	return &walletusecase.DebitWalletRequest{
		ShopID:          shopID,
		AmountCents:     10000,
		TransactionType: entity.WalletTxCommissionDebit,
		AllowNegative:   true,
	}
}

func newDebitWalletUsecase(ctrl *gomock.Controller) (
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

// ============================================================
// TESTS : DebitWalletUsecase.Execute()
// ============================================================

func TestDebitWalletUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	req.AmountCents = 0

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestDebitWalletUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newDebitWalletUsecase(ctrl)

	ctx := context.Background()
	req := debitWalletValidRequest("shop-1")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestDebitWalletUsecase_ShopIDMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newDebitWalletUsecase(ctrl)

	ctx, _ := walletTestContext()
	req := debitWalletValidRequest("other-shop-id")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "access denied")
}

func TestDebitWalletUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db down"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestDebitWalletUsecase_FindWalletError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(nil, errors.New("not found"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to find wallet")
}

func TestDebitWalletUsecase_WalletFrozen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "wallet is frozen")
}

func TestDebitWalletUsecase_ExceedsMaxNegative_Blocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	req.AmountCents = 500000
	req.AllowNegative = false
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "exceed max negative balance")
}

func TestDebitWalletUsecase_UpdateWalletError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = 20000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to update wallet")
}

func TestDebitWalletUsecase_CreateTransactionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = 20000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create transaction")
}

func TestDebitWalletUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = 20000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestDebitWalletUsecase_Success_NotNegative(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = 50000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(40000), resp.BalanceCents)
	assert.False(t, resp.IsNowNegative)
	assert.False(t, resp.ShouldFreeze)
}

func TestDebitWalletUsecase_Success_BecomesNegative_ShouldFreeze(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	req.AmountCents = 15000
	req.AllowNegative = true
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = 5000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(-10000), resp.BalanceCents)
	assert.True(t, resp.IsNowNegative)
	assert.True(t, resp.ShouldFreeze)
}

func TestDebitWalletUsecase_Success_AlreadyNegative_NoNewFreeze(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	req.AmountCents = 5000
	req.AllowNegative = true
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = -10000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsNowNegative)
	assert.False(t, resp.ShouldFreeze)
}

// ============================================================
// TESTS : Méthodes utilitaires (DebitXxx)
// ============================================================

func TestDebitWalletUsecase_DebitCommission_AllowsNegative_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(shopID.String())
	wallet.BalanceCents = 1000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID.String()).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, txn *entity.WalletTransaction) error {
			assert.Equal(t, entity.WalletTxCommissionDebit, txn.TransactionType)
			return nil
		},
	)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.DebitCommission(ctx, shopID.String(), 5000, "order", "order-99")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxCommissionDebit, resp.TransactionType)
}

func TestDebitWalletUsecase_DebitPayout_SetsCorrectFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newDebitWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(shopID.String())
	wallet.BalanceCents = 100000

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID.String()).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.DebitPayout(ctx, shopID.String(), 50000, "payout-1")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxPayout, resp.TransactionType)
}

// ============================================================
// TESTS : GetWallet
// ============================================================

func TestDebitWalletUsecase_GetWallet_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, _, _ := newDebitWalletUsecase(ctrl)

	wallet := entity.NewMerchantWallet("shop-1")
	walletRepo.EXPECT().FindByShopID(gomock.Any(), "shop-1").Return(wallet, nil)

	result, err := uc.GetWallet(context.Background(), "shop-1")

	assert.NoError(t, err)
	assert.Equal(t, wallet, result)
}

func TestDebitWalletUsecase_GetWallet_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, _, _ := newDebitWalletUsecase(ctrl)

	walletRepo.EXPECT().FindByShopID(gomock.Any(), "shop-1").Return(nil, errors.New("not found"))

	result, err := uc.GetWallet(context.Background(), "shop-1")

	assert.Error(t, err)
	assert.Nil(t, result)
}
