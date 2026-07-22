package walletusecase_test

import (
	"context"
	"errors"
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
// HELPERS PARTAGÉS (wallet_usecase package)
// ============================================================

func walletTestContext() (context.Context, uuid.UUID) {
	shopID := uuid.New()
	shop := &entity.Shop{ID: shopID, Name: "Test Shop", IsActive: true}
	return tenant.WithTenant(context.Background(), shop), shopID
}

func creditWalletValidRequest(shopID string) *walletusecase.CreditWalletRequest {
	return &walletusecase.CreditWalletRequest{
		ShopID:          shopID,
		AmountCents:     10000,
		TransactionType: entity.WalletTxSaleCredit,
	}
}

func newCreditWalletUsecase(ctrl *gomock.Controller) (
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

func expectWalletRepoWithTXSelf(walletRepo *mockrepo.MockMerchantWalletRepository, tx interface{}) {
	walletRepo.EXPECT().WithTX(tx).Return(walletRepo).AnyTimes()
}

func expectTxnRepoWithTXSelf(txnRepo *mockrepo.MockWalletTransactionRepository, tx interface{}) {
	txnRepo.EXPECT().WithTX(tx).Return(txnRepo).AnyTimes()
}

// ============================================================
// TESTS : CreditWalletUsecase.Execute()
// ============================================================

func TestCreditWalletUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	req.AmountCents = 0

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestCreditWalletUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newCreditWalletUsecase(ctrl)

	ctx := context.Background()
	req := creditWalletValidRequest("shop-1")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestCreditWalletUsecase_ShopIDMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newCreditWalletUsecase(ctrl)

	ctx, _ := walletTestContext()
	req := creditWalletValidRequest("other-shop-id")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "access denied")
}

func TestCreditWalletUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db down"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestCreditWalletUsecase_WalletNotFound_AutoCreates_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(nil, errors.New("merchant wallet not found"))
	walletRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(0), resp.PreviousBalance)
	assert.Equal(t, int64(10000), resp.BalanceCents)
}

func TestCreditWalletUsecase_WalletNotFound_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(nil, errors.New("merchant wallet not found"))
	walletRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create wallet")
}

func TestCreditWalletUsecase_FindWalletError_Other(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), req.ShopID).Return(nil, errors.New("connection reset"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to find wallet")
}

func TestCreditWalletUsecase_WalletFrozen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
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

func TestCreditWalletUsecase_UpdateWalletError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

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

func TestCreditWalletUsecase_CreateTransactionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

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

func TestCreditWalletUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

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

func TestCreditWalletUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := creditWalletValidRequest(shopID.String())
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
	assert.Equal(t, int64(5000), resp.PreviousBalance)
	assert.Equal(t, int64(15000), resp.BalanceCents)
	assert.NotEmpty(t, resp.TransactionID)
	assert.False(t, resp.IsFrozen)
}

// ============================================================
// TESTS : Méthodes utilitaires (CreditFromXxx)
// ============================================================

func TestCreditWalletUsecase_CreditFromSale_SetsCorrectFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(shopID.String())

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID.String()).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, txn *entity.WalletTransaction) error {
			assert.Equal(t, entity.WalletTxSaleCredit, txn.TransactionType)
			return nil
		},
	)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.CreditFromSale(ctx, shopID.String(), 20000, "order-1")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxSaleCredit, resp.TransactionType)
}

func TestCreditWalletUsecase_CreditFromCOD_SetsCorrectFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, txnRepo, txManager := newCreditWalletUsecase(ctrl)

	ctx, shopID := walletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(shopID.String())

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID.String()).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.CreditFromCOD(ctx, shopID.String(), 15000, "order-2")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.WalletTxCOD, resp.TransactionType)
}
