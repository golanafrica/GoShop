package wallet

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// ============================================================
// MERCHANT WALLET REPOSITORY
// ============================================================

type MerchantWalletRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewMerchantWalletRepositoryInfrastructure(db *sql.DB) repository.MerchantWalletRepository {
	return &MerchantWalletRepositoryInfrastructure{db: db}
}

func (r *MerchantWalletRepositoryInfrastructure) WithTX(tx repository.Tx) repository.MerchantWalletRepository {
	return &MerchantWalletRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *MerchantWalletRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *MerchantWalletRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *MerchantWalletRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *MerchantWalletRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// Colonnes wallet (Phase 2 : + held_cents)
const merchantWalletSelectCols = `
	shop_id, balance_cents, held_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
	max_negative_balance_cents,
	total_sales_cents, total_commissions_cents, total_payouts_cents,
	created_at, updated_at
`

func (r *MerchantWalletRepositoryInfrastructure) scanWallet(row *sql.Row) (*entity.MerchantWallet, error) {
	wallet := &entity.MerchantWallet{}
	var frozenAt, frozenUntil sql.NullTime
	var frozenReason sql.NullString

	err := row.Scan(
		&wallet.ShopID,
		&wallet.BalanceCents,
		&wallet.HeldCents,
		&wallet.IsFrozen,
		&frozenAt,
		&frozenReason,
		&frozenUntil,
		&wallet.MaxNegativeBalanceCents,
		&wallet.TotalSalesCents,
		&wallet.TotalCommissionsCents,
		&wallet.TotalPayoutsCents,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("merchant wallet not found")
		}
		return nil, fmt.Errorf("failed to scan merchant wallet: %w", err)
	}

	if frozenAt.Valid {
		wallet.FrozenAt = &frozenAt.Time
	}
	if frozenReason.Valid {
		wallet.FrozenReason = &frozenReason.String
	}
	if frozenUntil.Valid {
		wallet.FrozenUntil = &frozenUntil.Time
	}
	return wallet, nil
}

func (r *MerchantWalletRepositoryInfrastructure) scanWallets(ctx context.Context, query string, args ...interface{}) ([]*entity.MerchantWallet, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query merchant wallets: %w", err)
	}
	defer rows.Close()

	var wallets []*entity.MerchantWallet
	for rows.Next() {
		wallet := &entity.MerchantWallet{}
		var frozenAt, frozenUntil sql.NullTime
		var frozenReason sql.NullString

		err := rows.Scan(
			&wallet.ShopID,
			&wallet.BalanceCents,
			&wallet.HeldCents,
			&wallet.IsFrozen,
			&frozenAt,
			&frozenReason,
			&frozenUntil,
			&wallet.MaxNegativeBalanceCents,
			&wallet.TotalSalesCents,
			&wallet.TotalCommissionsCents,
			&wallet.TotalPayoutsCents,
			&wallet.CreatedAt,
			&wallet.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if frozenAt.Valid {
			wallet.FrozenAt = &frozenAt.Time
		}
		if frozenReason.Valid {
			wallet.FrozenReason = &frozenReason.String
		}
		if frozenUntil.Valid {
			wallet.FrozenUntil = &frozenUntil.Time
		}
		wallets = append(wallets, wallet)
	}
	if wallets == nil {
		wallets = []*entity.MerchantWallet{}
	}
	return wallets, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

func (r *MerchantWalletRepositoryInfrastructure) Create(ctx context.Context, wallet *entity.MerchantWallet) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if wallet.ShopID != shopID {
		return fmt.Errorf("access denied: wallet shop_id does not match tenant shop")
	}
	if err := wallet.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO merchant_wallets (
			shop_id, balance_cents, held_cents,
			max_negative_balance_cents,
			total_sales_cents, total_commissions_cents, total_payouts_cents,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at
	`
	err = r.queryRowContext(ctx, query,
		wallet.ShopID,
		wallet.BalanceCents,
		wallet.HeldCents,
		wallet.MaxNegativeBalanceCents,
		wallet.TotalSalesCents,
		wallet.TotalCommissionsCents,
		wallet.TotalPayoutsCents,
	).Scan(&wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create merchant wallet: %w", err)
	}
	return nil
}

func (r *MerchantWalletRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query wallet of another shop")
	}
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE shop_id = $1`
	return r.scanWallet(r.queryRowContext(ctx, query, shopID))
}

func (r *MerchantWalletRepositoryInfrastructure) FindByShopIDForUpdate(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query wallet of another shop")
	}
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE shop_id = $1 FOR UPDATE`
	return r.scanWallet(r.queryRowContext(ctx, query, shopID))
}

func (r *MerchantWalletRepositoryInfrastructure) FindAll(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets ORDER BY created_at DESC`
	return r.scanWallets(ctx, query)
}

