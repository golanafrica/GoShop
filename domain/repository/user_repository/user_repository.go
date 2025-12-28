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

type UserRepository interface {
	CreateUser(user *userentity.UserEntity) (*userentity.UserEntity, error)
	FindUserByEmail(email string) (*userentity.UserEntity, error)
	FindUserByID(id string) (*userentity.UserEntity, error)
}
