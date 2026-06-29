package escrow

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// ============================================================
// DELIVERY PROOF REPOSITORY
// ============================================================

// DeliveryProofRepositoryInfrastructure implémente repository.DeliveryProofRepository
type DeliveryProofRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewDeliveryProofRepositoryInfrastructure crée une nouvelle instance
func NewDeliveryProofRepositoryInfrastructure(db *sql.DB) repository.DeliveryProofRepository {
	return &DeliveryProofRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *DeliveryProofRepositoryInfrastructure) WithTX(tx repository.Tx) repository.DeliveryProofRepository {
	return &DeliveryProofRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *DeliveryProofRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *DeliveryProofRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *DeliveryProofRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *DeliveryProofRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanProof scanne une ligne dans une entité DeliveryProof
func (r *DeliveryProofRepositoryInfrastructure) scanProof(row *sql.Row) (*entity.DeliveryProof, error) {
	proof := &entity.DeliveryProof{}

	// Références polymorphiques
	var orderID, creditContractID, tontineVoucherID sql.NullString

	// Preuves marchand
	var shippingProofURL, shippingTrackingNumber, shippingCarrier, shippingNotes sql.NullString
	var shippingDate sql.NullTime

	// Preuves client
	var deliveryProofURL, deliverySignature, deliveryNotes sql.NullString
	var deliveryDate sql.NullTime
	var deliveryRating sql.NullInt64

	// Litige
	var disputeRaisedAt, disputeResolvedAt sql.NullTime
	var disputeReason, disputeResolution sql.NullString

	err := row.Scan(
		&proof.ID,
		&orderID,
		&creditContractID,
		&tontineVoucherID,
		&shippingProofURL,
		&shippingTrackingNumber,
		&shippingCarrier,
		&shippingDate,
		&shippingNotes,
		&deliveryProofURL,
		&deliverySignature,
		&deliveryDate,
		&deliveryNotes,
		&deliveryRating,
		&proof.EscrowStatus,
		&disputeRaisedAt,
		&disputeReason,
		&disputeResolvedAt,
		&proof.CreatedAt,
		&proof.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("delivery proof not found")
		}
		return nil, fmt.Errorf("failed to scan delivery proof: %w", err)
	}

	// Références
	if orderID.Valid {
		proof.OrderID = &orderID.String
	}
	if creditContractID.Valid {
		proof.CreditContractID = &creditContractID.String
	}
	if tontineVoucherID.Valid {
		proof.TontineVoucherID = &tontineVoucherID.String
	}

	// Preuves marchand
	if shippingProofURL.Valid {
		proof.ShippingProofURL = &shippingProofURL.String
	}
	if shippingTrackingNumber.Valid {
		proof.ShippingTrackingNumber = &shippingTrackingNumber.String
	}
	if shippingCarrier.Valid {
		proof.ShippingCarrier = &shippingCarrier.String
	}
	if shippingDate.Valid {
		proof.ShippingDate = &shippingDate.Time
	}
	if shippingNotes.Valid {
		proof.ShippingNotes = &shippingNotes.String
	}

	// Preuves client
	if deliveryProofURL.Valid {
		proof.DeliveryProofURL = &deliveryProofURL.String
	}
	if deliverySignature.Valid {
		proof.DeliverySignature = &deliverySignature.String
	}
	if deliveryDate.Valid {
		proof.DeliveryDate = &deliveryDate.Time
	}
	if deliveryNotes.Valid {
		proof.DeliveryNotes = &deliveryNotes.String
	}
	if deliveryRating.Valid {
		rating := int(deliveryRating.Int64)
		proof.DeliveryRating = &rating
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
	if disputeResolution.Valid {
		proof.DisputeResolution = &disputeResolution.String
	}

	return proof, nil
}

// scanProofs scanne plusieurs lignes
func (r *DeliveryProofRepositoryInfrastructure) scanProofs(ctx context.Context, query string, args ...interface{}) ([]*entity.DeliveryProof, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query delivery proofs: %w", err)
	}
	defer rows.Close()

	var proofs []*entity.DeliveryProof
	for rows.Next() {
		proof := &entity.DeliveryProof{}

		var orderID, creditContractID, tontineVoucherID sql.NullString
		var shippingProofURL, shippingTrackingNumber, shippingCarrier, shippingNotes sql.NullString
		var shippingDate sql.NullTime
		var deliveryProofURL, deliverySignature, deliveryNotes sql.NullString
		var deliveryDate sql.NullTime
		var deliveryRating sql.NullInt64
		var disputeRaisedAt, disputeResolvedAt sql.NullTime
		var disputeReason, disputeResolution sql.NullString

		err := rows.Scan(
			&proof.ID,
			&orderID,
			&creditContractID,
			&tontineVoucherID,
			&shippingProofURL,
			&shippingTrackingNumber,
			&shippingCarrier,
			&shippingDate,
			&shippingNotes,
			&deliveryProofURL,
			&deliverySignature,
			&deliveryDate,
			&deliveryNotes,
			&deliveryRating,
			&proof.EscrowStatus,
			&disputeRaisedAt,
			&disputeReason,
			&disputeResolvedAt,
			&proof.CreatedAt,
			&proof.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if orderID.Valid {
			proof.OrderID = &orderID.String
		}
		if creditContractID.Valid {
			proof.CreditContractID = &creditContractID.String
		}
		if tontineVoucherID.Valid {
			proof.TontineVoucherID = &tontineVoucherID.String
		}
		if shippingProofURL.Valid {
			proof.ShippingProofURL = &shippingProofURL.String
		}
		if shippingTrackingNumber.Valid {
			proof.ShippingTrackingNumber = &shippingTrackingNumber.String
		}
		if shippingCarrier.Valid {
			proof.ShippingCarrier = &shippingCarrier.String
		}
		if shippingDate.Valid {
			proof.ShippingDate = &shippingDate.Time
		}
		if shippingNotes.Valid {
			proof.ShippingNotes = &shippingNotes.String
		}
		if deliveryProofURL.Valid {
			proof.DeliveryProofURL = &deliveryProofURL.String
		}
		if deliverySignature.Valid {
			proof.DeliverySignature = &deliverySignature.String
		}
		if deliveryDate.Valid {
			proof.DeliveryDate = &deliveryDate.Time
		}
		if deliveryNotes.Valid {
			proof.DeliveryNotes = &deliveryNotes.String
		}
		if deliveryRating.Valid {
			rating := int(deliveryRating.Int64)
			proof.DeliveryRating = &rating
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
		if disputeResolution.Valid {
			proof.DisputeResolution = &disputeResolution.String
		}

		proofs = append(proofs, proof)
	}

	if proofs == nil {
		proofs = []*entity.DeliveryProof{}
	}
	return proofs, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée une nouvelle preuve de livraison
func (r *DeliveryProofRepositoryInfrastructure) Create(ctx context.Context, proof *entity.DeliveryProof) error {
	query := `
		INSERT INTO delivery_proofs (
			order_id, credit_contract_id, tontine_voucher_id,
			escrow_status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err := r.queryRowContext(ctx, query,
		proof.OrderID,
		proof.CreditContractID,
		proof.TontineVoucherID,
		proof.EscrowStatus,
	).Scan(&proof.ID, &proof.CreatedAt, &proof.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create delivery proof: %w", err)
	}

	return nil
}

// FindByID trouve une preuve par ID
func (r *DeliveryProofRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE id = $1
	`

	return r.scanProof(r.queryRowContext(ctx, query, id))
}

// FindByOrderID trouve une preuve par commande
func (r *DeliveryProofRepositoryInfrastructure) FindByOrderID(ctx context.Context, orderID string) (*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE order_id = $1
	`

	return r.scanProof(r.queryRowContext(ctx, query, orderID))
}

// FindByCreditContractID trouve une preuve par contrat de crédit
func (r *DeliveryProofRepositoryInfrastructure) FindByCreditContractID(ctx context.Context, contractID string) (*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE credit_contract_id = $1
	`

	return r.scanProof(r.queryRowContext(ctx, query, contractID))
}

// FindByTontineVoucherID trouve une preuve par voucher tontine
func (r *DeliveryProofRepositoryInfrastructure) FindByTontineVoucherID(ctx context.Context, voucherID string) (*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE tontine_voucher_id = $1
	`

	return r.scanProof(r.queryRowContext(ctx, query, voucherID))
}

// FindByReferenceID trouve une preuve par référence (polymorphique)
func (r *DeliveryProofRepositoryInfrastructure) FindByReferenceID(ctx context.Context, referenceType string, referenceID string) (*entity.DeliveryProof, error) {
	var query string
	switch referenceType {
	case "order":
		query = `
			SELECT id, order_id, credit_contract_id, tontine_voucher_id,
			       shipping_proof_url, shipping_tracking_number, shipping_carrier,
			       shipping_date, shipping_notes,
			       delivery_proof_url, delivery_signature, delivery_date,
			       delivery_notes, delivery_rating,
			       escrow_status,
			       dispute_raised_at, dispute_reason, dispute_resolved_at,
			       created_at, updated_at
			FROM delivery_proofs
			WHERE order_id = $1
		`
	case "credit_contract":
		query = `
			SELECT id, order_id, credit_contract_id, tontine_voucher_id,
			       shipping_proof_url, shipping_tracking_number, shipping_carrier,
			       shipping_date, shipping_notes,
			       delivery_proof_url, delivery_signature, delivery_date,
			       delivery_notes, delivery_rating,
			       escrow_status,
			       dispute_raised_at, dispute_reason, dispute_resolved_at,
			       created_at, updated_at
			FROM delivery_proofs
			WHERE credit_contract_id = $1
		`
	case "tontine_voucher":
		query = `
			SELECT id, order_id, credit_contract_id, tontine_voucher_id,
			       shipping_proof_url, shipping_tracking_number, shipping_carrier,
			       shipping_date, shipping_notes,
			       delivery_proof_url, delivery_signature, delivery_date,
			       delivery_notes, delivery_rating,
			       escrow_status,
			       dispute_raised_at, dispute_reason, dispute_resolved_at,
			       created_at, updated_at
			FROM delivery_proofs
			WHERE tontine_voucher_id = $1
		`
	default:
		return nil, fmt.Errorf("invalid reference type: %s", referenceType)
	}

	return r.scanProof(r.queryRowContext(ctx, query, referenceID))
}

// FindByStatus retourne les preuves par statut escrow
func (r *DeliveryProofRepositoryInfrastructure) FindByStatus(ctx context.Context, status entity.EscrowStatus) ([]*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE escrow_status = $1
		ORDER BY created_at DESC
	`

	return r.scanProofs(ctx, query, status)
}

// FindPendingShipmentByShopID retourne les preuves en attente d'envoi d'une boutique
func (r *DeliveryProofRepositoryInfrastructure) FindPendingShipmentByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT dp.id, dp.order_id, dp.credit_contract_id, dp.tontine_voucher_id,
		       dp.shipping_proof_url, dp.shipping_tracking_number, dp.shipping_carrier,
		       dp.shipping_date, dp.shipping_notes,
		       dp.delivery_proof_url, dp.delivery_signature, dp.delivery_date,
		       dp.delivery_notes, dp.delivery_rating,
		       dp.escrow_status,
		       dp.dispute_raised_at, dp.dispute_reason, dp.dispute_resolved_at,
		       dp.created_at, dp.updated_at
		FROM delivery_proofs dp
		LEFT JOIN orders o ON o.id = dp.order_id
		LEFT JOIN credit_contracts cc ON cc.id = dp.credit_contract_id
		LEFT JOIN tontine_vouchers tv ON tv.id = dp.tontine_voucher_id
		WHERE dp.escrow_status = 'pending_shipment'
		  AND (
		    (dp.order_id IS NOT NULL AND o.shop_id = $1) OR
		    (dp.credit_contract_id IS NOT NULL AND cc.shop_id = $1) OR
		    (dp.tontine_voucher_id IS NOT NULL AND tv.shop_id = $1)
		  )
		ORDER BY dp.created_at ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindShippedByShopID retourne les preuves expédiées d'une boutique
func (r *DeliveryProofRepositoryInfrastructure) FindShippedByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT dp.id, dp.order_id, dp.credit_contract_id, dp.tontine_voucher_id,
		       dp.shipping_proof_url, dp.shipping_tracking_number, dp.shipping_carrier,
		       dp.shipping_date, dp.shipping_notes,
		       dp.delivery_proof_url, dp.delivery_signature, dp.delivery_date,
		       dp.delivery_notes, dp.delivery_rating,
		       dp.escrow_status,
		       dp.dispute_raised_at, dp.dispute_reason, dp.dispute_resolved_at,
		       dp.created_at, dp.updated_at
		FROM delivery_proofs dp
		LEFT JOIN orders o ON o.id = dp.order_id
		LEFT JOIN credit_contracts cc ON cc.id = dp.credit_contract_id
		LEFT JOIN tontine_vouchers tv ON tv.id = dp.tontine_voucher_id
		WHERE dp.escrow_status = 'shipped'
		  AND (
		    (dp.order_id IS NOT NULL AND o.shop_id = $1) OR
		    (dp.credit_contract_id IS NOT NULL AND cc.shop_id = $1) OR
		    (dp.tontine_voucher_id IS NOT NULL AND tv.shop_id = $1)
		  )
		ORDER BY dp.shipping_date ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindDeliveredByShopID retourne les preuves livrées d'une boutique
func (r *DeliveryProofRepositoryInfrastructure) FindDeliveredByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT dp.id, dp.order_id, dp.credit_contract_id, dp.tontine_voucher_id,
		       dp.shipping_proof_url, dp.shipping_tracking_number, dp.shipping_carrier,
		       dp.shipping_date, dp.shipping_notes,
		       dp.delivery_proof_url, dp.delivery_signature, dp.delivery_date,
		       dp.delivery_notes, dp.delivery_rating,
		       dp.escrow_status,
		       dp.dispute_raised_at, dp.dispute_reason, dp.dispute_resolved_at,
		       dp.created_at, dp.updated_at
		FROM delivery_proofs dp
		LEFT JOIN orders o ON o.id = dp.order_id
		LEFT JOIN credit_contracts cc ON cc.id = dp.credit_contract_id
		LEFT JOIN tontine_vouchers tv ON tv.id = dp.tontine_voucher_id
		WHERE dp.escrow_status = 'delivered'
		  AND (
		    (dp.order_id IS NOT NULL AND o.shop_id = $1) OR
		    (dp.credit_contract_id IS NOT NULL AND cc.shop_id = $1) OR
		    (dp.tontine_voucher_id IS NOT NULL AND tv.shop_id = $1)
		  )
		ORDER BY dp.delivery_date ASC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindDisputedByShopID retourne les preuves en litige d'une boutique
func (r *DeliveryProofRepositoryInfrastructure) FindDisputedByShopID(ctx context.Context, shopID string) ([]*entity.DeliveryProof, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT dp.id, dp.order_id, dp.credit_contract_id, dp.tontine_voucher_id,
		       dp.shipping_proof_url, dp.shipping_tracking_number, dp.shipping_carrier,
		       dp.shipping_date, dp.shipping_notes,
		       dp.delivery_proof_url, dp.delivery_signature, dp.delivery_date,
		       dp.delivery_notes, dp.delivery_rating,
		       dp.escrow_status,
		       dp.dispute_raised_at, dp.dispute_reason, dp.dispute_resolved_at,
		       dp.created_at, dp.updated_at
		FROM delivery_proofs dp
		LEFT JOIN orders o ON o.id = dp.order_id
		LEFT JOIN credit_contracts cc ON cc.id = dp.credit_contract_id
		LEFT JOIN tontine_vouchers tv ON tv.id = dp.tontine_voucher_id
		WHERE dp.escrow_status = 'disputed'
		  AND (
		    (dp.order_id IS NOT NULL AND o.shop_id = $1) OR
		    (dp.credit_contract_id IS NOT NULL AND cc.shop_id = $1) OR
		    (dp.tontine_voucher_id IS NOT NULL AND tv.shop_id = $1)
		  )
		ORDER BY dp.dispute_raised_at DESC
	`

	return r.scanProofs(ctx, query, shopID)
}

// FindAutoReleaseEligible retourne les preuves éligibles au déblocage automatique
func (r *DeliveryProofRepositoryInfrastructure) FindAutoReleaseEligible(ctx context.Context) ([]*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE escrow_status = 'delivered'
		  AND delivery_date IS NOT NULL
		  AND delivery_date < NOW() - INTERVAL '3 days'
		  AND (dispute_raised_at IS NULL OR dispute_resolved_at IS NOT NULL)
		ORDER BY delivery_date ASC
	`

	return r.scanProofs(ctx, query)
}

// FindDisputeDeadlineExpired retourne les preuves dont le délai de litige est expiré
func (r *DeliveryProofRepositoryInfrastructure) FindDisputeDeadlineExpired(ctx context.Context) ([]*entity.DeliveryProof, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_voucher_id,
		       shipping_proof_url, shipping_tracking_number, shipping_carrier,
		       shipping_date, shipping_notes,
		       delivery_proof_url, delivery_signature, delivery_date,
		       delivery_notes, delivery_rating,
		       escrow_status,
		       dispute_raised_at, dispute_reason, dispute_resolved_at,
		       created_at, updated_at
		FROM delivery_proofs
		WHERE escrow_status = 'delivered'
		  AND delivery_date IS NOT NULL
		  AND delivery_date < NOW() - INTERVAL '72 hours'
		  AND dispute_raised_at IS NULL
		ORDER BY delivery_date ASC
	`

	return r.scanProofs(ctx, query)
}

// CountByStatusByShopID compte les preuves par statut pour une boutique
func (r *DeliveryProofRepositoryInfrastructure) CountByStatusByShopID(ctx context.Context, shopID string, status entity.EscrowStatus) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query proofs of another shop")
	}

	query := `
		SELECT COUNT(*)
		FROM delivery_proofs dp
		LEFT JOIN orders o ON o.id = dp.order_id
		LEFT JOIN credit_contracts cc ON cc.id = dp.credit_contract_id
		LEFT JOIN tontine_vouchers tv ON tv.id = dp.tontine_voucher_id
		WHERE dp.escrow_status = $2
		  AND (
		    (dp.order_id IS NOT NULL AND o.shop_id = $1) OR
		    (dp.credit_contract_id IS NOT NULL AND cc.shop_id = $1) OR
		    (dp.tontine_voucher_id IS NOT NULL AND tv.shop_id = $1)
		  )
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count delivery proofs: %w", err)
	}

	return count, nil
}

// Update met à jour une preuve
func (r *DeliveryProofRepositoryInfrastructure) Update(ctx context.Context, proof *entity.DeliveryProof) error {
	query := `
		UPDATE delivery_proofs
		SET shipping_proof_url = $2,
		    shipping_tracking_number = $3,
		    shipping_carrier = $4,
		    shipping_date = $5,
		    shipping_notes = $6,
		    delivery_proof_url = $7,
		    delivery_signature = $8,
		    delivery_date = $9,
		    delivery_notes = $10,
		    delivery_rating = $11,
		    escrow_status = $12,
		    dispute_raised_at = $13,
		    dispute_reason = $14,
		    dispute_resolved_at = $15,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.queryRowContext(ctx, query,
		proof.ID,
		proof.ShippingProofURL,
		proof.ShippingTrackingNumber,
		proof.ShippingCarrier,
		proof.ShippingDate,
		proof.ShippingNotes,
		proof.DeliveryProofURL,
		proof.DeliverySignature,
		proof.DeliveryDate,
		proof.DeliveryNotes,
		proof.DeliveryRating,
		proof.EscrowStatus,
		proof.DisputeRaisedAt,
		proof.DisputeReason,
		proof.DisputeResolvedAt,
	).Scan(&proof.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update delivery proof: %w", err)
	}

	return nil
}

// UpdateEscrowStatus met à jour uniquement le statut escrow
func (r *DeliveryProofRepositoryInfrastructure) UpdateEscrowStatus(ctx context.Context, id string, status entity.EscrowStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid escrow status: %s", status)
	}

	result, err := r.execContext(ctx, `
		UPDATE delivery_proofs
		SET escrow_status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, status)
	if err != nil {
		return fmt.Errorf("failed to update escrow status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("delivery proof not found")
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si une preuve existe
func (r *DeliveryProofRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT COUNT(*) FROM delivery_proofs WHERE id = $1`

	var count int
	err := r.queryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check proof existence: %w", err)
	}

	return count > 0, nil
}

// HasProofForOrder vérifie si une commande a une preuve
func (r *DeliveryProofRepositoryInfrastructure) HasProofForOrder(ctx context.Context, orderID string) (bool, error) {
	query := `SELECT COUNT(*) FROM delivery_proofs WHERE order_id = $1`

	var count int
	err := r.queryRowContext(ctx, query, orderID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check proof for order: %w", err)
	}

	return count > 0, nil
}

// CountByStatus compte les preuves par statut
func (r *DeliveryProofRepositoryInfrastructure) CountByStatus(ctx context.Context, status entity.EscrowStatus) (int, error) {
	query := `SELECT COUNT(*) FROM delivery_proofs WHERE escrow_status = $1`

	var count int
	err := r.queryRowContext(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count delivery proofs: %w", err)
	}

	return count, nil
}

// ============================================================
// DELIVERY PROOF EVENT REPOSITORY
// ============================================================

// DeliveryProofEventRepositoryInfrastructure implémente repository.DeliveryProofEventRepository
type DeliveryProofEventRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewDeliveryProofEventRepositoryInfrastructure crée une nouvelle instance
func NewDeliveryProofEventRepositoryInfrastructure(db *sql.DB) repository.DeliveryProofEventRepository {
	return &DeliveryProofEventRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *DeliveryProofEventRepositoryInfrastructure) WithTX(tx repository.Tx) repository.DeliveryProofEventRepository {
	return &DeliveryProofEventRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS (pour DeliveryProofEventRepositoryInfrastructure)
// ============================================================

func (r *DeliveryProofEventRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *DeliveryProofEventRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *DeliveryProofEventRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// scanEvent scanne une ligne dans une entité DeliveryProofEvent
func (r *DeliveryProofEventRepositoryInfrastructure) scanEvent(row *sql.Row) (*entity.DeliveryProofEvent, error) {
	event := &entity.DeliveryProofEvent{}

	err := row.Scan(
		&event.ID,
		&event.DeliveryProofID,
		&event.EventType,
		&event.PerformedBy,
		&event.PerformedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("delivery proof event not found")
		}
		return nil, fmt.Errorf("failed to scan delivery proof event: %w", err)
	}

	return event, nil
}

// scanEvents scanne plusieurs lignes
func (r *DeliveryProofEventRepositoryInfrastructure) scanEvents(ctx context.Context, query string, args ...interface{}) ([]*entity.DeliveryProofEvent, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query delivery proof events: %w", err)
	}
	defer rows.Close()

	var events []*entity.DeliveryProofEvent
	for rows.Next() {
		event := &entity.DeliveryProofEvent{}
		err := rows.Scan(
			&event.ID,
			&event.DeliveryProofID,
			&event.EventType,
			&event.PerformedBy,
			&event.PerformedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		events = append(events, event)
	}

	if events == nil {
		events = []*entity.DeliveryProofEvent{}
	}
	return events, rows.Err()
}

// Create crée un nouvel événement
func (r *DeliveryProofEventRepositoryInfrastructure) Create(ctx context.Context, event *entity.DeliveryProofEvent) error {
	query := `
		INSERT INTO delivery_proof_events (
			delivery_proof_id, event_type, event_data,
			performed_by, performed_at
		) VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, performed_at
	`

	err := r.queryRowContext(ctx, query,
		event.DeliveryProofID,
		event.EventType,
		event.EventData,
		event.PerformedBy,
	).Scan(&event.ID, &event.PerformedAt)

	if err != nil {
		return fmt.Errorf("failed to create delivery proof event: %w", err)
	}

	return nil
}

// FindByProofID retourne tous les événements d'une preuve (triés par date)
func (r *DeliveryProofEventRepositoryInfrastructure) FindByProofID(ctx context.Context, proofID string) ([]*entity.DeliveryProofEvent, error) {
	query := `
		SELECT id, delivery_proof_id, event_type, performed_by, performed_at
		FROM delivery_proof_events
		WHERE delivery_proof_id = $1
		ORDER BY performed_at ASC
	`

	return r.scanEvents(ctx, query, proofID)
}

// FindByProofIDAndType retourne les événements d'une preuve par type
func (r *DeliveryProofEventRepositoryInfrastructure) FindByProofIDAndType(ctx context.Context, proofID string, eventType entity.ProofEventType) ([]*entity.DeliveryProofEvent, error) {
	query := `
		SELECT id, delivery_proof_id, event_type, performed_by, performed_at
		FROM delivery_proof_events
		WHERE delivery_proof_id = $1 AND event_type = $2
		ORDER BY performed_at ASC
	`

	return r.scanEvents(ctx, query, proofID, eventType)
}

// FindByPerformedBy retourne les événements effectués par un utilisateur
func (r *DeliveryProofEventRepositoryInfrastructure) FindByPerformedBy(ctx context.Context, userID string) ([]*entity.DeliveryProofEvent, error) {
	query := `
		SELECT id, delivery_proof_id, event_type, performed_by, performed_at
		FROM delivery_proof_events
		WHERE performed_by = $1
		ORDER BY performed_at DESC
	`

	return r.scanEvents(ctx, query, userID)
}

// FindRecentByProofID retourne les N derniers événements d'une preuve
func (r *DeliveryProofEventRepositoryInfrastructure) FindRecentByProofID(ctx context.Context, proofID string, limit int) ([]*entity.DeliveryProofEvent, error) {
	query := `
		SELECT id, delivery_proof_id, event_type, performed_by, performed_at
		FROM delivery_proof_events
		WHERE delivery_proof_id = $1
		ORDER BY performed_at DESC
		LIMIT $2
	`

	return r.scanEvents(ctx, query, proofID, limit)
}

// CountByProofID compte les événements d'une preuve
func (r *DeliveryProofEventRepositoryInfrastructure) CountByProofID(ctx context.Context, proofID string) (int, error) {
	query := `SELECT COUNT(*) FROM delivery_proof_events WHERE delivery_proof_id = $1`

	var count int
	err := r.queryRowContext(ctx, query, proofID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}

	return count, nil
}
