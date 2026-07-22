package repository

//go:generate mockgen -destination=../../mocks/repository/mock_merchant_wallet_repository.go -package=repository . MerchantWalletRepository
//go:generate mockgen -destination=../../mocks/repository/mock_wallet_transaction_repository.go -package=repository . WalletTransactionRepository

import (
	"context"
	"time"

	"Goshop/domain/entity"
)

// ============================================================
// MERCHANT WALLET REPOSITORY
// ============================================================

// MerchantWalletRepository définit les opérations sur les portefeuilles marchands
type MerchantWalletRepository interface {
	// Create crée un nouveau portefeuille
	Create(ctx context.Context, wallet *entity.MerchantWallet) error

	// FindByShopID trouve un portefeuille par boutique
	FindByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error)

	// 🆕 FindByShopIDForUpdate trouve un portefeuille et le verrouille pour mise à jour (SELECT ... FOR UPDATE)
	// Cela empêche les Race Conditions lors de modifications concurrentes du solde.
	FindByShopIDForUpdate(ctx context.Context, shopID string) (*entity.MerchantWallet, error)

	// FindAll retourne tous les portefeuilles
	FindAll(ctx context.Context) ([]*entity.MerchantWallet, error)

	// FindFrozen retourne les portefeuilles gelés
	FindFrozen(ctx context.Context) ([]*entity.MerchantWallet, error)

	// FindFrozenByShopID retourne les portefeuilles gelés d'une boutique spécifique
	FindFrozenByShopID(ctx context.Context, shopID string) (*entity.MerchantWallet, error)

	// FindNegativeBalance retourne les portefeuilles avec solde négatif
	FindNegativeBalance(ctx context.Context) ([]*entity.MerchantWallet, error)

	// FindPositiveBalance retourne les portefeuilles avec solde positif
	FindPositiveBalance(ctx context.Context) ([]*entity.MerchantWallet, error)

	// FindBelowThreshold retourne les portefeuilles en dessous d'un seuil
	FindBelowThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error)

	// FindAboveThreshold retourne les portefeuilles au-dessus d'un seuil
	FindAboveThreshold(ctx context.Context, thresholdCents int64) ([]*entity.MerchantWallet, error)

	// FindGracePeriodExpiringSoon retourne les wallets dont la période de grâce expire bientôt
	// daysRemaining : nombre de jours restants ou moins
	FindGracePeriodExpiringSoon(ctx context.Context, daysRemaining int) ([]*entity.MerchantWallet, error)

	// FindGracePeriodExpired retourne les wallets dont la période de grâce est expirée
	FindGracePeriodExpired(ctx context.Context) ([]*entity.MerchantWallet, error)

	// SumTotalBalance somme totale de tous les soldes
	SumTotalBalance(ctx context.Context) (int64, error)

	// SumFrozenBalance somme totale des soldes gelés
	SumFrozenBalance(ctx context.Context) (int64, error)

	// SumNegativeBalance somme totale des soldes négatifs (dettes)
	SumNegativeBalance(ctx context.Context) (int64, error)

	// CountFrozen compte les portefeuilles gelés
	CountFrozen(ctx context.Context) (int, error)

	// CountNegativeBalance compte les portefeuilles avec solde négatif
	CountNegativeBalance(ctx context.Context) (int, error)

	// Update met à jour un portefeuille
	Update(ctx context.Context, wallet *entity.MerchantWallet) error

	// UpdateBalance met à jour uniquement le solde
	UpdateBalance(ctx context.Context, shopID string, balanceCents int64) error

	// Freeze gèle un portefeuille
	Freeze(ctx context.Context, shopID string, reason string, details string) error

	// Unfreeze dégèle un portefeuille
	Unfreeze(ctx context.Context, shopID string) error

	// UpdateStats met à jour les statistiques du portefeuille
	UpdateStats(ctx context.Context, wallet *entity.MerchantWallet) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) MerchantWalletRepository
}

// ============================================================
// WALLET TRANSACTION REPOSITORY
// ============================================================

// WalletTransactionRepository définit les opérations sur les transactions wallet
type WalletTransactionRepository interface {
	// Create crée une nouvelle transaction
	Create(ctx context.Context, tx *entity.WalletTransaction) error

	// FindByID trouve une transaction par ID
	FindByID(ctx context.Context, id string) (*entity.WalletTransaction, error)

	// FindByShopID retourne les transactions d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error)

	// FindByShopIDPaginated retourne les transactions d'une boutique avec pagination
	FindByShopIDPaginated(ctx context.Context, shopID string, limit, offset int) ([]*entity.WalletTransaction, error)

	// FindByType retourne les transactions par type
	FindByType(ctx context.Context, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error)

	// FindByShopIDAndType retourne les transactions d'une boutique par type
	FindByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) ([]*entity.WalletTransaction, error)

	// FindByStatus retourne les transactions par statut
	FindByStatus(ctx context.Context, status entity.WalletTransactionStatus) ([]*entity.WalletTransaction, error)

	// FindByDateRange retourne les transactions dans une plage de dates
	FindByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) ([]*entity.WalletTransaction, error)

	// FindByReferenceID trouve une transaction par référence
	FindByReferenceID(ctx context.Context, refType string, refID string) (*entity.WalletTransaction, error)

	// FindCreditsByShopID retourne uniquement les crédits d'une boutique
	FindCreditsByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error)

	// FindDebitsByShopID retourne uniquement les débits d'une boutique
	FindDebitsByShopID(ctx context.Context, shopID string) ([]*entity.WalletTransaction, error)

	// FindPendingTransactions retourne les transactions en attente
	FindPendingTransactions(ctx context.Context) ([]*entity.WalletTransaction, error)

	// FindFailedTransactions retourne les transactions échouées
	FindFailedTransactions(ctx context.Context) ([]*entity.WalletTransaction, error)

	// FindRecent retourne les N transactions les plus récentes
	FindRecent(ctx context.Context, shopID string, limit int) ([]*entity.WalletTransaction, error)

	// CountByShopID compte les transactions d'une boutique
	CountByShopID(ctx context.Context, shopID string) (int, error)

	// CountByShopIDAndType compte les transactions d'une boutique par type
	CountByShopIDAndType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int, error)

	// SumCreditsByShopID somme des crédits pour une boutique
	SumCreditsByShopID(ctx context.Context, shopID string) (int64, error)

	// SumDebitsByShopID somme des débits pour une boutique
	SumDebitsByShopID(ctx context.Context, shopID string) (int64, error)

	// SumCreditsByDateRange somme des crédits dans une plage de dates
	SumCreditsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error)

	// SumDebitsByDateRange somme des débits dans une plage de dates
	SumDebitsByDateRange(ctx context.Context, shopID string, startDate, endDate time.Time) (int64, error)

	// SumCreditsByType somme des crédits par type pour une boutique
	SumCreditsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error)

	// SumDebitsByType somme des débits par type pour une boutique
	SumDebitsByType(ctx context.Context, shopID string, txType entity.WalletTransactionType) (int64, error)

	// UpdateStatus met à jour le statut d'une transaction
	UpdateStatus(ctx context.Context, id string, status entity.WalletTransactionStatus) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) WalletTransactionRepository
}
