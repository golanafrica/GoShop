package walletusecase_test

import (
	"context"
	"errors"
	"testing"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS
// ============================================================

func freezeAccountValidRequest(shopID string) *walletusecase.FreezeAccountRequest {
	details := "Test freeze details"
	return &walletusecase.FreezeAccountRequest{
		ShopID:         shopID,
		Reason:         entity.FreezeReasonNegativeBalance,
		AmountDueCents: 50000,
		Details:        &details,
	}
}

func unfreezeAccountValidRequest(shopID string) *walletusecase.UnfreezeAccountRequest {
	return &walletusecase.UnfreezeAccountRequest{
		ShopID:     shopID,
		Resolution: entity.FreezeResolutionSuspended, // évite la branche "paid" (voir note plus bas)
		ResolvedBy: "admin-1",
	}
}

func newFreezeAccountUsecase(ctrl *gomock.Controller) (
	*walletusecase.FreezeAccountUsecase,
	*mockrepo.MockMerchantWalletRepository,
	*mockrepo.MockAccountFreezeRepository,
	*mockrepo.MockTxManager,
) {
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	freezeRepo := mockrepo.NewMockAccountFreezeRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewFreezeAccountUsecase(walletRepo, freezeRepo, txManager)
	return uc, walletRepo, freezeRepo, txManager
}

func newUnfreezeAccountUsecase(ctrl *gomock.Controller) (
	*walletusecase.UnfreezeAccountUsecase,
	*mockrepo.MockMerchantWalletRepository,
	*mockrepo.MockAccountFreezeRepository,
	*mockrepo.MockWalletTransactionRepository,
	*mockrepo.MockTxManager,
) {
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	freezeRepo := mockrepo.NewMockAccountFreezeRepository(ctrl)
	txnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	uc := walletusecase.NewUnfreezeAccountUsecase(walletRepo, freezeRepo, txnRepo, txManager)
	return uc, walletRepo, freezeRepo, txnRepo, txManager
}

func expectFreezeRepoWithTXSelf(freezeRepo *mockrepo.MockAccountFreezeRepository, tx interface{}) {
	freezeRepo.EXPECT().WithTX(tx).Return(freezeRepo).AnyTimes()
}

// ============================================================
// TESTS : FreezeAccountUsecase.Execute()
// ============================================================

func TestFreezeAccountUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	req.AmountDueCents = 0

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestFreezeAccountUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newFreezeAccountUsecase(ctrl)

	ctx := context.Background()
	req := freezeAccountValidRequest("shop-1")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestFreezeAccountUsecase_ShopIDMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _ := newFreezeAccountUsecase(ctrl)

	ctx, _ := walletTestContext()
	req := freezeAccountValidRequest("other-shop-id")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "access denied")
}

func TestFreezeAccountUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db down"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestFreezeAccountUsecase_ExistingActiveFreeze(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	existingFreeze := &entity.AccountFreeze{ID: "freeze-existing", ShopID: req.ShopID}

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(existingFreeze, nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already has an active freeze")
}

func TestFreezeAccountUsecase_WalletNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("not found"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to find wallet")
}

func TestFreezeAccountUsecase_WalletAlreadyFrozen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already frozen")
}

func TestFreezeAccountUsecase_UpdateWalletError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to update wallet")
}

func TestFreezeAccountUsecase_SaveFreezeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to save account freeze")
}

func TestFreezeAccountUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestFreezeAccountUsecase_Success_DefaultGracePeriod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// ✅ CORRECTION : Utiliser DoAndReturn pour simuler l'assignation de l'ID
	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, freeze *entity.AccountFreeze) error {
			if freeze.ID == "" {
				freeze.ID = uuid.New().String()
			}
			return nil
		},
	)

	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.FreezeID)
	assert.Equal(t, entity.DefaultGracePeriodDays, resp.DaysRemaining)
	assert.True(t, resp.WalletIsFrozen)
}

func TestFreezeAccountUsecase_Success_CustomGracePeriod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := freezeAccountValidRequest(shopID.String())
	customGrace := 14
	req.GracePeriodDays = &customGrace
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 14, resp.DaysRemaining)
}

// ============================================================
// TESTS : UnfreezeAccountUsecase.Execute()
// ============================================================

func TestUnfreezeAccountUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _ := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	req.ResolvedBy = ""

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "validation error")
}

func TestUnfreezeAccountUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _ := newUnfreezeAccountUsecase(ctrl)

	ctx := context.Background()
	req := unfreezeAccountValidRequest("shop-1")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestUnfreezeAccountUsecase_ShopIDMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _ := newUnfreezeAccountUsecase(ctrl)

	ctx, _ := walletTestContext()
	req := unfreezeAccountValidRequest("other-shop-id")

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "access denied")
}

func TestUnfreezeAccountUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("db down"))

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

func TestUnfreezeAccountUsecase_NoActiveFreezeFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(nil, errors.New("not found"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no active freeze found")
}

func TestUnfreezeAccountUsecase_WalletNotFrozen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	freeze := &entity.AccountFreeze{ID: "freeze-1", ShopID: req.ShopID}
	wallet := entity.NewMerchantWallet(req.ShopID) // IsFrozen = false

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(freeze, nil)
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not frozen")
}

