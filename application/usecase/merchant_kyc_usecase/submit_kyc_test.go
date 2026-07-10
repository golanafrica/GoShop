package merchantkycusecase_test

import (
	"context"
	"errors"
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

// createValidDocuments crée des documents valides pour les tests
func createValidDocuments() []merchantkycusecase.DocumentInput {
	return []merchantkycusecase.DocumentInput{
		{
			DocumentType:  "identity_card",
			FilePath:      "/uploads/id_card.jpg",
			FileName:      "id_card.jpg",
			FileSizeBytes: 1024 * 1024, // 1 MB
			MimeType:      "image/jpeg",
		},
		{
			DocumentType:  "business_registry",
			FilePath:      "/uploads/registry.pdf",
			FileName:      "registry.pdf",
			FileSizeBytes: 2 * 1024 * 1024, // 2 MB
			MimeType:      "application/pdf",
		},
	}
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Multi-tenant
// ============================================================

func TestSubmitMerchantKYCUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	// Contexte SANS tenant
	ctx := context.Background()
	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Règles métier
// ============================================================

func TestSubmitMerchantKYCUsecase_ShopInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	shop.IsActive = false // ❌ Inactif
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop is not active")
}

func TestSubmitMerchantKYCUsecase_KYCAlreadyVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	shop.KYCStatus = entity.ShopKYCStatusVerified // ❌ Déjà vérifié
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, entity.ErrShopKYCAlreadyVerified, err)
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Validation documents
// ============================================================

func TestSubmitMerchantKYCUsecase_NoDocuments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{}, // ❌ Vide
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at least one document is required")
}

func TestSubmitMerchantKYCUsecase_TooManyDocuments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	// 11 documents (> max 10)
	docs := make([]merchantkycusecase.DocumentInput, 11)
	for i := 0; i < 11; i++ {
		docs[i] = merchantkycusecase.DocumentInput{
			DocumentType:  "identity_card",
			FilePath:      "/uploads/doc.jpg",
			FileName:      "doc.jpg",
			FileSizeBytes: 1024,
			MimeType:      "image/jpeg",
		}
	}

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: docs,
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "maximum 10 documents allowed")
}

func TestSubmitMerchantKYCUsecase_InvalidDocumentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "invalid_type", // ❌ Invalide
				FilePath:      "/uploads/doc.jpg",
				FileName:      "doc.jpg",
				FileSizeBytes: 1024,
				MimeType:      "image/jpeg",
			},
		},
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid document type")
}

func TestSubmitMerchantKYCUsecase_FileTooLarge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "identity_card",
				FilePath:      "/uploads/large.jpg",
				FileName:      "large.jpg",
				FileSizeBytes: 6 * 1024 * 1024, // ❌ 6 MB (> max 5 MB)
				MimeType:      "image/jpeg",
			},
			{
				DocumentType:  "business_registry",
				FilePath:      "/uploads/registry.pdf",
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "file size exceeds maximum")
}

func TestSubmitMerchantKYCUsecase_InvalidMimeType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "identity_card",
				FilePath:      "/uploads/doc.exe",
				FileName:      "doc.exe",
				FileSizeBytes: 1024,
				MimeType:      "application/exe", // ❌ Invalide
			},
			{
				DocumentType:  "business_registry",
				FilePath:      "/uploads/registry.pdf",
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "mime type")
}

func TestSubmitMerchantKYCUsecase_MissingIdentityDocument(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	// Seulement business_registry, pas d'identity
	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "business_registry",
				FilePath:      "/uploads/registry.pdf",
				FileName:      "registry.pdf",
				FileSizeBytes: 1024,
				MimeType:      "application/pdf",
			},
		},
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "identity document required")
}

func TestSubmitMerchantKYCUsecase_MissingBusinessRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	// Seulement identity_card, pas de business_registry
	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: []merchantkycusecase.DocumentInput{
			{
				DocumentType:  "identity_card",
				FilePath:      "/uploads/id_card.jpg",
				FileName:      "id_card.jpg",
				FileSizeBytes: 1024,
				MimeType:      "image/jpeg",
			},
		},
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "business registry document required")
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Repository errors
// ============================================================

