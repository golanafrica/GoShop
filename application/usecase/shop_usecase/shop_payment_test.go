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
// 🆕 v4.4.18 : TESTS UNITAIRES - CONFIGURE PAYMENT USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func validPaymentRequest(shopID string) *shopdto.UpdatePaymentSettingsRequest {
	orangeEnabled := true
	return &shopdto.UpdatePaymentSettingsRequest{
		ShopID:             shopID,
		OrangeMoneyEnabled: &orangeEnabled,
	}
}

func validPaymentSettings(shopID uuid.UUID) *entity.ShopPaymentSettings {
	return &entity.ShopPaymentSettings{
		ShopID:                shopID,
		OrangeMoney:           false,
		MoovMoney:             false,
		Wave:                  false,
		CashOnDeliveryEnabled: true,
		CashCommissionRate:    250,
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
	enabled := true
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID: shopID,
		YengaPay: &shopdto.YengaPaySettingsDTO{
			Enabled:   &enabled,
			Operators: []string{"invalid_operator"},
		},
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid operator")
}

func TestUpdatePaymentSettingsRequest_Validate_InvalidEnv(t *testing.T) {
	shopID := uuid.New().String()
	enabled := true
	invalidEnv := "staging"
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID: shopID,
		YengaPay: &shopdto.YengaPaySettingsDTO{
			Enabled: &enabled,
			Env:     &invalidEnv,
		},
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "env must be 'test' or 'prod'")
}

func TestUpdatePaymentSettingsRequest_Validate_ValidEnv(t *testing.T) {
	shopID := uuid.New().String()
	enabled := true
	env := "prod"
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID: shopID,
		YengaPay: &shopdto.YengaPaySettingsDTO{
			Enabled: &enabled,
			Env:     &env,
		},
	}
	err := req.Validate()
	assert.NoError(t, err)
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
			assert.True(t, s.OrangeMoney)
			return nil
		},
	)

	result, err := uc.Execute(context.Background(), req, userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shopID.String(), result.ShopID)
	assert.True(t, result.OrangeMoneyEnabled)
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
	apiKey := "test-api-key"
	env := "prod"
	req := &shopdto.UpdatePaymentSettingsRequest{
		ShopID: shopID.String(),
		YengaPay: &shopdto.YengaPaySettingsDTO{
			Enabled:   &enabled,
			APIKey:    &apiKey,
			Env:       &env,
			Operators: []string{"orange_money", "moov_money"},
		},
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
			assert.Equal(t, "test-api-key", s.YengaPay.APIKey)
			assert.Equal(t, "prod", s.YengaPay.Env)
			assert.Len(t, s.YengaPay.Operators, 2)
			return nil
		},
	)

	result, err := uc.Execute(context.Background(), req, userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.YengaPay.Enabled)
	assert.True(t, result.YengaPay.HasAPIKey)
	assert.Equal(t, "prod", result.YengaPay.Env)
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
	settings.OrangeMoney = true
	settings.YengaPay = entity.YengaPayShopSettings{
		Enabled:        true,
		APIKey:         "test-key",
		OrganizationID: "org-123",
		ProjectID:      "proj-456",
		WebhookSecret:  "secret",
		Operators:      []string{"orange_money"},
		Env:            "prod",
	}
	paymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shopID).
		Return(settings, nil)

	result, err := uc.GetPaymentSettings(context.Background(), shopID.String(), userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shopID.String(), result.ShopID)
	assert.True(t, result.OrangeMoneyEnabled)
	assert.True(t, result.YengaPay.Enabled)
	assert.True(t, result.YengaPay.HasAPIKey)
	assert.True(t, result.YengaPay.HasOrgID)
	assert.True(t, result.YengaPay.HasProjectID)
	assert.True(t, result.YengaPay.HasWebhook)
	assert.False(t, result.YengaPay.IsUsingGlobal)
}
