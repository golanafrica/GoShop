// application/usecase/user_usecase/login_usecase.go
package userusecase

import (
	"context"
	"strings"
	"time"

	"Goshop/application/metrics"
	"Goshop/config/setupLogging"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"
)

// ============================================================
// 🆕 v4.0.0 : Types des générateurs de token mis à jour
// ============================================================

type generateAccessFunc func(userID, role string) (string, error)
type generateRefreshFunc func(userID, jti, role string) (string, error)

type LoginUsecase struct {
	repo            userrepository.UserRepository
	generateAccess  generateAccessFunc
	generateRefresh generateRefreshFunc
}

func NewLoginUsecase(repo userrepository.UserRepository, logger *setupLogging.Logger) *LoginUsecase {
	return &LoginUsecase{
		repo:            repo,
		generateAccess:  utils.GenerateAccessToken,  // 🆕 v4.0.0
		generateRefresh: utils.GenerateRefreshToken, // 🆕 v4.0.0
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
// EXECUTE - Retourne maintenant (accessToken, refreshToken, error)
// ============================================================

func (uc *LoginUsecase) Execute(ctx context.Context, email, password string) (string, string, error) {
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	maskedEmail := maskEmails(email)
	logger.Info().
		Str("operation", "login").
		Str("email", maskedEmail).
		Msg("🔐 Début authentification utilisateur")

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

		// 🆕 v4.0.0 : Enregistrer la tentative échouée
		user.RecordFailedLogin()
		// TODO: Sauvegarder en DB via repo.RecordFailedLogin()

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
	// TODO: Sauvegarder en DB via repo.UpdateLastLogin()

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

	// 🆕 v4.0.0 : Passer le rôle aux générateurs de token
	accessToken, err := uc.generateAccess(user.ID, user.Role)
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

	metrics.AuthLoginTotal.Inc()

	return accessToken, refreshToken, nil
}