func TestSubmitMerchantKYCUsecase_UpdateKYCStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	ctx := tenant.WithTenant(context.Background(), shop)

	// Mock : WithTX retourne les mêmes mocks
	mockShopRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockShopRepo).
		AnyTimes()

	mockKycRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockKycRepo).
		AnyTimes()

	// Mock : Create des documents réussit
	mockKycRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	// Mock : UpdateKYCStatus échoue
	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shop.ID, entity.ShopKYCStatusPending, "", nil).
		Return(errors.New("database error"))

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(),
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update kyc status")
}

// ============================================================
// TESTS : SubmitMerchantKYCUsecase - Happy paths
// ============================================================

func TestSubmitMerchantKYCUsecase_Success_FirstSubmission(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	shop.KYCSubmissionsCount = 0 // Première soumission
	ctx := tenant.WithTenant(context.Background(), shop)

	// Mock : WithTX retourne les mêmes mocks
	mockShopRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockShopRepo).
		AnyTimes()

	mockKycRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockKycRepo).
		AnyTimes()

	// Mock : Create des documents réussit (2 documents)
	mockKycRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	// Mock : UpdateKYCStatus réussit
	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shop.ID, entity.ShopKYCStatusPending, "", nil).
		Return(nil)

	// Mock : FindByID pour récupérer le shop mis à jour
	now := time.Now()
	updatedShop := createActiveShopForSubmit()
	updatedShop.ID = shop.ID
	updatedShop.KYCStatus = entity.ShopKYCStatusPending
	updatedShop.KYCSubmittedAt = &now
	updatedShop.KYCSubmissionsCount = 1

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(updatedShop, nil)

	// Mock : CountByShopID pour compter les documents
	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shop.ID).
		Return(2, nil)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(),
	}

	response, err := uc.Execute(ctx, req)

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewSubmitMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	shop := createActiveShopForSubmit()
	shop.KYCStatus = entity.ShopKYCStatusRejected // Rejeté précédemment
	shop.KYCSubmissionsCount = 1                  // Re-soumission
	ctx := tenant.WithTenant(context.Background(), shop)

	// Mock : WithTX retourne les mêmes mocks
	mockShopRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockShopRepo).
		AnyTimes()

	mockKycRepo.EXPECT().
		WithTX(gomock.Any()).
		Return(mockKycRepo).
		AnyTimes()

	// Mock : DeleteByShopID pour supprimer les anciens documents
	mockKycRepo.EXPECT().
		DeleteByShopID(gomock.Any(), shop.ID).
		Return(nil)

	// Mock : Create des nouveaux documents (2 documents)
	mockKycRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)

	// Mock : UpdateKYCStatus réussit
	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shop.ID, entity.ShopKYCStatusPending, "", nil).
		Return(nil)

	// Mock : FindByID pour récupérer le shop mis à jour
	now := time.Now()
	updatedShop := createActiveShopForSubmit()
	updatedShop.ID = shop.ID
	updatedShop.KYCStatus = entity.ShopKYCStatusPending
	updatedShop.KYCSubmittedAt = &now
	updatedShop.KYCSubmissionsCount = 2

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shop.ID).
		Return(updatedShop, nil)

	// Mock : CountByShopID pour compter les documents
	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shop.ID).
		Return(2, nil)

	req := &merchantkycusecase.SubmitMerchantKYCRequest{
		Documents: createValidDocuments(),
	}

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "pending", response.KYCStatus)
	assert.Equal(t, 2, response.KYCSubmissionsCount)
	assert.Equal(t, 2, response.DocumentsCount)
}

// ============================================================
// TEST : Helper strPtr (si pas déjà défini)
// ============================================================

func strPtrSubmit(s string) *string {
	return &s
}

// ============================================================
// TEST : Vérification interface ShopRepository
// ============================================================

func TestShopRepositoryInterface_WithTX(t *testing.T) {
	// Vérifie que ShopRepository implémente l'interface WithTX
	var _ repository.ShopRepository = (*mockrepo.MockShopRepository)(nil)
}
