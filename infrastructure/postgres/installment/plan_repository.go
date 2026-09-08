package installment

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

type installmentPlanRepository struct {
	db repository.DBExecutor
}

func NewInstallmentPlanRepository(db repository.DBExecutor) repository.InstallmentPlanRepository {
	return &installmentPlanRepository{db: db}
}

func (r *installmentPlanRepository) Create(ctx context.Context, plan *entity.InstallmentPlan) error {
	query := `
		INSERT INTO installment_plans (id, product_id, shop_id, nb_tranches, delai_jours, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query, plan.ID, plan.ProductID, plan.ShopID, plan.NbTranches, plan.DelaiJours, plan.IsActive, plan.CreatedAt, plan.UpdatedAt)
	return err
}

func (r *installmentPlanRepository) Update(ctx context.Context, plan *entity.InstallmentPlan) error {
	query := `
		UPDATE installment_plans 
		SET is_active = $1, updated_at = $2 
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, plan.IsActive, plan.UpdatedAt, plan.ID)
	return err
}

func (r *installmentPlanRepository) GetByProductID(ctx context.Context, productID string) (*entity.InstallmentPlan, error) {
	query := `
		SELECT id, product_id, shop_id, nb_tranches, delai_jours, is_active, created_at, updated_at 
		FROM installment_plans 
		WHERE product_id = $1
	`
	var plan entity.InstallmentPlan
	err := r.db.QueryRowContext(ctx, query, productID).Scan(
		&plan.ID, &plan.ProductID, &plan.ShopID, &plan.NbTranches,
		&plan.DelaiJours, &plan.IsActive, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Pas d'erreur, juste aucun plan configuré
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get installment plan: %w", err)
	}
	return &plan, nil
}

func (r *installmentPlanRepository) ListByShopID(ctx context.Context, shopID string) ([]*entity.InstallmentPlan, error) {
	query := `
		SELECT id, product_id, shop_id, nb_tranches, delai_jours, is_active, created_at, updated_at 
		FROM installment_plans 
		WHERE shop_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to list installment plans: %w", err)
	}
	defer rows.Close()

	var plans []*entity.InstallmentPlan
	for rows.Next() {
		var plan entity.InstallmentPlan
		if err := rows.Scan(&plan.ID, &plan.ProductID, &plan.ShopID, &plan.NbTranches, &plan.DelaiJours, &plan.IsActive, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan installment plan: %w", err)
		}
		plans = append(plans, &plan)
	}
	return plans, rows.Err()
}

func (r *installmentPlanRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM installment_plans WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
