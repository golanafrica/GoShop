package cod

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CODProofRepositoryInfrastructure implémente repository.CODProofRepository
type CODProofRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCODProofRepositoryInfrastructure crée une nouvelle instance
func NewCODProofRepositoryInfrastructure(db *sql.DB) repository.CODProofRepository {
	return &CODProofRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CODProofRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CODProofRepository {
	return &CODProofRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *CODProofRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CODProofRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CODProofRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *CODProofRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanProof scanne une ligne dans une entité CODProof
func (r *CODProofRepositoryInfrastructure) scanProof(row *sql.Row) (*entity.CODProof, error) {
	proof := &entity.CODProof{}

	// Preuves client
	var clientPaymentProofURL, clientReceiptNumber, clientNotes sql.NullString
	var clientPaymentAmountCents sql.NullInt64
	var clientPaymentDate, clientSubmittedAt sql.NullTime

	// Preuves marchand
	var merchantReceiptProofURL, merchantNotes sql.NullString
	var merchantReceivedAmountCents sql.NullInt64
	var merchantReceiptDate, merchantSubmittedAt sql.NullTime

	// Cohérence
	var amountsMatch, datesMatch sql.NullBool

	// Commission
	var commissionCollectedAt sql.NullTime

	// Litige
	var disputeRaisedAt, disputeResolvedAt sql.NullTime
	var disputeReason sql.NullString

	err := row.Scan(
		&proof.ID,
		&proof.OrderID,
		&proof.ShopID,
		&proof.CustomerID,
		&clientPaymentProofURL,
		&clientPaymentAmountCents,
		&clientPaymentDate,
		&clientReceiptNumber,
		&clientNotes,
		&clientSubmittedAt,
		&merchantReceiptProofURL,
		&merchantReceivedAmountCents,
		&merchantReceiptDate,
		&merchantNotes,
		&merchantSubmittedAt,
		&amountsMatch,
		&datesMatch,
		&proof.CommissionCents,
		&proof.CommissionStatus,
		&commissionCollectedAt,
		&proof.Status,
		&disputeRaisedAt,
		&disputeReason,
		&disputeResolvedAt,
		&proof.CreatedAt,
		&proof.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("COD proof not found")
		}
		return nil, fmt.Errorf("failed to scan COD proof: %w", err)
	}

	// Preuves client
	if clientPaymentProofURL.Valid {
		proof.ClientPaymentProofURL = &clientPaymentProofURL.String
	}
	if clientPaymentAmountCents.Valid {
		amt := clientPaymentAmountCents.Int64
		proof.ClientPaymentAmountCents = &amt
	}
	if clientPaymentDate.Valid {
		proof.ClientPaymentDate = &clientPaymentDate.Time
	}
	if clientReceiptNumber.Valid {
		proof.ClientReceiptNumber = &clientReceiptNumber.String
	}
	if clientNotes.Valid {
		proof.ClientNotes = &clientNotes.String
	}
	if clientSubmittedAt.Valid {
		proof.ClientSubmittedAt = &clientSubmittedAt.Time
	}

	// Preuves marchand
	if merchantReceiptProofURL.Valid {
		proof.MerchantReceiptProofURL = &merchantReceiptProofURL.String
	}
	if merchantReceivedAmountCents.Valid {
		amt := merchantReceivedAmountCents.Int64
		proof.MerchantReceivedAmountCents = &amt
	}
	if merchantReceiptDate.Valid {
		proof.MerchantReceiptDate = &merchantReceiptDate.Time
	}
	if merchantNotes.Valid {
		proof.MerchantNotes = &merchantNotes.String
	}
	if merchantSubmittedAt.Valid {
		proof.MerchantSubmittedAt = &merchantSubmittedAt.Time
	}

	// Cohérence
	if amountsMatch.Valid {
		v := amountsMatch.Bool
		proof.AmountsMatch = &v
	}
	if datesMatch.Valid {
		v := datesMatch.Bool
		proof.DatesMatch = &v
	}

	// Commission
	if commissionCollectedAt.Valid {
		proof.CommissionCollectedAt = &commissionCollectedAt.Time
	}

	// Litige
	if disputeRaisedAt.Valid {
		proof.DisputeRaisedAt = &disputeRaisedAt.Time
	}
	if disputeReason.Valid {
		proof.DisputeReason = &disputeReason.String
	}
	if disputeResolvedAt.Valid {
		proof.DisputeResolvedAt = &disputeResolvedAt.Time
	}

	return proof, nil
}

// scanProofs scanne plusieurs lignes
func (r *CODProofRepositoryInfrastructure) scanProofs(ctx context.Context, query string, args ...interface{}) ([]*entity.CODProof, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query COD proofs: %w", err)
	}
	defer rows.Close()

	var proofs []*entity.CODProof
	for rows.Next() {
		proof := &entity.CODProof{}

		var clientPaymentProofURL, clientReceiptNumber, clientNotes sql.NullString
		var clientPaymentAmountCents sql.NullInt64
		var clientPaymentDate, clientSubmittedAt sql.NullTime

		var merchantReceiptProofURL, merchantNotes sql.NullString
		var merchantReceivedAmountCents sql.NullInt64
		var merchantReceiptDate, merchantSubmittedAt sql.NullTime

		var amountsMatch, datesMatch sql.NullBool
		var commissionCollectedAt sql.NullTime
		var disputeRaisedAt, disputeResolvedAt sql.NullTime
		var disputeReason sql.NullString

		err := rows.Scan(
			&proof.ID,
			&proof.OrderID,
			&proof.ShopID,
			&proof.CustomerID,
			&clientPaymentProofURL,
			&clientPaymentAmountCents,
			&clientPaymentDate,
			&clientReceiptNumber,
			&clientNotes,
			&clientSubmittedAt,
			&merchantReceiptProofURL,
			&merchantReceivedAmountCents,
			&merchantReceiptDate,
			&merchantNotes,
			&merchantSubmittedAt,
			&amountsMatch,
			&datesMatch,
			&proof.CommissionCents,
			&proof.CommissionStatus,
			&commissionCollectedAt,
			&proof.Status,
			&disputeRaisedAt,
			&disputeReason,
			&disputeResolvedAt,
			&proof.CreatedAt,
			&proof.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if clientPaymentProofURL.Valid {
			proof.ClientPaymentProofURL = &clientPaymentProofURL.String
		}
		if clientPaymentAmountCents.Valid {
			amt := clientPaymentAmountCents.Int64
			proof.ClientPaymentAmountCents = &amt
		}
		if clientPaymentDate.Valid {
			proof.ClientPaymentDate = &clientPaymentDate.Time
		}
		if clientReceiptNumber.Valid {
			proof.ClientReceiptNumber = &clientReceiptNumber.String
		}
		if clientNotes.Valid {
			proof.ClientNotes = &clientNotes.String
		}
		if clientSubmittedAt.Valid {
			proof.ClientSubmittedAt = &clientSubmittedAt.Time
		}
		if merchantReceiptProofURL.Valid {
			proof.MerchantReceiptProofURL = &merchantReceiptProofURL.String
		}
		if merchantReceivedAmountCents.Valid {
			amt := merchantReceivedAmountCents.Int64
			proof.MerchantReceivedAmountCents = &amt
		}
		if merchantReceiptDate.Valid {
			proof.MerchantReceiptDate = &merchantReceiptDate.Time
		}
		if merchantNotes.Valid {
			proof.MerchantNotes = &merchantNotes.String
		}
		if merchantSubmittedAt.Valid {
			proof.MerchantSubmittedAt = &merchantSubmittedAt.Time
		}
		if amountsMatch.Valid {
			v := amountsMatch.Bool
			proof.AmountsMatch = &v
		}
		if datesMatch.Valid {
			v := datesMatch.Bool
			proof.DatesMatch = &v
		}
		if commissionCollectedAt.Valid {
			proof.CommissionCollectedAt = &commissionCollectedAt.Time
		}
		if disputeRaisedAt.Valid {
			proof.DisputeRaisedAt = &disputeRaisedAt.Time
		}
		if disputeReason.Valid {
			proof.DisputeReason = &disputeReason.String
		}
		if disputeResolvedAt.Valid {
			proof.DisputeResolvedAt = &disputeResolvedAt.Time
		}

		proofs = append(proofs, proof)
	}

	if proofs == nil {
		proofs = []*entity.CODProof{}
	}
	return proofs, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée une nouvelle preuve COD
func (r *CODProofRepositoryInfrastructure) Create(ctx context.Context, proof *entity.CODProof) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	if proof.ShopID != shopID {
		return fmt.Errorf("access denied: proof shop_id does not match tenant shop")
	}

	if err := proof.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO cod_proofs (
			order_id, shop_id, customer_id,
			commission_cents, commission_status,
			status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		proof.OrderID,
		proof.ShopID,
		proof.CustomerID,
		proof.CommissionCents,
		proof.CommissionStatus,
		proof.Status,
	).Scan(&proof.ID, &proof.CreatedAt, &proof.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create COD proof: %w", err)
	}

	return nil
}

// FindByID trouve une preuve par ID
func (r *CODProofRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanProof(r.queryRowContext(ctx, query, id, shopID))
}

// FindByOrderID trouve une preuve par commande
func (r *CODProofRepositoryInfrastructure) FindByOrderID(ctx context.Context, orderID string) (*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE order_id = $1 AND shop_id = $2
	`

	return r.scanProof(r.queryRowContext(ctx, query, orderID, shopID))
}

// FindByShopID retourne toutes les preuves COD d'une boutique
func (r *CODProofRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindByCustomerID retourne toutes les preuves COD d'un client
func (r *CODProofRepositoryInfrastructure) FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE customer_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, customerID, shopID)
}

// FindByStatus retourne les preuves par statut
func (r *CODProofRepositoryInfrastructure) FindByStatus(ctx context.Context, status entity.CODProofStatus) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID, status)
}

// FindByShopIDAndStatus retourne les preuves d'une boutique par statut
func (r *CODProofRepositoryInfrastructure) FindByShopIDAndStatus(ctx context.Context, shopID string, status entity.CODProofStatus) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID, status)
}

// FindPendingProofs retourne les preuves en attente de soumission
func (r *CODProofRepositoryInfrastructure) FindPendingProofs(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'pending_proofs'
		ORDER BY created_at ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindPendingProofsByShopID retourne les preuves en attente d'une boutique
func (r *CODProofRepositoryInfrastructure) FindPendingProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'pending_proofs'
		ORDER BY created_at ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindConfirmedProofs retourne les preuves confirmées (prêtes pour collecte commission)
func (r *CODProofRepositoryInfrastructure) FindConfirmedProofs(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'confirmed'
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindConfirmedProofsByShopID retourne les preuves confirmées d'une boutique
func (r *CODProofRepositoryInfrastructure) FindConfirmedProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'confirmed'
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindDisputedProofs retourne les preuves en litige
func (r *CODProofRepositoryInfrastructure) FindDisputedProofs(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'disputed'
		ORDER BY dispute_raised_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindDisputedProofsByShopID retourne les preuves en litige d'une boutique
func (r *CODProofRepositoryInfrastructure) FindDisputedProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'disputed'
		ORDER BY dispute_raised_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindPastDeadlineProofs retourne les preuves dont la date limite est dépassée
func (r *CODProofRepositoryInfrastructure) FindPastDeadlineProofs(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1
		  AND status = 'pending_proofs'
		  AND created_at < NOW() - INTERVAL '7 days'
		ORDER BY created_at ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindPastDeadlineProofsByShopID retourne les preuves en retard d'une boutique
func (r *CODProofRepositoryInfrastructure) FindPastDeadlineProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1
		  AND status = 'pending_proofs'
		  AND created_at < NOW() - INTERVAL '7 days'
		ORDER BY created_at ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindIncoherentProofs retourne les preuves incohérentes
func (r *CODProofRepositoryInfrastructure) FindIncoherentProofs(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1
		  AND status = 'confirmed'
		  AND (amounts_match = false OR dates_match = false)
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindByCommissionStatus retourne les preuves par statut de commission
func (r *CODProofRepositoryInfrastructure) FindByCommissionStatus(ctx context.Context, status entity.CODCommissionStatus) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND commission_status = $2
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID, status)
}

// FindCommissionDue retourne les preuves avec commission due (wallet négatif)
func (r *CODProofRepositoryInfrastructure) FindCommissionDue(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND commission_status = 'due'
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindCommissionDueByShopID retourne les commissions dues d'une boutique
func (r *CODProofRepositoryInfrastructure) FindCommissionDueByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND commission_status = 'due'
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindCompletedProofs retourne les preuves terminées (commission collectée)
func (r *CODProofRepositoryInfrastructure) FindCompletedProofs(ctx context.Context) ([]*entity.CODProof, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'completed'
		ORDER BY commission_collected_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindCompletedProofsByShopID retourne les preuves terminées d'une boutique
func (r *CODProofRepositoryInfrastructure) FindCompletedProofsByShopID(ctx context.Context, shopID string) ([]*entity.CODProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT id, order_id, shop_id, customer_id,
		       client_payment_proof_url, client_payment_amount_cents,
		       client_payment_date, client_receipt_number, client_notes, client_submitted_at,
		       merchant_receipt_proof_url, merchant_received_amount_cents,
		       merchant_receipt_date, merchant_notes, merchant_submitted_at,
		       amounts_match, dates_match,
		       commission_cents, commission_status, commission_collected_at,
		       status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM cod_proofs
		WHERE shop_id = $1 AND status = 'completed'
		ORDER BY commission_collected_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// CountByStatusByShopID compte les preuves par statut pour une boutique
func (r *CODProofRepositoryInfrastructure) CountByStatusByShopID(ctx context.Context, shopID string, status entity.CODProofStatus) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `SELECT COUNT(*) FROM cod_proofs WHERE shop_id = $1 AND status = $2`

	var count int
	err = r.queryRowContext(ctx, query, shopID, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count COD proofs: %w", err)
	}

	return count, nil
}

// CountPendingByShopID compte les preuves en attente pour une boutique
func (r *CODProofRepositoryInfrastructure) CountPendingByShopID(ctx context.Context, shopID string) (int, error) {
	return r.CountByStatusByShopID(ctx, shopID, entity.CODProofPendingProofs)
}

// CountConfirmedByShopID compte les preuves confirmées pour une boutique
func (r *CODProofRepositoryInfrastructure) CountConfirmedByShopID(ctx context.Context, shopID string) (int, error) {
	return r.CountByStatusByShopID(ctx, shopID, entity.CODProofConfirmed)
}

// CountDisputedByShopID compte les preuves en litige pour une boutique
func (r *CODProofRepositoryInfrastructure) CountDisputedByShopID(ctx context.Context, shopID string) (int, error) {
	return r.CountByStatusByShopID(ctx, shopID, entity.CODProofDisputed)
}

// SumCommissionPendingByShopID somme des commissions en attente pour une boutique
func (r *CODProofRepositoryInfrastructure) SumCommissionPendingByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum commissions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(commission_cents), 0)
		FROM cod_proofs
		WHERE shop_id = $1 AND commission_status = 'pending'
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum pending commissions: %w", err)
	}

	return total, nil
}

// SumCommissionDueByShopID somme des commissions dues pour une boutique
func (r *CODProofRepositoryInfrastructure) SumCommissionDueByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum commissions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(commission_cents), 0)
		FROM cod_proofs
		WHERE shop_id = $1 AND commission_status = 'due'
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum due commissions: %w", err)
	}

	return total, nil
}

// SumCommissionCollectedByShopID somme des commissions collectées pour une boutique
func (r *CODProofRepositoryInfrastructure) SumCommissionCollectedByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum commissions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(commission_cents), 0)
		FROM cod_proofs
		WHERE shop_id = $1 AND commission_status = 'collected'
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum collected commissions: %w", err)
	}

	return total, nil
}

// SumTotalCommissionPending somme totale des commissions en attente (toutes boutiques)
func (r *CODProofRepositoryInfrastructure) SumTotalCommissionPending(ctx context.Context) (int64, error) {
	query := `
		SELECT COALESCE(SUM(commission_cents), 0)
		FROM cod_proofs
		WHERE commission_status = 'pending'
	`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total pending commissions: %w", err)
	}

	return total, nil
}

// SumTotalCommissionDue somme totale des commissions dues (toutes boutiques)
func (r *CODProofRepositoryInfrastructure) SumTotalCommissionDue(ctx context.Context) (int64, error) {
	query := `
		SELECT COALESCE(SUM(commission_cents), 0)
		FROM cod_proofs
		WHERE commission_status = 'due'
	`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total due commissions: %w", err)
	}

	return total, nil
}

// SumTotalCommissionCollected somme totale des commissions collectées (toutes boutiques)
func (r *CODProofRepositoryInfrastructure) SumTotalCommissionCollected(ctx context.Context) (int64, error) {
	query := `
		SELECT COALESCE(SUM(commission_cents), 0)
		FROM cod_proofs
		WHERE commission_status = 'collected'
	`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total collected commissions: %w", err)
	}

	return total, nil
}

// Update met à jour une preuve
func (r *CODProofRepositoryInfrastructure) Update(ctx context.Context, proof *entity.CODProof) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que la preuve appartient à la boutique
	var proofShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM cod_proofs WHERE id = $1`, proof.ID).Scan(&proofShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("COD proof not found")
		}
		return fmt.Errorf("failed to verify proof: %w", err)
	}
	if proofShopID != shopID {
		return fmt.Errorf("access denied: proof does not belong to tenant shop")
	}

	if err := proof.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE cod_proofs
		SET client_payment_proof_url = $2,
		    client_payment_amount_cents = $3,
		    client_payment_date = $4,
		    client_receipt_number = $5,
		    client_notes = $6,
		    client_submitted_at = $7,
		    merchant_receipt_proof_url = $8,
		    merchant_received_amount_cents = $9,
		    merchant_receipt_date = $10,
		    merchant_notes = $11,
		    merchant_submitted_at = $12,
		    amounts_match = $13,
		    dates_match = $14,
		    commission_status = $15,
		    commission_collected_at = $16,
		    status = $17,
		    dispute_raised_at = $18,
		    dispute_reason = $19,
		    dispute_resolved_at = $20,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.queryRowContext(ctx, query,
		proof.ID,
		proof.ClientPaymentProofURL,
		proof.ClientPaymentAmountCents,
		proof.ClientPaymentDate,
		proof.ClientReceiptNumber,
		proof.ClientNotes,
		proof.ClientSubmittedAt,
		proof.MerchantReceiptProofURL,
		proof.MerchantReceivedAmountCents,
		proof.MerchantReceiptDate,
		proof.MerchantNotes,
		proof.MerchantSubmittedAt,
		proof.AmountsMatch,
		proof.DatesMatch,
		proof.CommissionStatus,
		proof.CommissionCollectedAt,
		proof.Status,
		proof.DisputeRaisedAt,
		proof.DisputeReason,
		proof.DisputeResolvedAt,
	).Scan(&proof.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update COD proof: %w", err)
	}

	return nil
}

