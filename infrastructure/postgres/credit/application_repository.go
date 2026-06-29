package credit

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CreditApplicationRepositoryInfrastructure implémente repository.CreditApplicationRepository
type CreditApplicationRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCreditApplicationRepositoryInfrastructure crée une nouvelle instance
func NewCreditApplicationRepositoryInfrastructure(db *sql.DB) repository.CreditApplicationRepository {
	return &CreditApplicationRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CreditApplicationRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CreditApplicationRepository {
	return &CreditApplicationRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *CreditApplicationRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CreditApplicationRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CreditApplicationRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *CreditApplicationRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanApplication scanne une ligne dans une entité CreditApplication
func (r *CreditApplicationRepositoryInfrastructure) scanApplication(row *sql.Row) (*entity.CreditApplication, error) {
	app := &entity.CreditApplication{}
	var rejectionReason, reviewedBy sql.NullString
	var reviewedAt sql.NullTime

	err := row.Scan(
		&app.ID,
		&app.CustomerID,
		&app.ProductID,
		&app.ShopID,
		&app.RequestedDurationMonths,
		&app.CreditScoreAtApplication,
		&app.ProductPriceCents,
		&app.DownPaymentCents,
		&app.FinancedAmountCents,
		&app.InterestAmountCents,
		&app.TotalAmountCents,
		&app.MonthlyPaymentCents,
		&app.Status,
		&rejectionReason,
		&reviewedBy,
		&reviewedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit application not found")
		}
		return nil, fmt.Errorf("failed to scan credit application: %w", err)
	}

	if rejectionReason.Valid {
		app.RejectionReason = &rejectionReason.String
	}
	if reviewedBy.Valid {
		app.ReviewedBy = &reviewedBy.String
	}
	if reviewedAt.Valid {
		app.ReviewedAt = &reviewedAt.Time
	}

	return app, nil
}

// scanApplications scanne plusieurs lignes
func (r *CreditApplicationRepositoryInfrastructure) scanApplications(ctx context.Context, query string, args ...interface{}) ([]*entity.CreditApplication, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query credit applications: %w", err)
	}
	defer rows.Close()

	var apps []*entity.CreditApplication
	for rows.Next() {
		app := &entity.CreditApplication{}
		var rejectionReason, reviewedBy sql.NullString
		var reviewedAt sql.NullTime

		err := rows.Scan(
			&app.ID,
			&app.CustomerID,
			&app.ProductID,
			&app.ShopID,
			&app.RequestedDurationMonths,
			&app.CreditScoreAtApplication,
			&app.ProductPriceCents,
			&app.DownPaymentCents,
			&app.FinancedAmountCents,
			&app.InterestAmountCents,
			&app.TotalAmountCents,
			&app.MonthlyPaymentCents,
			&app.Status,
			&rejectionReason,
			&reviewedBy,
			&reviewedAt,
			&app.CreatedAt,
			&app.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if rejectionReason.Valid {
			app.RejectionReason = &rejectionReason.String
		}
		if reviewedBy.Valid {
			app.ReviewedBy = &reviewedBy.String
		}
		if reviewedAt.Valid {
			app.ReviewedAt = &reviewedAt.Time
		}

		apps = append(apps, app)
	}

	if apps == nil {
		apps = []*entity.CreditApplication{}
	}
	return apps, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée une nouvelle demande de crédit
func (r *CreditApplicationRepositoryInfrastructure) Create(ctx context.Context, app *entity.CreditApplication) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que la demande appartient à la boutique
	if app.ShopID != shopID {
		return fmt.Errorf("access denied: application shop_id does not match tenant shop")
	}

	query := `
		INSERT INTO credit_applications (
			customer_id, product_id, shop_id,
			requested_duration_months, credit_score_at_application,
			product_price_cents, down_payment_cents,
			financed_amount_cents, interest_amount_cents,
			total_amount_cents, monthly_payment_cents,
			status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		app.CustomerID,
		app.ProductID,
		app.ShopID,
		app.RequestedDurationMonths,
		app.CreditScoreAtApplication,
		app.ProductPriceCents,
		app.DownPaymentCents,
		app.FinancedAmountCents,
		app.InterestAmountCents,
		app.TotalAmountCents,
		app.MonthlyPaymentCents,
		app.Status,
	).Scan(&app.ID, &app.CreatedAt, &app.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create credit application: %w", err)
	}

	return nil
}

// FindByID trouve une demande par ID
func (r *CreditApplicationRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.CreditApplication, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, product_id, shop_id,
		       requested_duration_months, credit_score_at_application,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       status, rejection_reason, reviewed_by, reviewed_at,
		       created_at, updated_at
		FROM credit_applications
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanApplication(r.queryRowContext(ctx, query, id, shopID))
}

// FindByCustomerID retourne les demandes d'un client
func (r *CreditApplicationRepositoryInfrastructure) FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditApplication, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, product_id, shop_id,
		       requested_duration_months, credit_score_at_application,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       status, rejection_reason, reviewed_by, reviewed_at,
		       created_at, updated_at
		FROM credit_applications
		WHERE customer_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanApplications(ctx, query, customerID, shopID)
}

// FindPendingByShopID retourne les demandes en attente d'une boutique
func (r *CreditApplicationRepositoryInfrastructure) FindPendingByShopID(ctx context.Context, shopID string) ([]*entity.CreditApplication, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query applications of another shop")
	}

	query := `
		SELECT id, customer_id, product_id, shop_id,
		       requested_duration_months, credit_score_at_application,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       status, rejection_reason, reviewed_by, reviewed_at,
		       created_at, updated_at
		FROM credit_applications
		WHERE shop_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
	`

	return r.scanApplications(ctx, query, shopID)
}

// FindByCustomerAndProduct trouve une demande par client et produit
func (r *CreditApplicationRepositoryInfrastructure) FindByCustomerAndProduct(ctx context.Context, customerID, productID string) (*entity.CreditApplication, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, product_id, shop_id,
		       requested_duration_months, credit_score_at_application,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       status, rejection_reason, reviewed_by, reviewed_at,
		       created_at, updated_at
		FROM credit_applications
		WHERE customer_id = $1 AND product_id = $2 AND shop_id = $3
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanApplication(r.queryRowContext(ctx, query, customerID, productID, shopID))
}

// CountByCustomerAndStatus compte les demandes d'un client par statut
func (r *CreditApplicationRepositoryInfrastructure) CountByCustomerAndStatus(ctx context.Context, customerID string, status entity.CreditApplicationStatus) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM credit_applications
		WHERE customer_id = $1 AND shop_id = $2 AND status = $3
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, shopID, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count credit applications: %w", err)
	}

	return count, nil
}

// Update met à jour une demande
func (r *CreditApplicationRepositoryInfrastructure) Update(ctx context.Context, app *entity.CreditApplication) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que la demande appartient à la boutique
	var appShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM credit_applications WHERE id = $1`, app.ID).Scan(&appShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("credit application not found")
		}
		return fmt.Errorf("failed to verify application: %w", err)
	}
	if appShopID != shopID {
		return fmt.Errorf("access denied: application does not belong to tenant shop")
	}

	query := `
		UPDATE credit_applications
		SET status = $2,
		    rejection_reason = $3,
		    reviewed_by = $4,
		    reviewed_at = $5,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.queryRowContext(ctx, query,
		app.ID,
		app.Status,
		app.RejectionReason,
		app.ReviewedBy,
		app.ReviewedAt,
	).Scan(&app.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update credit application: %w", err)
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si une demande existe
func (r *CreditApplicationRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_applications
		WHERE id = $1 AND shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check application existence: %w", err)
	}

	return count > 0, nil
}

// HasPendingApplication vérifie si un client a une demande en attente
func (r *CreditApplicationRepositoryInfrastructure) HasPendingApplication(ctx context.Context, customerID, productID string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_applications
		WHERE customer_id = $1 AND product_id = $2 AND shop_id = $3 AND status = 'pending'
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, productID, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check pending application: %w", err)
	}

	return count > 0, nil
}

// HasApprovedApplication vérifie si un client a une demande approuvée pour un produit
func (r *CreditApplicationRepositoryInfrastructure) HasApprovedApplication(ctx context.Context, customerID, productID string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_applications
		WHERE customer_id = $1 AND product_id = $2 AND shop_id = $3 AND status = 'approved'
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, productID, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check approved application: %w", err)
	}

	return count > 0, nil
}

// CountPendingByShopID compte les demandes en attente pour une boutique
func (r *CreditApplicationRepositoryInfrastructure) CountPendingByShopID(ctx context.Context, shopID string) (int, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count applications of another shop")
	}

	query := `
		SELECT COUNT(*) FROM credit_applications
		WHERE shop_id = $1 AND status = 'pending'
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending applications: %w", err)
	}

	return count, nil
}

