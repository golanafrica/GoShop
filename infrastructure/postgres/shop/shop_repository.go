package shop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/infrastructure/crypto"

	"github.com/google/uuid"
)

type ShopRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

func NewShopRepositoryInfrastructure(db *sql.DB) repository.ShopRepository {
	return &ShopRepositoryInfrastructure{db: db}
}

func (r *ShopRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ShopRepository {
	return &ShopRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *ShopRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *ShopRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *ShopRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// ============================================================
// 🆕 v4.2.0 : CONSTANTES SQL (évite duplication)
// ============================================================

// shopColumnsFull contient toutes les colonnes pour SELECT
const shopColumnsFull = `
	id, name, slug, custom_domain, owner_id, logo_url, theme, plan, db_schema, is_active,
	kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
	kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
	suspended_at, suspended_by, suspension_reason,
	health_score, health_level, health_updated_at,
	admin_notes, last_reviewed_at, last_reviewed_by,
	created_at, updated_at
`

// ============================================================
// CREATE
// ============================================================

func (r *ShopRepositoryInfrastructure) Create(ctx context.Context, shop *entity.Shop) error {
	query := `
		INSERT INTO shops (
			id, name, slug, custom_domain, owner_id, logo_url, theme, plan, db_schema, is_active,
			kyc_status, kyc_submissions_count,
			health_score, health_level,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at, updated_at
	`

	themeJSON, err := json.Marshal(shop.Theme)
	if err != nil {
		return err
	}

	err = r.queryRowContext(ctx, query,
		shop.ID,
		shop.Name,
		shop.Slug,
		shop.CustomDomain,
		shop.OwnerID,
		shop.LogoURL,
		themeJSON,
		shop.Plan,
		shop.DBSchema,
		shop.IsActive,
		shop.KYCStatus,
		shop.KYCSubmissionsCount,
		shop.HealthScore,
		shop.HealthLevel,
		shop.CreatedAt,
		shop.UpdatedAt,
	).Scan(&shop.CreatedAt, &shop.UpdatedAt)

	return err
}

// ============================================================
// FIND METHODS
// ============================================================

func (r *ShopRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.Shop, error) {
	query := fmt.Sprintf("SELECT %s FROM shops WHERE id = $1", shopColumnsFull)
	return r.scanShopFull(r.queryRowContext(ctx, query, id))
}

func (r *ShopRepositoryInfrastructure) FindBySlug(ctx context.Context, slug string) (*entity.Shop, error) {
	query := fmt.Sprintf("SELECT %s FROM shops WHERE slug = $1", shopColumnsFull)
	return r.scanShopFull(r.queryRowContext(ctx, query, slug))
}

func (r *ShopRepositoryInfrastructure) FindByCustomDomain(ctx context.Context, domain string) (*entity.Shop, error) {
	query := fmt.Sprintf("SELECT %s FROM shops WHERE custom_domain = $1", shopColumnsFull)
	return r.scanShopFull(r.queryRowContext(ctx, query, domain))
}

func (r *ShopRepositoryInfrastructure) FindByOwnerID(ctx context.Context, ownerID string) ([]*entity.Shop, error) {
	query := fmt.Sprintf("SELECT %s FROM shops WHERE owner_id = $1 ORDER BY created_at DESC", shopColumnsFull)

	rows, err := r.queryContext(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFullFromRows(rows)
		if err != nil {
			return nil, err
		}
		shops = append(shops, shop)
	}

	return shops, rows.Err()
}

// ============================================================
// UPDATE
// ============================================================

func (r *ShopRepositoryInfrastructure) Update(ctx context.Context, shop *entity.Shop) error {
	query := `
		UPDATE shops
		SET name = $2, slug = $3, custom_domain = $4, logo_url = $5,
		    theme = $6, plan = $7, db_schema = $8, is_active = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	themeJSON, err := json.Marshal(shop.Theme)
	if err != nil {
		return err
	}

	err = r.queryRowContext(ctx, query,
		shop.ID,
		shop.Name,
		shop.Slug,
		shop.CustomDomain,
		shop.LogoURL,
		themeJSON,
		shop.Plan,
		shop.DBSchema,
		shop.IsActive,
	).Scan(&shop.UpdatedAt)

	return err
}

func (r *ShopRepositoryInfrastructure) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE shops SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.execContext(ctx, query, id)
	return err
}

// ============================================================
// 🆕 v4.2.0 : SCAN HELPERS COMPLETS (KYC + Admin)
// ============================================================

// scanShopFull scanne une ligne complète depuis sql.Row
func (r *ShopRepositoryInfrastructure) scanShopFull(row *sql.Row) (*entity.Shop, error) {
	shop := &entity.Shop{}
	var themeJSON []byte

	// Champs KYC nullable
	var kycSubmittedAt, kycVerifiedAt, kycLastSubmissionAt sql.NullTime
	var kycVerifiedBy, kycRejectionReason sql.NullString

	// 🆕 v4.2.0 : Champs Admin nullable
	var suspendedAt, healthUpdatedAt, lastReviewedAt sql.NullTime
	var suspendedBy, suspensionReason, adminNotes, lastReviewedBy sql.NullString

	err := row.Scan(
		&shop.ID,
		&shop.Name,
		&shop.Slug,
		&shop.CustomDomain,
		&shop.OwnerID,
		&shop.LogoURL,
		&themeJSON,
		&shop.Plan,
		&shop.DBSchema,
		&shop.IsActive,
		// KYC
		&shop.KYCStatus,
		&kycSubmittedAt,
		&kycVerifiedAt,
		&kycVerifiedBy,
		&kycRejectionReason,
		&shop.KYCSubmissionsCount,
		&kycLastSubmissionAt,
		// 🆕 v4.2.0 : Admin
		&suspendedAt,
		&suspendedBy,
		&suspensionReason,
		&shop.HealthScore,
		&shop.HealthLevel,
		&healthUpdatedAt,
		&adminNotes,
		&lastReviewedAt,
		&lastReviewedBy,
		&shop.CreatedAt,
		&shop.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Gestion des champs nullable KYC
	if kycSubmittedAt.Valid {
		shop.KYCSubmittedAt = &kycSubmittedAt.Time
	}
	if kycVerifiedAt.Valid {
		shop.KYCVerifiedAt = &kycVerifiedAt.Time
	}
	if kycVerifiedBy.Valid {
		shop.KYCVerifiedBy = &kycVerifiedBy.String
	}
	if kycRejectionReason.Valid {
		shop.KYCRejectionReason = &kycRejectionReason.String
	}
	if kycLastSubmissionAt.Valid {
		shop.KYCLastSubmissionAt = &kycLastSubmissionAt.Time
	}

	// 🆕 v4.2.0 : Gestion des champs nullable Admin
	if suspendedAt.Valid {
		shop.SuspendedAt = &suspendedAt.Time
	}
	if suspendedBy.Valid {
		shop.SuspendedBy = &suspendedBy.String
	}
	if suspensionReason.Valid {
		shop.SuspensionReason = &suspensionReason.String
	}
	if healthUpdatedAt.Valid {
		shop.HealthUpdatedAt = &healthUpdatedAt.Time
	}
	if adminNotes.Valid {
		shop.AdminNotes = &adminNotes.String
	}
	if lastReviewedAt.Valid {
		shop.LastReviewedAt = &lastReviewedAt.Time
	}
	if lastReviewedBy.Valid {
		shop.LastReviewedBy = &lastReviewedBy.String
	}

	if len(themeJSON) > 0 {
		if err := json.Unmarshal(themeJSON, &shop.Theme); err != nil {
			return nil, err
		}
	} else {
		shop.Theme = make(map[string]interface{})
	}

	return shop, nil
}

// scanShopFullFromRows scanne une ligne complète depuis sql.Rows
func (r *ShopRepositoryInfrastructure) scanShopFullFromRows(rows *sql.Rows) (*entity.Shop, error) {
	shop := &entity.Shop{}
	var themeJSON []byte

	var kycSubmittedAt, kycVerifiedAt, kycLastSubmissionAt sql.NullTime
	var kycVerifiedBy, kycRejectionReason sql.NullString

	var suspendedAt, healthUpdatedAt, lastReviewedAt sql.NullTime
	var suspendedBy, suspensionReason, adminNotes, lastReviewedBy sql.NullString

	err := rows.Scan(
		&shop.ID,
		&shop.Name,
		&shop.Slug,
		&shop.CustomDomain,
		&shop.OwnerID,
		&shop.LogoURL,
		&themeJSON,
		&shop.Plan,
		&shop.DBSchema,
		&shop.IsActive,
		&shop.KYCStatus,
		&kycSubmittedAt,
		&kycVerifiedAt,
		&kycVerifiedBy,
		&kycRejectionReason,
		&shop.KYCSubmissionsCount,
		&kycLastSubmissionAt,
		&suspendedAt,
		&suspendedBy,
		&suspensionReason,
		&shop.HealthScore,
		&shop.HealthLevel,
		&healthUpdatedAt,
		&adminNotes,
		&lastReviewedAt,
		&lastReviewedBy,
		&shop.CreatedAt,
		&shop.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if kycSubmittedAt.Valid {
		shop.KYCSubmittedAt = &kycSubmittedAt.Time
	}
	if kycVerifiedAt.Valid {
		shop.KYCVerifiedAt = &kycVerifiedAt.Time
	}
	if kycVerifiedBy.Valid {
		shop.KYCVerifiedBy = &kycVerifiedBy.String
	}
	if kycRejectionReason.Valid {
		shop.KYCRejectionReason = &kycRejectionReason.String
	}
	if kycLastSubmissionAt.Valid {
		shop.KYCLastSubmissionAt = &kycLastSubmissionAt.Time
	}

	if suspendedAt.Valid {
		shop.SuspendedAt = &suspendedAt.Time
	}
	if suspendedBy.Valid {
		shop.SuspendedBy = &suspendedBy.String
	}
	if suspensionReason.Valid {
		shop.SuspensionReason = &suspensionReason.String
	}
	if healthUpdatedAt.Valid {
		shop.HealthUpdatedAt = &healthUpdatedAt.Time
	}
	if adminNotes.Valid {
		shop.AdminNotes = &adminNotes.String
	}
	if lastReviewedAt.Valid {
		shop.LastReviewedAt = &lastReviewedAt.Time
	}
	if lastReviewedBy.Valid {
		shop.LastReviewedBy = &lastReviewedBy.String
	}

	if len(themeJSON) > 0 {
		if err := json.Unmarshal(themeJSON, &shop.Theme); err != nil {
			return nil, err
		}
	} else {
		shop.Theme = make(map[string]interface{})
	}

	return shop, nil
}

// ============================================================
// CONFIG PAR BOUTIQUE (v2.9.0: + Cash COD + Tontine)
// ============================================================

func (r *ShopRepositoryInfrastructure) GetPaymentSettings(ctx context.Context, shopID uuid.UUID) (*entity.ShopPaymentSettings, error) {
	query := `
		SELECT 
			shop_id,
			COALESCE(orange_money_enabled, false),
			COALESCE(moov_money_enabled, false),
			COALESCE(wave_enabled, false),
			COALESCE(yenga_pay_enabled, false),
			COALESCE(yenga_pay_api_key, ''),
			COALESCE(yenga_pay_organization_id, ''),
			COALESCE(yenga_pay_project_id, ''),
			COALESCE(yenga_pay_webhook_secret, ''),
			COALESCE(yenga_pay_operators, '["orange_money","moov_money","telecel","coris_money","sank_money"]'::jsonb),
			COALESCE(yenga_pay_env, 'test'),
			COALESCE(cash_on_delivery_enabled, false),
			COALESCE(cash_commission_rate, 250),
			COALESCE(tontine_enabled, false),
			COALESCE(tontine_commission_rate, 250)
		FROM shop_payment_settings
		WHERE shop_id = $1
	`

	var settings entity.ShopPaymentSettings
	var yengaAPIKey, yengaOrgID, yengaProjectID, yengaWebhookSecret string
	var yengaOperatorsJSON []byte

	err := r.db.QueryRowContext(ctx, query, shopID).Scan(
		&settings.ShopID,
		&settings.OrangeMoney,
		&settings.MoovMoney,
		&settings.Wave,
		&settings.YengaPay.Enabled,
		&yengaAPIKey,
		&yengaOrgID,
		&yengaProjectID,
		&yengaWebhookSecret,
		&yengaOperatorsJSON,
		&settings.YengaPay.Env,
		&settings.CashOnDeliveryEnabled,
		&settings.CashCommissionRate,
		&settings.TontineEnabled,
		&settings.TontineCommissionRate,
	)

	if err == sql.ErrNoRows {
		return &entity.ShopPaymentSettings{
			ShopID: shopID,
			YengaPay: entity.YengaPayShopSettings{
				Enabled:   false,
				Operators: []string{"orange_money", "moov_money", "telecel", "coris_money", "sank_money"},
				Env:       "test",
			},
			CashOnDeliveryEnabled: false,
			CashCommissionRate:    250,
			TontineEnabled:        false,
			TontineCommissionRate: 250,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query payment settings: %w", err)
	}

	if yengaAPIKey != "" {
		settings.YengaPay.APIKey, err = crypto.DecryptOrEmpty(yengaAPIKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt yenga api key: %w", err)
		}
	}
	if yengaOrgID != "" {
		settings.YengaPay.OrganizationID, err = crypto.DecryptOrEmpty(yengaOrgID)
		if err != nil {
			return nil, fmt.Errorf("decrypt yenga org id: %w", err)
		}
	}
	if yengaProjectID != "" {
		settings.YengaPay.ProjectID, err = crypto.DecryptOrEmpty(yengaProjectID)
		if err != nil {
			return nil, fmt.Errorf("decrypt yenga project id: %w", err)
		}
	}
	if yengaWebhookSecret != "" {
		settings.YengaPay.WebhookSecret, err = crypto.DecryptOrEmpty(yengaWebhookSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt yenga webhook secret: %w", err)
		}
	}

	if len(yengaOperatorsJSON) > 0 {
		if err := json.Unmarshal(yengaOperatorsJSON, &settings.YengaPay.Operators); err != nil {
			return nil, fmt.Errorf("parse yenga operators: %w", err)
		}
	}

	return &settings, nil
}

func (r *ShopRepositoryInfrastructure) UpsertPaymentSettings(ctx context.Context, settings *entity.ShopPaymentSettings) error {
	yengaAPIKey, err := crypto.EncryptOrEmpty(settings.YengaPay.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt yenga api key: %w", err)
	}
	yengaOrgID, err := crypto.EncryptOrEmpty(settings.YengaPay.OrganizationID)
	if err != nil {
		return fmt.Errorf("encrypt yenga org id: %w", err)
	}
	yengaProjectID, err := crypto.EncryptOrEmpty(settings.YengaPay.ProjectID)
	if err != nil {
		return fmt.Errorf("encrypt yenga project id: %w", err)
	}
	yengaWebhookSecret, err := crypto.EncryptOrEmpty(settings.YengaPay.WebhookSecret)
	if err != nil {
		return fmt.Errorf("encrypt yenga webhook secret: %w", err)
	}

	operatorsJSON, err := json.Marshal(settings.YengaPay.Operators)
	if err != nil {
		return fmt.Errorf("serialize yenga operators: %w", err)
	}

	query := `
		INSERT INTO shop_payment_settings (
			shop_id,
			orange_money_enabled, moov_money_enabled, wave_enabled,
			yenga_pay_enabled, yenga_pay_api_key, yenga_pay_organization_id,
			yenga_pay_project_id, yenga_pay_webhook_secret, yenga_pay_operators, yenga_pay_env,
			cash_on_delivery_enabled, cash_commission_rate,
			tontine_enabled, tontine_commission_rate
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (shop_id) DO UPDATE SET
			orange_money_enabled = EXCLUDED.orange_money_enabled,
			moov_money_enabled = EXCLUDED.moov_money_enabled,
			wave_enabled = EXCLUDED.wave_enabled,
			yenga_pay_enabled = EXCLUDED.yenga_pay_enabled,
			yenga_pay_api_key = EXCLUDED.yenga_pay_api_key,
			yenga_pay_organization_id = EXCLUDED.yenga_pay_organization_id,
			yenga_pay_project_id = EXCLUDED.yenga_pay_project_id,
			yenga_pay_webhook_secret = EXCLUDED.yenga_pay_webhook_secret,
			yenga_pay_operators = EXCLUDED.yenga_pay_operators,
			yenga_pay_env = EXCLUDED.yenga_pay_env,
			cash_on_delivery_enabled = EXCLUDED.cash_on_delivery_enabled,
			cash_commission_rate = EXCLUDED.cash_commission_rate,
			tontine_enabled = EXCLUDED.tontine_enabled,
			tontine_commission_rate = EXCLUDED.tontine_commission_rate,
			updated_at = NOW()
	`

	_, err = r.db.ExecContext(ctx, query,
		settings.ShopID,
		settings.OrangeMoney,
		settings.MoovMoney,
		settings.Wave,
		settings.YengaPay.Enabled,
		yengaAPIKey,
		yengaOrgID,
		yengaProjectID,
		yengaWebhookSecret,
		operatorsJSON,
		settings.YengaPay.Env,
		settings.CashOnDeliveryEnabled,
		settings.CashCommissionRate,
		settings.TontineEnabled,
		settings.TontineCommissionRate,
	)

	if err != nil {
		return fmt.Errorf("upsert payment settings: %w", err)
	}

	return nil
}

func (r *ShopRepositoryInfrastructure) IsOwner(ctx context.Context, shopID uuid.UUID, userID uuid.UUID) (bool, error) {
	var ownerID uuid.UUID
	query := `SELECT owner_id FROM shops WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, shopID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check ownership: %w", err)
	}

	return ownerID == userID, nil
}

// ============================================================
// 🆕 v4.1.0 : MÉTHODES KYC MARCHAND
// ============================================================

func (r *ShopRepositoryInfrastructure) FindByKYCStatus(
	ctx context.Context,
	status entity.ShopKYCStatus,
	limit, offset int,
) ([]*entity.Shop, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM shops
		WHERE kyc_status = $1
		ORDER BY kyc_submitted_at DESC NULLS LAST, created_at DESC
		LIMIT $2 OFFSET $3
	`, shopColumnsFull)

	rows, err := r.queryContext(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find by kyc status: %w", err)
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFullFromRows(rows)
		if err != nil {
			return nil, err
		}
		shops = append(shops, shop)
	}

	return shops, rows.Err()
}

func (r *ShopRepositoryInfrastructure) CountByKYCStatus(ctx context.Context) (map[entity.ShopKYCStatus]int, error) {
	query := `
		SELECT kyc_status, COUNT(*)
		FROM shops
		GROUP BY kyc_status
	`

	rows, err := r.queryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by kyc status: %w", err)
	}
	defer rows.Close()

	counts := make(map[entity.ShopKYCStatus]int)
	for rows.Next() {
		var status entity.ShopKYCStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}

	return counts, rows.Err()
}

func (r *ShopRepositoryInfrastructure) UpdateKYCStatus(
	ctx context.Context,
	shopID uuid.UUID,
	status entity.ShopKYCStatus,
	adminID string,
	rejectionReason *string,
) error {
	var query string
	var args []interface{}

	now := time.Now()

	switch status {
	case entity.ShopKYCStatusVerified:
		query = `
			UPDATE shops
			SET kyc_status = $2,
			    kyc_verified_at = $3,
			    kyc_verified_by = $4,
			    kyc_rejection_reason = NULL,
			    updated_at = NOW()
			WHERE id = $1
		`
		args = []interface{}{shopID, status, now, adminID}

	case entity.ShopKYCStatusRejected:
		query = `
			UPDATE shops
			SET kyc_status = $2,
			    kyc_verified_by = $3,
			    kyc_rejection_reason = $4,
			    updated_at = NOW()
			WHERE id = $1
		`
		args = []interface{}{shopID, status, adminID, rejectionReason}

	case entity.ShopKYCStatusPending:
		query = `
			UPDATE shops
			SET kyc_status = $2,
			    kyc_submitted_at = $3,
			    kyc_last_submission_at = $3,
			    kyc_submissions_count = kyc_submissions_count + 1,
			    kyc_rejection_reason = NULL,
			    updated_at = NOW()
			WHERE id = $1
		`
		args = []interface{}{shopID, status, now}

	default:
		query = `
			UPDATE shops
			SET kyc_status = $2,
			    updated_at = NOW()
			WHERE id = $1
		`
		args = []interface{}{shopID, status}
	}

	_, err := r.execContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update kyc status: %w", err)
	}

	return nil
}

// ============================================================
// 🆕 v4.2.0 : MÉTHODES ADMIN SHOP MANAGEMENT
// ============================================================

// FindAllShopsAdmin retourne tous les shops avec filtres avancés
func (r *ShopRepositoryInfrastructure) FindAllShopsAdmin(
	ctx context.Context,
	filters *repository.ShopAdminFilters,
) ([]*entity.Shop, int, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIndex := 1

	if filters != nil {
		if filters.KYCStatus != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("kyc_status = $%d", argIndex))
			args = append(args, *filters.KYCStatus)
			argIndex++
		}
		if filters.Plan != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("plan = $%d", argIndex))
			args = append(args, *filters.Plan)
			argIndex++
		}
		if filters.IsActive != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("is_active = $%d", argIndex))
			args = append(args, *filters.IsActive)
			argIndex++
		}
		if filters.IsSuspended != nil {
			if *filters.IsSuspended {
				whereClauses = append(whereClauses, "suspended_at IS NOT NULL")
			} else {
				whereClauses = append(whereClauses, "suspended_at IS NULL")
			}
		}
		if filters.HealthLevel != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("health_level = $%d", argIndex))
			args = append(args, *filters.HealthLevel)
			argIndex++
		}
		if filters.MinHealthScore != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("health_score >= $%d", argIndex))
			args = append(args, *filters.MinHealthScore)
			argIndex++
		}
		if filters.MaxHealthScore != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("health_score <= $%d", argIndex))
			args = append(args, *filters.MaxHealthScore)
			argIndex++
		}
		if filters.CreatedAfter != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIndex))
			args = append(args, *filters.CreatedAfter)
			argIndex++
		}
		if filters.CreatedBefore != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIndex))
			args = append(args, *filters.CreatedBefore)
			argIndex++
		}
		if filters.Search != "" {
			whereClauses = append(whereClauses, fmt.Sprintf(
				"(name ILIKE $%d OR slug ILIKE $%d OR owner_id ILIKE $%d)",
				argIndex, argIndex, argIndex,
			))
			args = append(args, "%"+filters.Search+"%")
			argIndex++
		}
	}

	whereClause := strings.Join(whereClauses, " AND ")

	// Compter le total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM shops WHERE %s", whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count shops: %w", err)
	}

	// 🛡️ SÉCURITÉ CRITIQUE : Whitelist des colonnes autorisées pour le tri (prévention injection SQL)
	allowedSortColumns := map[string]bool{
		"name":         true,
		"slug":         true,
		"created_at":   true,
		"updated_at":   true,
		"health_score": true,
		"health_level": true,
		"kyc_status":   true,
		"plan":         true,
		"is_active":    true,
		"suspended_at": true,
	}

	// Validation stricte de SortBy
	sortBy := "created_at" // Valeur par défaut sûre
	if filters != nil && filters.SortBy != "" {
		if allowedSortColumns[filters.SortBy] {
			sortBy = filters.SortBy
		}
	}

	// Validation stricte de SortOrder
	sortOrder := "DESC" // Valeur par défaut sûre
	if filters != nil && filters.SortOrder != "" {
		upperOrder := strings.ToUpper(filters.SortOrder)
		if upperOrder == "ASC" || upperOrder == "DESC" {
			sortOrder = upperOrder
		}
	}

	// Pagination par défaut
	limit := 20
	offset := 0
	if filters != nil {
		if filters.Limit > 0 {
			limit = filters.Limit
		}
		if filters.Offset > 0 {
			offset = filters.Offset
		}
	}

	// Récupérer les shops (sortBy et sortOrder sont maintenant 100% sûrs)
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT %s
		FROM shops
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, shopColumnsFull, whereClause, sortBy, sortOrder, argIndex, argIndex+1)

	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("find all shops admin: %w", err)
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFullFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		shops = append(shops, shop)
	}

	return shops, total, rows.Err()
}

// UpdateShopStatus active ou désactive une boutique (admin)
func (r *ShopRepositoryInfrastructure) UpdateShopStatus(
	ctx context.Context,
	shopID uuid.UUID,
	isActive bool,
	adminID string,
) error {
	query := `
		UPDATE shops
		SET is_active = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query, shopID, isActive)
	if err != nil {
		return fmt.Errorf("update shop status: %w", err)
	}
	return nil
}

// UpdateShopPlan change le plan d'abonnement (super_admin uniquement)
func (r *ShopRepositoryInfrastructure) UpdateShopPlan(
	ctx context.Context,
	shopID uuid.UUID,
	plan entity.ShopPlan,
	adminID string,
) error {
	query := `
		UPDATE shops
		SET plan = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query, shopID, plan)
	if err != nil {
		return fmt.Errorf("update shop plan: %w", err)
	}
	return nil
}

// SuspendShop suspend une boutique avec raison
func (r *ShopRepositoryInfrastructure) SuspendShop(
	ctx context.Context,
	shopID uuid.UUID,
	adminID, reason string,
) error {
	now := time.Now()
	query := `
		UPDATE shops
		SET suspended_at = $2,
		    suspended_by = $3,
		    suspension_reason = $4,
		    is_active = false,
		    updated_at = NOW()
		WHERE id = $1 AND suspended_at IS NULL
	`
	result, err := r.execContext(ctx, query, shopID, now, adminID, reason)
	if err != nil {
		return fmt.Errorf("suspend shop: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrShopAlreadySuspended
	}

	return nil
}

// ActivateShop réactive une boutique suspendue
func (r *ShopRepositoryInfrastructure) ActivateShop(
	ctx context.Context,
	shopID uuid.UUID,
	adminID string,
) error {
	query := `
		UPDATE shops
		SET suspended_at = NULL,
		    suspended_by = NULL,
		    suspension_reason = NULL,
		    is_active = true,
		    updated_at = NOW()
		WHERE id = $1 AND suspended_at IS NOT NULL
	`
	result, err := r.execContext(ctx, query, shopID)
	if err != nil {
		return fmt.Errorf("activate shop: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return entity.ErrShopNotSuspended
	}

	return nil
}

// UpdateHealthScore met à jour le score de santé d'un shop
func (r *ShopRepositoryInfrastructure) UpdateHealthScore(
	ctx context.Context,
	shopID uuid.UUID,
	score int,
	level entity.ShopHealthLevel,
) error {
	// Validation
	if score < 0 || score > 1000 {
		return entity.ErrInvalidHealthScore
	}
	if !entity.IsValidHealthLevel(level) {
		return entity.ErrInvalidHealthLevel
	}

	query := `
		UPDATE shops
		SET health_score = $2,
		    health_level = $3,
		    health_updated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query, shopID, score, level)
	if err != nil {
		return fmt.Errorf("update health score: %w", err)
	}

	return nil
}

