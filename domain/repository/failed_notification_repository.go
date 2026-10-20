package repository

import (
	"Goshop/domain/entity"
	"context"
)

type FailedNotificationRepository interface {
	SaveFailedNotification(ctx context.Context, notif *entity.FailedNotification) error
	FindRetryableNotifications(ctx context.Context, limit int) ([]*entity.FailedNotification, error)
	IncrementRetryCount(ctx context.Context, id string, errorMsg string) error
	MarkAsPermanentFailure(ctx context.Context, id string) error
	MarkAsSuccess(ctx context.Context, id string) error
}
