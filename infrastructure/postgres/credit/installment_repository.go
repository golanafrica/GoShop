package credit

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CreditInstallmentRepositoryInfrastructure implémente repository.CreditInstallmentRepository
type CreditInstallmentRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCreditInstallmentRepositoryInfrastructure crée une nouvelle instance
func NewCreditInstallmentRepositoryInfrastructure(db *sql.DB) repository.CreditInstallmentRepository {
	return &CreditInstallmentRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CreditInstallmentRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CreditInstallmentRepository {
	return &CreditInstallmentRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// HELPERS
// ============================================================

func (r *CreditInstallmentRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CreditInstallmentRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CreditInstallmentRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *CreditInstallmentRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// verifyContractBelongsToShop vérifie qu'un contrat appartient à la boutique
func (r *CreditInstallmentRepositoryInfrastructure) verifyContractBelongsToShop(ctx context.Context, contractID, shopID string) error {
	var contractShopID string
	err := r.queryRowContext(ctx, `SELECT shop_id FROM credit_contracts WHERE id = $1`, contractID).Scan(&contractShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("credit contract not found")
		}
		return fmt.Errorf("failed to verify contract: %w", err)
	}
	if contractShopID != shopID {
		return fmt.Errorf("access denied: contract does not belong to tenant shop")
	}
	return nil
}

// scanInstallment scanne une ligne dans une entité CreditInstallment
func (r *CreditInstallmentRepositoryInfrastructure) scanInstallment(row *sql.Row) (*entity.CreditInstallment, error) {
	installment := &entity.CreditInstallment{}
	var paymentID sql.NullString
	var paidAt sql.NullTime

	err := row.Scan(
		&installment.ID,
		&installment.ContractID,
		&installment.InstallmentNumber,
		&installment.DueDate,
		&installment.AmountCents,
		&paymentID,
		&paidAt,
		&installment.Status,
		&installment.LateFeeCents,
		&installment.CreatedAt,
		&installment.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit installment not found")
		}
		return nil, fmt.Errorf("failed to scan credit installment: %w", err)
	}

	if paymentID.Valid {
		installment.PaymentID = &paymentID.String
	}
	if paidAt.Valid {
		installment.PaidAt = &paidAt.Time
	}

	return installment, nil
}

// scanInstallments scanne plusieurs lignes
func (r *CreditInstallmentRepositoryInfrastructure) scanInstallments(ctx context.Context, query string, args ...interface{}) ([]*entity.CreditInstallment, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query credit installments: %w", err)
	}
	defer rows.Close()

	var installments []*entity.CreditInstallment
	for rows.Next() {
		installment := &entity.CreditInstallment{}
		var paymentID sql.NullString
		var paidAt sql.NullTime

		err := rows.Scan(
			&installment.ID,
			&installment.ContractID,
			&installment.InstallmentNumber,
			&installment.DueDate,
			&installment.AmountCents,
			&paymentID,
			&paidAt,
			&installment.Status,
			&installment.LateFeeCents,
			&installment.CreatedAt,
			&installment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if paymentID.Valid {
			installment.PaymentID = &paymentID.String
		}
		if paidAt.Valid {
			installment.PaidAt = &paidAt.Time
		}

		installments = append(installments, installment)
	}

	if installments == nil {
		installments = []*entity.CreditInstallment{}
	}
	return installments, rows.Err()
}

// ============================================================
// IMPLÉMENTATION
// ============================================================

// Create crée une nouvelle échéance
func (r *CreditInstallmentRepositoryInfrastructure) Create(ctx context.Context, installment *entity.CreditInstallment) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le contrat parent appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, installment.ContractID, shopID); err != nil {
		return err
	}

	query := `
		INSERT INTO credit_installments (
			contract_id, installment_number, due_date, amount_cents,
			payment_id, paid_at, status, late_fee_cents,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		installment.ContractID,
		installment.InstallmentNumber,
		installment.DueDate,
		installment.AmountCents,
		installment.PaymentID,
		installment.PaidAt,
		installment.Status,
		installment.LateFeeCents,
	).Scan(&installment.ID, &installment.CreatedAt, &installment.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create credit installment: %w", err)
	}

	return nil
}

// CreateBatch crée plusieurs échéances en lot
func (r *CreditInstallmentRepositoryInfrastructure) CreateBatch(ctx context.Context, installments []*entity.CreditInstallment) error {
	if len(installments) == 0 {
		return nil
	}

	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que tous les contrats appartiennent à la boutique
	for _, installment := range installments {
		if err := r.verifyContractBelongsToShop(ctx, installment.ContractID, shopID); err != nil {
			return err
		}
	}

	// Créer chaque échéance individuellement (pour récupérer les IDs)
	for _, installment := range installments {
		if err := r.Create(ctx, installment); err != nil {
			return fmt.Errorf("failed to create installment %d: %w", installment.InstallmentNumber, err)
		}
	}

	return nil
}

// FindByID trouve une échéance par ID
func (r *CreditInstallmentRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.CreditInstallment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT i.id, i.contract_id, i.installment_number, i.due_date, i.amount_cents,
		       i.payment_id, i.paid_at, i.status, i.late_fee_cents,
		       i.created_at, i.updated_at
		FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE i.id = $1 AND c.shop_id = $2
	`

	return r.scanInstallment(r.queryRowContext(ctx, query, id, shopID))
}

