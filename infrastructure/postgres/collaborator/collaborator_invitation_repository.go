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
// 🆕 v4.3.0 : COLLABORATOR INVITATION REPOSITORY
// ============================================================

type CollaboratorInvitationRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewCollaboratorInvitationRepositoryInfrastructure(db *sql.DB) repository.CollaboratorInvitationRepository {
	return &CollaboratorInvitationRepositoryInfrastructure{db: db}
}

func (r *CollaboratorInvitationRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CollaboratorInvitationRepository {
	return &CollaboratorInvitationRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *CollaboratorInvitationRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CollaboratorInvitationRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CollaboratorInvitationRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// CREATE
// ============================================================

func (r *CollaboratorInvitationRepositoryInfrastructure) Create(ctx context.Context, invitation *entity.CollaboratorInvitation) error {
	query := `
		INSERT INTO collaborator_invitations (
			id, invitation_type, shop_id, email, role, permissions, token,
			invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
			message, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	_, err := r.execContext(ctx, query,
		invitation.ID,
		invitation.InvitationType,
		invitation.ShopID,
		invitation.Email,
		invitation.Role,
		invitation.Permissions,
		invitation.Token,
		invitation.InvitedBy,
		invitation.Status,
		invitation.InvitedAt,
		invitation.ExpiresAt,
		invitation.AcceptedAt,
		invitation.CancelledAt,
		invitation.Message,
		invitation.CreatedAt,
		invitation.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}

	return nil
}

// ============================================================
// FIND METHODS
// ============================================================

func (r *CollaboratorInvitationRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.CollaboratorInvitation, error) {
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE id = $1
	`
	return r.scanInvitation(r.queryRowContext(ctx, query, id))
}

func (r *CollaboratorInvitationRepositoryInfrastructure) FindByToken(ctx context.Context, token string) (*entity.CollaboratorInvitation, error) {
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE token = $1
	`
	return r.scanInvitation(r.queryRowContext(ctx, query, token))
}

func (r *CollaboratorInvitationRepositoryInfrastructure) FindByEmail(ctx context.Context, email string) ([]*entity.CollaboratorInvitation, error) {
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE LOWER(email) = LOWER($1)
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("find invitations by email: %w", err)
	}
	defer rows.Close()

	return r.scanInvitations(rows), nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) FindPending(ctx context.Context, limit, offset int) ([]*entity.CollaboratorInvitation, int, error) {
	// Compter le total
	countQuery := `
		SELECT COUNT(*) FROM collaborator_invitations
		WHERE status = 'pending' AND expires_at > NOW()
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count pending invitations: %w", err)
	}

	// Récupérer les invitations
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.queryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("find pending invitations: %w", err)
	}
	defer rows.Close()

	return r.scanInvitations(rows), total, nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) FindPendingByShopID(ctx context.Context, shopID uuid.UUID) ([]*entity.CollaboratorInvitation, error) {
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE shop_id = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("find pending invitations by shop: %w", err)
	}
	defer rows.Close()

	return r.scanInvitations(rows), nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) FindPendingByType(ctx context.Context, invType entity.InvitationType) ([]*entity.CollaboratorInvitation, error) {
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE invitation_type = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, invType)
	if err != nil {
		return nil, fmt.Errorf("find pending invitations by type: %w", err)
	}
	defer rows.Close()

	return r.scanInvitations(rows), nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) FindByInvitedBy(ctx context.Context, invitedBy string) ([]*entity.CollaboratorInvitation, error) {
	query := `
		SELECT id, invitation_type, shop_id, email, role, permissions, token,
		       invited_by, status, invited_at, expires_at, accepted_at, cancelled_at,
		       message, created_at, updated_at
		FROM collaborator_invitations
		WHERE invited_by = $1
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, invitedBy)
	if err != nil {
		return nil, fmt.Errorf("find invitations by inviter: %w", err)
	}
	defer rows.Close()

	return r.scanInvitations(rows), nil
}

// ============================================================
// UPDATE
// ============================================================

func (r *CollaboratorInvitationRepositoryInfrastructure) Update(ctx context.Context, invitation *entity.CollaboratorInvitation) error {
	query := `
		UPDATE collaborator_invitations
		SET status = $2, accepted_at = $3, cancelled_at = $4, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.execContext(ctx, query,
		invitation.ID,
		invitation.Status,
		invitation.AcceptedAt,
		invitation.CancelledAt,
	)

	if err != nil {
		return fmt.Errorf("update invitation: %w", err)
	}

	return nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM collaborator_invitations WHERE id = $1`

	result, err := r.execContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrInvitationNotFound
	}

	return nil
}

// ============================================================
// MÉTHODES SPÉCIFIQUES
// ============================================================

func (r *CollaboratorInvitationRepositoryInfrastructure) MarkAccepted(ctx context.Context, token string) error {
	now := time.Now()
	query := `
		UPDATE collaborator_invitations
		SET status = 'accepted',
		    accepted_at = $2,
		    updated_at = NOW()
		WHERE token = $1
		  AND status = 'pending'
		  AND expires_at > NOW()
	`

	result, err := r.execContext(ctx, query, token, now)
	if err != nil {
		return fmt.Errorf("mark invitation accepted: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Vérifier pourquoi ça a échoué
		var status string
		var expiresAt time.Time
		checkQuery := `SELECT status, expires_at FROM collaborator_invitations WHERE token = $1`
		err := r.db.QueryRowContext(ctx, checkQuery, token).Scan(&status, &expiresAt)
		if err == sql.ErrNoRows {
			return entity.ErrInvitationNotFound
		}
		if err != nil {
			return fmt.Errorf("check invitation: %w", err)
		}

		switch status {
		case "accepted":
			return entity.ErrInvitationAlreadyUsed
		case "cancelled":
			return entity.ErrInvitationCancelled
		}
		if time.Now().After(expiresAt) {
			return entity.ErrInvitationExpired
		}
	}

	return nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) Cancel(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE collaborator_invitations
		SET status = 'cancelled',
		    cancelled_at = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`

	result, err := r.execContext(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("cancel invitation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrInvitationNotFound
	}

	return nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) CleanupExpired(ctx context.Context) (int, error) {
	query := `
		UPDATE collaborator_invitations
		SET status = 'expired',
		    updated_at = NOW()
		WHERE status = 'pending'
		  AND expires_at <= NOW()
	`

	result, err := r.execContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired invitations: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) CountPending(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*) FROM collaborator_invitations
		WHERE status = 'pending' AND expires_at > NOW()
	`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending invitations: %w", err)
	}

	return count, nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) CountByStatus(ctx context.Context) (map[entity.InvitationStatus]int, error) {
	query := `
		SELECT status, COUNT(*)
		FROM collaborator_invitations
		GROUP BY status
	`

	rows, err := r.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[entity.InvitationStatus]int)
	for rows.Next() {
		var status entity.InvitationStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}

	return counts, rows.Err()
}

func (r *CollaboratorInvitationRepositoryInfrastructure) IsTokenValid(ctx context.Context, token string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM collaborator_invitations
			WHERE token = $1
			  AND status = 'pending'
			  AND expires_at > NOW()
		)
	`

	var valid bool
	err := r.db.QueryRowContext(ctx, query, token).Scan(&valid)
	if err != nil {
		return false, fmt.Errorf("check token validity: %w", err)
	}

	return valid, nil
}

