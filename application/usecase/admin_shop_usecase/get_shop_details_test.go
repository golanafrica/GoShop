package adminshopusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adminshopusecase "Goshop/application/usecase/admin_shop_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.11 : TESTS UNITAIRES - GET SHOP DETAILS USECASE
// ============================================================

// ============================================================
// TESTS : GetShopDetailsUsecase - Validation
// ============================================================

func TestGetShopDetailsUsecase_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: "invalid-uuid", // ❌ UUID invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

// ============================================================
// TESTS : GetShopDetailsUsecase - Repository errors
// ============================================================

func TestGetShopDetailsUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Mock : Shop non trouvée
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, errors.New("shop not found"))

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find shop")
}

func TestGetShopDetailsUsecase_ShopNotFound_Nil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Mock : Shop nil
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop not found")
}

// ============================================================
// TESTS : GetShopDetailsUsecase - KYC repo error (non bloquant)
// ============================================================

func TestGetShopDetailsUsecase_KYCRepoError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	testShop := createTestShop("Test Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Mock : KYC repo échoue (non bloquant)
	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return(nil, errors.New("kyc repo error"))

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Shop)
	assert.NotNil(t, response.KYC)
	assert.Empty(t, response.KYC.Documents)
}

// ============================================================
// TESTS : GetShopDetailsUsecase - Happy paths
// ============================================================

func TestGetShopDetailsUsecase_Success_MinimalShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Shop minimal (pas de suspension, pas de notes, pas de review)
	testShop := createTestShop("Minimal Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return([]*entity.ShopKYCDocument{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier les infos de base
	assert.Equal(t, shopID.String(), response.Shop.ID)
	assert.Equal(t, "Minimal Shop", response.Shop.Name)
	assert.Equal(t, "free", response.Shop.Plan)
	assert.True(t, response.Shop.IsActive)

	// Vérifier KYC
	assert.Equal(t, "verified", response.KYC.Status)
	assert.True(t, response.KYC.CanWithdraw)
	assert.Equal(t, 0, response.KYC.DocumentsCount)

	// Vérifier Health
	assert.NotNil(t, response.Health)
	assert.Equal(t, 1000, response.Health.Score)
	assert.Equal(t, "excellent", response.Health.Level)

	// Suspension doit être nil (pas suspendu)
	assert.Nil(t, response.Suspension)

	// Admin notes doit exister mais sans notes
	assert.NotNil(t, response.AdminNotes)
	assert.False(t, response.AdminNotes.HasNotes)

	// Review doit indiquer "needs review" (jamais revu)
	assert.NotNil(t, response.Review)
	assert.True(t, response.Review.NeedsReview)
}

