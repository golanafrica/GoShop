package repository

//go:generate mockgen -destination=../../mocks/repository/mock_commission_batch_repository.go -package=repository . CommissionBatchRepository

import (
	"context"
	"time"

	"Goshop/domain/entity"
)

// ============================================================
// ENTITÉS DU BATCH
// ============================================================

// CommissionBatch représente un lot d'exécution du scheduler
type CommissionBatch struct {
	ID                       string
	StartedAt                time.Time
	CompletedAt              *time.Time
	DurationMs               *int
	TotalProofs              int
	SuccessfulCollections    int
	FailedCollections        int
	SkippedProofs            int
	TotalCommissionCents     int64
	CollectedCommissionCents int64
	FailedCommissionCents    int64
	Status                   string // running, completed, failed
	ErrorMessage             *string
	TriggeredBy              string // scheduler, manual, admin
	ExecutedBy               *string
	CreatedAt                time.Time
}

// CommissionBatchItem représente le résultat pour une preuve individuelle
type CommissionBatchItem struct {
	ID                  string
	BatchID             string
	CODProofID          string
	OrderID             string
	ShopID              string
	CustomerID          string
	Status              string // success, failed, skipped
	CommissionCents     int64
	ErrorMessage        *string
	WalletBalanceBefore *int64
	WalletBalanceAfter  *int64
	AccountFrozen       bool
	ProcessedAt         time.Time
}

// ============================================================
// INTERFACE DU REPOSITORY
// ============================================================

// CommissionBatchRepository gère la persistance des batches de commissions
type CommissionBatchRepository interface {
	// Avec transaction
	WithTX(tx Tx) CommissionBatchRepository

	// CRUD Batch
	CreateBatch(ctx context.Context, batch *CommissionBatch) error
	UpdateBatch(ctx context.Context, batch *CommissionBatch) error
	FindBatchByID(ctx context.Context, id string) (*CommissionBatch, error)
	FindRecentBatches(ctx context.Context, limit int) ([]*CommissionBatch, error)

	// CRUD BatchItem
	CreateBatchItem(ctx context.Context, item *CommissionBatchItem) error
	FindItemsByBatchID(ctx context.Context, batchID string) ([]*CommissionBatchItem, error)

	// Requêtes métier
	FindPendingProofsForCollection(ctx context.Context, limit int) ([]*entity.CODProof, error)
	GetDailyStats(ctx context.Context, days int) ([]*DailyCommissionStats, error)

	// GetMonthlyCommissionByShop retourne le total des commissions collectées ce mois-ci pour une boutique
	GetMonthlyCommissionByShop(ctx context.Context, shopID string) (int64, error)
}

// DailyCommissionStats représente les stats quotidiennes
type DailyCommissionStats struct {
	ExecutionDate       time.Time
	TotalBatches        int
	TotalSuccessful     int
	TotalFailed         int
	TotalSkipped        int
	TotalCollectedCents int64
	TotalFailedCents    int64
}