// FindRecentByShopID retourne les N demandes les plus récentes d'une boutique
func (r *CreditApplicationRepositoryInfrastructure) FindRecentByShopID(ctx context.Context, shopID string, limit int) ([]*entity.CreditApplication, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query applications of another shop")
	}

	query := `
		SELECT id, customer_id, product_id, shop_id,
		       requested_duration_months, credit_score_at_application,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       status, rejection_reason, reviewed_by, reviewed_at,
		       created_at, updated_at
		FROM credit_applications
		WHERE shop_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	return r.scanApplications(ctx, query, shopID, limit)
}

// FindByStatus retourne les demandes par statut pour une boutique
func (r *CreditApplicationRepositoryInfrastructure) FindByStatus(ctx context.Context, shopID string, status entity.CreditApplicationStatus) ([]*entity.CreditApplication, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query applications of another shop")
	}

	query := `
		SELECT id, customer_id, product_id, shop_id,
		       requested_duration_months, credit_score_at_application,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       status, rejection_reason, reviewed_by, reviewed_at,
		       created_at, updated_at
		FROM credit_applications
		WHERE shop_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	return r.scanApplications(ctx, query, shopID, status)
}

// SumPendingAmountByShopID somme des montants en attente pour une boutique
func (r *CreditApplicationRepositoryInfrastructure) SumPendingAmountByShopID(ctx context.Context, shopID string) (int64, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum applications of another shop")
	}

	query := `
		SELECT COALESCE(SUM(total_amount_cents), 0)
		FROM credit_applications
		WHERE shop_id = $1 AND status = 'pending'
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum pending applications: %w", err)
	}

	return total, nil
}
