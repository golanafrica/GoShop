package escrow

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// EscrowAccountRepositoryInfrastructure implémente repository.EscrowAccountRepository
type EscrowAccountRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewEscrowAccountRepositoryInfrastructure crée une nouvelle instance
func NewEscrowAccountRepositoryInfrastructure(db *sql.DB) repository.EscrowAccountRepository {
	return &EscrowAccountRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *EscrowAccountRepositoryInfrastructure) WithTX(tx repository.Tx) repository.EscrowAccountRepository {
	return &EscrowAccountRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *EscrowAccountRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *EscrowAccountRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *EscrowAccountRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *EscrowAccountRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanAccount scanne une ligne dans une entité EscrowAccount
func (r *EscrowAccountRepositoryInfrastructure) scanAccount(row *sql.Row) (*entity.EscrowAccount, error) {
	account := &entity.EscrowAccount{}
	var orderID, creditContractID, tontineGroupID sql.NullString
	var fundsReleasedAt sql.NullTime

	err := row.Scan(
		&account.ID,
		&orderID,
		&creditContractID,
		&tontineGroupID,
		&account.SourceType,
		&account.TotalAmountCents,
		&account.ReleasedAmountCents,
		&account.CommissionCents,
		&account.Status,
		&account.FundsHeldAt,
		&fundsReleasedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("escrow account not found")
		}
		return nil, fmt.Errorf("failed to scan escrow account: %w", err)
	}

	if orderID.Valid {
		account.OrderID = &orderID.String
	}
	if creditContractID.Valid {
		account.CreditContractID = &creditContractID.String
	}
	if tontineGroupID.Valid {
		account.TontineGroupID = &tontineGroupID.String
	}
	if fundsReleasedAt.Valid {
		account.FundsReleasedAt = &fundsReleasedAt.Time
	}

	return account, nil
}

