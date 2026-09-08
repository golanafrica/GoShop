package installment

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

type orderInstallmentRepository struct {
	db repository.DBExecutor
}

func NewOrderInstallmentRepository(db repository.DBExecutor) repository.OrderInstallmentRepository {
	return &orderInstallmentRepository{db: db}
}

func (r *orderInstallmentRepository) CreateBatch(ctx context.Context, installments []*entity.OrderInstallment) error {
	if len(installments) == 0 {
		return nil
	}

	query := `
		INSERT INTO order_installments (id, order_id, tranche_number, amount_cents, due_date, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	for _, inst := range installments {
		_, err := r.db.ExecContext(ctx, query, inst.ID, inst.OrderID, inst.TrancheNumber, inst.AmountCents, inst.DueDate, inst.Status, inst.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to create installment %d: %w", inst.TrancheNumber, err)
		}
	}
	return nil
}

func (r *orderInstallmentRepository) GetByOrderID(ctx context.Context, orderID string) ([]*entity.OrderInstallment, error) {
	query := `
		SELECT id, order_id, tranche_number, amount_cents, due_date, status, paid_at, payment_ref, created_at
		FROM order_installments
		WHERE order_id = $1
		ORDER BY tranche_number ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get installments: %w", err)
	}
	defer rows.Close()

	var installments []*entity.OrderInstallment
	for rows.Next() {
		var inst entity.OrderInstallment
		err := rows.Scan(&inst.ID, &inst.OrderID, &inst.TrancheNumber, &inst.AmountCents, &inst.DueDate, &inst.Status, &inst.PaidAt, &inst.PaymentRef, &inst.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan installment: %w", err)
		}
		installments = append(installments, &inst)
	}
	return installments, rows.Err()
}

func (r *orderInstallmentRepository) MarkAsPaid(ctx context.Context, id string, paymentRef string) error {
	query := `
		UPDATE order_installments
		SET status = 'paid', paid_at = NOW(), payment_ref = $2
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, paymentRef)
	return err
}

func (r *orderInstallmentRepository) GetOverdueInstallments(ctx context.Context) ([]*entity.OrderInstallment, error) {
	query := `
		SELECT id, order_id, tranche_number, amount_cents, due_date, status, paid_at, payment_ref, created_at
		FROM order_installments
		WHERE status = 'pending' AND due_date < NOW()
		ORDER BY due_date ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get overdue installments: %w", err)
	}
	defer rows.Close()

	var installments []*entity.OrderInstallment
	for rows.Next() {
		var inst entity.OrderInstallment
		err := rows.Scan(&inst.ID, &inst.OrderID, &inst.TrancheNumber, &inst.AmountCents, &inst.DueDate, &inst.Status, &inst.PaidAt, &inst.PaymentRef, &inst.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan overdue installment: %w", err)
		}
		installments = append(installments, &inst)
	}
	return installments, rows.Err()
}

func (r *orderInstallmentRepository) MarkAsOverdue(ctx context.Context, id string) error {
	query := `UPDATE order_installments SET status = 'overdue' WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
