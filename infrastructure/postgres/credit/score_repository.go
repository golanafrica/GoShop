package credit

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CreditScoreRepositoryInfrastructure implémente repository.CreditScoreRepository
type CreditScoreRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCreditScoreRepositoryInfrastructure crée une nouvelle instance
func NewCreditScoreRepositoryInfrastructure(db *sql.DB) repository.CreditScoreRepository {
	return &CreditScoreRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CreditScoreRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CreditScoreRepository {
	return &CreditScoreRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *CreditScoreRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CreditScoreRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CreditScoreRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *CreditScoreRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanScore scanne une ligne dans une entité CreditScore
func (r *CreditScoreRepositoryInfrastructure) scanScore(row *sql.Row) (*entity.CreditScore, error) {
	score := &entity.CreditScore{}

	err := row.Scan(
		&score.ID,
		&score.CustomerID,
		&score.ShopID,
		&score.Score,
		&score.TotalContracts,
		&score.CompletedContracts,
		&score.OnTimePayments,
		&score.LatePayments,
		&score.Defaults,
		&score.LastUpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit score not found")
		}
		return nil, fmt.Errorf("failed to scan credit score: %w", err)
	}

	return score, nil
}

// scanScores scanne plusieurs lignes
func (r *CreditScoreRepositoryInfrastructure) scanScores(ctx context.Context, query string, args ...interface{}) ([]*entity.CreditScore, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query credit scores: %w", err)
	}
	defer rows.Close()

	var scores []*entity.CreditScore
	for rows.Next() {
		score := &entity.CreditScore{}
		err := rows.Scan(
			&score.ID,
			&score.CustomerID,
			&score.ShopID,
			&score.Score,
			&score.TotalContracts,
			&score.CompletedContracts,
			&score.OnTimePayments,
			&score.LatePayments,
			&score.Defaults,
			&score.LastUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		scores = append(scores, score)
	}

	if scores == nil {
		scores = []*entity.CreditScore{}
	}
	return scores, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée un nouveau score
func (r *CreditScoreRepositoryInfrastructure) Create(ctx context.Context, score *entity.CreditScore) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le score appartient à la boutique
	if score.ShopID != shopID {
		return fmt.Errorf("access denied: score shop_id does not match tenant shop")
	}

	query := `
		INSERT INTO credit_scores (
			customer_id, shop_id, score,
			total_contracts, completed_contracts,
			on_time_payments, late_payments, defaults,
			last_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING id, last_updated_at
	`

	err = r.queryRowContext(ctx, query,
		score.CustomerID,
		score.ShopID,
		score.Score,
		score.TotalContracts,
		score.CompletedContracts,
		score.OnTimePayments,
		score.LatePayments,
		score.Defaults,
	).Scan(&score.ID, &score.LastUpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create credit score: %w", err)
	}

	return nil
}

// FindByCustomerAndShop trouve un score par client et boutique
func (r *CreditScoreRepositoryInfrastructure) FindByCustomerAndShop(ctx context.Context, customerID, shopID string) (*entity.CreditScore, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT id, customer_id, shop_id, score,
		       total_contracts, completed_contracts,
		       on_time_payments, late_payments, defaults,
		       last_updated_at
		FROM credit_scores
		WHERE customer_id = $1 AND shop_id = $2
	`

	return r.scanScore(r.queryRowContext(ctx, query, customerID, shopID))
}

// FindByCustomerID retourne tous les scores d'un client (multi-boutique)
func (r *CreditScoreRepositoryInfrastructure) FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CreditScore, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, shop_id, score,
		       total_contracts, completed_contracts,
		       on_time_payments, late_payments, defaults,
		       last_updated_at
		FROM credit_scores
		WHERE customer_id = $1 AND shop_id = $2
		ORDER BY last_updated_at DESC
	`

	return r.scanScores(ctx, query, customerID, shopID)
}

// FindByShopID retourne tous les scores d'une boutique
func (r *CreditScoreRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.CreditScore, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT id, customer_id, shop_id, score,
		       total_contracts, completed_contracts,
		       on_time_payments, late_payments, defaults,
		       last_updated_at
		FROM credit_scores
		WHERE shop_id = $1
		ORDER BY score DESC
	`

	return r.scanScores(ctx, query, shopID)
}

// FindLowScores retourne les scores faibles (< seuil)
func (r *CreditScoreRepositoryInfrastructure) FindLowScores(ctx context.Context, shopID string, minScore int) ([]*entity.CreditScore, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT id, customer_id, shop_id, score,
		       total_contracts, completed_contracts,
		       on_time_payments, late_payments, defaults,
		       last_updated_at
		FROM credit_scores
		WHERE shop_id = $1 AND score < $2
		ORDER BY score ASC
	`

	return r.scanScores(ctx, query, shopID, minScore)
}

// Update met à jour un score
func (r *CreditScoreRepositoryInfrastructure) Update(ctx context.Context, score *entity.CreditScore) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le score appartient à la boutique
	var scoreShopID string
	err = r.queryRowContext(ctx, `SELECT shop_id FROM credit_scores WHERE id = $1`, score.ID).Scan(&scoreShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("credit score not found")
		}
		return fmt.Errorf("failed to verify score: %w", err)
	}
	if scoreShopID != shopID {
		return fmt.Errorf("access denied: score does not belong to tenant shop")
	}

	query := `
		UPDATE credit_scores
		SET score = $2,
		    total_contracts = $3,
		    completed_contracts = $4,
		    on_time_payments = $5,
		    late_payments = $6,
		    defaults = $7,
		    last_updated_at = NOW()
		WHERE id = $1
		RETURNING last_updated_at
	`

	err = r.queryRowContext(ctx, query,
		score.ID,
		score.Score,
		score.TotalContracts,
		score.CompletedContracts,
		score.OnTimePayments,
		score.LatePayments,
		score.Defaults,
	).Scan(&score.LastUpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update credit score: %w", err)
	}

	return nil
}

