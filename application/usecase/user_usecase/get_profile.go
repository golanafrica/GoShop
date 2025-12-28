// application/usecase/user_usecase/get_profile.go
package userusecase

import (
	"context"
	"time"

	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type GetProfileUsecase struct {
	repo userrepository.UserRepository
	//logger *setupLogging.Logger
}

func NewGetProfileUsecase(repo userrepository.UserRepository) *GetProfileUsecase {
	return &GetProfileUsecase{
		repo: repo,
		//logger: logger.WithComponent("get_profile_usecase"),
	}
}

func (uc *GetProfileUsecase) Execute(userID string) (*userentity.UserEntity, error) {
	// Version sans contexte (pour compatibilité)
	return uc.ExecuteWithContext(context.Background(), userID)
}

func (uc *GetProfileUsecase) ExecuteWithContext(ctx context.Context, userID string) (*userentity.UserEntity, error) {
	logger := zerolog.Ctx(ctx)
	start := time.Now()

	// ✅ Récupère le logger du contexte (toujours présent grâce à ton middleware)
	//loggerFromCtx := setupLogging.FromContext(ctx)
	//logger := loggerFromCtx.WithOperation("get_profile")

	logger.Info().
		Str("user_id", utils.SecureLogUserID(userID)).
		Msg("Début récupération profil utilisateur")

	// Validation basique
	if userID == "" {
		logger.Warn().
			Str("error_type", "empty_user_id").
			Msg("UserID vide reçu")
		return nil, utils.ErrInvalidCredentials
	}

	logger.Debug().Msg("Appel repository FindUserByID")

	// Appel au repository
	user, err := uc.repo.FindUserByID(userID)
	if err != nil {
		if err == userrepository.ErrUserNotFound {
			logger.Warn().
				Str("error_type", "user_not_found").
				Dur("duration_ms", time.Since(start)).
				Msg("Utilisateur non trouvé en base")
			return nil, utils.ErrUserNotFound
		}

		logger.Error().
			Err(err).
			Str("error_type", "database_error").
			Str("database_operation", "FindUserByID").
			Dur("duration_ms", time.Since(start)).
			Msg("Erreur base de données lors de la récupération utilisateur")
		return nil, utils.ErrInternalServer
	}

	if user == nil {
		logger.Error().
			Str("error_type", "nil_user").
			Dur("duration_ms", time.Since(start)).
			Msg("Repository a retourné un utilisateur nil")
		return nil, utils.ErrInternalServer
	}

	logger.Info().
		Str("user_email", utils.SecureLogEmail(user.Email)).
		Dur("duration_ms", time.Since(start)).
		Msg("Profil utilisateur récupéré avec succès")

	return user, nil
}