// FindByContractID retourne les échéances d'un contrat
func (r *CreditInstallmentRepositoryInfrastructure) FindByContractID(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, contract_id, installment_number, due_date, amount_cents,
		       payment_id, paid_at, status, late_fee_cents,
		       created_at, updated_at
		FROM credit_installments
		WHERE contract_id = $1
		ORDER BY installment_number ASC
	`

	return r.scanInstallments(ctx, query, contractID)
}

// FindByContractIDAndNumber trouve une échéance par contrat et numéro
func (r *CreditInstallmentRepositoryInfrastructure) FindByContractIDAndNumber(ctx context.Context, contractID string, number int) (*entity.CreditInstallment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, contract_id, installment_number, due_date, amount_cents,
		       payment_id, paid_at, status, late_fee_cents,
		       created_at, updated_at
		FROM credit_installments
		WHERE contract_id = $1 AND installment_number = $2
	`

	return r.scanInstallment(r.queryRowContext(ctx, query, contractID, number))
}

// FindPendingByContractID retourne les échéances en attente d'un contrat
func (r *CreditInstallmentRepositoryInfrastructure) FindPendingByContractID(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, contract_id, installment_number, due_date, amount_cents,
		       payment_id, paid_at, status, late_fee_cents,
		       created_at, updated_at
		FROM credit_installments
		WHERE contract_id = $1 AND status = 'pending'
		ORDER BY due_date ASC
	`

	return r.scanInstallments(ctx, query, contractID)
}

// FindOverdue retourne les échéances échues (due_date < now)
func (r *CreditInstallmentRepositoryInfrastructure) FindOverdue(ctx context.Context) ([]*entity.CreditInstallment, error) {
	query := `
		SELECT i.id, i.contract_id, i.installment_number, i.due_date, i.amount_cents,
		       i.payment_id, i.paid_at, i.status, i.late_fee_cents,
		       i.created_at, i.updated_at
		FROM credit_installments i
		WHERE i.due_date < NOW()
		  AND i.status IN ('pending', 'late')
		ORDER BY i.due_date ASC
	`

	return r.scanInstallments(ctx, query)
}

// FindOverdueByContractID retourne les échéances échues d'un contrat
func (r *CreditInstallmentRepositoryInfrastructure) FindOverdueByContractID(ctx context.Context, contractID string) ([]*entity.CreditInstallment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, contract_id, installment_number, due_date, amount_cents,
		       payment_id, paid_at, status, late_fee_cents,
		       created_at, updated_at
		FROM credit_installments
		WHERE contract_id = $1
		  AND due_date < NOW()
		  AND status IN ('pending', 'late')
		ORDER BY due_date ASC
	`

	return r.scanInstallments(ctx, query, contractID)
}

// CountPendingByContractID compte les échéances en attente d'un contrat
func (r *CreditInstallmentRepositoryInfrastructure) CountPendingByContractID(ctx context.Context, contractID string) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM credit_installments
		WHERE contract_id = $1 AND status = 'pending'
	`

	var count int
	err = r.queryRowContext(ctx, query, contractID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending installments: %w", err)
	}

	return count, nil
}

// SumPendingAmountByContractID somme des montants en attente
func (r *CreditInstallmentRepositoryInfrastructure) SumPendingAmountByContractID(ctx context.Context, contractID string) (int64, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return 0, err
	}

	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM credit_installments
		WHERE contract_id = $1 AND status = 'pending'
	`

	var total int64
	err = r.queryRowContext(ctx, query, contractID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum pending installments: %w", err)
	}

	return total, nil
}