func TestGetShopDetailsUsecase_Success_WithKYCDocuments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	testShop := createTestShop("Shop with KYC", entity.ShopPlanPro, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID

	// Créer 3 documents KYC avec différents statuts
	doc1 := &entity.ShopKYCDocument{
		ID:            uuid.New(),
		ShopID:        shopID,
		DocumentType:  entity.ShopKYCDocIdentityCard,
		FileName:      "id_card.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Status:        "approved",
		CreatedAt:     time.Now(),
	}

	doc2 := &entity.ShopKYCDocument{
		ID:            uuid.New(),
		ShopID:        shopID,
		DocumentType:  entity.ShopKYCDocBusinessRegistry,
		FileName:      "registry.pdf",
		FileSizeBytes: 2048,
		MimeType:      "application/pdf",
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	doc3 := &entity.ShopKYCDocument{
		ID:            uuid.New(),
		ShopID:        shopID,
		DocumentType:  entity.ShopKYCDocTaxCertificate,
		FileName:      "tax.pdf",
		FileSizeBytes: 3072,
		MimeType:      "application/pdf",
		Status:        "rejected",
		CreatedAt:     time.Now(),
	}

	documents := []*entity.ShopKYCDocument{doc1, doc2, doc3}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return(documents, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier les documents KYC
	assert.Equal(t, 3, response.KYC.DocumentsCount)
	assert.Equal(t, 1, response.KYC.DocumentsApproved)
	assert.Equal(t, 1, response.KYC.DocumentsPending)
	assert.Equal(t, 1, response.KYC.DocumentsRejected)
	assert.Len(t, response.KYC.Documents, 3)

	// Vérifier le premier document
	assert.Equal(t, doc1.ID.String(), response.KYC.Documents[0].ID)
	assert.Equal(t, "identity_card", response.KYC.Documents[0].DocumentType)
	assert.Equal(t, "approved", response.KYC.Documents[0].Status)
}

func TestGetShopDetailsUsecase_Success_WithSuspension(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Shop suspendu
	now := time.Now().Add(-48 * time.Hour) // Il y a 2 jours
	adminID := "admin-456"
	reason := "Violation des conditions d'utilisation"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		Slug:             "suspended-shop",
		OwnerID:          "owner-123",
		Plan:             entity.ShopPlanFree,
		IsActive:         false,
		KYCStatus:        entity.ShopKYCStatusVerified,
		HealthScore:      500,
		HealthLevel:      entity.ShopHealthWarning,
		SuspendedAt:      &now,
		SuspendedBy:      &adminID,
		SuspensionReason: &reason,
		CreatedAt:        time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:        time.Now(),
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return([]*entity.ShopKYCDocument{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier la suspension
	assert.NotNil(t, response.Suspension)
	assert.True(t, response.Suspension.IsSuspended)
	assert.Equal(t, adminID, *response.Suspension.SuspendedBy)
	assert.Equal(t, reason, *response.Suspension.Reason)
	assert.GreaterOrEqual(t, response.Suspension.DaysSuspended, 1)
}

func TestGetShopDetailsUsecase_Success_WithAdminNotes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Shop avec notes admin
	notes := "Première note\n\n[2026-07-10 12:00 - admin-123]\nDeuxième note"
	testShop := createTestShop("Shop with Notes", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.AdminNotes = &notes

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return([]*entity.ShopKYCDocument{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier les notes admin
	assert.NotNil(t, response.AdminNotes)
	assert.True(t, response.AdminNotes.HasNotes)
	assert.Equal(t, notes, *response.AdminNotes.Notes)
	assert.GreaterOrEqual(t, response.AdminNotes.NotesCount, 1)
}

func TestGetShopDetailsUsecase_Success_WithReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Shop revue il y a 45 jours (> 30 jours → needs review)
	reviewedAt := time.Now().Add(-45 * 24 * time.Hour)
	reviewedBy := "admin-789"
	testShop := createTestShop("Reviewed Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.LastReviewedAt = &reviewedAt
	testShop.LastReviewedBy = &reviewedBy

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return([]*entity.ShopKYCDocument{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier la review
	assert.NotNil(t, response.Review)
	assert.Equal(t, reviewedBy, *response.Review.LastReviewedBy)
	assert.GreaterOrEqual(t, response.Review.DaysSinceReview, 44)
	assert.True(t, response.Review.NeedsReview) // > 30 jours
}

func TestGetShopDetailsUsecase_Success_WithRecentReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := adminshopusecase.NewGetShopDetailsUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopDetailsRequest{
		ShopID: shopID.String(),
	}

	// Shop revue il y a 10 jours (< 30 jours → pas besoin de review)
	reviewedAt := time.Now().Add(-10 * 24 * time.Hour)
	reviewedBy := "admin-789"
	testShop := createTestShop("Recently Reviewed Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.LastReviewedAt = &reviewedAt
	testShop.LastReviewedBy = &reviewedBy

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockKycRepo.EXPECT().
		FindByShopID(gomock.Any(), shopID).
		Return([]*entity.ShopKYCDocument{}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier la review
	assert.NotNil(t, response.Review)
	assert.LessOrEqual(t, response.Review.DaysSinceReview, 11)
	assert.False(t, response.Review.NeedsReview) // < 30 jours
}
