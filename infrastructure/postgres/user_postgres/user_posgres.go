package userpostgres

import (
	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	"database/sql"
	"errors"
	"strings"
)

type UserPostgres struct {
	db *sql.DB
}

func NewUserPostgres(db *sql.DB) userrepository.UserRepository {
	return &UserPostgres{db: db}
}

func (ur *UserPostgres) CreateUser(user *userentity.UserEntity) (*userentity.UserEntity, error) {

	query := `
        INSERT INTO users (id, email, password)
        VALUES ($1, $2, $3)
        RETURNING id, email, password
    `
	row := ur.db.QueryRow(query, user.ID, user.Email, user.Password)

	var out userentity.UserEntity
	err := row.Scan(&out.ID, &out.Email, &out.Password)
	if err != nil {

		// email déjà utilisé → contrainte UNIQUE
		if strings.Contains(err.Error(), "users_email_key") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, userrepository.ErrUserAlreadyExists
		}

		return nil, err
	}

	return &out, nil
}

func (ur *UserPostgres) FindUserByEmail(email string) (*userentity.UserEntity, error) {

	query := `
        SELECT id, email, password
        FROM users
        WHERE email = $1
    `
	row := ur.db.QueryRow(query, email)

	var out userentity.UserEntity
	err := row.Scan(&out.ID, &out.Email, &out.Password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, userrepository.ErrUserNotFound
		}
		return nil, err
	}

	return &out, nil
}

func (ur *UserPostgres) FindUserByID(id string) (*userentity.UserEntity, error) {

	query := `
        SELECT id, email, password
        FROM users
        WHERE id = $1
    `
	row := ur.db.QueryRow(query, id)

	var out userentity.UserEntity
	err := row.Scan(&out.ID, &out.Email, &out.Password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, userrepository.ErrUserNotFound
		}
		return nil, err
	}

	return &out, nil
}
