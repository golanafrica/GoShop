package dispute

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

type DisputeRepositoryPostgres struct {
	db *sql.DB
	tx repository.Tx
}

func NewDisputeRepositoryPostgres(db *sql.DB) repository.DisputeRepository {
	return &DisputeRepositoryPostgres{db: db}
}

func (r *DisputeRepositoryPostgres) WithTX(tx repository.Tx) repository.DisputeRepository {
	return &DisputeRepositoryPostgres{db: r.db, tx: tx}
}

func (r *DisputeRepositoryPostgres) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *DisputeRepositoryPostgres) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *DisputeRepositoryPostgres) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// Create insère un nouveau litige en base de données
func (r *DisputeRepositoryPostgres) Create(ctx context.Context, dispute *entity.Dispute) error {
	// ✅ AJOUT de shop_id dans la liste des colonnes
	query := `
		INSERT INTO disputes (
			id, order_id, shop_id, payment_id, initiator_id, initiator_role, 
			reason, status, resolution_notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	var paymentID *string
	if dispute.PaymentID != nil {
		s := dispute.PaymentID.String()
		paymentID = &s
	}

	_, err := r.execContext(ctx, query,
		dispute.ID,
		dispute.OrderID,
		dispute.ShopID, // ✅ 3ème argument
		paymentID,
		dispute.InitiatorID,
		dispute.InitiatorRole,
		dispute.Reason,
		dispute.Status,
		dispute.ResolutionNotes,
		dispute.CreatedAt,
		dispute.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create dispute: %w", err)
	}
	return nil
}

// FindByID récupère un litige par son ID
func (r *DisputeRepositoryPostgres) FindByID(ctx context.Context, id uuid.UUID) (*entity.Dispute, error) {
	// ✅ AJOUT de shop_id dans le SELECT
	query := `
		SELECT id, order_id, shop_id, payment_id, initiator_id, initiator_role, 
		       reason, status, resolution_notes, created_at, updated_at
		FROM disputes
		WHERE id = $1
	`
	return r.scanDispute(r.queryRowContext(ctx, query, id))
}

// FindByOrderID récupère le litige associé à une commande
func (r *DisputeRepositoryPostgres) FindByOrderID(ctx context.Context, orderID uuid.UUID) (*entity.Dispute, error) {
	// ✅ AJOUT de shop_id dans le SELECT
	query := `
		SELECT id, order_id, shop_id, payment_id, initiator_id, initiator_role, 
		       reason, status, resolution_notes, created_at, updated_at
		FROM disputes
		WHERE order_id = $1
	`
	return r.scanDispute(r.queryRowContext(ctx, query, orderID))
}

// FindByPaymentID récupère le litige associé à un paiement
func (r *DisputeRepositoryPostgres) FindByPaymentID(ctx context.Context, paymentID uuid.UUID) (*entity.Dispute, error) {
	// ✅ AJOUT de shop_id dans le SELECT
	query := `
		SELECT id, order_id, shop_id, payment_id, initiator_id, initiator_role, 
		       reason, status, resolution_notes, created_at, updated_at
		FROM disputes
		WHERE payment_id = $1
	`
	return r.scanDispute(r.queryRowContext(ctx, query, paymentID))
}

// ExistsByOrderID vérifie si un litige actif existe pour une commande
func (r *DisputeRepositoryPostgres) ExistsByOrderID(ctx context.Context, orderID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM disputes 
			WHERE order_id = $1 AND status NOT IN ('cancelled', 'resolved_merchant', 'resolved_customer')
		)
	`
	var exists bool
	err := r.queryRowContext(ctx, query, orderID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check dispute exists: %w", err)
	}
	return exists, nil
}

