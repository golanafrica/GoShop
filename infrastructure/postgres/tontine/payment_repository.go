package tontine

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// TontinePaymentRepositoryInfrastructure implémente repository.TontinePaymentRepository
type TontinePaymentRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewTontinePaymentRepositoryInfrastructure crée une nouvelle instance
func NewTontinePaymentRepositoryInfrastructure(db *sql.DB) repository.TontinePaymentRepository {
	return &TontinePaymentRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *TontinePaymentRepositoryInfrastructure) WithTX(tx repository.Tx) repository.TontinePaymentRepository {
	return &TontinePaymentRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers
// ============================================================

func (r *TontinePaymentRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *TontinePaymentRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *TontinePaymentRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *TontinePaymentRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

func (r *TontinePaymentRepositoryInfrastructure) scanPayment(row *sql.Row) (*entity.TontinePayment, error) {
	p := &entity.TontinePayment{}
	var yengapayRef, providerIntentID, yengapayTxID sql.NullString
	var paidAt sql.NullTime
	var commissionStatus sql.NullString

	err := row.Scan(
		&p.ID, &p.GroupID, &p.ParticipantID, &p.CustomerID,
		&p.CycleNumber, &p.AmountCents, &p.CommissionCents,
		&yengapayRef, &providerIntentID, &yengapayTxID,
		&p.PaymentProvider, &p.Status,
		&p.DueDate, &paidAt,
		&p.CreatedAt, &p.UpdatedAt,
		&commissionStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tontine payment not found")
		}
		return nil, fmt.Errorf("failed to scan payment: %w", err)
	}

	if yengapayRef.Valid {
		p.YengaPayReference = &yengapayRef.String
	}
	if providerIntentID.Valid {
		p.ProviderIntentID = providerIntentID.String
	}
	if yengapayTxID.Valid {
		p.YengaPayTransactionID = &yengapayTxID.String
	}
	if paidAt.Valid {
		p.PaidAt = &paidAt.Time
	}
	if commissionStatus.Valid {
		p.CommissionStatus = commissionStatus.String
	}

	return p, nil
}

func (r *TontinePaymentRepositoryInfrastructure) scanPayments(ctx context.Context, query string, args ...interface{}) ([]*entity.TontinePayment, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []*entity.TontinePayment
	for rows.Next() {
		p := &entity.TontinePayment{}
		var yengapayRef, providerIntentID, yengapayTxID sql.NullString
		var paidAt sql.NullTime
		var commissionStatus sql.NullString

		err := rows.Scan(
			&p.ID, &p.GroupID, &p.ParticipantID, &p.CustomerID,
			&p.CycleNumber, &p.AmountCents, &p.CommissionCents,
			&yengapayRef, &providerIntentID, &yengapayTxID,
			&p.PaymentProvider, &p.Status,
			&p.DueDate, &paidAt,
			&p.CreatedAt, &p.UpdatedAt,
			&commissionStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if yengapayRef.Valid {
			p.YengaPayReference = &yengapayRef.String
		}
		if providerIntentID.Valid {
			p.ProviderIntentID = providerIntentID.String
		}
		if yengapayTxID.Valid {
			p.YengaPayTransactionID = &yengapayTxID.String
		}
		if paidAt.Valid {
			p.PaidAt = &paidAt.Time
		}
		if commissionStatus.Valid {
			p.CommissionStatus = commissionStatus.String
		}

		payments = append(payments, p)
	}

	if payments == nil {
		payments = []*entity.TontinePayment{}
	}
	return payments, rows.Err()
}

// ============================================================
// Implémentation
// ============================================================

func (r *TontinePaymentRepositoryInfrastructure) Create(ctx context.Context, payment *entity.TontinePayment) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	var groupShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM tontine_groups WHERE id = $1`, payment.GroupID).Scan(&groupShopID)
	if err != nil {
		return fmt.Errorf("failed to verify group: %w", err)
	}
	if groupShopID != shopID {
		return fmt.Errorf("access denied: group does not belong to tenant shop")
	}

	query := `
		INSERT INTO tontine_payments (
			group_id, participant_id, customer_id,
			cycle_number, amount_cents, commission_cents,
			yengapay_reference, provider_intent_id, yengapay_transaction_id,
			payment_provider, status, due_date,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		payment.GroupID,
		payment.ParticipantID,
		payment.CustomerID,
		payment.CycleNumber,
		payment.AmountCents,
		payment.CommissionCents,
		payment.YengaPayReference,
		payment.ProviderIntentID, // 🆕 AJOUT
		payment.YengaPayTransactionID,
		payment.PaymentProvider,
		payment.Status,
		payment.DueDate,
	).Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}
	return nil
}

