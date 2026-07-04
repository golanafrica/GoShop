package shop

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

// ShopKYCDocumentRepositoryInfrastructure implémente repository.ShopKYCDocumentRepository
type ShopKYCDocumentRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewShopKYCDocumentRepositoryInfrastructure crée une nouvelle instance
func NewShopKYCDocumentRepositoryInfrastructure(db *sql.DB) repository.ShopKYCDocumentRepository {
	return &ShopKYCDocumentRepositoryInfrastructure{db: db}
}

// WithTX retourne un nouveau repository avec transaction
func (r *ShopKYCDocumentRepositoryInfrastructure) WithTX(tx repository.Tx) repository.ShopKYCDocumentRepository {
	return &ShopKYCDocumentRepositoryInfrastructure{tx: tx, db: r.db}
}

func (r *ShopKYCDocumentRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

func (r *ShopKYCDocumentRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *ShopKYCDocumentRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

// ============================================================
// CREATE
// ============================================================

// Create crée un nouveau document KYC
func (r *ShopKYCDocumentRepositoryInfrastructure) Create(ctx context.Context, doc *entity.ShopKYCDocument) error {
	query := `
		INSERT INTO shop_kyc_documents (
			id, shop_id, document_type, file_path, file_name,
			file_size_bytes, mime_type, status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.execContext(ctx, query,
		doc.ID,
		doc.ShopID,
		doc.DocumentType,
		doc.FilePath,
		doc.FileName,
		doc.FileSizeBytes,
		doc.MimeType,
		doc.Status,
		doc.CreatedAt,
		doc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create kyc document: %w", err)
	}
	return nil
}

// ============================================================
// FIND METHODS
// ============================================================

// FindByID retourne un document par son ID
func (r *ShopKYCDocumentRepositoryInfrastructure) FindByID(ctx context.Context, id uuid.UUID) (*entity.ShopKYCDocument, error) {
	query := `
		SELECT id, shop_id, document_type, file_path, file_name,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM shop_kyc_documents
		WHERE id = $1
	`
	return r.scanDocument(r.queryRowContext(ctx, query, id))
}

// FindByShopID retourne tous les documents d'un shop
func (r *ShopKYCDocumentRepositoryInfrastructure) FindByShopID(ctx context.Context, shopID uuid.UUID) ([]*entity.ShopKYCDocument, error) {
	query := `
		SELECT id, shop_id, document_type, file_path, file_name,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM shop_kyc_documents
		WHERE shop_id = $1
		ORDER BY created_at DESC
	`
	return r.scanDocuments(ctx, query, shopID)
}

// FindPendingByShopID retourne les documents en attente d'un shop
func (r *ShopKYCDocumentRepositoryInfrastructure) FindPendingByShopID(ctx context.Context, shopID uuid.UUID) ([]*entity.ShopKYCDocument, error) {
	query := `
		SELECT id, shop_id, document_type, file_path, file_name,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM shop_kyc_documents
		WHERE shop_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
	`
	return r.scanDocuments(ctx, query, shopID)
}

// FindByTypeAndShop retourne un document spécifique par type et shop
func (r *ShopKYCDocumentRepositoryInfrastructure) FindByTypeAndShop(
	ctx context.Context,
	shopID uuid.UUID,
	docType entity.ShopKYCDocumentType,
) (*entity.ShopKYCDocument, error) {
	query := `
		SELECT id, shop_id, document_type, file_path, file_name,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM shop_kyc_documents
		WHERE shop_id = $1 AND document_type = $2
		LIMIT 1
	`
	return r.scanDocument(r.queryRowContext(ctx, query, shopID, docType))
}

// FindAllPending retourne tous les documents en attente (dashboard admin)
func (r *ShopKYCDocumentRepositoryInfrastructure) FindAllPending(
	ctx context.Context,
	limit, offset int,
) ([]*entity.ShopKYCDocument, error) {
	query := `
		SELECT id, shop_id, document_type, file_path, file_name,
		       file_size_bytes, mime_type, status,
		       reviewed_by, reviewed_at, rejection_reason,
		       created_at, updated_at
		FROM shop_kyc_documents
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2
	`
	return r.scanDocuments(ctx, query, limit, offset)
}

// ============================================================
// UPDATE / DELETE
// ============================================================

// Update met à jour un document (statut, review, etc.)
func (r *ShopKYCDocumentRepositoryInfrastructure) Update(ctx context.Context, doc *entity.ShopKYCDocument) error {
	query := `
		UPDATE shop_kyc_documents
		SET status = $2,
		    reviewed_by = $3,
		    reviewed_at = $4,
		    rejection_reason = $5,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.execContext(ctx, query,
		doc.ID,
		doc.Status,
		doc.ReviewedBy,
		doc.ReviewedAt,
		doc.RejectionReason,
	)
	if err != nil {
		return fmt.Errorf("update kyc document: %w", err)
	}
	return nil
}

// Delete supprime un document
func (r *ShopKYCDocumentRepositoryInfrastructure) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM shop_kyc_documents WHERE id = $1`
	_, err := r.execContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete kyc document: %w", err)
	}
	return nil
}

// DeleteByShopID supprime tous les documents d'un shop
func (r *ShopKYCDocumentRepositoryInfrastructure) DeleteByShopID(ctx context.Context, shopID uuid.UUID) error {
	query := `DELETE FROM shop_kyc_documents WHERE shop_id = $1`
	_, err := r.execContext(ctx, query, shopID)
	if err != nil {
		return fmt.Errorf("delete kyc documents by shop: %w", err)
	}
	return nil
}

// ============================================================
// COUNT METHODS
// ============================================================

// CountByShopID compte les documents d'un shop
func (r *ShopKYCDocumentRepositoryInfrastructure) CountByShopID(ctx context.Context, shopID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM shop_kyc_documents WHERE shop_id = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count kyc documents: %w", err)
	}
	return count, nil
}

// CountPendingByShopID compte les documents en attente d'un shop
func (r *ShopKYCDocumentRepositoryInfrastructure) CountPendingByShopID(ctx context.Context, shopID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM shop_kyc_documents WHERE shop_id = $1 AND status = 'pending'`
	var count int
	err := r.db.QueryRowContext(ctx, query, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending kyc documents: %w", err)
	}
	return count, nil
}

// ============================================================
// SCAN HELPERS
// ============================================================

// scanDocument scanne une ligne depuis sql.Row
func (r *ShopKYCDocumentRepositoryInfrastructure) scanDocument(row *sql.Row) (*entity.ShopKYCDocument, error) {
	doc := &entity.ShopKYCDocument{}
	var reviewedBy sql.NullString
	var reviewedAt sql.NullTime
	var rejectionReason sql.NullString

	err := row.Scan(
		&doc.ID,
		&doc.ShopID,
		&doc.DocumentType,
		&doc.FilePath,
		&doc.FileName,
		&doc.FileSizeBytes,
		&doc.MimeType,
		&doc.Status,
		&reviewedBy,
		&reviewedAt,
		&rejectionReason,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
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

// scanDocuments scanne plusieurs lignes
func (r *ShopKYCDocumentRepositoryInfrastructure) scanDocuments(ctx context.Context, query string, args ...interface{}) ([]*entity.ShopKYCDocument, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*entity.ShopKYCDocument
	for rows.Next() {
		doc := &entity.ShopKYCDocument{}
		var reviewedBy sql.NullString
		var reviewedAt sql.NullTime
		var rejectionReason sql.NullString

		err := rows.Scan(
			&doc.ID,
			&doc.ShopID,
			&doc.DocumentType,
			&doc.FilePath,
			&doc.FileName,
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
			return nil, err
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

	return docs, rows.Err()
}
