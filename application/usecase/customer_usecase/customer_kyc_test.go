package customerusecase_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
// 🆕 v4.4.19 : TESTS UNITAIRES - CUSTOMER KYC USECASES
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// setupTempUploads crée un répertoire uploads temporaire
func setupTempUploads(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}

	originalWd, _ := os.Getwd()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}

	t.Cleanup(func() {
		os.Chdir(originalWd)
	})

	return uploadsDir
}

// createTestFile crée un fichier de test dans le répertoire uploads
// Retourne le chemin au format cross-platform pour la validation
// createTestFile crée un fichier de test dans le répertoire uploads
func createTestFile(t *testing.T, filename string) string {
	t.Helper()
	// Créer le fichier sur disque avec le chemin OS-compatible
	diskPath := filepath.Join("uploads", filename)
	content := []byte("test file content for KYC testing")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file %s: %v", diskPath, err)
	}
	// Retourner avec forward slashes (format attendu par le usecase)
	return "uploads/" + filename
}

func createTestShopForKYC() *entity.Shop {
	return &entity.Shop{
		ID:       uuid.New(),
		Name:     "Test Shop",
		OwnerID:  "merchant-1",
		IsActive: true,
	}
}

func createTestCustomerKYC(customerID string) *entity.Customer {
	return &entity.Customer{
		ID:        customerID,
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@mail.com",
		KYCLevel:  entity.KYCLevelPending,
	}
}

func createTestKYCDocument(id, customerID string, status entity.KYCDocumentStatus) *entity.CustomerKYCDocument {
	return &entity.CustomerKYCDocument{
		ID:           id,
		CustomerID:   customerID,
		DocumentType: entity.KYCDocumentCNI,
		FilePath:     "/uploads/kyc/test.jpg",
		Status:       status,
		CreatedAt:    time.Now(),
	}
}

// createValidUploadToken crée un token valide pour les tests
func createValidUploadToken(userID, filePath string) *entity.UploadToken {
	token, _ := entity.NewUploadToken(userID, filePath, "test.jpg", 1024, "image/jpeg")
	return token
}

// ============================================================
// TESTS : ReviewKYCRequest.Validate()
// ============================================================

func TestReviewKYCRequest_Validate_Success_Approve(t *testing.T) {
	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}
	err := req.Validate()
	assert.NoError(t, err)
}

func TestReviewKYCRequest_Validate_Success_Reject(t *testing.T) {
	req := &customerusecase.ReviewKYCRequest{
		CustomerID:      "cust-1",
		Action:          customerusecase.ReviewKYCReject,
		RejectionReason: "Document flou",
	}
	err := req.Validate()
	assert.NoError(t, err)
}

func TestReviewKYCRequest_Validate_EmptyCustomerID(t *testing.T) {
	req := &customerusecase.ReviewKYCRequest{
		Action: customerusecase.ReviewKYCApprove,
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestReviewKYCRequest_Validate_InvalidAction(t *testing.T) {
	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     "invalid",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "action must be")
}

func TestReviewKYCRequest_Validate_RejectWithoutReason(t *testing.T) {
	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCReject,
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rejection_reason is required")
}

// ============================================================
// TESTS : ReviewKYCUsecase.Execute()
// ============================================================

func TestReviewKYCUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := customerusecase.NewReviewKYCUsecase(nil, nil, nil, nil)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestReviewKYCUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	uc := customerusecase.NewReviewKYCUsecase(nil, nil, nil, nil)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation error")
}

func TestReviewKYCUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	notifService := mockservice.NewMockNotificationService(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReviewKYCUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
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

	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(nil, errors.New("not found"))

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "customer not found")
}

func TestReviewKYCUsecase_NoPendingDocuments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
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

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{}, nil)

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no pending KYC documents")
}

func TestReviewKYCUsecase_Success_Approve(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
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

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	pendingDoc := createTestKYCDocument("doc-1", "cust-1", entity.KYCDocumentStatusPending)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	kycDocRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(customer, nil)

	updatedDocs := []*entity.CustomerKYCDocument{pendingDoc}
	kycDocRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(updatedDocs, nil)

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID: "cust-1",
		Action:     customerusecase.ReviewKYCApprove,
	}

	result, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Documents))
}

