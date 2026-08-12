package withdrawalusecase_test

import (
	"context"
	"errors"
	"testing"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	walletusecase "Goshop/application/usecase/wallet_usecase"
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

func createValidWithdrawalRequest() *withdrawaldto.CreateWithdrawalRequest {
	return &withdrawaldto.CreateWithdrawalRequest{
		AmountCents:       50000,
		PaymentMethod:     "ORANGE_MONEY",
		DestinationNumber: "+22670123456",
	}
}

func mockCashOutProviderFactory(mockProvider *mockusecase.MockCashOutProvider) withdrawalusecase.YengaPayProviderFactory {
	return func(config payment.YengaPayConfig) (withdrawalusecase.CashOutProvider, error) {
		return mockProvider, nil
	}
}

func mockCashOutProviderFactoryWithError(err error) withdrawalusecase.YengaPayProviderFactory {
	return func(config payment.YengaPayConfig) (withdrawalusecase.CashOutProvider, error) {
		return nil, err
	}
}

// Helper mis à jour : FindByShopID + HeldCents pour le pré-check available
func createMockDebitWalletUsecase(ctrl *gomock.Controller) (*walletusecase.DebitWalletUsecase, *mockrepo.MockMerchantWalletRepository) {
	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	mockTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockWalletRepo.EXPECT().WithTX(mockTx).Return(mockWalletRepo).AnyTimes()

	// Phase 1 : pré-check available (FindByShopID hors TX)
	mockWalletRepo.EXPECT().FindByShopID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
			return &entity.MerchantWallet{
				ShopID:                  shopID,
				BalanceCents:            1_000_000,
				HeldCents:               0,
				IsFrozen:                false,
				MaxNegativeBalanceCents: -500_000,
			}, nil
		},
	).AnyTimes()

	// Débit réel sous FOR UPDATE
	mockWalletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
			return &entity.MerchantWallet{
				ShopID:                  shopID,
				BalanceCents:            1_000_000,
				HeldCents:               0,
				IsFrozen:                false,
				MaxNegativeBalanceCents: -500_000,
			}, nil
		},
	).AnyTimes()

	mockWalletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockTxnRepo.EXPECT().WithTX(mockTx).Return(mockTxnRepo).AnyTimes()
	mockTxnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil).AnyTimes()
	mockTx.EXPECT().Rollback().AnyTimes()

	return walletusecase.NewDebitWalletUsecase(mockWalletRepo, mockTxnRepo, mockTxManager), mockWalletRepo
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Multi-tenant
// ============================================================

func TestCreateWithdrawalUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
	)

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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	req := &withdrawaldto.CreateWithdrawalRequest{
		AmountCents:       0,
		PaymentMethod:     "ORANGE_MONEY",
		DestinationNumber: "+22670123456",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

// ============================================================
// TESTS : CreateWithdrawalUsecase - Repository errors
// ============================================================

func TestCreateWithdrawalUsecase_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)

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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactoryWithError(errors.New("invalid config")),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

	mockCashOut.EXPECT().
		CashOut(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("insufficient funds"))

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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
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

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

func TestCreateWithdrawalUsecase_ShopSettingsDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{
				Enabled: false,
			},
		}, nil)

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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCashOut := mockusecase.NewMockCashOutProvider(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecaseWithFactory(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockCashOutProviderFactory(mockCashOut),
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
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
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
	)

	assert.NotNil(t, uc)
}

func TestNewCreateWithdrawalUsecase_UsesDefaultFactory_ConfigMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWithdrawalRepo := mockrepo.NewMockWithdrawalRepository(ctrl)
	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockDebitWalletUC, mockWalletRepo := createMockDebitWalletUsecase(ctrl)

	uc := withdrawalusecase.NewCreateWithdrawalUsecase(
		mockWithdrawalRepo,
		mockShopRepo,
		mockWalletRepo,
		mockRegistry,
		mockDebitWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	mockWithdrawalRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	mockShopRepo.EXPECT().
		GetPaymentSettings(gomock.Any(), shop.ID).
		Return(&entity.ShopPaymentSettings{
			YengaPay: entity.YengaPayShopSettings{Enabled: false},
		}, nil)

	mockWithdrawalRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidWithdrawalRequest()
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "create yenga provider")
}
