package tontine

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// TontineParticipantRepositoryInfrastructure implémente repository.TontineParticipantRepository
type TontineParticipantRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewTontineParticipantRepositoryInfrastructure crée une nouvelle instance
func NewTontineParticipantRepositoryInfrastructure(db *sql.DB) repository.TontineParticipantRepository {
	return &TontineParticipantRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *TontineParticipantRepositoryInfrastructure) WithTX(tx repository.Tx) repository.TontineParticipantRepository {
	return &TontineParticipantRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers
// ============================================================

func (r *TontineParticipantRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *TontineParticipantRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *TontineParticipantRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *TontineParticipantRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanParticipant scanne une ligne dans une entité TontineParticipant
func (r *TontineParticipantRepositoryInfrastructure) scanParticipant(row *sql.Row) (*entity.TontineParticipant, error) {
	p := &entity.TontineParticipant{}
	err := row.Scan(
		&p.ID,
		&p.GroupID,
		&p.CustomerID,
		&p.PayoutPosition,
		&p.Status,
		&p.JoinedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tontine participant not found")
		}
		return nil, fmt.Errorf("failed to scan participant: %w", err)
	}
	return p, nil
}

// scanParticipants scanne plusieurs lignes
func (r *TontineParticipantRepositoryInfrastructure) scanParticipants(ctx context.Context, query string, args ...interface{}) ([]*entity.TontineParticipant, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query participants: %w", err)
	}
	defer rows.Close()

	var participants []*entity.TontineParticipant
	for rows.Next() {
		p := &entity.TontineParticipant{}
		err := rows.Scan(
			&p.ID, &p.GroupID, &p.CustomerID,
			&p.PayoutPosition, &p.Status, &p.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		participants = append(participants, p)
	}

	if participants == nil {
		participants = []*entity.TontineParticipant{}
	}
	return participants, rows.Err()
}

// ============================================================
// Implémentation
// ============================================================

// Add ajoute un participant à un groupe
func (r *TontineParticipantRepositoryInfrastructure) Add(ctx context.Context, participant *entity.TontineParticipant) error {
	// Vérification multi-tenant via le groupe
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le groupe appartient à la boutique
	var groupShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM tontine_groups WHERE id = $1`, participant.GroupID).Scan(&groupShopID)
	if err != nil {
		return fmt.Errorf("failed to verify group: %w", err)
	}
	if groupShopID != shopID {
		return fmt.Errorf("access denied: group does not belong to tenant shop")
	}

	query := `
		INSERT INTO tontine_participants (group_id, customer_id, payout_position, status, joined_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, joined_at
	`

	err = r.queryRowContext(ctx, query,
		participant.GroupID,
		participant.CustomerID,
		participant.PayoutPosition,
		participant.Status,
	).Scan(&participant.ID, &participant.JoinedAt)

	if err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}

	return nil
}

// FindByID trouve un participant par son ID
func (r *TontineParticipantRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.TontineParticipant, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.customer_id, p.payout_position, p.status, p.joined_at
		FROM tontine_participants p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.id = $1 AND g.shop_id = $2
	`

	return r.scanParticipant(r.queryRowContext(ctx, query, id, shopID))
}

// FindByGroupID retourne tous les participants d'un groupe
func (r *TontineParticipantRepositoryInfrastructure) FindByGroupID(ctx context.Context, groupID string) ([]*entity.TontineParticipant, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.customer_id, p.payout_position, p.status, p.joined_at
		FROM tontine_participants p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND g.shop_id = $2
		ORDER BY p.payout_position ASC
	`

	return r.scanParticipants(ctx, query, groupID, shopID)
}

// FindByGroupAndCustomer trouve un participant par groupe et client
func (r *TontineParticipantRepositoryInfrastructure) FindByGroupAndCustomer(ctx context.Context, groupID, customerID string) (*entity.TontineParticipant, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.customer_id, p.payout_position, p.status, p.joined_at
		FROM tontine_participants p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND p.customer_id = $2 AND g.shop_id = $3
	`

	return r.scanParticipant(r.queryRowContext(ctx, query, groupID, customerID, shopID))
}

// FindByPosition trouve le participant à une position donnée dans un groupe
func (r *TontineParticipantRepositoryInfrastructure) FindByPosition(ctx context.Context, groupID string, position int) (*entity.TontineParticipant, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.customer_id, p.payout_position, p.status, p.joined_at
		FROM tontine_participants p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND p.payout_position = $2 AND g.shop_id = $3
	`

	return r.scanParticipant(r.queryRowContext(ctx, query, groupID, position, shopID))
}

// CountByGroup compte le nombre de participants dans un groupe
func (r *TontineParticipantRepositoryInfrastructure) CountByGroup(ctx context.Context, groupID string) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM tontine_participants p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND g.shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, groupID, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count participants: %w", err)
	}
	return count, nil
}

// CountActiveByGroup compte le nombre de participants actifs
func (r *TontineParticipantRepositoryInfrastructure) CountActiveByGroup(ctx context.Context, groupID string) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM tontine_participants p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND p.status = 'active' AND g.shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, groupID, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active participants: %w", err)
	}
	return count, nil
}

// UpdateStatus met à jour le statut d'un participant
func (r *TontineParticipantRepositoryInfrastructure) UpdateStatus(ctx context.Context, participantID string, status string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE tontine_participants SET status = $1
		WHERE id = $2 AND group_id IN (
			SELECT id FROM tontine_groups WHERE shop_id = $3
		)
	`
	result, err := r.execContext(ctx, query, status, participantID, shopID)
	if err != nil {
		return fmt.Errorf("failed to update participant status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("participant not found")
	}
	return nil
}
