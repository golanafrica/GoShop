package apikeyusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	apikeyusecase "Goshop/application/usecase/apikey_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.4 : TESTS UNITAIRES - USECASES API KEYS
// ============================================================

// ============================================================
// HELPERS
// ============================================================

func createAPIKeyAdminContext(role string) *apikeyusecase.AdminContext {
	return &apikeyusecase.AdminContext{
		AdminID:    "admin-123",
		AdminEmail: "admin@example.com",
		AdminRole:  role,
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
	}
}

func createTestAPIKey(id, userID string, isActive bool) *entity.APIKey {
	now := time.Now()
	future := now.Add(365 * 24 * time.Hour)
	return &entity.APIKey{
		ID:                 id,
		UserID:             userID,
		Name:               "Test Key",
		KeyPrefix:          "gsk_test_abc123",
		KeyHash:            "hash123",
		Scopes:             []entity.APIKeyScope{entity.ScopeReadProducts},
		ExpiresAt:          &future,
		IsActive:           isActive,
		RateLimitPerMinute: 60,
		RateLimitPerDay:    10000,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// ============================================================
// TESTS : CreateAPIKeyUsecase
// ============================================================

func TestCreateAPIKeyUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	// Mock : CountActiveByUserID retourne 0 (pas de limite atteinte)
	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(0, nil)

	// Mock : Create réussit
	mockAPIKeyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:   "Integration Test",
		Scopes: []entity.APIKeyScope{entity.ScopeReadProducts, entity.ScopeReadShops},
		IsTest: true,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.FullKey, "Full key should be displayed once")
	assert.NotNil(t, response.APIKey)
	assert.Contains(t, response.Warning, "JAMAIS")
}

func TestCreateAPIKeyUsecase_EmptyName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:   "", // Vide
		Scopes: []entity.APIKeyScope{entity.ScopeReadProducts},
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "name is required")
}

func TestCreateAPIKeyUsecase_EmptyScopes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:   "Test",
		Scopes: []entity.APIKeyScope{}, // Vide
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at least one scope is required")
}

func TestCreateAPIKeyUsecase_InvalidScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:   "Test",
		Scopes: []entity.APIKeyScope{"invalid:scope"},
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid scope")
}

func TestCreateAPIKeyUsecase_LimitReached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	// Mock : 10 clés actives (limite atteinte)
	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(10, nil)

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:   "Test",
		Scopes: []entity.APIKeyScope{entity.ScopeReadProducts},
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "maximum API keys limit reached")
}

func TestCreateAPIKeyUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)

	// User normal (pas admin)
	adminCtx := &apikeyusecase.AdminContext{
		AdminID:   "user-123",
		AdminRole: "user",
	}

	req := &apikeyusecase.CreateAPIKeyRequest{
		UserID: "other-user", // Tente de créer pour un autre user
		Name:   "Test",
		Scopes: []entity.APIKeyScope{entity.ScopeReadProducts},
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestCreateAPIKeyUsecase_AdminCreatesForOtherUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("admin")

	// Mock : CountActiveByUserID pour l'autre user
	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "other-user").
		Return(0, nil)

	mockAPIKeyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &apikeyusecase.CreateAPIKeyRequest{
		UserID: "other-user", // Admin crée pour un autre user
		Name:   "Test",
		Scopes: []entity.APIKeyScope{entity.ScopeReadProducts},
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}

// ============================================================
// TESTS : ListAPIKeysUsecase
// ============================================================

func TestListAPIKeysUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewListAPIKeysUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	apiKeys := []*entity.APIKey{
		createTestAPIKey("k1", "admin-123", true),
		createTestAPIKey("k2", "admin-123", false),
	}

	mockAPIKeyRepo.EXPECT().
		FindByUserID(gomock.Any(), "admin-123").
		Return(apiKeys, nil)

	mockAPIKeyRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "admin-123").
		Return(&entity.APIKeyStatistics{TotalKeys: 2, ActiveKeys: 1}, nil)

	req := &apikeyusecase.ListAPIKeysRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Total)
	assert.Len(t, response.APIKeys, 2)
}

