package merchantkycusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	merchantkycusecase "Goshop/application/usecase/merchant_kyc_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.13 : TESTS UNITAIRES - GET MERCHANT KYC STATUS USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestShopForKYC crée un shop de test avec statut KYC
func createTestShopForKYC(kycStatus entity.ShopKYCStatus) *entity.Shop {
	shop := &entity.Shop{
		ID:        uuid.New(),
		Name:      "Test Shop",
		Slug:      "test-shop",
		OwnerID:   "owner-123",
		Plan:      entity.ShopPlanFree,
		IsActive:  true,
		KYCStatus: kycStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return shop
}

// createTestContextWithShop crée un contexte avec un shop
func createTestContextWithShop(shop *entity.Shop) context.Context {
	ctx := context.Background()
	return tenant.WithTenant(ctx, shop)
}

// createTestKYCDocument crée un document KYC de test
func createTestKYCDocument(shopID uuid.UUID, docType entity.ShopKYCDocumentType, status string) *entity.ShopKYCDocument {
	return &entity.ShopKYCDocument{
		ID:            uuid.New(),
		ShopID:        shopID,
		DocumentType:  docType,
		FilePath:      "/uploads/test.pdf",
		FileName:      "test.pdf",
		FileSizeBytes: 1024,
		MimeType:      "application/pdf",
		Status:        status,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// ============================================================
// TESTS : GetMerchantKYCStatusUsecase - Multi-tenant
// ============================================================

func TestGetMerchantKYCStatusUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewGetMerchantKYCStatusUsecase(mockShopRepo, mockKycRepo)

	// Contexte SANS tenant
	ctx := context.Background()

	response, err := uc.Execute(ctx)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : GetMerchantKYCStatusUsecase - Repository errors
// ============================================================

func TestGetMerchantKYCStatusUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewGetMerchantKYCStatusUsecase(mockShopRepo, mockKycRepo)

	shop := createTestShopForKYC(entity.ShopKYCStatusPending)
	ctx := createTestContextWithShop(shop)

	// Mock : FindByShopID échoue
	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID).
		Return(nil, errors.New("database error"))

	response, err := uc.Execute(ctx)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find documents")
}

// ============================================================
// TESTS : GetMerchantKYCStatusUsecase - Happy paths
// ============================================================

func TestGetMerchantKYCStatusUsecase_Success_NoDocuments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewGetMerchantKYCStatusUsecase(mockShopRepo, mockKycRepo)

	shop := createTestShopForKYC(entity.ShopKYCStatusUnverified)
	ctx := createTestContextWithShop(shop)

	// Mock : Liste vide
	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID).
		Return([]*entity.ShopKYCDocument{}, nil)

	response, err := uc.Execute(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, shop.ID.String(), response.ShopID)
	assert.Equal(t, "Test Shop", response.ShopName)
	assert.Equal(t, "unverified", response.KYCStatus)
	assert.False(t, response.CanWithdraw)
	assert.Empty(t, response.Documents)
	assert.Equal(t, 0, response.DocumentsSummary.Total)
	assert.Equal(t, 0, response.DocumentsSummary.Pending)
	assert.Equal(t, 0, response.DocumentsSummary.Approved)
	assert.Equal(t, 0, response.DocumentsSummary.Rejected)
}

func TestGetMerchantKYCStatusUsecase_Success_WithDocuments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewGetMerchantKYCStatusUsecase(mockShopRepo, mockKycRepo)

	shop := createTestShopForKYC(entity.ShopKYCStatusPending)
	ctx := createTestContextWithShop(shop)

	// Créer 3 documents avec différents statuts
	doc1 := createTestKYCDocument(shop.ID, entity.ShopKYCDocIdentityCard, "approved")
	doc2 := createTestKYCDocument(shop.ID, entity.ShopKYCDocBusinessRegistry, "pending")
	doc3 := createTestKYCDocument(shop.ID, entity.ShopKYCDocTaxCertificate, "rejected")

	documents := []*entity.ShopKYCDocument{doc1, doc2, doc3}

	// Mock : Liste avec 3 documents
	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID).
		Return(documents, nil)

	response, err := uc.Execute(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "pending", response.KYCStatus)
	assert.False(t, response.CanWithdraw) // ❌ Pas encore vérifié
	assert.Len(t, response.Documents, 3)

	// Vérifier le résumé
	assert.Equal(t, 3, response.DocumentsSummary.Total)
	assert.Equal(t, 1, response.DocumentsSummary.Pending)
	assert.Equal(t, 1, response.DocumentsSummary.Approved)
	assert.Equal(t, 1, response.DocumentsSummary.Rejected)

	// Vérifier le premier document
	assert.Equal(t, doc1.ID.String(), response.Documents[0].ID)
	assert.Equal(t, "identity_card", response.Documents[0].DocumentType)
	assert.Equal(t, "approved", response.Documents[0].Status)
}

func TestGetMerchantKYCStatusUsecase_Success_VerifiedShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewGetMerchantKYCStatusUsecase(mockShopRepo, mockKycRepo)

	// Shop vérifié avec tous les champs KYC
	now := time.Now()
	adminID := "admin-123"
	shop := createTestShopForKYC(entity.ShopKYCStatusVerified)
	shop.KYCSubmittedAt = &now
	shop.KYCVerifiedAt = &now
	shop.KYCVerifiedBy = &adminID
	shop.KYCSubmissionsCount = 2

	ctx := createTestContextWithShop(shop)

	// Mock : 2 documents approuvés
	doc1 := createTestKYCDocument(shop.ID, entity.ShopKYCDocIdentityCard, "approved")
	doc2 := createTestKYCDocument(shop.ID, entity.ShopKYCDocBusinessRegistry, "approved")

	documents := []*entity.ShopKYCDocument{doc1, doc2}

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shop.ID).
		Return(documents, nil)

	response, err := uc.Execute(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "verified", response.KYCStatus)
	assert.True(t, response.CanWithdraw) // ✅ Vérifié = peut retirer
	assert.NotNil(t, response.KYCSubmittedAt)
	assert.NotNil(t, response.KYCVerifiedAt)
	// ❌ SUPPRIMER : assert.NotNil(t, response.KYCVerifiedBy)
	// ❌ SUPPRIMER : assert.Equal(t, adminID, *response.KYCVerifiedBy)
	assert.Equal(t, 2, response.KYCSubmissionsCount)
	assert.Len(t, response.Documents, 2)

	// Vérifier le résumé
	assert.Equal(t, 2, response.DocumentsSummary.Total)
	assert.Equal(t, 0, response.DocumentsSummary.Pending)
	assert.Equal(t, 2, response.DocumentsSummary.Approved)
	assert.Equal(t, 0, response.DocumentsSummary.Rejected)
}
