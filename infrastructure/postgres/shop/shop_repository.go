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
// CREATE
// ============================================================

func (r *ShopRepositoryInfrastructure) Create(ctx context.Context, shop *entity.Shop) error {
	query := `
		INSERT INTO shops (
			id, name, slug, custom_domain, owner_id, logo_url, theme, plan, db_schema, is_active,
			kyc_status, kyc_submissions_count,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
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
		shop.KYCStatus,           // 🆕 v4.1.0
		shop.KYCSubmissionsCount, // 🆕 v4.1.0
		shop.CreatedAt,
		shop.UpdatedAt,
	).Scan(&shop.CreatedAt, &shop.UpdatedAt)

	return err
}

// ============================================================
// FIND METHODS
// ============================================================

func (r *ShopRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan, 
		       db_schema, is_active,
		       kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
		       kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
		       created_at, updated_at
		FROM shops
		WHERE id = $1
	`
	return r.scanShop(r.queryRowContext(ctx, query, id))
}

func (r *ShopRepositoryInfrastructure) FindBySlug(ctx context.Context, slug string) (*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active,
		       kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
		       kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
		       created_at, updated_at
		FROM shops
		WHERE slug = $1
	`
	return r.scanShop(r.queryRowContext(ctx, query, slug))
}

func (r *ShopRepositoryInfrastructure) FindByCustomDomain(ctx context.Context, domain string) (*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active,
		       kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
		       kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
		       created_at, updated_at
		FROM shops
		WHERE custom_domain = $1
	`
	return r.scanShop(r.queryRowContext(ctx, query, domain))
}

func (r *ShopRepositoryInfrastructure) FindByOwnerID(ctx context.Context, ownerID string) ([]*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active,
		       kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
		       kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
		       created_at, updated_at
		FROM shops
		WHERE owner_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.queryContext(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFromRows(rows)
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
// SCAN HELPERS (mis à jour avec champs KYC)
// ============================================================

// scanShop scanne une ligne depuis sql.Row
func (r *ShopRepositoryInfrastructure) scanShop(row *sql.Row) (*entity.Shop, error) {
	shop := &entity.Shop{}
	var themeJSON []byte
	var kycSubmittedAt, kycVerifiedAt, kycLastSubmissionAt sql.NullTime
	var kycVerifiedBy, kycRejectionReason sql.NullString

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
		// 🆕 v4.1.0 : Champs KYC
		&shop.KYCStatus,
		&kycSubmittedAt,
		&kycVerifiedAt,
		&kycVerifiedBy,
		&kycRejectionReason,
		&shop.KYCSubmissionsCount,
		&kycLastSubmissionAt,
		&shop.CreatedAt,
		&shop.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Gestion des champs nullable
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

	if len(themeJSON) > 0 {
		if err := json.Unmarshal(themeJSON, &shop.Theme); err != nil {
			return nil, err
		}
	} else {
		shop.Theme = make(map[string]interface{})
	}

	return shop, nil
}

// scanShopFromRows scanne une ligne depuis sql.Rows
func (r *ShopRepositoryInfrastructure) scanShopFromRows(rows *sql.Rows) (*entity.Shop, error) {
	shop := &entity.Shop{}
	var themeJSON []byte
	var kycSubmittedAt, kycVerifiedAt, kycLastSubmissionAt sql.NullTime
	var kycVerifiedBy, kycRejectionReason sql.NullString

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
		// 🆕 v4.1.0 : Champs KYC
		&shop.KYCStatus,
		&kycSubmittedAt,
		&kycVerifiedAt,
		&kycVerifiedBy,
		&kycRejectionReason,
		&shop.KYCSubmissionsCount,
		&kycLastSubmissionAt,
		&shop.CreatedAt,
		&shop.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Gestion des champs nullable
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

// GetPaymentSettings récupère les settings de paiement d'une boutique
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

	// Déchiffrer les clés API
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

// UpsertPaymentSettings crée ou met à jour les settings de paiement
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

// IsOwner vérifie qu'un utilisateur est propriétaire d'une boutique
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

// FindByKYCStatus retourne les shops filtrés par statut KYC
func (r *ShopRepositoryInfrastructure) FindByKYCStatus(
	ctx context.Context,
	status entity.ShopKYCStatus,
	limit, offset int,
) ([]*entity.Shop, error) {
	query := `
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active,
		       kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
		       kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
		       created_at, updated_at
		FROM shops
		WHERE kyc_status = $1
		ORDER BY kyc_submitted_at DESC NULLS LAST, created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.queryContext(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find by kyc status: %w", err)
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFromRows(rows)
		if err != nil {
			return nil, err
		}
		shops = append(shops, shop)
	}

	return shops, rows.Err()
}

// CountByKYCStatus compte les shops par statut KYC
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

// UpdateKYCStatus met à jour le statut KYC d'un shop
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

// FindAllShopsAdmin retourne tous les shops (cross-tenant) avec pagination et filtres
func (r *ShopRepositoryInfrastructure) FindAllShopsAdmin(
	ctx context.Context,
	limit, offset int,
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
		if filters.Search != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("(name ILIKE $%d OR slug ILIKE $%d)", argIndex, argIndex))
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

	// Récupérer les shops
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, name, slug, custom_domain, owner_id, logo_url, theme, plan,
		       db_schema, is_active,
		       kyc_status, kyc_submitted_at, kyc_verified_at, kyc_verified_by,
		       kyc_rejection_reason, kyc_submissions_count, kyc_last_submission_at,
		       created_at, updated_at
		FROM shops
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("find all shops admin: %w", err)
	}
	defer rows.Close()

	var shops []*entity.Shop
	for rows.Next() {
		shop, err := r.scanShopFromRows(rows)
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
