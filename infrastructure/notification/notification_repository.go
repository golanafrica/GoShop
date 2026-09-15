package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

// NotificationRepositoryInfrastructure implémente repository.NotificationRepository
// Utilise PostgreSQL pour la persistance des notifications in-app
type NotificationRepositoryInfrastructure struct {
	db *sql.DB
	tx repository.Tx
}

// NewNotificationRepositoryInfrastructure crée une nouvelle instance du repository
func NewNotificationRepositoryInfrastructure(db *sql.DB) repository.NotificationRepository {
	return &NotificationRepositoryInfrastructure{db: db}
}

// WithTX retourne le repository attaché à une transaction
func (r *NotificationRepositoryInfrastructure) WithTX(tx repository.Tx) repository.NotificationRepository {
	return &NotificationRepositoryInfrastructure{tx: tx, db: r.db}
}

// ============================================================
// Helpers
// ============================================================

func (r *NotificationRepositoryInfrastructure) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if r.tx != nil {
		return r.tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *NotificationRepositoryInfrastructure) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if r.tx != nil {
		return r.tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *NotificationRepositoryInfrastructure) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if r.tx != nil {
		return r.tx.ExecContext(ctx, query, args...)
	}
	return r.db.ExecContext(ctx, query, args...)
}

// scanNotification scanne une ligne SQL dans une entité Notification
func (r *NotificationRepositoryInfrastructure) scanNotification(row *sql.Row) (*entity.Notification, error) {
	n := &entity.Notification{}
	var shopID sql.NullString
	var readAt sql.NullTime
	var expiresAt sql.NullTime

	err := row.Scan(
		&n.ID,
		&n.UserID,
		&shopID,
		&n.NotificationType,
		&n.Title,
		&n.Message,
		&n.Data,
		&n.IsRead,
		&readAt,
		&n.CreatedAt,
		&expiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("failed to scan notification: %w", err)
	}

	// Gestion des champs nullable
	if shopID.Valid {
		n.ShopID = &shopID.String
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}
	if expiresAt.Valid {
		n.ExpiresAt = &expiresAt.Time
	}

	return n, nil
}

// scanNotifications scanne plusieurs lignes SQL dans une slice de notifications
func (r *NotificationRepositoryInfrastructure) scanNotifications(ctx context.Context, query string, args ...interface{}) ([]*entity.Notification, error) {
	rows, err := r.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*entity.Notification
	for rows.Next() {
		n := &entity.Notification{}
		var shopID sql.NullString
		var readAt sql.NullTime
		var expiresAt sql.NullTime

		err := rows.Scan(
			&n.ID,
			&n.UserID,
			&shopID,
			&n.NotificationType,
			&n.Title,
			&n.Message,
			&n.Data,
			&n.IsRead,
			&readAt,
			&n.CreatedAt,
			&expiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification row: %w", err)
		}

		// Gestion des champs nullable
		if shopID.Valid {
			n.ShopID = &shopID.String
		}
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		if expiresAt.Valid {
			n.ExpiresAt = &expiresAt.Time
		}

		notifications = append(notifications, n)
	}

	// Retourner une slice vide plutôt que nil pour la cohérence JSON
	if notifications == nil {
		notifications = []*entity.Notification{}
	}

	return notifications, rows.Err()
}

// ============================================================
// Implémentation de l'interface NotificationRepository
// ============================================================

// Create sauvegarde une nouvelle notification en base de données
func (r *NotificationRepositoryInfrastructure) Create(ctx context.Context, n *entity.Notification) error {
	if n == nil {
		return fmt.Errorf("notification cannot be nil")
	}

	// Validation minimale
	if n.ID == "" || n.UserID == "" || n.NotificationType == "" {
		return fmt.Errorf("notification missing required fields (id, user_id, notification_type)")
	}

	// Préparer les données JSON (fallback sur '{}' si nil)
	dataJSON := n.Data
	if len(dataJSON) == 0 {
		dataJSON = json.RawMessage("{}")
	}

	query := `
		INSERT INTO notifications (
			id, user_id, shop_id, notification_type, title, message, data,
			is_read, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.execContext(ctx, query,
		n.ID,
		n.UserID,
		n.ShopID,
		n.NotificationType,
		n.Title,
		n.Message,
		dataJSON,
		n.IsRead,
		n.CreatedAt,
		n.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// FindByID récupère une notification par son ID
func (r *NotificationRepositoryInfrastructure) FindByID(ctx context.Context, id string) (*entity.Notification, error) {
	if id == "" {
		return nil, fmt.Errorf("notification id cannot be empty")
	}

	query := `
		SELECT 
			id, user_id, shop_id::text, notification_type, title, message, data,
			is_read, read_at, created_at, expires_at
		FROM notifications
		WHERE id = $1
	`

	return r.scanNotification(r.queryRowContext(ctx, query, id))
}

// FindByUserID récupère les notifications d'un utilisateur avec pagination
func (r *NotificationRepositoryInfrastructure) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error) {
	if userID == "" {
		return nil, fmt.Errorf("user id cannot be empty")
	}

	// Validation des paramètres de pagination
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT 
			id, user_id, shop_id::text, notification_type, title, message, data,
			is_read, read_at, created_at, expires_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.scanNotifications(ctx, query, userID, limit, offset)
}

// CountUnread retourne le nombre de notifications non lues pour un utilisateur
func (r *NotificationRepositoryInfrastructure) CountUnread(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("user id cannot be empty")
	}

	query := `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1 AND is_read = false
	`

	var count int
	err := r.queryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}

	return count, nil
}

// MarkAsRead marque une notification spécifique comme lue
func (r *NotificationRepositoryInfrastructure) MarkAsRead(ctx context.Context, notificationID, userID string) error {
	if notificationID == "" || userID == "" {
		return fmt.Errorf("notification id and user id are required")
	}

	query := `
		UPDATE notifications
		SET is_read = true, read_at = NOW()
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.execContext(ctx, query, notificationID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notification not found or not owned by user")
	}

	return nil
}

// MarkAllAsRead marque toutes les notifications d'un utilisateur comme lues
func (r *NotificationRepositoryInfrastructure) MarkAllAsRead(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user id cannot be empty")
	}

	query := `
		UPDATE notifications
		SET is_read = true, read_at = NOW()
		WHERE user_id = $1 AND is_read = false
	`

	_, err := r.execContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// DeleteExpired supprime les notifications qui ont expiré
func (r *NotificationRepositoryInfrastructure) DeleteExpired(ctx context.Context) (int, error) {
	query := `
		DELETE FROM notifications
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
	`

	result, err := r.execContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired notifications: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}
