package customer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	credit_dto "Goshop/application/dto/credit_dto"
	dto "Goshop/application/dto/customer_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

type CustomerRepoInfrastructurePostgres struct {
	db *sql.DB
	tx repository.Tx
}

func NewCustomerRepoInfrastructurePostgres(db *sql.DB) *CustomerRepoInfrastructurePostgres {
	return &CustomerRepoInfrastructurePostgres{db: db}
}

func (cr *CustomerRepoInfrastructurePostgres) WithTX(tx repository.Tx) repository.CustomerRepositoryInterface {
	return &CustomerRepoInfrastructurePostgres{
		db: cr.db,
		tx: tx,
	}
}

func (cr *CustomerRepoInfrastructurePostgres) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if cr.tx != nil {
		return cr.tx.QueryRowContext(ctx, query, args...)
	}
	return cr.db.QueryRowContext(ctx, query, args...)
}

func (cr *CustomerRepoInfrastructurePostgres) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if cr.tx != nil {
		return cr.tx.QueryContext(ctx, query, args...)
	}
	return cr.db.QueryContext(ctx, query, args...)
}

func (cr *CustomerRepoInfrastructurePostgres) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if cr.tx != nil {
		return cr.tx.ExecContext(ctx, query, args...)
	}
	return cr.db.ExecContext(ctx, query, args...)
}

// getShopID extrait le shop_id du contexte (multi-tenant)
func (cr *CustomerRepoInfrastructurePostgres) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// CountAllCustomers compte les clients du shop courant avec filtres
func (cr *CustomerRepoInfrastructurePostgres) CountAllCustomers(ctx context.Context, filter dto.CustomerFilter) (int, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	baseQuery := `SELECT COUNT(*) FROM customers WHERE shop_id = $1`
	args := []interface{}{shopID}
	argPos := 2
	conditions := []string{}

	if filter.Name != nil {
		conditions = append(conditions, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d)", argPos, argPos))
		args = append(args, "%"+*filter.Name+"%")
		argPos++
	}

	if filter.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argPos))
		args = append(args, "%"+*filter.Email+"%")
		argPos++
	}

	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	var count int
	err = cr.queryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count customers: %w", err)
	}
	return count, nil
}

// FindAllCustomersWithPagination récupère les clients du shop avec pagination
func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomersWithPagination(
	ctx context.Context,
	limit, offset int,
	filter dto.CustomerFilter,
) ([]*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	baseQuery := `
		SELECT id, user_id, first_name, last_name, email, created_at, updated_at
		FROM customers
		WHERE shop_id = $1`
	args := []interface{}{shopID}
	argPos := 2
	conditions := []string{}

	if filter.Name != nil {
		conditions = append(conditions, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d)", argPos, argPos))
		args = append(args, "%"+*filter.Name+"%")
		argPos++
	}

	if filter.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argPos))
		args = append(args, "%"+*filter.Email+"%")
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	query := baseQuery + whereClause + " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argPos) + " OFFSET $" + strconv.Itoa(argPos+1)
	args = append(args, limit, offset)

	rows, err := cr.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch paginated customers: %w", err)
	}
	defer rows.Close()

	var customers []*entity.Customer
	for rows.Next() {
		c := &entity.Customer{}
		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.FirstName,
			&c.LastName,
			&c.Email,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan customer: %w", err)
		}
		customers = append(customers, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return customers, nil
}

