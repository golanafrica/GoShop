package userrepository

import (
	userentity "Goshop/domain/entity/user_entity"
	"errors"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserCreateFailed  = errors.New("failed to create user")
)

//go:generate mockgen -destination=../../../mocks/repository/mock_user_repository.go -package=repository -source=user_repository.go UserRepository

// UserRepository définit les opérations sur les utilisateurs
type UserRepository interface {
	// Create crée un nouvel utilisateur
	CreateUser(user *userentity.UserEntity) (*userentity.UserEntity, error)

	// FindUserByEmail retourne un utilisateur par son email
	FindUserByEmail(email string) (*userentity.UserEntity, error)

	// FindUserByID retourne un utilisateur par son ID
	FindUserByID(id string) (*userentity.UserEntity, error)

	// UpdateLastLogin met à jour la dernière connexion
	UpdateLastLogin(id string) error

	// RecordFailedLogin enregistre une tentative échouée
	RecordFailedLogin(id string, attempts int, lockedUntil *string) error

	// UpdateRole change le rôle d'un utilisateur
	UpdateRole(id, role, updatedBy string) error

	// UpdateStatus change le statut d'un utilisateur
	UpdateStatus(id, status, updatedBy string) error

	// ListUsersByRole liste les utilisateurs par rôle (admin)
	ListUsersByRole(role string, limit, offset int) ([]*userentity.UserEntity, error)

	// CountUsersByRole compte les utilisateurs par rôle
	CountUsersByRole(role string) (int, error)
}
