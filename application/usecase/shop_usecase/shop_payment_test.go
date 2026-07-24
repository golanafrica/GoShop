package shopusecase_test

import (
	"context"
	"errors"
	"testing"

	shopdto "Goshop/application/dto/shop_dto"
	shopusecase "Goshop/application/usecase/shop_usecase"
	"Goshop/domain/entity"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.5.2 : TESTS UNITAIRES - CONFIGURE PAYMENT USECASE (Marketplace Centralisé)
// ============================================================

func validPaymentRequest(shopID string) *shopdto.UpdatePaymentSettingsRequest {
	codEnabled := true
	rate := 250
	return &shopdto.UpdatePaymentSettingsRequest{
		ShopID:                shopID,
		CashOnDeliveryEnabled: &codEnabled,
		CashCommissionRate:    &rate,
	}
}

func validPaymentSettings(shopID uuid.UUID) *entity.ShopPaymentSettings {
	codEnabled := true
	rate := 250
	yengaEnabled := true
	return &entity.ShopPaymentSettings{
		ShopID:                shopID,
		CashOnDeliveryEnabled: codEnabled,
		CashCommissionRate:    rate,
		YengaPay: entity.YengaPayShopSettings{
			Enabled:   yengaEnabled,
			Operators: []string{"orange_money", "moov_money"},
		},
	}
}

// ============================================================
// TESTS : UpdatePaymentSettingsRequest.Validate()
// ============================================================

func TestUpdatePaymentSettingsRequest_Validate_Success(t *testing.T) {
	req := validPaymentRequest(uuid.New().String())
	err := req.Validate()
	assert.NoError(t, err)
}

func TestUpdatePaymentSettingsRequest_Validate_EmptyShopID(t *testing.T) {
	req := validPaymentRequest("")
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestUpdatePaymentSettingsRequest_Validate_InvalidShopID(t *testing.T) {
	req := validPaymentRequest("invalid-uuid")
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

func TestUpdatePaymentSettingsRequest_Validate_InvalidOperator(t *testing.T) {
	shopID := uuid.New().String()
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID:            shopID,
		YengaPayOperators: []string{"invalid_operator"},
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid operator")
}

func TestUpdatePaymentSettingsRequest_Validate_InvalidCommissionRate(t *testing.T) {
	shopID := uuid.New().String()
	invalidRate := 15000 // > 10000
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID:             shopID,
		CashCommissionRate: &invalidRate,
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cash_commission_rate must be between 0 and 10000")
}

// ============================================================
// TESTS : ConfigurePaymentUsecase.Execute()
// ============================================================

func TestConfigurePaymentUsecase_Execute_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	req := validPaymentRequest("invalid-uuid")
	userID := uuid.New()

	result, err := uc.Execute(context.Background(), req, userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid shop_id")
}

func TestConfigurePaymentUsecase_Execute_IsOwnerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()
	req := validPaymentRequest(shopID.String())

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(false, errors.New("db error"))

	result, err := uc.Execute(context.Background(), req, userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "check ownership")
}

func TestConfigurePaymentUsecase_Execute_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()
	req := validPaymentRequest(shopID.String())

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(false, nil)

	result, err := uc.Execute(context.Background(), req, userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not the owner")
}

func TestConfigurePaymentUsecase_Execute_GetSettingsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()
	req := validPaymentRequest(shopID.String())

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(true, nil)

	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(nil, errors.New("db error"))

	result, err := uc.Execute(context.Background(), req, userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get current settings")
}

func TestConfigurePaymentUsecase_Execute_UpsertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()
	req := validPaymentRequest(shopID.String())

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(true, nil)

	settings := validPaymentSettings(shopID)
	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(settings, nil)

	paymentRepo.EXPECT().
		UpsertPaymentSettings(gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	result, err := uc.Execute(context.Background(), req, userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "save settings")
}

func TestConfigurePaymentUsecase_Execute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()
	req := validPaymentRequest(shopID.String())

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(true, nil)

	settings := validPaymentSettings(shopID)
	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(settings, nil)

	paymentRepo.EXPECT().
		UpsertPaymentSettings(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, s *entity.ShopPaymentSettings) error {
			assert.True(t, s.CashOnDeliveryEnabled)
			assert.Equal(t, 250, s.CashCommissionRate)
			return nil
		},
	)

	result, err := uc.Execute(context.Background(), req, userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shopID.String(), result.ShopID)
	assert.True(t, result.CashOnDeliveryEnabled)
	assert.Equal(t, 250, result.CashCommissionRate)
}

func TestConfigurePaymentUsecase_Execute_WithYengaPay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()
	enabled := true
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID:            shopID.String(),
		YengaPayEnabled:   &enabled,
		YengaPayOperators: []string{"orange_money", "moov_money"},
	}

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(true, nil)

	settings := validPaymentSettings(shopID)
	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(settings, nil)

	paymentRepo.EXPECT().
		UpsertPaymentSettings(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, s *entity.ShopPaymentSettings) error {
			assert.True(t, s.YengaPay.Enabled)
			assert.Len(t, s.YengaPay.Operators, 2)
			return nil
		},
	)

	result, err := uc.Execute(context.Background(), req, userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.YengaPayEnabled)
	assert.Len(t, result.YengaPayOperators, 2)
}

// ============================================================
// TESTS : ConfigurePaymentUsecase.GetPaymentSettings()
// ============================================================

func TestConfigurePaymentUsecase_GetPaymentSettings_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	userID := uuid.New()
	result, err := uc.GetPaymentSettings(context.Background(), "invalid-uuid", userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid shop_id")
}

func TestConfigurePaymentUsecase_GetPaymentSettings_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(false, nil)

	result, err := uc.GetPaymentSettings(context.Background(), shopID.String(), userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not the owner")
}

func TestConfigurePaymentUsecase_GetPaymentSettings_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(true, nil)

	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(nil, errors.New("db error"))

	result, err := uc.GetPaymentSettings(context.Background(), shopID.String(), userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get settings")
}

func TestConfigurePaymentUsecase_GetPaymentSettings_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	paymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	ownerVerifier := mockusecase.NewMockShopOwnerVerifier(ctrl)

	uc := shopusecase.NewConfigurePaymentUsecase(nil, paymentRepo, ownerVerifier)

	shopID := uuid.New()
	userID := uuid.New()

	ownerVerifier.EXPECT().
		IsOwner(gomock.Any(), shopID, userID).
		Return(true, nil)

	settings := validPaymentSettings(shopID)
	settings.CashOnDeliveryEnabled = true
	settings.CashCommissionRate = 250
	settings.YengaPay = entity.YengaPayShopSettings{
		Enabled:   true,
		Operators: []string{"orange_money"},
	}
	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(settings, nil)

	result, err := uc.GetPaymentSettings(context.Background(), shopID.String(), userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shopID.String(), result.ShopID)
	assert.True(t, result.CashOnDeliveryEnabled)
	assert.Equal(t, 250, result.CashCommissionRate)
	assert.True(t, result.YengaPayEnabled)
	assert.Len(t, result.YengaPayOperators, 1)
}
