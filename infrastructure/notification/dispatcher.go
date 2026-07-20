package notification

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	wsinfra "Goshop/infrastructure/websocket"

	"github.com/rs/zerolog"
)

// NotificationDispatcher implémente service.NotificationService
// Il orchestre l'envoi de notifications via WebSocket, Email, etc.
type NotificationDispatcher struct {
	wsHub        *wsinfra.Hub
	emailSvc     service.EmailService
	customerRepo repository.CustomerRepositoryInterface // ✅ AJOUTÉ
	logger       zerolog.Logger
}

// NewNotificationDispatcher crée une nouvelle instance du dispatcher
func NewNotificationDispatcher(
	wsHub *wsinfra.Hub,
	emailSvc service.EmailService,
	customerRepo repository.CustomerRepositoryInterface, // ✅ AJOUTÉ
	logger zerolog.Logger,
) service.NotificationService {
	return &NotificationDispatcher{
		wsHub:        wsHub,
		emailSvc:     emailSvc,
		customerRepo: customerRepo, // ✅ AJOUTÉ
		logger:       logger.With().Str("component", "notification_dispatcher").Logger(),
	}
}

// sendWebSocketNotification envoie une notification via WebSocket de manière asynchrone
func (d *NotificationDispatcher) sendWebSocketNotification(ctx context.Context, userID string, eventType, title, message string, data map[string]interface{}) {
	if d.wsHub == nil || userID == "" {
		return
	}

	msg := wsinfra.NotificationMessage{
		Type:    eventType,
		Title:   title,
		Message: message,
		Data:    data,
	}

	// Envoi asynchrone (goroutine) pour ne jamais bloquer le usecase métier
	go func() {
		if err := d.wsHub.SendToUser(ctx, userID, msg); err != nil {
			d.logger.Error().Err(err).Str("user_id", userID).Str("event", eventType).Msg("Failed to send WebSocket notification")
		} else {
			d.logger.Debug().Str("user_id", userID).Str("event", eventType).Msg("WebSocket notification sent successfully")
		}
	}()
}

// ============================================================
// Implémentation de l'interface service.NotificationService
// ============================================================

func (d *NotificationDispatcher) NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	title := "Nouvelle commande reçue"
	message := fmt.Sprintf("Vous avez une nouvelle commande en attente (#%s).", order.ID)

	d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantOrderReceived), title, message, map[string]interface{}{
		"order_id": order.ID,
	})
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error {
	title := "Commande confirmée"
	message := "Votre commande a été acceptée par le marchand."

	// ✅ Récupérer le UserID à partir du CustomerID
	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID == "" {
		d.logger.Warn().Str("customer_id", order.CustomerID).Msg("Cannot send WS notification: no linked user_id")
		return nil
	}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderConfirmed), title, message, map[string]interface{}{
		"order_id": order.ID,
	})
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error {
	title := "Commande refusée"
	message := fmt.Sprintf("Votre commande a été refusée. Raison : %s", reason)

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID == "" {
		d.logger.Warn().Str("customer_id", order.CustomerID).Msg("Cannot send WS notification: no linked user_id")
		return nil
	}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderRejected), title, message, map[string]interface{}{
		"order_id": order.ID,
		"reason":   reason,
	})
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error {
	title := "Commande expirée"
	message := "Le délai de paiement de votre commande a expiré."

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID == "" {
		d.logger.Warn().Str("customer_id", order.CustomerID).Msg("Cannot send WS notification: no linked user_id")
		return nil
	}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderExpired), title, message, map[string]interface{}{
		"order_id": order.ID,
	})
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	title := "Commande prête à être livrée"
	message := fmt.Sprintf("La commande #%s est prête pour la livraison.", order.ID)

	d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantDeliveryReady), title, message, map[string]interface{}{
		"order_id": order.ID,
	})
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error {
	title := "Commande livrée"
	message := fmt.Sprintf("Votre commande a été livrée avec succès. Montant reçu : %d FCFA", amountReceived/100)

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID == "" {
		d.logger.Warn().Str("customer_id", order.CustomerID).Msg("Cannot send WS notification: no linked user_id")
		return nil
	}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderDelivered), title, message, map[string]interface{}{
		"order_id": order.ID,
		"amount":   amountReceived,
	})
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error {
	title := "Commission prélevée"
	message := fmt.Sprintf("Une commission de %d FCFA a été prélevée sur la commande #%s.", commissionCents/100, order.ID)

	d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantCommissionPaid), title, message, map[string]interface{}{
		"order_id":   order.ID,
		"commission": commissionCents,
	})
	return nil
}

func (d *NotificationDispatcher) SendNotification(ctx context.Context, req *service.NotificationRequest) error {
	// Méthode générique pour les cas particuliers non couverts par les méthodes spécifiques
	d.sendWebSocketNotification(ctx, req.RecipientPhone, string(req.Type), string(req.Type), "Notification", req.Data)
	return nil
}

// ============================================================
// ✅ NOUVELLE MÉTHODE HELPER : Fait le pont entre CustomerID et UserID
// ============================================================
func (d *NotificationDispatcher) getUserIDFromCustomerID(ctx context.Context, customerID string) string {
	if d.customerRepo == nil || customerID == "" {
		return ""
	}

	// ✅ CORRECTION : Utilisation de FindByCustomerID qui est la méthode réelle de ton interface
	customer, err := d.customerRepo.FindByCustomerID(ctx, customerID)
	if err != nil || customer == nil {
		d.logger.Warn().Err(err).Str("customer_id", customerID).Msg("Failed to find customer to resolve user_id")
		return ""
	}

	if customer.UserID == "" {
		d.logger.Warn().Str("customer_id", customerID).Msg("Customer exists but has no linked user_id")
		return ""
	}

	return customer.UserID
}
