package freeze

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// AccountFreezeRepositoryInfrastructure implémente repository.AccountFreezeRepository
type AccountFreezeRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewAccountFreezeRepositoryInfrastructure crée une nouvelle instance
func NewAccountFreezeRepositoryInfrastructure(db *sql.DB) repository.AccountFreezeRepository {
	return &AccountFreezeRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *AccountFreezeRepositoryInfrastructure) WithTX(tx repository.Tx) repository.AccountFreezeRepository {
	return &AccountFreezeRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *AccountFreezeRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *AccountFreezeRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *AccountFreezeRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *AccountFreezeRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanFreeze scanne une ligne dans une entité AccountFreeze
func (r *AccountFreezeRepositoryInfrastructure) scanFreeze(row *sql.Row) (*entity.AccountFreeze, error) {
	freeze := &entity.AccountFreeze{}
	var freezeDetails sql.NullString
	var resolvedAt sql.NullTime
	var resolution sql.NullString
	var resolvedBy sql.NullString
	var reminder1, reminder2, reminder3 sql.NullTime

	err := row.Scan(
		&freeze.ID,
		&freeze.ShopID,
		&freeze.FreezeReason,
		&freezeDetails,
		&freeze.AmountDueCents,
		&freeze.FrozenAt,
		&freeze.GracePeriodDays,
		&freeze.GracePeriodEndsAt,
		&resolvedAt,
		&resolution,
		&resolvedBy,
		&reminder1,
		&reminder2,
		&reminder3,
		&freeze.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account freeze not found")
		}
		return nil, fmt.Errorf("failed to scan account freeze: %w", err)
	}

	if freezeDetails.Valid {
		freeze.FreezeDetails = &freezeDetails.String
	}
	if resolvedAt.Valid {
		freeze.ResolvedAt = &resolvedAt.Time
	}
	if resolution.Valid {
		res := entity.FreezeResolution(resolution.String)
		freeze.Resolution = &res
	}
	if resolvedBy.Valid {
		freeze.ResolvedBy = &resolvedBy.String
	}
	if reminder1.Valid {
		freeze.Reminder1SentAt = &reminder1.Time
	}
	if reminder2.Valid {
		freeze.Reminder2SentAt = &reminder2.Time
	}
	if reminder3.Valid {
		freeze.Reminder3SentAt = &reminder3.Time
	}

	return freeze, nil
}

