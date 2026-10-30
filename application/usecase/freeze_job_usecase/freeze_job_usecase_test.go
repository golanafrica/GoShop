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

func TestFreezeJobUsecase_ListGraceExpired_NilDatesNoPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// On mock uniquement le repository qui est réellement appelé par ListGraceExpired
	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)

	// On passe nil pour les autres dépendances car elles ne sont pas utilisées dans cette méthode
	uc := freezejobusecase.NewFreezeJobUsecase(nil, mockWalletRepo, nil)

	ctx := context.Background()
	shopID := uuid.New().String()

	// 🛡️ SCÉNARIO P1 : Données historiques avec FrozenAt et FrozenUntil à nil
	walletsWithNilDates := []*entity.MerchantWallet{
		{
			ShopID:      shopID,
			FrozenAt:    nil, // Pointeur nil
			FrozenUntil: nil, // Pointeur nil
			DebtCents:   50000,
		},
	}

	mockWalletRepo.EXPECT().
		FindGracePeriodExpired(gomock.Any()).
		Return(walletsWithNilDates, nil)

	// L'appel NE DOIT PAS paniquer
	result, err := uc.ListGraceExpired(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, shopID, result[0].ShopID)
	assert.Equal(t, int64(50000), result[0].DebtCents)

	// 🛡️ VÉRIFICATION : Les chaînes de caractères doivent être vides, pas de panic
	assert.Equal(t, "", result[0].FrozenAt)
	assert.Equal(t, "", result[0].FrozenUntil)
	assert.Equal(t, 0, result[0].DaysExpired)
}
