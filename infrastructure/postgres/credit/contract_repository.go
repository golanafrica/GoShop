package credit

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CreditContractRepositoryInfrastructure implémente repository.CreditContractRepository
type CreditContractRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCreditContractRepositoryInfrastructure crée une nouvelle instance
func NewCreditContractRepositoryInfrastructure(db *sql.DB) repository.CreditContractRepository {
	return &CreditContractRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CreditContractRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CreditContractRepository {
	return &CreditContractRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *CreditContractRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CreditContractRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CreditContractRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *CreditContractRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanContract scanne une ligne dans une entité CreditContract
func (r *CreditContractRepositoryInfrastructure) scanContract(row *sql.Row) (*entity.CreditContract, error) {
	contract := &entity.CreditContract{}
	var downPaymentPaidAt, completedAt sql.NullTime

	err := row.Scan(
		&contract.ID,
		&contract.ApplicationID,
		&contract.CustomerID,
		&contract.ProductID,
		&contract.ShopID,
		&contract.ProductPriceCents,
		&contract.DownPaymentCents,
		&contract.FinancedAmountCents,
		&contract.InterestAmountCents,
		&contract.TotalAmountCents,
		&contract.MonthlyPaymentCents,
		&contract.DurationMonths,
		&contract.StartDate,
		&contract.EndDate,
		&contract.Status,
		&downPaymentPaidAt,
		&completedAt,
		&contract.CreatedAt,
		&contract.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit contract not found")
		}
		return nil, fmt.Errorf("failed to scan credit contract: %w", err)
	}

	if downPaymentPaidAt.Valid {
		contract.DownPaymentPaidAt = &downPaymentPaidAt.Time
	}
	if completedAt.Valid {
		contract.CompletedAt = &completedAt.Time
	}

	return contract, nil
}

// scanContracts scanne plusieurs lignes
func (r *CreditContractRepositoryInfrastructure) scanContracts(ctx context.Context, query string, args ...interface{}) ([]*entity.CreditContract, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query credit contracts: %w", err)
	}
	defer rows.Close()

	var contracts []*entity.CreditContract
	for rows.Next() {
		contract := &entity.CreditContract{}
		var downPaymentPaidAt, completedAt sql.NullTime

		err := rows.Scan(
			&contract.ID,
			&contract.ApplicationID,
			&contract.CustomerID,
			&contract.ProductID,
			&contract.ShopID,
			&contract.ProductPriceCents,
			&contract.DownPaymentCents,
			&contract.FinancedAmountCents,
			&contract.InterestAmountCents,
			&contract.TotalAmountCents,
			&contract.MonthlyPaymentCents,
			&contract.DurationMonths,
			&contract.StartDate,
			&contract.EndDate,
			&contract.Status,
			&downPaymentPaidAt,
			&completedAt,
			&contract.CreatedAt,
			&contract.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if downPaymentPaidAt.Valid {
			contract.DownPaymentPaidAt = &downPaymentPaidAt.Time
		}
		if completedAt.Valid {
			contract.CompletedAt = &completedAt.Time
		}

		contracts = append(contracts, contract)
	}

	if contracts == nil {
		contracts = []*entity.CreditContract{}
	}
	return contracts, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée un nouveau contrat
func (r *CreditContractRepositoryInfrastructure) Create(ctx context.Context, contract *entity.CreditContract) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le contrat appartient à la boutique
	if contract.ShopID != shopID {
		return fmt.Errorf("access denied: contract shop_id does not match tenant shop")
	}

	query := `
		INSERT INTO credit_contracts (
			application_id, customer_id, product_id, shop_id,
			product_price_cents, down_payment_cents,
			financed_amount_cents, interest_amount_cents,
			total_amount_cents, monthly_payment_cents,
			duration_months, start_date, end_date,
			status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		contract.ApplicationID,
		contract.CustomerID,
		contract.ProductID,
		contract.ShopID,
		contract.ProductPriceCents,
		contract.DownPaymentCents,
		contract.FinancedAmountCents,
		contract.InterestAmountCents,
		contract.TotalAmountCents,
		contract.MonthlyPaymentCents,
		contract.DurationMonths,
		contract.StartDate,
		contract.EndDate,
		contract.Status,
	).Scan(&contract.ID, &contract.CreatedAt, &contract.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create credit contract: %w", err)
	}

	return nil
}

// FindByID trouve un contrat par ID
func (r *CreditContractRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.CreditContract, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanContract(r.queryRowContext(ctx, query, id, shopID))
}

// FindByApplicationID trouve un contrat par demande
func (r *CreditContractRepositoryInfrastructure) FindByApplicationID(ctx context.Context, applicationID string) (*entity.CreditContract, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE application_id = $1 AND shop_id = $2
	`

	return r.scanContract(r.queryRowContext(ctx, query, applicationID, shopID))
}

// FindByCustomerID retourne les contrats d'un client
func (r *CreditContractRepositoryInfrastructure) FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditContract, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE customer_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanContracts(ctx, query, customerID, shopID)
}

// FindActiveByCustomerID retourne les contrats actifs d'un client
func (r *CreditContractRepositoryInfrastructure) FindActiveByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditContract, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE customer_id = $1 AND shop_id = $2 AND status = 'active'
		ORDER BY created_at DESC
	`

	return r.scanContracts(ctx, query, customerID, shopID)
}

// FindByShopID retourne les contrats d'une boutique
func (r *CreditContractRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.CreditContract, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query contracts of another shop")
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	return r.scanContracts(ctx, query, shopID)
}

// FindActiveByShopID retourne les contrats actifs d'une boutique
func (r *CreditContractRepositoryInfrastructure) FindActiveByShopID(ctx context.Context, shopID string) ([]*entity.CreditContract, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query contracts of another shop")
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE shop_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`

	return r.scanContracts(ctx, query, shopID)
}

