package withdrawalusecase_test

import (
	"context"
	"errors"
	"testing"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	withdrawalusecase "Goshop/application/usecase/withdrawal_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.10 : TESTS UNITAIRES - CREATE WITHDRAWAL USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestContextForWithdrawalWithKYC crée un contexte tenant avec KYC spécifique
func createTestContextForWithdrawalWithKYC(kycStatus entity.ShopKYCStatus, rejectionReason *string) context.Context {
	ctx := context.Background()
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:                 shopID,
		Name:               "Test Shop",
		KYCStatus:          kycStatus,
		KYCRejectionReason: rejectionReason,
	}
	return tenant.WithTenant(ctx, shop)
}

// createValidWithdrawalRequest crée une requête de retrait valide
func createValidWithdrawalRequest() *withdrawaldto.CreateWithdrawalRequest {
	return &withdrawaldto.CreateWithdrawalRequest{
		AmountCents:       50000,
		PaymentMethod:     "ORANGE_MONEY",
		DestinationNumber: "+22670123456",
	}
}

// mockCashOutProviderFactory crée une factory qui retourne le mock fourni
func mockCashOutProviderFactory(mockProvider *mockusecase.MockCashOutProvider) withdrawalusecase.YengaPayProviderFactory {
	return func(config payment.YengaPayConfig) (withdrawalusecase.CashOutProvider, error) {
		return mockProvider, nil
	}
}

// mockCashOutProviderFactoryWithError crée une factory qui échoue
func mockCashOutProviderFactoryWithError(err error) withdrawalusecase.YengaPayProviderFactory {
	return func(config payment.YengaPayConfig) (withdrawalusecase.CashOutProvider, error) {
		return nil, err
	}
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Multi-tenant
// ============================================================

func TestCreateWithdrawalUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	// Contexte SANS tenant
	ctx := context.Background()
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - KYC Verification
// ============================================================

func TestCreateWithdrawalUsecase_KYC_Unverified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusUnverified, nil)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "KYC verification required")
	assert.Contains(t, err.Error(), "unverified")
}

func TestCreateWithdrawalUsecase_KYC_Pending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusPending, nil)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "under review")
	assert.Contains(t, err.Error(), "pending")
}

func TestCreateWithdrawalUsecase_KYC_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusRejected, nil)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "rejected")
}

func TestCreateWithdrawalUsecase_KYC_RejectedWithReason(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	reason := "Document illisible"
	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusRejected, &reason)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "Document illisible")
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Entity Creation Error
// ============================================================

func TestCreateWithdrawalUsecase_EntityCreationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	req := &withdrawaldto.CreateWithdrawalRequest{
		AmountCents:       0, // ❌ Montant invalide
		PaymentMethod:     "ORANGE_MONEY",
		DestinationNumber: "+22670123456",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "create withdrawal entity")
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Repository errors
// ============================================================

func TestCreateWithdrawalUsecase_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)

	// Mock : Create échoue
	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "save withdrawal")
}

func TestCreateWithdrawalUsecase_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-123",
			Status:      "PROCESSING",
			Amount:      500,
			Fees:        5,
		}, nil)

	// Mock : Update échoue
	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update withdrawal")
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Provider errors
// ============================================================

func TestCreateWithdrawalUsecase_ProviderFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactoryWithError(errors.New("invalid config")),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	// Mock : Update pour marquer comme failed
	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "create yenga provider")
}

func TestCreateWithdrawalUsecase_CashOutError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	// Mock : CashOut échoue
	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("insufficient funds"))

	// Mock : Update pour marquer comme failed
	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cash-out with Yenga Pay")
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Shop Settings
// ============================================================

func TestCreateWithdrawalUsecase_ShopSettingsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : GetPaymentSettings échoue (fallback global)
	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(nil, errors.New("settings not found"))

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-GLOBAL",
			Status:      "PROCESSING",
			Amount:      500,
			Fees:        5,
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	// ✅ Erreur ignorée, fallback vers config globale
	assert.NoError(t, err)
	assert.NotNil(t, response)
}

func TestCreateWithdrawalUsecase_ShopSettingsDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Mock : YengaPay désactivé (fallback global)
	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false, // ❌ Désactivé
			},
		}, nil)

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-GLOBAL",
			Status:      "PROCESSING",
			Amount:      500,
			Fees:        5,
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Happy paths
// ============================================================

func TestCreateWithdrawalUsecase_Success_Processing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-123",
			Status:      "PROCESSING",
			Amount:      500,
			Fees:        5,
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "YENGA-REF-123", response.ProviderRef)
	assert.Equal(t, "processing", response.Status)
	assert.Equal(t, int64(50000), response.AmountCents)
}

func TestCreateWithdrawalUsecase_Success_ImmediateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	// ✅ Mock : Réponse SUCCESS immédiate
	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-SUCCESS",
			Status:      "SUCCESS",
			Amount:      500,
			Fees:        5,
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "success", response.Status)
}

func TestCreateWithdrawalUsecase_Success_WithOptionalFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-123",
			Status:      "PROCESSING",
			Amount:      500,
			Fees:        5,
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	req.DestinationName = "John Doe"
	req.DestinationEmail = "john@example.com"
	req.Description = "Test withdrawal"

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "John Doe", response.DestinationName)
	assert.Equal(t, "john@example.com", response.DestinationEmail)
	assert.Equal(t, "Test withdrawal", response.Description)
}

func TestCreateWithdrawalUsecase_Success_ShopSpecificConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopPaymentRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// ✅ Mock : Config boutique activée
	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled:        true,
				APIKey:         "shop-api-key",
				OrganizationID: "shop-org",
				ProjectID:      "shop-project",
				WebhookSecret:  "shop-secret",
				Env:            "production",
			},
		}, nil)

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(&payment.CashOutResponse{
			ProviderRef: "YENGA-REF-SHOP",
			Status:      "PROCESSING",
			Amount:      500,
			Fees:        5,
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "YENGA-REF-SHOP", response.ProviderRef)
}

func TestNewCreateWithdrawalUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(mockWithdrawalRepo, mockShopPaymentRepo, mockRegistry)

	assert.NotNil(t, uc)
}

func TestNewCreateWithdrawalUsecase_UsesDefaultFactory_ConfigMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopPaymentRepo := mockusecase.NewMockShopPaymentSettingsRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ⚠️ Usecase construit avec le VRAI defaultYengaPayFactory (pas de mock de factory)
	uc := withdrawalusecase.NewCreateWithdrawalUsecase(mockWithdrawalRepo, mockShopPaymentRepo, mockRegistry)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	// Shop settings désactivés -> fallback vers config globale (vide dans l'environnement de test)
	mockShopPaymentRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{Enabled: false},
		}, nil)

	// Comme la factory va échouer (config vide), le usecase marque le withdrawal en failed
	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	// La vraie factory a été appelée et a échoué faute de config -> aucun appel réseau n'a eu lieu
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "create yenga provider")
}
