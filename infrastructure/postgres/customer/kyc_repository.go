package customer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

// CustomerKYCRepositoryInfrastructure implémente repository.CustomerKYCRepository
type CustomerKYCRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewCustomerKYCRepositoryInfrastructure crée une nouvelle instance du repository KYC
func NewCustomerKYCRepositoryInfrastructure(db *sql.DB) repository.CustomerKYCRepository {
	return &CustomerKYCRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *CustomerKYCRepositoryInfrastructure) WithTX(tx repository.Tx) repository.CustomerKYCRepository {
	return &CustomerKYCRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers internes
// ============================================================

func (r *CustomerKYCRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *CustomerKYCRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *CustomerKYCRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// getShopID extrait le shop_id du contexte (multi-tenant)
func (r *CustomerKYCRepositoryInfrastructure) getShopID(ctx context.Context) (string, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("multi-tenant: %w", err)
	}
	return shop.ID.String(), nil
}

// scanKYCDocument scanne une ligne SQL dans une entité CustomerKYCDocument
func (r *CustomerKYCRepositoryInfrastructure) scanKYCDocument(row *sql.Row) (*entity.CustomerKYCDocument, error) {
	doc := &entity.CustomerKYCDocument{}
	var reviewedBy sql.NullString
	var reviewedAt sql.NullTime
	var rejectionReason sql.NullString

	err := row.Scan(
		&doc.ID,
		&doc.CustomerID,
		&doc.ShopID,
		&doc.DocumentType,
		&doc.FilePath,
		&doc.FileSizeBytes,
		&doc.MimeType,
		&doc.Status,
		&reviewedBy,
		&reviewedAt,
		&rejectionReason,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("kyc document not found")
		}
		return nil, fmt.Errorf("failed to scan kyc document: %w", err)
	}

	if reviewedBy.Valid {
		doc.ReviewedBy = &reviewedBy.String
	}
	if reviewedAt.Valid {
		doc.ReviewedAt = &reviewedAt.Time
	}
	if rejectionReason.Valid {
		doc.RejectionReason = &rejectionReason.String
	}

	return doc, nil
}

// ============================================================
// Implémentation des méthodes du repository
// ============================================================

// Create crée un nouveau document KYC
func (r *CustomerKYCRepositoryInfrastructure) Create(ctx context.Context, doc *entity.CustomerKYCDocument) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	// Vérifier que le document appartient bien à la boutique du contexte
	if doc.ShopID != shopID {
		return fmt.Errorf("document shop_id does not match tenant shop_id")
	}

	query := `
		INSERT INTO customer_kyc_documents (
			customer_id, shop_id, document_type, file_path,
			file_size_bytes, mime_type, status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.queryRowContext(ctx, query,
		doc.CustomerID,
		doc.ShopID,
		string(doc.DocumentType),
		doc.FilePath,
		doc.FileSizeBytes,
		doc.MimeType,
		string(doc.Status),
	).Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create kyc document: %w", err)
	}

	return nil
}

// FindByID trouve un document KYC par son ID
func (r *CustomerKYCRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.CustomerKYCDocument, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, shop_id, document_type, file_path,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM customer_kyc_documents
		WHERE id = $1 AND shop_id = $2
	`

	return r.scanKYCDocument(r.queryRowContext(ctx, query, id, shopID))
}

// FindByCustomerID retourne tous les documents KYC d'un client
func (r *CustomerKYCRepositoryInfrastructure) FindByCustomerID(ctx context.Context, customerID string) ([]*entity.CustomerKYCDocument, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, shop_id, document_type, file_path,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM customer_kyc_documents
		WHERE customer_id = $1 AND shop_id = $2
		ORDER BY created_at DESC
	`

	return r.scanKYCDocuments(ctx, query, customerID, shopID)
}

// FindPendingByCustomer retourne les documents KYC en attente pour un client
func (r *CustomerKYCRepositoryInfrastructure) FindPendingByCustomer(ctx context.Context, customerID string) ([]*entity.CustomerKYCDocument, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, customer_id, shop_id, document_type, file_path,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM customer_kyc_documents
		WHERE customer_id = $1 AND shop_id = $2 AND status = 'pending'
		ORDER BY created_at DESC
	`

	return r.scanKYCDocuments(ctx, query, customerID, shopID)
}

