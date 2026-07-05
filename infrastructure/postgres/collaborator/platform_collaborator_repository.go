package collaborator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

// ============================================================
// 🆕 v4.3.0 : PLATFORM COLLABORATOR REPOSITORY
// ============================================================

type PlatformCollaboratorRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewPlatformCollaboratorRepositoryInfrastructure(db *sql.DB) repository.PlatformCollaboratorRepository {
	return &PlatformCollaboratorRepositoryInfrastructure{db: db}
}

func (r *PlatformCollaboratorRepositoryInfrastructure) WithTX(tx repository.Tx) repository.PlatformCollaboratorRepository {
	return &PlatformCollaboratorRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *PlatformCollaboratorRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PlatformCollaboratorRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *PlatformCollaboratorRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// CREATE
// ============================================================

func (r *PlatformCollaboratorRepositoryInfrastructure) Create(ctx context.Context, collab *entity.PlatformCollaborator) error {
	query := `
		INSERT INTO platform_collaborators (
			id, user_id, role, permissions, invited_by, invited_at, accepted_at,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	permJSON, err := json.Marshal(collab.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	_, err = r.execContext(ctx, query,
		collab.ID,
		collab.UserID,
		collab.Role,
		permJSON,
		collab.InvitedBy,
		collab.InvitedAt,
		collab.AcceptedAt,
		collab.IsActive,
		collab.CreatedAt,
		collab.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create platform collaborator: %w", err)
	}

	return nil
}

// ============================================================
// FIND METHODS
// ============================================================

func (r *PlatformCollaboratorRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.PlatformCollaborator, error) {
	query := `
		SELECT id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM platform_collaborators
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanCollaborator(r.queryRowContext(ctx, query, id))
}

func (r *PlatformCollaboratorRepositoryInfrastructure) FindByUserID(ctx context.Context, userID string) (*entity.PlatformCollaborator, error) {
	query := `
		SELECT id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM platform_collaborators
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	return r.scanCollaborator(r.queryRowContext(ctx, query, userID))
}

func (r *PlatformCollaboratorRepositoryInfrastructure) FindAll(ctx context.Context, limit, offset int) ([]*entity.PlatformCollaborator, int, error) {
	// Compter le total
	countQuery := `
		SELECT COUNT(*) FROM platform_collaborators
		WHERE is_active = true AND deleted_at IS NULL
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count collaborators: %w", err)
	}

	// Récupérer les collaborateurs
	query := `
		SELECT id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM platform_collaborators
		WHERE is_active = true AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.queryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("find all collaborators: %w", err)
	}
	defer rows.Close()

	return r.scanCollaborators(rows), total, nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) FindByRole(ctx context.Context, role entity.PlatformRole, limit, offset int) ([]*entity.PlatformCollaborator, int, error) {
	// Compter le total
	countQuery := `
		SELECT COUNT(*) FROM platform_collaborators
		WHERE role = $1 AND is_active = true AND deleted_at IS NULL
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, role).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count collaborators by role: %w", err)
	}

	// Récupérer les collaborateurs
	query := `
		SELECT id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM platform_collaborators
		WHERE role = $1 AND is_active = true AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.queryContext(ctx, query, role, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("find collaborators by role: %w", err)
	}
	defer rows.Close()

	return r.scanCollaborators(rows), total, nil
}

// ============================================================
// UPDATE
// ============================================================

func (r *PlatformCollaboratorRepositoryInfrastructure) Update(ctx context.Context, collab *entity.PlatformCollaborator) error {
	query := `
		UPDATE platform_collaborators
		SET role = $2, permissions = $3, accepted_at = $4, is_active = $5,
		    last_login_at = $6, last_activity_at = $7, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	permJSON, err := json.Marshal(collab.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	_, err = r.execContext(ctx, query,
		collab.ID,
		collab.Role,
		permJSON,
		collab.AcceptedAt,
		collab.IsActive,
		collab.LastLoginAt,
		collab.LastActivityAt,
	)

	if err != nil {
		return fmt.Errorf("update platform collaborator: %w", err)
	}

	return nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) Deactivate(ctx context.Context, id uuid.UUID, deletedBy, reason string) error {
	now := time.Now()
	query := `
		UPDATE platform_collaborators
		SET is_active = false,
		    deleted_at = $2,
		    deleted_by = $3,
		    deletion_reason = $4,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.execContext(ctx, query, id, now, deletedBy, reason)
	if err != nil {
		return fmt.Errorf("deactivate platform collaborator: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrCollaboratorNotFound
	}

	return nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) Reactivate(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE platform_collaborators
		SET is_active = true,
		    deleted_at = NULL,
		    deleted_by = NULL,
		    deletion_reason = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NOT NULL
	`

	result, err := r.execContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("reactivate platform collaborator: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrCollaboratorNotFound
	}

	return nil
}

// ============================================================
// MÉTHODES SPÉCIFIQUES
// ============================================================

