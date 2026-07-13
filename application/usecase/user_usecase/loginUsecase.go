// application/usecase/user_usecase/login_usecase.go
package userusecase

import (
	"context"
	"strings"
	"time"

	"Goshop/application/metrics"
	"Goshop/config/setupLogging"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/domain/service"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// 🆕 v4.0.0 : Types des générateurs de token mis à jour
// 🆕 v4.4.2 : + Intégration Session Management
// 🆕 v4.4.21 : + Rate Limiting pour le login
// ============================================================

type generateAccessFunc func(userID, role string) (string, error)
type generateRefreshFunc func(userID, jti, role string) (string, error)

type LoginUsecase struct {
	repo            userrepository.UserRepository
	sessionRepo     repository.UserSessionRepository // 🆕 v4.4.2
	rateLimiter     service.LoginRateLimiter         // 🆕 v4.4.21
	generateAccess  generateAccessFunc
	generateRefresh generateRefreshFunc
}

// 🆕 v4.4.2 : Nouveau constructeur avec sessionRepo et rateLimiter
func NewLoginUsecase(
	repo userrepository.UserRepository,
	sessionRepo repository.UserSessionRepository, // 🆕 v4.4.2
	rateLimiter service.LoginRateLimiter, // 🆕 v4.4.21
	logger *setupLogging.Logger,
) *LoginUsecase {
	return &LoginUsecase{
		repo:            repo,
		sessionRepo:     sessionRepo,
		rateLimiter:     rateLimiter, // 🆕
		generateAccess:  utils.GenerateAccessToken,
		generateRefresh: utils.GenerateRefreshToken,
	}
}

// ============================================================
// HELPERS
// ============================================================

func maskEmails(e string) string {
	if e == "" {
		return ""
	}
	parts := strings.Split(e, "@")
	if len(parts) != 2 {
		return "invalid_email"
	}
	localPart := parts[0]
	domain := parts[1]
	if len(localPart) > 3 {
		return localPart[:3] + "***@" + domain
	}
	return localPart + "***@" + domain
}

func maskUsersID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:4] + "..." + id[len(id)-4:]
}

// ============================================================
// 🆕 v4.4.2 : ExecuteWithContext - Accepte IP et UserAgent
// ============================================================

func (uc *LoginUsecase) Execute(ctx context.Context, email, password string) (string, string, error) {
	return uc.ExecuteWithContext(ctx, email, password, "", "")
}

