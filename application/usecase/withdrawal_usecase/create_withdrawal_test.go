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

// createMockDebitWalletUsecase : FindByShopID + FOR UPDATE pour pré-check + débit
func createMockDebitWalletUsecase(ctrl *gomock.Controller) (*walletusecase.DebitWalletUsecase, *mockrepo.MockMerchantWalletRepository) {
	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	mockTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockWalletRepo.EXPECT().WithTX(mockTx).Return(mockWalletRepo).AnyTimes()

	mockWalletRepo.EXPECT().FindByShopID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
			return &entity.MerchantWallet{
				ShopID:                  shopID,
				BalanceCents:            1_000_000,
				HeldCents:               0,
				DebtCents:               0,
				IsFrozen:                false,
				MaxNegativeBalanceCents: -500_000,
			}, nil
		},
	).AnyTimes()

	mockWalletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
			return &entity.MerchantWallet{
				ShopID:                  shopID,
				BalanceCents:            1_000_000,
				HeldCents:               0,
				DebtCents:               0,
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

// creditWalletUC nil : reverse non testé (log CRITICAL si cash-out fail).
// Pour un test P0-A reverse, injecter un vrai CreditWalletUsecase mocké.
var nilCreditWalletUC *walletusecase.CreditWalletUsecase = nil

// ============================================================
// Multi-tenant
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
		nilCreditWalletUC,
	)

	ctx := context.Background()
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// KYC (messages FR du usecase)
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
		nilCreditWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusUnverified, nil)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "KYC")
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
		nilCreditWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusPending, nil)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "KYC")
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
		nilCreditWalletUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusRejected, nil)
	req := createValidWithdrawalRequest()

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "rejete")
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
		nilCreditWalletUC,
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
// Entity / validation
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
		nilCreditWalletUC,
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
	assert.Contains(t, err.Error(), "positif")
}

// ============================================================
// Repository errors
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
// Provider errors (P0-A : reverse tenté si creditUC non nil)
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
	// Message actuel (failAfterDebit + reverse impossible car creditUC nil)
	assert.Contains(t, err.Error(), "cash-out definitively rejected by provider")
	assert.Contains(t, err.Error(), "insufficient funds")
	assert.Contains(t, err.Error(), "compensation pending")
}

// ============================================================
// Shop settings
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
// Happy paths
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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
		nilCreditWalletUC,
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

// ============================================================
// P0-A : Tests de la logique de Compensation (Reverse)
// ============================================================

// createMockCreditWalletUsecase : Helper pour mocker le usecase de crédit
// createMockCreditWalletUsecase : Helper pour mocker le usecase de crédit
func createMockCreditWalletUsecase(ctrl *gomock.Controller, shouldFail bool) *walletusecase.CreditWalletUsecase {
	mockWalletRepo := mockrepo.NewMockMerchantWalletRepository(ctrl)
	mockTxnRepo := mockrepo.NewMockWalletTransactionRepository(ctrl)
	mockEscrowRepo := mockrepo.NewMockEscrowAccountRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil).AnyTimes()
	mockWalletRepo.EXPECT().WithTX(mockTx).Return(mockWalletRepo).AnyTimes()
	mockTxnRepo.EXPECT().WithTX(mockTx).Return(mockTxnRepo).AnyTimes()

	// 1. Verrouillage FOR UPDATE
	mockWalletRepo.EXPECT().FindByShopIDForUpdate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
			return &entity.MerchantWallet{
				ShopID:                  shopID,
				BalanceCents:            1_000_000,
				HeldCents:               0,
				DebtCents:               0,
				IsFrozen:                false,
				MaxNegativeBalanceCents: -500_000,
			}, nil
		},
	).AnyTimes()

	// 🆕 2. Vérification d'idempotence (le reverse ne doit pas créditer 2 fois)
	// On retourne (nil, nil) pour simuler qu'aucune transaction n'existe encore pour cette référence.
	mockTxnRepo.EXPECT().FindByReferenceID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	if shouldFail {
		mockWalletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("database locked")).AnyTimes()
	} else {
		mockWalletRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		mockTxnRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	}

	mockTx.EXPECT().Commit().Return(nil).AnyTimes()
	mockTx.EXPECT().Rollback().AnyTimes()

	return walletusecase.NewCreditWalletUsecase(mockWalletRepo, mockTxnRepo, mockEscrowRepo, mockTxManager)
}

// TestCreateWithdrawalUsecase_ReversePayoutDebit_Success vérifie que le reverse fonctionne
func TestCreateWithdrawalUsecase_ReversePayoutDebit_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	creditUC := createMockCreditWalletUsecase(ctrl, false)

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
		creditUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	amount := int64(50000)
	withdrawalID := "550e8400-e29b-41d4-a716-446655440000"

	err := uc.ReversePayoutDebitForTest(ctx, shop.ID.String(), amount, withdrawalID, "test reason")
	assert.NoError(t, err)
}

// TestCreateWithdrawalUsecase_ReversePayoutDebit_Fails_ReturnsError vérifie la propagation de l'erreur
func TestCreateWithdrawalUsecase_ReversePayoutDebit_Fails_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	creditUC := createMockCreditWalletUsecase(ctrl, true) // true = simule l'échec

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
		creditUC,
	)

	ctx := createTestContextForWithdrawalWithKYC(entity.ShopKYCStatusVerified, nil)
	shop, _ := tenant.FromContext(ctx)

	amount := int64(50000)
	withdrawalID := "550e8400-e29b-41d4-a716-446655440000"

	err := uc.ReversePayoutDebitForTest(ctx, shop.ID.String(), amount, withdrawalID, "test reason")

	// On s'attend à ce que l'erreur soit propagée pour déclencher "compensation_pending"
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credit reverse failed")
}