func TestListAPIKeysUsecase_ActiveOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewListAPIKeysUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	apiKeys := []*entity.APIKey{
		createTestAPIKey("k1", "admin-123", true),
	}

	mockAPIKeyRepo.EXPECT().
		FindActiveByUserID(gomock.Any(), "admin-123").
		Return(apiKeys, nil)

	mockAPIKeyRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "admin-123").
		Return(&entity.APIKeyStatistics{}, nil)

	req := &apikeyusecase.ListAPIKeysRequest{
		ActiveOnly: true,
	}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, response.Total)
}

func TestListAPIKeysUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewListAPIKeysUsecase(mockAPIKeyRepo)

	adminCtx := &apikeyusecase.AdminContext{
		AdminID:   "user-123",
		AdminRole: "user",
	}

	req := &apikeyusecase.ListAPIKeysRequest{
		UserID: "other-user", // Tente de voir les clés d'un autre user
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestListAPIKeysUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewListAPIKeysUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		FindByUserID(gomock.Any(), "admin-123").
		Return(nil, errors.New("database error"))

	req := &apikeyusecase.ListAPIKeysRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

// ============================================================
// TESTS : RevokeAPIKeyUsecase
// ============================================================

func TestRevokeAPIKeyUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	apiKey := createTestAPIKey("k1", "admin-123", true)

	mockAPIKeyRepo.EXPECT().
		FindByID(gomock.Any(), "k1").
		Return(apiKey, nil)

	mockAPIKeyRepo.EXPECT().
		RevokeAPIKey(gomock.Any(), "k1", "admin-123", "Security concern").
		Return(nil)

	req := &apikeyusecase.RevokeAPIKeyRequest{
		APIKeyID: "k1",
		Reason:   "Security concern",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "k1", response.RevokedKeyID)
}

func TestRevokeAPIKeyUsecase_EmptyAPIKeyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	req := &apikeyusecase.RevokeAPIKeyRequest{
		APIKeyID: "", // Vide
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "api_key_id is required")
}

func TestRevokeAPIKeyUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		FindByID(gomock.Any(), "unknown").
		Return(nil, repository.ErrAPIKeyNotFound)

	req := &apikeyusecase.RevokeAPIKeyRequest{
		APIKeyID: "unknown",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "API key not found")
}

func TestRevokeAPIKeyUsecase_AlreadyRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	// Clé déjà révoquée
	revokedAt := time.Now()
	apiKey := &entity.APIKey{
		ID:        "k1",
		UserID:    "admin-123",
		IsActive:  false,
		RevokedAt: &revokedAt,
	}

	mockAPIKeyRepo.EXPECT().
		FindByID(gomock.Any(), "k1").
		Return(apiKey, nil)

	req := &apikeyusecase.RevokeAPIKeyRequest{
		APIKeyID: "k1",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "already revoked")
}

func TestRevokeAPIKeyUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAPIKeyUsecase(mockAPIKeyRepo)

	adminCtx := &apikeyusecase.AdminContext{
		AdminID:   "user-123",
		AdminRole: "user",
	}

	// Clé appartenant à un autre user
	apiKey := createTestAPIKey("k1", "other-user", true)

	mockAPIKeyRepo.EXPECT().
		FindByID(gomock.Any(), "k1").
		Return(apiKey, nil)

	req := &apikeyusecase.RevokeAPIKeyRequest{
		APIKeyID: "k1",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

// ============================================================
// TESTS : RevokeAllAPIKeysUsecase
// ============================================================

func TestRevokeAllAPIKeysUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAllAPIKeysUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "user-123").
		Return(5, nil)

	mockAPIKeyRepo.EXPECT().
		RevokeAllUserAPIKeys(gomock.Any(), "user-123", "", "admin-123", "Security breach").
		Return(nil)

	req := &apikeyusecase.RevokeAllAPIKeysRequest{
		UserID: "user-123",
		Reason: "Security breach",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 5, response.RevokedCount)
}

func TestRevokeAllAPIKeysUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAllAPIKeysUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(2, nil)

	mockAPIKeyRepo.EXPECT().
		RevokeAllUserAPIKeys(gomock.Any(), "admin-123", "", "admin-123", gomock.Any()).
		Return(nil)

	req := &apikeyusecase.RevokeAllAPIKeysRequest{
		UserID: "", // Vide → utilise AdminID
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

func TestRevokeAllAPIKeysUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewRevokeAllAPIKeysUsecase(mockAPIKeyRepo)

	adminCtx := &apikeyusecase.AdminContext{
		AdminID:   "user-123",
		AdminRole: "user",
	}

	req := &apikeyusecase.RevokeAllAPIKeysRequest{
		UserID: "other-user",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

// ============================================================
// TESTS : GetAPIKeyStatsUsecase
// ============================================================

func TestGetAPIKeyStatsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewGetAPIKeyStatsUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	stats := &entity.APIKeyStatistics{
		TotalKeys:         100,
		ActiveKeys:        50,
		RevokedKeys:       30,
		ExpiredKeys:       20,
		UniqueUsers:       25,
		CallsToday:        1000,
		CallsLast7Days:    5000,
		TotalCallsAllTime: 100000,
	}

	mockAPIKeyRepo.EXPECT().
		GetStatistics(gomock.Any()).
		Return(stats, nil)

	mockAPIKeyRepo.EXPECT().
		GetMostUsedKeys(gomock.Any(), 10).
		Return([]repository.APIKeyUsageCount{}, nil)

	req := &apikeyusecase.GetAPIKeyStatsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 100, response.Statistics.TotalKeys)
	assert.Equal(t, 50, response.Statistics.ActiveKeys)
}

func TestGetAPIKeyStatsUsecase_ByUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewGetAPIKeyStatsUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	stats := &entity.APIKeyStatistics{
		TotalKeys:  5,
		ActiveKeys: 3,
	}

	mockAPIKeyRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "user-123").
		Return(stats, nil)

	req := &apikeyusecase.GetAPIKeyStatsRequest{
		UserID: "user-123",
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 5, response.Statistics.TotalKeys)
}

func TestGetAPIKeyStatsUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewGetAPIKeyStatsUsecase(mockAPIKeyRepo)

	adminCtx := &apikeyusecase.AdminContext{
		AdminID:   "user-123",
		AdminRole: "user",
	}

	req := &apikeyusecase.GetAPIKeyStatsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "admin role required")
}

func TestGetAPIKeyStatsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewGetAPIKeyStatsUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		GetStatistics(gomock.Any()).
		Return(nil, errors.New("database error"))

	req := &apikeyusecase.GetAPIKeyStatsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

// ============================================================
// TESTS : ValidateAPIKeyUsecase
// ============================================================

func TestValidateAPIKeyUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewValidateAPIKeyUsecase(mockAPIKeyRepo)

	// Clé valide (format gsk_test_ + 48 caractères)
	validKey := "gsk_test_" + "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6"

	apiKey := createTestAPIKey("k1", "user-123", true)

	mockAPIKeyRepo.EXPECT().
		ValidateKey(gomock.Any(), gomock.Any()).
		Return(apiKey, nil)

	mockAPIKeyRepo.EXPECT().
		MarkKeyUsed(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes() // Goroutine non bloquante

	req := &apikeyusecase.ValidateAPIKeyRequest{
		FullKey: validKey,
	}

	response, err := uc.Execute(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Valid)
	assert.NotNil(t, response.APIKey)
}

func TestValidateAPIKeyUsecase_WithScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewValidateAPIKeyUsecase(mockAPIKeyRepo)

	validKey := "gsk_test_" + "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6"

	mockAPIKeyRepo.EXPECT().
		CheckScope(gomock.Any(), gomock.Any(), entity.ScopeReadProducts).
		Return(nil)

	mockAPIKeyRepo.EXPECT().
		FindByHash(gomock.Any(), gomock.Any()).
		Return(createTestAPIKey("k1", "user-123", true), nil)

	mockAPIKeyRepo.EXPECT().
		MarkKeyUsed(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	req := &apikeyusecase.ValidateAPIKeyRequest{
		FullKey:       validKey,
		RequiredScope: entity.ScopeReadProducts,
	}

	response, err := uc.Execute(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Valid)
}