func (r *TontinePaymentRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.id = $1 AND g.shop_id = $2
	`
	return r.scanPayment(r.queryRowContext(ctx, query, id, shopID))
}

// 🆕 FindByIDUnscoped trouve un paiement par ID SANS vérification de tenant (pour webhooks/sync)
func (r *TontinePaymentRepositoryInfrastructure) FindByIDUnscoped(ctx context.Context, id string) (*entity.TontinePayment, error) {
	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		WHERE p.id = $1
		LIMIT 1
	`
	return r.scanPayment(r.queryRowContext(ctx, query, id))
}

// 🆕 SetProviderIntentID met à jour l'ID d'intention de paiement côté provider
func (r *TontinePaymentRepositoryInfrastructure) SetProviderIntentID(ctx context.Context, id, intentID string) error {
	query := `
		UPDATE tontine_payments 
		SET provider_intent_id = $1, updated_at = NOW()
		WHERE id = $2
	`
	result, err := r.execContext(ctx, query, intentID, id)
	if err != nil {
		return fmt.Errorf("failed to update provider intent id: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("payment not found")
	}
	return nil
}

func (r *TontinePaymentRepositoryInfrastructure) FindByReference(ctx context.Context, reference string) (*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.yengapay_reference = $1 AND g.shop_id = $2
	`
	return r.scanPayment(r.queryRowContext(ctx, query, reference, shopID))
}

// FindByReferenceUnscoped trouve un paiement par yengapay_reference SANS tenant
// Utilisé par les webhooks (pas de contexte multi-tenant)
func (r *TontinePaymentRepositoryInfrastructure) FindByReferenceUnscoped(ctx context.Context, reference string) (*entity.TontinePayment, error) {
	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		WHERE p.yengapay_reference = $1
		LIMIT 1
	`
	return r.scanPayment(r.queryRowContext(ctx, query, reference))
}

func (r *TontinePaymentRepositoryInfrastructure) FindByReferencePrefix(ctx context.Context, referencePrefix string) (*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.yengapay_reference = $1 AND g.shop_id = $2
	`
	return r.scanPayment(r.queryRowContext(ctx, query, referencePrefix, shopID))
}

func (r *TontinePaymentRepositoryInfrastructure) FindByGroupAndCycle(ctx context.Context, groupID string, cycle int) ([]*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND p.cycle_number = $2 AND g.shop_id = $3
		ORDER BY p.created_at ASC
	`
	return r.scanPayments(ctx, query, groupID, cycle, shopID)
}

func (r *TontinePaymentRepositoryInfrastructure) FindByCustomerAndGroup(ctx context.Context, customerID, groupID string) ([]*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.customer_id = $1 AND p.group_id = $2 AND g.shop_id = $3
		ORDER BY p.cycle_number ASC
	`
	return r.scanPayments(ctx, query, customerID, groupID, shopID)
}

func (r *TontinePaymentRepositoryInfrastructure) FindByParticipantAndCycle(ctx context.Context, participantID string, cycle int) (*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.participant_id = $1 AND p.cycle_number = $2 AND g.shop_id = $3
	`
	return r.scanPayment(r.queryRowContext(ctx, query, participantID, cycle, shopID))
}

func (r *TontinePaymentRepositoryInfrastructure) CountDoneByGroupAndCycle(ctx context.Context, groupID string, cycle int) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.group_id = $1 AND p.cycle_number = $2 AND p.status = 'DONE' AND g.shop_id = $3
	`
	var count int
	err = r.queryRowContext(ctx, query, groupID, cycle, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count done payments: %w", err)
	}
	return count, nil
}

func (r *TontinePaymentRepositoryInfrastructure) UpdateStatus(ctx context.Context, paymentID string, status string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE tontine_payments SET status = $1, updated_at = NOW()
		WHERE id = $2 AND group_id IN (
			SELECT id FROM tontine_groups WHERE shop_id = $3
		)
	`
	result, err := r.execContext(ctx, query, status, paymentID, shopID)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("payment not found")
	}
	return nil
}

