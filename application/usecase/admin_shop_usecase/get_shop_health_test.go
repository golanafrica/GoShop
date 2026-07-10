package adminshopusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adminshopusecase "Goshop/application/usecase/admin_shop_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.11 : TESTS UNITAIRES - GET SHOP HEALTH USECASE
// ============================================================

// ============================================================
// TESTS : GetShopHealthUsecase - Validation
// ============================================================

func TestGetShopHealthUsecase_InvalidShopID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: "invalid-uuid", // ❌ UUID invalide
	}

	response, err := uc.Execute(ctx, admin, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid shop_id format")
}

// ============================================================
// TESTS : GetShopHealthUsecase - Repository errors
// ============================================================

func TestGetShopHealthUsecase_ShopNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
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

func TestGetShopHealthUsecase_ShopNotFound_Nil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
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
// TESTS : GetShopHealthUsecase - Stats error (non bloquant)
// ============================================================

func TestGetShopHealthUsecase_GetHealthStatsError_NonBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	testShop := createTestShop("Test Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.HealthScore = 1000
	testShop.HealthLevel = entity.ShopHealthExcellent

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	// Mock : GetHealthStats échoue (non bloquant)
	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(nil, errors.New("stats error"))

	response, err := uc.Execute(ctx, admin, req)

	// ✅ Erreur non bloquante
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.PlatformComparison)
	assert.Equal(t, 0, response.PlatformComparison.TotalShops)
}

// ============================================================
// TESTS : GetShopHealthUsecase - Happy paths (différents états)
// ============================================================

func TestGetShopHealthUsecase_Success_ExcellentShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop en excellent état
	now := time.Now()
	excellentShop := &entity.Shop{
		ID:              shopID,
		Name:            "Excellent Shop",
		Slug:            "excellent-shop",
		OwnerID:         "owner-123",
		Plan:            entity.ShopPlanPro,
		IsActive:        true,
		KYCStatus:       entity.ShopKYCStatusVerified,
		HealthScore:     1000,
		HealthLevel:     entity.ShopHealthExcellent,
		HealthUpdatedAt: &now,
		CreatedAt:       time.Now().Add(-200 * 24 * time.Hour), // > 6 mois
		UpdatedAt:       time.Now(),
		LastReviewedAt:  &now, // Revue récente
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(excellentShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   100,
			AverageScore: 800,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier le score actuel
	assert.NotNil(t, response.Current)
	assert.Equal(t, 1000, response.Current.Score)
	assert.Equal(t, "excellent", response.Current.Level)
	assert.False(t, response.Current.NeedsAction)

	// Vérifier le score recalculé
	assert.NotNil(t, response.Recalculated)
	assert.Equal(t, 1000, response.Recalculated.Score)
	assert.False(t, response.Recalculated.IsDifferent)

	// Vérifier le breakdown
	assert.NotNil(t, response.Breakdown)
	assert.Equal(t, 1000, response.Breakdown.BaseScore)
	assert.Equal(t, 1000, response.Breakdown.FinalScore)
	assert.Len(t, response.Breakdown.Adjustments, 4) // kyc, active, suspended, seniority

	// Vérifier les recommandations (vide pour shop excellent)
	assert.NotNil(t, response.Recommendations)
	// Pas de recommandations critiques pour shop excellent avec revue récente

	// Vérifier la comparaison plateforme
	assert.NotNil(t, response.PlatformComparison)
	assert.Equal(t, 800, response.PlatformComparison.PlatformAverage)
	assert.Equal(t, 100, response.PlatformComparison.TotalShops)
}

func TestGetShopHealthUsecase_Success_ShopWithKYCUnverified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop avec KYC non vérifié
	testShop := createTestShop("Unverified Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusUnverified)
	testShop.ID = shopID
	testShop.HealthScore = 800
	testShop.HealthLevel = entity.ShopHealthExcellent
	testShop.CreatedAt = time.Now().Add(-30 * 24 * time.Hour) // < 6 mois

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation KYC critique
	hasKYCRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "critical" && rec.Title == "Vérifier le KYC du marchand" {
			hasKYCRecommendation = true
			assert.Equal(t, 200, rec.Impact)
			break
		}
	}
	assert.True(t, hasKYCRecommendation, "Devrait avoir une recommandation KYC critique")

	// Vérifier le breakdown (KYC non vérifié = -200)
	assert.NotNil(t, response.Breakdown)
	hasKYCAdjustment := false
	for _, adj := range response.Breakdown.Adjustments {
		if adj.Factor == "kyc_status" {
			hasKYCAdjustment = true
			assert.Equal(t, -200, adj.Value)
			assert.Equal(t, "critical", adj.Status)
			break
		}
	}
	assert.True(t, hasKYCAdjustment, "Devrait avoir un ajustement KYC")
}

