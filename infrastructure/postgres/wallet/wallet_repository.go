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

// MerchantWalletRepositoryInfrastructure implémente repository.MerchantWalletRepository
type MerchantWalletRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewMerchantWalletRepositoryInfrastructure crée une nouvelle instance
func NewMerchantWalletRepositoryInfrastructure(db *sql.DB) repository.MerchantWalletRepository {
	return &MerchantWalletRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
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

// scanWallet scanne une ligne dans une entité MerchantWallet
func (r *MerchantWalletRepositoryInfrastructure) scanWallet(row *sql.Row) (*entity.MerchantWallet, error) {
	wallet := &entity.MerchantWallet{}
	var frozenAt, frozenUntil sql.NullTime
	var frozenReason sql.NullString

	err := row.Scan(
		&wallet.ShopID,
		&wallet.BalanceCents,
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

// scanWallets scanne plusieurs lignes
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

// Create crée un nouveau portefeuille
func (r *MerchantWalletRepositoryInfrastructure) Create(ctx context.Context, wallet *entity.MerchantWallet) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le wallet appartient à la boutique
	if wallet.ShopID != shopID {
		return fmt.Errorf("access denied: wallet shop_id does not match tenant shop")
	}

	// Valider le wallet
	if err := wallet.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO merchant_wallets (
			shop_id, balance_cents,
			max_negative_balance_cents,
			total_sales_cents, total_commissions_cents, total_payouts_cents,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		wallet.ShopID,
		wallet.BalanceCents,
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

// FindByShopID trouve un portefeuille par boutique
func (r *MerchantWalletRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query wallet of another shop")
	}

	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE shop_id = $1
	`

	return r.scanWallet(r.queryRowContext(ctx, query, shopID))
}

// FindAll retourne tous les portefeuilles
func (r *MerchantWalletRepositoryInfrastructure) FindAll(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		ORDER BY created_at DESC
	`

	return r.scanWallets(ctx, query)
}

// FindFrozen retourne les portefeuilles gelés
func (r *MerchantWalletRepositoryInfrastructure) FindFrozen(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE is_frozen = true
		ORDER BY frozen_at ASC
	`

	return r.scanWallets(ctx, query)
}

// FindFrozenByShopID retourne les portefeuilles gelés d'une boutique spécifique
func (r *MerchantWalletRepositoryInfrastructure) FindFrozenByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query wallet of another shop")
	}

	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE shop_id = $1 AND is_frozen = true
	`

	return r.scanWallet(r.queryRowContext(ctx, query, shopID))
}

// FindNegativeBalance retourne les portefeuilles avec solde négatif
func (r *MerchantWalletRepositoryInfrastructure) FindNegativeBalance(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE balance_cents < 0
		ORDER BY balance_cents ASC
	`

	return r.scanWallets(ctx, query)
}

// FindPositiveBalance retourne les portefeuilles avec solde positif
func (r *MerchantWalletRepositoryInfrastructure) FindPositiveBalance(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE balance_cents > 0
		ORDER BY balance_cents DESC
	`

	return r.scanWallets(ctx, query)
}

// FindBelowThreshold retourne les portefeuilles en dessous d'un seuil
func (r *MerchantWalletRepositoryInfrastructure) FindBelowThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE balance_cents < $1
		ORDER BY balance_cents ASC
	`

	return r.scanWallets(ctx, query, thresholdCents)
}

// FindAboveThreshold retourne les portefeuilles au-dessus d'un seuil
func (r *MerchantWalletRepositoryInfrastructure) FindAboveThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE balance_cents > $1
		ORDER BY balance_cents DESC
	`

	return r.scanWallets(ctx, query, thresholdCents)
}

// FindGracePeriodExpiringSoon retourne les wallets dont la période de grâce expire bientôt
func (r *MerchantWalletRepositoryInfrastructure) FindGracePeriodExpiringSoon(ctx context.Context, daysRemaining int) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE is_frozen = true
		  AND frozen_until IS NOT NULL
		  AND frozen_until <= NOW() + INTERVAL '1 day' * $1
		  AND frozen_until > NOW()
		ORDER BY frozen_until ASC
	`

	return r.scanWallets(ctx, query, daysRemaining)
}

// FindGracePeriodExpired retourne les wallets dont la période de grâce est expirée
func (r *MerchantWalletRepositoryInfrastructure) FindGracePeriodExpired(ctx context.Context) ([]*entity.MerchantWallet, error) {
	query := `
		SELECT shop_id, balance_cents, is_frozen, frozen_at, frozen_reason, frozen_until,
		       max_negative_balance_cents,
		       total_sales_cents, total_commissions_cents, total_payouts_cents,
		       created_at, updated_at
		FROM merchant_wallets
		WHERE is_frozen = true
		  AND frozen_until IS NOT NULL
		  AND frozen_until <= NOW()
		ORDER BY frozen_until ASC
	`

	return r.scanWallets(ctx, query)
}

// SumTotalBalance somme totale de tous les soldes
func (r *MerchantWalletRepositoryInfrastructure) SumTotalBalance(ctx context.Context) (int64, error) {
	query := `SELECT COALESCE(SUM(balance_cents), 0) FROM merchant_wallets`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum total balance: %w", err)
	}

	return total, nil
}

// SumFrozenBalance somme totale des soldes gelés
func (r *MerchantWalletRepositoryInfrastructure) SumFrozenBalance(ctx context.Context) (int64, error) {
	query := `SELECT COALESCE(SUM(balance_cents), 0) FROM merchant_wallets WHERE is_frozen = true`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum frozen balance: %w", err)
	}

	return total, nil
}

// SumNegativeBalance somme totale des soldes négatifs (dettes)
func (r *MerchantWalletRepositoryInfrastructure) SumNegativeBalance(ctx context.Context) (int64, error) {
	query := `SELECT COALESCE(SUM(balance_cents), 0) FROM merchant_wallets WHERE balance_cents < 0`

	var total int64
	err := r.queryRowContext(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum negative balance: %w", err)
	}

	return total, nil
}

// CountFrozen compte les portefeuilles gelés
func (r *MerchantWalletRepositoryInfrastructure) CountFrozen(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM merchant_wallets WHERE is_frozen = true`

	var count int
	err := r.queryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count frozen wallets: %w", err)
	}

	return count, nil
}