func (r *TontinePaymentRepositoryInfrastructure) MarkDone(ctx context.Context, paymentID string, transactionID string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE tontine_payments
		SET status = 'DONE', yengapay_transaction_id = $2, paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND group_id IN (
			SELECT id FROM tontine_groups WHERE shop_id = $3
		)
	`
	result, err := r.execContext(ctx, query, paymentID, transactionID, shopID)
	if err != nil {
		return fmt.Errorf("failed to mark payment done: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("payment not found")
	}
	return nil
}

// ============================================================
// v3.3.0 : Méthodes pour le scheduler de commissions
// ============================================================

func (r *TontinePaymentRepositoryInfrastructure) FindDoneWithoutCommission(
	ctx context.Context,
	limit int,
) ([]*entity.TontinePayment, error) {
	query := `
		SELECT
			p.id, p.group_id, p.participant_id, p.customer_id,
			p.cycle_number, p.amount_cents, p.commission_cents,
			p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
			p.payment_provider, p.status,
			p.due_date, p.paid_at,
			p.created_at, p.updated_at,
			COALESCE(p.commission_status, 'pending') as commission_status,
			g.shop_id
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.status = 'DONE'
		  AND (p.commission_status IS NULL OR p.commission_status = 'pending')
		ORDER BY p.paid_at ASC
		LIMIT $1
	`
	rows, err := r.queryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query tontine payments: %w", err)
	}
	defer rows.Close()

	var payments []*entity.TontinePayment
	for rows.Next() {
		p := &entity.TontinePayment{}
		var yengapayRef, providerIntentID, yengapayTxID sql.NullString
		var paidAt sql.NullTime
		var commissionStatus sql.NullString

		err := rows.Scan(
			&p.ID, &p.GroupID, &p.ParticipantID, &p.CustomerID,
			&p.CycleNumber, &p.AmountCents, &p.CommissionCents,
			&yengapayRef, &providerIntentID, &yengapayTxID,
			&p.PaymentProvider, &p.Status,
			&p.DueDate, &paidAt,
			&p.CreatedAt, &p.UpdatedAt,
			&commissionStatus,
			&p.ShopID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}

		if yengapayRef.Valid {
			p.YengaPayReference = &yengapayRef.String
		}
		if providerIntentID.Valid {
			p.ProviderIntentID = providerIntentID.String
		}
		if yengapayTxID.Valid {
			p.YengaPayTransactionID = &yengapayTxID.String
		}
		if paidAt.Valid {
			p.PaidAt = &paidAt.Time
		}
		if commissionStatus.Valid {
			p.CommissionStatus = commissionStatus.String
		}

		payments = append(payments, p)
	}

	if payments == nil {
		payments = []*entity.TontinePayment{}
	}
	return payments, rows.Err()
}

func (r *TontinePaymentRepositoryInfrastructure) UpdateTontineCommissionStatus(
	ctx context.Context,
	paymentID string,
	status string,
	batchID *string,
) error {
	query := `
		UPDATE tontine_payments SET
			commission_status = $1,
			commission_batch_id = $2,
			commission_collected_at = CASE WHEN $1 = 'collected' THEN NOW() ELSE commission_collected_at END,
			updated_at = NOW()
		WHERE id = $3
	`
	result, err := r.execContext(ctx, query, status, batchID, paymentID)
	if err != nil {
		return fmt.Errorf("failed to update tontine commission status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tontine payment not found")
	}
	return nil
}

// ============================================================
// 🛡️ v4.11.0 : MÉTHODES AVEC VERROU PESSIMISTE (FOR UPDATE)
// ============================================================

// FindByParticipantAndCycleForUpdate trouve un paiement avec verrou pessimiste
// Utilisé dans pay_cycle.go pour éviter les doubles paiements
func (r *TontinePaymentRepositoryInfrastructure) FindByParticipantAndCycleForUpdate(ctx context.Context, participantID string, cycle int) (*entity.TontinePayment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		JOIN tontine_groups g ON g.id = p.group_id
		WHERE p.participant_id = $1 AND p.cycle_number = $2 AND g.shop_id = $3
		FOR UPDATE
	`
	return r.scanPayment(r.queryRowContext(ctx, query, participantID, cycle, shopID))
}

// FindByReferenceUnscopedForUpdate trouve un paiement par référence avec verrou (sans tenant)
// Utilisé dans process_tontine_webhook.go pour éviter les doubles webhooks
func (r *TontinePaymentRepositoryInfrastructure) FindByReferenceUnscopedForUpdate(ctx context.Context, reference string) (*entity.TontinePayment, error) {
	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		WHERE p.yengapay_reference = $1
		LIMIT 1
		FOR UPDATE
	`
	return r.scanPayment(r.queryRowContext(ctx, query, reference))
}

// FindByIDUnscopedForUpdate trouve un paiement par ID avec verrou (sans tenant)
// Utilisé dans sync_tontine_payment.go pour éviter les doubles syncs
func (r *TontinePaymentRepositoryInfrastructure) FindByIDUnscopedForUpdate(ctx context.Context, id string) (*entity.TontinePayment, error) {
	query := `
		SELECT p.id, p.group_id, p.participant_id, p.customer_id,
		       p.cycle_number, p.amount_cents, p.commission_cents,
		       p.yengapay_reference, p.provider_intent_id, p.yengapay_transaction_id,
		       p.payment_provider, p.status,
		       p.due_date, p.paid_at,
		       p.created_at, p.updated_at,
		       COALESCE(p.commission_status, 'pending') as commission_status
		FROM tontine_payments p
		WHERE p.id = $1
		LIMIT 1
		FOR UPDATE
	`
	return r.scanPayment(r.queryRowContext(ctx, query, id))
}
