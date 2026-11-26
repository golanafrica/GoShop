package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"
	mockservice "Goshop/mocks/service"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCompensationPending_AtomicRollbackOnWithdrawalUpdateFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	withdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	walletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	txnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	escrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)

	creditWalletUC := walletusecase.NewCreditWalletUsecase(walletRepo, txnRepo, escrowRepo, txManager)
	sched := NewCompensationPendingScheduler(
		nil,
		withdrawalRepo,
		creditWalletUC,
		txManager,
		notifService,
		zerolog.Nop(),
	)

	ctx := context.Background()
	shopID := uuid.New()
	withdrawalID := uuid.New()

	w := &entity.Withdrawal{
		ID:                   withdrawalID,
		ShopID:               shopID,
		AmountCents:          50000,
		Status:               entity.WithdrawalStatusCompensationPending,
		CompensationAttempts: 0,
	}

	mockTx := mockrepo.NewMockTx(ctrl)

	// 1. Démarrer la transaction
	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// 2. Verrouiller le retrait
	withdrawalRepo.EXPECT().WithTX(mockTx).Return(withdrawalRepo).AnyTimes()
	withdrawalRepo.EXPECT().FindByIDForUpdate(gomock.Any(), withdrawalID).Return(w, nil)

	// 3. Préparer le wallet
	walletRepo.EXPECT().WithTX(mockTx).Return(walletRepo).AnyTimes()
	wallet := entity.NewMerchantWallet(shopID.String())
	wallet.BalanceCents = 10000
	walletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), shopID.String()).Return(wallet, nil)

	// 4. Opérations de transaction (Idempotence + Création)
	txnRepo.EXPECT().WithTX(mockTx).Return(txnRepo).AnyTimes()

	// ✅ FIX : Utiliser gomock.Any() pour garantir que l'appel est intercepté, peu importe le contexte
	txnRepo.EXPECT().
		FindByReferenceID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, sql.ErrNoRows).
		AnyTimes()

	walletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	txnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	// 5. ⚠️ POINT CRITIQUE : La mise à jour du retrait ÉCHOUÉ
	withdrawalRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db connection lost"))

	// 6. ✅ PREUVE D'ATOMICITÉ : Le rollback DOIT être appelé pour annuler le crédit
	mockTx.EXPECT().Rollback().Return(nil)

	// Exécution
	err := sched.processOne(ctx, w)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update withdrawal status to failed")
}