// Update met à jour une échéance
func (r *CreditInstallmentRepositoryInfrastructure) Update(ctx context.Context, installment *entity.CreditInstallment) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que l'échéance appartient à la boutique via le contrat
	query := `
		SELECT c.shop_id FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE i.id = $1
	`

	var installmentShopID string
	err = r.queryRowContext(ctx, query, installment.ID).Scan(&installmentShopID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("credit installment not found")
		}
		return fmt.Errorf("failed to verify installment: %w", err)
	}
	if installmentShopID != shopID {
		return fmt.Errorf("access denied: installment does not belong to tenant shop")
	}

	query = `
		UPDATE credit_installments
		SET payment_id = $2,
		    paid_at = $3,
		    status = $4,
		    late_fee_cents = $5,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.queryRowContext(ctx, query,
		installment.ID,
		installment.PaymentID,
		installment.PaidAt,
		installment.Status,
		installment.LateFeeCents,
	).Scan(&installment.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update credit installment: %w", err)
	}

	return nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// Exists vérifie si une échéance existe
func (r *CreditInstallmentRepositoryInfrastructure) Exists(ctx context.Context, id string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT COUNT(*) FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE i.id = $1 AND c.shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check installment existence: %w", err)
	}

	return count > 0, nil
}

// IsPaid vérifie si une échéance est payée
func (r *CreditInstallmentRepositoryInfrastructure) IsPaid(ctx context.Context, id string) (bool, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return false, err
	}

	query := `
		SELECT i.status FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE i.id = $1 AND c.shop_id = $2
	`

	var status entity.InstallmentStatus
	err = r.queryRowContext(ctx, query, id, shopID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("credit installment not found")
		}
		return false, fmt.Errorf("failed to check installment status: %w", err)
	}

	return status == entity.InstallmentPaid, nil
}

// CountByContractIDAndStatus compte les échéances par statut pour un contrat
func (r *CreditInstallmentRepositoryInfrastructure) CountByContractIDAndStatus(ctx context.Context, contractID string, status entity.InstallmentStatus) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM credit_installments
		WHERE contract_id = $1 AND status = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, contractID, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count installments: %w", err)
	}

	return count, nil
}

// SumAmountByContractIDAndStatus somme des montants par statut pour un contrat
func (r *CreditInstallmentRepositoryInfrastructure) SumAmountByContractIDAndStatus(ctx context.Context, contractID string, status entity.InstallmentStatus) (int64, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return 0, err
	}

	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM credit_installments
		WHERE contract_id = $1 AND status = $2
	`

	var total int64
	err = r.queryRowContext(ctx, query, contractID, status).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum installments: %w", err)
	}

	return total, nil
}