func TestReviewKYCUsecase_Success_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
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

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	pendingDoc := createTestKYCDocument("doc-1", "cust-1", entity.KYCDocumentStatusPending)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	kycDocRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(customer, nil)

	updatedDocs := []*entity.CustomerKYCDocument{pendingDoc}
	kycDocRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(updatedDocs, nil)

	uc := customerusecase.NewReviewKYCUsecase(customerRepo, kycDocRepo, notifService, txManager)

	req := &customerusecase.ReviewKYCRequest{
		CustomerID:      "cust-1",
		Action:          customerusecase.ReviewKYCReject,
		RejectionReason: "Document flou",
	}

	result, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// ============================================================
// TESTS : ListPendingKYCUsecase.Execute()
// ============================================================

func TestListPendingKYCUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := customerusecase.NewListPendingKYCUsecase(nil, nil)

	result, err := uc.Execute(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestListPendingKYCUsecase_Success_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	kycDocRepo.EXPECT().FindPendingByShop(gomock.Any(), shop.ID.String()).Return([]*entity.CustomerKYCDocument{}, nil)

	uc := customerusecase.NewListPendingKYCUsecase(kycDocRepo, customerRepo)

	result, err := uc.Execute(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestListPendingKYCUsecase_Success_WithPendingDocs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	pendingDoc := createTestKYCDocument("doc-1", "cust-1", entity.KYCDocumentStatusPending)
	kycDocRepo.EXPECT().FindPendingByShop(gomock.Any(), shop.ID.String()).Return([]*entity.CustomerKYCDocument{pendingDoc}, nil)

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	uc := customerusecase.NewListPendingKYCUsecase(kycDocRepo, customerRepo)

	result, err := uc.Execute(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, "cust-1", result[0].Customer.ID)
}

// ============================================================
// TESTS : UploadKYCRequest.Validate()
// ============================================================

func TestUploadKYCRequest_Validate_Success(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "/uploads/test.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token-123",
	}
	err := req.Validate()
	assert.NoError(t, err)
}

func TestUploadKYCRequest_Validate_EmptyCustomerID(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "/uploads/test.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestUploadKYCRequest_Validate_InvalidDocumentType(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  "invalid",
		FilePath:      "/uploads/test.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid document type")
}

func TestUploadKYCRequest_Validate_EmptyFilePath(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file_path is required")
}

func TestUploadKYCRequest_Validate_FileTooLarge(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "/uploads/test.jpg",
		FileSizeBytes: 10 * 1024 * 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestUploadKYCRequest_Validate_InvalidMimeType(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "/uploads/test.exe",
		FileSizeBytes: 1024,
		MimeType:      "application/exe",
		Token:         "test-token",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mime type not allowed")
}

func TestUploadKYCRequest_Validate_EmptyToken(t *testing.T) {
	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "/uploads/test.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "",
	}
	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upload token is required")
}

// ============================================================
// TESTS : UploadKYCDocumentUsecase.Execute()
// ============================================================

func TestUploadKYCDocumentUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := customerusecase.NewUploadKYCDocumentUsecase(nil, nil, nil, nil)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      "/uploads/test.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token",
	}

	result, err := uc.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestUploadKYCDocumentUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")

	shop := createTestShopForKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)

	mockTx := mockrepo.NewMockTx(ctrl)

	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)
	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	txManager := mockrepo.NewMockTxManager(ctrl)
	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	validToken := createValidUploadToken(authUserID, testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "test-token-123").Return(validToken, nil)
	tokenRepo.EXPECT().MarkUsed(gomock.Any(), "test-token-123").Return(nil)

	txManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()
	mockTx.EXPECT().Commit().Return(nil)

	kycDocRepo.EXPECT().WithTX(mockTx).Return(kycDocRepo).AnyTimes()
	customerRepo.EXPECT().WithTX(mockTx).Return(customerRepo).AnyTimes()

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(0, nil)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{}, nil)

	kycDocRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	customerRepo.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(customer, nil)

	uc := customerusecase.NewUploadKYCDocumentUsecase(kycDocRepo, customerRepo, txManager, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "test-token-123",
	}

	result, err := uc.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUploadKYCDocumentUsecase_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")
	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	tokenRepo.EXPECT().FindByID(gomock.Any(), "invalid-token").Return(nil, errors.New("not found"))

	uc := customerusecase.NewUploadKYCDocumentUsecase(nil, nil, nil, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "invalid-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid upload token")
}

func TestUploadKYCDocumentUsecase_TokenExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")
	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	expiredToken := createValidUploadToken("user-1", testFilePath)
	expiredToken.Used = true
	tokenRepo.EXPECT().FindByID(gomock.Any(), "expired-token").Return(expiredToken, nil)

	uc := customerusecase.NewUploadKYCDocumentUsecase(nil, nil, nil, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "expired-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "expired or already used")
}

