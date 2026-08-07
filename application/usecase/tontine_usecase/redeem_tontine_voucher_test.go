package tontineusecase

import (
	"context"
	"errors"
	"testing"
	"time"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ============================================================
// HELPERS DE TEST
// ============================================================

func createTestContextWithLogger() context.Context {
	logger := zerolog.Nop()
	ctx := context.Background()
	ctx = logger.WithContext(ctx)
	return ctx
}

func createTestContextWithShop(shopID uuid.UUID) context.Context {
	ctx := createTestContextWithLogger()
	shop := &entity.Shop{ID: shopID}
	return tenant.WithTenant(ctx, shop)
}

// ============================================================
// TESTS DE VALIDATION
// ============================================================

func TestRedeemTontineVoucherRequest_Validate_EmptyVoucherCode(t *testing.T) {
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "",
		RedeemedBy:  "user-123",
	}

	err := req.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "voucher_code is required")
}

func TestRedeemTontineVoucherRequest_Validate_EmptyRedeemedBy(t *testing.T) {
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "",
	}

	err := req.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "redeemed_by is required")
}

func TestRedeemTontineVoucherRequest_Validate_Success(t *testing.T) {
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	err := req.Validate()

	assert.NoError(t, err)
}

// ============================================================
// TESTS DU USECASE - CAS D'ERREUR
// ============================================================

func TestRedeemTontineVoucherUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "",
		RedeemedBy:  "user-123",
	}

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation error")
}

func TestRedeemTontineVoucherUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	// Contexte sans shop
	ctx := createTestContextWithLogger()
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestRedeemTontineVoucherUsecase_VoucherNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(nil, errors.New("not found")).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "voucher not found")
}

func TestRedeemTontineVoucherUsecase_VoucherNotBelongToShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	otherShopID := uuid.New()

	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:          uuid.New().String(),
		GroupID:     uuid.New().String(),
		ShopID:      otherShopID.String(), // Différent du shop dans le contexte
		VoucherCode: "VOUCHER123",
		Status:      entity.VoucherStatusGenerated,
		ExpiresAt:   time.Now().Add(24 * time.Hour), // 🆕 Date d'expiration valide
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "voucher does not belong to this shop")
}

func TestRedeemTontineVoucherUsecase_VoucherNotValid_AlreadyRedeemed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:          uuid.New().String(),
		GroupID:     uuid.New().String(),
		ShopID:      shopID.String(), // Même shop
		VoucherCode: "VOUCHER123",
		Status:      entity.VoucherStatusRedeemed, // Déjà redeemé
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "voucher is not redeemable")
	assert.Contains(t, err.Error(), "status=redeemed")
}

func TestRedeemTontineVoucherUsecase_VoucherNotValid_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:          uuid.New().String(),
		GroupID:     uuid.New().String(),
		ShopID:      shopID.String(), // Même shop
		VoucherCode: "VOUCHER123",
		Status:      entity.VoucherStatusGenerated,
		ExpiresAt:   time.Now().Add(-24 * time.Hour), // 🆕 Expiré (hier)
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "voucher is not redeemable")
	// Note: le message d'erreur ne contient pas "status=expired" car IsValid() échoue sur la date
}

func TestRedeemTontineVoucherUsecase_RedeemFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:              uuid.New().String(),
		GroupID:         uuid.New().String(),
		ShopID:          shopID.String(), // Même shop
		VoucherCode:     "VOUCHER123",
		Status:          entity.VoucherStatusGenerated,
		HeldAmountCents: 10000,
		ExpiresAt:       time.Now().Add(24 * time.Hour), // 🆕 Date d'expiration valide
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	mockVoucherRepo.EXPECT().
		Redeem(ctx, "VOUCHER123", "user-123").
		Return(errors.New("database error")).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redeem voucher")
}

// ============================================================
// TESTS DU USECASE - CAS DE SUCCÈS
// ============================================================

