package service

//go:generate mockgen -destination=../../mocks/service/mock_notification_service.go -package=service . NotificationService

import (
	"context"

	"Goshop/domain/entity"

	"github.com/google/uuid"
)

// NotificationType représente le type de notification
type NotificationType string

const (
	NotificationMerchantOrderReceived  NotificationType = "merchant_order_received"
	NotificationClientOrderConfirmed   NotificationType = "client_order_confirmed"
	NotificationClientOrderRejected    NotificationType = "client_order_rejected"
	NotificationClientOrderExpired     NotificationType = "client_order_expired"
	NotificationMerchantDeliveryReady  NotificationType = "merchant_delivery_ready"
	NotificationClientOrderDelivered   NotificationType = "client_order_delivered"
	NotificationMerchantCommissionPaid NotificationType = "merchant_commission_paid"

	// 🆕 NOUVEAUX TYPES POUR LES LITIGES
	NotificationClientDisputeResolved   NotificationType = "client_dispute_resolved"
	NotificationMerchantDisputeResolved NotificationType = "merchant_dispute_resolved"
)

// NotificationRequest représente une demande de notification
type NotificationRequest struct {
	Type           NotificationType
	RecipientPhone string
	RecipientEmail string
	ShopID         string
	OrderID        string
	Data           map[string]interface{}
}

// NotificationService définit le contrat pour l'envoi de notifications
type NotificationService interface {
	NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error
	NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error
	NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error
	NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error
	NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error
	NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error
	NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error

	// 🆕 Notifications pour les litiges
	NotifyClientDisputeResolved(ctx context.Context, customerID, orderID, resolution string) error
	NotifyMerchantDisputeResolved(ctx context.Context, shopID, orderID, resolution string) error

	SendNotification(ctx context.Context, req *NotificationRequest) error

	// 🆕 AJOUT : Méthode générique pour les changements de statut de commande (Option A)
	NotifyOrderStatusChange(ctx context.Context, order *entity.Order, customerID string, shopID uuid.UUID) error
}
