package merchantkycusecase_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	merchantkycusecase "Goshop/application/usecase/merchant_kyc_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.13 : TESTS UNITAIRES - SUBMIT MERCHANT KYC USECASE
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
func createTestFile(t *testing.T, filename string) string {
	t.Helper()
	diskPath := filepath.Join("uploads", filename)
	content := []byte("test file content for KYC testing")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file %s: %v", diskPath, err)
	}
	return "uploads/" + filename
}

// createActiveShopForSubmit crée un shop actif pour les tests submit
func createActiveShopForSubmit() *entity.Shop {
	return &entity.Shop{
		ID:                  uuid.New(),
		Name:                "Active Shop",
		Slug:                "active-shop",
		OwnerID:             "owner-123",
		Plan:                entity.ShopPlanFree,
		IsActive:            true,
		KYCStatus:           entity.ShopKYCStatusUnverified,
		KYCSubmissionsCount: 0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// createValidDocuments crée des documents valides avec fichiers réels
// IMPORTANT: setupTempUploads(t) doit être appelé AVANT cette fonction
func createValidDocuments(t *testing.T) []merchantkycusecase.DocumentInput {
	t.Helper()
	return []merchantkycusecase.DocumentInput{
		{
			DocumentType:  "identity_card",
			FilePath:      createTestFile(t, "id_card.jpg"),
			FileName:      "id_card.jpg",
			FileSizeBytes: 1024 * 1024,
			MimeType:      "image/jpeg",
		},
		{
			DocumentType:  "business_registry",
			FilePath:      createTestFile(t, "registry.pdf"),
			FileName:      "registry.pdf",
			FileSizeBytes: 2 * 1024 * 1024,
			MimeType:      "application/pdf",
		},
	}
}

// testContext contient tous les mocks nécessaires pour les tests
type testContext struct {
	ctrl      *gomock.Controller
	shopRepo  *mockrepo.MockShopRepository
	kycRepo   *mockrepo.MockShopKYCDocumentRepository
	txManager *mockrepo.MockTxManager
	tx        *mockrepo.MockTx
	uc        *merchantkycusecase.SubmitMerchantKYCUsecase
}

// setupTest initialise tous les mocks et le usecase
func setupTest(t *testing.T, transactional bool) *testContext {
	ctrl := gomock.NewController(t)

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)

	if transactional {
		mockTxManager.EXPECT().
			BeginTx(gomock.Any()).
			Return(mockTx, nil).
			AnyTimes()

		mockTx.EXPECT().
			Rollback().
			Return(nil).
			AnyTimes()
	}

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(
		mockShopRepo,
		mockKycRepo,
		mockTxManager,
	)

	return &testContext{
		ctrl:      ctrl,
		shopRepo:  mockShopRepo,
		kycRepo:   mockKycRepo,
		txManager: mockTxManager,
		tx:        mockTx,
		uc:        uc,
	}
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Multi-tenant
// ============================================================

func TestSubmitMerchantKYCUsecase_MultiTenantError(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	ctx := context.Background()
	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(t),
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Règles métier
// ============================================================

func TestSubmitMerchantKYCUsecase_ShopInactive(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	shop.IsActive = false
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(t),
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop is not active")
}

func TestSubmitMerchantKYCUsecase_KYCAlreadyVerified(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	shop.KYCStatus = entity.ShopKYCStatusVerified
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(t),
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrShopKYCAlreadyVerified, err)
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Validation documents
// ============================================================

func TestSubmitMerchantKYCUsecase_NoDocuments(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{},
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at least one document is required")
}

func TestSubmitMerchantKYCUsecase_TooManyDocuments(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	docs := make([]merchantkycusecase.DocumentInput, 11)
	for i := 0; i < 11; i++ {
		docs[i] = merchantkycusecase.DocumentInput{
			DocumentType:  "identity_card",
			FilePath:      createTestFile(t, "doc.jpg"),
			FileName:      "doc.jpg",
			FileSizeBytes: 1024,
			MimeType:      "image/jpeg",
		}
	}

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: docs,
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "maximum 10 documents allowed")
}

func TestSubmitMerchantKYCUsecase_InvalidDocumentType(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "invalid_type",
				FilePath:      createTestFile(t, "doc.jpg"),
				FileName:      "doc.jpg",
				FileSizeBytes: 1024,
				MimeType:      "image/jpeg",
			},
			{
				DocumentType:  "business_registry",
				FilePath:      createTestFile(t, "registry.pdf"),
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid document type")
}

func TestSubmitMerchantKYCUsecase_FileTooLarge(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "identity_card",
				FilePath:      createTestFile(t, "large.jpg"),
				FileName:      "large.jpg",
				FileSizeBytes: 6 * 1024 * 1024,
				MimeType:      "image/jpeg",
			},
			{
				DocumentType:  "business_registry",
				FilePath:      createTestFile(t, "registry.pdf"),
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "file size exceeds maximum")
}

func TestSubmitMerchantKYCUsecase_InvalidMimeType(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "identity_card",
				FilePath:      createTestFile(t, "doc.exe"),
				FileName:      "doc.exe",
				FileSizeBytes: 1024,
				MimeType:      "application/exe",
			},
			{
				DocumentType:  "business_registry",
				FilePath:      createTestFile(t, "registry.pdf"),
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "mime type")
}

