package repository

//go:generate mockgen -destination=../../mocks/repository/mock_merchant_wallet_repository.go -package=repository . MerchantWalletRepository
//go:generate mockgen -destination=../../mocks/repository/mock_wallet_transaction_repository.go -package=repository . WalletTransactionRepository

import (
	"context"
	"errors"
	"time"

	"Goshop/domain/entity"
)

// ============================================================
// ERREURS SENTINELLES (Typées pour errors.Is)
// ============================================================

var ErrWalletNotFound = errors.New("merchant wallet not found")
var ErrWalletTransactionNotFound = errors.New("wallet transaction not found")
var ErrLegacyIdempotencyAmbiguous = errors.New("idempotency conflict: legacy transaction without gross amount, requires manual reconciliation")

// ============================================================
// TYPES DE DONNÉES AGRÉGÉES
// ============================================================

type LedgerSum struct {
	TotalCredits int64
	TotalDebits  int64
	NetBalance   int64
}

// 🆕 PR-D FIX : Snapshot atomique pour éviter les race conditions entre lecture wallet et ledger
type ReconciliationSnapshot struct {
	WalletExists  bool
	WalletBalance int64
	WalletHeld    int64
	WalletDebt    int64
	LedgerCredits int64
	LedgerDebits  int64
	LedgerNet     int64
}

// ============================================================
// MERCHANT WALLET REPOSITORY
// ============================================================

type MerchantWalletRepository interface {
	Create(ctx context.Context, wallet *entity.MerchantWallet) error
	FindByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error)
	FindByShopIDForUpdate(ctx context.Context, shopID string) (*entity.MerchantWallet, error)
	FindByShopIDForUpdateAdmin(ctx context.Context, shopID string) (*entity.MerchantWallet, error)
	FindAll(ctx context.Context) ([]*entity.MerchantWallet, error)
	FindFrozen(ctx context.Context) ([]*entity.MerchantWallet, error)
	FindFrozenByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error)
	FindNegativeBalance(ctx context.Context) ([]*entity.MerchantWallet, error)
	FindPositiveBalance(ctx context.Context) ([]*entity.MerchantWallet, error)
	FindBelowThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error)
	FindAboveThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error)
	FindGracePeriodExpiringSoon(ctx context.Context, daysRemaining int) ([]*entity.MerchantWallet, error)
	FindGracePeriodExpired(ctx context.Context) ([]*entity.MerchantWallet, error)
	FindGracePeriodExpiredPaginated(ctx context.Context, limit, offset int) ([]*entity.MerchantWallet, error)
	SumTotalBalance(ctx context.Context) (int64, error)
	SumFrozenBalance(ctx context.Context) (int64, error)
	SumNegativeBalance(ctx context.Context) (int64, error)
	CountFrozen(ctx context.Context) (int, error)
	CountNegativeBalance(ctx context.Context) (int, error)

	// 🆕 PR-D FIX : Méthode unifiée pour une lecture cohérente (snapshot)
	GetReconciliationSnapshot(ctx context.Context, shopID string) (*ReconciliationSnapshot, error)

	GetLedgerSum(ctx context.Context, shopID string) (*LedgerSum, error)
	Update(ctx context.Context, wallet *entity.MerchantWallet) error
	UpdateAdmin(ctx context.Context, wallet *entity.MerchantWallet) error
	CreateAdmin(ctx context.Context, wallet *entity.MerchantWallet) error
	WithTX(tx Tx) MerchantWalletRepository
}

// ============================================================
// WALLET TRANSACTION REPOSITORY
// ============================================================

type WalletTransactionRepository interface {
	Create(ctx context.Context, tx *entity.WalletTransaction) error
	CreateAdmin(ctx context.Context, tx *entity.WalletTransaction) error
	FindByID(ctx context.Context, id string) (*entity.WalletTransaction, error)
	FindByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error)
	FindByShopIDPaginated(ctx context.Context, shopID string, limit, offset int) ([]*entity.WalletTransaction, error)
	FindByType(ctx context.Context, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error)
	FindByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error)
	FindByStatus(ctx context.Context, status entity.WalletTransactionStatus) ([]*entity.WalletTransaction, error)
	FindByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) ([]*entity.WalletTransaction, error)
	FindByReferenceID(ctx context.Context, refType string, refID string) (*entity.WalletTransaction, error)
	FindByReferenceIDAdmin(ctx context.Context, refType string, refID string) (*entity.WalletTransaction, error)
	FindCreditsByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error)
	FindDebitsByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error)
	FindPendingTransactions(ctx context.Context) ([]*entity.WalletTransaction, error)
	FindFailedTransactions(ctx context.Context) ([]*entity.WalletTransaction, error)
	FindRecent(ctx context.Context, shopID string, limit int) ([]*entity.WalletTransaction, error)
	CountByShopID(ctx context.Context, shopID string) (int, error)
	CountByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int, error)
	SumCreditsByShopID(ctx context.Context, shopID string) (int64, error)
	SumDebitsByShopID(ctx context.Context, shopID string) (int64, error)
	SumCreditsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error)
	SumDebitsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error)
	SumCreditsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error)
	SumDebitsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error)
	UpdateStatus(ctx context.Context, id string, status entity.WalletTransactionStatus) error
	WithTX(tx Tx) WalletTransactionRepository
}
