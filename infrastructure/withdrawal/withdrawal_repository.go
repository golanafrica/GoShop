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
	tx repository.Tx
}

func NewWithdrawalRepositoryPostgres(db *sql.DB) repository.WithdrawalRepository {
	return &WithdrawalRepositoryPostgres{db: db}
}

func (r *WithdrawalRepositoryPostgres) WithTX(tx repository.Tx) repository.WithdrawalRepository {
	return &WithdrawalRepositoryPostgres{tx: tx, db: r.db}
}

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

const withdrawalSelectCols = `
	id, shop_id, provider, provider_ref, amount, currency, fees, net_amount,
	status, payment_method, destination_number, destination_name, destination_email,
	description, error_message, operator_transaction_id, processed_at, created_at, updated_at,
	COALESCE(compensation_attempts, 0), last_compensation_at, failure_reason
`

func (r *WithdrawalRepositoryPostgres) Create(ctx context.Context, w *entity.Withdrawal) error {
	query := `
		INSERT INTO withdrawals (
			id, shop_id, provider, provider_ref, amount, currency, fees, net_amount,
			status, payment_method, destination_number, destination_name, destination_email,
			description, error_message, operator_transaction_id, processed_at, created_at, updated_at,
			compensation_attempts, last_compensation_at, failure_reason
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22
		)
	`
	_, err := r.execContext(ctx, query,
		w.ID, w.ShopID, w.Provider, w.ProviderRef, w.AmountCents, string(w.Currency),
		w.FeesCents, w.NetAmountCents, string(w.Status), string(w.PaymentMethod),
		w.DestinationNumber, w.DestinationName, w.DestinationEmail,
		w.Description, w.ErrorMessage, w.OperatorTransactionID, w.ProcessedAt,
		w.CreatedAt, w.UpdatedAt,
		w.CompensationAttempts, w.LastCompensationAt, w.FailureReason,
	)
	if err != nil {
		return fmt.Errorf("create withdrawal: %w", err)
	}
	return nil
}

func (r *WithdrawalRepositoryPostgres) FindByID(ctx context.Context, id uuid.UUID) (*entity.Withdrawal, error) {
	query := `SELECT ` + withdrawalSelectCols + ` FROM withdrawals WHERE id = $1`
	return r.scan(r.queryRowContext(ctx, query, id))
}

func (r *WithdrawalRepositoryPostgres) FindByProviderRef(ctx context.Context, providerRef string) (*entity.Withdrawal, error) {
	query := `SELECT ` + withdrawalSelectCols + ` FROM withdrawals WHERE provider_ref = $1`
	return r.scan(r.queryRowContext(ctx, query, providerRef))
}

func (r *WithdrawalRepositoryPostgres) FindByShopID(ctx context.Context, shopID uuid.UUID, limit, offset int) ([]*entity.Withdrawal, error) {
	query := `
		SELECT ` + withdrawalSelectCols + `
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

// FindCompensationPending file d'attente du job retry (FIFO).
// 🛡️ CORRECTION CRITIQUE P1 : Filtre STRICTEMENT sur 'compensation_pending'.
// Ne doit JAMAIS inclure 'reconciliation_required' pour éviter un reverse automatique non désiré.
func (r *WithdrawalRepositoryPostgres) FindCompensationPending(
	ctx context.Context,
	limit int,
	maxAttempts int,
) ([]*entity.Withdrawal, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT ` + withdrawalSelectCols + `
		FROM withdrawals
		WHERE status = 'compensation_pending'
	`
	args := []interface{}{}
	argN := 1

	if maxAttempts > 0 {
		query += fmt.Sprintf(` AND COALESCE(compensation_attempts, 0) < $%d`, argN)
		args = append(args, maxAttempts)
		argN++
	}

	query += fmt.Sprintf(`
		ORDER BY COALESCE(last_compensation_at, created_at) ASC, created_at ASC
		LIMIT $%d
	`, argN)
	args = append(args, limit)

	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find compensation_pending: %w", err)
	}
	defer rows.Close()

	var out []*entity.Withdrawal
	for rows.Next() {
		w, err := r.scanFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WithdrawalRepositoryPostgres) Update(ctx context.Context, w *entity.Withdrawal) error {
	query := `
		UPDATE withdrawals SET
			provider_ref = $2,
			status = $3,
			fees = $4,
			net_amount = $5,
			error_message = $6,
			operator_transaction_id = $7,
			processed_at = $8,
			updated_at = $9,
			compensation_attempts = $10,
			last_compensation_at = $11,
			failure_reason = $12
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query,
		w.ID, w.ProviderRef, string(w.Status), w.FeesCents, w.NetAmountCents,
		w.ErrorMessage, w.OperatorTransactionID, w.ProcessedAt, w.UpdatedAt,
		w.CompensationAttempts, w.LastCompensationAt, w.FailureReason,
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
		&w.CompensationAttempts, &w.LastCompensationAt, &w.FailureReason,
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
		&w.CompensationAttempts, &w.LastCompensationAt, &w.FailureReason,
	)
	if err != nil {
		return nil, fmt.Errorf("scan withdrawal: %w", err)
	}
	return w, nil
}

// FindStuckPendingWithdrawals trouve les retraits 'pending' ou 'processing' avec un débit wallet vieux de X minutes
// 🛡️ AMÉLIORATION : Vérifie aussi 'processing' au cas où le crash surviendrait après l'envoi au prestataire mais avant la réponse.
func (r *WithdrawalRepositoryPostgres) FindStuckPendingWithdrawals(ctx context.Context, limit, minAgeMinutes int) ([]*entity.Withdrawal, error) {
	if limit <= 0 {
		limit = 50
	}
	if minAgeMinutes <= 0 {
		minAgeMinutes = 15
	}

	query := `
		SELECT ` + withdrawalSelectCols + `
		FROM withdrawals w
		WHERE w.status IN ('pending', 'processing')
		  AND EXISTS (
		      SELECT 1 FROM wallet_transactions wt 
		      WHERE wt.reference_id = w.id::text 
		        AND wt.transaction_type = 'payout'
		        AND wt.created_at < NOW() - ($1 * INTERVAL '1 minute')
		  )
		ORDER BY w.created_at ASC
		LIMIT $2
	`
	rows, err := r.queryContext(ctx, query, minAgeMinutes, limit)
	if err != nil {
		return nil, fmt.Errorf("find stuck pending withdrawals: %w", err)
	}
	defer rows.Close()

	var out []*entity.Withdrawal
	for rows.Next() {
		w, err := r.scanFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
