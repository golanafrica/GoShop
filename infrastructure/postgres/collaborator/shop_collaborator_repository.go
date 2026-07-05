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
// 🆕 v4.3.0 : SHOP COLLABORATOR REPOSITORY
// ============================================================

type ShopCollaboratorRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewShopCollaboratorRepositoryInfrastructure(db *sql.DB) repository.ShopCollaboratorRepository {
	return &ShopCollaboratorRepositoryInfrastructure{db: db}
}

func (r *ShopCollaboratorRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ShopCollaboratorRepository {
	return &ShopCollaboratorRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *ShopCollaboratorRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *ShopCollaboratorRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *ShopCollaboratorRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// CREATE
// ============================================================

func (r *ShopCollaboratorRepositoryInfrastructure) Create(ctx context.Context, collab *entity.ShopCollaborator) error {
	query := `
		INSERT INTO shop_collaborators (
			id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	permJSON, err := json.Marshal(collab.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	_, err = r.execContext(ctx, query,
		collab.ID,
		collab.ShopID,
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
		return fmt.Errorf("create shop collaborator: %w", err)
	}

	return nil
}

// ============================================================
// FIND METHODS
// ============================================================

func (r *ShopCollaboratorRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.ShopCollaborator, error) {
	query := `
		SELECT id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM shop_collaborators
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanCollaborator(r.queryRowContext(ctx, query, id))
}

func (r *ShopCollaboratorRepositoryInfrastructure) FindByShopIDAndUserID(
	ctx context.Context,
	shopID uuid.UUID,
	userID string,
) (*entity.ShopCollaborator, error) {
	query := `
		SELECT id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM shop_collaborators
		WHERE shop_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`
	return r.scanCollaborator(r.queryRowContext(ctx, query, shopID, userID))
}

func (r *ShopCollaboratorRepositoryInfrastructure) FindByShopID(
	ctx context.Context,
	shopID uuid.UUID,
	limit, offset int,
) ([]*entity.ShopCollaborator, int, error) {
	// Compter le total
	countQuery := `
		SELECT COUNT(*) FROM shop_collaborators
		WHERE shop_id = $1 AND is_active = true AND deleted_at IS NULL
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, shopID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count collaborators: %w", err)
	}

	// Récupérer les collaborateurs
	query := `
		SELECT id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM shop_collaborators
		WHERE shop_id = $1 AND is_active = true AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.queryContext(ctx, query, shopID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("find collaborators by shop: %w", err)
	}
	defer rows.Close()

	return r.scanCollaborators(rows), total, nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) FindByUserID(ctx context.Context, userID string) ([]*entity.ShopCollaborator, error) {
	query := `
		SELECT id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM shop_collaborators
		WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("find collaborators by user: %w", err)
	}
	defer rows.Close()

	return r.scanCollaborators(rows), nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) FindByRole(
	ctx context.Context,
	shopID uuid.UUID,
	role entity.ShopRole,
) ([]*entity.ShopCollaborator, error) {
	query := `
		SELECT id, shop_id, user_id, role, permissions, invited_by, invited_at, accepted_at,
		       is_active, deleted_at, deleted_by, deletion_reason,
		       last_login_at, last_activity_at, created_at, updated_at
		FROM shop_collaborators
		WHERE shop_id = $1 AND role = $2 AND is_active = true AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, shopID, role)
	if err != nil {
		return nil, fmt.Errorf("find collaborators by role: %w", err)
	}
	defer rows.Close()

	return r.scanCollaborators(rows), nil
}

// ============================================================
// UPDATE
// ============================================================

func (r *ShopCollaboratorRepositoryInfrastructure) Update(ctx context.Context, collab *entity.ShopCollaborator) error {
	query := `
		UPDATE shop_collaborators
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
		return fmt.Errorf("update shop collaborator: %w", err)
	}

	return nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) Deactivate(ctx context.Context, id uuid.UUID, deletedBy, reason string) error {
	now := time.Now()
	query := `
		UPDATE shop_collaborators
		SET is_active = false,
		    deleted_at = $2,
		    deleted_by = $3,
		    deletion_reason = $4,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.execContext(ctx, query, id, now, deletedBy, reason)
	if err != nil {
		return fmt.Errorf("deactivate shop collaborator: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrCollaboratorNotFound
	}

	return nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) DeactivateByShopIDAndUserID(
	ctx context.Context,
	shopID uuid.UUID,
	userID string,
	deletedBy, reason string,
) error {
	now := time.Now()
	query := `
		UPDATE shop_collaborators
		SET is_active = false,
		    deleted_at = $2,
		    deleted_by = $3,
		    deletion_reason = $4,
		    updated_at = NOW()
		WHERE shop_id = $1 AND user_id = $5 AND deleted_at IS NULL
	`

	result, err := r.execContext(ctx, query, shopID, now, deletedBy, reason, userID)
	if err != nil {
		return fmt.Errorf("deactivate shop collaborator: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrCollaboratorNotFound
	}

	return nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) Reactivate(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE shop_collaborators
		SET is_active = true,
		    deleted_at = NULL,
		    deleted_by = NULL,
		    deletion_reason = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NOT NULL
	`

	result, err := r.execContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("reactivate shop collaborator: %w", err)
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

func (r *ShopCollaboratorRepositoryInfrastructure) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE shop_collaborators
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

func (r *ShopCollaboratorRepositoryInfrastructure) UpdateRole(
	ctx context.Context,
	id uuid.UUID,
	role entity.ShopRole,
	permissions entity.ShopPermissions,
) error {
	permJSON, err := json.Marshal(permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	query := `
		UPDATE shop_collaborators
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

func (r *ShopCollaboratorRepositoryInfrastructure) HasPermission(
	ctx context.Context,
	userID string,
	shopID uuid.UUID,
	permission string,
) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM shop_collaborators
			WHERE user_id = $1
			  AND shop_id = $2
			  AND is_active = true
			  AND deleted_at IS NULL
			  AND (
			      permissions @> jsonb_build_object($3, true)
			      OR role = 'shop_admin'
			  )
		)
	`

	var hasPerm bool
	err := r.db.QueryRowContext(ctx, query, userID, shopID, permission).Scan(&hasPerm)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}

	return hasPerm, nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM shop_collaborators
		WHERE shop_id = $1 AND is_active = true AND deleted_at IS NULL
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count collaborators by shop: %w", err)
	}

	return count, nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) CountByUserID(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT shop_id) FROM shop_collaborators
		WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count shops by user: %w", err)
	}

	return count, nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) IsShopCollaborator(
	ctx context.Context,
	userID string,
	shopID uuid.UUID,
) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM shop_collaborators
			WHERE user_id = $1
			  AND shop_id = $2
			  AND is_active = true
			  AND deleted_at IS NULL
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, shopID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check shop collaborator: %w", err)
	}

	return exists, nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) IsShopAdmin(
	ctx context.Context,
	userID string,
	shopID uuid.UUID,
) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM shop_collaborators
			WHERE user_id = $1
			  AND shop_id = $2
			  AND role = 'shop_admin'
			  AND is_active = true
			  AND deleted_at IS NULL
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, shopID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check shop admin: %w", err)
	}

	return exists, nil
}

func (r *ShopCollaboratorRepositoryInfrastructure) DeleteAllByShopID(ctx context.Context, shopID uuid.UUID) error {
	query := `
		UPDATE shop_collaborators
		SET is_active = false,
		    deleted_at = NOW(),
		    deleted_by = 'system',
		    deletion_reason = 'Shop deleted',
		    updated_at = NOW()
		WHERE shop_id = $1 AND deleted_at IS NULL
	`

	_, err := r.execContext(ctx, query, shopID)
	if err != nil {
		return fmt.Errorf("delete all collaborators by shop: %w", err)
	}

	return nil
}

// ============================================================
// SCAN HELPERS
// ============================================================

func (r *ShopCollaboratorRepositoryInfrastructure) scanCollaborator(row *sql.Row) (*entity.ShopCollaborator, error) {
	collab := &entity.ShopCollaborator{}
	var permJSON []byte
	var deletedAt, acceptedAt, lastLoginAt, lastActivityAt sql.NullTime
	var deletedBy, deletionReason sql.NullString

	err := row.Scan(
		&collab.ID,
		&collab.ShopID,
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

func (r *ShopCollaboratorRepositoryInfrastructure) scanCollaborators(rows *sql.Rows) []*entity.ShopCollaborator {
	var collabs []*entity.ShopCollaborator
	for rows.Next() {
		collab := &entity.ShopCollaborator{}
		var permJSON []byte
		var deletedAt, acceptedAt, lastLoginAt, lastActivityAt sql.NullTime
		var deletedBy, deletionReason sql.NullString

		err := rows.Scan(
			&collab.ID,
			&collab.ShopID,
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
