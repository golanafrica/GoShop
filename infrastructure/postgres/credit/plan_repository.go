package credit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CreditPlanRepositoryInfrastructure implémente repository.CreditPlanRepository
type CreditPlanRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCreditPlanRepositoryInfrastructure crée une nouvelle instance
func NewCreditPlanRepositoryInfrastructure(db *sql.DB) repository.CreditPlanRepository {
	return &CreditPlanRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CreditPlanRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CreditPlanRepository {
	return &CreditPlanRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *CreditPlanRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CreditPlanRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CreditPlanRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *CreditPlanRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanPlan scanne une ligne dans une entité CreditPlan
func (r *CreditPlanRepositoryInfrastructure) scanPlan(row *sql.Row) (*entity.CreditPlan, error) {
	plan := &entity.CreditPlan{}

	err := row.Scan(
		&plan.ID,
		&plan.ProductID,
		&plan.ShopID,
		&plan.IsEnabled,
		&plan.MinDownPaymentPercent,
		&plan.MaxDurationMonths,
		&plan.InterestRateBps,
		&plan.PenaltyRateBps,
		&plan.MinCreditScore,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit plan not found")
		}
		return nil, fmt.Errorf("failed to scan credit plan: %w", err)
	}

	return plan, nil
}

// scanPlans scanne plusieurs lignes
func (r *CreditPlanRepositoryInfrastructure) scanPlans(ctx context.Context, query string, args ...interface{}) ([]*entity.CreditPlan, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query credit plans: %w", err)
	}
	defer rows.Close()

	var plans []*entity.CreditPlan
	for rows.Next() {
		plan := &entity.CreditPlan{}
		err := rows.Scan(
			&plan.ID,
			&plan.ProductID,
			&plan.ShopID,
			&plan.IsEnabled,
			&plan.MinDownPaymentPercent,
			&plan.MaxDurationMonths,
			&plan.InterestRateBps,
			&plan.PenaltyRateBps,
			&plan.MinCreditScore,
			&plan.CreatedAt,
			&plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		plans = append(plans, plan)
	}

	if plans == nil {
		plans = []*entity.CreditPlan{}
	}
	return plans, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée un nouveau plan de crédit
func (r *CreditPlanRepositoryInfrastructure) Create(ctx context.Context, plan *entity.CreditPlan) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le shop correspond au contexte multi-tenant
	if plan.ShopID != shopID {
		return fmt.Errorf("access denied: plan shop_id does not match tenant shop")
	}

	// Valider le plan
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO credit_plans (
			product_id, shop_id, is_enabled,
			min_down_payment_percent, max_duration_months,
			interest_rate_bps, penalty_rate_bps,
			min_credit_score,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		plan.ProductID,
		plan.ShopID,
		plan.IsEnabled,
		plan.MinDownPaymentPercent,
		plan.MaxDurationMonths,
		plan.InterestRateBps,
		plan.PenaltyRateBps,
		plan.MinCreditScore,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create credit plan: %w", err)
	}

	return nil
}

// FindByProductID trouve un plan par produit
func (r *CreditPlanRepositoryInfrastructure) FindByProductID(ctx context.Context, productID string) (*entity.CreditPlan, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, shop_id, is_enabled,
		       min_down_payment_percent, max_duration_months,
		       interest_rate_bps, penalty_rate_bps,
		       min_credit_score,
		       created_at, updated_at
		FROM credit_plans
		WHERE product_id = $1 AND shop_id = $2
	`

	return r.scanPlan(r.queryRowContext(ctx, query, productID, shopID))
}

// FindByShopID retourne tous les plans d'une boutique
func (r *CreditPlanRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.CreditPlan, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query plans of another shop")
	}

	query := `
		SELECT id, product_id, shop_id, is_enabled,
		       min_down_payment_percent, max_duration_months,
		       interest_rate_bps, penalty_rate_bps,
		       min_credit_score,
		       created_at, updated_at
		FROM credit_plans
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	return r.scanPlans(ctx, query, shopID)
}

// FindEnabledByShopID retourne les plans activés d'une boutique
func (r *CreditPlanRepositoryInfrastructure) FindEnabledByShopID(ctx context.Context, shopID string) ([]*entity.CreditPlan, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query plans of another shop")
	}

	query := `
		SELECT id, product_id, shop_id, is_enabled,
		       min_down_payment_percent, max_duration_months,
		       interest_rate_bps, penalty_rate_bps,
		       min_credit_score,
		       created_at, updated_at
		FROM credit_plans
		WHERE shop_id = $1 AND is_enabled = true
		ORDER BY created_at DESC
	`

	return r.scanPlans(ctx, query, shopID)
}

