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

// ============================================================
// CREATE USER
// ============================================================

func (ur *UserPostgres) CreateUser(user *userentity.UserEntity) (*userentity.UserEntity, error) {
	query := `
        INSERT INTO users (id, email, password, role, is_active, status)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, email, password, role, is_active, status
    `
	row := ur.db.QueryRow(
		query,
		user.ID,
		user.Email,
		user.Password,
		user.Role,
		user.Active,
		user.Status,
	)

	var out userentity.UserEntity
	err := row.Scan(
		&out.ID,
		&out.Email,
		&out.Password,
		&out.Role,
		&out.Active,
		&out.Status,
	)
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

// ============================================================
// FIND USER BY EMAIL
// ============================================================

func (ur *UserPostgres) FindUserByEmail(email string) (*userentity.UserEntity, error) {
	query := `
        SELECT 
            id, email, password, 
            COALESCE(role, 'merchant') as role,
            COALESCE(is_active, true) as is_active,
            COALESCE(status, 'active') as status,
            failed_login_attempts,
            locked_until,
            last_login_at,
            created_at,
            updated_at
        FROM users
        WHERE email = $1
    `
	row := ur.db.QueryRow(query, email)

	var out userentity.UserEntity
	var lockedUntil, lastLoginAt sql.NullTime

	err := row.Scan(
		&out.ID,
		&out.Email,
		&out.Password,
		&out.Role,
		&out.Active,
		&out.Status,
		&out.FailedLoginAttempts,
		&lockedUntil,
		&lastLoginAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, userrepository.ErrUserNotFound
		}
		return nil, err
	}

	// Gestion des champs nullable
	if lockedUntil.Valid {
		out.LockedUntil = &lockedUntil.Time
	}
	if lastLoginAt.Valid {
		out.LastLoginAt = &lastLoginAt.Time
	}

	return &out, nil
}

// ============================================================
// FIND USER BY ID
// ============================================================

func (ur *UserPostgres) FindUserByID(id string) (*userentity.UserEntity, error) {
	query := `
        SELECT 
            id, email, password, 
            COALESCE(role, 'merchant') as role,
            COALESCE(is_active, true) as is_active,
            COALESCE(status, 'active') as status,
            failed_login_attempts,
            locked_until,
            last_login_at,
            created_at,
            updated_at
        FROM users
        WHERE id = $1
    `
	row := ur.db.QueryRow(query, id)

	var out userentity.UserEntity
	var lockedUntil, lastLoginAt sql.NullTime

	err := row.Scan(
		&out.ID,
		&out.Email,
		&out.Password,
		&out.Role,
		&out.Active,
		&out.Status,
		&out.FailedLoginAttempts,
		&lockedUntil,
		&lastLoginAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, userrepository.ErrUserNotFound
		}
		return nil, err
	}

	// Gestion des champs nullable
	if lockedUntil.Valid {
		out.LockedUntil = &lockedUntil.Time
	}
	if lastLoginAt.Valid {
		out.LastLoginAt = &lastLoginAt.Time
	}

	return &out, nil
}

// ============================================================
// 🆕 v4.0.0 : MÉTHODES RBAC
// ============================================================

// UpdateLastLogin met à jour la dernière connexion
func (ur *UserPostgres) UpdateLastLogin(id string) error {
	query := `
        UPDATE users 
        SET last_login_at = NOW(), 
            failed_login_attempts = 0,
            locked_until = NULL,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := ur.db.Exec(query, id)
	return err
}

// RecordFailedLogin enregistre une tentative échouée
func (ur *UserPostgres) RecordFailedLogin(id string, attempts int, lockedUntil *string) error {
	query := `
        UPDATE users 
        SET failed_login_attempts = $2,
            locked_until = $3,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := ur.db.Exec(query, id, attempts, lockedUntil)
	return err
}

// UpdateRole change le rôle d'un utilisateur
func (ur *UserPostgres) UpdateRole(id, role, updatedBy string) error {
	query := `
        UPDATE users 
        SET role = $2,
            updated_by = $3,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := ur.db.Exec(query, id, role, updatedBy)
	return err
}

// UpdateStatus change le statut d'un utilisateur
func (ur *UserPostgres) UpdateStatus(id, status, updatedBy string) error {
	query := `
        UPDATE users 
        SET status = $2,
            is_active = CASE WHEN $2 = 'active' THEN true ELSE false END,
            updated_by = $3,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := ur.db.Exec(query, id, status, updatedBy)
	return err
}

// ListUsersByRole liste les utilisateurs par rôle (admin)
func (ur *UserPostgres) ListUsersByRole(role string, limit, offset int) ([]*userentity.UserEntity, error) {
	query := `
        SELECT 
            id, email, password, 
            COALESCE(role, 'merchant') as role,
            COALESCE(is_active, true) as is_active,
            COALESCE(status, 'active') as status,
            failed_login_attempts,
            locked_until,
            last_login_at,
            created_at,
            updated_at
        FROM users
        WHERE role = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := ur.db.Query(query, role, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*userentity.UserEntity
	for rows.Next() {
		var out userentity.UserEntity
		var lockedUntil, lastLoginAt sql.NullTime

		err := rows.Scan(
			&out.ID,
			&out.Email,
			&out.Password,
			&out.Role,
			&out.Active,
			&out.Status,
			&out.FailedLoginAttempts,
			&lockedUntil,
			&lastLoginAt,
			&out.CreatedAt,
			&out.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if lockedUntil.Valid {
			out.LockedUntil = &lockedUntil.Time
		}
		if lastLoginAt.Valid {
			out.LastLoginAt = &lastLoginAt.Time
		}

		users = append(users, &out)
	}

	return users, rows.Err()
}

// CountUsersByRole compte les utilisateurs par rôle
func (ur *UserPostgres) CountUsersByRole(role string) (int, error) {
	query := `SELECT COUNT(*) FROM users WHERE role = $1`
	var count int
	err := ur.db.QueryRow(query, role).Scan(&count)
	return count, err
}