// FindNextPendingByContractID retourne la prochaine échéance en attente
func (r *CreditInstallmentRepositoryInfrastructure) FindNextPendingByContractID(ctx context.Context, contractID string) (*entity.CreditInstallment, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	// Vérifier que le contrat appartient à la boutique
	if err := r.verifyContractBelongsToShop(ctx, contractID, shopID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, contract_id, installment_number, due_date, amount_cents,
		       payment_id, paid_at, status, late_fee_cents,
		       created_at, updated_at
		FROM credit_installments
		WHERE contract_id = $1 AND status = 'pending'
		ORDER BY due_date ASC
		LIMIT 1
	`

	return r.scanInstallment(r.queryRowContext(ctx, query, contractID))
}

// FindOverdueCountByShopID compte les échéances échues pour une boutique
func (r *CreditInstallmentRepositoryInfrastructure) FindOverdueCountByShopID(ctx context.Context, shopID string) (int, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot query installments of another shop")
	}

	query := `
		SELECT COUNT(*) FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE c.shop_id = $1
		  AND i.due_date < NOW()
		  AND i.status IN ('pending', 'late')
	`

	var count int
	err = r.queryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count overdue installments: %w", err)
	}

	return count, nil
}

// SumOverdueAmountByShopID somme des montants échus pour une boutique
func (r *CreditInstallmentRepositoryInfrastructure) SumOverdueAmountByShopID(ctx context.Context, shopID string) (int64, error) {
	// Vérifier le multi-tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}
	if shopID != currentShopID {
		return 0, fmt.Errorf("access denied: cannot sum installments of another shop")
	}

	query := `
		SELECT COALESCE(SUM(i.amount_cents), 0)
		FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE c.shop_id = $1
		  AND i.due_date < NOW()
		  AND i.status IN ('pending', 'late')
	`

	var total int64
	err = r.queryRowContext(ctx, query, shopID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum overdue installments: %w", err)
	}

	return total, nil
}

// ============================================================
// 🆕 v3.4.0 : MÉTHODES POUR LE SCHEDULER DE COMMISSIONS
// ============================================================

// FindPaidWithoutCommission récupère les échéances payées sans commission collectée
// 🆕 v3.4.0 : Fait un JOIN avec credit_contracts pour récupérer le shop_id
func (r *CreditInstallmentRepositoryInfrastructure) FindPaidWithoutCommission(
	ctx context.Context,
	limit int,
) ([]*entity.CreditInstallment, error) {
	query := `
		SELECT 
			i.id, i.contract_id, i.installment_number, i.due_date, i.amount_cents,
			i.payment_id, i.paid_at, i.status, i.late_fee_cents,
			i.created_at, i.updated_at,
			COALESCE(i.commission_status, 'pending') as commission_status,
			COALESCE(i.commission_cents, 0) as commission_cents,
			i.commission_batch_id,
			i.commission_collected_at,
			c.shop_id
		FROM credit_installments i
		JOIN credit_contracts c ON c.id = i.contract_id
		WHERE i.status = 'paid'
		  AND (i.commission_status IS NULL OR i.commission_status = 'pending')
		ORDER BY i.paid_at ASC
		LIMIT $1
	`
	rows, err := r.queryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query paid installments: %w", err)
	}
	defer rows.Close()

	var installments []*entity.CreditInstallment
	for rows.Next() {
		inst := &entity.CreditInstallment{}
		var paymentID sql.NullString
		var paidAt sql.NullTime
		var commissionStatus sql.NullString
		var commissionBatchID sql.NullString
		var commissionCollectedAt sql.NullTime

		err := rows.Scan(
			&inst.ID,
			&inst.ContractID,
			&inst.InstallmentNumber,
			&inst.DueDate,
			&inst.AmountCents,
			&paymentID,
			&paidAt,
			&inst.Status,
			&inst.LateFeeCents,
			&inst.CreatedAt,
			&inst.UpdatedAt,
			&commissionStatus,
			&inst.CommissionCents,
			&commissionBatchID,
			&commissionCollectedAt,
			&inst.ShopID, // 🆕 v3.4.0 : Remplit le champ ShopID
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan installment: %w", err)
		}

		if paymentID.Valid {
			inst.PaymentID = &paymentID.String
		}
		if paidAt.Valid {
			inst.PaidAt = &paidAt.Time
		}
		if commissionStatus.Valid {
			inst.CommissionStatus = commissionStatus.String
		}
		if commissionBatchID.Valid {
			inst.CommissionBatchID = &commissionBatchID.String
		}
		if commissionCollectedAt.Valid {
			inst.CommissionCollectedAt = &commissionCollectedAt.Time
		}

		installments = append(installments, inst)
	}

	if installments == nil {
		installments = []*entity.CreditInstallment{}
	}
	return installments, rows.Err()
}

// UpdateCreditCommissionStatus met à jour le statut de commission d'une échéance
// 🆕 v3.4.0 : Utilisé par le scheduler après collecte réussie/échouée
func (r *CreditInstallmentRepositoryInfrastructure) UpdateCreditCommissionStatus(
	ctx context.Context,
	installmentID string,
	status string,
	commissionCents int64,
	batchID *string,
) error {
	query := `
		UPDATE credit_installments SET
			commission_status = $1,
			commission_cents = $2,
			commission_batch_id = $3,
			commission_collected_at = CASE 
				WHEN $1 = 'collected' THEN NOW() 
				ELSE commission_collected_at 
			END,
			updated_at = NOW()
		WHERE id = $4
	`
	result, err := r.execContext(ctx, query, status, commissionCents, batchID, installmentID)
	if err != nil {
		return fmt.Errorf("failed to update credit commission status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("credit installment not found")
	}
	return nil
}
