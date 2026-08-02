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

// 🆕 getUserEmailByID récupère l'email directement depuis le userID (plus efficace pour Tontine/Crédit)
func (d *NotificationDispatcher) getUserEmailByID(userID string) string {
	if d.userRepo == nil || userID == "" {
		return ""
	}
	user, err := d.userRepo.FindUserByID(userID)
	if err == nil && user != nil {
		return user.Email
	}
	return ""
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
	return d.getUserEmailByID(customer.UserID)
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
	return d.getUserEmailByID(shop.OwnerID)
}

// ============================================================
// MÉTHODES GÉNÉRIQUES POUR LES COMMANDES
// ============================================================

func (d *NotificationDispatcher) NotifyOrderStatusChange(ctx context.Context, order *entity.Order, customerID string, shopID uuid.UUID) error {
	title, message := d.buildOrderStatusMessages(string(order.Status))
	data := map[string]interface{}{
		"order_id": order.ID,
		"status":   order.Status,
	}

	userID := d.getUserIDFromCustomerID(ctx, customerID)
	userEmail := d.getUserEmail(ctx, customerID)

	go func() {
		bgCtx := context.Background()
		if userID != "" {
			eventType := "client_order_" + string(order.Status)
			d.sendWebSocketNotification(bgCtx, userID, eventType, title, message, data)
			d.sendEmailNotification(bgCtx, userEmail, title, message, data)
		}
	}()

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
// MÉTHODES EXISTANTES (Commandes & Litiges)
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

func (d *NotificationDispatcher) NotifyClientDisputeResolved(ctx context.Context, customerID, orderID, resolution string, refundedAmount int64) error {
	title := "Litige résolu"
	message := fmt.Sprintf("Votre litige concernant la commande #%s a été traité en votre faveur. Un remboursement de %d FCFA (montant net après déduction des frais de transaction) a été initié vers votre compte.", orderID, refundedAmount/100)
	data := map[string]interface{}{"order_id": orderID, "resolution": resolution, "refunded_amount": refundedAmount / 100}

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

func (d *NotificationDispatcher) SendNotification(ctx context.Context, req *service.NotificationRequest) error {
	title := string(req.Type)
	message := "Notification"

	d.sendWebSocketNotification(ctx, req.RecipientPhone, string(req.Type), title, message, req.Data)
	d.sendEmailNotification(ctx, req.RecipientEmail, title, message, req.Data)
	return nil
}

// ============================================================
// 🆕 NOUVELLES MÉTHODES POUR LA TONTINE
// ============================================================

func (d *NotificationDispatcher) NotifyTontineCyclePaid(ctx context.Context, userID, payerName, groupName, amount string) error {
	title := "Cotisation Tontine reçue"
	message := fmt.Sprintf("Le membre %s a réglé sa cotisation de %s FCFA pour le groupe '%s'.", payerName, amount, groupName)
	data := map[string]interface{}{"group_name": groupName, "amount": amount, "payer_name": payerName}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationTontineCyclePaid), title, message, data)
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyTontineTurnSoon(ctx context.Context, userID, groupName, turnDate string) error {
	title := "Votre tour approche !"
	message := fmt.Sprintf("Préparez-vous, votre tour de recevoir le bien du groupe '%s' est prévu le %s.", groupName, turnDate)
	data := map[string]interface{}{"group_name": groupName, "turn_date": turnDate}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationTontineTurnSoon), title, message, data)
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

// ============================================================
// 🆕 NOUVELLES MÉTHODES POUR LE CRÉDIT
// ============================================================

func (d *NotificationDispatcher) NotifyCreditInstallmentDue(ctx context.Context, userID, contractID, amount string, daysLeft int) error {
	title := "Rappel d'échéance de crédit"
	message := fmt.Sprintf("Votre échéance de %s FCFA arrive dans %d jour(s). Pensez à effectuer votre paiement pour éviter les pénalités.", amount, daysLeft)
	data := map[string]interface{}{"contract_id": contractID, "amount": amount, "days_left": daysLeft}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationCreditInstallmentDue), title, message, data)
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyCreditInstallmentPaid(ctx context.Context, userID, contractID, amount string) error {
	title := "Paiement de crédit reçu"
	message := fmt.Sprintf("Merci ! Votre paiement de %s FCFA a été enregistré avec succès.", amount)
	data := map[string]interface{}{"contract_id": contractID, "amount": amount}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationCreditInstallmentPaid), title, message, data)
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyCreditOverdue(ctx context.Context, userID, contractID, amount string, daysOverdue int) error {
	title := "⚠️ Alerte : Retard de paiement"
	message := fmt.Sprintf("Votre échéance de %s FCFA est en retard de %d jour(s). Veuillez régulariser votre situation dès que possible.", amount, daysOverdue)
	data := map[string]interface{}{"contract_id": contractID, "amount": amount, "days_overdue": daysOverdue}

	d.sendWebSocketNotification(ctx, userID, string(service.NotificationCreditOverdue), title, message, data)
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
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
