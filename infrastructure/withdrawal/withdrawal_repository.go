package withdrawal

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

type WithdrawalRepositoryPostgres struct {
	db *sql.DB
}

func NewWithdrawalRepositoryPostgres(db *sql.DB) repository.WithdrawalRepository {
	return &WithdrawalRepositoryPostgres{db: db}
}

func (r *WithdrawalRepositoryPostgres) Create(ctx context.Context, w *entity.Withdrawal) error {
	query := `
		INSERT INTO withdrawals (
			id, shop_id, provider, provider_ref, amount, currency, fees, net_amount,
			status, payment_method, destination_number, destination_name, destination_email,
			description, error_message, operator_transaction_id, processed_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
	`
	_, err := r.db.ExecContext(ctx, query,
		w.ID, w.ShopID, w.Provider, w.ProviderRef, w.AmountCents, string(w.Currency),
		w.FeesCents, w.NetAmountCents, string(w.Status), string(w.PaymentMethod),
		w.DestinationNumber, w.DestinationName, w.DestinationEmail,
		w.Description, w.ErrorMessage, w.OperatorTransactionID, w.ProcessedAt,
		w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create withdrawal: %w", err)
	}
	return nil
}

func (r *WithdrawalRepositoryPostgres) FindByID(ctx context.Context, id uuid.UUID) (*entity.Withdrawal, error) {
	query := `
		SELECT id, shop_id, provider, provider_ref, amount, currency, fees, net_amount,
		       status, payment_method, destination_number, destination_name, destination_email,
		       description, error_message, operator_transaction_id, processed_at, created_at, updated_at
		FROM withdrawals
		WHERE id = $1
	`
	return r.scan(r.db.QueryRowContext(ctx, query, id))
}

func (r *WithdrawalRepositoryPostgres) FindByShopID(ctx context.Context, shopID uuid.UUID, limit, offset int) ([]*entity.Withdrawal, error) {
	query := `
		SELECT id, shop_id, provider, provider_ref, amount, currency, fees, net_amount,
		       status, payment_method, destination_number, destination_name, destination_email,
		       description, error_message, operator_transaction_id, processed_at, created_at, updated_at
		FROM withdrawals
		WHERE shop_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, shopID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []*entity.Withdrawal
	for rows.Next() {
		w, err := r.scanFromRows(rows)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}
	return withdrawals, rows.Err()
}

func (r *WithdrawalRepositoryPostgres) Update(ctx context.Context, w *entity.Withdrawal) error {
	query := `
		UPDATE withdrawals SET
			provider_ref = $2, status = $3, fees = $4, net_amount = $5,
			error_message = $6, operator_transaction_id = $7, processed_at = $8, updated_at = $9
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query,
		w.ID, w.ProviderRef, string(w.Status), w.FeesCents, w.NetAmountCents,
		w.ErrorMessage, w.OperatorTransactionID, w.ProcessedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update withdrawal: %w", err)
	}
	return nil
}

func (r *WithdrawalRepositoryPostgres) CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM withdrawals WHERE shop_id = $1`
	err := r.db.QueryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count withdrawals: %w", err)
	}
	return count, nil
}

func (r *WithdrawalRepositoryPostgres) scan(row *sql.Row) (*entity.Withdrawal, error) {
	w := &entity.Withdrawal{}
	err := row.Scan(
		&w.ID, &w.ShopID, &w.Provider, &w.ProviderRef, &w.AmountCents, &w.Currency,
		&w.FeesCents, &w.NetAmountCents, &w.Status, &w.PaymentMethod,
		&w.DestinationNumber, &w.DestinationName, &w.DestinationEmail,
		&w.Description, &w.ErrorMessage, &w.OperatorTransactionID, &w.ProcessedAt,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan withdrawal: %w", err)
	}
	return w, nil
}

func (r *WithdrawalRepositoryPostgres) scanFromRows(rows *sql.Rows) (*entity.Withdrawal, error) {
	w := &entity.Withdrawal{}
	err := rows.Scan(
		&w.ID, &w.ShopID, &w.Provider, &w.ProviderRef, &w.AmountCents, &w.Currency,
		&w.FeesCents, &w.NetAmountCents, &w.Status, &w.PaymentMethod,
		&w.DestinationNumber, &w.DestinationName, &w.DestinationEmail,
		&w.Description, &w.ErrorMessage, &w.OperatorTransactionID, &w.ProcessedAt,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan withdrawal: %w", err)
	}
	return w, nil
}