// UpdateStatus met à jour uniquement le statut
func (r *CODProofRepositoryInfrastructure) UpdateStatus(ctx context.Context, id string, status entity.CODProofStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	result, err := r.execContext(ctx, `
		UPDATE cod_proofs
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1 AND shop_id = $3
	`, id, status, shopID)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("COD proof not found")
	}

	return nil
}

// UpdateCommissionStatus met à jour uniquement le statut de commission
func (r *CODProofRepositoryInfrastructure) UpdateCommissionStatus(ctx context.Context, id string, status entity.CODCommissionStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid commission status: %s", status)
	}

	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	result, err := r.execContext(ctx, `
		UPDATE cod_proofs
		SET commission_status = $2,
		    updated_at = NOW()
		WHERE id = $1 AND shop_id = $3
	`, id, status, shopID)
	if err != nil {
		return fmt.Errorf("failed to update commission status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("COD proof not found")
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si une preuve existe
func (r *CODProofRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `SELECT COUNT(*) FROM cod_proofs WHERE id = $1 AND shop_id = $2`

	var count int
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check proof existence: %w", err)
	}

	return count > 0, nil
}

// HasProofForOrder vérifie si une commande a une preuve COD
func (r *CODProofRepositoryInfrastructure) HasProofForOrder(ctx context.Context, orderID string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `SELECT COUNT(*) FROM cod_proofs WHERE order_id = $1 AND shop_id = $2`

	var count int
	err = r.queryRowContext(ctx, query, orderID, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check proof for order: %w", err)
	}

	return count > 0, nil
}

// CountByShopID compte les preuves d'une boutique
func (r *CODProofRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID string) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count proofs of another shop")
	}

	query := `SELECT COUNT(*) FROM cod_proofs WHERE shop_id = $1`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count COD proofs: %w", err)
	}

	return count, nil
}

