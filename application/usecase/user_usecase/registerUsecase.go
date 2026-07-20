package userusecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"Goshop/application/metrics"
	"Goshop/config/setupLogging"
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"
)

type RegisterUsecase struct {
	repo userrepository.UserRepository
}

func NewRegisterUsecase(repo userrepository.UserRepository, logger *setupLogging.Logger) *RegisterUsecase {
	return &RegisterUsecase{
		repo: repo,
	}
}

func maskEmail(email string) string {
	if len(email) > 3 && len(email) < 100 {
		return email[:3] + "***@" + email[strings.Index(email, "@")+1:]
	}
	return "***@***"
}

func maskUserID(userID string) string {
	if len(userID) > 8 {
		return userID[:4] + "..." + userID[len(userID)-4:]
	}
	return userID
}

func (uc *RegisterUsecase) Execute(ctx context.Context, email, password string) (*userentity.UserEntity, error) {
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	maskedEmail := maskEmail(email)

	logger.Info().
		Str("operation", "register").
		Str("email", maskedEmail).
		Msg("🚀 Début création utilisateur")

	// 1. Vérifier si l'utilisateur existe déjà
	_, err := uc.repo.FindUserByEmail(email)
	if err == nil {
		logger.Warn().
			Str("operation", "register").
			Str("email", maskedEmail).
			Str("error_type", "user_already_exists").
			Dur("duration_ms", time.Since(start)).
			Msg("❌ Utilisateur existe déjà")
		return nil, utils.ErrUserAlreadyExists
	}

	if err != userrepository.ErrUserNotFound {
		logger.Error().
			Err(err).
			Str("operation", "register").
			Str("email", maskedEmail).
			Str("error_type", "database_error").
			Dur("duration_ms", time.Since(start)).
			Msg("❌ Erreur base de données lors de la vérification email")
		return nil, utils.ErrInternalServer
	}

	// 2. Hash du mot de passe
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().
			Err(err).
			Str("operation", "register").
			Str("email", maskedEmail).
			Str("error_type", "hash_error").
			Dur("duration_ms", time.Since(start)).
			Msg("❌ Erreur lors du hash du mot de passe")
		return nil, utils.ErrInternalServer
	}

	// 3. Création de l'entité
	user := &userentity.UserEntity{
		ID:                  uuid.NewString(),
		Email:               email,
		Password:            string(hashed),
		Role:                "user", // ✅ CORRIGÉ : Était "merchant". Un utilisateur qui s'inscrit publiquement est un client par défaut.
		Active:              true,
		Status:              "active",
		FailedLoginAttempts: 0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	maskedUserID := maskUserID(user.ID)

	// 4. Sauvegarde en base
	created, err := uc.repo.CreateUser(user)
	if err != nil {
		if err == userrepository.ErrUserAlreadyExists {
			logger.Warn().
				Str("operation", "register").
				Str("email", maskedEmail).
				Str("error_type", "user_already_exists_db").
				Dur("duration_ms", time.Since(start)).
				Msg("❌ Conflit: utilisateur existe déjà (race condition)")
			return nil, utils.ErrUserAlreadyExists
		}

		logger.Error().
			Err(err).
			Str("operation", "register").
			Str("email", maskedEmail).
			Str("user_id", maskedUserID).
			Str("error_type", "database_error").
			Dur("duration_ms", time.Since(start)).
			Msg("❌ Erreur base de données lors de la création")
		return nil, utils.ErrInternalServer
	}

	logger.Info().
		Str("operation", "register").
		Str("email", maskedEmail).
		Str("user_id", maskUserID(created.ID)).
		Str("role", created.Role).
		Dur("duration_ms", time.Since(start)).
		Msg("🎉 Utilisateur créé avec succès")

	metrics.AuthRegisterTotal.Inc()

	return created, nil
}
