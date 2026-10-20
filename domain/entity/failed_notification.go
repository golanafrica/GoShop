package entity

import (
	"encoding/json"
	"time"
)

type FailedNotification struct {
	ID               string
	NotificationType string
	ShopID           *string
	Payload          json.RawMessage // Contient le NotificationRequest sérialisé
	ErrorMessage     *string
	RetryCount       int
	MaxRetries       int
	NextRetryAt      time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
