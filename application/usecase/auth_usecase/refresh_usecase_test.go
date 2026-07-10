package authusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authusecase "Goshop/application/usecase/auth_usecase"
	authentity "Goshop/domain/auth_entity"
	"Goshop/interfaces/utils"
	mockauthrepo "Goshop/mocks/auth_repository"
	mockrepo "Goshop/mocks/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.14 : TESTS UNITAIRES - REFRESH USECASE
// ============================================================

// ============================================================
// HELPERS
// ============================================================

// createValidClaims crée des claims valides pour les tests
func createValidClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"type": "refresh",
		"sub":  "user-123",
		"jti":  "jti-abc-123",
		"role": "merchant",
	}
}

// createValidSession crée une session valide pour les tests
func createValidSession() *authentity.RefreshSession {
	return &authentity.RefreshSession{
		ID:        "jti-abc-123",
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
		CreatedAt: time.Now(),
	}
}

// createRefreshUsecase crée un usecase avec mocks
func createRefreshUsecase(
	mockRefreshRepo *mockauthrepo.MockRefreshSessionRepository,
	mockSessionRepo *mockrepo.MockUserSessionRepository,
	validateToken func(string) (jwt.MapClaims, error),
	generateAccess func(string, string) (string, error),
	generateRefresh func(string, string, string) (string, error),
) *authusecase.RefreshUsecase {
	now := func() time.Time { return time.Now() }
	newJTI := func() string { return "new-jti-456" }

	return authusecase.NewRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		generateAccess,
		generateRefresh,
		now,
		newJTI,
		7*24*time.Hour,
	)
}

// ============================================================
// TESTS : RefreshUsecase - Validation token
// ============================================================

func TestRefreshUsecase_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	// Mock validateToken qui échoue
	validateToken := func(token string) (jwt.MapClaims, error) {
		return nil, errors.New("invalid token")
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()
	access, refresh, err := uc.Execute(ctx, "invalid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrRefreshTokenInvalid, err)
}

func TestRefreshUsecase_InvalidTokenType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	// Mock validateToken qui retourne un token de type "access" (pas "refresh")
	validateToken := func(token string) (jwt.MapClaims, error) {
		return jwt.MapClaims{
			"type": "access", // ❌ Pas "refresh"
			"sub":  "user-123",
			"jti":  "jti-abc-123",
			"role": "merchant",
		}, nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()
	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrTokenTypeInvalid, err)
}

func TestRefreshUsecase_MissingSub(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	// Mock validateToken sans sub
	validateToken := func(token string) (jwt.MapClaims, error) {
		return jwt.MapClaims{
			"type": "refresh",
			// ❌ Pas de "sub"
			"jti":  "jti-abc-123",
			"role": "merchant",
		}, nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()
	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrInvalidPayload, err)
}

func TestRefreshUsecase_MissingJTI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	// Mock validateToken sans jti
	validateToken := func(token string) (jwt.MapClaims, error) {
		return jwt.MapClaims{
			"type": "refresh",
			"sub":  "user-123",
			// ❌ Pas de "jti"
			"role": "merchant",
		}, nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()
	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrTokenJTIInvalid, err)
}

// ============================================================
// TESTS : RefreshUsecase - Session validation
// ============================================================

func TestRefreshUsecase_SessionNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()

	// Mock : Session non trouvée
	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(nil, errors.New("not found"))

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrRefreshTokenNotFound, err)
}

func TestRefreshUsecase_SessionRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()

	// Session révoquée
	revokedSession := createValidSession()
	revokedSession.Revoked = true

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(revokedSession, nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrRefreshTokenRevoked, err)
}

func TestRefreshUsecase_SessionExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()

	// Session expirée
	expiredSession := createValidSession()
	expiredSession.ExpiresAt = time.Now().Add(-24 * time.Hour) // ❌ Expirée

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(expiredSession, nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrRefreshTokenExpired, err)
}

func TestRefreshUsecase_UserIDMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()

	// Session avec un UserID différent
	mismatchSession := createValidSession()
	mismatchSession.UserID = "other-user-456" // ❌ Différent du token

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(mismatchSession, nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Contains(t, err.Error(), "mismatch")
}

// ============================================================
// TESTS : RefreshUsecase - Repository errors
// ============================================================

func TestRefreshUsecase_RevokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	// Mock : Revoke échoue
	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(errors.New("database error"))

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestRefreshUsecase_CreateSessionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		nil,
		nil,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(nil)

	// ✅ Mock : RevokeSession (appelé avant Create)
	mockSessionRepo.EXPECT().
		RevokeSession(gomock.Any(), "jti-abc-123", "user-123").
		Return(nil).
		AnyTimes()

	// Mock : Create échoue
	mockRefreshRepo.EXPECT().
		Create(gomock.Any()).
		Return(errors.New("database error"))

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestRefreshUsecase_GenerateAccessError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	generateAccess := func(sub, role string) (string, error) {
		return "", errors.New("generate error")
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		generateAccess,
		nil,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(nil)

	// ✅ Mock : RevokeSession (appelé avant Create)
	mockSessionRepo.EXPECT().
		RevokeSession(gomock.Any(), "jti-abc-123", "user-123").
		Return(nil).
		AnyTimes()

	mockRefreshRepo.EXPECT().
		Create(gomock.Any()).
		Return(nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrInternalServer, err)
}