// NOTE IMPORTANTE : dans freeze_account.go, la résolution "paid" appelle
// wallet.Credit(...) AVANT wallet.ForceUnfreeze(). Or MerchantWallet.Credit()
// bloque explicitement si wallet.IsFrozen == true. Le wallet est encore gelé
// à ce stade de l'exécution (le ForceUnfreeze n'a pas encore eu lieu), donc
// ce chemin échoue systématiquement avec l'erreur "wallet is frozen, cannot
// credit". Ce test documente ce comportement réel du code (probable bug
// d'ordonnancement des étapes à signaler côté métier).
func TestUnfreezeAccountUsecase_ResolutionPaid_CreditFailsBecauseWalletStillFrozen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	req.Resolution = entity.FreezeResolutionPaid
	req.DepositAmountCents = 50000
	mockTx := mockrepo.NewMockTx(ctrl)
	freeze := &entity.AccountFreeze{ID: "freeze-1", ShopID: req.ShopID, AmountDueCents: 50000}
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(freeze, nil)
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to credit wallet")
}

func TestUnfreezeAccountUsecase_ResolutionSuspended_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	req.Resolution = entity.FreezeResolutionSuspended
	mockTx := mockrepo.NewMockTx(ctrl)
	freeze := &entity.AccountFreeze{ID: "freeze-1", ShopID: req.ShopID}
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(freeze, nil)
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().UpdateResolution(gomock.Any(), freeze.ID, req.Resolution, req.ResolvedBy).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.WalletIsFrozen)
	assert.Equal(t, entity.FreezeResolutionSuspended, resp.Resolution)
}

func TestUnfreezeAccountUsecase_UpdateWalletError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	freeze := &entity.AccountFreeze{ID: "freeze-1", ShopID: req.ShopID}
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(freeze, nil)
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to update wallet")
}

func TestUnfreezeAccountUsecase_ResolveFreezeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	freeze := &entity.AccountFreeze{ID: "freeze-1", ShopID: req.ShopID}
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(freeze, nil)
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().UpdateResolution(gomock.Any(), freeze.ID, req.Resolution, req.ResolvedBy).Return(errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to resolve freeze")
}

func TestUnfreezeAccountUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txnRepo, txManager := newUnfreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := unfreezeAccountValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	freeze := &entity.AccountFreeze{ID: "freeze-1", ShopID: req.ShopID}
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)
	expectTxnRepoWithTXSelf(txnRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), req.ShopID).Return(freeze, nil)
	walletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().UpdateResolution(gomock.Any(), freeze.ID, req.Resolution, req.ResolvedBy).Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

// ============================================================
// TESTS : Méthodes utilitaires FreezeAccountUsecase (FreezeForXxx)
// ============================================================

func TestFreezeAccountUsecase_FreezeForNegativeBalance_SetsCorrectReason(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, walletRepo, freezeRepo, txManager := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(shopID.String())

	expectWalletRepoWithTXSelf(walletRepo, mockTx)
	expectFreezeRepoWithTXSelf(freezeRepo, mockTx)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	freezeRepo.EXPECT().FindActiveByShopID(gomock.Any(), shopID.String()).Return(nil, errors.New("no active freeze"))
	walletRepo.EXPECT().FindByShopID(gomock.Any(), shopID.String()).Return(wallet, nil)
	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	freezeRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, f *entity.AccountFreeze) error {
			assert.Equal(t, entity.FreezeReasonNegativeBalance, f.FreezeReason)
			return nil
		},
	)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	resp, err := uc.FreezeForNegativeBalance(ctx, shopID.String(), 30000)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, entity.FreezeReasonNegativeBalance, resp.Reason)
}

// ============================================================
// TESTS : DebitWithAutoFreeze
// ============================================================

func TestDebitWithAutoFreeze_NoFreeze_WhenShouldFreezeFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	debitUC, debitWalletRepo, debitTxnRepo, debitTxManager := newDebitWalletUsecase(ctrl)
	freezeUC, _, _, _ := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	req.AmountCents = 10000
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.BalanceCents = 50000 // reste positif -> ShouldFreeze = false

	expectWalletRepoWithTXSelf(debitWalletRepo, mockTx)
	expectTxnRepoWithTXSelf(debitTxnRepo, mockTx)

	debitTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	debitWalletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	debitWalletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	debitTxnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)
	// Aucune expectation sur freezeUC : ne doit jamais être appelé

	resp, err := walletusecase.DebitWithAutoFreeze(ctx, debitUC, freezeUC, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.ShouldFreeze)
}

func TestDebitWithAutoFreeze_DebitFails_NoFreezeAttempted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	debitUC, debitWalletRepo, debitTxnRepo, debitTxManager := newDebitWalletUsecase(ctrl)
	freezeUC, _, _, _ := newFreezeAccountUsecase(ctrl)

	ctx, shopID := walletTestContext()
	req := debitWalletValidRequest(shopID.String())
	mockTx := mockrepo.NewMockTx(ctrl)
	wallet := entity.NewMerchantWallet(req.ShopID)
	wallet.IsFrozen = true // fait échouer le débit immédiatement

	expectWalletRepoWithTXSelf(debitWalletRepo, mockTx)
	expectTxnRepoWithTXSelf(debitTxnRepo, mockTx)

	debitTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	debitWalletRepo.EXPECT().FindByShopID(gomock.Any(), req.ShopID).Return(wallet, nil)
	mockTx.EXPECT().Rollback().Return(nil)
	// Aucune expectation sur freezeUC : ne doit jamais être appelé

	resp, err := walletusecase.DebitWithAutoFreeze(ctx, debitUC, freezeUC, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}
