package service

import (
	"context"

	"Goshop/domain/entity"
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
)

// NotificationRequest représente une demande de notification
type NotificationRequest struct {
	Type           NotificationType
	RecipientPhone string // Numéro du destinataire (format international : +226...)
	RecipientEmail string // Email du destinataire (optionnel)
	ShopID         string
	OrderID        string
	Data           map[string]interface{} // Données spécifiques à la notification
}

// NotificationService définit le contrat pour l'envoi de notifications
// Cette interface permet de découpler les usecases du provider réel (SMS, email, etc.)
type NotificationService interface {
	// NotifyMerchantOrderReceived notifie le marchand qu'une nouvelle commande cash a été reçue
	NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error

	// NotifyClientOrderConfirmed notifie le client que sa commande a été acceptée
	NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error

	// NotifyClientOrderRejected notifie le client que sa commande a été refusée
	NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error

	// NotifyClientOrderExpired notifie le client que sa commande a expiré
	NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error

	// NotifyMerchantDeliveryReady notifie le marchand qu'il doit préparer la commande
	NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error

	// NotifyClientOrderDelivered notifie le client que sa commande a été livrée
	NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error

	// NotifyMerchantCommissionPaid notifie le marchand de la commission GoShop prélevée
	NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error

	// SendNotification envoie une notification générique (pour cas particuliers)
	SendNotification(ctx context.Context, req *NotificationRequest) error
}