func TestGetShopHealthUsecase_Success_InactiveShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop inactive
	testShop := createTestShop("Inactive Shop", entity.ShopPlanFree, false, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.HealthScore = 700
	testShop.HealthLevel = entity.ShopHealthGood
	testShop.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation "Réactiver la boutique"
	hasActivateRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "critical" && rec.Title == "Réactiver la boutique" {
			hasActivateRecommendation = true
			assert.Equal(t, 300, rec.Impact)
			break
		}
	}
	assert.True(t, hasActivateRecommendation, "Devrait avoir une recommandation d'activation")

	// Vérifier le breakdown (shop inactif = -300)
	hasActiveAdjustment := false
	for _, adj := range response.Breakdown.Adjustments {
		if adj.Factor == "is_active" {
			hasActiveAdjustment = true
			assert.Equal(t, -300, adj.Value)
			assert.Equal(t, "critical", adj.Status)
			break
		}
	}
	assert.True(t, hasActiveAdjustment, "Devrait avoir un ajustement is_active")
}

func TestGetShopHealthUsecase_Success_SuspendedShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop suspendue
	now := time.Now()
	adminID := "admin-456"
	reason := "Violation des conditions"
	suspendedShop := &entity.Shop{
		ID:               shopID,
		Name:             "Suspended Shop",
		Slug:             "suspended-shop",
		OwnerID:          "owner-123",
		Plan:             entity.ShopPlanFree,
		IsActive:         false,
		KYCStatus:        entity.ShopKYCStatusVerified,
		HealthScore:      300,
		HealthLevel:      entity.ShopHealthCritical,
		SuspendedAt:      &now,
		SuspendedBy:      &adminID,
		SuspensionReason: &reason,
		CreatedAt:        time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:        time.Now(),
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(suspendedShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation "Lever la suspension"
	hasSuspendRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "critical" && rec.Title == "Lever la suspension" {
			hasSuspendRecommendation = true
			assert.Equal(t, 500, rec.Impact)
			break
		}
	}
	assert.True(t, hasSuspendRecommendation, "Devrait avoir une recommandation de suspension")

	// Vérifier le breakdown (suspended = -500)
	hasSuspendedAdjustment := false
	for _, adj := range response.Breakdown.Adjustments {
		if adj.Factor == "suspended" {
			hasSuspendedAdjustment = true
			assert.Equal(t, -500, adj.Value)
			assert.Equal(t, "critical", adj.Status)
			break
		}
	}
	assert.True(t, hasSuspendedAdjustment, "Devrait avoir un ajustement suspended")

	// Vérifier NeedsAction = true pour shop critical
	assert.True(t, response.Current.NeedsAction)
}

func TestGetShopHealthUsecase_Success_CriticalScore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop avec score critique (mais pas suspendue, active)
	now := time.Now()
	criticalShop := &entity.Shop{
		ID:              shopID,
		Name:            "Critical Shop",
		Slug:            "critical-shop",
		OwnerID:         "owner-123",
		Plan:            entity.ShopPlanFree,
		IsActive:        true,
		KYCStatus:       entity.ShopKYCStatusVerified,
		HealthScore:     300, // < 400 = critical
		HealthLevel:     entity.ShopHealthCritical,
		HealthUpdatedAt: &now,
		CreatedAt:       time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:       time.Now(),
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(criticalShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation "Score de santé critique"
	hasCriticalRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "high" && rec.Title == "Score de santé critique" {
			hasCriticalRecommendation = true
			break
		}
	}
	assert.True(t, hasCriticalRecommendation, "Devrait avoir une recommandation de score critique")
}

