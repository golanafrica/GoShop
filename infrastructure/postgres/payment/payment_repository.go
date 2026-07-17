package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

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

	// ✅ AJOUT : reference_type et reference_id dans l'INSERT
	query := `
		INSERT INTO payments (
			id, shop_id, order_id, provider, provider_ref,
			amount_cents, currency, customer_phone, customer_email,
			description, status, metadata, initiated_at, completed_at, expires_at,
			reference_type, reference_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
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
		payment.ReferenceType, // ✅ AJOUT
		payment.ReferenceID,   // ✅ AJOUT
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

	// ✅ AJOUT : reference_type et reference_id dans le SELECT
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, created_at, updated_at
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
		       expires_at, reference_type, reference_id, created_at, updated_at
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
	// ✅ AJOUT : reference_type et reference_id dans le SELECT
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, created_at, updated_at
		FROM payments
		WHERE provider = $1 AND provider_ref = $2
	`

	return r.scanPayment(r.queryRowContext(ctx, query, provider, providerRef))
}

func (r *PaymentRepositoryPostgres) FindByShop(ctx context.Context, shopID uuid.UUID, filters repository.PaymentFilters) ([]*entity.Payment, error) {
	query := `
		SELECT id, shop_id, order_id, provider, provider_ref,
		       amount_cents, currency, customer_phone, customer_email,
		       description, status, metadata, initiated_at, completed_at,
		       expires_at, reference_type, reference_id, created_at, updated_at
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

	// ✅ AJOUT : reference_type et reference_id dans l'UPDATE
	query := `
		UPDATE payments SET
			provider_ref = $1,
			status = $2,
			metadata = $3,
			initiated_at = $4,
			completed_at = $5,
			reference_type = $6,
			reference_id = $7,
			updated_at = NOW()
		WHERE id = $8 AND shop_id = $9
	`

	result, err := r.execContext(ctx, query,
		payment.ProviderRef,
		payment.Status,
		metadataJSON,
		payment.InitiatedAt,
		payment.CompletedAt,
		payment.ReferenceType, // ✅ AJOUT
		payment.ReferenceID,   // ✅ AJOUT
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
	var providerRef, customerPhone, customerEmail, description, referenceType, referenceID sql.NullString

	// ✅ AJOUT : reference_type et reference_id dans le Scan
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
		&referenceType, // ✅ AJOUT
		&referenceID,   // ✅ AJOUT
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
		var providerRef, customerPhone, customerEmail, description, referenceType, referenceID sql.NullString

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
			&referenceType, // ✅ AJOUT
			&referenceID,   // ✅ AJOUT
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

// FindCompletedWithoutCommission récupère les paiements success sans commission collectée
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
            commission_collected_at
        FROM payments
        WHERE status = 'success'
          AND (commission_status IS NULL OR commission_status = 'pending')
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

// UpdateCommissionStatus met à jour le statut de commission
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

// Unused import prevention
var _ = time.Now
