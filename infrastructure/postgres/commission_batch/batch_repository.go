package commissionbatch

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

// ============================================================
// IMPLÉMENTATION POSTGRESQL
// ============================================================

// CommissionBatchRepositoryPostgres implémente CommissionBatchRepository
type CommissionBatchRepositoryPostgres struct {
	db *sql.DB
	tx *sql.Tx
}

// NewCommissionBatchRepositoryPostgres crée une nouvelle instance
func NewCommissionBatchRepositoryPostgres(db *sql.DB) *CommissionBatchRepositoryPostgres {
	return &CommissionBatchRepositoryPostgres{db: db}
}

// WithTX retourne une version du repo attachée à une transaction
func (r *CommissionBatchRepositoryPostgres) WithTX(tx repository.Tx) repository.CommissionBatchRepository {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return r
	}
	return &CommissionBatchRepositoryPostgres{
		db: r.db,
		tx: sqlTx,
	}
}

// executor retourne soit la transaction, soit la DB
func (r *CommissionBatchRepositoryPostgres) executor() interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ============================================================
// CRUD BATCH
// ============================================================

// CreateBatch crée un nouveau batch
func (r *CommissionBatchRepositoryPostgres) CreateBatch(ctx context.Context, batch *repository.CommissionBatch) error {
	query := `
		INSERT INTO commission_batches (
			id, started_at, status, triggered_by, executed_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.executor().ExecContext(ctx, query,
		batch.ID,
		batch.StartedAt,
		batch.Status,
		batch.TriggeredBy,
		batch.ExecutedBy,
		batch.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create batch: %w", err)
	}
	return nil
}

// UpdateBatch met à jour un batch existant
func (r *CommissionBatchRepositoryPostgres) UpdateBatch(ctx context.Context, batch *repository.CommissionBatch) error {
	query := `
		UPDATE commission_batches SET
			completed_at = $1,
			duration_ms = $2,
			total_proofs = $3,
			successful_collections = $4,
			failed_collections = $5,
			skipped_proofs = $6,
			total_commission_cents = $7,
			collected_commission_cents = $8,
			failed_commission_cents = $9,
			status = $10,
			error_message = $11
		WHERE id = $12
	`
	_, err := r.executor().ExecContext(ctx, query,
		batch.CompletedAt,
		batch.DurationMs,
		batch.TotalProofs,
		batch.SuccessfulCollections,
		batch.FailedCollections,
		batch.SkippedProofs,
		batch.TotalCommissionCents,
		batch.CollectedCommissionCents,
		batch.FailedCommissionCents,
		batch.Status,
		batch.ErrorMessage,
		batch.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update batch: %w", err)
	}
	return nil
}

// FindBatchByID récupère un batch par son ID
func (r *CommissionBatchRepositoryPostgres) FindBatchByID(ctx context.Context, id string) (*repository.CommissionBatch, error) {
	query := `
		SELECT 
			id, started_at, completed_at, duration_ms,
			total_proofs, successful_collections, failed_collections, skipped_proofs,
			total_commission_cents, collected_commission_cents, failed_commission_cents,
			status, error_message, triggered_by, executed_by, created_at
		FROM commission_batches
		WHERE id = $1
	`
	batch := &repository.CommissionBatch{}
	err := r.executor().QueryRowContext(ctx, query, id).Scan(
		&batch.ID,
		&batch.StartedAt,
		&batch.CompletedAt,
		&batch.DurationMs,
		&batch.TotalProofs,
		&batch.SuccessfulCollections,
		&batch.FailedCollections,
		&batch.SkippedProofs,
		&batch.TotalCommissionCents,
		&batch.CollectedCommissionCents,
		&batch.FailedCommissionCents,
		&batch.Status,
		&batch.ErrorMessage,
		&batch.TriggeredBy,
		&batch.ExecutedBy,
		&batch.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("batch not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find batch: %w", err)
	}
	return batch, nil
}

// FindRecentBatches récupère les N derniers batches
func (r *CommissionBatchRepositoryPostgres) FindRecentBatches(ctx context.Context, limit int) ([]*repository.CommissionBatch, error) {
	query := `
		SELECT 
			id, started_at, completed_at, duration_ms,
			total_proofs, successful_collections, failed_collections, skipped_proofs,
			total_commission_cents, collected_commission_cents, failed_commission_cents,
			status, error_message, triggered_by, executed_by, created_at
		FROM commission_batches
		ORDER BY started_at DESC
		LIMIT $1
	`
	rows, err := r.executor().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query batches: %w", err)
	}
	defer rows.Close()

	var batches []*repository.CommissionBatch
	for rows.Next() {
		batch := &repository.CommissionBatch{}
		err := rows.Scan(
			&batch.ID,
			&batch.StartedAt,
			&batch.CompletedAt,
			&batch.DurationMs,
			&batch.TotalProofs,
			&batch.SuccessfulCollections,
			&batch.FailedCollections,
			&batch.SkippedProofs,
			&batch.TotalCommissionCents,
			&batch.CollectedCommissionCents,
			&batch.FailedCommissionCents,
			&batch.Status,
			&batch.ErrorMessage,
			&batch.TriggeredBy,
			&batch.ExecutedBy,
			&batch.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch: %w", err)
		}
		batches = append(batches, batch)
	}
	return batches, nil
}

// ============================================================
// CRUD BATCH ITEM
// ============================================================

// CreateBatchItem crée un item de batch
func (r *CommissionBatchRepositoryPostgres) CreateBatchItem(ctx context.Context, item *repository.CommissionBatchItem) error {
	query := `
		INSERT INTO commission_batch_items (
			id, batch_id, cod_proof_id, order_id, shop_id, customer_id,
			status, commission_cents, error_message,
			wallet_balance_before, wallet_balance_after, account_frozen,
			processed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.executor().ExecContext(ctx, query,
		item.ID,
		item.BatchID,
		item.CODProofID,
		item.OrderID,
		item.ShopID,
		item.CustomerID,
		item.Status,
		item.CommissionCents,
		item.ErrorMessage,
		item.WalletBalanceBefore,
		item.WalletBalanceAfter,
		item.AccountFrozen,
		item.ProcessedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create batch item: %w", err)
	}
	return nil
}

// FindItemsByBatchID récupère tous les items d'un batch
func (r *CommissionBatchRepositoryPostgres) FindItemsByBatchID(ctx context.Context, batchID string) ([]*repository.CommissionBatchItem, error) {
	query := `
		SELECT 
			id, batch_id, cod_proof_id, order_id, shop_id, customer_id,
			status, commission_cents, error_message,
			wallet_balance_before, wallet_balance_after, account_frozen,
			processed_at
		FROM commission_batch_items
		WHERE batch_id = $1
		ORDER BY processed_at ASC
	`
	rows, err := r.executor().QueryContext(ctx, query, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to query batch items: %w", err)
	}
	defer rows.Close()

	var items []*repository.CommissionBatchItem
	for rows.Next() {
		item := &repository.CommissionBatchItem{}
		err := rows.Scan(
			&item.ID,
			&item.BatchID,
			&item.CODProofID,
			&item.OrderID,
			&item.ShopID,
			&item.CustomerID,
			&item.Status,
			&item.CommissionCents,
			&item.ErrorMessage,
			&item.WalletBalanceBefore,
			&item.WalletBalanceAfter,
			&item.AccountFrozen,
			&item.ProcessedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// ============================================================
// REQUÊTES MÉTIER
// ============================================================

// FindPendingProofsForCollection récupère les preuves COD prêtes à être collectées
func (r *CommissionBatchRepositoryPostgres) FindPendingProofsForCollection(
	ctx context.Context,
	limit int,
) ([]*entity.CODProof, error) {
	query := `
		SELECT 
			id, order_id, shop_id, customer_id,
			commission_cents, commission_status, status,
			created_at, updated_at
		FROM cod_proofs
		WHERE status = 'confirmed'
		  AND commission_status IN ('pending', 'due')
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.executor().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending proofs: %w", err)
	}
	defer rows.Close()

	var proofs []*entity.CODProof
	for rows.Next() {
		proof := &entity.CODProof{}
		err := rows.Scan(
			&proof.ID,
			&proof.OrderID,
			&proof.ShopID,
			&proof.CustomerID,
			&proof.CommissionCents,
			&proof.CommissionStatus,
			&proof.Status,
			&proof.CreatedAt,
			&proof.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan proof: %w", err)
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

// GetDailyStats récupère les statistiques quotidiennes
func (r *CommissionBatchRepositoryPostgres) GetDailyStats(
	ctx context.Context,
	days int,
) ([]*repository.DailyCommissionStats, error) {
	query := `
		SELECT 
			execution_date,
			total_batches,
			total_successful,
			total_failed,
			total_skipped,
			total_collected_cents,
			total_failed_cents
		FROM v_commission_daily_stats
		WHERE execution_date >= CURRENT_DATE - $1::integer
		ORDER BY execution_date DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, days)
	if err != nil {
		// Si la vue n'existe pas, fallback sur requête directe
		return r.getDailyStatsFallback(ctx, days)
	}
	defer rows.Close()

	var stats []*repository.DailyCommissionStats
	for rows.Next() {
		stat := &repository.DailyCommissionStats{}
		err := rows.Scan(
			&stat.ExecutionDate,
			&stat.TotalBatches,
			&stat.TotalSuccessful,
			&stat.TotalFailed,
			&stat.TotalSkipped,
			&stat.TotalCollectedCents,
			&stat.TotalFailedCents,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

// getDailyStatsFallback requête de secours si la vue n'existe pas
func (r *CommissionBatchRepositoryPostgres) getDailyStatsFallback(
	ctx context.Context,
	days int,
) ([]*repository.DailyCommissionStats, error) {
	query := `
		SELECT 
			DATE(started_at) as execution_date,
			COUNT(*) as total_batches,
			SUM(successful_collections) as total_successful,
			SUM(failed_collections) as total_failed,
			SUM(skipped_proofs) as total_skipped,
			SUM(collected_commission_cents) as total_collected_cents,
			SUM(failed_commission_cents) as total_failed_cents
		FROM commission_batches
		WHERE status = 'completed'
		  AND started_at >= CURRENT_DATE - $1::integer
		GROUP BY DATE(started_at)
		ORDER BY execution_date DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily stats: %w", err)
	}
	defer rows.Close()

	var stats []*repository.DailyCommissionStats
	for rows.Next() {
		stat := &repository.DailyCommissionStats{}
		err := rows.Scan(
			&stat.ExecutionDate,
			&stat.TotalBatches,
			&stat.TotalSuccessful,
			&stat.TotalFailed,
			&stat.TotalSkipped,
			&stat.TotalCollectedCents,
			&stat.TotalFailedCents,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

// ============================================================
// VÉRIFICATION DE L'INTERFACE
// ============================================================

var _ repository.CommissionBatchRepository = (*CommissionBatchRepositoryPostgres)(nil)

// Silence "imported and not used" pour time
var _ = time.Now