// scanFreezes scanne plusieurs lignes
func (r *AccountFreezeRepositoryInfrastructure) scanFreezes(ctx context.Context, query string, args ...interface{}) ([]*entity.AccountFreeze, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query account freezes: %w", err)
	}
	defer rows.Close()

	var freezes []*entity.AccountFreeze
	for rows.Next() {
		freeze := &entity.AccountFreeze{}
		var freezeDetails sql.NullString
		var resolvedAt sql.NullTime
		var resolution sql.NullString
		var resolvedBy sql.NullString
		var reminder1, reminder2, reminder3 sql.NullTime

		err := rows.Scan(
			&freeze.ID,
			&freeze.ShopID,
			&freeze.FreezeReason,
			&freezeDetails,
			&freeze.AmountDueCents,
			&freeze.FrozenAt,
			&freeze.GracePeriodDays,
			&freeze.GracePeriodEndsAt,
			&resolvedAt,
			&resolution,
			&resolvedBy,
			&reminder1,
			&reminder2,
			&reminder3,
			&freeze.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if freezeDetails.Valid {
			freeze.FreezeDetails = &freezeDetails.String
		}
		if resolvedAt.Valid {
			freeze.ResolvedAt = &resolvedAt.Time
		}
		if resolution.Valid {
			res := entity.FreezeResolution(resolution.String)
			freeze.Resolution = &res
		}
		if resolvedBy.Valid {
			freeze.ResolvedBy = &resolvedBy.String
		}
		if reminder1.Valid {
			freeze.Reminder1SentAt = &reminder1.Time
		}
		if reminder2.Valid {
			freeze.Reminder2SentAt = &reminder2.Time
		}
		if reminder3.Valid {
			freeze.Reminder3SentAt = &reminder3.Time
		}

		freezes = append(freezes, freeze)
	}

	if freezes == nil {
		freezes = []*entity.AccountFreeze{}
	}
	return freezes, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée un nouveau gel de compte
func (r *AccountFreezeRepositoryInfrastructure) Create(ctx context.Context, freeze *entity.AccountFreeze) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	if freeze.ShopID != shopID {
		return fmt.Errorf("access denied: freeze shop_id does not match tenant shop")
	}

	if err := freeze.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO account_freezes (
			shop_id, freeze_reason, freeze_details,
			amount_due_cents,
			frozen_at, grace_period_days, grace_period_ends_at,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, created_at
	`

	err = r.queryRowContext(ctx, query,
		freeze.ShopID,
		freeze.FreezeReason,
		freeze.FreezeDetails,
		freeze.AmountDueCents,
		freeze.FrozenAt,
		freeze.GracePeriodDays,
		freeze.GracePeriodEndsAt,
	).Scan(&freeze.ID, &freeze.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create account freeze: %w", err)
	}

	return nil
}

// FindByID trouve un gel par ID
func (r *AccountFreezeRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.AccountFreeze, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanFreeze(r.queryRowContext(ctx, query, id, shopID))
}

// FindByShopID trouve un gel par boutique
func (r *AccountFreezeRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) (*entity.AccountFreeze, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query freezes of another shop")
	}

	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE shop_id = $1
		ORDER BY frozen_at DESC
		LIMIT 1
	`

	return r.scanFreeze(r.queryRowContext(ctx, query, shopID))
}

// FindActiveByShopID trouve le gel actif d'une boutique (non résolu)
func (r *AccountFreezeRepositoryInfrastructure) FindActiveByShopID(ctx context.Context, shopID string) (*entity.AccountFreeze, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query freezes of another shop")
	}

	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE shop_id = $1 AND resolved_at IS NULL
		ORDER BY frozen_at DESC
		LIMIT 1
	`

	return r.scanFreeze(r.queryRowContext(ctx, query, shopID))
}

// FindAll retourne tous les gels
func (r *AccountFreezeRepositoryInfrastructure) FindAll(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		ORDER BY frozen_at DESC
	`

	return r.scanFreezes(ctx, query)
}

// FindActive retourne les gels actifs (non résolus)
func (r *AccountFreezeRepositoryInfrastructure) FindActive(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		ORDER BY frozen_at ASC
	`

	return r.scanFreezes(ctx, query)
}

// FindResolved retourne les gels résolus
func (r *AccountFreezeRepositoryInfrastructure) FindResolved(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NOT NULL
		ORDER BY resolved_at DESC
	`

	return r.scanFreezes(ctx, query)
}

// FindByReason retourne les gels par raison
func (r *AccountFreezeRepositoryInfrastructure) FindByReason(ctx context.Context, reason entity.FreezeReason) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE freeze_reason = $1
		ORDER BY frozen_at DESC
	`

	return r.scanFreezes(ctx, query, reason)
}

// FindByResolution retourne les gels par résolution
func (r *AccountFreezeRepositoryInfrastructure) FindByResolution(ctx context.Context, resolution entity.FreezeResolution) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolution = $1
		ORDER BY resolved_at DESC
	`

	return r.scanFreezes(ctx, query, resolution)
}

// FindGracePeriodExpired retourne les gels dont la période de grâce est expirée
func (r *AccountFreezeRepositoryInfrastructure) FindGracePeriodExpired(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		  AND grace_period_ends_at <= NOW()
		ORDER BY grace_period_ends_at ASC
	`

	return r.scanFreezes(ctx, query)
}

// FindGracePeriodExpiringSoon retourne les gels dont la période de grâce expire bientôt
func (r *AccountFreezeRepositoryInfrastructure) FindGracePeriodExpiringSoon(ctx context.Context, daysRemaining int) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		  AND grace_period_ends_at <= NOW() + INTERVAL '1 day' * $1
		  AND grace_period_ends_at > NOW()
		ORDER BY grace_period_ends_at ASC
	`

	return r.scanFreezes(ctx, query, daysRemaining)
}