// ============================================================
// SCAN HELPERS
// ============================================================

func (r *CollaboratorInvitationRepositoryInfrastructure) scanInvitation(row *sql.Row) (*entity.CollaboratorInvitation, error) {
	invitation := &entity.CollaboratorInvitation{}
	var shopID sql.NullString
	var acceptedAt, cancelledAt sql.NullTime
	var message sql.NullString

	err := row.Scan(
		&invitation.ID,
		&invitation.InvitationType,
		&shopID,
		&invitation.Email,
		&invitation.Role,
		&invitation.Permissions,
		&invitation.Token,
		&invitation.InvitedBy,
		&invitation.Status,
		&invitation.InvitedAt,
		&invitation.ExpiresAt,
		&acceptedAt,
		&cancelledAt,
		&message,
		&invitation.CreatedAt,
		&invitation.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Gérer les champs nullable
	if shopID.Valid {
		parsedID, err := uuid.Parse(shopID.String)
		if err == nil {
			invitation.ShopID = &parsedID
		}
	}
	if acceptedAt.Valid {
		invitation.AcceptedAt = &acceptedAt.Time
	}
	if cancelledAt.Valid {
		invitation.CancelledAt = &cancelledAt.Time
	}
	if message.Valid {
		invitation.Message = &message.String
	}

	// Valider que permissions est un JSON valide
	if len(invitation.Permissions) == 0 {
		invitation.Permissions = json.RawMessage("{}")
	}

	return invitation, nil
}

func (r *CollaboratorInvitationRepositoryInfrastructure) scanInvitations(rows *sql.Rows) []*entity.CollaboratorInvitation {
	var invitations []*entity.CollaboratorInvitation
	for rows.Next() {
		invitation := &entity.CollaboratorInvitation{}
		var shopID sql.NullString
		var acceptedAt, cancelledAt sql.NullTime
		var message sql.NullString

		err := rows.Scan(
			&invitation.ID,
			&invitation.InvitationType,
			&shopID,
			&invitation.Email,
			&invitation.Role,
			&invitation.Permissions,
			&invitation.Token,
			&invitation.InvitedBy,
			&invitation.Status,
			&invitation.InvitedAt,
			&invitation.ExpiresAt,
			&acceptedAt,
			&cancelledAt,
			&message,
			&invitation.CreatedAt,
			&invitation.UpdatedAt,
		)

		if err != nil {
			continue
		}

		if shopID.Valid {
			parsedID, err := uuid.Parse(shopID.String)
			if err == nil {
				invitation.ShopID = &parsedID
			}
		}
		if acceptedAt.Valid {
			invitation.AcceptedAt = &acceptedAt.Time
		}
		if cancelledAt.Valid {
			invitation.CancelledAt = &cancelledAt.Time
		}
		if message.Valid {
			invitation.Message = &message.String
		}

		if len(invitation.Permissions) == 0 {
			invitation.Permissions = json.RawMessage("{}")
		}

		invitations = append(invitations, invitation)
	}

	return invitations
}
