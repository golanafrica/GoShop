package tontine

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// TontineGroupRepositoryInfrastructure implémente repository.TontineGroupRepository
type TontineGroupRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewTontineGroupRepositoryInfrastructure crée une nouvelle instance
func NewTontineGroupRepositoryInfrastructure(db *sql.DB) repository.TontineGroupRepository {
	return &TontineGroupRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *TontineGroupRepositoryInfrastructure) WithTX(tx repository.Tx) repository.TontineGroupRepository {
	return &TontineGroupRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers
// ============================================================

func (r *TontineGroupRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *TontineGroupRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *TontineGroupRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *TontineGroupRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanGroup scanne une ligne dans une entité TontineGroup
func (r *TontineGroupRepositoryInfrastructure) scanGroup(row *sql.Row) (*entity.TontineGroup, error) {
	group := &entity.TontineGroup{}
	var creatorCustomerID sql.NullString
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	err := row.Scan(
		&group.ID,
		&group.ProductID,
		&group.ShopID,
		&creatorCustomerID,
		&group.CreatorType,
		&group.CircleType,
		&group.AmountPerCycleCents,
		&group.TotalCycles,
		&group.CurrentCycle,
		&group.InviteCode,
		&group.Status,
		&startedAt,
		&completedAt,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tontine group not found")
		}
		return nil, fmt.Errorf("failed to scan tontine group: %w", err)
	}

	if creatorCustomerID.Valid {
		group.CreatorCustomerID = &creatorCustomerID.String
	}
	if startedAt.Valid {
		group.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		group.CompletedAt = &completedAt.Time
	}

	return group, nil
}

// scanGroups scanne plusieurs lignes
func (r *TontineGroupRepositoryInfrastructure) scanGroups(ctx context.Context, query string, args ...interface{}) ([]*entity.TontineGroup, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tontine groups: %w", err)
	}
	defer rows.Close()

	var groups []*entity.TontineGroup
	for rows.Next() {
		group := &entity.TontineGroup{}
		var creatorCustomerID sql.NullString
		var startedAt sql.NullTime
		var completedAt sql.NullTime

		err := rows.Scan(
			&group.ID,
			&group.ProductID,
			&group.ShopID,
			&creatorCustomerID,
			&group.CreatorType,
			&group.CircleType,
			&group.AmountPerCycleCents,
			&group.TotalCycles,
			&group.CurrentCycle,
			&group.InviteCode,
			&group.Status,
			&startedAt,
			&completedAt,
			&group.CreatedAt,
			&group.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if creatorCustomerID.Valid {
			group.CreatorCustomerID = &creatorCustomerID.String
		}
		if startedAt.Valid {
			group.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			group.CompletedAt = &completedAt.Time
		}

		groups = append(groups, group)
	}

	if groups == nil {
		groups = []*entity.TontineGroup{}
	}
	return groups, rows.Err()
}

// ============================================================
// Implémentation
// ============================================================

// Create crée un nouveau groupe de tontine
func (r *TontineGroupRepositoryInfrastructure) Create(ctx context.Context, group *entity.TontineGroup) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	if group.ShopID != shopID {
		return fmt.Errorf("group shop_id does not match tenant shop_id")
	}

	query := `
		INSERT INTO tontine_groups (
			product_id, shop_id, creator_customer_id, creator_type,
			circle_type, amount_per_cycle_cents,
			total_cycles, current_cycle, invite_code,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	var creatorCustomerID *string
	if group.CreatorCustomerID != nil {
		creatorCustomerID = group.CreatorCustomerID
	}

	err = r.queryRowContext(ctx, query,
		group.ProductID,
		group.ShopID,
		creatorCustomerID,
		group.CreatorType,
		group.CircleType,
		group.AmountPerCycleCents,
		group.TotalCycles,
		group.CurrentCycle,
		group.InviteCode,
		group.Status,
	).Scan(&group.ID, &group.CreatedAt, &group.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create tontine group: %w", err)
	}

	return nil
}

// FindByID trouve un groupe par son ID
func (r *TontineGroupRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.TontineGroup, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, shop_id, creator_customer_id, creator_type,
		       circle_type, amount_per_cycle_cents,
		       total_cycles, current_cycle, invite_code,
		       status, started_at, completed_at,
		       created_at, updated_at
		FROM tontine_groups
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanGroup(r.queryRowContext(ctx, query, id, shopID))
}

// FindByInviteCode trouve un groupe par son code d'invitation
func (r *TontineGroupRepositoryInfrastructure) FindByInviteCode(ctx context.Context, code string) (*entity.TontineGroup, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, shop_id, creator_customer_id, creator_type,
		       circle_type, amount_per_cycle_cents,
		       total_cycles, current_cycle, invite_code,
		       status, started_at, completed_at,
		       created_at, updated_at
		FROM tontine_groups
		WHERE invite_code = $1 AND shop_id = $2
	`

	return r.scanGroup(r.queryRowContext(ctx, query, code, shopID))
}