func TestValidateAPIKeyUsecase_InvalidFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewValidateAPIKeyUsecase(mockAPIKeyRepo)

	req := &apikeyusecase.ValidateAPIKeyRequest{
		FullKey: "invalid-key", // Format invalide
	}

	response, err := uc.Execute(context.Background(), req)

	assert.NoError(t, err) // Pas d'erreur, juste Valid=false
	assert.NotNil(t, response)
	assert.False(t, response.Valid)
	assert.NotEmpty(t, response.Error)
}

func TestValidateAPIKeyUsecase_KeyRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewValidateAPIKeyUsecase(mockAPIKeyRepo)

	validKey := "gsk_test_" + "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6"

	mockAPIKeyRepo.EXPECT().
		CheckScope(gomock.Any(), gomock.Any(), entity.ScopeReadProducts).
		Return(entity.ErrAPIKeyRevoked)

	req := &apikeyusecase.ValidateAPIKeyRequest{
		FullKey:       validKey,
		RequiredScope: entity.ScopeReadProducts,
	}

	response, err := uc.Execute(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.False(t, response.Valid)
	assert.Contains(t, response.Error, "revoked")
}

func TestValidateAPIKeyUsecase_KeyExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewValidateAPIKeyUsecase(mockAPIKeyRepo)

	validKey := "gsk_test_" + "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6"

	mockAPIKeyRepo.EXPECT().
		CheckScope(gomock.Any(), gomock.Any(), entity.ScopeReadProducts).
		Return(entity.ErrAPIKeyExpired)

	req := &apikeyusecase.ValidateAPIKeyRequest{
		FullKey:       validKey,
		RequiredScope: entity.ScopeReadProducts,
	}

	response, err := uc.Execute(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.False(t, response.Valid)
	assert.Contains(t, response.Error, "expired")
}

func TestValidateAPIKeyUsecase_InsufficientScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewValidateAPIKeyUsecase(mockAPIKeyRepo)

	validKey := "gsk_test_" + "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6"

	mockAPIKeyRepo.EXPECT().
		CheckScope(gomock.Any(), gomock.Any(), entity.ScopeWriteProducts).
		Return(entity.ErrAPIKeyInsufficientScope)

	req := &apikeyusecase.ValidateAPIKeyRequest{
		FullKey:       validKey,
		RequiredScope: entity.ScopeWriteProducts,
	}

	response, err := uc.Execute(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.False(t, response.Valid)
	assert.Contains(t, response.Error, "scope")
}

// ============================================================
// TESTS : Edge cases
// ============================================================

func TestCreateAPIKeyUsecase_Performance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(0, nil)

	mockAPIKeyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:   "Test",
		Scopes: []entity.APIKeyScope{entity.ScopeReadProducts},
	}

	start := time.Now()
	response, err := uc.Execute(context.Background(), adminCtx, req)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, duration < 100*time.Millisecond,
		"CreateAPIKey should complete in less than 100ms, got %v", duration)
}

func TestCreateAPIKeyUsecase_CustomRateLimits(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(0, nil)

	mockAPIKeyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:            "Test",
		Scopes:          []entity.APIKeyScope{entity.ScopeReadProducts},
		RateLimitMinute: 100,  // Custom
		RateLimitDay:    5000, // Custom
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}

func TestCreateAPIKeyUsecase_CustomLifetime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPIKeyRepo := mockrepo.NewMockAPIKeyRepository(ctrl)
	uc := apikeyusecase.NewCreateAPIKeyUsecase(mockAPIKeyRepo)
	adminCtx := createAPIKeyAdminContext("super_admin")

	mockAPIKeyRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(0, nil)

	mockAPIKeyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	req := &apikeyusecase.CreateAPIKeyRequest{
		Name:         "Test",
		Scopes:       []entity.APIKeyScope{entity.ScopeReadProducts},
		LifetimeDays: 30, // 30 jours au lieu de 365
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
}