// FindNeedsReminder1 retourne les gels nécessitant le rappel J+1
func (r *AccountFreezeRepositoryInfrastructure) FindNeedsReminder1(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		  AND reminder_1_sent_at IS NULL
		  AND frozen_at <= NOW() - INTERVAL '1 day'
		ORDER BY frozen_at ASC
	`

	return r.scanFreezes(ctx, query)
}

// FindNeedsReminder2 retourne les gels nécessitant le rappel J+3
func (r *AccountFreezeRepositoryInfrastructure) FindNeedsReminder2(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		  AND reminder_1_sent_at IS NOT NULL
		  AND reminder_2_sent_at IS NULL
		  AND frozen_at <= NOW() - INTERVAL '3 days'
		ORDER BY frozen_at ASC
	`

	return r.scanFreezes(ctx, query)
}

// FindNeedsReminder3 retourne les gels nécessitant le rappel J+6
func (r *AccountFreezeRepositoryInfrastructure) FindNeedsReminder3(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		  AND reminder_2_sent_at IS NOT NULL
		  AND reminder_3_sent_at IS NULL
		  AND frozen_at <= NOW() - INTERVAL '6 days'
		ORDER BY frozen_at ASC
	`

	return r.scanFreezes(ctx, query)
}

// FindNeedsSuspension retourne les gels nécessitant une suspension définitive
func (r *AccountFreezeRepositoryInfrastructure) FindNeedsSuspension(ctx context.Context) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE resolved_at IS NULL
		  AND grace_period_ends_at <= NOW()
		ORDER BY grace_period_ends_at ASC
	`

	return r.scanFreezes(ctx, query)
}

// FindHighValue retourne les gels avec montant dû élevé
func (r *AccountFreezeRepositoryInfrastructure) FindHighValue(ctx context.Context, minAmountCents int64) ([]*entity.AccountFreeze, error) {
	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE amount_due_cents >= $1
		ORDER BY amount_due_cents DESC
	`

	return r.scanFreezes(ctx, query, minAmountCents)
}

// CountActive compte les gels actifs
func (r *AccountFreezeRepositoryInfrastructure) CountActive(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM account_freezes WHERE resolved_at IS NULL`

	var count int
	err := r.queryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active freezes: %w", err)
	}

	return count, nil
}

// CountResolved compte les gels résolus
func (r *AccountFreezeRepositoryInfrastructure) CountResolved(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM account_freezes WHERE resolved_at IS NOT NULL`

	var count int
	err := r.queryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count resolved freezes: %w", err)
	}

	return count, nil
}

// CountByReason compte les gels par raison
func (r *AccountFreezeRepositoryInfrastructure) CountByReason(ctx context.Context, reason entity.FreezeReason) (int, error) {
	query := `SELECT COUNT(*) FROM account_freezes WHERE freeze_reason = $1`

	var count int
	err := r.queryRowContext(ctx, query, reason).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count freezes by reason: %w", err)
	}

	return count, nil
}

// CountByResolution compte les gels par résolution
func (r *AccountFreezeRepositoryInfrastructure) CountByResolution(ctx context.Context, resolution entity.FreezeResolution) (int, error) {
	query := `SELECT COUNT(*) FROM account_freezes WHERE resolution = $1`

	var count int
	err := r.queryRowContext(ctx, query, resolution).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count freezes by resolution: %w", err)
	}

	return count, nil
}

// CountGracePeriodExpired compte les gels avec période de grâce expirée
func (r *AccountFreezeRepositoryInfrastructure) CountGracePeriodExpired(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*) FROM account_freezes
		WHERE resolved_at IS NULL
		  AND grace_period_ends_at <= NOW()
	`

	var count int
	err := r.queryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count expired freezes: %w", err)
	}

	return count, nil
}

// SumAmountDueActive somme des montants dus pour les gels actifs
func (r *AccountFreezeRepositoryInfrastructure) SumAmountDueActive(ctx context.Context) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_due_cents), 0)
		FROM account_freezes
		WHERE resolved_at IS NULL
	`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum active amount due: %w", err)
	}

	return total, nil
}

// SumAmountDueByReason somme des montants dus par raison
func (r *AccountFreezeRepositoryInfrastructure) SumAmountDueByReason(ctx context.Context, reason entity.FreezeReason) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_due_cents), 0)
		FROM account_freezes
		WHERE freeze_reason = $1
	`

	var total int64
	err := r.queryRowContext(ctx, query, reason).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum amount due by reason: %w", err)
	}

	return total, nil
}