// AddAdminNote ajoute une note admin à un shop
func (r *ShopRepositoryInfrastructure) AddAdminNote(
	ctx context.Context,
	shopID uuid.UUID,
	note string,
	adminID string,
) error {
	now := time.Now()
	formattedNote := fmt.Sprintf("[%s - %s]\n%s", now.Format("2006-01-02 15:04"), adminID, note)

	query := `
		UPDATE shops
		SET admin_notes = CASE 
			WHEN admin_notes IS NULL OR admin_notes = '' THEN $2
			ELSE admin_notes || E'\n\n' || $2
		END,
		updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query, shopID, formattedNote)
	if err != nil {
		return fmt.Errorf("add admin note: %w", err)
	}

	return nil
}

// MarkShopReviewed marque un shop comme revu par un admin
func (r *ShopRepositoryInfrastructure) MarkShopReviewed(
	ctx context.Context,
	shopID uuid.UUID,
	adminID string,
) error {
	now := time.Now()
	query := `
		UPDATE shops
		SET last_reviewed_at = $2,
		    last_reviewed_by = $3,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query, shopID, now, adminID)
	if err != nil {
		return fmt.Errorf("mark shop reviewed: %w", err)
	}

	return nil
}

// ============================================================
// 🆕 v4.2.0 : MÉTHODES DASHBOARD ADMIN
// ============================================================

