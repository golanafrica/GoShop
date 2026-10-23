package repository

import (
	"context"

	"Goshop/domain/entity"
)

type FailedNotificationRepository interface {
	SaveFailedNotification(ctx context.Context, notif *entity.FailedNotification) error

	// FetchAndClaimRetryableNotifications récupère ET verrouille atomiquement les notifications.
	// Utilise FOR UPDATE SKIP LOCKED pour éviter les courses entre instances multiples.
	FetchAndClaimRetryableNotifications(ctx context.Context, limit int) ([]*entity.FailedNotification, error)

	IncrementRetryCount(ctx context.Context, id string, errorMsg string) error
	MarkAsPermanentFailure(ctx context.Context, id string) error
	MarkAsSuccess(ctx context.Context, id string) error
}