func (r *PlatformCollaboratorRepositoryInfrastructure) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE platform_collaborators
		SET last_login_at = $2,
		    last_activity_at = $2,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.execContext(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}

	return nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) UpdateRole(ctx context.Context, id uuid.UUID, role entity.PlatformRole, permissions entity.PlatformPermissions) error {
	permJSON, err := json.Marshal(permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	query := `
		UPDATE platform_collaborators
		SET role = $2,
		    permissions = $3,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err = r.execContext(ctx, query, id, role, permJSON)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}

	return nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) HasPermission(ctx context.Context, userID string, permission string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM platform_collaborators
			WHERE user_id = $1
			  AND is_active = true
			  AND deleted_at IS NULL
			  AND permissions @> jsonb_build_object($2::text, true)
		)
	`

	var hasPerm bool
	err := r.db.QueryRowContext(ctx, query, userID, permission).Scan(&hasPerm)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}

	return hasPerm, nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) CountActive(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*) FROM platform_collaborators
		WHERE is_active = true AND deleted_at IS NULL
	`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active collaborators: %w", err)
	}

	return count, nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) CountByRole(ctx context.Context) (map[entity.PlatformRole]int, error) {
	query := `
		SELECT role, COUNT(*)
		FROM platform_collaborators
		WHERE is_active = true AND deleted_at IS NULL
		GROUP BY role
	`

	rows, err := r.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by role: %w", err)
	}
	defer rows.Close()

	counts := make(map[entity.PlatformRole]int)
	for rows.Next() {
		var role entity.PlatformRole
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			return nil, err
		}
		counts[role] = count
	}

	return counts, rows.Err()
}

func (r *PlatformCollaboratorRepositoryInfrastructure) IsPlatformCollaborator(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM platform_collaborators
			WHERE user_id = $1
			  AND is_active = true
			  AND deleted_at IS NULL
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check platform collaborator: %w", err)
	}

	return exists, nil
}

// ============================================================
// SCAN HELPERS
// ============================================================

func (r *PlatformCollaboratorRepositoryInfrastructure) scanCollaborator(row *sql.Row) (*entity.PlatformCollaborator, error) {
	collab := &entity.PlatformCollaborator{}
	var permJSON []byte
	var deletedAt, acceptedAt, lastLoginAt, lastActivityAt sql.NullTime
	var deletedBy, deletionReason sql.NullString

	err := row.Scan(
		&collab.ID,
		&collab.UserID,
		&collab.Role,
		&permJSON,
		&collab.InvitedBy,
		&collab.InvitedAt,
		&acceptedAt,
		&collab.IsActive,
		&deletedAt,
		&deletedBy,
		&deletionReason,
		&lastLoginAt,
		&lastActivityAt,
		&collab.CreatedAt,
		&collab.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Désérialiser permissions
	if len(permJSON) > 0 {
		if err := json.Unmarshal(permJSON, &collab.Permissions); err != nil {
			return nil, fmt.Errorf("unmarshal permissions: %w", err)
		}
	}

	// Gérer les champs nullable
	if acceptedAt.Valid {
		collab.AcceptedAt = &acceptedAt.Time
	}
	if deletedAt.Valid {
		collab.DeletedAt = &deletedAt.Time
	}
	if deletedBy.Valid {
		collab.DeletedBy = &deletedBy.String
	}
	if deletionReason.Valid {
		collab.DeletionReason = &deletionReason.String
	}
	if lastLoginAt.Valid {
		collab.LastLoginAt = &lastLoginAt.Time
	}
	if lastActivityAt.Valid {
		collab.LastActivityAt = &lastActivityAt.Time
	}

	return collab, nil
}

func (r *PlatformCollaboratorRepositoryInfrastructure) scanCollaborators(rows *sql.Rows) []*entity.PlatformCollaborator {
	var collabs []*entity.PlatformCollaborator
	for rows.Next() {
		collab := &entity.PlatformCollaborator{}
		var permJSON []byte
		var deletedAt, acceptedAt, lastLoginAt, lastActivityAt sql.NullTime
		var deletedBy, deletionReason sql.NullString

		err := rows.Scan(
			&collab.ID,
			&collab.UserID,
			&collab.Role,
			&permJSON,
			&collab.InvitedBy,
			&collab.InvitedAt,
			&acceptedAt,
			&collab.IsActive,
			&deletedAt,
			&deletedBy,
			&deletionReason,
			&lastLoginAt,
			&lastActivityAt,
			&collab.CreatedAt,
			&collab.UpdatedAt,
		)

		if err != nil {
			continue
		}

		if len(permJSON) > 0 {
			json.Unmarshal(permJSON, &collab.Permissions)
		}

		if acceptedAt.Valid {
			collab.AcceptedAt = &acceptedAt.Time
		}
		if deletedAt.Valid {
			collab.DeletedAt = &deletedAt.Time
		}
		if deletedBy.Valid {
			collab.DeletedBy = &deletedBy.String
		}
		if deletionReason.Valid {
			collab.DeletionReason = &deletionReason.String
		}
		if lastLoginAt.Valid {
			collab.LastLoginAt = &lastLoginAt.Time
		}
		if lastActivityAt.Valid {
			collab.LastActivityAt = &lastActivityAt.Time
		}

		collabs = append(collabs, collab)
	}

	return collabs
}