func TestGetShopHealthUsecase_Success_WarningScore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop avec score warning (400-599)
	now := time.Now()
	warningShop := &entity.Shop{
		ID:              shopID,
		Name:            "Warning Shop",
		Slug:            "warning-shop",
		OwnerID:         "owner-123",
		Plan:            entity.ShopPlanFree,
		IsActive:        true,
		KYCStatus:       entity.ShopKYCStatusVerified,
		HealthScore:     500, // 400-599 = warning
		HealthLevel:     entity.ShopHealthWarning,
		HealthUpdatedAt: &now,
		CreatedAt:       time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:       time.Now(),
	}

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(warningShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation "Score de santé à améliorer"
	hasWarningRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "medium" && rec.Title == "Score de santé à améliorer" {
			hasWarningRecommendation = true
			break
		}
	}
	assert.True(t, hasWarningRecommendation, "Devrait avoir une recommandation de score warning")
}

func TestGetShopHealthUsecase_Success_NeverReviewed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop jamais revue
	testShop := createTestShop("Never Reviewed Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.HealthScore = 1000
	testShop.HealthLevel = entity.ShopHealthExcellent
	testShop.LastReviewedAt = nil // ❌ Jamais revue
	testShop.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation "Effectuer une revue admin"
	hasReviewRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "low" && rec.Title == "Effectuer une revue admin" {
			hasReviewRecommendation = true
			break
		}
	}
	assert.True(t, hasReviewRecommendation, "Devrait avoir une recommandation de revue admin")
}

func TestGetShopHealthUsecase_Success_OldReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop revue il y a 45 jours (> 30 jours)
	oldReview := time.Now().Add(-45 * 24 * time.Hour)
	reviewedBy := "admin-789"
	testShop := createTestShop("Old Review Shop", entity.ShopPlanFree, true, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.HealthScore = 1000
	testShop.HealthLevel = entity.ShopHealthExcellent
	testShop.LastReviewedAt = &oldReview
	testShop.LastReviewedBy = &reviewedBy
	testShop.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier qu'il y a une recommandation "Revue admin requise"
	hasOldReviewRecommendation := false
	for _, rec := range response.Recommendations {
		if rec.Priority == "low" && rec.Title == "Revue admin requise" {
			hasOldReviewRecommendation = true
			break
		}
	}
	assert.True(t, hasOldReviewRecommendation, "Devrait avoir une recommandation de revue admin requise")
}

func TestGetShopHealthUsecase_Success_ScoreRecalculated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)

	uc := adminshopusecase.NewGetShopHealthUsecase(mockShopRepo)

	ctx := context.Background()
	admin := createTestAdminContext()
	shopID := uuid.New()
	req := &adminshopusecase.GetShopHealthRequest{
		ShopID: shopID.String(),
	}

	// Shop avec score stocké différent du score recalculé
	// (ex: shop devenu inactif depuis le dernier calcul)
	testShop := createTestShop("Recalculated Shop", entity.ShopPlanFree, false, entity.ShopKYCStatusVerified)
	testShop.ID = shopID
	testShop.HealthScore = 1000 // Score stocké (ancien)
	testShop.HealthLevel = entity.ShopHealthExcellent
	testShop.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)

	mockShopRepo.EXPECT().
		FindByID(gomock.Any(), shopID).
		Return(testShop, nil)

	mockShopRepo.EXPECT().
		GetHealthStats(gomock.Any()).
		Return(&repository.ShopHealthStats{
			TotalShops:   50,
			AverageScore: 700,
		}, nil)

	response, err := uc.Execute(ctx, admin, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Vérifier que le score recalculé est différent du score stocké
	assert.NotNil(t, response.Recalculated)
	assert.True(t, response.Recalculated.IsDifferent, "Le score recalculé devrait être différent du stocké")
	assert.Less(t, response.Recalculated.Score, response.Current.Score) // Score recalculé < score stocké
}
