package notification

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/domain/service"
	wsinfra "Goshop/infrastructure/websocket"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// NotificationDispatcher implémente service.NotificationService
type NotificationDispatcher struct {
	wsHub         *wsinfra.Hub
	emailProvider NotificationProvider
	customerRepo  repository.CustomerRepositoryInterface
	shopRepo      repository.ShopRepository
	userRepo      userrepository.UserRepository
	logger        zerolog.Logger
}

// NewNotificationDispatcher crée une nouvelle instance du dispatcher
func NewNotificationDispatcher(
	wsHub *wsinfra.Hub,
	emailProvider NotificationProvider,
	customerRepo repository.CustomerRepositoryInterface,
	shopRepo repository.ShopRepository,
	userRepo userrepository.UserRepository,
	logger zerolog.Logger,
) service.NotificationService {
	return &NotificationDispatcher{
		wsHub:         wsHub,
		emailProvider: emailProvider,
		customerRepo:  customerRepo,
		shopRepo:      shopRepo,
		userRepo:      userRepo,
		logger:        logger.With().Str("component", "notification_dispatcher").Logger(),
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

	go func() {
		if err := d.wsHub.SendToUser(ctx, userID, msg); err != nil {
			d.logger.Error().Err(err).Str("user_id", userID).Str("event", eventType).Msg("Failed to send WebSocket notification")
		} else {
			d.logger.Debug().Str("user_id", userID).Str("event", eventType).Msg("WebSocket notification sent successfully")
		}
	}()
}

// sendEmailNotification envoie une notification par email de manière asynchrone
func (d *NotificationDispatcher) sendEmailNotification(ctx context.Context, userEmail, title, message string, data map[string]interface{}) {
	if d.emailProvider == nil || userEmail == "" {
		return
	}

	go func() {
		if err := d.emailProvider.SendClientNotification(ctx, userEmail, title, message, data); err != nil {
			d.logger.Error().Err(err).Str("email", userEmail).Msg("Failed to send email notification")
		} else {
			d.logger.Info().Str("email", userEmail).Msg("Email notification sent successfully")
		}
	}()
}

// getUserEmail récupère l'email d'un utilisateur à partir de son CustomerID
func (d *NotificationDispatcher) getUserEmail(ctx context.Context, customerID string) string {
	if d.customerRepo == nil || customerID == "" {
		return ""
	}
	customer, err := d.customerRepo.FindByCustomerID(ctx, customerID)
	if err != nil || customer == nil || customer.UserID == "" {
		return ""
	}

	if d.userRepo != nil {
		user, err := d.userRepo.FindUserByID(customer.UserID)
		if err == nil && user != nil {
			return user.Email
		}
	}
	return ""
}

// getShopEmail récupère l'email d'un marchand à partir de son ShopID
func (d *NotificationDispatcher) getShopEmail(ctx context.Context, shopID uuid.UUID) string {
	if d.shopRepo == nil {
		return ""
	}
	shop, err := d.shopRepo.FindByID(ctx, shopID)
	if err != nil || shop == nil || shop.OwnerID == "" {
		return ""
	}

	if d.userRepo != nil {
		user, err := d.userRepo.FindUserByID(shop.OwnerID)
		if err == nil && user != nil {
			return user.Email
		}
	}
	return ""
}

// ============================================================
// 🆕 MÉTHODE GÉNÉRIQUE POUR LES COMMANDES (Option A)
// ============================================================

// NotifyOrderStatusChange notifie le client et le marchand d'un changement de statut de commande.
func (d *NotificationDispatcher) NotifyOrderStatusChange(ctx context.Context, order *entity.Order, customerID string, shopID uuid.UUID) error {
	title, message := d.buildOrderStatusMessages(string(order.Status))
	data := map[string]interface{}{
		"order_id": order.ID,
		"status":   order.Status,
	}

	// 1. Récupérer les infos du client AVANT la goroutine
	userID := d.getUserIDFromCustomerID(ctx, customerID)
	userEmail := d.getUserEmail(ctx, customerID)

	// 2. Notification au Client (Asynchrone)
	go func() {
		bgCtx := context.Background()
		if userID != "" {
			eventType := "client_order_" + string(order.Status)
			d.sendWebSocketNotification(bgCtx, userID, eventType, title, message, data)
			d.sendEmailNotification(bgCtx, userEmail, title, message, data)
		}
	}()

	// 3. Notification au Marchand (Asynchrone)
	go func() {
		bgCtx := context.Background()
		shop, err := d.shopRepo.FindByID(bgCtx, shopID)
		if err == nil && shop != nil {
			eventType := "merchant_order_" + string(order.Status)
			d.sendWebSocketNotification(bgCtx, shop.OwnerID, eventType, title, message, data)
			d.sendEmailNotification(bgCtx, d.getShopEmail(bgCtx, shop.ID), title, message, data)
		}
	}()

	return nil
}

// ============================================================
// MÉTHODES EXISTANTES
// ============================================================

func (d *NotificationDispatcher) NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	title := "Nouvelle commande reçue"
	message := fmt.Sprintf("Vous avez une nouvelle commande en attente (#%s).", order.ID)
	data := map[string]interface{}{"order_id": order.ID}

	d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantOrderReceived), title, message, data)
	d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error {
	title := "Commande confirmée"
	message := "Votre commande a été acceptée par le marchand."
	data := map[string]interface{}{"order_id": order.ID}

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID != "" {
		d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderConfirmed), title, message, data)
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, order.CustomerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error {
	title := "Commande refusée"
	message := fmt.Sprintf("Votre commande a été refusée. Raison : %s", reason)
	data := map[string]interface{}{"order_id": order.ID, "reason": reason}

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID != "" {
		d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderRejected), title, message, data)
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, order.CustomerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error {
	title := "Commande expirée"
	message := "Le délai de paiement de votre commande a expiré."
	data := map[string]interface{}{"order_id": order.ID}

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID != "" {
		d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderExpired), title, message, data)
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, order.CustomerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	title := "Commande prête à être livrée"
	message := fmt.Sprintf("La commande #%s est prête pour la livraison.", order.ID)
	data := map[string]interface{}{"order_id": order.ID}

	d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantDeliveryReady), title, message, data)
	d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error {
	title := "Commande livrée"
	message := fmt.Sprintf("Votre commande a été livrée avec succès. Montant reçu : %d FCFA", amountReceived/100)
	data := map[string]interface{}{"order_id": order.ID, "amount": amountReceived}

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID != "" {
		d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientOrderDelivered), title, message, data)
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, order.CustomerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error {
	title := "Commission prélevée"
	message := fmt.Sprintf("Une commission de %d FCFA a été prélevée sur la commande #%s.", commissionCents/100, order.ID)
	data := map[string]interface{}{"order_id": order.ID, "commission": commissionCents}

	d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantCommissionPaid), title, message, data)
	d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) SendNotification(ctx context.Context, req *service.NotificationRequest) error {
	title := string(req.Type)
	message := "Notification"

	d.sendWebSocketNotification(ctx, req.RecipientPhone, string(req.Type), title, message, req.Data)
	d.sendEmailNotification(ctx, req.RecipientEmail, title, message, req.Data)
	return nil
}

