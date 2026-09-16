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
	tx repository.Tx // 🆕 AJOUTÉ pour supporter les transactions
}

func NewWithdrawalRepositoryPostgres(db *sql.DB) repository.WithdrawalRepository {
	return &WithdrawalRepositoryPostgres{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *WithdrawalRepositoryPostgres) WithTX(tx repository.Tx) repository.WithdrawalRepository {
	return &WithdrawalRepositoryPostgres{tx: tx, db: r.db}
}

// Helpers pour exécuter les requêtes (soit via tx, soit via db)
func (r *WithdrawalRepositoryPostgres) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *WithdrawalRepositoryPostgres) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *WithdrawalRepositoryPostgres) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
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
	_, err := r.execContext(ctx, query,
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
	return r.scan(r.queryRowContext(ctx, query, id))
}

// 🆕 FindByProviderRef trouve un retrait par sa référence YengaPay (pour les webhooks)
func (r *WithdrawalRepositoryPostgres) FindByProviderRef(ctx context.Context, providerRef string) (*entity.Withdrawal, error) {
	query := `
		SELECT id, shop_id, provider, provider_ref, amount, currency, fees, net_amount,
		       status, payment_method, destination_number, destination_name, destination_email,
		       description, error_message, operator_transaction_id, processed_at, created_at, updated_at
		FROM withdrawals
		WHERE provider_ref = $1
	`
	return r.scan(r.queryRowContext(ctx, query, providerRef))
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
	rows, err := r.queryContext(ctx, query, shopID, limit, offset)
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
	_, err := r.execContext(ctx, query,
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
	err := r.queryRowContext(ctx, query, shopID).Scan(&count)
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
		return nil, fmt.Errorf("withdrawal not found")
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