// Update met à jour un litige existant
func (r *DisputeRepositoryPostgres) Update(ctx context.Context, dispute *entity.Dispute) error {
	query := `
		UPDATE disputes SET
			status = $1,
			resolution_notes = $2,
			updated_at = $3
		WHERE id = $4
	`

	result, err := r.execContext(ctx, query,
		dispute.Status,
		dispute.ResolutionNotes,
		dispute.UpdatedAt,
		dispute.ID,
	)
	if err != nil {
		return fmt.Errorf("update dispute: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("dispute not found or access denied")
	}

	return nil
}

// List retourne une liste de litiges avec pagination et filtres
func (r *DisputeRepositoryPostgres) List(ctx context.Context, filters repository.DisputeFilters) ([]*entity.Dispute, error) {
	// ✅ AJOUT de shop_id dans le SELECT
	query := `
		SELECT id, order_id, shop_id, payment_id, initiator_id, initiator_role, 
		       reason, status, resolution_notes, created_at, updated_at
		FROM disputes
		WHERE 1=1
	`
	var args []interface{}
	argPos := 1

	if filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, filters.Status)
		argPos++
	}

	if filters.InitiatorRole != "" {
		query += fmt.Sprintf(" AND initiator_role = $%d", argPos)
		args = append(args, filters.InitiatorRole)
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
		return nil, fmt.Errorf("list disputes: %w", err)
	}
	defer rows.Close()

	return r.scanDisputes(rows)
}

// Count retourne le nombre total de litiges correspondant aux filtres
func (r *DisputeRepositoryPostgres) Count(ctx context.Context, filters repository.DisputeFilters) (int, error) {
	query := `SELECT COUNT(*) FROM disputes WHERE 1=1`
	var args []interface{}
	argPos := 1

	if filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, filters.Status)
		argPos++
	}

	if filters.InitiatorRole != "" {
		query += fmt.Sprintf(" AND initiator_role = $%d", argPos)
		args = append(args, filters.InitiatorRole)
		argPos++
	}

	var count int
	err := r.queryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count disputes: %w", err)
	}

	return count, nil
}

// ============================================================
// Helpers de scan
// ============================================================

func (r *DisputeRepositoryPostgres) scanDispute(row *sql.Row) (*entity.Dispute, error) {
	var d entity.Dispute
	var paymentID sql.NullString
	var resolutionNotes sql.NullString

	// ✅ AJOUT de &d.ShopID dans le Scan (3ème position)
	err := row.Scan(
		&d.ID,
		&d.OrderID,
		&d.ShopID,
		&paymentID,
		&d.InitiatorID,
		&d.InitiatorRole,
		&d.Reason,
		&d.Status,
		&resolutionNotes,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dispute not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan dispute: %w", err)
	}

	if paymentID.Valid {
		pid, err := uuid.Parse(paymentID.String)
		if err == nil {
			d.PaymentID = &pid
		}
	}

	if resolutionNotes.Valid {
		d.ResolutionNotes = &resolutionNotes.String
	}

	return &d, nil
}

func (r *DisputeRepositoryPostgres) scanDisputes(rows *sql.Rows) ([]*entity.Dispute, error) {
	var disputes []*entity.Dispute

	for rows.Next() {
		var d entity.Dispute
		var paymentID sql.NullString
		var resolutionNotes sql.NullString

		// ✅ AJOUT de &d.ShopID dans le Scan (3ème position)
		err := rows.Scan(
			&d.ID,
			&d.OrderID,
			&d.ShopID,
			&paymentID,
			&d.InitiatorID,
			&d.InitiatorRole,
			&d.Reason,
			&d.Status,
			&resolutionNotes,
			&d.CreatedAt,
			&d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan dispute row: %w", err)
		}

		if paymentID.Valid {
			pid, err := uuid.Parse(paymentID.String)
			if err == nil {
				d.PaymentID = &pid
			}
		}

		if resolutionNotes.Valid {
			d.ResolutionNotes = &resolutionNotes.String
		}

		disputes = append(disputes, &d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate disputes: %w", err)
	}

	return disputes, nil
}
