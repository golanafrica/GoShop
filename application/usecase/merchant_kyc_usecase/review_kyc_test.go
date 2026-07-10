package merchantkycusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	merchantkycusecase "Goshop/application/usecase/merchant_kyc_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.13 : TESTS UNITAIRES - REVIEW MERCHANT KYC USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createTestPendingShop crée un shop en statut pending
func createTestPendingShop() *entity.Shop {
	now := time.Now()
	return &entity.Shop{
		ID:                  uuid.New(),
		Name:                "Pending Shop",
		Slug:                "pending-shop",
		OwnerID:             "owner-123",
		Plan:                entity.ShopPlanFree,
		IsActive:            true,
		KYCStatus:           entity.ShopKYCStatusPending,
		KYCSubmittedAt:      &now,
		KYCSubmissionsCount: 1,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// ============================================================
// TESTS : ReviewMerchantKYCUsecase - Validation
// ============================================================

func TestReviewMerchantKYCUsecase_EmptyShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  "", // ❌ Vide
		Action:  "approve",
		AdminID: "admin-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestReviewMerchantKYCUsecase_EmptyAdminID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  uuid.New().String(),
		Action:  "approve",
		AdminID: "", // ❌ Vide
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "admin_id is required")
}

func TestReviewMerchantKYCUsecase_InvalidAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  uuid.New().String(),
		Action:  "invalid", // ❌ Invalide
		AdminID: "admin-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "action must be 'approve' or 'reject'")
}

func TestReviewMerchantKYCUsecase_RejectWithoutReason(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:          uuid.New().String(),
		Action:          "reject",
		AdminID:         "admin-123",
		RejectionReason: nil, // ❌ Nil pour reject
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "rejection_reason is required")
}

func TestReviewMerchantKYCUsecase_InvalidShopIDFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  "invalid-uuid", // ❌ UUID invalide
		Action:  "approve",
		AdminID: "admin-123",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

// ============================================================
// TESTS : ReviewMerchantKYCUsecase - Repository errors
// ============================================================

func TestReviewMerchantKYCUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	shopID := uuid.New()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  shopID.String(),
		Action:  "approve",
		AdminID: "admin-123",
	}

	// Mock : Shop non trouvée
	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(nil, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "shop not found")
}

func TestReviewMerchantKYCUsecase_ShopNotPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	shopID := uuid.New()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  shopID.String(),
		Action:  "approve",
		AdminID: "admin-123",
	}

	// Shop déjà vérifiée (pas pending)
	verifiedShop := &entity.Shop{
		ID:        shopID,
		Name:      "Verified Shop",
		KYCStatus: entity.ShopKYCStatusVerified,
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(verifiedShop, nil)

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not pending")
}

func TestReviewMerchantKYCUsecase_UpdateKYCStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	shopID := uuid.New()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  shopID.String(),
		Action:  "approve",
		AdminID: "admin-123",
	}

	pendingShop := createTestPendingShop()
	pendingShop.ID = shopID

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(pendingShop, nil)

	// Mock : UpdateKYCStatus échoue
	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shopID, entity.ShopKYCStatusVerified, "admin-123", nil).
		Return(errors.New("database error"))

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "update kyc status")
}

// ============================================================
// TESTS : ReviewMerchantKYCUsecase - Happy paths
// ============================================================

func TestReviewMerchantKYCUsecase_Success_Approve(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	shopID := uuid.New()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  shopID.String(),
		Action:  "approve",
		AdminID: "admin-123",
	}

	pendingShop := createTestPendingShop()
	pendingShop.ID = shopID

	// Shop mise à jour (vérifiée)
	verifiedShop := createTestPendingShop()
	verifiedShop.ID = shopID
	verifiedShop.KYCStatus = entity.ShopKYCStatusVerified
	now := time.Now()
	verifiedShop.KYCVerifiedAt = &now
	verifiedShop.KYCVerifiedBy = strPtr("admin-123")

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(pendingShop, nil)

	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shopID, entity.ShopKYCStatusVerified, "admin-123", nil).
		Return(nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(verifiedShop, nil)

	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(2, nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, shopID.String(), response.ShopID)
	assert.Equal(t, "Pending Shop", response.ShopName)
	assert.Equal(t, "verified", response.KYCStatus)
	assert.NotNil(t, response.KYCVerifiedAt)
	assert.Equal(t, "admin-123", response.KYCVerifiedBy)
	assert.Equal(t, 2, response.DocumentsCount)
	assert.Contains(t, response.Message, "approuvé")
}

func TestReviewMerchantKYCUsecase_Success_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	shopID := uuid.New()
	reason := "Documents illisibles"
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:          shopID.String(),
		Action:          "reject",
		AdminID:         "admin-123",
		RejectionReason: &reason,
	}

	pendingShop := createTestPendingShop()
	pendingShop.ID = shopID

	// Shop mise à jour (rejetée)
	rejectedShop := createTestPendingShop()
	rejectedShop.ID = shopID
	rejectedShop.KYCStatus = entity.ShopKYCStatusRejected
	rejectedShop.KYCRejectionReason = &reason

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(pendingShop, nil)

	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shopID, entity.ShopKYCStatusRejected, "admin-123", &reason).
		Return(nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(rejectedShop, nil)

	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(2, nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "rejected", response.KYCStatus)
	assert.Contains(t, response.Message, "rejeté")
	assert.Contains(t, response.Message, reason)
}