func (r *MerchantWalletRepositoryInfrastructure) FindFrozen(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE is_frozen = true ORDER BY frozen_at ASC`
	return r.scanWallets(ctx, query)
}

func (r *MerchantWalletRepositoryInfrastructure) FindFrozenByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query wallet of another shop")
	}
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE shop_id = $1 AND is_frozen = true`
	return r.scanWallet(r.queryRowContext(ctx, query, shopID))
}

func (r *MerchantWalletRepositoryInfrastructure) FindNegativeBalance(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE balance_cents < 0 ORDER BY balance_cents ASC`
	return r.scanWallets(ctx, query)
}

func (r *MerchantWalletRepositoryInfrastructure) FindPositiveBalance(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE balance_cents > 0 ORDER BY balance_cents DESC`
	return r.scanWallets(ctx, query)
}

func (r *MerchantWalletRepositoryInfrastructure) FindBelowThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE balance_cents < $1 ORDER BY balance_cents ASC`
	return r.scanWallets(ctx, query, thresholdCents)
}

func (r *MerchantWalletRepositoryInfrastructure) FindAboveThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE balance_cents > $1 ORDER BY balance_cents DESC`
	return r.scanWallets(ctx, query, thresholdCents)
}

func (r *MerchantWalletRepositoryInfrastructure) FindGracePeriodExpiringSoon(ctx context.Context, daysRemaining int) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT ` + merchantWalletSelectCols + `
		FROM merchant_wallets
		WHERE is_frozen = true
		  AND frozen_until IS NOT NULL
		  AND frozen_until <= NOW() + INTERVAL '1 day' * $1
		  AND frozen_until > NOW()
		ORDER BY frozen_until ASC
	`
	return r.scanWallets(ctx, query, daysRemaining)
}

func (r *MerchantWalletRepositoryInfrastructure) FindGracePeriodExpired(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT ` + merchantWalletSelectCols + `
		FROM merchant_wallets
		WHERE is_frozen = true
		  AND frozen_until IS NOT NULL
		  AND frozen_until <= NOW()
		ORDER BY frozen_until ASC
	`
	return r.scanWallets(ctx, query)
}

func (r *MerchantWalletRepositoryInfrastructure) SumTotalBalance(ctx context.Context) (int64, error) {
	var total int64
	err := r.queryRowContext(ctx, `SELECT COALESCE(SUM(balance_cents), 0) FROM merchant_wallets`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total balance: %w", err)
	}
	return total, nil
}

func (r *MerchantWalletRepositoryInfrastructure) SumFrozenBalance(ctx context.Context) (int64, error) {
	var total int64
	err := r.queryRowContext(ctx, `SELECT COALESCE(SUM(balance_cents), 0) FROM merchant_wallets WHERE is_frozen = true`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum frozen balance: %w", err)
	}
	return total, nil
}

func (r *MerchantWalletRepositoryInfrastructure) SumNegativeBalance(ctx context.Context) (int64, error) {
	var total int64
	err := r.queryRowContext(ctx, `SELECT COALESCE(SUM(balance_cents), 0) FROM merchant_wallets WHERE balance_cents < 0`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum negative balance: %w", err)
	}
	return total, nil
}

func (r *MerchantWalletRepositoryInfrastructure) CountFrozen(ctx context.Context) (int, error) {
	var count int
	err := r.queryRowContext(ctx, `SELECT COUNT(*) FROM merchant_wallets WHERE is_frozen = true`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count frozen wallets: %w", err)
	}
	return count, nil
}