// CountNegativeBalance compte les portefeuilles avec solde négatif
func (r *MerchantWalletRepositoryInfrastructure) CountNegativeBalance(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM merchant_wallets WHERE balance_cents < 0`

	var count int
	err := r.queryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count negative balance wallets: %w", err)
	}

	return count, nil
}

// Update met à jour un portefeuille
func (r *MerchantWalletRepositoryInfrastructure) Update(ctx context.Context, wallet *entity.MerchantWallet) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le wallet appartient à la boutique
	if wallet.ShopID != shopID {
		return fmt.Errorf("access denied: wallet does not belong to tenant shop")
	}

	// Valider le wallet
	if err := wallet.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE merchant_wallets
		SET balance_cents = $2,
		    is_frozen = $3,
		    frozen_at = $4,
		    frozen_reason = $5,
		    frozen_until = $6,
		    max_negative_balance_cents = $7,
		    total_sales_cents = $8,
		    total_commissions_cents = $9,
		    total_payouts_cents = $10,
		    updated_at = NOW()
		WHERE shop_id = $1
		RETURNING updated_at
	`

	err = r.queryRowContext(ctx, query,
		wallet.ShopID,
		wallet.BalanceCents,
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

// UpdateBalance met à jour uniquement le solde
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
		SET balance_cents = $2,
		    updated_at = NOW()
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

// Freeze gèle un portefeuille
func (r *MerchantWalletRepositoryInfrastructure) Freeze(ctx context.Context, shopID string, reason string, details string) error {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}
	if shopID != currentShopID {
		return fmt.Errorf("access denied: cannot freeze wallet of another shop")
	}

	now := time.Now().UTC()
	gracePeriodEnds := now.AddDate(0, 0, entity.DefaultGracePeriodDays)

	result, err := r.execContext(ctx, `
		UPDATE merchant_wallets
		SET is_frozen = true,
		    frozen_at = $2,
		    frozen_reason = $3,
		    frozen_until = $4,
		    updated_at = NOW()
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

// Unfreeze dégèle un portefeuille
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
		SET is_frozen = false,
		    frozen_at = NULL,
		    frozen_reason = NULL,
		    frozen_until = NULL,
		    updated_at = NOW()
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

// UpdateStats met à jour les statistiques du portefeuille
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
		SET total_sales_cents = $2,
		    total_commissions_cents = $3,
		    total_payouts_cents = $4,
		    updated_at = NOW()
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
// WALLET TRANSACTION REPOSITORY
// ============================================================

// WalletTransactionRepositoryInfrastructure implémente repository.WalletTransactionRepository
type WalletTransactionRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewWalletTransactionRepositoryInfrastructure crée une nouvelle instance
func NewWalletTransactionRepositoryInfrastructure(db *sql.DB) repository.WalletTransactionRepository {
	return &WalletTransactionRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *WalletTransactionRepositoryInfrastructure) WithTX(tx repository.Tx) repository.WalletTransactionRepository {
	return &WalletTransactionRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

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

// scanTransaction scanne une ligne dans une entité WalletTransaction
func (r *WalletTransactionRepositoryInfrastructure) scanTransaction(row *sql.Row) (*entity.WalletTransaction, error) {
	txn := &entity.WalletTransaction{}
	var referenceType, referenceID, description sql.NullString

	err := row.Scan(
		&txn.ID,
		&txn.ShopID,
		&txn.TransactionType,
		&txn.AmountCents,
		&txn.BalanceAfterCents,
		&referenceType,
		&referenceID,
		&description,
		&txn.Status,
		&txn.CreatedAt,
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

// scanTransactions scanne plusieurs lignes
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
			&txn.ID,
			&txn.ShopID,
			&txn.TransactionType,
			&txn.AmountCents,
			&txn.BalanceAfterCents,
			&referenceType,
			&referenceID,
			&description,
			&txn.Status,
			&txn.CreatedAt,
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

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée une nouvelle transaction
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
			reference_type, reference_id, description, status,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING id, created_at
	`

	err = r.queryRowContext(ctx, query,
		txn.ShopID,
		txn.TransactionType,
		txn.AmountCents,
		txn.BalanceAfterCents,
		txn.ReferenceType,
		txn.ReferenceID,
		txn.Description,
		txn.Status,
	).Scan(&txn.ID, &txn.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create wallet transaction: %w", err)
	}

	return nil
}

// FindByID trouve une transaction par ID
func (r *WalletTransactionRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanTransaction(r.queryRowContext(ctx, query, id, shopID))
}

// FindByShopID retourne les transactions d'une boutique
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
		FROM wallet_transactions
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID)
}