// Upsert crée ou met à jour un score
func (r *CreditScoreRepositoryInfrastructure) Upsert(ctx context.Context, score *entity.CreditScore) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le score appartient à la boutique
	if score.ShopID != shopID {
		return fmt.Errorf("access denied: score shop_id does not match tenant shop")
	}

	query := `
		INSERT INTO credit_scores (
			customer_id, shop_id, score,
			total_contracts, completed_contracts,
			on_time_payments, late_payments, defaults,
			last_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (customer_id, shop_id) DO UPDATE SET
			score = EXCLUDED.score,
			total_contracts = EXCLUDED.total_contracts,
			completed_contracts = EXCLUDED.completed_contracts,
			on_time_payments = EXCLUDED.on_time_payments,
			late_payments = EXCLUDED.late_payments,
			defaults = EXCLUDED.defaults,
			last_updated_at = NOW()
		RETURNING id, last_updated_at
	`

	err = r.queryRowContext(ctx, query,
		score.CustomerID,
		score.ShopID,
		score.Score,
		score.TotalContracts,
		score.CompletedContracts,
		score.OnTimePayments,
		score.LatePayments,
		score.Defaults,
	).Scan(&score.ID, &score.LastUpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert credit score: %w", err)
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si un score existe pour un client/boutique
func (r *CreditScoreRepositoryInfrastructure) Exists(ctx context.Context, customerID, shopID string) (bool, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}
	if shopID != currentShopID {
		return false, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT COUNT(*) FROM credit_scores
		WHERE customer_id = $1 AND shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check score existence: %w", err)
	}

	return count > 0, nil
}

// GetScoreValue retourne uniquement la valeur du score
func (r *CreditScoreRepositoryInfrastructure) GetScoreValue(ctx context.Context, customerID, shopID string) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT score FROM credit_scores
		WHERE customer_id = $1 AND shop_id = $2
	`

	var score int
	err = r.queryRowContext(ctx, query, customerID, shopID).Scan(&score)
	if err != nil {
		if err == sql.ErrNoRows {
			return 500, nil // Score par défaut si pas de score
		}
		return 0, fmt.Errorf("failed to get score value: %w", err)
	}

	return score, nil
}

// GetOrCreateScore retourne le score existant ou crée un score par défaut
func (r *CreditScoreRepositoryInfrastructure) GetOrCreateScore(ctx context.Context, customerID, shopID string) (*entity.CreditScore, error) {
	score, err := r.FindByCustomerAndShop(ctx, customerID, shopID)
	if err == nil {
		return score, nil
	}

	// Score par défaut
	newScore := entity.NewCreditScore(customerID, shopID)
	if err := r.Create(ctx, newScore); err != nil {
		return nil, fmt.Errorf("failed to create default score: %w", err)
	}

	return newScore, nil
}

// CountByShopID compte les scores d'une boutique
func (r *CreditScoreRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID string) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot count scores of another shop")
	}

	query := `SELECT COUNT(*) FROM credit_scores WHERE shop_id = $1`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count credit scores: %w", err)
	}

	return count, nil
}

// AverageScoreByShopID retourne le score moyen d'une boutique
func (r *CreditScoreRepositoryInfrastructure) AverageScoreByShopID(ctx context.Context, shopID string) (float64, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `SELECT COALESCE(AVG(score), 0) FROM credit_scores WHERE shop_id = $1`

	var avg float64
	err = r.queryRowContext(ctx, query, shopID).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate average score: %w", err)
	}

	return avg, nil
}

// CountByScoreRange compte les scores dans une plage
func (r *CreditScoreRepositoryInfrastructure) CountByScoreRange(ctx context.Context, shopID string, minScore, maxScore int) (int, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT COUNT(*) FROM credit_scores
		WHERE shop_id = $1 AND score >= $2 AND score <= $3
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID, minScore, maxScore).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count scores in range: %w", err)
	}

	return count, nil
}

// FindExcellentScores retourne les scores excellents (≥700)
func (r *CreditScoreRepositoryInfrastructure) FindExcellentScores(ctx context.Context, shopID string) ([]*entity.CreditScore, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT id, customer_id, shop_id, score,
		       total_contracts, completed_contracts,
		       on_time_payments, late_payments, defaults,
		       last_updated_at
		FROM credit_scores
		WHERE shop_id = $1 AND score >= 700
		ORDER BY score DESC
	`

	return r.scanScores(ctx, query, shopID)
}

// FindBadScores retourne les scores mauvais (<300)
func (r *CreditScoreRepositoryInfrastructure) FindBadScores(ctx context.Context, shopID string) ([]*entity.CreditScore, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if shopID != currentShopID {
		return nil, fmt.Errorf("access denied: cannot query scores of another shop")
	}

	query := `
		SELECT id, customer_id, shop_id, score,
		       total_contracts, completed_contracts,
		       on_time_payments, late_payments, defaults,
		       last_updated_at
		FROM credit_scores
		WHERE shop_id = $1 AND score < 300
		ORDER BY score ASC
	`

	return r.scanScores(ctx, query, shopID)
}