// 🆕 v4.4.2 : Nouvelle méthode avec contexte complet
func (uc *LoginUsecase) ExecuteWithContext(
	ctx context.Context,
	email, password string,
	ipAddress, userAgent string,
) (string, string, error) {
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	maskedEmail := maskEmails(email)
	logger.Info().
		Str("operation", "login").
		Str("email", maskedEmail).
		Msg("🔐 Début authentification utilisateur")

	// 🆕 v4.4.21 : ÉTAPE 0 - Vérifier le rate limiting
	if uc.rateLimiter != nil {
		if err := uc.rateLimiter.CheckEmailLimit(ctx, email); err != nil {
			logger.Warn().
				Str("operation", "login").
				Str("email", maskedEmail).
				Str("error_type", "email_rate_limit").
				Msg("⛔ Email rate limit exceeded")
			return "", "", utils.ErrTooManyAttempts
		}

		if err := uc.rateLimiter.CheckIPLimit(ctx, ipAddress); err != nil {
			logger.Warn().
				Str("operation", "login").
				Str("ip", ipAddress).
				Str("error_type", "ip_rate_limit").
				Msg("⛔ IP rate limit exceeded")
			return "", "", utils.ErrTooManyAttempts
		}
	}

	// 1. Recherche de l'utilisateur
	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Msg("🔍 Recherche utilisateur par email")

	user, err := uc.repo.FindUserByEmail(email)
	if err != nil {
		if err == userrepository.ErrUserNotFound {
			logger.Warn().
				Str("operation", "login").
				Str("email", maskedEmail).
				Str("error_type", "user_not_found").
				Dur("duration_ms", time.Since(start)).
				Msg("❌ Utilisateur non trouvé")

			// 🆕 v4.4.21 : Enregistrer la tentative échouée
			if uc.rateLimiter != nil {
				_ = uc.rateLimiter.RecordFailedAttempt(ctx, email, ipAddress)
			}

			metrics.AuthLoginFailedTotal.Inc()
			return "", "", utils.ErrInvalidCredentials
		}

		logger.Error().
			Err(err).
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("error_type", "database_error").
			Str("database_operation", "FindUserByEmail").
			Dur("duration_ms", time.Since(start)).
			Msg("❌ Erreur base de données lors de la recherche utilisateur")

		metrics.AuthLoginFailedTotal.Inc()
		return "", "", utils.ErrInternalServer
	}

	maskedUserID := maskUsersID(user.ID)
	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Dur("find_user_duration_ms", time.Since(start)).
		Msg("✅ Utilisateur trouvé en base")

	// ============================================================
	// 🆕 v4.0.0 : Vérification du statut utilisateur
	// ============================================================
	if err := user.CanLogin(); err != nil {
		logger.Warn().
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("status", user.Status).
			Str("error_type", "account_not_allowed").
			Msg("❌ Compte non autorisé à se connecter")

		// 🆕 v4.4.21 : Enregistrer la tentative échouée
		if uc.rateLimiter != nil {
			_ = uc.rateLimiter.RecordFailedAttempt(ctx, email, ipAddress)
		}

		metrics.AuthLoginFailedTotal.Inc()
		return "", "", utils.ErrInvalidCredentials
	}

	// ============================================================
	// 🆕 v4.0.0 : Vérification du verrouillage
	// ============================================================
	if user.IsLocked() {
		logger.Warn().
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("error_type", "account_locked").
			Time("locked_until", *user.LockedUntil).
			Msg("❌ Compte temporairement verrouillé")

		// 🆕 v4.4.21 : Enregistrer la tentative échouée
		if uc.rateLimiter != nil {
			_ = uc.rateLimiter.RecordFailedAttempt(ctx, email, ipAddress)
		}

		metrics.AuthLoginFailedTotal.Inc()
		return "", "", utils.ErrInvalidCredentials
	}

	// 2. Vérification du mot de passe
	passwordStart := time.Now()
	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Msg("🔒 Vérification hash mot de passe")

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		logger.Warn().
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("error_type", "invalid_password").
			Dur("password_check_duration_ms", time.Since(passwordStart)).
			Dur("total_duration_ms", time.Since(start)).
			Msg("❌ Mot de passe incorrect")

		user.RecordFailedLogin()

		// 🆕 v4.4.21 : Enregistrer la tentative échouée
		if uc.rateLimiter != nil {
			_ = uc.rateLimiter.RecordFailedAttempt(ctx, email, ipAddress)
		}

		metrics.AuthLoginFailedTotal.Inc()
		return "", "", utils.ErrInvalidCredentials
	}

	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Dur("password_check_duration_ms", time.Since(passwordStart)).
		Msg("✅ Mot de passe validé")

	// ============================================================
	// 🆕 v4.0.0 : Enregistrer la connexion réussie
	// ============================================================
	user.RecordSuccessfulLogin()

	// 3. Génération du refresh token (JTI)
	tokenStart := time.Now()
	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Str("role", user.Role).
		Msg("🔄 Génération tokens JWT")

	// 🆕 v4.0.0 : Générer un JTI pour le refresh token
	jti := utils.GenerateUUID()

	// ============================================================
	// 🆕 v4.4.2 : Générer l'access token AVEC le jti (session_id)
	// ============================================================
	accessToken, err := utils.GenerateAccessTokenWithSession(user.ID, user.Role, jti)
	if err != nil {
		logger.Error().
			Err(err).
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("error_type", "token_generation_error").
			Dur("token_gen_duration_ms", time.Since(tokenStart)).
			Dur("total_duration_ms", time.Since(start)).
			Msg("❌ Erreur génération access token")

		metrics.AuthLoginFailedTotal.Inc()
		return "", "", utils.ErrInternalServer
	}

	refreshToken, err := uc.generateRefresh(user.ID, jti, user.Role)
	if err != nil {
		logger.Error().
			Err(err).
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("error_type", "refresh_token_generation_error").
			Dur("token_gen_duration_ms", time.Since(tokenStart)).
			Dur("total_duration_ms", time.Since(start)).
			Msg("❌ Erreur génération refresh token")

		metrics.AuthLoginFailedTotal.Inc()
		return "", "", utils.ErrInternalServer
	}

	logger.Info().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Str("role", user.Role).
		Int("access_token_length", len(accessToken)).
		Int("refresh_token_length", len(refreshToken)).
		Dur("token_gen_duration_ms", time.Since(tokenStart)).
		Dur("total_duration_ms", time.Since(start)).
		Msg("✅ Authentification réussie, tokens générés")

	// ============================================================
	// 🆕 v4.4.2 : CRÉATION DE LA SESSION
	// ============================================================
	if uc.sessionRepo != nil {
		sessionStart := time.Now()

		// Le session_id est le même que le jti du refresh token
		accessSessionID := jti

		session, err := entity.NewUserSession(
			user.ID,
			accessSessionID,
			accessToken, // Le token complet sera hashé dans NewUserSession
			ipAddress,
			userAgent,
			entity.DefaultSessionDuration,
		)
		if err != nil {
			logger.Error().
				Err(err).
				Str("user_id", user.ID).
				Msg("❌ Erreur création session")
			// Non bloquant : on continue même si la session échoue
		} else {
			if err := uc.sessionRepo.Create(ctx, session); err != nil {
				logger.Error().
					Err(err).
					Str("user_id", user.ID).
					Str("session_id", accessSessionID).
					Msg("❌ Erreur sauvegarde session")
				// Non bloquant
			} else {
				logger.Info().
					Str("user_id", user.ID).
					Str("session_id", accessSessionID).
					Str("ip_address", ipAddress).
					Str("browser", session.DeviceInfo.Browser).
					Str("os", session.DeviceInfo.OS).
					Dur("session_creation_ms", time.Since(sessionStart)).
					Msg("✅ Session créée avec succès")
			}
		}
	}

	// 🆕 v4.4.21 : Réinitialiser le rate limiter après une connexion réussie
	if uc.rateLimiter != nil {
		_ = uc.rateLimiter.ResetOnSuccess(ctx, email, ipAddress)
	}

	metrics.AuthLoginTotal.Inc()

	return accessToken, refreshToken, nil
}