// FindByShopID retourne tous les groupes d'une boutique
func (r *TontineGroupRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.TontineGroup, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if currentShopID != shopID {
		return nil, fmt.Errorf("access denied: shop_id mismatch")
	}

	query := `
		SELECT id, product_id, shop_id, creator_customer_id, creator_type,
		       circle_type, amount_per_cycle_cents,
		       total_cycles, current_cycle, invite_code,
		       status, started_at, completed_at,
		       created_at, updated_at
		FROM tontine_groups
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	return r.scanGroups(ctx, query, shopID)
}

// FindByProductID retourne tous les groupes d'un produit
func (r *TontineGroupRepositoryInfrastructure) FindByProductID(ctx context.Context, productID string) ([]*entity.TontineGroup, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, shop_id, creator_customer_id, creator_type,
		       circle_type, amount_per_cycle_cents,
		       total_cycles, current_cycle, invite_code,
		       status, started_at, completed_at,
		       created_at, updated_at
		FROM tontine_groups
		WHERE product_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanGroups(ctx, query, productID, shopID)
}

// FindByCreatorCustomerID retourne tous les groupes créés par un client
func (r *TontineGroupRepositoryInfrastructure) FindByCreatorCustomerID(ctx context.Context, customerID string) ([]*entity.TontineGroup, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, shop_id, creator_customer_id, creator_type,
		       circle_type, amount_per_cycle_cents,
		       total_cycles, current_cycle, invite_code,
		       status, started_at, completed_at,
		       created_at, updated_at
		FROM tontine_groups
		WHERE creator_customer_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanGroups(ctx, query, customerID, shopID)
}

// UpdateStatus met à jour le statut d'un groupe
func (r *TontineGroupRepositoryInfrastructure) UpdateStatus(ctx context.Context, groupID string, status string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `UPDATE tontine_groups SET status = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3`
	result, err := r.execContext(ctx, query, status, groupID, shopID)
	if err != nil {
		return fmt.Errorf("failed to update group status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tontine group not found")
	}
	return nil
}

// IncrementCycle passe le groupe au cycle suivant
func (r *TontineGroupRepositoryInfrastructure) IncrementCycle(ctx context.Context, groupID string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `UPDATE tontine_groups SET current_cycle = current_cycle + 1, updated_at = NOW() WHERE id = $1 AND shop_id = $2`
	result, err := r.execContext(ctx, query, groupID, shopID)
	if err != nil {
		return fmt.Errorf("failed to increment cycle: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tontine group not found")
	}
	return nil
}

// Start démarre le groupe
func (r *TontineGroupRepositoryInfrastructure) Start(ctx context.Context, groupID string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `UPDATE tontine_groups SET status = 'ACTIVE', started_at = NOW(), updated_at = NOW() WHERE id = $1 AND shop_id = $2`
	result, err := r.execContext(ctx, query, groupID, shopID)
	if err != nil {
		return fmt.Errorf("failed to start group: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tontine group not found")
	}
	return nil
}

// Complete marque le groupe comme terminé
func (r *TontineGroupRepositoryInfrastructure) Complete(ctx context.Context, groupID string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `UPDATE tontine_groups SET status = 'COMPLETED', completed_at = NOW(), updated_at = NOW() WHERE id = $1 AND shop_id = $2`
	result, err := r.execContext(ctx, query, groupID, shopID)
	if err != nil {
		return fmt.Errorf("failed to complete group: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tontine group not found")
	}
	return nil
}