// SumTotalCommissionByShopID somme totale des commissions pour une boutique
func (r *CODProofRepositoryInfrastructure) SumTotalCommissionByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum commissions of another shop")
	}

	query := `SELECT COALESCE(SUM(commission_cents), 0) FROM cod_proofs WHERE shop_id = $1`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total commissions: %w", err)
	}

	return total, nil
}

// FindProofsReadyForCollection retourne les preuves confirmées prêtes pour la collecte de commission
// en fonction du délai de la zone de livraison (cod_confirmation_delay_days)
// Utilise COALESCE pour fallback à 7 jours si la zone n'est pas définie
func (r *CODProofRepositoryInfrastructure) FindProofsReadyForCollection(ctx context.Context, limit int) ([]*entity.CODProof, error) {
	query := `
		SELECT cp.id, cp.order_id, cp.shop_id, cp.customer_id,
		       cp.client_payment_proof_url, cp.client_payment_amount_cents,
		       cp.client_payment_date, cp.client_receipt_number, cp.client_notes, cp.client_submitted_at,
		       cp.merchant_receipt_proof_url, cp.merchant_received_amount_cents,
		       cp.merchant_receipt_date, cp.merchant_notes, cp.merchant_submitted_at,
		       cp.amounts_match, cp.dates_match,
		       cp.commission_cents, cp.commission_status, cp.commission_collected_at,
		       cp.status,
		       cp.dispute_raised_at, cp.dispute_reason, cp.dispute_resolved_at,
		       cp.created_at, cp.updated_at
		FROM cod_proofs cp
		LEFT JOIN delivery_zones dz ON cp.delivery_zone_id = dz.id
		WHERE cp.status = 'confirmed'
		  AND cp.commission_status IN ('pending', 'due')
		  AND cp.created_at + (COALESCE(dz.cod_confirmation_delay_days, 7) || ' days')::interval <= NOW()
		ORDER BY cp.created_at ASC
		LIMIT $1
	`
	return r.scanProofs(ctx, query, limit)
}
