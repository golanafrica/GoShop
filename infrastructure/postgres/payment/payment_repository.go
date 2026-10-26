package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
)

type PaymentRepositoryPostgres struct {
	db *sql.DB
	tx repository.Tx
}

func NewPaymentRepositoryPostgres(db *sql.DB) repository.PaymentRepository {
	return &PaymentRepositoryPostgres{db: db}
}

func (r *PaymentRepositoryPostgres) WithTX(tx repository.Tx) repository.PaymentRepository {
	return &PaymentRepositoryPostgres{db: r.db, tx: tx}
}

func (r *PaymentRepositoryPostgres) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PaymentRepositoryPostgres) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *PaymentRepositoryPostgres) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *PaymentRepositoryPostgres) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

func (r *PaymentRepositoryPostgres) Create(ctx context.Context, payment *entity.Payment) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(payment.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO payments (
			id, shop_id, order_id, provider, provider_ref,
			amount_cents, currency, customer_phone, customer_email,
			description, status, metadata, initiated_at, completed_at, expires_at,
			reference_type, reference_id, webhook_external_id, provider_fees_cents,
			refunded_amount_cents, commission_cents, commission_rate_bps, commission_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
	`

	_, err = r.execContext(ctx, query,
		payment.ID,
		shopID,
		payment.OrderID,
		payment.Provider,
		payment.ProviderRef,
		payment.AmountCents,
		payment.Currency,
		payment.CustomerPhone,
		payment.CustomerEmail,
		payment.Description,
		payment.Status,
		metadataJSON,
		payment.InitiatedAt,
		payment.CompletedAt,
		payment.ExpiresAt,
		payment.ReferenceType,
		payment.ReferenceID,
		payment.WebhookExternalID,
		payment.ProviderFeesCents,
		payment.RefundedAmountCents,
		payment.CommissionCents,
		payment.CommissionRateBps,
		payment.CommissionStatus,
	)

	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}

	return nil
}

func (r *PaymentRepositoryPostgres) FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, webhook_external_id, provider_fees_cents,
		       refunded_amount_cents, commission_cents, commission_rate_bps, commission_status, commission_collected_at,
		       created_at, updated_at
		FROM payments
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanPayment(r.queryRowContext(ctx, query, id, shopID))
}

func (r *PaymentRepositoryPostgres) FindByOrderID(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, webhook_external_id, provider_fees_cents,
		       refunded_amount_cents, commission_cents, commission_rate_bps, commission_status, commission_collected_at,
		       created_at, updated_at
		FROM payments
		WHERE order_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, orderID, shopID)
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}
	defer rows.Close()

	return r.scanPayments(rows)
}

func (r *PaymentRepositoryPostgres) FindByProviderRef(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error) {
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, webhook_external_id, provider_fees_cents,
		       refunded_amount_cents, commission_cents, commission_rate_bps, commission_status, commission_collected_at,
		       created_at, updated_at
		FROM payments
		WHERE provider = $1 AND provider_ref = $2
	`

	return r.scanPayment(r.queryRowContext(ctx, query, provider, providerRef))
}

func (r *PaymentRepositoryPostgres) FindByProviderRefForUpdate(ctx context.Context, provider entity.PaymentProvider, providerRef string) (*entity.Payment, error) {
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, webhook_external_id, provider_fees_cents,
		       refunded_amount_cents, commission_cents, commission_rate_bps, commission_status, commission_collected_at,
		       created_at, updated_at
		FROM payments
		WHERE provider = $1 AND provider_ref = $2
		FOR UPDATE
	`

	return r.scanPayment(r.queryRowContext(ctx, query, provider, providerRef))
}

func (r *PaymentRepositoryPostgres) FindByShop(ctx context.Context, shopID uuid.UUID, filters repository.PaymentFilters) ([]*entity.Payment, error) {
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, webhook_external_id, provider_fees_cents,
		       refunded_amount_cents, commission_cents, commission_rate_bps, commission_status, commission_collected_at,
		       created_at, updated_at
		FROM payments
		WHERE shop_id = $1
	`
	args := []interface{}{shopID}
	argPos := 2

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, *filters.Status)
		argPos++
	}

	if filters.Provider != nil {
		query += fmt.Sprintf(" AND provider = $%d", argPos)
		args = append(args, *filters.Provider)
		argPos++
	}

	query += " ORDER BY created_at DESC"

	limit := 50
	if filters.Limit > 0 && filters.Limit <= 100 {
		limit = filters.Limit
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}
	defer rows.Close()

	return r.scanPayments(rows)
}