// FindPendingByShop retourne tous les documents KYC en attente pour une boutique
func (r *CustomerKYCRepositoryInfrastructure) FindPendingByShop(ctx context.Context, shopID string) ([]*entity.CustomerKYCDocument, error) {
	// Vérification multi-tenant : le shopID demandé doit correspondre au tenant
	currentShopID, err := r.getShopID(ctx)
	if err != nil {
		return nil, err
	}
	if currentShopID != shopID {
		return nil, fmt.Errorf("access denied: shop_id mismatch")
	}

	query := `
		SELECT id, customer_id, shop_id, document_type, file_path,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM customer_kyc_documents
		WHERE shop_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
	`

	return r.scanKYCDocuments(ctx, query, shopID)
}

// CountByCustomer compte le nombre de documents KYC d'un client
func (r *CustomerKYCRepositoryInfrastructure) CountByCustomer(ctx context.Context, customerID string) (int, error) {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM customer_kyc_documents
		WHERE customer_id = $1 AND shop_id = $2
	`

	var count int
	err = r.queryRowContext(ctx, query, customerID, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count kyc documents: %w", err)
	}

	return count, nil
}

// Update met à jour un document KYC
func (r *CustomerKYCRepositoryInfrastructure) Update(ctx context.Context, doc *entity.CustomerKYCDocument) error {
	shopID, err := r.getShopID(ctx)
	if err != nil {
		return err
	}

	if doc.ShopID != shopID {
		return fmt.Errorf("document shop_id does not match tenant shop_id")
	}

	query := `
		UPDATE customer_kyc_documents SET
			status = $1,
			reviewed_by = $2,
			reviewed_at = $3,
			rejection_reason = $4,
			updated_at = NOW()
		WHERE id = $5 AND shop_id = $6
	`

	result, err := r.execContext(ctx, query,
		string(doc.Status),
		doc.ReviewedBy,
		doc.ReviewedAt,
		doc.RejectionReason,
		doc.ID,
		shopID,
	)
	if err != nil {
		return fmt.Errorf("failed to update kyc document: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("kyc document not found")
	}

	return nil
}

// ============================================================
// Helpers de scan
// ============================================================

// scanKYCDocuments scanne plusieurs lignes SQL dans une slice de CustomerKYCDocument
func (r *CustomerKYCRepositoryInfrastructure) scanKYCDocuments(ctx context.Context, query string, args ...interface{}) ([]*entity.CustomerKYCDocument, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query kyc documents: %w", err)
	}
	defer rows.Close()

	var docs []*entity.CustomerKYCDocument
	for rows.Next() {
		doc := &entity.CustomerKYCDocument{}
		var reviewedBy sql.NullString
		var reviewedAt sql.NullTime
		var rejectionReason sql.NullString

		err := rows.Scan(
			&doc.ID,
			&doc.CustomerID,
			&doc.ShopID,
			&doc.DocumentType,
			&doc.FilePath,
			&doc.FileSizeBytes,
			&doc.MimeType,
			&doc.Status,
			&reviewedBy,
			&reviewedAt,
			&rejectionReason,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kyc document row: %w", err)
		}

		if reviewedBy.Valid {
			doc.ReviewedBy = &reviewedBy.String
		}
		if reviewedAt.Valid {
			doc.ReviewedAt = &reviewedAt.Time
		}
		if rejectionReason.Valid {
			doc.RejectionReason = &rejectionReason.String
		}

		docs = append(docs, doc)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Retourner une slice vide plutôt que nil si aucun résultat
	if docs == nil {
		docs = []*entity.CustomerKYCDocument{}
	}

	return docs, nil
}

// ============================================================
// Variable non utilisée (pour éviter warning)
// ============================================================
var _ time.Time // Import time utilisé pour les types NullTime
