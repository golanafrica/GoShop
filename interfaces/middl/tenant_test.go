package middl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"
	mockrepo "Goshop/mocks/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.11.0 : TESTS UNITAIRES - TENANT RESOLVER
// ============================================================

func TestTenantResolver_OwnerAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// ✅ Utiliser des UUIDs valides (pas "owner-123")
	ownerUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:       shopID,
		Name:     "Test Shop",
		Slug:     "test-shop",
		OwnerID:  ownerUUID.String(),
		IsActive: true,
	}

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	logger := zerolog.Nop()

	mockShopRepo.EXPECT().
		FindBySlug(gomock.Any(), "test-shop").
		Return(shop, nil)

	// Mock : IsOwner reçoit un uuid.UUID parsé depuis le string
	mockShopRepo.EXPECT().
		IsOwner(gomock.Any(), shopID, ownerUUID).
		Return(true, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resolvedShop, err := tenant.FromContext(r.Context())
		assert.NoError(t, err)
		assert.NotNil(t, resolvedShop)
		assert.Equal(t, shopID, resolvedShop.ID)
		w.WriteHeader(http.StatusOK)
	})

	middleware := middl.TenantResolver(mockShopRepo, mockCollabRepo, logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Shop-Slug", "test-shop")
	ctx := utils.WithUserID(context.Background(), ownerUUID.String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantResolver_CollaboratorAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ownerUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	collabUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:       shopID,
		Name:     "Test Shop",
		Slug:     "test-shop",
		OwnerID:  ownerUUID.String(),
		IsActive: true,
	}

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	logger := zerolog.Nop()

	mockShopRepo.EXPECT().
		FindBySlug(gomock.Any(), "test-shop").
		Return(shop, nil)

	mockShopRepo.EXPECT().
		IsOwner(gomock.Any(), shopID, collabUUID).
		Return(false, nil)

	mockCollabRepo.EXPECT().
		IsShopCollaborator(gomock.Any(), collabUUID.String(), shopID).
		Return(true, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := middl.TenantResolver(mockShopRepo, mockCollabRepo, logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Shop-Slug", "test-shop")
	ctx := utils.WithUserID(context.Background(), collabUUID.String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantResolver_AccessDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ownerUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	maliciousUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440099")
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:       shopID,
		Name:     "Victim Shop",
		Slug:     "victim-shop",
		OwnerID:  ownerUUID.String(),
		IsActive: true,
	}

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	logger := zerolog.Nop()

	mockShopRepo.EXPECT().
		FindBySlug(gomock.Any(), "victim-shop").
		Return(shop, nil)

	mockShopRepo.EXPECT().
		IsOwner(gomock.Any(), shopID, maliciousUUID).
		Return(false, nil)

	mockCollabRepo.EXPECT().
		IsShopCollaborator(gomock.Any(), maliciousUUID.String(), shopID).
		Return(false, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	middleware := middl.TenantResolver(mockShopRepo, mockCollabRepo, logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Shop-Slug", "victim-shop")
	ctx := utils.WithUserID(context.Background(), maliciousUUID.String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Access denied")
}

func TestTenantResolver_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shopID := uuid.New()
	shop := &entity.Shop{
		ID:       shopID,
		Name:     "Test Shop",
		Slug:     "test-shop",
		OwnerID:  "550e8400-e29b-41d4-a716-446655440001",
		IsActive: true,
	}

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	logger := zerolog.Nop()

	mockShopRepo.EXPECT().
		FindBySlug(gomock.Any(), "test-shop").
		Return(shop, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	middleware := middl.TenantResolver(mockShopRepo, mockCollabRepo, logger)
	wrappedHandler := middleware(handler)

	// Requête SANS userID dans contexte
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Shop-Slug", "test-shop")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestTenantResolver_InactiveShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ownerUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	shopID := uuid.New()
	shop := &entity.Shop{
		ID:       shopID,
		Name:     "Test Shop",
		Slug:     "test-shop",
		OwnerID:  ownerUUID.String(),
		IsActive: false,
	}

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	logger := zerolog.Nop()

	mockShopRepo.EXPECT().
		FindBySlug(gomock.Any(), "test-shop").
		Return(shop, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	middleware := middl.TenantResolver(mockShopRepo, mockCollabRepo, logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Shop-Slug", "test-shop")
	ctx := utils.WithUserID(context.Background(), ownerUUID.String())
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Shop is inactive")
}

func TestTenantResolver_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	shopID := uuid.New()
	shop := &entity.Shop{
		ID:       shopID,
		Name:     "Test Shop",
		Slug:     "test-shop",
		OwnerID:  "550e8400-e29b-41d4-a716-446655440001",
		IsActive: true,
	}

	mockShopRepo := mockrepo.NewMockShopRepository(ctrl)
	mockCollabRepo := mockrepo.NewMockShopCollaboratorRepository(ctrl)
	logger := zerolog.Nop()

	mockShopRepo.EXPECT().
		FindBySlug(gomock.Any(), "test-shop").
		Return(shop, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	middleware := middl.TenantResolver(mockShopRepo, mockCollabRepo, logger)
	wrappedHandler := middleware(handler)

	// Requête avec userID invalide (pas un UUID)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Shop-Slug", "test-shop")
	ctx := utils.WithUserID(context.Background(), "invalid-uuid-format")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid user ID")
}