func TestRefreshUsecase_GenerateRefreshError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	generateAccess := func(sub, role string) (string, error) {
		return "new-access-token", nil
	}

	generateRefresh := func(sub, jti, role string) (string, error) {
		return "", errors.New("generate error")
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		generateAccess,
		generateRefresh,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(nil)

	// ✅ Mock : RevokeSession (appelé avant Create)
	mockSessionRepo.EXPECT().
		RevokeSession(gomock.Any(), "jti-abc-123", "user-123").
		Return(nil).
		AnyTimes()

	mockRefreshRepo.EXPECT().
		Create(gomock.Any()).
		Return(nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.Error(t, err)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	assert.Equal(t, utils.ErrInternalServer, err)
}

// ============================================================
// TESTS : RefreshUsecase - Happy paths
// ============================================================

func TestRefreshUsecase_Success_WithRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	generateAccess := func(sub, role string) (string, error) {
		assert.Equal(t, "user-123", sub)
		assert.Equal(t, "merchant", role)
		return "new-access-token", nil
	}

	generateRefresh := func(sub, jti, role string) (string, error) {
		assert.Equal(t, "user-123", sub)
		assert.Equal(t, "new-jti-456", jti)
		assert.Equal(t, "merchant", role)
		return "new-refresh-token", nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		generateAccess,
		generateRefresh,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(nil)

	// Mock : RevokeSession (non bloquant)
	mockSessionRepo.EXPECT().
		RevokeSession(gomock.Any(), "jti-abc-123", "user-123").
		Return(nil).
		AnyTimes()

	mockRefreshRepo.EXPECT().
		Create(gomock.Any()).
		DoAndReturn(func(session *authentity.RefreshSession) error {
			assert.Equal(t, "new-jti-456", session.ID)
			assert.Equal(t, "user-123", session.UserID)
			assert.False(t, session.Revoked)
			return nil
		})

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", access)
	assert.Equal(t, "new-refresh-token", refresh)
}

func TestRefreshUsecase_Success_DefaultRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)
	mockSessionRepo := mockrepo.NewMockUserSessionRepository(ctrl)

	// Token sans rôle → doit utiliser "merchant" par défaut
	validateToken := func(token string) (jwt.MapClaims, error) {
		return jwt.MapClaims{
			"type": "refresh",
			"sub":  "user-123",
			"jti":  "jti-abc-123",
			// ❌ Pas de "role"
		}, nil
	}

	generateAccess := func(sub, role string) (string, error) {
		assert.Equal(t, "merchant", role) // ✅ Default role
		return "new-access-token", nil
	}

	generateRefresh := func(sub, jti, role string) (string, error) {
		assert.Equal(t, "merchant", role)
		return "new-refresh-token", nil
	}

	uc := createRefreshUsecase(
		mockRefreshRepo,
		mockSessionRepo,
		validateToken,
		generateAccess,
		generateRefresh,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(nil)

	mockSessionRepo.EXPECT().
		RevokeSession(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	mockRefreshRepo.EXPECT().
		Create(gomock.Any()).
		Return(nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", access)
	assert.Equal(t, "new-refresh-token", refresh)
}

func TestRefreshUsecase_Success_NilSessionRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefreshRepo := mockauthrepo.NewMockRefreshSessionRepository(ctrl)

	validateToken := func(token string) (jwt.MapClaims, error) {
		return createValidClaims(), nil
	}

	generateAccess := func(sub, role string) (string, error) {
		return "new-access-token", nil
	}

	generateRefresh := func(sub, jti, role string) (string, error) {
		return "new-refresh-token", nil
	}

	// ✅ Passer nil pour sessionRepo
	uc := authusecase.NewRefreshUsecase(
		mockRefreshRepo,
		nil, // ❌ SessionRepo nil
		validateToken,
		generateAccess,
		generateRefresh,
		func() time.Time { return time.Now() },
		func() string { return "new-jti-456" },
		7*24*time.Hour,
	)

	ctx := context.Background()

	validSession := createValidSession()

	mockRefreshRepo.EXPECT().
		FindByID("jti-abc-123").
		Return(validSession, nil)

	mockRefreshRepo.EXPECT().
		Revoke("jti-abc-123").
		Return(nil)

	// ❌ Pas d'appel à RevokeSession car sessionRepo est nil

	mockRefreshRepo.EXPECT().
		Create(gomock.Any()).
		Return(nil)

	access, refresh, err := uc.Execute(ctx, "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", access)
	assert.Equal(t, "new-refresh-token", refresh)
}