// 🆕 NotifyClientDisputeResolved avec montant remboursé pour la transparence
func (d *NotificationDispatcher) NotifyClientDisputeResolved(ctx context.Context, customerID, orderID, resolution string, refundedAmount int64) error {
	title := "Litige résolu"
	// Message transparent expliquant la déduction des frais
	message := fmt.Sprintf("Votre litige concernant la commande #%s a été traité en votre faveur. Un remboursement de %d FCFA (montant net après déduction des frais de transaction) a été initié vers votre compte.", orderID, refundedAmount/100)

	data := map[string]interface{}{
		"order_id":        orderID,
		"resolution":      resolution,
		"refunded_amount": refundedAmount / 100, // En FCFA pour le frontend
	}

	userID := d.getUserIDFromCustomerID(ctx, customerID)
	if userID != "" {
		d.sendWebSocketNotification(ctx, userID, string(service.NotificationClientDisputeResolved), title, message, data)
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, customerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantDisputeResolved(ctx context.Context, shopID, orderID, resolution string) error {
	shopUUID, err := uuid.Parse(shopID)
	if err != nil {
		d.logger.Warn().Err(err).Str("shop_id", shopID).Msg("Cannot send notification: invalid shop UUID")
		return nil
	}

	title := "Litige résolu"
	message := fmt.Sprintf("Le litige concernant la commande #%s a été traité. Statut : %s.", orderID, resolution)
	data := map[string]interface{}{"order_id": orderID, "resolution": resolution}

	shop, err := d.shopRepo.FindByID(ctx, shopUUID)
	if err == nil && shop != nil {
		d.sendWebSocketNotification(ctx, shop.OwnerID, string(service.NotificationMerchantDisputeResolved), title, message, data)
		d.sendEmailNotification(ctx, d.getShopEmail(ctx, shopUUID), title, message, data)
	} else {
		d.logger.Warn().Err(err).Str("shop_id", shopID).Msg("Cannot send notification: shop not found")
	}
	return nil
}

// ============================================================
// HELPERS
// ============================================================

func (d *NotificationDispatcher) getUserIDFromCustomerID(ctx context.Context, customerID string) string {
	if d.customerRepo == nil || customerID == "" {
		return ""
	}
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

// buildOrderStatusMessages génère le titre et le message en fonction du statut
func (d *NotificationDispatcher) buildOrderStatusMessages(status string) (string, string) {
	switch status {
	case "confirmed":
		return "Commande confirmée", "Votre commande a été confirmée et est en cours de préparation."
	case "out_for_delivery":
		return "Commande en cours de livraison", "Votre commande a été expédiée et est en chemin."
	case "delivered":
		return "Commande livrée", "Votre commande a été livrée avec succès. Merci de votre confiance !"
	default:
		return "Mise à jour de commande", fmt.Sprintf("Le statut de votre commande est maintenant : %s.", status)
	}
}