// SumAmountDueResolved somme des montants dus pour les gels résolus
func (r *AccountFreezeRepositoryInfrastructure) SumAmountDueResolved(ctx context.Context) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_due_cents), 0)
		FROM account_freezes
		WHERE resolved_at IS NOT NULL
	`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum resolved amount due: %w", err)
	}

	return total, nil
}

// SumTotalAmountDue somme totale des montants dus (tous gels)
func (r *AccountFreezeRepositoryInfrastructure) SumTotalAmountDue(ctx context.Context) (int64, error) {
	query := `SELECT COALESCE(SUM(amount_due_cents), 0) FROM account_freezes`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total amount due: %w", err)
	}

	return total, nil
}

// Update met à jour un gel
func (r *AccountFreezeRepositoryInfrastructure) Update(ctx context.Context, freeze *entity.AccountFreeze) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le gel appartient à la boutique
	var freezeShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM account_freezes WHERE id = $1`, freeze.ID).Scan(&freezeShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("account freeze not found")
		}
		return fmt.Errorf("failed to verify freeze: %w", err)
	}
	if freezeShopID != shopID {
		return fmt.Errorf("access denied: freeze does not belong to tenant shop")
	}

	if err := freeze.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE account_freezes
		SET freeze_details = $2,
		    resolved_at = $3,
		    resolution = $4,
		    resolved_by = $5,
		    reminder_1_sent_at = $6,
		    reminder_2_sent_at = $7,
		    reminder_3_sent_at = $8
		WHERE id = $1
	`

	_, err = r.execContext(ctx, query,
		freeze.ID,
		freeze.FreezeDetails,
		freeze.ResolvedAt,
		freeze.Resolution,
		freeze.ResolvedBy,
		freeze.Reminder1SentAt,
		freeze.Reminder2SentAt,
		freeze.Reminder3SentAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update account freeze: %w", err)
	}

	return nil
}

// UpdateResolution met à jour la résolution d'un gel
func (r *AccountFreezeRepositoryInfrastructure) UpdateResolution(ctx context.Context, id string, resolution entity.FreezeResolution, resolvedBy string) error {
	if !resolution.IsValid() {
		return fmt.Errorf("invalid resolution: %s", resolution)
	}
	if resolvedBy == "" {
		return fmt.Errorf("resolved_by is required")
	}

	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := r.execContext(ctx, `
		UPDATE account_freezes
		SET resolved_at = $2,
		    resolution = $3,
		    resolved_by = $4
		WHERE id = $1 AND shop_id = $5
	`, id, now, resolution, resolvedBy, shopID)
	if err != nil {
		return fmt.Errorf("failed to update resolution: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("account freeze not found")
	}

	return nil
}

// UpdateReminder1 met à jour la date du rappel J+1
func (r *AccountFreezeRepositoryInfrastructure) UpdateReminder1(ctx context.Context, id string, sentAt time.Time) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	result, err := r.execContext(ctx, `
		UPDATE account_freezes
		SET reminder_1_sent_at = $2
		WHERE id = $1 AND shop_id = $3
	`, id, sentAt, shopID)
	if err != nil {
		return fmt.Errorf("failed to update reminder 1: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("account freeze not found")
	}

	return nil
}

// UpdateReminder2 met à jour la date du rappel J+3
func (r *AccountFreezeRepositoryInfrastructure) UpdateReminder2(ctx context.Context, id string, sentAt time.Time) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	result, err := r.execContext(ctx, `
		UPDATE account_freezes
		SET reminder_2_sent_at = $2
		WHERE id = $1 AND shop_id = $3
	`, id, sentAt, shopID)
	if err != nil {
		return fmt.Errorf("failed to update reminder 2: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("account freeze not found")
	}

	return nil
}

// UpdateReminder3 met à jour la date du rappel J+6
func (r *AccountFreezeRepositoryInfrastructure) UpdateReminder3(ctx context.Context, id string, sentAt time.Time) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	result, err := r.execContext(ctx, `
		UPDATE account_freezes
		SET reminder_3_sent_at = $2
		WHERE id = $1 AND shop_id = $3
	`, id, sentAt, shopID)
	if err != nil {
		return fmt.Errorf("failed to update reminder 3: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("account freeze not found")
	}

	return nil
}

// ResolvePaid résout un gel par paiement
func (r *AccountFreezeRepositoryInfrastructure) ResolvePaid(ctx context.Context, id string, resolvedBy string) error {
	return r.UpdateResolution(ctx, id, entity.FreezeResolutionPaid, resolvedBy)
}

// ResolveSuspended résout un gel par suspension
func (r *AccountFreezeRepositoryInfrastructure) ResolveSuspended(ctx context.Context, id string, resolvedBy string) error {
	return r.UpdateResolution(ctx, id, entity.FreezeResolutionSuspended, resolvedBy)
}

// ResolveWaived résout un gel par annulation
func (r *AccountFreezeRepositoryInfrastructure) ResolveWaived(ctx context.Context, id string, resolvedBy string) error {
	return r.UpdateResolution(ctx, id, entity.FreezeResolutionWaived, resolvedBy)
}

// ResolveEscalated résout un gel par escalade
func (r *AccountFreezeRepositoryInfrastructure) ResolveEscalated(ctx context.Context, id string, resolvedBy string) error {
	return r.UpdateResolution(ctx, id, entity.FreezeResolutionEscalated, resolvedBy)
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si un gel existe
func (r *AccountFreezeRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `SELECT COUNT(*) FROM account_freezes WHERE id = $1 AND shop_id = $2`

	var count int
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check freeze existence: %w", err)
	}

	return count > 0, nil
}

// HasActiveFreeze vérifie si une boutique a un gel actif
func (r *AccountFreezeRepositoryInfrastructure) HasActiveFreeze(ctx context.Context, shopID string) (bool, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}
	if shopID != currentShopID {
		return false, fmt.Errorf("access denied: cannot query freezes of another shop")
	}

	query := `
		SELECT COUNT(*) FROM account_freezes
		WHERE shop_id = $1 AND resolved_at IS NULL
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check active freeze: %w", err)
	}

	return count > 0, nil
}

