package freezejobusecase_test

import (
	"context"
	"testing"

	freezejobusecase "Goshop/application/usecase/freeze_job_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// 🛡️ Test existant : Vérifie que les dates nulles ne provoquent pas de panic
func TestFreezeJobUsecase_ListGraceExpired_NilDatesNoPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	uc := freezejobusecase.NewFreezeJobUsecase(nil, mockWalletRepo, nil)

	ctx := context.Background()
	shopID := uuid.New().String()

	walletsWithNilDates := []*entity.MerchantWallet{
		{
			ShopID:      shopID,
			FrozenAt:    nil,
			FrozenUntil: nil,
			DebtCents:   50000,
		},
	}

	mockWalletRepo.EXPECT().
		FindGracePeriodExpired(gomock.Any()).
		Return(walletsWithNilDates, nil)

	result, err := uc.ListGraceExpired(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, shopID, result[0].ShopID)
	assert.Equal(t, int64(50000), result[0].DebtCents)

	assert.Equal(t, "", result[0].FrozenAt)
	assert.Equal(t, "", result[0].FrozenUntil)
	assert.Equal(t, 0, result[0].DaysExpired)
}

// 🛡️ A07 FIX : Test de la nouvelle signature de RunJob (validation du mode)
func TestFreezeJobUsecase_RunJob_InvalidMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)

	// On passe nil pour settingsRepo car isSuperAdmin=true empêche l'appel à GetConfig
	uc := freezejobusecase.NewFreezeJobUsecase(nil, mockWalletRepo, nil)
	ctx := context.Background()

	// Le mode doit être "live" ou "dry_run". On teste un mode invalide.
	runID, err := uc.RunJob(ctx, "invalid_mode", true)

	assert.Error(t, err)
	assert.Empty(t, runID)
	assert.Contains(t, err.Error(), "invalid mode")
}

// 🛡️ A07 FIX : Test de RunJob avec mode valide mais scheduler non configuré
func TestFreezeJobUsecase_RunJob_SchedulerNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)

	// On passe nil pour settingsRepo car isSuperAdmin=true empêche l'appel à GetConfig
	uc := freezejobusecase.NewFreezeJobUsecase(nil, mockWalletRepo, nil)
	ctx := context.Background()

	runID, err := uc.RunJob(ctx, "live", true)

	assert.Error(t, err)
	assert.Empty(t, runID)
	assert.Contains(t, err.Error(), "scheduler is not configured")
}
