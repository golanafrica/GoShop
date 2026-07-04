package shop

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
// 🆕 v4.2.0 : SHOP ADMIN ACTION REPOSITORY
// ============================================================
//
// 🎯 Objectif :
//   Implémenter l'audit trail complet des actions admin sur les shops.
//   Inspiré d'Amazon Activity Log.
//
// 📋 Actions tracées :
//   - suspend, activate, change_plan
//   - update_health, add_note, review
//   - export_data
//
// 🔐 Sécurité :
//   - IP address de l'admin
//   - User agent (navigateur)
//   - Request ID (corrélation logs)
//   - Old/New values (JSONB)
//
// ============================================================

// ShopAdminActionRepositoryInfrastructure implémente repository.ShopAdminActionRepository
type ShopAdminActionRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewShopAdminActionRepositoryInfrastructure crée une nouvelle instance
func NewShopAdminActionRepositoryInfrastructure(db *sql.DB) repository.ShopAdminActionRepository {
	return &ShopAdminActionRepositoryInfrastructure{db: db}
}

// WithTX retourne un nouveau repository avec transaction
func (r *ShopAdminActionRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ShopAdminActionRepository {
	return &ShopAdminActionRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS TX
// ============================================================

func (r *ShopAdminActionRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *ShopAdminActionRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *ShopAdminActionRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// CREATE
// ============================================================

// Create crée une nouvelle action admin
func (r *ShopAdminActionRepositoryInfrastructure) Create(ctx context.Context, action *entity.ShopAdminAction) error {
	query := `
		INSERT INTO shop_admin_actions (
			id, shop_id, admin_id, admin_email, admin_role,
			action_type, old_value, new_value, reason,
			ip_address, user_agent, request_id,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10::inet, $11, $12,
			$13
		)
	`

	// Sérialiser old_value et new_value en JSON
	var oldValueJSON, newValueJSON []byte
	var err error

	if action.OldValue != nil {
		oldValueJSON, err = json.Marshal(action.OldValue)
		if err != nil {
			return fmt.Errorf("marshal old_value: %w", err)
		}
	}

	if action.NewValue != nil {
		newValueJSON, err = json.Marshal(action.NewValue)
		if err != nil {
			return fmt.Errorf("marshal new_value: %w", err)
		}
	}

	_, err = r.execContext(ctx, query,
		action.ID,
		action.ShopID,
		action.AdminID,
		action.AdminEmail,
		action.AdminRole,
		action.ActionType,
		oldValueJSON,
		newValueJSON,
		action.Reason,
		nullIfEmpty(action.IPAddress),
		nullIfEmpty(action.UserAgent),
		nullIfEmpty(action.RequestID),
		action.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create admin action: %w", err)
	}

	return nil
}

// ============================================================
// FIND METHODS
// ============================================================

// FindByID retourne une action par son ID
func (r *ShopAdminActionRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.ShopAdminAction, error) {
	query := `
		SELECT id, shop_id, admin_id, admin_email, admin_role,
		       action_type, old_value, new_value, reason,
		       COALESCE(ip_address::text, ''), 
		       COALESCE(user_agent, ''),
		       COALESCE(request_id, ''),
		       created_at
		FROM shop_admin_actions
		WHERE id = $1
	`
	return r.scanAction(r.queryRowContext(ctx, query, id))
}

// FindByShopID retourne toutes les actions d'un shop
func (r *ShopAdminActionRepositoryInfrastructure) FindByShopID(
	ctx context.Context,
	shopID uuid.UUID,
	limit, offset int,
) ([]*entity.ShopAdminAction, error) {
	query := `
		SELECT id, shop_id, admin_id, admin_email, admin_role,
		       action_type, old_value, new_value, reason,
		       COALESCE(ip_address::text, ''),
		       COALESCE(user_agent, ''),
		       COALESCE(request_id, ''),
		       created_at
		FROM shop_admin_actions
		WHERE shop_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.scanActions(ctx, query, shopID, limit, offset)
}

// FindByAdminID retourne toutes les actions d'un admin
func (r *ShopAdminActionRepositoryInfrastructure) FindByAdminID(
	ctx context.Context,
	adminID string,
	limit, offset int,
) ([]*entity.ShopAdminAction, error) {
	query := `
		SELECT id, shop_id, admin_id, admin_email, admin_role,
		       action_type, old_value, new_value, reason,
		       COALESCE(ip_address::text, ''),
		       COALESCE(user_agent, ''),
		       COALESCE(request_id, ''),
		       created_at
		FROM shop_admin_actions
		WHERE admin_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.scanActions(ctx, query, adminID, limit, offset)
}

// FindByActionType retourne les actions par type
func (r *ShopAdminActionRepositoryInfrastructure) FindByActionType(
	ctx context.Context,
	actionType entity.ShopAdminActionType,
	limit, offset int,
) ([]*entity.ShopAdminAction, error) {
	query := `
		SELECT id, shop_id, admin_id, admin_email, admin_role,
		       action_type, old_value, new_value, reason,
		       COALESCE(ip_address::text, ''),
		       COALESCE(user_agent, ''),
		       COALESCE(request_id, ''),
		       created_at
		FROM shop_admin_actions
		WHERE action_type = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.scanActions(ctx, query, actionType, limit, offset)
}

// FindRecent retourne les actions récentes (tous shops)
func (r *ShopAdminActionRepositoryInfrastructure) FindRecent(
	ctx context.Context,
	limit, offset int,
) ([]*entity.ShopAdminAction, error) {
	query := `
		SELECT id, shop_id, admin_id, admin_email, admin_role,
		       action_type, old_value, new_value, reason,
		       COALESCE(ip_address::text, ''),
		       COALESCE(user_agent, ''),
		       COALESCE(request_id, ''),
		       created_at
		FROM shop_admin_actions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	return r.scanActions(ctx, query, limit, offset)
}

// ============================================================
// COUNT METHODS
// ============================================================

// CountByShopID compte les actions d'un shop
func (r *ShopAdminActionRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM shop_admin_actions WHERE shop_id = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count actions by shop: %w", err)
	}
	return count, nil
}

// CountByAdminID compte les actions d'un admin
func (r *ShopAdminActionRepositoryInfrastructure) CountByAdminID(ctx context.Context, adminID string) (int, error) {
	query := `SELECT COUNT(*) FROM shop_admin_actions WHERE admin_id = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, adminID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count actions by admin: %w", err)
	}
	return count, nil
}

// CountRecent compte les actions récentes
func (r *ShopAdminActionRepositoryInfrastructure) CountRecent(ctx context.Context, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM shop_admin_actions WHERE created_at >= $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recent actions: %w", err)
	}
	return count, nil
}

// ============================================================
// DELETE METHODS
// ============================================================

// DeleteByShopID supprime toutes les actions d'un shop
func (r *ShopAdminActionRepositoryInfrastructure) DeleteByShopID(ctx context.Context, shopID uuid.UUID) error {
	query := `DELETE FROM shop_admin_actions WHERE shop_id = $1`
	_, err := r.execContext(ctx, query, shopID)
	if err != nil {
		return fmt.Errorf("delete actions by shop: %w", err)
	}
	return nil
}

// ============================================================
// SCAN HELPERS
// ============================================================

// scanAction scanne une ligne depuis sql.Row
func (r *ShopAdminActionRepositoryInfrastructure) scanAction(row *sql.Row) (*entity.ShopAdminAction, error) {
	action := &entity.ShopAdminAction{}
	var oldValueJSON, newValueJSON []byte

	err := row.Scan(
		&action.ID,
		&action.ShopID,
		&action.AdminID,
		&action.AdminEmail,
		&action.AdminRole,
		&action.ActionType,
		&oldValueJSON,
		&newValueJSON,
		&action.Reason,
		&action.IPAddress,
		&action.UserAgent,
		&action.RequestID,
		&action.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Désérialiser JSON
	if len(oldValueJSON) > 0 {
		if err := json.Unmarshal(oldValueJSON, &action.OldValue); err != nil {
			return nil, fmt.Errorf("unmarshal old_value: %w", err)
		}
	}

	if len(newValueJSON) > 0 {
		if err := json.Unmarshal(newValueJSON, &action.NewValue); err != nil {
			return nil, fmt.Errorf("unmarshal new_value: %w", err)
		}
	}

	return action, nil
}

// scanActions scanne plusieurs lignes
func (r *ShopAdminActionRepositoryInfrastructure) scanActions(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]*entity.ShopAdminAction, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []*entity.ShopAdminAction
	for rows.Next() {
		action := &entity.ShopAdminAction{}
		var oldValueJSON, newValueJSON []byte

		err := rows.Scan(
			&action.ID,
			&action.ShopID,
			&action.AdminID,
			&action.AdminEmail,
			&action.AdminRole,
			&action.ActionType,
			&oldValueJSON,
			&newValueJSON,
			&action.Reason,
			&action.IPAddress,
			&action.UserAgent,
			&action.RequestID,
			&action.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(oldValueJSON) > 0 {
			if err := json.Unmarshal(oldValueJSON, &action.OldValue); err != nil {
				return nil, fmt.Errorf("unmarshal old_value: %w", err)
			}
		}

		if len(newValueJSON) > 0 {
			if err := json.Unmarshal(newValueJSON, &action.NewValue); err != nil {
				return nil, fmt.Errorf("unmarshal new_value: %w", err)
			}
		}

		actions = append(actions, action)
	}

	return actions, rows.Err()
}

// ============================================================
// HELPERS
// ============================================================

// nullIfEmpty retourne nil si la chaîne est vide (pour INET)
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
