package customerreliabilityscore

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

type customerReliabilityScoreRepository struct {
	db *sql.DB
	tx repository.Tx
}

func NewCustomerReliabilityScoreRepository(db *sql.DB) repository.CustomerReliabilityScoreRepository {
	return &customerReliabilityScoreRepository{db: db}
}

func (r *customerReliabilityScoreRepository) WithTX(tx repository.Tx) repository.CustomerReliabilityScoreRepository {
	return &customerReliabilityScoreRepository{db: r.db, tx: tx}
}

func (r *customerReliabilityScoreRepository) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *customerReliabilityScoreRepository) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *customerReliabilityScoreRepository) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// CRUD DE BASE
// ============================================================

func (r *customerReliabilityScoreRepository) Create(ctx context.Context, score *entity.CustomerReliabilityScore) error {
	query := `
		INSERT INTO customer_reliability_scores (
			customer_id, score, tier, last_calculated_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err := r.queryRowContext(ctx, query,
		score.CustomerID,
		score.Score,
		score.Tier,
		score.LastCalculatedAt,
	).Scan(&score.ID, &score.CreatedAt, &score.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create reliability score: %w", err)
	}

	return nil
}

func (r *customerReliabilityScoreRepository) Update(ctx context.Context, score *entity.CustomerReliabilityScore) error {
	query := `
		UPDATE customer_reliability_scores
		SET score = $1,
		    tier = $2,
		    last_calculated_at = $3,
		    updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`

	err := r.queryRowContext(ctx, query,
		score.Score,
		score.Tier,
		score.LastCalculatedAt,
		score.ID,
	).Scan(&score.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update reliability score: %w", err)
	}

	return nil
}

func (r *customerReliabilityScoreRepository) FindByCustomerID(ctx context.Context, customerID string) (*entity.CustomerReliabilityScore, error) {
	query := `
		SELECT id, customer_id, score, tier, last_calculated_at, created_at, updated_at
		FROM customer_reliability_scores
		WHERE customer_id = $1
	`

	score := &entity.CustomerReliabilityScore{}
	err := r.queryRowContext(ctx, query, customerID).Scan(
		&score.ID,
		&score.CustomerID,
		&score.Score,
		&score.Tier,
		&score.LastCalculatedAt,
		&score.CreatedAt,
		&score.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Pas de score = nouveau client
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find reliability score: %w", err)
	}

	return score, nil
}

func (r *customerReliabilityScoreRepository) Upsert(ctx context.Context, score *entity.CustomerReliabilityScore) error {
	query := `
		INSERT INTO customer_reliability_scores (
			customer_id, score, tier, last_calculated_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (customer_id) DO UPDATE
		SET score = EXCLUDED.score,
		    tier = EXCLUDED.tier,
		    last_calculated_at = EXCLUDED.last_calculated_at,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := r.queryRowContext(ctx, query,
		score.CustomerID,
		score.Score,
		score.Tier,
		score.LastCalculatedAt,
	).Scan(&score.ID, &score.CreatedAt, &score.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert reliability score: %w", err)
	}

	return nil
}

// ============================================================
// STATISTIQUES
// ============================================================

func (r *customerReliabilityScoreRepository) CountByTier(ctx context.Context, tier entity.ReliabilityTier) (int, error) {
	query := `SELECT COUNT(*) FROM customer_reliability_scores WHERE tier = $1`

	var count int
	err := r.queryRowContext(ctx, query, tier).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count by tier: %w", err)
	}

	return count, nil
}

func (r *customerReliabilityScoreRepository) GetAverageScore(ctx context.Context) (float64, error) {
	query := `SELECT COALESCE(AVG(score), 0) FROM customer_reliability_scores`

	var avg float64
	err := r.queryRowContext(ctx, query).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("failed to get average score: %w", err)
	}

	return avg, nil
}

// ============================================================
// REQUÊTES POUR CALCUL AUTOMATIQUE
// ============================================================

// GetCustomersWithOverdueInstallments retourne les customer_ids avec des tranches en retard de +15 jours
func (r *customerReliabilityScoreRepository) GetCustomersWithOverdueInstallments(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT o.customer_id
		FROM orders o
		JOIN order_installments oi ON o.id = oi.order_id
		WHERE oi.status = 'pending'
		  AND oi.due_date < NOW() - INTERVAL '15 days'
		  AND o.created_at > NOW() - INTERVAL '90 days'
	`

	rows, err := r.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get customers with overdue installments: %w", err)
	}
	defer rows.Close()

	var customerIDs []string
	for rows.Next() {
		var customerID string
		if err := rows.Scan(&customerID); err != nil {
			return nil, fmt.Errorf("failed to scan customer_id: %w", err)
		}
		customerIDs = append(customerIDs, customerID)
	}

	return customerIDs, nil
}

// GetCustomersWithCompletedTontineCycles retourne les customer_ids ayant complété des cycles de tontine à l'heure
func (r *customerReliabilityScoreRepository) GetCustomersWithCompletedTontineCycles(ctx context.Context, sinceDays int) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT tp.customer_id
		FROM tontine_payments tp
		WHERE tp.status = 'DONE'
		  AND tp.paid_at >= NOW() - INTERVAL '%d days'
		  AND tp.paid_at <= tp.due_date
	`, sinceDays)

	rows, err := r.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get customers with completed tontine cycles: %w", err)
	}
	defer rows.Close()

	var customerIDs []string
	for rows.Next() {
		var customerID string
		if err := rows.Scan(&customerID); err != nil {
			return nil, fmt.Errorf("failed to scan customer_id: %w", err)
		}
		customerIDs = append(customerIDs, customerID)
	}

	return customerIDs, nil
}

// GetCustomersWithSuccessfulCODOrders retourne les customer_ids avec des commandes COD réussies et cohérentes
func (r *customerReliabilityScoreRepository) GetCustomersWithSuccessfulCODOrders(ctx context.Context, sinceDays int) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT o.customer_id
		FROM orders o
		JOIN cod_proofs cp ON o.id = cp.order_id
		WHERE o.payment_method = 'cash_on_delivery'
		  AND o.status = 'delivered'
		  AND cp.status = 'completed'
		  AND o.delivered_at >= NOW() - INTERVAL '%d days'
		  AND cp.amounts_match = true
		  AND cp.dates_match = true
	`, sinceDays)

	rows, err := r.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get customers with successful COD orders: %w", err)
	}
	defer rows.Close()

	var customerIDs []string
	for rows.Next() {
		var customerID string
		if err := rows.Scan(&customerID); err != nil {
			return nil, fmt.Errorf("failed to scan customer_id: %w", err)
		}
		customerIDs = append(customerIDs, customerID)
	}

	return customerIDs, nil
}
