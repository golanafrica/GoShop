package shopusecase_test

import (
	"context"
	"errors"
	"testing"

	shopusecase "Goshop/application/usecase/shop_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.18 : TESTS UNITAIRES - CONFIGURE TONTINE USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func validTontineRequest(productID, shopID string) *shopusecase.ConfigureTontineRequest {
	return &shopusecase.ConfigureTontineRequest{
		ProductID:             productID,
		ShopID:                shopID,
		IsTontineEnabled:      true,
		AllowCommercialCircle: true,
		AllowCorporateCircle:  false,
		AllowFamilyCircle:     false,
		MinParticipants:       5,
		MaxParticipants:       10,
	}
}

func validTontineSettings(productID, shopID string) *entity.ProductTontineSettings {
	return &entity.ProductTontineSettings{
		ProductID:             productID,
		ShopID:                shopID,
		IsTontineEnabled:      true,
		AllowCommercialCircle: true,
		AllowCorporateCircle:  false,
		AllowFamilyCircle:     false,
		MinParticipants:       5,
		MaxParticipants:       10,
	}
}

// ============================================================
// TESTS : ConfigureTontineRequest.Validate()
// ============================================================

func TestConfigureTontineRequest_Validate_Success(t *testing.T) {
	req := validTontineRequest("product-1", "shop-1")
	err := req.Validate()
	assert.NoError(t, err)
}

func TestConfigureTontineRequest_Validate_EmptyProductID(t *testing.T) {
	req := validTontineRequest("", "shop-1")
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product_id is required")
}

func TestConfigureTontineRequest_Validate_EmptyShopID(t *testing.T) {
	req := validTontineRequest("product-1", "")
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestConfigureTontineRequest_Validate_MinParticipantsTooLow(t *testing.T) {
	req := validTontineRequest("product-1", "shop-1")
	req.MinParticipants = 1 // ❌ < 2
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2")
}

func TestConfigureTontineRequest_Validate_MaxParticipantsTooHigh(t *testing.T) {
	req := validTontineRequest("product-1", "shop-1")
	req.MaxParticipants = 51 // ❌ > 50
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot exceed 50")
}

func TestConfigureTontineRequest_Validate_MinGreaterThanMax(t *testing.T) {
	req := validTontineRequest("product-1", "shop-1")
	req.MinParticipants = 15
	req.MaxParticipants = 10
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot exceed max_participants")
}

func TestConfigureTontineRequest_Validate_NoCircleTypeAllowed(t *testing.T) {
	req := validTontineRequest("product-1", "shop-1")
	req.AllowCommercialCircle = false
	req.AllowCorporateCircle = false
	req.AllowFamilyCircle = false
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one circle type")
}

func TestConfigureTontineRequest_Validate_AllCirclesAllowed(t *testing.T) {
	req := validTontineRequest("product-1", "shop-1")
	req.AllowCommercialCircle = true
	req.AllowCorporateCircle = true
	req.AllowFamilyCircle = true
	err := req.Validate()
	assert.NoError(t, err)
}

// ============================================================
// TESTS : ConfigureTontineUsecase.Execute()
// ============================================================

func TestConfigureTontineUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)

	uc := shopusecase.NewConfigureTontineUsecase(settingsRepo, productRepo)

	req := &shopusecase.ConfigureTontineRequest{
		ProductID: "", // ❌ Vide
		ShopID:    "shop-1",
	}

	result, err := uc.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation error")
}

func TestConfigureTontineUsecase_UpsertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)

	uc := shopusecase.NewConfigureTontineUsecase(settingsRepo, productRepo)

	req := validTontineRequest("product-1", "shop-1")

	settingsRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	result, err := uc.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to upsert settings")
}

func TestConfigureTontineUsecase_FindByProductIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)

	uc := shopusecase.NewConfigureTontineUsecase(settingsRepo, productRepo)

	req := validTontineRequest("product-1", "shop-1")

	settingsRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(nil, errors.New("db error"))

	result, err := uc.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to retrieve updated settings")
}

func TestConfigureTontineUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)

	uc := shopusecase.NewConfigureTontineUsecase(settingsRepo, productRepo)

	req := validTontineRequest("product-1", "shop-1")

	settingsRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, settings *entity.ProductTontineSettings) error {
			assert.Equal(t, req.ProductID, settings.ProductID)
			assert.Equal(t, req.ShopID, settings.ShopID)
			assert.True(t, settings.IsTontineEnabled)
			return nil
		},
	)

	expectedSettings := validTontineSettings(req.ProductID, req.ShopID)
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), req.ProductID).Return(expectedSettings, nil)

	result, err := uc.Execute(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.ProductID, result.ProductID)
	assert.True(t, result.IsTontineEnabled)
	assert.Equal(t, 5, result.MinParticipants)
	assert.Equal(t, 10, result.MaxParticipants)
}

// ============================================================
// TESTS : ConfigureTontineUsecase.GetTontineSettings()
// ============================================================

func TestConfigureTontineUsecase_GetTontineSettings_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)

	uc := shopusecase.NewConfigureTontineUsecase(settingsRepo, productRepo)

	settingsRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(nil, errors.New("not found"))

	result, err := uc.GetTontineSettings(context.Background(), "product-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tontine settings not found")
}

func TestConfigureTontineUsecase_GetTontineSettings_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mockrepo.NewMockProductTontineSettingsRepository(ctrl)
	productRepo := mockrepo.NewMockProductRepository(ctrl)

	uc := shopusecase.NewConfigureTontineUsecase(settingsRepo, productRepo)

	expectedSettings := validTontineSettings("product-1", "shop-1")
	settingsRepo.EXPECT().FindByProductID(gomock.Any(), "product-1").Return(expectedSettings, nil)

	result, err := uc.GetTontineSettings(context.Background(), "product-1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "product-1", result.ProductID)
	assert.True(t, result.IsTontineEnabled)
	assert.True(t, result.AllowCommercialCircle)
	assert.False(t, result.AllowCorporateCircle)
}