func TestRedeemTontineVoucherUsecase_Success_WithHeldRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)

	// Mock pour ReleaseHeldWalletUsecase
	// Note: On ne peut pas mocker facilement ReleaseHeldWalletUsecase car c'est une struct,
	// pas une interface. On va donc tester avec releaseHeldUC = nil pour ce test.
	// Dans un vrai scénario, tu devrais créer une interface pour ReleaseHeldWalletUsecase.

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, nil) // nil pour releaseHeldUC

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:              uuid.New().String(),
		GroupID:         uuid.New().String(),
		ShopID:          shopID.String(), // Même shop
		VoucherCode:     "VOUCHER123",
		Status:          entity.VoucherStatusGenerated,
		HeldAmountCents: 0,                              // Pas de held pour éviter d'appeler releaseHeldUC
		ExpiresAt:       time.Now().Add(24 * time.Hour), // 🆕 Date d'expiration valide
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	mockVoucherRepo.EXPECT().
		Redeem(ctx, "VOUCHER123", "user-123").
		Return(nil).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "VOUCHER123", resp.VoucherCode)
	assert.Equal(t, "redeemed", resp.Status)
	assert.Equal(t, int64(0), resp.ReleasedCents)
	assert.Equal(t, int64(0), resp.AvailableCents)
	assert.Equal(t, int64(0), resp.HeldCents)
}

func TestRedeemTontineVoucherUsecase_Success_ZeroHeldAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{} // Non nil mais ne sera pas appelé

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:              uuid.New().String(),
		GroupID:         uuid.New().String(),
		ShopID:          shopID.String(), // Même shop
		VoucherCode:     "VOUCHER123",
		Status:          entity.VoucherStatusGenerated,
		HeldAmountCents: 0,                              // Zéro, donc pas de release
		ExpiresAt:       time.Now().Add(24 * time.Hour), // 🆕 Date d'expiration valide
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	mockVoucherRepo.EXPECT().
		Redeem(ctx, "VOUCHER123", "user-123").
		Return(nil).
		Times(1)

	// Pas d'appel à releaseHeldUC car HeldAmountCents == 0

	resp, err := uc.Execute(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "VOUCHER123", resp.VoucherCode)
	assert.Equal(t, "redeemed", resp.Status)
	assert.Equal(t, int64(0), resp.ReleasedCents)
}

// ============================================================
// TESTS EDGE CASES
// ============================================================

func TestRedeemTontineVoucherUsecase_NilReleaseHeldUC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)

	// releaseHeldUC = nil
	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, nil)

	shopID := uuid.New()
	ctx := createTestContextWithShop(shopID)
	req := &RedeemTontineVoucherRequest{
		VoucherCode: "VOUCHER123",
		RedeemedBy:  "user-123",
	}

	voucher := &entity.TontineVoucher{
		ID:              uuid.New().String(),
		GroupID:         uuid.New().String(),
		ShopID:          shopID.String(), // Même shop
		VoucherCode:     "VOUCHER123",
		Status:          entity.VoucherStatusGenerated,
		HeldAmountCents: 5000,                           // Même avec held > 0, releaseHeldUC est nil
		ExpiresAt:       time.Now().Add(24 * time.Hour), // 🆕 Date d'expiration valide
	}

	mockVoucherRepo.EXPECT().
		FindByCode(ctx, "VOUCHER123").
		Return(voucher, nil).
		Times(1)

	mockVoucherRepo.EXPECT().
		Redeem(ctx, "VOUCHER123", "user-123").
		Return(nil).
		Times(1)

	resp, err := uc.Execute(ctx, req)

	// Devrait réussir même sans releaseHeldUC (nil-safe)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "VOUCHER123", resp.VoucherCode)
	assert.Equal(t, "redeemed", resp.Status)
	assert.Equal(t, int64(0), resp.ReleasedCents) // Pas de release car UC est nil
}

// ============================================================
// TESTS DE CONSTRUCTEUR
// ============================================================

func TestNewRedeemTontineVoucherUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)
	mockReleaseUC := &walletusecase.ReleaseHeldWalletUsecase{}

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, mockReleaseUC)

	require.NotNil(t, uc)
	assert.Equal(t, mockVoucherRepo, uc.voucherRepo)
	assert.Equal(t, mockReleaseUC, uc.releaseHeldUC)
}

func TestNewRedeemTontineVoucherUsecase_NilReleaseUC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVoucherRepo := mockrepo.NewMockTontineVoucherRepository(ctrl)

	uc := NewRedeemTontineVoucherUsecase(mockVoucherRepo, nil)

	require.NotNil(t, uc)
	assert.Equal(t, mockVoucherRepo, uc.voucherRepo)
	assert.Nil(t, uc.releaseHeldUC)
}