func TestUploadKYCDocumentUsecase_TokenWrongUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")
	shop := createTestShopForKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)

	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	wrongUserToken := createValidUploadToken("different-user", testFilePath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "wrong-token").Return(wrongUserToken, nil)

	uc := customerusecase.NewUploadKYCDocumentUsecase(nil, nil, nil, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "wrong-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not belong to authenticated user")
}

func TestUploadKYCDocumentUsecase_TokenFilePathMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")
	differentPath := createTestFile(t, "different.jpg")
	shop := createTestShopForKYC()
	authUserID := "auth-user-1"
	ctx := tenant.WithTenant(context.Background(), shop)
	ctx = utils.WithUserID(ctx, authUserID)

	tokenRepo := mockrepo.NewMockUploadTokenRepository(ctrl)

	wrongPathToken := createValidUploadToken(authUserID, differentPath)
	tokenRepo.EXPECT().FindByID(gomock.Any(), "mismatch-token").Return(wrongPathToken, nil)

	uc := customerusecase.NewUploadKYCDocumentUsecase(nil, nil, nil, tokenRepo)

	req := &customerusecase.UploadKYCRequest{
		CustomerID:    "cust-1",
		DocumentType:  entity.KYCDocumentCNI,
		FilePath:      testFilePath,
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Token:         "mismatch-token",
	}

	result, err := uc.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "file path does not match")
}

func TestUploadKYCDocumentUsecase_MaxDocumentsReached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")
	shop := createTestShopForKYC()
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

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(3, nil)

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
	assert.Contains(t, err.Error(), "maximum number of documents")
}

func TestUploadKYCDocumentUsecase_DuplicatePendingDocument(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setupTempUploads(t)
	testFilePath := createTestFile(t, "test.jpg")
	shop := createTestShopForKYC()
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

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	kycDocRepo.EXPECT().CountByCustomer(gomock.Any(), "cust-1").Return(1, nil)

	existingDoc := createTestKYCDocument("doc-1", "cust-1", entity.KYCDocumentStatusPending)
	kycDocRepo.EXPECT().FindPendingByCustomer(gomock.Any(), "cust-1").Return([]*entity.CustomerKYCDocument{existingDoc}, nil)

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
	assert.Contains(t, err.Error(), "already pending review")
}

// ============================================================
// TESTS : GetKYCStatusUsecase.Execute()
// ============================================================

func TestGetKYCStatusUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := customerusecase.NewGetKYCStatusUsecase(nil, nil)

	result, err := uc.Execute(context.Background(), "cust-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "multi-tenant")
}

func TestGetKYCStatusUsecase_CustomerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)

	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(nil, errors.New("not found"))

	uc := customerusecase.NewGetKYCStatusUsecase(customerRepo, kycDocRepo)

	result, err := uc.Execute(ctx, "cust-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "customer not found")
}

func TestGetKYCStatusUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shop := createTestShopForKYC()
	ctx := tenant.WithTenant(context.Background(), shop)

	customerRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	kycDocRepo := mockrepo.NewMockCustomerKYCRepository(ctrl)

	customer := createTestCustomerKYC("cust-1")
	customerRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(customer, nil)

	docs := []*entity.CustomerKYCDocument{
		createTestKYCDocument("doc-1", "cust-1", entity.KYCDocumentStatusApproved),
	}
	kycDocRepo.EXPECT().FindByCustomerID(gomock.Any(), "cust-1").Return(docs, nil)

	uc := customerusecase.NewGetKYCStatusUsecase(customerRepo, kycDocRepo)

	result, err := uc.Execute(ctx, "cust-1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "cust-1", result.CustomerID)
	assert.Len(t, result.Documents, 1)
}

// ============================================================
// TESTS : GenerateKYCFilePath
// ============================================================

func TestGenerateKYCFilePath_JPEG(t *testing.T) {
	path := customerusecase.GenerateKYCFilePath("cust-1", entity.KYCDocumentCNI, "image/jpeg")
	assert.Contains(t, path, "/uploads/kyc/cust-1/")
	assert.Contains(t, path, "cni_")
	assert.Contains(t, path, ".jpg")
}

func TestGenerateKYCFilePath_PNG(t *testing.T) {
	path := customerusecase.GenerateKYCFilePath("cust-1", entity.KYCDocumentPassport, "image/png")
	assert.Contains(t, path, ".png")
}

func TestGenerateKYCFilePath_PDF(t *testing.T) {
	path := customerusecase.GenerateKYCFilePath("cust-1", entity.KYCDocumentOther, "application/pdf")
	assert.Contains(t, path, ".pdf")
}
