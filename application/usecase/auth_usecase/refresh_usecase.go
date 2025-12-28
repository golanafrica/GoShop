// application/usecase/auth_usecase/refresh_usecase.go
package authusecase

import (
	"context"
	"errors"
	"time"

	"Goshop/config/setupLogging"
	authentity "Goshop/domain/auth_entity"
	authrepository "Goshop/domain/repository/auth_repository"
	"Goshop/interfaces/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
)

type RefreshUsecase struct {
	repo               authrepository.RefreshSessionRepository
	validateToken      func(string) (jwt.MapClaims, error)
	generateAccess     func(string) (string, error)
	generateRefresh    func(string, string) (string, error)
	now                func() time.Time
	newJTI             func() string
	refreshExpiryDelta time.Duration
	logger             *setupLogging.Logger // ✅ Wrapper personnalisé
}

func NewRefreshUsecase(
	repo authrepository.RefreshSessionRepository,
	validateToken func(string) (jwt.MapClaims, error),
	generateAccess func(string) (string, error),
	generateRefresh func(string, string) (string, error),
	now func() time.Time,
	newJTI func() string,
	refreshExpiry time.Duration,
	//logger *setupLogging.Logger, // ✅ Type corrigé
) *RefreshUsecase {
	return &RefreshUsecase{
		repo:               repo,
		validateToken:      validateToken,
		generateAccess:     generateAccess,
		generateRefresh:    generateRefresh,
		now:                now,
		newJTI:             newJTI,
		refreshExpiryDelta: refreshExpiry,
		//logger:             logger.WithComponent("refresh_usecase"), // ✅ Cohérent
	}
}

func (uc *RefreshUsecase) Execute(ctx context.Context, oldRefreshToken string) (string, string, error) {
	start := time.Now()

	logger := zerolog.Ctx(ctx)

	// ✅ Pas de .With().Logger() → on reste dans *setupLogging.Logger
	// On ajoute les champs directement dans chaque log
	logger.Info().
		Str("operation", "execute").
		Int("token_length", len(oldRefreshToken)).
		Msg("Starting refresh token process")

	// 1. Parse & validate refresh token
	logger.Debug().Msg("Validating refresh token")
	claims, err := uc.validateToken(oldRefreshToken)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("error_type", "invalid_token").
			Msg("Refresh token validation failed")
		return "", "", utils.ErrRefreshTokenInvalid
	}

	// 2. Vérifie le type
	t, ok := claims["type"].(string)
	if !ok || t != "refresh" {
		logger.Warn().
			Str("token_type", t).
			Str("expected_type", "refresh").
			Msg("Invalid token type")
		return "", "", utils.ErrTokenTypeInvalid
	}

	// 3. Récupère sub (userID)
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		logger.Warn().
			Interface("claims", claims).
			Msg("Missing or invalid sub claim in token")
		return "", "", utils.ErrInvalidPayload
	}

	// 4. Récupère jti
	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		logger.Warn().
			Interface("claims", claims).
			Msg("Missing or invalid jti claim in token")
		return "", "", utils.ErrTokenJTIInvalid
	}

	// Masque les données sensibles pour les logs suivants
	maskedUserID := maskUserID(sub)
	maskedJTI := maskJTI(jti)

	logger.Debug().
		Str("user_id", maskedUserID).
		Str("jti", maskedJTI).
		Msg("Token claims validated successfully")

	// 5. Charge la session
	logger.Debug().Msg("Loading refresh session from repository")
	session, err := uc.repo.FindByID(jti)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("error_type", "session_not_found").
			Msg("Refresh session not found")
		return "", "", utils.ErrRefreshTokenNotFound
	}

	now := uc.now()

	// 6. Vérifications de session
	if session.Revoked {
		logger.Warn().
			Time("revoked_at", session.CreatedAt).
			Msg("Refresh token already revoked")
		return "", "", utils.ErrRefreshTokenRevoked
	}

	if session.ExpiresAt.Before(now) {
		logger.Warn().
			Time("expires_at", session.ExpiresAt).
			Time("current_time", now).
			Msg("Refresh token expired")
		return "", "", utils.ErrRefreshTokenExpired
	}

	if session.UserID != sub {
		logger.Warn().
			Str("session_user_id", session.UserID).
			Str("token_user_id", sub).
			Msg("User ID mismatch between session and token")
		return "", "", errors.New("refresh token subject mismatch")
	}

	logger.Debug().
		Time("session_created", session.CreatedAt).
		Time("session_expires", session.ExpiresAt).
		Bool("session_revoked", session.Revoked).
		Msg("Session validation passed")

	// 7. Révoque l'ancien token
	logger.Debug().Msg("Revoking old refresh token")
	if err := uc.repo.Revoke(jti); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Msg("Failed to revoke refresh token")
		return "", "", utils.ErrInternalServer
	}

	logger.Info().Msg("Old refresh token revoked successfully")

	// 8. Crée la nouvelle session
	newJti := uc.newJTI()
	expiresAt := now.Add(uc.refreshExpiryDelta)

	newSession := &authentity.RefreshSession{
		ID:        newJti,
		UserID:    sub,
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: now,
	}

	maskedNewJTI := maskJTI(newJti)
	logger.Debug().
		Str("new_jti", maskedNewJTI).
		Time("new_expires_at", expiresAt).
		Msg("Creating new refresh session")

	if err := uc.repo.Create(newSession); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Msg("Failed to create new refresh session")
		return "", "", utils.ErrInternalServer
	}

	logger.Debug().Msg("New refresh session created successfully")

	// 9. Génère les tokens
	logger.Debug().Msg("Generating new access token")
	access, err := uc.generateAccess(sub)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Msg("Failed to generate access token")
		return "", "", utils.ErrInternalServer
	}

	logger.Debug().Msg("Generating new refresh token")
	refresh, err := uc.generateRefresh(sub, newJti)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Msg("Failed to generate refresh token")
		return "", "", utils.ErrInternalServer
	}

	logger.Info().
		Str("old_jti", maskedJTI).
		Str("new_jti", maskedNewJTI).
		Dur("duration_ms", time.Since(start)).
		Int("access_token_length", len(access)).
		Int("refresh_token_length", len(refresh)).
		Msg("Token refresh completed successfully")

	return access, refresh, nil
}

// Helpers de masquage (inchangés)
func maskUserID(userID string) string {
	if len(userID) <= 8 {
		return userID
	}
	return userID[:4] + "..." + userID[len(userID)-4:]
}

func maskJTI(jti string) string {
	if len(jti) <= 8 {
		return jti
	}
	return jti[:4] + "..." + jti[len(jti)-4:]
}