// FindAllCustomersWithSorting récupère les clients du shop avec tri
func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomersWithSorting(ctx context.Context, sortBy, order string) ([]*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	allowedColumns := map[string]string{
		"first_name": "first_name",
		"last_name":  "last_name",
		"email":      "email",
		"created_at": "created_at",
	}

	allowedOrders := map[string]string{
		"asc":  "ASC",
		"desc": "DESC",
	}

	column := allowedColumns[sortBy]
	if column == "" {
		column = "created_at"
	}

	direction := allowedOrders[strings.ToLower(order)]
	if direction == "" {
		direction = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, first_name, last_name, email, created_at, updated_at
		FROM customers
		WHERE shop_id = $1
		ORDER BY %s %s`, column, direction)

	rows, err := cr.queryContext(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sorted customers: %w", err)
	}
	defer rows.Close()

	var customers []*entity.Customer
	for rows.Next() {
		c := &entity.Customer{}
		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.FirstName,
			&c.LastName,
			&c.Email,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan customer: %w", err)
		}
		customers = append(customers, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return customers, nil
}

// Create crée un client avec shop_id, user_id et KYC par défaut (none)
func (cr *CustomerRepoInfrastructurePostgres) Create(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	if customer.KYCLevel == "" {
		customer.KYCLevel = entity.KYCLevelNone
	}

	query := `
		INSERT INTO customers (
			shop_id, user_id, first_name, last_name, email,
			kyc_level, kyc_validated_at, kyc_validated_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, user_id, first_name, last_name, email,
		          kyc_level, kyc_validated_at, kyc_validated_by,
		          created_at, updated_at
	`

	err = cr.queryRowContext(ctx, query,
		shopID,
		customer.UserID,
		customer.FirstName,
		customer.LastName,
		customer.Email,
		string(customer.KYCLevel),
		customer.KYCValidatedAt,
		customer.KYCValidatedBy,
	).Scan(
		&customer.ID,
		&customer.UserID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.KYCLevel,
		&customer.KYCValidatedAt,
		&customer.KYCValidatedBy,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	return customer, nil
}

// FindByCustomerID trouve un client par ID dans le shop courant
func (cr *CustomerRepoInfrastructurePostgres) FindByCustomerID(ctx context.Context, id string) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	customer := &entity.Customer{}
	query := `
		SELECT id, user_id, first_name, last_name, email,
		       kyc_level, kyc_validated_at, kyc_validated_by,
		       created_at, updated_at
		FROM customers WHERE id = $1 AND shop_id = $2
	`

	err = cr.queryRowContext(ctx, query, id, shopID).Scan(
		&customer.ID,
		&customer.UserID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.KYCLevel,
		&customer.KYCValidatedAt,
		&customer.KYCValidatedBy,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan customer: %w", err)
	}

	return customer, nil
}

// 🆕 FindByUserID trouve un client à partir de son user_id (JWT) dans le shop courant
func (cr *CustomerRepoInfrastructurePostgres) FindByUserID(ctx context.Context, userID string) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	customer := &entity.Customer{}
	query := `
		SELECT id, user_id, first_name, last_name, email,
		       kyc_level, kyc_validated_at, kyc_validated_by,
		       created_at, updated_at
		FROM customers WHERE user_id = $1 AND shop_id = $2
	`

	err = cr.queryRowContext(ctx, query, userID, shopID).Scan(
		&customer.ID,
		&customer.UserID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.KYCLevel,
		&customer.KYCValidatedAt,
		&customer.KYCValidatedBy,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan customer by user_id: %w", err)
	}

	return customer, nil
}

// FindAllCustomers retourne tous les clients du shop courant
func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomers(ctx context.Context) ([]*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, user_id, first_name, last_name, email,
		       kyc_level, kyc_validated_at, kyc_validated_by,
		       created_at, updated_at
		FROM customers WHERE shop_id = $1
	`

	rows, err := cr.queryContext(ctx, query, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*entity.Customer
	for rows.Next() {
		customer := &entity.Customer{}
		if err := rows.Scan(
			&customer.ID,
			&customer.UserID,
			&customer.FirstName,
			&customer.LastName,
			&customer.Email,
			&customer.KYCLevel,
			&customer.KYCValidatedAt,
			&customer.KYCValidatedBy,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		); err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}

	return customers, nil
}

// UpdateCustomer met à jour un client (y compris les champs KYC) dans le shop courant
func (cr *CustomerRepoInfrastructurePostgres) UpdateCustomer(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		UPDATE customers SET
			first_name = $1,
			last_name = $2,
			email = $3,
			kyc_level = $4,
			kyc_validated_at = $5,
			kyc_validated_by = $6,
			updated_at = NOW()
		WHERE id = $7 AND shop_id = $8
		RETURNING id, user_id, first_name, last_name, email,
		          kyc_level, kyc_validated_at, kyc_validated_by,
		          created_at, updated_at
	`

	err = cr.queryRowContext(ctx, query,
		customer.FirstName,
		customer.LastName,
		customer.Email,
		string(customer.KYCLevel),
		customer.KYCValidatedAt,
		customer.KYCValidatedBy,
		customer.ID,
		shopID,
	).Scan(
		&customer.ID,
		&customer.UserID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.KYCLevel,
		&customer.KYCValidatedAt,
		&customer.KYCValidatedBy,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("customer not found")
		}
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	return customer, nil
}

// DeleteCustomer supprime un client du shop courant
func (cr *CustomerRepoInfrastructurePostgres) DeleteCustomer(ctx context.Context, id string) error {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return err
	}

	log.Printf("🔍 DeleteCustomer appelé avec ID: '%s', shop_id: '%s'", id, shopID)
	query := `DELETE FROM customers WHERE id=$1 AND shop_id=$2`
	_, err = cr.execContext(ctx, query, id, shopID)

	return err
}

// FindByEmail trouve un client par email dans le shop courant
func (cr *CustomerRepoInfrastructurePostgres) FindByEmail(ctx context.Context, email string) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	customer := &entity.Customer{}
	query := `
		SELECT id, user_id, first_name, last_name, email,
		       kyc_level, kyc_validated_at, kyc_validated_by,
		       created_at, updated_at
		FROM customers WHERE email = $1 AND shop_id = $2
	`

	err = cr.queryRowContext(ctx, query, email, shopID).Scan(
		&customer.ID,
		&customer.UserID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.KYCLevel,
		&customer.KYCValidatedAt,
		&customer.KYCValidatedBy,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to find customer by email: %w", err)
	}

	return customer, nil
}

// GetClientDashboard récupère le dashboard client en une seule requête JSON optimisée
func (cr *CustomerRepoInfrastructurePostgres) GetClientDashboard(ctx context.Context, customerID string) (*credit_dto.ClientDashboardResponse, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	query := `
		SELECT 
			c.id, c.first_name, c.last_name, c.kyc_level,
			
			-- 1. Score de crédit agrégé
			json_build_object(
				'score', COALESCE(cs.score, 500),
				'on_time_payments', COALESCE(cs.on_time_payments, 0),
				'late_payments', COALESCE(cs.late_payments, 0),
				'defaults', COALESCE(cs.defaults, 0)
			) AS credit_score_json,

			-- 2. Contrats actifs agrégés
			(
				SELECT json_agg(json_build_object(
					'contract_id', cc.id,
					'status', cc.status,
					'total_amount_cents', cc.total_amount_cents,
					'monthly_payment_cents', cc.monthly_payment_cents
				))
				FROM credit_contracts cc
				WHERE cc.customer_id = c.id AND cc.shop_id = $2 AND cc.status = 'active'
			) AS active_contracts_json,

			-- 3. Prochaines échéances (max 3) agrégées
			(
				SELECT json_agg(row_to_json(inst))
				FROM (
					SELECT ci.id as installment_id, ci.contract_id, ci.due_date, ci.amount_cents, ci.status
					FROM credit_installments ci
					JOIN credit_contracts cc ON ci.contract_id = cc.id
					WHERE cc.customer_id = c.id AND cc.shop_id = $2
					  AND ci.status IN ('pending', 'late')
					ORDER BY ci.due_date ASC
					LIMIT 3
				) inst
			) AS upcoming_installments_json

		FROM customers c
		LEFT JOIN credit_scores cs ON c.id = cs.customer_id AND cs.shop_id = $2
		WHERE c.id = $1 AND c.shop_id = $2
	`

	var resp credit_dto.ClientDashboardResponse
	var scoreJSON, contractsJSON, installmentsJSON sql.NullString

	err = cr.queryRowContext(ctx, query, customerID, shopID).Scan(
		&resp.CustomerID,
		&resp.FirstName,
		&resp.LastName,
		&resp.KYCLevel,
		&scoreJSON,
		&contractsJSON,
		&installmentsJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// ✅ FALLBACK ÉLÉGANT : Retourne un dashboard par défaut si le profil client n'existe pas encore
			resp.CustomerID = customerID
			resp.FirstName = "Client"
			resp.LastName = "Nouveau"
			resp.KYCLevel = "none"
			resp.CreditScore = credit_dto.CreditScoreDTO{Score: 500}
			resp.ActiveContracts = []credit_dto.ActiveContractDTO{}
			resp.UpcomingInstallments = []credit_dto.UpcomingInstallmentDTO{}
			return &resp, nil
		}
		return nil, fmt.Errorf("failed to query client dashboard: %w", err)
	}

	// Désérialisation du JSON Postgres vers les structs Go
	if scoreJSON.Valid {
		if err := json.Unmarshal([]byte(scoreJSON.String), &resp.CreditScore); err != nil {
			return nil, fmt.Errorf("failed to unmarshal credit score: %w", err)
		}
	} else {
		resp.CreditScore = credit_dto.CreditScoreDTO{Score: 500}
	}

	if contractsJSON.Valid {
		if err := json.Unmarshal([]byte(contractsJSON.String), &resp.ActiveContracts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal active contracts: %w", err)
		}
	}
	if resp.ActiveContracts == nil {
		resp.ActiveContracts = []credit_dto.ActiveContractDTO{}
	}

	if installmentsJSON.Valid {
		if err := json.Unmarshal([]byte(installmentsJSON.String), &resp.UpcomingInstallments); err != nil {
			return nil, fmt.Errorf("failed to unmarshal upcoming installments: %w", err)
		}
	}
	if resp.UpcomingInstallments == nil {
		resp.UpcomingInstallments = []credit_dto.UpcomingInstallmentDTO{}
	}

	return &resp, nil
}