// CountActiveByCustomerID compte les contrats actifs d'un client
func (r *CreditContractRepositoryInfrastructure) CountActiveByCustomerID(ctx context.Context, customerID string) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM credit_contracts
		WHERE customer_id = $1 AND shop_id = $2 AND status = 'active'
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active contracts: %w", err)
	}

	return count, nil
}

// Update met à jour un contrat
func (r *CreditContractRepositoryInfrastructure) Update(ctx context.Context, contract *entity.CreditContract) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le contrat appartient à la boutique
	var contractShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM credit_contracts WHERE id = $1`, contract.ID).Scan(&contractShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("credit contract not found")
		}
		return fmt.Errorf("failed to verify contract: %w", err)
	}
	if contractShopID != shopID {
		return fmt.Errorf("access denied: contract does not belong to tenant shop")
	}

	query := `
		UPDATE credit_contracts
		SET status = $2,
		    down_payment_paid_at = $3,
		    completed_at = $4,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.queryRowContext(ctx, query,
		contract.ID,
		contract.Status,
		contract.DownPaymentPaidAt,
		contract.CompletedAt,
	).Scan(&contract.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update credit contract: %w", err)
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si un contrat existe
func (r *CreditContractRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_contracts
		WHERE id = $1 AND shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check contract existence: %w", err)
	}

	return count > 0, nil
}

// HasActiveContract vérifie si un client a un contrat actif
func (r *CreditContractRepositoryInfrastructure) HasActiveContract(ctx context.Context, customerID string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_contracts
		WHERE customer_id = $1 AND shop_id = $2 AND status = 'active'
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check active contract: %w", err)
	}

	return count > 0, nil
}

// FindExpiringSoon retourne les contrats qui expirent bientôt
func (r *CreditContractRepositoryInfrastructure) FindExpiringSoon(ctx context.Context, shopID string, days int) ([]*entity.CreditContract, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query contracts of another shop")
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE shop_id = $1 
		  AND status = 'active'
		  AND end_date <= NOW() + INTERVAL '1 day' * $2
		  AND end_date > NOW()
		ORDER BY end_date ASC
	`

	return r.scanContracts(ctx, query, shopID, days)
}

// SumActiveAmountByShopID somme des montants actifs pour une boutique
func (r *CreditContractRepositoryInfrastructure) SumActiveAmountByShopID(ctx context.Context, shopID string) (int64, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum contracts of another shop")
	}

	query := `
		SELECT COALESCE(SUM(total_amount_cents), 0)
		FROM credit_contracts
		WHERE shop_id = $1 AND status = 'active'
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum active contracts: %w", err)
	}

	return total, nil
}

// CountByStatusByShopID compte les contrats par statut pour une boutique
func (r *CreditContractRepositoryInfrastructure) CountByStatusByShopID(ctx context.Context, shopID string, status entity.CreditContractStatus) (int, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count contracts of another shop")
	}

	query := `
		SELECT COUNT(*) FROM credit_contracts
		WHERE shop_id = $1 AND status = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count contracts: %w", err)
	}

	return count, nil
}

// FindDefaultedByShopID retourne les contrats en défaut d'une boutique
func (r *CreditContractRepositoryInfrastructure) FindDefaultedByShopID(ctx context.Context, shopID string) ([]*entity.CreditContract, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query contracts of another shop")
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE shop_id = $1 AND status = 'defaulted'
		ORDER BY created_at DESC
	`

	return r.scanContracts(ctx, query, shopID)
}

// FindCompletedByShopID retourne les contrats terminés d'une boutique
func (r *CreditContractRepositoryInfrastructure) FindCompletedByShopID(ctx context.Context, shopID string) ([]*entity.CreditContract, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query contracts of another shop")
	}

	query := `
		SELECT id, application_id, customer_id, product_id, shop_id,
		       product_price_cents, down_payment_cents,
		       financed_amount_cents, interest_amount_cents,
		       total_amount_cents, monthly_payment_cents,
		       duration_months, start_date, end_date,
		       status, down_payment_paid_at, completed_at,
		       created_at, updated_at
		FROM credit_contracts
		WHERE shop_id = $1 AND status = 'completed'
		ORDER BY completed_at DESC
	`

	return r.scanContracts(ctx, query, shopID)
}

// GetMerchantCreditStats retourne les statistiques de crédit pour un marchand
func (r *CreditContractRepositoryInfrastructure) GetMerchantCreditStats(ctx context.Context, shopID string) (*repository.MerchantCreditStats, error) {
	query := `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'active') as active_contracts,
			COALESCE(SUM(financed_amount_cents) FILTER (WHERE status = 'active'), 0) as total_financed,
			COALESCE(SUM(total_amount_cents - down_payment_cents - 
				(SELECT COALESCE(SUM(amount_cents), 0) 
				 FROM credit_installments ci 
				 WHERE ci.contract_id = cc.id AND ci.status = 'paid')
			) FILTER (WHERE status = 'active'), 0) as total_outstanding
		FROM credit_contracts cc
		WHERE shop_id = $1
	`

	var stats repository.MerchantCreditStats
	err := r.queryRowContext(ctx, query, shopID).Scan(
		&stats.ActiveContractsCount,
		&stats.TotalFinancedCents,
		&stats.TotalOutstandingCents,
	)

	if err != nil {
		return nil, fmt.Errorf("query credit stats: %w", err)
	}

	return &stats, nil
}