func (r *MerchantWalletRepositoryInfrastructure) CountNegativeBalance(ctx context.Context) (int, error) {
	var count int
	err := r.queryRowContext(ctx, `SELECT COUNT(*) FROM merchant_wallets WHERE balance_cents < 0`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count negative balance wallets: %w", err)
	}
	return count, nil
}

func (r *MerchantWalletRepositoryInfrastructure) Update(ctx context.Context, wallet *entity.MerchantWallet) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if wallet.ShopID != shopID {
		return fmt.Errorf("access denied: wallet does not belong to tenant shop")
	}
	if err := wallet.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE merchant_wallets
		SET balance_cents = $2,
		    held_cents = $3,
		    is_frozen = $4,
		    frozen_at = $5,
		    frozen_reason = $6,
		    frozen_until = $7,
		    max_negative_balance_cents = $8,
		    total_sales_cents = $9,
		    total_commissions_cents = $10,
		    total_payouts_cents = $11,
		    updated_at = NOW()
		WHERE shop_id = $1
		RETURNING updated_at
	`
	err = r.queryRowContext(ctx, query,
		wallet.ShopID,
		wallet.BalanceCents,
		wallet.HeldCents,
		wallet.IsFrozen,
		wallet.FrozenAt,
		wallet.FrozenReason,
		wallet.FrozenUntil,
		wallet.MaxNegativeBalanceCents,
		wallet.TotalSalesCents,
		wallet.TotalCommissionsCents,
		wallet.TotalPayoutsCents,
	).Scan(&wallet.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update merchant wallet: %w", err)
	}
	return nil
}

func (r *MerchantWalletRepositoryInfrastructure) UpdateBalance(ctx context.Context, shopID string, balanceCents int64) error {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if shopID != currentShopID {
		return fmt.Errorf("access denied: cannot update wallet of another shop")
	}
	result, err := r.execContext(ctx, `
		UPDATE merchant_wallets
		SET balance_cents = $2, updated_at = NOW()
		WHERE shop_id = $1
	`, shopID, balanceCents)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("merchant wallet not found")
	}
	return nil
}

// UpdateHeld met à jour uniquement held_cents (Phase 2)
func (r *MerchantWalletRepositoryInfrastructure) UpdateHeld(ctx context.Context, shopID string, heldCents int64) error {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if shopID != currentShopID {
		return fmt.Errorf("access denied: cannot update wallet of another shop")
	}
	if heldCents < 0 {
		return fmt.Errorf("held_cents cannot be negative")
	}
	result, err := r.execContext(ctx, `
		UPDATE merchant_wallets
		SET held_cents = $2, updated_at = NOW()
		WHERE shop_id = $1
	`, shopID, heldCents)
	if err != nil {
		return fmt.Errorf("failed to update held_cents: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("merchant wallet not found")
	}
	return nil
}

func (r *MerchantWalletRepositoryInfrastructure) Freeze(ctx context.Context, shopID string, reason string, details string) error {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if shopID != currentShopID {
		return fmt.Errorf("access denied: cannot freeze wallet of another shop")
	}
	_ = details
	now := time.Now().UTC()
	gracePeriodEnds := now.AddDate(0, 0, entity.DefaultGracePeriodDays)
	result, err := r.execContext(ctx, `
		UPDATE merchant_wallets
		SET is_frozen = true, frozen_at = $2, frozen_reason = $3, frozen_until = $4, updated_at = NOW()
		WHERE shop_id = $1 AND is_frozen = false
	`, shopID, now, reason, gracePeriodEnds)
	if err != nil {
		return fmt.Errorf("failed to freeze wallet: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("wallet not found or already frozen")
	}
	return nil
}

func (r *MerchantWalletRepositoryInfrastructure) Unfreeze(ctx context.Context, shopID string) error {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if shopID != currentShopID {
		return fmt.Errorf("access denied: cannot unfreeze wallet of another shop")
	}
	result, err := r.execContext(ctx, `
		UPDATE merchant_wallets
		SET is_frozen = false, frozen_at = NULL, frozen_reason = NULL, frozen_until = NULL, updated_at = NOW()
		WHERE shop_id = $1 AND is_frozen = true
	`, shopID)
	if err != nil {
		return fmt.Errorf("failed to unfreeze wallet: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("wallet not found or not frozen")
	}
	return nil
}

func (r *MerchantWalletRepositoryInfrastructure) UpdateStats(ctx context.Context, wallet *entity.MerchantWallet) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if wallet.ShopID != shopID {
		return fmt.Errorf("access denied: wallet does not belong to tenant shop")
	}
	result, err := r.execContext(ctx, `
		UPDATE merchant_wallets
		SET total_sales_cents = $2, total_commissions_cents = $3, total_payouts_cents = $4, updated_at = NOW()
		WHERE shop_id = $1
	`, wallet.ShopID, wallet.TotalSalesCents, wallet.TotalCommissionsCents, wallet.TotalPayoutsCents)
	if err != nil {
		return fmt.Errorf("failed to update stats: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("merchant wallet not found")
	}
	return nil
}

// ============================================================
// WALLET TRANSACTION REPOSITORY (inchangé structurellement)
// ============================================================

type WalletTransactionRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewWalletTransactionRepositoryInfrastructure(db *sql.DB) repository.WalletTransactionRepository {
	return &WalletTransactionRepositoryInfrastructure{db: db}
}

func (r *WalletTransactionRepositoryInfrastructure) WithTX(tx repository.Tx) repository.WalletTransactionRepository {
	return &WalletTransactionRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *WalletTransactionRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *WalletTransactionRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *WalletTransactionRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *WalletTransactionRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

func (r *WalletTransactionRepositoryInfrastructure) scanTransaction(row *sql.Row) (*entity.WalletTransaction, error) {
	txn := &entity.WalletTransaction{}
	var referenceType, referenceID, description sql.NullString
	err := row.Scan(
		&txn.ID, &txn.ShopID, &txn.TransactionType, &txn.AmountCents, &txn.BalanceAfterCents,
		&referenceType, &referenceID, &description, &txn.Status, &txn.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("wallet transaction not found")
		}
		return nil, fmt.Errorf("failed to scan wallet transaction: %w", err)
	}
	if referenceType.Valid {
		txn.ReferenceType = &referenceType.String
	}
	if referenceID.Valid {
		txn.ReferenceID = &referenceID.String
	}
	if description.Valid {
		txn.Description = &description.String
	}
	return txn, nil
}

func (r *WalletTransactionRepositoryInfrastructure) scanTransactions(ctx context.Context, query string, args ...interface{}) ([]*entity.WalletTransaction, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallet transactions: %w", err)
	}
	defer rows.Close()
	var txns []*entity.WalletTransaction
	for rows.Next() {
		txn := &entity.WalletTransaction{}
		var referenceType, referenceID, description sql.NullString
		err := rows.Scan(
			&txn.ID, &txn.ShopID, &txn.TransactionType, &txn.AmountCents, &txn.BalanceAfterCents,
			&referenceType, &referenceID, &description, &txn.Status, &txn.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if referenceType.Valid {
			txn.ReferenceType = &referenceType.String
		}
		if referenceID.Valid {
			txn.ReferenceID = &referenceID.String
		}
		if description.Valid {
			txn.Description = &description.String
		}
		txns = append(txns, txn)
	}
	if txns == nil {
		txns = []*entity.WalletTransaction{}
	}
	return txns, rows.Err()
}

func (r *WalletTransactionRepositoryInfrastructure) Create(ctx context.Context, txn *entity.WalletTransaction) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if txn.ShopID != shopID {
		return fmt.Errorf("access denied: transaction shop_id does not match tenant shop")
	}
	query := `
		INSERT INTO wallet_transactions (
			shop_id, transaction_type, amount_cents, balance_after_cents,
			reference_type, reference_id, description, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING id, created_at
	`
	err = r.queryRowContext(ctx, query,
		txn.ShopID, txn.TransactionType, txn.AmountCents, txn.BalanceAfterCents,
		txn.ReferenceType, txn.ReferenceID, txn.Description, txn.Status,
	).Scan(&txn.ID, &txn.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create wallet transaction: %w", err)
	}
	return nil
}

func (r *WalletTransactionRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE id = $1 AND shop_id = $2
	`
	return r.scanTransaction(r.queryRowContext(ctx, query, id, shopID))
}

