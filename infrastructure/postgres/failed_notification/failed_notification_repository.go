package failednotification

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

type FailedNotificationRepositoryPostgres struct {
	db *sql.DB
}

func NewFailedNotificationRepositoryPostgres(db *sql.DB) repository.FailedNotificationRepository {
	return &FailedNotificationRepositoryPostgres{db: db}
}

func (r *FailedNotificationRepositoryPostgres) SaveFailedNotification(ctx context.Context, notif *entity.FailedNotification) error {
	query := `
		INSERT INTO failed_notifications (id, notification_type, shop_id, payload, error_message, retry_count, max_retries, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '5 minutes')
	`
	_, err := r.db.ExecContext(ctx, query,
		uuid.New().String(), notif.NotificationType, notif.ShopID, notif.Payload, notif.ErrorMessage, notif.RetryCount, notif.MaxRetries,
	)
	return err
}

func (r *FailedNotificationRepositoryPostgres) FindRetryableNotifications(ctx context.Context, limit int) ([]*entity.FailedNotification, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, notification_type, shop_id, payload, error_message, retry_count, max_retries, next_retry_at, created_at, updated_at
		FROM failed_notifications
		WHERE retry_count < max_retries AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("find retryable notifications: %w", err)
	}
	defer rows.Close()

	var notifs []*entity.FailedNotification
	for rows.Next() {
		var n entity.FailedNotification
		var shopID sql.NullString
		var errMsg sql.NullString
		if err := rows.Scan(&n.ID, &n.NotificationType, &shopID, &n.Payload, &errMsg, &n.RetryCount, &n.MaxRetries, &n.NextRetryAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan failed notification: %w", err)
		}
		if shopID.Valid {
			n.ShopID = &shopID.String
		}
		if errMsg.Valid {
			n.ErrorMessage = &errMsg.String
		}
		notifs = append(notifs, &n)
	}
	return notifs, rows.Err()
}

func (r *FailedNotificationRepositoryPostgres) IncrementRetryCount(ctx context.Context, id string, errorMsg string) error {
	query := `
		UPDATE failed_notifications
		SET retry_count = retry_count + 1,
		    next_retry_at = NOW() + (retry_count + 1) * INTERVAL '10 minutes', -- Backoff exponentiel simple
		    error_message = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, errorMsg)
	return err
}

func (r *FailedNotificationRepositoryPostgres) MarkAsPermanentFailure(ctx context.Context, id string) error {
	query := `UPDATE failed_notifications SET updated_at = NOW() WHERE id = $1` // On garde l'historique, on incrémente juste plus
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *FailedNotificationRepositoryPostgres) MarkAsSuccess(ctx context.Context, id string) error {
	query := `DELETE FROM failed_notifications WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
