package sessionusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sessionusecase "Goshop/application/usecase/session_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func createSessionAdminContext() *sessionusecase.AdminContext {
	return &sessionusecase.AdminContext{
		AdminID:          "admin-123",
		AdminEmail:       "admin@example.com",
		AdminRole:        "super_admin",
		IPAddress:        "192.168.1.1",
		UserAgent:        "Mozilla/5.0",
		CurrentSessionID: "current-jti",
	}
}

func createTestSession(id, userID, sessionID string, isActive bool) *entity.UserSession {
	now := time.Now()
	return &entity.UserSession{
		ID:           id,
		UserID:       userID,
		SessionID:    sessionID,
		IsActive:     isActive,
		LastActivity: now,
		CreatedAt:    now.Add(-1 * time.Hour),
		ExpiresAt:    now.Add(7 * 24 * time.Hour),
		DeviceInfo: entity.DeviceInfo{
			Browser: "Chrome",
			OS:      "Windows",
			Device:  "Desktop",
		},
		IPAddress: "192.168.1.1",
	}
}

// ============================================================
// TESTS : ListSessionsUsecase
// ============================================================

func TestListSessionsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewListSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	sessions := []*entity.UserSession{
		createTestSession("s1", "user-123", "jti-1", true),
		createTestSession("s2", "user-123", "jti-2", false),
	}

	mockSessionRepo.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(sessions, nil)

	mockSessionRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "user-123").
		Return(&entity.SessionStatistics{TotalActive: 1, TotalRevoked: 1}, nil)

	req := &sessionusecase.ListSessionsRequest{UserID: "user-123"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Total)
	assert.Len(t, response.Sessions, 2)
}

func TestListSessionsUsecase_ActiveOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewListSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	sessions := []*entity.UserSession{
		createTestSession("s1", "user-123", "jti-1", true),
	}

	mockSessionRepo.EXPECT().
		FindActiveByUserID(gomock.Any(), "user-123").
		Return(sessions, nil)

	mockSessionRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "user-123").
		Return(&entity.SessionStatistics{}, nil)

	req := &sessionusecase.ListSessionsRequest{
		UserID:     "user-123",
		ActiveOnly: true,
	}

	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, response.Total)
}

func TestListSessionsUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewListSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		FindByUserID(gomock.Any(), "admin-123").
		Return([]*entity.UserSession{}, nil)

	mockSessionRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "admin-123").
		Return(&entity.SessionStatistics{}, nil)

	req := &sessionusecase.ListSessionsRequest{UserID: ""}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.Total)
}

func TestListSessionsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewListSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return(nil, errors.New("database error"))

	req := &sessionusecase.ListSessionsRequest{UserID: "user-123"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

// ============================================================
// TESTS : RevokeSessionUsecase
// ============================================================

func TestRevokeSessionUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeSessionUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	session := createTestSession("s1", "admin-123", "target-jti", true)
	mockSessionRepo.EXPECT().
		FindBySessionID(gomock.Any(), "target-jti").
		Return(session, nil)

	mockSessionRepo.EXPECT().
		RevokeSession(gomock.Any(), "target-jti", "admin-123").
		Return(nil)

	req := &sessionusecase.RevokeSessionRequest{TargetSessionID: "target-jti"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, "target-jti", response.RevokedSessionID)
}

func TestRevokeSessionUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeSessionUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		FindBySessionID(gomock.Any(), "unknown-jti").
		Return(nil, repository.ErrSessionNotFound)

	req := &sessionusecase.RevokeSessionRequest{TargetSessionID: "unknown-jti"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestRevokeSessionUsecase_AlreadyRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeSessionUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	session := createTestSession("s1", "admin-123", "target-jti", false)
	mockSessionRepo.EXPECT().
		FindBySessionID(gomock.Any(), "target-jti").
		Return(session, nil)

	req := &sessionusecase.RevokeSessionRequest{TargetSessionID: "target-jti"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "already revoked")
}

func TestRevokeSessionUsecase_CannotRevokeCurrent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeSessionUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	req := &sessionusecase.RevokeSessionRequest{TargetSessionID: "current-jti"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "cannot revoke current")
}

func TestRevokeSessionUsecase_EmptyTargetSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeSessionUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	req := &sessionusecase.RevokeSessionRequest{TargetSessionID: ""}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "required")
}

// ============================================================
// TESTS : RevokeAllSessionsUsecase
// ============================================================

func TestRevokeAllSessionsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeAllSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "user-123").
		Return(3, nil)

	mockSessionRepo.EXPECT().
		RevokeAllUserSessions(gomock.Any(), "user-123", "current-jti").
		Return(nil)

	mockSessionRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "user-123").
		Return(1, nil)

	req := &sessionusecase.RevokeAllSessionsRequest{UserID: "user-123"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.RevokedCount)
	assert.Equal(t, "current-jti", response.ExcludedSession)
}

func TestRevokeAllSessionsUsecase_NoActiveSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeAllSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "user-123").
		Return(0, nil)

	mockSessionRepo.EXPECT().
		RevokeAllUserSessions(gomock.Any(), "user-123", "current-jti").
		Return(nil)

	mockSessionRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "user-123").
		Return(0, nil)

	req := &sessionusecase.RevokeAllSessionsRequest{UserID: "user-123"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.RevokedCount)
}

