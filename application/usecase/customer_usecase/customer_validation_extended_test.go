package customerusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"
	mockrepo "Goshop/mocks/repository"
	mockservice "Goshop/mocks/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.20 : TESTS COMPLÉMENTAIRES - VALIDATION & UPLOAD KYC
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestFile crée un fichier de test dans le répertoire uploads

func createTestShopForCustomerKYC() *entity.Shop {
	return &entity.Shop{
		ID:       uuid.New(),
		Name:     "Test Shop",
		OwnerID:  "merchant-1",
		IsActive: true,
	}
}

func createTestCustomerForKYC(customerID string) *entity.Customer {
	return &entity.Customer{
		ID:        customerID,
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@mail.com",
		KYCLevel:  entity.KYCLevelNone,
	}
}

// ============================================================
// TESTS : UploadKYCDocumentUsecase.Execute - Branches d'erreur
// ============================================================

func TestUploadKYCDocumentUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to start transaction")
}

func TestUploadKYCDocumentUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)
	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(nil, errors.New("not found"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "customer not found")
}

func TestUploadKYCDocumentUsecase_CountByCustomerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)
	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(0, errors.New("db error"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to count documents")
}

func TestUploadKYCDocumentUsecase_FindPendingByCustomerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)
	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(0, nil)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return(nil, errors.New("db error"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to check pending documents")
}

func TestUploadKYCDocumentUsecase_CreateDocumentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)
	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(0, nil)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{}, nil)

	kycDocRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to save document")
}

func TestUploadKYCDocumentUsecase_UpdateCustomerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)
	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(0, nil)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{}, nil)

	kycDocRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update customer KYC status")
}

func TestUploadKYCDocumentUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForCustomerKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)
	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token").Return(validToken, nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(0, nil)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{}, nil)

	kycDocRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(customer, nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

// ============================================================
// TESTS : ReviewKYCUsecase.Execute - Branches d'erreur
// ============================================================

func TestReviewKYCUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForCustomerKYC()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()
	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	pendingDoc := &entity.CustomerKYCDocument{
		ID:           "doc-1",
		CustomerID:   "cust-1",
		DocumentType: entity.KYCDocumentCNI,
		Status:       entity.KYCDocumentStatusPending,
		CreatedAt:    time.Now(),
	}
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	kycDocRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(customer, nil)

	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestReviewKYCUsecase_UpdateDocumentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForCustomerKYC()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()
	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	pendingDoc := &entity.CustomerKYCDocument{
		ID:           "doc-1",
		CustomerID:   "cust-1",
		DocumentType: entity.KYCDocumentCNI,
		Status:       entity.KYCDocumentStatusPending,
		CreatedAt:    time.Now(),
	}
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	kycDocRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update document")
}

func TestReviewKYCUsecase_UpdateCustomerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForCustomerKYC()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()
	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	pendingDoc := &entity.CustomerKYCDocument{
		ID:           "doc-1",
		CustomerID:   "cust-1",
		DocumentType: entity.KYCDocumentCNI,
		Status:       entity.KYCDocumentStatusPending,
		CreatedAt:    time.Now(),
	}
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	kycDocRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update customer KYC status")
}

func TestReviewKYCUsecase_FindByCustomerIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForCustomerKYC()
	ctx := tenant.WithTenant(context.Background(), shop)
	mockTx := mockrepo.NewMockTx(ctrl)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()
	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()

	customer := createTestCustomerForKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	pendingDoc := &entity.CustomerKYCDocument{
		ID:           "doc-1",
		CustomerID:   "cust-1",
		DocumentType: entity.KYCDocumentCNI,
		Status:       entity.KYCDocumentStatusPending,
		CreatedAt:    time.Now(),
	}
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	kycDocRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(customer, nil)

	// Erreur lors de la récupération finale des documents
	kycDocRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(nil, errors.New("db error"))

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch updated documents")
}