func TestReviewMerchantKYCUsecase_Success_WithDocumentReviews(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewReviewMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	shopID := uuid.New()
	docID1 := uuid.New()
	docID2 := uuid.New()
	req := &merchantkycusecase.ReviewMerchantKYCRequest{
		ShopID:  shopID.String(),
		Action:  "approve",
		AdminID: "admin-123",
		DocumentReviews: []merchantkycusecase.DocumentReview{
			{
				DocumentID: docID1.String(),
				Action:     "approve",
			},
			{
				DocumentID:      docID2.String(),
				Action:          "reject",
				RejectionReason: strPtr("Document flou"),
			},
		},
	}

	pendingShop := createTestPendingShop()
	pendingShop.ID = shopID

	verifiedShop := createTestPendingShop()
	verifiedShop.ID = shopID
	verifiedShop.KYCStatus = entity.ShopKYCStatusVerified

	// Documents
	doc1 := &entity.ShopKYCDocument{
		ID:           docID1,
		ShopID:       shopID,
		DocumentType: entity.ShopKYCDocIdentityCard,
		Status:       "pending",
	}

	doc2 := &entity.ShopKYCDocument{
		ID:           docID2,
		ShopID:       shopID,
		DocumentType: entity.ShopKYCDocBusinessRegistry,
		Status:       "pending",
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(pendingShop, nil)

	mockShopRepo.EXPECT().
		UpdateKYCStatus(gomock.Any(), shopID, entity.ShopKYCStatusVerified, "admin-123", nil).
		Return(nil)

	// Document 1 : approve
	mockKycRepo.EXPECT().
		FindByID(gomock.Any(), docID1).
		Return(doc1, nil)

	mockKycRepo.EXPECT().
		Update(gomock.Any(), doc1).
		Return(nil)

	// Document 2 : reject
	mockKycRepo.EXPECT().
		FindByID(gomock.Any(), docID2).
		Return(doc2, nil)

	mockKycRepo.EXPECT().
		Update(gomock.Any(), doc2).
		Return(nil)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(verifiedShop, nil)

	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shopID).
		Return(2, nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	assert.Equal(t, "verified", response.KYCStatus)
}

// ============================================================
// TESTS : ListPendingMerchantKYCUsecase
// ============================================================

func TestListPendingMerchantKYCUsecase_AutoCorrection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewListPendingMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ListPendingRequest{
		Limit:  0,  // ❌ Sera corrigé à 20
		Offset: -5, // ❌ Sera corrigé à 0
	}

	// Mock : Liste vide
	mockShopRepo.EXPECT().
		FindByKYCStatus(gomock.Any(), entity.ShopKYCStatusPending, 20, 0).
		Return([]*entity.Shop{}, nil)

	mockShopRepo.EXPECT().
		CountByKYCStatus(gomock.Any()).
		Return(map[entity.ShopKYCStatus]int{
			entity.ShopKYCStatusPending: 0,
		}, nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 20, response.Limit)
	assert.Equal(t, 0, response.Offset)
	assert.Empty(t, response.Shops)
}

func TestListPendingMerchantKYCUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewListPendingMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ListPendingRequest{
		Limit:  20,
		Offset: 0,
	}

	// Mock : FindByKYCStatus échoue
	mockShopRepo.EXPECT().
		FindByKYCStatus(gomock.Any(), entity.ShopKYCStatusPending, 20, 0).
		Return(nil, errors.New("database error"))

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "find pending shops")
}

func TestListPendingMerchantKYCUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockKycRepo := mockrepo.NewMockShopKYCDocumentRepository(ctrl)

	uc := merchantkycusecase.NewListPendingMerchantKYCUsecase(mockShopRepo, mockKycRepo)

	ctx := context.Background()
	req := &merchantkycusecase.ListPendingRequest{
		Limit:  20,
		Offset: 0,
	}

	// Créer 2 shops pending
	shop1 := createTestPendingShop()
	shop2 := createTestPendingShop()
	shop2.Name = "Another Pending Shop"

	shops := []*entity.Shop{shop1, shop2}

	mockShopRepo.EXPECT().
		FindByKYCStatus(gomock.Any(), entity.ShopKYCStatusPending, 20, 0).
		Return(shops, nil)

	mockShopRepo.EXPECT().
		CountByKYCStatus(gomock.Any()).
		Return(map[entity.ShopKYCStatus]int{
			entity.ShopKYCStatusPending:  2,
			entity.ShopKYCStatusVerified: 10,
			entity.ShopKYCStatusRejected: 3,
		}, nil)

	// Compter les documents pour chaque shop
	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shop1.ID).
		Return(2, nil)

	mockKycRepo.EXPECT().
		CountByShopID(gomock.Any(), shop2.ID).
		Return(3, nil)

	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Shops, 2)
	assert.Equal(t, 2, response.TotalCount)
	assert.Equal(t, 20, response.Limit)
	assert.Equal(t, 0, response.Offset)

	// Vérifier le premier shop
	assert.Equal(t, shop1.ID.String(), response.Shops[0].ShopID)
	assert.Equal(t, "Pending Shop", response.Shops[0].ShopName)
	assert.Equal(t, "pending", response.Shops[0].KYCStatus)
	assert.Equal(t, 2, response.Shops[0].DocumentsCount)
	assert.GreaterOrEqual(t, response.Shops[0].DaysWaiting, 0)

	// Vérifier le deuxième shop
	assert.Equal(t, shop2.ID.String(), response.Shops[1].ShopID)
	assert.Equal(t, "Another Pending Shop", response.Shops[1].ShopName)
	assert.Equal(t, 3, response.Shops[1].DocumentsCount)
}

// Helper pour créer des pointeurs de string
func strPtr(s string) *string {
	return &s
}
