package tontine

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// TontineVoucherRepositoryInfrastructure implémente repository.TontineVoucherRepository
type TontineVoucherRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewTontineVoucherRepositoryInfrastructure crée une nouvelle instance
func NewTontineVoucherRepositoryInfrastructure(db *sql.DB) repository.TontineVoucherRepository {
	return &TontineVoucherRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *TontineVoucherRepositoryInfrastructure) WithTX(tx repository.Tx) repository.TontineVoucherRepository {
	return &TontineVoucherRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers
// ============================================================

func (r *TontineVoucherRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *TontineVoucherRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *TontineVoucherRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *TontineVoucherRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanVoucher scanne une ligne dans une entité TontineVoucher
// Colonnes : id, group_id, participant_id, customer_id, product_id, shop_id,
//
//	voucher_code, cycle_number, status, expires_at, redeemed_at,
//	redeemed_by, held_amount_cents, created_at
func (r *TontineVoucherRepositoryInfrastructure) scanVoucher(row *sql.Row) (*entity.TontineVoucher, error) {
	v := &entity.TontineVoucher{}
	var redeemedAt sql.NullTime
	var redeemedBy sql.NullString

	err := row.Scan(
		&v.ID, &v.GroupID, &v.ParticipantID, &v.CustomerID,
		&v.ProductID, &v.ShopID, &v.VoucherCode,
		&v.CycleNumber, &v.Status, &v.ExpiresAt,
		&redeemedAt, &redeemedBy,
		&v.HeldAmountCents, // Phase 5
		&v.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tontine voucher not found")
		}
		return nil, fmt.Errorf("failed to scan voucher: %w", err)
	}

	if redeemedAt.Valid {
		v.RedeemedAt = &redeemedAt.Time
	}
	if redeemedBy.Valid {
		v.RedeemedBy = &redeemedBy.String
	}

	return v, nil
}

// scanVouchers scanne plusieurs lignes
func (r *TontineVoucherRepositoryInfrastructure) scanVouchers(ctx context.Context, query string, args ...interface{}) ([]*entity.TontineVoucher, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query vouchers: %w", err)
	}
	defer rows.Close()

	var vouchers []*entity.TontineVoucher
	for rows.Next() {
		v := &entity.TontineVoucher{}
		var redeemedAt sql.NullTime
		var redeemedBy sql.NullString

		err := rows.Scan(
			&v.ID, &v.GroupID, &v.ParticipantID, &v.CustomerID,
			&v.ProductID, &v.ShopID, &v.VoucherCode,
			&v.CycleNumber, &v.Status, &v.ExpiresAt,
			&redeemedAt, &redeemedBy,
			&v.HeldAmountCents, // Phase 5
			&v.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if redeemedAt.Valid {
			v.RedeemedAt = &redeemedAt.Time
		}
		if redeemedBy.Valid {
			v.RedeemedBy = &redeemedBy.String
		}

		vouchers = append(vouchers, v)
	}

	if vouchers == nil {
		vouchers = []*entity.TontineVoucher{}
	}
	return vouchers, rows.Err()
}

// ============================================================
// Implémentation
// ============================================================

// Create crée un nouveau voucher (inclut held_amount_cents — Phase 5)
func (r *TontineVoucherRepositoryInfrastructure) Create(ctx context.Context, voucher *entity.TontineVoucher) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	if voucher.ShopID != shopID {
		return fmt.Errorf("voucher shop_id does not match tenant shop_id")
	}

	query := `
		INSERT INTO tontine_vouchers (
			group_id, participant_id, customer_id,
			product_id, shop_id, voucher_code,
			cycle_number, status, expires_at,
			held_amount_cents,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		RETURNING id, created_at
	`

	err = r.queryRowContext(ctx, query,
		voucher.GroupID,
		voucher.ParticipantID,
		voucher.CustomerID,
		voucher.ProductID,
		voucher.ShopID,
		voucher.VoucherCode,
		voucher.CycleNumber,
		voucher.Status,
		voucher.ExpiresAt,
		voucher.HeldAmountCents, // Phase 5
	).Scan(&voucher.ID, &voucher.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create voucher: %w", err)
	}

	return nil
}

// FindByID trouve un voucher par son ID
func (r *TontineVoucherRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.TontineVoucher, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanVoucher(r.queryRowContext(ctx, query, id, shopID))
}