func (r *WalletTransactionRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID)
}

func (r *WalletTransactionRepositoryInfrastructure) FindByShopIDPaginated(ctx context.Context, shopID string, limit, offset int) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	return r.scanTransactions(ctx, query, shopID, limit, offset)
}

func (r *WalletTransactionRepositoryInfrastructure) FindByType(ctx context.Context, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND transaction_type = $2 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID, txType)
}

func (r *WalletTransactionRepositoryInfrastructure) FindByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND transaction_type = $2 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID, txType)
}

func (r *WalletTransactionRepositoryInfrastructure) FindByStatus(ctx context.Context, status entity.WalletTransactionStatus) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND status = $2 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID, status)
}

func (r *WalletTransactionRepositoryInfrastructure) FindByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND created_at >= $2 AND created_at <= $3 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID, startDate, endDate)
}

func (r *WalletTransactionRepositoryInfrastructure) FindByReferenceID(ctx context.Context, refType string, refID string) (*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND reference_type = $2 AND reference_id = $3 LIMIT 1
	`
	return r.scanTransaction(r.queryRowContext(ctx, query, shopID, refType, refID))
}

func (r *WalletTransactionRepositoryInfrastructure) FindCreditsByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND amount_cents > 0 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID)
}

func (r *WalletTransactionRepositoryInfrastructure) FindDebitsByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND amount_cents < 0 ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID)
}

func (r *WalletTransactionRepositoryInfrastructure) FindPendingTransactions(ctx context.Context) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND status = 'pending' ORDER BY created_at ASC
	`
	return r.scanTransactions(ctx, query, shopID)
}