// Update met à jour un plan
func (r *CreditPlanRepositoryInfrastructure) Update(ctx context.Context, plan *entity.CreditPlan) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le plan appartient à la boutique
	var planShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM credit_plans WHERE id = $1`, plan.ID).Scan(&planShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("credit plan not found")
		}
		return fmt.Errorf("failed to verify plan: %w", err)
	}
	if planShopID != shopID {
		return fmt.Errorf("access denied: plan does not belong to tenant shop")
	}

	// Valider le plan
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE credit_plans
		SET is_enabled = $2,
		    min_down_payment_percent = $3,
		    max_duration_months = $4,
		    interest_rate_bps = $5,
		    penalty_rate_bps = $6,
		    min_credit_score = $7,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.queryRowContext(ctx, query,
		plan.ID,
		plan.IsEnabled,
		plan.MinDownPaymentPercent,
		plan.MaxDurationMonths,
		plan.InterestRateBps,
		plan.PenaltyRateBps,
		plan.MinCreditScore,
	).Scan(&plan.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update credit plan: %w", err)
	}

	return nil
}

// Upsert crée ou met à jour un plan
func (r *CreditPlanRepositoryInfrastructure) Upsert(ctx context.Context, plan *entity.CreditPlan) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le plan appartient à la boutique
	if plan.ShopID != shopID {
		return fmt.Errorf("access denied: plan shop_id does not match tenant shop")
	}

	// Valider le plan
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO credit_plans (
			product_id, shop_id, is_enabled,
			min_down_payment_percent, max_duration_months,
			interest_rate_bps, penalty_rate_bps,
			min_credit_score,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (product_id) DO UPDATE SET
			is_enabled = EXCLUDED.is_enabled,
			min_down_payment_percent = EXCLUDED.min_down_payment_percent,
			max_duration_months = EXCLUDED.max_duration_months,
			interest_rate_bps = EXCLUDED.interest_rate_bps,
			penalty_rate_bps = EXCLUDED.penalty_rate_bps,
			min_credit_score = EXCLUDED.min_credit_score,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		plan.ProductID,
		plan.ShopID,
		plan.IsEnabled,
		plan.MinDownPaymentPercent,
		plan.MaxDurationMonths,
		plan.InterestRateBps,
		plan.PenaltyRateBps,
		plan.MinCreditScore,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert credit plan: %w", err)
	}

	return nil
}

// Delete supprime un plan
func (r *CreditPlanRepositoryInfrastructure) Delete(ctx context.Context, productID string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		DELETE FROM credit_plans
		WHERE product_id = $1 AND shop_id = $2
	`

	result, err := r.execContext(ctx, query, productID, shopID)
	if err != nil {
		return fmt.Errorf("failed to delete credit plan: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("credit plan not found")
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si un plan existe pour un produit
func (r *CreditPlanRepositoryInfrastructure) Exists(ctx context.Context, productID string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_plans
		WHERE product_id = $1 AND shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, productID, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check plan existence: %w", err)
	}

	return count > 0, nil
}

// IsEnabled vérifie si le crédit est activé pour un produit
func (r *CreditPlanRepositoryInfrastructure) IsEnabled(ctx context.Context, productID string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT is_enabled FROM credit_plans
		WHERE product_id = $1 AND shop_id = $2
	`

	var isEnabled bool
	err = r.queryRowContext(ctx, query, productID, shopID).Scan(&isEnabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Pas de plan = pas de crédit
		}
		return false, fmt.Errorf("failed to check plan enabled: %w", err)
	}

	return isEnabled, nil
}

// CountByShopID compte les plans d'une boutique
func (r *CreditPlanRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID string) (int, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count plans of another shop")
	}

	query := `SELECT COUNT(*) FROM credit_plans WHERE shop_id = $1`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count credit plans: %w", err)
	}

	return count, nil
}

// UpdateTimestamp met à jour uniquement le timestamp updated_at
func (r *CreditPlanRepositoryInfrastructure) UpdateTimestamp(ctx context.Context, id string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE credit_plans
		SET updated_at = NOW()
		WHERE id = $1 AND shop_id = $2
	`

	result, err := r.execContext(ctx, query, id, shopID)
	if err != nil {
		return fmt.Errorf("failed to update timestamp: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("credit plan not found")
	}

	return nil
}

// ============================================================
// INITIALISATION
// ============================================================

// EnsureWalletExists crée un wallet pour un shop s'il n'existe pas
// Cette méthode est appelée lors du premier plan de crédit créé
func (r *CreditPlanRepositoryInfrastructure) EnsureWalletExists(ctx context.Context, shopID string) error {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if shopID != currentShopID {
		return fmt.Errorf("access denied: cannot create wallet for another shop")
	}

	query := `
		INSERT INTO merchant_wallets (shop_id, balance_cents, created_at, updated_at)
		VALUES ($1, 0, NOW(), NOW())
		ON CONFLICT (shop_id) DO NOTHING
	`

	_, err = r.execContext(ctx, query, shopID)
	if err != nil {
		return fmt.Errorf("failed to ensure wallet exists: %w", err)
	}

	return nil
}

// ============================================================
// MÉTHODES DE DEBUG / TEST
// ============================================================

// FindByID trouve un plan par ID (pour tests/debug)
func (r *CreditPlanRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.CreditPlan, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, product_id, shop_id, is_enabled,
		       min_down_payment_percent, max_duration_months,
		       interest_rate_bps, penalty_rate_bps,
		       min_credit_score,
		       created_at, updated_at
		FROM credit_plans
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanPlan(r.queryRowContext(ctx, query, id, shopID))
}

// FindAll retourne tous les plans (admin only, sans filtre multi-tenant)
// ⚠️ À utiliser avec précaution
func (r *CreditPlanRepositoryInfrastructure) FindAll(ctx context.Context) ([]*entity.CreditPlan, error) {
	query := `
		SELECT id, product_id, shop_id, is_enabled,
		       min_down_payment_percent, max_duration_months,
		       interest_rate_bps, penalty_rate_bps,
		       min_credit_score,
		       created_at, updated_at
		FROM credit_plans
		ORDER BY created_at DESC
	`

	return r.scanPlans(ctx, query)
}

// GetLastUpdated retourne la date de dernière mise à jour d'un plan
func (r *CreditPlanRepositoryInfrastructure) GetLastUpdated(ctx context.Context, id string) (*time.Time, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `SELECT updated_at FROM credit_plans WHERE id = $1 AND shop_id = $2`

	var updatedAt time.Time
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit plan not found")
		}
		return nil, fmt.Errorf("failed to get last updated: %w", err)
	}

	return &updatedAt, nil
}