// FindByShopIDPaginated retourne les transactions d'une boutique avec pagination
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
		FROM wallet_transactions
		WHERE shop_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.scanTransactions(ctx, query, shopID, limit, offset)
}

// FindByType retourne les transactions par type
func (r *WalletTransactionRepositoryInfrastructure) FindByType(ctx context.Context, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID, txType)
}

// FindByShopIDAndType retourne les transactions d'une boutique par type
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
		FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID, txType)
}

// FindByStatus retourne les transactions par statut
func (r *WalletTransactionRepositoryInfrastructure) FindByStatus(ctx context.Context, status entity.WalletTransactionStatus) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID, status)
}

// FindByDateRange retourne les transactions dans une plage de dates
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
		WHERE shop_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID, startDate, endDate)
}

// FindByReferenceID trouve une transaction par référence
func (r *WalletTransactionRepositoryInfrastructure) FindByReferenceID(ctx context.Context, refType string, refID string) (*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND reference_type = $2 AND reference_id = $3
		LIMIT 1
	`

	return r.scanTransaction(r.queryRowContext(ctx, query, shopID, refType, refID))
}

// FindCreditsByShopID retourne uniquement les crédits d'une boutique
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
		FROM wallet_transactions
		WHERE shop_id = $1 AND amount_cents > 0
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID)
}

// FindDebitsByShopID retourne uniquement les débits d'une boutique
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
		FROM wallet_transactions
		WHERE shop_id = $1 AND amount_cents < 0
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID)
}

// FindPendingTransactions retourne les transactions en attente
func (r *WalletTransactionRepositoryInfrastructure) FindPendingTransactions(ctx context.Context) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
	`

	return r.scanTransactions(ctx, query, shopID)
}