func (r *WalletTransactionRepositoryInfrastructure) FindFailedTransactions(ctx context.Context) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 AND status = 'failed' ORDER BY created_at DESC
	`
	return r.scanTransactions(ctx, query, shopID)
}

func (r *WalletTransactionRepositoryInfrastructure) FindRecent(ctx context.Context, shopID string, limit int) ([]*entity.WalletTransaction, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query transactions of another shop")
	}
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions WHERE shop_id = $1 ORDER BY created_at DESC LIMIT $2
	`
	return r.scanTransactions(ctx, query, shopID, limit)
}

func (r *WalletTransactionRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID string) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count transactions of another shop")
	}
	var count int
	err = r.queryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE shop_id = $1`, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}
	return count, nil
}

func (r *WalletTransactionRepositoryInfrastructure) CountByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count transactions of another shop")
	}
	var count int
	err = r.queryRowContext(ctx, `
		SELECT COUNT(*) FROM wallet_transactions WHERE shop_id = $1 AND transaction_type = $2
	`, shopID, txType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}
	return count, nil
}

func (r *WalletTransactionRepositoryInfrastructure) SumCreditsByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}
	var total int64
	err = r.queryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM wallet_transactions WHERE shop_id = $1 AND amount_cents > 0
	`, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum credits: %w", err)
	}
	return total, nil
}

func (r *WalletTransactionRepositoryInfrastructure) SumDebitsByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}
	var total int64
	err = r.queryRowContext(ctx, `
		SELECT COALESCE(SUM(ABS(amount_cents)), 0) FROM wallet_transactions WHERE shop_id = $1 AND amount_cents < 0
	`, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum debits: %w", err)
	}
	return total, nil
}

func (r *WalletTransactionRepositoryInfrastructure) SumCreditsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}
	var total int64
	err = r.queryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM wallet_transactions
		WHERE shop_id = $1 AND amount_cents > 0 AND created_at >= $2 AND created_at <= $3
	`, shopID, startDate, endDate).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum credits: %w", err)
	}
	return total, nil
}

func (r *WalletTransactionRepositoryInfrastructure) SumDebitsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}
	var total int64
	err = r.queryRowContext(ctx, `
		SELECT COALESCE(SUM(ABS(amount_cents)), 0) FROM wallet_transactions
		WHERE shop_id = $1 AND amount_cents < 0 AND created_at >= $2 AND created_at <= $3
	`, shopID, startDate, endDate).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum debits: %w", err)
	}
	return total, nil
}

func (r *WalletTransactionRepositoryInfrastructure) SumCreditsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}
	var total int64
	err = r.queryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2 AND amount_cents > 0
	`, shopID, txType).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum credits by type: %w", err)
	}
	return total, nil
}

