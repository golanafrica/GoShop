package customer

import (
	dto "Goshop/application/dto/customer_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
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
		SELECT id, first_name, last_name, email, created_at, updated_at
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
		SELECT id, first_name, last_name, email, created_at, updated_at
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

// Create crée un client avec shop_id
func (cr *CustomerRepoInfrastructurePostgres) Create(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO customers(shop_id, first_name, last_name, email, created_at, updated_at) 
	VALUES ($1, $2, $3, $4, NOW(), NOW())
	RETURNING id, first_name, last_name, email, created_at, updated_at`

	err = cr.queryRowContext(ctx, query,
		shopID,
		customer.FirstName,
		customer.LastName,
		customer.Email,
	).Scan(
		&customer.ID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	return customer, err
}

// FindByCustomerID trouve un client par ID dans le shop courant
func (cr *CustomerRepoInfrastructurePostgres) FindByCustomerID(ctx context.Context, id string) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	customer := entity.Customer{}
	query := `SELECT id, first_name, last_name, email, created_at, updated_at 
	FROM customers WHERE id=$1 AND shop_id=$2`

	err = cr.queryRowContext(ctx, query, id, shopID).Scan(
		&customer.ID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, err
	}

	return &customer, nil
}

// FindAllCustomers retourne tous les clients du shop courant
func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomers(ctx context.Context) ([]*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `SELECT id, first_name, last_name, email, created_at, updated_at 
	FROM customers WHERE shop_id = $1`

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
			&customer.FirstName,
			&customer.LastName,
			&customer.Email,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		); err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}

	return customers, nil
}

// UpdateCustomer met à jour un client dans le shop courant
func (cr *CustomerRepoInfrastructurePostgres) UpdateCustomer(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	shopID, err := cr.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
    UPDATE customers
    SET first_name = $1, last_name = $2, email = $3, updated_at = NOW()
    WHERE id = $4 AND shop_id = $5
    RETURNING first_name, last_name, email, created_at, updated_at, id
    `

	err = cr.queryRowContext(ctx, query,
		customer.FirstName,
		customer.LastName,
		customer.Email,
		customer.ID,
		shopID,
	).Scan(
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.CreatedAt,
		&customer.UpdatedAt,
		&customer.ID,
	)

	return customer, err
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

	log.Printf("🔍 FindByEmail appelé avec email: '%s', shop_id: '%s'", email, shopID)

	customer := &entity.Customer{}
	query := `SELECT id, first_name, last_name, email, created_at, updated_at 
	FROM customers WHERE email = $1 AND shop_id = $2`

	err = cr.queryRowContext(ctx, query, email, shopID).Scan(
		&customer.ID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("📭 Aucun customer trouvé avec l'email: %s dans le shop %s", email, shopID)
			return nil, sql.ErrNoRows
		}
		log.Printf("❌ Erreur FindByEmail pour %s: %v", email, err)
		return nil, fmt.Errorf("failed to find customer by email: %w", err)
	}

	log.Printf("✅ Customer trouvé par email %s: ID=%s", email, customer.ID)
	return customer, nil
}