// FindByCode trouve un voucher par son code unique
func (r *TontineVoucherRepositoryInfrastructure) FindByCode(ctx context.Context, code string) (*entity.TontineVoucher, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE voucher_code = $1 AND shop_id = $2
	`

	return r.scanVoucher(r.queryRowContext(ctx, query, code, shopID))
}

// FindByGroupID retourne tous les vouchers d'un groupe (filtré par tenant shop_id)
// 🆕 Phase 5 : utilisé par ListVouchers endpoint
func (r *TontineVoucherRepositoryInfrastructure) FindByGroupID(ctx context.Context, groupID string) ([]*entity.TontineVoucher, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE group_id = $1 AND shop_id = $2
		ORDER BY cycle_number ASC
	`

	return r.scanVouchers(ctx, query, groupID, shopID)
}

// FindByParticipantAndCycle trouve le voucher d'un participant pour un cycle
func (r *TontineVoucherRepositoryInfrastructure) FindByParticipantAndCycle(ctx context.Context, participantID string, cycle int) (*entity.TontineVoucher, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE participant_id = $1 AND cycle_number = $2 AND shop_id = $3
	`

	return r.scanVoucher(r.queryRowContext(ctx, query, participantID, cycle, shopID))
}

// FindByCustomerID retourne tous les vouchers d'un client
func (r *TontineVoucherRepositoryInfrastructure) FindByCustomerID(ctx context.Context, customerID string) ([]*entity.TontineVoucher, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE customer_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanVouchers(ctx, query, customerID, shopID)
}

// FindByShopID retourne tous les vouchers d'une boutique
func (r *TontineVoucherRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID string) ([]*entity.TontineVoucher, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if currentShopID != shopID {
		return nil, fmt.Errorf("access denied: shop_id mismatch")
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`

	return r.scanVouchers(ctx, query, shopID)
}

// FindActiveByShopID retourne les vouchers actifs (generated non expirés) d'une boutique
func (r *TontineVoucherRepositoryInfrastructure) FindActiveByShopID(ctx context.Context, shopID string) ([]*entity.TontineVoucher, error) {
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if currentShopID != shopID {
		return nil, fmt.Errorf("access denied: shop_id mismatch")
	}

	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE shop_id = $1 AND status = 'generated' AND expires_at > NOW()
		ORDER BY expires_at ASC
	`

	return r.scanVouchers(ctx, query, shopID)
}

// Redeem marque un voucher comme utilisé
func (r *TontineVoucherRepositoryInfrastructure) Redeem(ctx context.Context, voucherCode string, redeemedBy string) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE tontine_vouchers
		SET status = 'redeemed', redeemed_at = NOW(), redeemed_by = $2
		WHERE voucher_code = $1 AND shop_id = $3 AND status = 'generated' AND expires_at > NOW()
	`
	result, err := r.execContext(ctx, query, voucherCode, redeemedBy, shopID)
	if err != nil {
		return fmt.Errorf("failed to redeem voucher: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("voucher not found, already redeemed, or expired")
	}
	return nil
}

// ExpireOldVouchers est "unscoped" (sans vérification de tenant).
// Permet au cron job global d'expirer les vouchers de TOUTES les boutiques.
func (r *TontineVoucherRepositoryInfrastructure) ExpireOldVouchers(ctx context.Context) (int, error) {
	query := `
		UPDATE tontine_vouchers
		SET status = 'expired'
		WHERE status = 'generated' AND expires_at < $1
	`
	result, err := r.execContext(ctx, query, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("failed to expire old vouchers: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// 🆕 v4.8.3 : FindByIDAdmin sans vérification multi-tenant
// Utilisé par le scheduler escrow auto-release qui tourne en contexte background
func (r *TontineVoucherRepositoryInfrastructure) FindByIDAdmin(ctx context.Context, id string) (*entity.TontineVoucher, error) {
	query := `
		SELECT id, group_id, participant_id, customer_id,
		       product_id, shop_id, voucher_code,
		       cycle_number, status, expires_at,
		       redeemed_at, redeemed_by,
		       held_amount_cents,
		       created_at
		FROM tontine_vouchers
		WHERE id = $1
	`

	return r.scanVoucher(r.queryRowContext(ctx, query, id))
}