// CountByShopID compte les gels d'une boutique
func (r *AccountFreezeRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID string) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count freezes of another shop")
	}

	query := `SELECT COUNT(*) FROM account_freezes WHERE shop_id = $1`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count freezes: %w", err)
	}

	return count, nil
}

// SumAmountDueByShopID somme des montants dus pour une boutique
func (r *AccountFreezeRepositoryInfrastructure) SumAmountDueByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum freezes of another shop")
	}

	query := `
		SELECT COALESCE(SUM(amount_due_cents), 0)
		FROM account_freezes
		WHERE shop_id = $1 AND resolved_at IS NULL
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum amount due: %w", err)
	}

	return total, nil
}

// FindByShopIDAll retourne tous les gels d'une boutique
func (r *AccountFreezeRepositoryInfrastructure) FindByShopIDAll(ctx context.Context, shopID string) ([]*entity.AccountFreeze, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query freezes of another shop")
	}

	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE shop_id = $1
		ORDER BY frozen_at DESC
	`

	return r.scanFreezes(ctx, query, shopID)
}

// FindRecentByShopID retourne les N gels les plus récents d'une boutique
func (r *AccountFreezeRepositoryInfrastructure) FindRecentByShopID(ctx context.Context, shopID string, limit int) ([]*entity.AccountFreeze, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query freezes of another shop")
	}

	query := `
		SELECT id, shop_id, freeze_reason, freeze_details,
		       amount_due_cents,
		       frozen_at, grace_period_days, grace_period_ends_at,
		       resolved_at, resolution, resolved_by,
		       reminder_1_sent_at, reminder_2_sent_at, reminder_3_sent_at,
		       created_at
		FROM account_freezes
		WHERE shop_id = $1
		ORDER BY frozen_at DESC
		LIMIT $2
	`

	return r.scanFreezes(ctx, query, shopID, limit)
}