func TestRevokeAllSessionsUsecase_DefaultUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeAllSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(1, nil)

	mockSessionRepo.EXPECT().
		RevokeAllUserSessions(gomock.Any(), "admin-123", "current-jti").
		Return(nil)

	mockSessionRepo.EXPECT().
		CountActiveByUserID(gomock.Any(), "admin-123").
		Return(1, nil)

	req := &sessionusecase.RevokeAllSessionsRequest{UserID: ""}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

// ============================================================
// TESTS : GetSessionStatsUsecase
// ============================================================

func TestGetSessionStatsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewGetSessionStatsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	stats := &entity.SessionStatistics{
		TotalActive:       10,
		TotalRevoked:      5,
		TotalExpired:      2,
		UniqueUsersActive: 8,
		AvgIdleMinutes:    15.5,
	}

	mockSessionRepo.EXPECT().
		GetStatistics(gomock.Any()).
		Return(stats, nil)

	mockSessionRepo.EXPECT().
		GetMostActiveUsers(gomock.Any(), 10).
		Return([]repository.UserSessionCount{}, nil)

	// ✅ CORRECTION : Remplacer nil par une requête vide
	req := &sessionusecase.GetSessionStatsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 10, response.Statistics.TotalActive)
	assert.Equal(t, 5, response.Statistics.TotalRevoked)
	assert.Equal(t, 8, response.Statistics.UniqueUsersActive)
}

func TestGetSessionStatsUsecase_ByUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewGetSessionStatsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	stats := &entity.SessionStatistics{TotalActive: 2}

	mockSessionRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "user-123").
		Return(stats, nil)

	req := &sessionusecase.GetSessionStatsRequest{UserID: "user-123"}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 2, response.Statistics.TotalActive)
}

func TestGetSessionStatsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewGetSessionStatsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		GetStatistics(gomock.Any()).
		Return(nil, errors.New("database error"))

	// ✅ CORRECTION : Remplacer nil par une requête vide
	req := &sessionusecase.GetSessionStatsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestGetSessionStatsUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewGetSessionStatsUsecase(mockSessionRepo)

	adminCtx := &sessionusecase.AdminContext{
		AdminID:   "user-123",
		AdminRole: "user",
	}

	// ✅ CORRECTION : Remplacer nil par une requête vide
	req := &sessionusecase.GetSessionStatsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "admin role required")
}

// ============================================================
// TESTS : CleanupSessionsUsecase
// ============================================================

func TestCleanupSessionsUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewCleanupSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CleanupExpiredSessions(gomock.Any()).
		Return(15, nil)

	req := &sessionusecase.CleanupSessionsRequest{IncludeRevoked: false}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, 15, response.DeletedCount)
}

func TestCleanupSessionsUsecase_IncludeRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewCleanupSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CleanupAllInactive(gomock.Any()).
		Return(25, nil)

	req := &sessionusecase.CleanupSessionsRequest{IncludeRevoked: true}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 25, response.DeletedCount)
}

func TestCleanupSessionsUsecase_NothingToClean(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewCleanupSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CleanupExpiredSessions(gomock.Any()).
		Return(0, nil)

	req := &sessionusecase.CleanupSessionsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.DeletedCount)
}

func TestCleanupSessionsUsecase_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewCleanupSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		CleanupExpiredSessions(gomock.Any()).
		Return(0, errors.New("cleanup failed"))

	req := &sessionusecase.CleanupSessionsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestCleanupSessionsUsecase_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewCleanupSessionsUsecase(mockSessionRepo)

	adminCtx := &sessionusecase.AdminContext{
		AdminID:   "admin-123",
		AdminRole: "admin",
	}

	req := &sessionusecase.CleanupSessionsRequest{}
	response, err := uc.Execute(context.Background(), adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "super_admin role required")
}

// ============================================================
// TESTS : Edge cases
// ============================================================

func TestListSessionsUsecase_Performance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewListSessionsUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	mockSessionRepo.EXPECT().
		FindByUserID(gomock.Any(), "user-123").
		Return([]*entity.UserSession{}, nil)

	mockSessionRepo.EXPECT().
		GetStatisticsByUser(gomock.Any(), "user-123").
		Return(&entity.SessionStatistics{}, nil)

	req := &sessionusecase.ListSessionsRequest{UserID: "user-123"}

	start := time.Now()
	response, err := uc.Execute(context.Background(), adminCtx, req)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, duration < 100*time.Millisecond,
		"ListSessions should complete in less than 100ms, got %v", duration)
}

func TestRevokeSessionUsecase_ContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)
	uc := sessionusecase.NewRevokeSessionUsecase(mockSessionRepo)
	adminCtx := createSessionAdminContext()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockSessionRepo.EXPECT().
		FindBySessionID(gomock.Any(), "target-jti").
		Return(nil, context.Canceled)

	req := &sessionusecase.RevokeSessionRequest{TargetSessionID: "target-jti"}
	response, err := uc.Execute(ctx, adminCtx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.True(t, errors.Is(err, context.Canceled))
}