// scanAccounts scanne plusieurs lignes
func (r *EscrowAccountRepositoryInfrastructure) scanAccounts(ctx context.Context, query string, args ...interface{}) ([]*entity.EscrowAccount, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query escrow accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*entity.EscrowAccount
	for rows.Next() {
		account := &entity.EscrowAccount{}
		var orderID, creditContractID, tontineGroupID sql.NullString
		var fundsReleasedAt sql.NullTime

		err := rows.Scan(
			&account.ID,
			&orderID,
			&creditContractID,
			&tontineGroupID,
			&account.SourceType,
			&account.TotalAmountCents,
			&account.ReleasedAmountCents,
			&account.CommissionCents,
			&account.Status,
			&account.FundsHeldAt,
			&fundsReleasedAt,
			&account.CreatedAt,
			&account.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if orderID.Valid {
			account.OrderID = &orderID.String
		}
		if creditContractID.Valid {
			account.CreditContractID = &creditContractID.String
		}
		if tontineGroupID.Valid {
			account.TontineGroupID = &tontineGroupID.String
		}
		if fundsReleasedAt.Valid {
			account.FundsReleasedAt = &fundsReleasedAt.Time
		}

		accounts = append(accounts, account)
	}

	if accounts == nil {
		accounts = []*entity.EscrowAccount{}
	}
	return accounts, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée un nouveau compte séquestre
func (r *EscrowAccountRepositoryInfrastructure) Create(ctx context.Context, account *entity.EscrowAccount) error {
	// Valider le compte
	if err := account.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO escrow_accounts (
			order_id, credit_contract_id, tontine_group_id,
			source_type, total_amount_cents,
			released_amount_cents, commission_cents,
			status, funds_held_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err := r.queryRowContext(ctx, query,
		account.OrderID,
		account.CreditContractID,
		account.TontineGroupID,
		account.SourceType,
		account.TotalAmountCents,
		account.ReleasedAmountCents,
		account.CommissionCents,
		account.Status,
		account.FundsHeldAt,
	).Scan(&account.ID, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create escrow account: %w", err)
	}

	return nil
}

// FindByID trouve un compte séquestre par ID
func (r *EscrowAccountRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.EscrowAccount, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_group_id,
		       source_type, total_amount_cents,
		       released_amount_cents, commission_cents,
		       status, funds_held_at, funds_released_at,
		       created_at, updated_at
		FROM escrow_accounts
		WHERE id = $1
	`

	return r.scanAccount(r.queryRowContext(ctx, query, id))
}

// FindByOrderID trouve un compte séquestre par commande
func (r *EscrowAccountRepositoryInfrastructure) FindByOrderID(ctx context.Context, orderID string) (*entity.EscrowAccount, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_group_id,
		       source_type, total_amount_cents,
		       released_amount_cents, commission_cents,
		       status, funds_held_at, funds_released_at,
		       created_at, updated_at
		FROM escrow_accounts
		WHERE order_id = $1
	`

	return r.scanAccount(r.queryRowContext(ctx, query, orderID))
}

// FindByCreditContractID trouve un compte séquestre par contrat de crédit
func (r *EscrowAccountRepositoryInfrastructure) FindByCreditContractID(ctx context.Context, contractID string) (*entity.EscrowAccount, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_group_id,
		       source_type, total_amount_cents,
		       released_amount_cents, commission_cents,
		       status, funds_held_at, funds_released_at,
		       created_at, updated_at
		FROM escrow_accounts
		WHERE credit_contract_id = $1
	`

	return r.scanAccount(r.queryRowContext(ctx, query, contractID))
}

// FindByTontineGroupID trouve un compte séquestre par groupe tontine
func (r *EscrowAccountRepositoryInfrastructure) FindByTontineGroupID(ctx context.Context, groupID string) (*entity.EscrowAccount, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_group_id,
		       source_type, total_amount_cents,
		       released_amount_cents, commission_cents,
		       status, funds_held_at, funds_released_at,
		       created_at, updated_at
		FROM escrow_accounts
		WHERE tontine_group_id = $1
	`

	return r.scanAccount(r.queryRowContext(ctx, query, groupID))
}

// FindByReferenceID trouve un compte séquestre par référence (polymorphique)
func (r *EscrowAccountRepositoryInfrastructure) FindByReferenceID(ctx context.Context, sourceType entity.EscrowSourceType, referenceID string) (*entity.EscrowAccount, error) {
	var query string
	switch sourceType {
	case entity.EscrowSourceOrder:
		query = `
			SELECT id, order_id, credit_contract_id, tontine_group_id,
			       source_type, total_amount_cents,
			       released_amount_cents, commission_cents,
			       status, funds_held_at, funds_released_at,
			       created_at, updated_at
			FROM escrow_accounts
			WHERE order_id = $1 AND source_type = $2
		`
	case entity.EscrowSourceCreditContract:
		query = `
			SELECT id, order_id, credit_contract_id, tontine_group_id,
			       source_type, total_amount_cents,
			       released_amount_cents, commission_cents,
			       status, funds_held_at, funds_released_at,
			       created_at, updated_at
			FROM escrow_accounts
			WHERE credit_contract_id = $1 AND source_type = $2
		`
	case entity.EscrowSourceTontineGroup:
		query = `
			SELECT id, order_id, credit_contract_id, tontine_group_id,
			       source_type, total_amount_cents,
			       released_amount_cents, commission_cents,
			       status, funds_held_at, funds_released_at,
			       created_at, updated_at
			FROM escrow_accounts
			WHERE tontine_group_id = $1 AND source_type = $2
		`
	default:
		return nil, fmt.Errorf("invalid source type: %s", sourceType)
	}

	return r.scanAccount(r.queryRowContext(ctx, query, referenceID, sourceType))
}

// FindByStatus retourne les comptes séquestres par statut
func (r *EscrowAccountRepositoryInfrastructure) FindByStatus(ctx context.Context, status entity.EscrowAccountStatus) ([]*entity.EscrowAccount, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_group_id,
		       source_type, total_amount_cents,
		       released_amount_cents, commission_cents,
		       status, funds_held_at, funds_released_at,
		       created_at, updated_at
		FROM escrow_accounts
		WHERE status = $1
		ORDER BY created_at DESC
	`

	return r.scanAccounts(ctx, query, status)
}

// FindHeldByShopID retourne les comptes séquestres bloqués d'une boutique
func (r *EscrowAccountRepositoryInfrastructure) FindHeldByShopID(ctx context.Context, shopID string) ([]*entity.EscrowAccount, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query escrow accounts of another shop")
	}

	query := `
		SELECT ea.id, ea.order_id, ea.credit_contract_id, ea.tontine_group_id,
		       ea.source_type, ea.total_amount_cents,
		       ea.released_amount_cents, ea.commission_cents,
		       ea.status, ea.funds_held_at, ea.funds_released_at,
		       ea.created_at, ea.updated_at
		FROM escrow_accounts ea
		LEFT JOIN orders o ON o.id = ea.order_id
		LEFT JOIN credit_contracts cc ON cc.id = ea.credit_contract_id
		LEFT JOIN tontine_groups tg ON tg.id = ea.tontine_group_id
		WHERE ea.status = 'funds_held'
		  AND (
		    (ea.source_type = 'order' AND o.shop_id = $1) OR
		    (ea.source_type = 'credit_contract' AND cc.shop_id = $1) OR
		    (ea.source_type = 'tontine_group' AND tg.shop_id = $1)
		  )
		ORDER BY ea.created_at DESC
	`

	return r.scanAccounts(ctx, query, shopID)
}

// FindDisputedByShopID retourne les comptes séquestres en litige d'une boutique
func (r *EscrowAccountRepositoryInfrastructure) FindDisputedByShopID(ctx context.Context, shopID string) ([]*entity.EscrowAccount, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query escrow accounts of another shop")
	}

	query := `
		SELECT ea.id, ea.order_id, ea.credit_contract_id, ea.tontine_group_id,
		       ea.source_type, ea.total_amount_cents,
		       ea.released_amount_cents, ea.commission_cents,
		       ea.status, ea.funds_held_at, ea.funds_released_at,
		       ea.created_at, ea.updated_at
		FROM escrow_accounts ea
		LEFT JOIN orders o ON o.id = ea.order_id
		LEFT JOIN credit_contracts cc ON cc.id = ea.credit_contract_id
		LEFT JOIN tontine_groups tg ON tg.id = ea.tontine_group_id
		WHERE ea.status = 'disputed'
		  AND (
		    (ea.source_type = 'order' AND o.shop_id = $1) OR
		    (ea.source_type = 'credit_contract' AND cc.shop_id = $1) OR
		    (ea.source_type = 'tontine_group' AND tg.shop_id = $1)
		  )
		ORDER BY ea.created_at DESC
	`

	return r.scanAccounts(ctx, query, shopID)
}

// SumHeldAmountByShopID somme des montants bloqués par boutique
func (r *EscrowAccountRepositoryInfrastructure) SumHeldAmountByShopID(ctx context.Context, shopID string) (int64, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum escrow accounts of another shop")
	}

	query := `
		SELECT COALESCE(SUM(ea.total_amount_cents - ea.released_amount_cents - ea.commission_cents), 0)
		FROM escrow_accounts ea
		LEFT JOIN orders o ON o.id = ea.order_id
		LEFT JOIN credit_contracts cc ON cc.id = ea.credit_contract_id
		LEFT JOIN tontine_groups tg ON tg.id = ea.tontine_group_id
		WHERE ea.status IN ('funds_held', 'partial_release', 'disputed')
		  AND (
		    (ea.source_type = 'order' AND o.shop_id = $1) OR
		    (ea.source_type = 'credit_contract' AND cc.shop_id = $1) OR
		    (ea.source_type = 'tontine_group' AND tg.shop_id = $1)
		  )
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum held amount: %w", err)
	}

	return total, nil
}

// SumTotalHeldAmount somme totale des montants bloqués (toutes boutiques)
func (r *EscrowAccountRepositoryInfrastructure) SumTotalHeldAmount(ctx context.Context) (int64, error) {
	query := `
		SELECT COALESCE(SUM(total_amount_cents - released_amount_cents - commission_cents), 0)
		FROM escrow_accounts
		WHERE status IN ('funds_held', 'partial_release', 'disputed')
	`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total held amount: %w", err)
	}

	return total, nil
}

// Update met à jour un compte séquestre
func (r *EscrowAccountRepositoryInfrastructure) Update(ctx context.Context, account *entity.EscrowAccount) error {
	// Valider le compte
	if err := account.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE escrow_accounts
		SET total_amount_cents = $2,
		    released_amount_cents = $3,
		    commission_cents = $4,
		    status = $5,
		    funds_released_at = $6,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.queryRowContext(ctx, query,
		account.ID,
		account.TotalAmountCents,
		account.ReleasedAmountCents,
		account.CommissionCents,
		account.Status,
		account.FundsReleasedAt,
	).Scan(&account.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update escrow account: %w", err)
	}

	return nil
}

// UpdateStatus met à jour uniquement le statut
func (r *EscrowAccountRepositoryInfrastructure) UpdateStatus(ctx context.Context, id string, status entity.EscrowAccountStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	var query string
	var args []interface{}

	if status == entity.EscrowAccountFullyReleased || status == entity.EscrowAccountRefunded {
		// Définir funds_released_at
		query = `
			UPDATE escrow_accounts
			SET status = $2,
			    funds_released_at = NOW(),
			    updated_at = NOW()
			WHERE id = $1
		`
		args = []interface{}{id, status}
	} else {
		query = `
			UPDATE escrow_accounts
			SET status = $2,
			    updated_at = NOW()
			WHERE id = $1
		`
		args = []interface{}{id, status}
	}

	result, err := r.execContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update escrow status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("escrow account not found")
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// ClaimRelease — UPDATE conditionnel anti race multi-instance
// fromStatus typique : funds_held (auto-release) ou disputed (merchant_wins)
func (r *EscrowAccountRepositoryInfrastructure) ClaimRelease(
	ctx context.Context,
	escrowID string,
	fromStatus entity.EscrowAccountStatus,
	releasedAmountCents int64,
) (bool, error) {
	if escrowID == "" {
		return false, fmt.Errorf("escrow id is required")
	}
	if !fromStatus.IsValid() {
		return false, fmt.Errorf("invalid fromStatus: %s", fromStatus)
	}

	query := `
		UPDATE escrow_accounts
		SET status = $3,
		    released_amount_cents = $4,
		    funds_released_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = $2
	`
	res, err := r.execContext(ctx, query,
		escrowID,
		fromStatus,
		entity.EscrowAccountFullyReleased, // "released"
		releasedAmountCents,
	)
	if err != nil {
		return false, fmt.Errorf("claim release failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim release rows affected: %w", err)
	}
	return n == 1, nil
}

// Exists vérifie si un compte séquestre existe
func (r *EscrowAccountRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT COUNT(*) FROM escrow_accounts WHERE id = $1`

	var count int
	err := r.queryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check escrow existence: %w", err)
	}

	return count > 0, nil
}

// HasEscrowForOrder vérifie si une commande a un compte séquestre
func (r *EscrowAccountRepositoryInfrastructure) HasEscrowForOrder(ctx context.Context, orderID string) (bool, error) {
	query := `SELECT COUNT(*) FROM escrow_accounts WHERE order_id = $1`

	var count int
	err := r.queryRowContext(ctx, query, orderID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check escrow for order: %w", err)
	}

	return count > 0, nil
}

// CountByStatus compte les comptes séquestres par statut
func (r *EscrowAccountRepositoryInfrastructure) CountByStatus(ctx context.Context, status entity.EscrowAccountStatus) (int, error) {
	query := `SELECT COUNT(*) FROM escrow_accounts WHERE status = $1`

	var count int
	err := r.queryRowContext(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count escrow accounts: %w", err)
	}

	return count, nil
}

// CountBySourceTypeByStatus compte par type source et statut
func (r *EscrowAccountRepositoryInfrastructure) CountBySourceTypeByStatus(ctx context.Context, sourceType entity.EscrowSourceType, status entity.EscrowAccountStatus) (int, error) {
	query := `SELECT COUNT(*) FROM escrow_accounts WHERE source_type = $1 AND status = $2`

	var count int
	err := r.queryRowContext(ctx, query, sourceType, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count escrow accounts: %w", err)
	}

	return count, nil
}

// SumCommissionByStatusByShopID somme des commissions par statut pour une boutique
func (r *EscrowAccountRepositoryInfrastructure) SumCommissionByStatusByShopID(ctx context.Context, shopID string, status entity.EscrowAccountStatus) (int64, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query escrow accounts of another shop")
	}

	query := `
		SELECT COALESCE(SUM(ea.commission_cents), 0)
		FROM escrow_accounts ea
		LEFT JOIN orders o ON o.id = ea.order_id
		LEFT JOIN credit_contracts cc ON cc.id = ea.credit_contract_id
		LEFT JOIN tontine_groups tg ON tg.id = ea.tontine_group_id
		WHERE ea.status = $2
		  AND (
		    (ea.source_type = 'order' AND o.shop_id = $1) OR
		    (ea.source_type = 'credit_contract' AND cc.shop_id = $1) OR
		    (ea.source_type = 'tontine_group' AND tg.shop_id = $1)
		  )
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID, status).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum commissions: %w", err)
	}

	return total, nil
}

// FindOldHeldAccounts retourne les comptes séquestres bloqués depuis plus de X jours
func (r *EscrowAccountRepositoryInfrastructure) FindOldHeldAccounts(ctx context.Context, days int) ([]*entity.EscrowAccount, error) {
	query := `
		SELECT id, order_id, credit_contract_id, tontine_group_id,
		       source_type, total_amount_cents,
		       released_amount_cents, commission_cents,
		       status, funds_held_at, funds_released_at,
		       created_at, updated_at
		FROM escrow_accounts
		WHERE status = 'funds_held'
		  AND funds_held_at < NOW() - INTERVAL '1 day' * $1
		ORDER BY funds_held_at ASC
	`

	return r.scanAccounts(ctx, query, days)
}