func (r *PaymentRepositoryPostgres) Update(ctx context.Context, payment *entity.Payment) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(payment.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		UPDATE payments SET
			provider_ref = $1,
			status = $2,
			metadata = $3,
			initiated_at = $4,
			completed_at = $5,
			reference_type = $6,
			reference_id = $7,
			webhook_external_id = $8,
			provider_fees_cents = $9,
			refunded_amount_cents = $10,
			commission_cents = $11,
			commission_rate_bps = $12,
			commission_status = $13,
			updated_at = NOW()
		WHERE id = $14 AND shop_id = $15
	`

	result, err := r.execContext(ctx, query,
		payment.ProviderRef,
		payment.Status,
		metadataJSON,
		payment.InitiatedAt,
		payment.CompletedAt,
		payment.ReferenceType,
		payment.ReferenceID,
		payment.WebhookExternalID,
		payment.ProviderFeesCents,
		payment.RefundedAmountCents,
		payment.CommissionCents,
		payment.CommissionRateBps,
		payment.CommissionStatus,
		payment.ID,
		shopID,
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("payment not found or access denied")
	}

	return nil
}

// ============ Helpers de scan ============

func (r *PaymentRepositoryPostgres) scanPayment(row *sql.Row) (*entity.Payment, error) {
	var p entity.Payment
	var metadataBytes []byte
	var providerRef, customerPhone, customerEmail, description, referenceType, referenceID, webhookExternalID sql.NullString
	var commissionStatus sql.NullString
	var commissionCollectedAt sql.NullTime

	err := row.Scan(
		&p.ID,
		&p.ShopID,
		&p.OrderID,
		&p.Provider,
		&providerRef,
		&p.AmountCents,
		&p.Currency,
		&customerPhone,
		&customerEmail,
		&description,
		&p.Status,
		&metadataBytes,
		&p.InitiatedAt,
		&p.CompletedAt,
		&p.ExpiresAt,
		&referenceType,
		&referenceID,
		&webhookExternalID,
		&p.ProviderFeesCents,
		&p.RefundedAmountCents,
		&p.CommissionCents,
		&p.CommissionRateBps,
		&commissionStatus,
		&commissionCollectedAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("payment not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan payment: %w", err)
	}

	if providerRef.Valid {
		p.ProviderRef = &providerRef.String
	}
	if customerPhone.Valid {
		p.CustomerPhone = &customerPhone.String
	}
	if customerEmail.Valid {
		p.CustomerEmail = &customerEmail.String
	}
	if description.Valid {
		p.Description = &description.String
	}
	if referenceType.Valid {
		p.ReferenceType = &referenceType.String
	}
	if referenceID.Valid {
		p.ReferenceID = &referenceID.String
	}
	if webhookExternalID.Valid {
		p.WebhookExternalID = &webhookExternalID.String
	}
	if commissionStatus.Valid {
		p.CommissionStatus = commissionStatus.String
	}
	if commissionCollectedAt.Valid {
		p.CommissionCollectedAt = &commissionCollectedAt.Time
	}

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	} else {
		p.Metadata = make(map[string]interface{})
	}

	return &p, nil
}

func (r *PaymentRepositoryPostgres) scanPayments(rows *sql.Rows) ([]*entity.Payment, error) {
	var payments []*entity.Payment

	for rows.Next() {
		var p entity.Payment
		var metadataBytes []byte
		var providerRef, customerPhone, customerEmail, description, referenceType, referenceID, webhookExternalID sql.NullString
		var commissionStatus sql.NullString
		var commissionCollectedAt sql.NullTime

		err := rows.Scan(
			&p.ID,
			&p.ShopID,
			&p.OrderID,
			&p.Provider,
			&providerRef,
			&p.AmountCents,
			&p.Currency,
			&customerPhone,
			&customerEmail,
			&description,
			&p.Status,
			&metadataBytes,
			&p.InitiatedAt,
			&p.CompletedAt,
			&p.ExpiresAt,
			&referenceType,
			&referenceID,
			&webhookExternalID,
			&p.ProviderFeesCents,
			&p.RefundedAmountCents,
			&p.CommissionCents,
			&p.CommissionRateBps,
			&commissionStatus,
			&commissionCollectedAt,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment row: %w", err)
		}

		if providerRef.Valid {
			p.ProviderRef = &providerRef.String
		}
		if customerPhone.Valid {
			p.CustomerPhone = &customerPhone.String
		}
		if customerEmail.Valid {
			p.CustomerEmail = &customerEmail.String
		}
		if description.Valid {
			p.Description = &description.String
		}
		if referenceType.Valid {
			p.ReferenceType = &referenceType.String
		}
		if referenceID.Valid {
			p.ReferenceID = &referenceID.String
		}
		if webhookExternalID.Valid {
			p.WebhookExternalID = &webhookExternalID.String
		}
		if commissionStatus.Valid {
			p.CommissionStatus = commissionStatus.String
		}
		if commissionCollectedAt.Valid {
			p.CommissionCollectedAt = &commissionCollectedAt.Time
		}

		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		} else {
			p.Metadata = make(map[string]interface{})
		}

		payments = append(payments, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments: %w", err)
	}

	return payments, nil
}

func (r *PaymentRepositoryPostgres) FindCompletedWithoutCommission(
	ctx context.Context,
	limit int,
) ([]*entity.Payment, error) {
	query := `
        SELECT 
            id, shop_id, order_id, provider, provider_ref,
            amount_cents, currency, customer_phone, customer_email,
            description, status, metadata, 
            initiated_at, completed_at, expires_at,
            created_at, updated_at,
            COALESCE(commission_rate_bps, 0) as commission_rate_bps,
            COALESCE(commission_cents, 0) as commission_cents,
            COALESCE(commission_status, 'pending') as commission_status,
            commission_collected_at,
            COALESCE(provider_fees_cents, 0) as provider_fees_cents,
            COALESCE(refunded_amount_cents, 0) as refunded_amount_cents
        FROM payments
        WHERE status = 'success'
          AND (commission_status IS NULL OR commission_status = 'pending')
          AND (reference_type IS NULL OR reference_type = 'order')
        ORDER BY completed_at ASC
        LIMIT $1
    `
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []*entity.Payment
	for rows.Next() {
		var p entity.Payment
		var metadataBytes []byte
		var providerRef, customerPhone, customerEmail, description sql.NullString

		err := rows.Scan(
			&p.ID,
			&p.ShopID,
			&p.OrderID,
			&p.Provider,
			&providerRef,
			&p.AmountCents,
			&p.Currency,
			&customerPhone,
			&customerEmail,
			&description,
			&p.Status,
			&metadataBytes,
			&p.InitiatedAt,
			&p.CompletedAt,
			&p.ExpiresAt,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.CommissionRateBps,
			&p.CommissionCents,
			&p.CommissionStatus,
			&p.CommissionCollectedAt,
			&p.ProviderFeesCents,
			&p.RefundedAmountCents,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}

		if providerRef.Valid {
			p.ProviderRef = &providerRef.String
		}
		if customerPhone.Valid {
			p.CustomerPhone = &customerPhone.String
		}
		if customerEmail.Valid {
			p.CustomerEmail = &customerEmail.String
		}
		if description.Valid {
			p.Description = &description.String
		}

		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		} else {
			p.Metadata = make(map[string]interface{})
		}

		payments = append(payments, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments: %w", err)
	}

	return payments, nil
}

func (r *PaymentRepositoryPostgres) UpdateCommissionStatus(
	ctx context.Context,
	paymentID string,
	status string,
	commissionCents int64,
) error {
	query := `
        UPDATE payments SET
            commission_status = $1,
            commission_cents = $2,
            commission_collected_at = CASE WHEN $1 = 'collected' THEN NOW() ELSE commission_collected_at END,
            updated_at = NOW()
        WHERE id = $3
    `
	_, err := r.db.ExecContext(ctx, query, status, commissionCents, paymentID)
	if err != nil {
		return fmt.Errorf("failed to update commission status: %w", err)
	}
	return nil
}

func (r *PaymentRepositoryPostgres) FindByOrderIDUnscoped(ctx context.Context, orderID uuid.UUID) ([]*entity.Payment, error) {
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, webhook_external_id, provider_fees_cents,
		       refunded_amount_cents, commission_cents, commission_rate_bps, commission_status, commission_collected_at,
		       created_at, updated_at
		FROM payments
		WHERE order_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("query payments unscoped: %w", err)
	}
	defer rows.Close()

	return r.scanPayments(rows)
}

func (r *PaymentRepositoryPostgres) AddRefundAmount(
	ctx context.Context,
	paymentID uuid.UUID,
	refundAmountCents int64,
	newStatus entity.PaymentStatus,
) (bool, error) {
	query := `
		UPDATE payments
		SET 
			refunded_amount_cents = refunded_amount_cents + $1,
			status = $2,
			updated_at = NOW()
		WHERE id = $3
		  AND (amount_cents - refunded_amount_cents) >= $1
	`

	res, err := r.execContext(ctx, query, refundAmountCents, string(newStatus), paymentID)
	if err != nil {
		return false, fmt.Errorf("add refund amount: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check rows affected: %w", err)
	}

	return rows > 0, nil
}

func (r *PaymentRepositoryPostgres) ReverseRefundAmount(
	ctx context.Context,
	paymentID uuid.UUID,
	refundAmountCents int64,
	newStatus entity.PaymentStatus,
) (bool, error) {
	query := `
		UPDATE payments
		SET
			refunded_amount_cents = refunded_amount_cents - $1,
			status = $2,
			updated_at = NOW()
		WHERE id = $3
		  AND refunded_amount_cents >= $1
	`

	res, err := r.execContext(ctx, query, refundAmountCents, string(newStatus), paymentID)
	if err != nil {
		return false, fmt.Errorf("reverse refund amount: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check rows affected: %w", err)
	}

	return rows > 0, nil
}

func (r *PaymentRepositoryPostgres) FindStuckNonTerminalPayments(
	ctx context.Context,
	minAgeMinutes int,
	limit int,
) ([]*entity.Payment, error) {
	if limit <= 0 {
		limit = 50
	}
	if minAgeMinutes <= 0 {
		minAgeMinutes = 5
	}

	query := `
		SELECT id, shop_id, order_id, provider, provider_ref, amount_cents, currency,
		       customer_phone, customer_email, description, status, metadata,
		       initiated_at, completed_at, expires_at, created_at, updated_at,
		       COALESCE(provider_fees_cents, 0),
		       COALESCE(refunded_amount_cents, 0),
		       COALESCE(commission_cents, 0),
		       COALESCE(commission_rate_bps, 0),
		       COALESCE(commission_status, 'pending'),
		       commission_collected_at
		FROM payments
		WHERE status IN ('pending', 'processing')
		  AND provider_ref IS NOT NULL
		  AND provider_ref <> ''
		  AND updated_at < NOW() - ($1 || ' minutes')::interval
		ORDER BY updated_at ASC
		LIMIT $2
	`

	rows, err := r.queryContext(ctx, query, minAgeMinutes, limit)
	if err != nil {
		return nil, fmt.Errorf("find stuck non-terminal payments: %w", err)
	}
	defer rows.Close()

	return r.scanPayments(rows)
}

func (r *PaymentRepositoryPostgres) ReverseRefundAmountAndRecompute(
	ctx context.Context,
	paymentID uuid.UUID,
	refundAmountCents int64,
) (bool, error) {
	query := `
		UPDATE payments
		SET
			refunded_amount_cents = refunded_amount_cents - $1,
			status = CASE
				WHEN (refunded_amount_cents - $1) <= 0 THEN 'success'
				WHEN (refunded_amount_cents - $1) < amount_cents THEN 'partially_refunded'
				ELSE 'refunded'
			END,
			updated_at = NOW()
		WHERE id = $2
		  AND refunded_amount_cents >= $1
	`

	res, err := r.execContext(ctx, query, refundAmountCents, paymentID)
	if err != nil {
		return false, fmt.Errorf("reverse refund amount and recompute: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check rows affected: %w", err)
	}

	return rows > 0, nil
}

func (r *PaymentRepositoryPostgres) CreatePaymentRefund(ctx context.Context, refund *entity.PaymentRefund) error {
	query := `
		INSERT INTO payment_refunds (
			id, payment_id, shop_id, amount_cents, status, reason,
			idempotency_key, provider_ref, provider_error, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
	`
	_, err := r.execContext(ctx, query,
		refund.ID,
		refund.PaymentID,
		refund.ShopID,
		refund.AmountCents,
		string(refund.Status),
		refund.Reason,
		refund.IdempotencyKey,
		refund.ProviderRef,
		refund.ProviderError,
	)
	if err != nil {
		return fmt.Errorf("create payment refund: %w", err)
	}
	return nil
}

func (r *PaymentRepositoryPostgres) UpdatePaymentRefundStatus(
	ctx context.Context,
	id uuid.UUID,
	status entity.PaymentRefundStatus,
	providerErr string,
) error {
	var pe interface{}
	if providerErr != "" {
		pe = providerErr
	}
	query := `
		UPDATE payment_refunds
		SET status = $1, provider_error = $2, updated_at = NOW()
		WHERE id = $3
	`
	res, err := r.execContext(ctx, query, string(status), pe, id)
	if err != nil {
		return fmt.Errorf("update payment refund status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("payment refund not found: %s", id)
	}
	return nil
}

func (r *PaymentRepositoryPostgres) FindPaymentRefundByIdempotency(
	ctx context.Context,
	shopID uuid.UUID,
	key string,
) (*entity.PaymentRefund, error) {
	query := `
		SELECT id, payment_id, shop_id, amount_cents, status, reason,
		       idempotency_key, provider_ref, provider_error, created_at, updated_at
		FROM payment_refunds
		WHERE shop_id = $1 AND idempotency_key = $2
	`
	row := r.queryRowContext(ctx, query, shopID, key)

	var rf entity.PaymentRefund
	var reason, providerRef, providerError sql.NullString
	err := row.Scan(
		&rf.ID, &rf.PaymentID, &rf.ShopID, &rf.AmountCents, &rf.Status, &reason,
		&rf.IdempotencyKey, &providerRef, &providerError, &rf.CreatedAt, &rf.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find payment refund by idempotency: %w", err)
	}
	if reason.Valid {
		rf.Reason = reason.String
	}
	if providerRef.Valid {
		rf.ProviderRef = &providerRef.String
	}
	if providerError.Valid {
		rf.ProviderError = &providerError.String
	}
	return &rf, nil
}

// 🆕 C3 AUDIT : FindStuckPaymentsMissingProviderRef trouve les paiements bloqués sans référence fournisseur.
func (r *PaymentRepositoryPostgres) FindStuckPaymentsMissingProviderRef(
	ctx context.Context,
	minAgeMinutes int,
	limit int,
) ([]*entity.Payment, error) {
	if limit <= 0 {
		limit = 50
	}
	if minAgeMinutes <= 0 {
		minAgeMinutes = 15 // 15 min par défaut pour éviter les faux positifs sur les initiations lentes
	}

	query := `
		SELECT id, shop_id, order_id, provider, provider_ref, amount_cents, currency,
		       customer_phone, customer_email, description, status, metadata,
		       initiated_at, completed_at, expires_at, created_at, updated_at,
		       COALESCE(provider_fees_cents, 0),
		       COALESCE(refunded_amount_cents, 0),
		       COALESCE(commission_cents, 0),
		       COALESCE(commission_rate_bps, 0),
		       COALESCE(commission_status, 'pending'),
		       commission_collected_at
		FROM payments
		WHERE status IN ('pending', 'processing')
		  AND (provider_ref IS NULL OR provider_ref = '')
		  AND updated_at < NOW() - ($1 || ' minutes')::interval
		ORDER BY updated_at ASC
		LIMIT $2
	`

	rows, err := r.queryContext(ctx, query, minAgeMinutes, limit)
	if err != nil {
		return nil, fmt.Errorf("find stuck payments missing provider ref: %w", err)
	}
	defer rows.Close()

	return r.scanPayments(rows)
}
