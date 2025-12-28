// application/usecase/user_usecase/login_usecase.go
package userusecase

import (
	"context"
	"strings"
	"time"

	"Goshop/application/metrics" // ← AJOUTÉ
	"Goshop/config/setupLogging"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"
)

type LoginUsecase struct {
	repo          userrepository.UserRepository
	generateToken func(string) (string, error)
	//logger        *setupLogging.Logger
}

func NewLoginUsecase(repo userrepository.UserRepository, logger *setupLogging.Logger) *LoginUsecase {
	return &LoginUsecase{
		repo:          repo,
		generateToken: utils.GenerateAccessToken,
		//logger:        logger.WithComponent("login_usecase"),
	}
}

// Helper local (inchangé)
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

func (uc *LoginUsecase) Execute(ctx context.Context, email, password string) (string, error) {
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

			// ✅ Incrémenter métrique d'échec
			metrics.AuthLoginFailedTotal.Inc()

			return "", utils.ErrInvalidCredentials
		}

		logger.Error().
			Err(err).
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("error_type", "database_error").
			Str("database_operation", "FindUserByEmail").
			Dur("duration_ms", time.Since(start)).
			Msg("❌ Erreur base de données lors de la recherche utilisateur")

		// ✅ Incrémenter métrique d'échec (erreur système)
		metrics.AuthLoginFailedTotal.Inc()

		return "", utils.ErrInternalServer
	}

	maskedUserID := maskUsersID(user.ID)
	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Dur("find_user_duration_ms", time.Since(start)).
		Msg("✅ Utilisateur trouvé en base")

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

		// ✅ Incrémenter métrique d'échec
		metrics.AuthLoginFailedTotal.Inc()

		return "", utils.ErrInvalidCredentials
	}

	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Dur("password_check_duration_ms", time.Since(passwordStart)).
		Msg("✅ Mot de passe validé")

	// 3. Génération du token
	tokenStart := time.Now()
	logger.Debug().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Msg("🔄 Génération token JWT")

	token, err := uc.generateToken(user.ID)
	if err != nil {
		logger.Error().
			Err(err).
			Str("operation", "login").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("error_type", "token_generation_error").
			Dur("token_gen_duration_ms", time.Since(tokenStart)).
			Dur("total_duration_ms", time.Since(start)).
			Msg("❌ Erreur génération token")

		// ✅ Incrémenter métrique d'échec (erreur système)
		metrics.AuthLoginFailedTotal.Inc()

		return "", utils.ErrInternalServer
	}

	logger.Info().
		Str("operation", "login").
		Str("email", maskedEmail).
		Str("user_id", maskedUserID).
		Int("token_length", len(token)).
		Dur("token_gen_duration_ms", time.Since(tokenStart)).
		Dur("total_duration_ms", time.Since(start)).
		Msg("✅ Authentification réussie, token généré")

	// ✅ Incrémenter métrique de succès
	metrics.AuthLoginTotal.Inc()

	return token, nil
}