// GetHealthStats retourne les statistiques de santé globales
func (r *ShopRepositoryInfrastructure) GetHealthStats(ctx context.Context) (*repository.ShopHealthStats, error) {
	stats := &repository.ShopHealthStats{}

	// Statistiques de base
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE is_active = true) as active,
			COUNT(*) FILTER (WHERE is_active = false) as inactive,
			COUNT(*) FILTER (WHERE suspended_at IS NOT NULL) as suspended,
			COUNT(*) FILTER (WHERE kyc_status = 'pending') as pending_kyc,
			COUNT(*) FILTER (WHERE health_level = 'excellent') as excellent,
			COUNT(*) FILTER (WHERE health_level = 'good') as good,
			COUNT(*) FILTER (WHERE health_level = 'warning') as warning,
			COUNT(*) FILTER (WHERE health_level = 'critical') as critical,
			COALESCE(AVG(health_score), 0) as avg_score,
			COALESCE(MIN(health_score), 0) as min_score,
			COALESCE(MAX(health_score), 0) as max_score,
			COUNT(*) FILTER (WHERE plan = 'free') as free,
			COUNT(*) FILTER (WHERE plan = 'pro') as pro,
			COUNT(*) FILTER (WHERE plan = 'business') as business
		FROM shops
	`

	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalShops,
		&stats.ActiveShops,
		&stats.InactiveShops,
		&stats.SuspendedShops,
		&stats.PendingKYC,
		&stats.ExcellentCount,
		&stats.GoodCount,
		&stats.WarningCount,
		&stats.CriticalCount,
		&stats.AverageScore,
		&stats.MinScore,
		&stats.MaxScore,
		&stats.FreeCount,
		&stats.ProCount,
		&stats.BusinessCount,
	)
	if err != nil {
		return nil, fmt.Errorf("get health stats: %w", err)
	}

	// Actions admin récentes
	actionsQuery := `
		SELECT 
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours') as last_24h,
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '7 days') as last_7d
		FROM shop_admin_actions
	`
	err = r.db.QueryRowContext(ctx, actionsQuery).Scan(
		&stats.ActionsLast24h,
		&stats.ActionsLast7d,
	)
	if err != nil {
		// Non bloquant si la table n'existe pas encore
		stats.ActionsLast24h = 0
		stats.ActionsLast7d = 0
	}

	return stats, nil
}

// GetSuspendedShops retourne la liste des shops suspendus
func (r *ShopRepositoryInfrastructure) GetSuspendedShops(
	ctx context.Context,
	limit, offset int,
) ([]*entity.Shop, int, error) {
	filters := &repository.ShopAdminFilters{
		IsSuspended: boolPtr(true),
		Limit:       limit,
		Offset:      offset,
		SortBy:      "suspended_at",
		SortOrder:   "DESC",
	}
	return r.FindAllShopsAdmin(ctx, filters)
}

// GetCriticalShops retourne la liste des shops critiques (score < 400)
func (r *ShopRepositoryInfrastructure) GetCriticalShops(
	ctx context.Context,
	limit, offset int,
) ([]*entity.Shop, int, error) {
	filters := &repository.ShopAdminFilters{
		HealthLevel: healthLevelPtr(entity.ShopHealthCritical),
		Limit:       limit,
		Offset:      offset,
		SortBy:      "health_score",
		SortOrder:   "ASC",
	}
	return r.FindAllShopsAdmin(ctx, filters)
}

// GetShopsByHealthLevel retourne les shops par niveau de santé
func (r *ShopRepositoryInfrastructure) GetShopsByHealthLevel(
	ctx context.Context,
	level entity.ShopHealthLevel,
	limit, offset int,
) ([]*entity.Shop, int, error) {
	filters := &repository.ShopAdminFilters{
		HealthLevel: &level,
		Limit:       limit,
		Offset:      offset,
		SortBy:      "health_score",
		SortOrder:   "DESC",
	}
	return r.FindAllShopsAdmin(ctx, filters)
}

// SearchShopsAdmin recherche des shops par nom/slug/email
func (r *ShopRepositoryInfrastructure) SearchShopsAdmin(
	ctx context.Context,
	query string,
	limit, offset int,
) ([]*entity.Shop, int, error) {
	filters := &repository.ShopAdminFilters{
		Search:    query,
		Limit:     limit,
		Offset:    offset,
		SortBy:    "name",
		SortOrder: "ASC",
	}
	return r.FindAllShopsAdmin(ctx, filters)
}

// ============================================================
// HELPERS
// ============================================================

func boolPtr(b bool) *bool {
	return &b
}

func healthLevelPtr(level entity.ShopHealthLevel) *entity.ShopHealthLevel {
	return &level
}