func TestSubmitMerchantKYCUsecase_MissingIdentityDocument(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "business_registry",
				FilePath:      createTestFile(t, "registry.pdf"),
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "identity document required")
}

func TestSubmitMerchantKYCUsecase_MissingBusinessRegistry(t *testing.T) {
	tc := setupTest(t, false)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "identity_card",
				FilePath:      createTestFile(t, "id_card.jpg"),
				FileName:      "id_card.jpg",
				FileSizeBytes: 1024,
				MimeType:      "image/jpeg",
			},
		},
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "business registry document required")
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Repository errors (avec transaction)
// ============================================================

func TestSubmitMerchantKYCUsecase_UpdateKYCStatusError(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	tc.shopRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(tc.shopRepo).
		AnyTimes()

	tc.kycRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(tc.kycRepo).
		AnyTimes()

	tc.kycRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	tc.shopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shop.ID, entity.ShopKYCStatusPending, "", nil).
		Return(errors.New("database error"))

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(t),
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update kyc status")
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Happy paths (avec transaction)
// ============================================================

func TestSubmitMerchantKYCUsecase_Success_FirstSubmission(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	shop.KYCSubmissionsCount = 0
	ctx := tenant.WithTenant(context.Background(), shop)

	tc.tx.EXPECT().Rollback().Return(nil).Times(0)
	tc.tx.EXPECT().Commit().Return(nil).Times(1)

	tc.shopRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(tc.shopRepo).
		AnyTimes()

	tc.kycRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(tc.kycRepo).
		AnyTimes()

	tc.kycRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	tc.shopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shop.ID, entity.ShopKYCStatusPending, "", nil).
		Return(nil)

	now := time.Now()
	updatedShop := createActiveShopForSubmit()
	updatedShop.ID = shop.ID
	updatedShop.KYCStatus = entity.ShopKYCStatusPending
	updatedShop.KYCSubmittedAt = &now
	updatedShop.KYCSubmissionsCount = 1

	tc.shopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(updatedShop, nil)

	tc.kycRepo.EXPECT().
		CountByShopID(gomock.Any(), shop.ID).
		Return(2, nil)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(t),
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, shop.ID.String(), response.ShopID)
	assert.Equal(t, "Active Shop", response.ShopName)
	assert.Equal(t, "pending", response.KYCStatus)
	assert.NotNil(t, response.KYCSubmittedAt)
	assert.Equal(t, 1, response.KYCSubmissionsCount)
	assert.Equal(t, 2, response.DocumentsCount)
	assert.Contains(t, response.Message, "succès")
}

func TestSubmitMerchantKYCUsecase_Success_Resubmission(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.ctrl.Finish()

	setupTempUploads(t)
	shop := createActiveShopForSubmit()
	shop.KYCStatus = entity.ShopKYCStatusRejected
	shop.KYCSubmissionsCount = 1
	ctx := tenant.WithTenant(context.Background(), shop)

	tc.tx.EXPECT().Rollback().Return(nil).Times(0)
	tc.tx.EXPECT().Commit().Return(nil).Times(1)

	tc.shopRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(tc.shopRepo).
		AnyTimes()

	tc.kycRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(tc.kycRepo).
		AnyTimes()

	tc.kycRepo.EXPECT().
		DeleteByShopID(gomock.Any(), shop.ID).
		Return(nil)

	tc.kycRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	tc.shopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shop.ID, entity.ShopKYCStatusPending, "", nil).
		Return(nil)

	now := time.Now()
	updatedShop := createActiveShopForSubmit()
	updatedShop.ID = shop.ID
	updatedShop.KYCStatus = entity.ShopKYCStatusPending
	updatedShop.KYCSubmittedAt = &now
	updatedShop.KYCSubmissionsCount = 2

	tc.shopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(updatedShop, nil)

	tc.kycRepo.EXPECT().
		CountByShopID(gomock.Any(), shop.ID).
		Return(2, nil)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(t),
	}

	response, err := tc.uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "pending", response.KYCStatus)
	assert.Equal(t, 2, response.KYCSubmissionsCount)
	assert.Equal(t, 2, response.DocumentsCount)
}

func strPtrSubmit(s string) *string {
	return &s
}

func TestShopRepositoryInterface_WithTX(t *testing.T) {
	var _ repository.ShopRepository = (*mockrepo.MockShopRepository)(nil)
}