func (r *WalletTransactionRepositoryInfrastructure) SumDebitsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}
	var total int64
	err = r.queryRowContext(ctx, `
		SELECT COALESCE(SUM(ABS(amount_cents)), 0) FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2 AND amount_cents < 0
	`, shopID, txType).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum debits by type: %w", err)
	}
	return total, nil
}

func (r *WalletTransactionRepositoryInfrastructure) UpdateStatus(ctx context.Context, id string, status entity.WalletTransactionStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid transaction status: %s", status)
	}
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	result, err := r.execContext(ctx, `
		UPDATE wallet_transactions SET status = $2 WHERE id = $1 AND shop_id = $3
	`, id, status, shopID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("wallet transaction not found")
	}
	return nil
}

// ============================================================
// ADMIN (bypass tenant)
// ============================================================

func (r *MerchantWalletRepositoryInfrastructure) FindByShopIDForUpdateAdmin(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	query := `SELECT ` + merchantWalletSelectCols + ` FROM merchant_wallets WHERE shop_id = $1 FOR UPDATE`
	return r.scanWallet(r.queryRowContext(ctx, query, shopID))
}

func (r *MerchantWalletRepositoryInfrastructure) UpdateAdmin(ctx context.Context, wallet *entity.MerchantWallet) error {
	if err := wallet.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	query := `
		UPDATE merchant_wallets
		SET balance_cents = $2,
		    held_cents = $3,
		    is_frozen = $4,
		    frozen_at = $5,
		    frozen_reason = $6,
		    frozen_until = $7,
		    max_negative_balance_cents = $8,
		    total_sales_cents = $9,
		    total_commissions_cents = $10,
		    total_payouts_cents = $11,
		    updated_at = NOW()
		WHERE shop_id = $1
		RETURNING updated_at
	`
	err := r.queryRowContext(ctx, query,
		wallet.ShopID,
		wallet.BalanceCents,
		wallet.HeldCents,
		wallet.IsFrozen,
		wallet.FrozenAt,
		wallet.FrozenReason,
		wallet.FrozenUntil,
		wallet.MaxNegativeBalanceCents,
		wallet.TotalSalesCents,
		wallet.TotalCommissionsCents,
		wallet.TotalPayoutsCents,
	).Scan(&wallet.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update merchant wallet: %w", err)
	}
	return nil
}

func (r *MerchantWalletRepositoryInfrastructure) CreateAdmin(ctx context.Context, wallet *entity.MerchantWallet) error {
	if err := wallet.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	query := `
		INSERT INTO merchant_wallets (
			shop_id, balance_cents, held_cents,
			max_negative_balance_cents,
			total_sales_cents, total_commissions_cents, total_payouts_cents,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at
	`
	err := r.queryRowContext(ctx, query,
		wallet.ShopID,
		wallet.BalanceCents,
		wallet.HeldCents,
		wallet.MaxNegativeBalanceCents,
		wallet.TotalSalesCents,
		wallet.TotalCommissionsCents,
		wallet.TotalPayoutsCents,
	).Scan(&wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create merchant wallet: %w", err)
	}
	return nil
}

func (r *WalletTransactionRepositoryInfrastructure) CreateAdmin(ctx context.Context, txn *entity.WalletTransaction) error {
	query := `
		INSERT INTO wallet_transactions (
			shop_id, transaction_type, amount_cents, balance_after_cents,
			reference_type, reference_id, description, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING id, created_at
	`
	err := r.queryRowContext(ctx, query,
		txn.ShopID, txn.TransactionType, txn.AmountCents, txn.BalanceAfterCents,
		txn.ReferenceType, txn.ReferenceID, txn.Description, txn.Status,
	).Scan(&txn.ID, &txn.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create wallet transaction: %w", err)
	}
	return nil
}

// FindByReferenceIDAdmin — sans filtre tenant (scheduler / system)
func (r *WalletTransactionRepositoryInfrastructure) FindByReferenceIDAdmin(
	ctx context.Context,
	refType string,
	refID string,
) (*entity.WalletTransaction, error) {
	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE reference_type = $1 AND reference_id = $2
		LIMIT 1
	`
	return r.scanTransaction(r.queryRowContext(ctx, query, refType, refID))
}