// FindFailedTransactions retourne les transactions échouées
func (r *WalletTransactionRepositoryInfrastructure) FindFailedTransactions(ctx context.Context) ([]*entity.WalletTransaction, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, shop_id, transaction_type, amount_cents, balance_after_cents,
		       reference_type, reference_id, description, status, created_at
		FROM wallet_transactions
		WHERE shop_id = $1 AND status = 'failed'
		ORDER BY created_at DESC
	`

	return r.scanTransactions(ctx, query, shopID)
}

// FindRecent retourne les N transactions les plus récentes
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
		FROM wallet_transactions
		WHERE shop_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	return r.scanTransactions(ctx, query, shopID, limit)
}

// CountByShopID compte les transactions d'une boutique
func (r *WalletTransactionRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID string) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count transactions of another shop")
	}

	query := `SELECT COUNT(*) FROM wallet_transactions WHERE shop_id = $1`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

// CountByShopIDAndType compte les transactions d'une boutique par type
func (r *WalletTransactionRepositoryInfrastructure) CountByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count transactions of another shop")
	}

	query := `
		SELECT COUNT(*) FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID, txType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

// SumCreditsByShopID somme des crédits pour une boutique
func (r *WalletTransactionRepositoryInfrastructure) SumCreditsByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM wallet_transactions
		WHERE shop_id = $1 AND amount_cents > 0
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum credits: %w", err)
	}

	return total, nil
}

// SumDebitsByShopID somme des débits pour une boutique
func (r *WalletTransactionRepositoryInfrastructure) SumDebitsByShopID(ctx context.Context, shopID string) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(ABS(amount_cents)), 0)
		FROM wallet_transactions
		WHERE shop_id = $1 AND amount_cents < 0
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum debits: %w", err)
	}

	return total, nil
}

// SumCreditsByDateRange somme des crédits dans une plage de dates
func (r *WalletTransactionRepositoryInfrastructure) SumCreditsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM wallet_transactions
		WHERE shop_id = $1
		  AND amount_cents > 0
		  AND created_at >= $2
		  AND created_at <= $3
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID, startDate, endDate).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum credits: %w", err)
	}

	return total, nil
}

// SumDebitsByDateRange somme des débits dans une plage de dates
func (r *WalletTransactionRepositoryInfrastructure) SumDebitsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(ABS(amount_cents)), 0)
		FROM wallet_transactions
		WHERE shop_id = $1
		  AND amount_cents < 0
		  AND created_at >= $2
		  AND created_at <= $3
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID, startDate, endDate).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum debits: %w", err)
	}

	return total, nil
}

// SumCreditsByType somme des crédits par type pour une boutique
func (r *WalletTransactionRepositoryInfrastructure) SumCreditsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2 AND amount_cents > 0
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID, txType).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum credits by type: %w", err)
	}

	return total, nil
}

// SumDebitsByType somme des débits par type pour une boutique
func (r *WalletTransactionRepositoryInfrastructure) SumDebitsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum transactions of another shop")
	}

	query := `
		SELECT COALESCE(SUM(ABS(amount_cents)), 0)
		FROM wallet_transactions
		WHERE shop_id = $1 AND transaction_type = $2 AND amount_cents < 0
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID, txType).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum debits by type: %w", err)
	}

	return total, nil
}

// UpdateStatus met à jour le statut d'une transaction
func (r *WalletTransactionRepositoryInfrastructure) UpdateStatus(ctx context.Context, id string, status entity.WalletTransactionStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid transaction status: %s", status)
	}

	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	result, err := r.execContext(ctx, `
		UPDATE wallet_transactions
		SET status = $2
		WHERE id = $1 AND shop_id = $3
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
