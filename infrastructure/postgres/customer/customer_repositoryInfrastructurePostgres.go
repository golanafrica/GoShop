package customer

import (
	dto "Goshop/application/dto/customer_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
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

func (cr *CustomerRepoInfrastructurePostgres) CountAllCustomers(ctx context.Context, filter dto.CustomerFilter) (int, error) {
	baseQuery := `SELECT COUNT(*) FROM customers`
	conditions := []string{}
	args := []interface{}{}
	argPos := 1

	if filter.Name != nil {
		// Recherche dans first_name OU last_name (insensible à la casse)
		conditions = append(conditions, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d)", argPos, argPos))
		args = append(args, "%"+*filter.Name+"%")
		argPos++
	}

	if filter.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argPos))
		args = append(args, "%"+*filter.Email+"%")
		// argPos++ // non utilisé après
	}

	query := baseQuery
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := cr.queryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count customers: %w", err)
	}
	return count, nil
}

func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomersWithPagination(
	ctx context.Context,
	limit, offset int,
	filter dto.CustomerFilter,
) ([]*entity.Customer, error) {
	baseQuery := `
		SELECT id, first_name, last_name, email, created_at, updated_at
		FROM customers`
	conditions := []string{}
	args := []interface{}{}
	argPos := 1

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
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Ajouter ORDER + LIMIT/OFFSET
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

func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomersWithSorting(ctx context.Context, sortBy, order string) ([]*entity.Customer, error) {
	// Liste blanche des colonnes autorisées
	allowedColumns := map[string]string{
		"first_name": "first_name",
		"last_name":  "last_name",
		"email":      "email",
		"created_at": "created_at",
	}

	// Liste blanche des ordres
	allowedOrders := map[string]string{
		"asc":  "ASC",
		"desc": "DESC",
	}

	column := allowedColumns[sortBy]
	if column == "" {
		column = "created_at" // valeur par défaut
	}

	direction := allowedOrders[strings.ToLower(order)]
	if direction == "" {
		direction = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, first_name, last_name, email, created_at, updated_at
		FROM customers
		ORDER BY %s %s`, column, direction)

	rows, err := cr.queryContext(ctx, query)
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

func (cr *CustomerRepoInfrastructurePostgres) Create(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	query := `INSERT INTO customers(id, first_name, last_name, email, created_at, updated_at) VALUES(gen_random_uuid(),$1, $2, $3, NOW(), NOW())
	RETURNING id, first_name, last_name, email, created_at, Updated_at`
	err := cr.queryRowContext(ctx, query, customer.FirstName, customer.LastName, customer.Email).Scan(
		&customer.ID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	return customer, err
}
func (cr *CustomerRepoInfrastructurePostgres) FindByCustomerID(ctx context.Context, id string) (*entity.Customer, error) {
	customer := entity.Customer{}
	query := `SELECT id, first_name, last_name, email, created_at, updated_at FROM
	customers WHERE id=$1`
	err := cr.queryRowContext(ctx, query, id).Scan(
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

func (cr *CustomerRepoInfrastructurePostgres) FindAllCustomers(ctx context.Context) ([]*entity.Customer, error) {
	query := `SELECT id, first_name, last_name, email, created_at, updated_at FROM customers `
	rows, err := cr.queryContext(ctx, query)
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

func (cr *CustomerRepoInfrastructurePostgres) UpdateCustomer(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	query := `
    UPDATE customers
    SET first_name = $1, last_name = $2, email = $3, updated_at = NOW()
    WHERE id = $4
    RETURNING first_name, last_name, email, created_at, updated_at, id
    `

	err := cr.queryRowContext(ctx, query,
		customer.FirstName,
		customer.LastName,
		customer.Email,
		customer.ID, // ID doit être le dernier paramètre
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

func (cr *CustomerRepoInfrastructurePostgres) DeleteCustomer(ctx context.Context, id string) error {
	log.Printf("🔍 DeleteCustomer appelé avec ID: '%s'", id)
	query := `DELETE FROM customers WHERE id=$1`
	_, err := cr.execContext(ctx, query, id)

	return err
}

func (cr *CustomerRepoInfrastructurePostgres) FindByEmail(ctx context.Context, email string) (*entity.Customer, error) {
	log.Printf("🔍 FindByEmail appelé avec email: '%s'", email)

	customer := &entity.Customer{}
	query := `SELECT id, first_name, last_name, email, created_at, updated_at FROM customers WHERE email = $1`

	err := cr.queryRowContext(ctx, query, email).Scan(
		&customer.ID,
		&customer.FirstName,
		&customer.LastName,
		&customer.Email,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("📭 Aucun customer trouvé avec l'email: %s", email)
			return nil, sql.ErrNoRows
		}

		log.Printf("❌ Erreur FindByEmail pour %s: %v", email, err)
		return nil, fmt.Errorf("failed to find customer by email: %w", err)
	}

	log.Printf("✅ Customer trouvé par email %s: ID=%s", email, customer.ID)
	return customer, nil
}
