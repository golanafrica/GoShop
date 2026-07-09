package withdrawalusecase_test

import (
	"context"
	"errors"
	"testing"

	withdrawalusecase "Goshop/application/usecase/withdrawal_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.10 : TESTS UNITAIRES - LIST WITHDRAWALS USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestContextForWithdrawal crée un contexte tenant pour les tests withdrawal
func createTestContextForWithdrawal() context.Context {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:        shopID,
		Name:      "Test Shop",
		KYCStatus: entity.ShopKYCStatusVerified,
	}
	return tenant.WithTenant(ctx, shop)
}

// createTestWithdrawal crée un retrait de test
func createTestWithdrawal(shopID uuid.UUID, amountCents int64) *entity.Withdrawal {
	withdrawal, _ := entity.NewWithdrawal(
		shopID,
		amountCents,
		entity.WithdrawalOrangeMoney, // ✅ Bonne constante
		"+22670123456",
	)
	return withdrawal
}

// strPtr helper pour créer des pointeurs de string
func strPtr(s string) *string {
	return &s
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.Execute - Multi-tenant
// ============================================================

func TestListWithdrawalsUsecase_Execute_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	// Contexte SANS tenant
	ctx := context.Background()

	response, err := uc.Execute(ctx, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.Execute - Repository errors
// ============================================================

func TestListWithdrawalsUsecase_Execute_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	shop, _ := tenant.FromContext(ctx)

	// Mock : FindByShopID échoue
	mockWithdrawalRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID, 10, 0).
		Return(nil, errors.New("database error"))

	response, err := uc.Execute(ctx, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find withdrawals")
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.Execute - Happy paths
// ============================================================

func TestListWithdrawalsUsecase_Execute_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Liste vide
	mockWithdrawalRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID, 10, 0).
		Return([]*entity.Withdrawal{}, nil)

	response, err := uc.Execute(ctx, 10, 0)

	assert.NoError(t, err)
	// ✅ CORRECTION : Utiliser assert.Empty au lieu de assert.NotNil
	// En Go, une slice non-initialisée est nil, pas une slice vide
	assert.Empty(t, response)
}

func TestListWithdrawalsUsecase_Execute_SuccessWithMultipleWithdrawals(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	shop, _ := tenant.FromContext(ctx)

	// Créer 3 retraits de test
	withdrawal1 := createTestWithdrawal(shop.ID, 50000)
	withdrawal2 := createTestWithdrawal(shop.ID, 75000)
	withdrawal3 := createTestWithdrawal(shop.ID, 100000)

	withdrawals := []*entity.Withdrawal{withdrawal1, withdrawal2, withdrawal3}

	// Mock : Liste avec 3 retraits
	mockWithdrawalRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID, 10, 0).
		Return(withdrawals, nil)

	response, err := uc.Execute(ctx, 10, 0)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response, 3)

	// Vérifier le mapping DTO
	assert.Equal(t, withdrawal1.ID.String(), response[0].ID)
	assert.Equal(t, shop.ID.String(), response[0].ShopID)
	assert.Equal(t, int64(50000), response[0].AmountCents)
	assert.Equal(t, string(entity.WithdrawalOrangeMoney), response[0].PaymentMethod)
}

func TestListWithdrawalsUsecase_Execute_WithPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	shop, _ := tenant.FromContext(ctx)

	// Mock : Pagination (page 2, 10 par page)
	mockWithdrawalRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID, 10, 10).
		Return([]*entity.Withdrawal{}, nil)

	response, err := uc.Execute(ctx, 10, 10)

	assert.NoError(t, err)
	// ✅ CORRECTION : Utiliser assert.Empty au lieu de assert.NotNil
	assert.Empty(t, response)
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.GetWithdrawal - Multi-tenant
// ============================================================

func TestListWithdrawalsUsecase_GetWithdrawal_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	// Contexte SANS tenant
	ctx := context.Background()
	withdrawalID := uuid.New().String()

	response, err := uc.GetWithdrawal(ctx, withdrawalID)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.GetWithdrawal - Validation
// ============================================================

func TestListWithdrawalsUsecase_GetWithdrawal_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()

	response, err := uc.GetWithdrawal(ctx, "invalid-uuid")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid withdrawal id")
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.GetWithdrawal - Repository errors
// ============================================================

func TestListWithdrawalsUsecase_GetWithdrawal_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	withdrawalID := uuid.New()

	// Mock : Retrait non trouvé
	mockWithdrawalRepo.EXPECT().
		FindByID(gomock.Any(), withdrawalID).
		Return(nil, errors.New("withdrawal not found"))

	response, err := uc.GetWithdrawal(ctx, withdrawalID.String())

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find withdrawal")
}

func TestListWithdrawalsUsecase_GetWithdrawal_NilWithdrawal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	withdrawalID := uuid.New()

	// Mock : Retrait nil (cas spécial)
	mockWithdrawalRepo.EXPECT().
		FindByID(gomock.Any(), withdrawalID).
		Return(nil, nil)

	response, err := uc.GetWithdrawal(ctx, withdrawalID.String())

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "withdrawal not found")
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.GetWithdrawal - Règles métier
// ============================================================

func TestListWithdrawalsUsecase_GetWithdrawal_ShopMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	withdrawalID := uuid.New()

	// Créer un retrait pour un AUTRE shop
	otherShopID := uuid.New()
	wrongShopWithdrawal := createTestWithdrawal(otherShopID, 50000)

	mockWithdrawalRepo.EXPECT().
		FindByID(gomock.Any(), withdrawalID).
		Return(wrongShopWithdrawal, nil)

	response, err := uc.GetWithdrawal(ctx, withdrawalID.String())

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "does not belong to this shop")
}

// ============================================================
// TESTS : ListWithdrawalsUsecase.GetWithdrawal - Happy path
// ============================================================

func TestListWithdrawalsUsecase_GetWithdrawal_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)

	uc := withdrawalusecase.NewListWithdrawalsUsecase(mockWithdrawalRepo)

	ctx := createTestContextForWithdrawal()
	shop, _ := tenant.FromContext(ctx)
	withdrawalID := uuid.New()

	// Créer un retrait valide
	validWithdrawal := createTestWithdrawal(shop.ID, 50000)
	validWithdrawal.DestinationName = strPtr("John Doe")
	validWithdrawal.DestinationEmail = strPtr("john@example.com")
	validWithdrawal.Description = strPtr("Test withdrawal")

	mockWithdrawalRepo.EXPECT().
		FindByID(gomock.Any(), withdrawalID).
		Return(validWithdrawal, nil)

	response, err := uc.GetWithdrawal(ctx, withdrawalID.String())

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, validWithdrawal.ID.String(), response.ID)
	assert.Equal(t, shop.ID.String(), response.ShopID)
	assert.Equal(t, int64(50000), response.AmountCents)
	assert.Equal(t, "John Doe", response.DestinationName)
	assert.Equal(t, "john@example.com", response.DestinationEmail)
	assert.Equal(t, "Test withdrawal", response.Description)
}
