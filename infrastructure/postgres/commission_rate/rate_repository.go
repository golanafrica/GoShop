package commissionrate

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

// CommissionRateRepositoryPostgres implémente CommissionRateRepository
type CommissionRateRepositoryPostgres struct {
	db *sql.DB
	tx *sql.Tx
}

// NewCommissionRateRepositoryPostgres crée une nouvelle instance
func NewCommissionRateRepositoryPostgres(db *sql.DB) *CommissionRateRepositoryPostgres {
	return &CommissionRateRepositoryPostgres{db: db}
}

// WithTX retourne une version attachée à une transaction
func (r *CommissionRateRepositoryPostgres) WithTX(tx repository.Tx) repository.CommissionRateRepository {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return r
	}
	return &CommissionRateRepositoryPostgres{
		db: r.db,
		tx: sqlTx,
	}
}

func (r *CommissionRateRepositoryPostgres) executor() interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
} {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// Create crée un nouveau taux
func (r *CommissionRateRepositoryPostgres) Create(ctx context.Context, rate *entity.CommissionRate) error {
	query := `
        INSERT INTO commission_rates (
            id, shop_id, transaction_type, rate_bps,
            min_commission_cents, max_commission_cents,
            is_active, created_by, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
	_, err := r.executor().ExecContext(ctx, query,
		rate.ID, rate.ShopID, rate.TransactionType, rate.RateBps,
		rate.MinCommissionCents, rate.MaxCommissionCents,
		rate.IsActive, rate.CreatedBy, rate.CreatedAt, rate.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create commission rate: %w", err)
	}
	return nil
}

// Update met à jour un taux
func (r *CommissionRateRepositoryPostgres) Update(ctx context.Context, rate *entity.CommissionRate) error {
	query := `
        UPDATE commission_rates SET
            rate_bps = $1,
            min_commission_cents = $2,
            max_commission_cents = $3,
            is_active = $4,
            updated_at = $5
        WHERE shop_id = $6 AND transaction_type = $7
    `
	_, err := r.executor().ExecContext(ctx, query,
		rate.RateBps, rate.MinCommissionCents, rate.MaxCommissionCents,
		rate.IsActive, rate.UpdatedAt, rate.ShopID, rate.TransactionType,
	)
	if err != nil {
		return fmt.Errorf("failed to update commission rate: %w", err)
	}
	return nil
}

// FindByShopAndType récupère un taux par boutique et type
func (r *CommissionRateRepositoryPostgres) FindByShopAndType(
	ctx context.Context,
	shopID, transactionType string,
) (*entity.CommissionRate, error) {
	query := `
        SELECT 
            id, shop_id, transaction_type, rate_bps,
            min_commission_cents, max_commission_cents,
            is_active, created_at, updated_at, created_by
        FROM commission_rates
        WHERE shop_id = $1 AND transaction_type = $2
    `
	rate := &entity.CommissionRate{}
	err := r.executor().QueryRowContext(ctx, query, shopID, transactionType).Scan(
		&rate.ID, &rate.ShopID, &rate.TransactionType, &rate.RateBps,
		&rate.MinCommissionCents, &rate.MaxCommissionCents,
		&rate.IsActive, &rate.CreatedAt, &rate.UpdatedAt, &rate.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("commission rate not found for shop %s and type %s", shopID, transactionType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find commission rate: %w", err)
	}
	return rate, nil
}

// FindByShop récupère tous les taux d'une boutique
func (r *CommissionRateRepositoryPostgres) FindByShop(
	ctx context.Context,
	shopID string,
) ([]*entity.CommissionRate, error) {
	query := `
        SELECT 
            id, shop_id, transaction_type, rate_bps,
            min_commission_cents, max_commission_cents,
            is_active, created_at, updated_at, created_by
        FROM commission_rates
        WHERE shop_id = $1
        ORDER BY transaction_type
    `
	rows, err := r.executor().QueryContext(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to query commission rates: %w", err)
	}
	defer rows.Close()

	var rates []*entity.CommissionRate
	for rows.Next() {
		rate := &entity.CommissionRate{}
		err := rows.Scan(
			&rate.ID, &rate.ShopID, &rate.TransactionType, &rate.RateBps,
			&rate.MinCommissionCents, &rate.MaxCommissionCents,
			&rate.IsActive, &rate.CreatedAt, &rate.UpdatedAt, &rate.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan commission rate: %w", err)
		}
		rates = append(rates, rate)
	}
	return rates, nil
}

// GetDefaultRate récupère le taux par défaut (avec fallback sur les constantes)
func (r *CommissionRateRepositoryPostgres) GetDefaultRate(
	ctx context.Context,
	shopID, transactionType string,
) (*entity.CommissionRate, error) {
	rate, err := r.FindByShopAndType(ctx, shopID, transactionType)
	if err == nil {
		return rate, nil
	}

	// Fallback : retourner le taux par défaut selon le type
	var defaultRateBps int
	switch transactionType {
	case entity.TransactionTypeOnlinePayment:
		defaultRateBps = entity.DefaultRateOnlinePayment
	case entity.TransactionTypeCOD:
		defaultRateBps = entity.DefaultRateCOD
	case entity.TransactionTypeTontineSolo:
		defaultRateBps = entity.DefaultRateTontineSolo
	case entity.TransactionTypeTontineGroup:
		defaultRateBps = entity.DefaultRateTontineGroup
	case entity.TransactionTypeCredit:
		defaultRateBps = entity.DefaultRateCredit
	default:
		defaultRateBps = entity.DefaultRateOnlinePayment
	}

	return &entity.CommissionRate{
		ShopID:             shopID,
		TransactionType:    transactionType,
		RateBps:            defaultRateBps,
		MinCommissionCents: 0,
		MaxCommissionCents: 10000000,
		IsActive:           true,
	}, nil
}

// Vérification de l'interface
var _ repository.CommissionRateRepository = (*CommissionRateRepositoryPostgres)(nil)
