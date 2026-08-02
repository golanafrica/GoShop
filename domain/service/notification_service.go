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
	// --- Commandes ---
	NotificationMerchantOrderReceived  NotificationType = "merchant_order_received"
	NotificationClientOrderConfirmed   NotificationType = "client_order_confirmed"
	NotificationClientOrderRejected    NotificationType = "client_order_rejected"
	NotificationClientOrderExpired     NotificationType = "client_order_expired"
	NotificationMerchantDeliveryReady  NotificationType = "merchant_delivery_ready"
	NotificationClientOrderDelivered   NotificationType = "client_order_delivered"
	NotificationMerchantCommissionPaid NotificationType = "merchant_commission_paid"

	// --- Litiges ---
	NotificationClientDisputeResolved   NotificationType = "client_dispute_resolved"
	NotificationMerchantDisputeResolved NotificationType = "merchant_dispute_resolved"

	// 🆕 --- Tontine ---
	NotificationTontineCyclePaid NotificationType = "tontine_cycle_paid"
	NotificationTontineTurnSoon  NotificationType = "tontine_turn_soon"

	// 🆕 --- Crédit ---
	NotificationCreditInstallmentDue  NotificationType = "credit_installment_due"
	NotificationCreditInstallmentPaid NotificationType = "credit_installment_paid"
	NotificationCreditOverdue         NotificationType = "credit_overdue"
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
	// --- Commandes ---
	NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error
	NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error
	NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error
	NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error
	NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error
	NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error
	NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error

	// --- Litiges ---
	NotifyClientDisputeResolved(ctx context.Context, customerID, orderID, resolution string, refundedAmount int64) error
	NotifyMerchantDisputeResolved(ctx context.Context, shopID, orderID, resolution string) error

	// 🆕 --- Tontine ---
	// Notifie un membre que son cycle a été payé avec succès
	NotifyTontineCyclePaid(ctx context.Context, userID, payerName, groupName, amount string) error
	// Notifie un membre que son tour de recevoir la cagnotte approche
	NotifyTontineTurnSoon(ctx context.Context, userID, groupName, turnDate string) error

	// 🆕 --- Crédit ---
	// Rappel X jours avant l'échéance
	NotifyCreditInstallmentDue(ctx context.Context, userID, contractID, amount string, daysLeft int) error
	// Confirmation de paiement d'une échéance
	NotifyCreditInstallmentPaid(ctx context.Context, userID, contractID, amount string) error
	// Alerte de retard de paiement
	NotifyCreditOverdue(ctx context.Context, userID, contractID, amount string, daysOverdue int) error

	// --- Générique ---
	SendNotification(ctx context.Context, req *NotificationRequest) error

	// --- Méthode générique pour les changements de statut de commande ---
	NotifyOrderStatusChange(ctx context.Context, order *entity.Order, customerID string, shopID uuid.UUID) error
}
