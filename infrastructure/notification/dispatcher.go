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
	wsHub           *wsinfra.Hub
	emailProvider   NotificationProvider
	telegramService *TelegramService
	notifRepo       repository.NotificationRepository // 🆕 AJOUTÉ : Pour la persistance
	customerRepo    repository.CustomerRepositoryInterface
	shopRepo        repository.ShopRepository
	userRepo        userrepository.UserRepository
	logger          zerolog.Logger
}

// NewNotificationDispatcher crée une nouvelle instance du dispatcher
func NewNotificationDispatcher(
	wsHub *wsinfra.Hub,
	emailProvider NotificationProvider,
	telegramService *TelegramService,
	notifRepo repository.NotificationRepository, // 🆕 AJOUTÉ
	customerRepo repository.CustomerRepositoryInterface,
	shopRepo repository.ShopRepository,
	userRepo userrepository.UserRepository,
	logger zerolog.Logger,
) service.NotificationService {
	return &NotificationDispatcher{
		wsHub:           wsHub,
		emailProvider:   emailProvider,
		telegramService: telegramService,
		notifRepo:       notifRepo, // 🆕 AJOUTÉ
		customerRepo:    customerRepo,
		shopRepo:        shopRepo,
		userRepo:        userRepo,
		logger:          logger.With().Str("component", "notification_dispatcher").Logger(),
	}
}

// ============================================================
// NOUVEAU HELPER : Sauvegarde + Envoi
// ============================================================

// saveAndNotify sauvegarde la notification en base de données puis l'envoie via WebSocket
func (d *NotificationDispatcher) saveAndNotify(userID, notifType, title, message string, data map[string]interface{}) {
	if userID == "" {
		return
	}

	// 1. Créer l'entité notification avec un ID unique
	notif := entity.NewNotification(uuid.New().String(), userID, notifType, title, message, data)

	// 2. Sauvegarder en base de données de manière asynchrone (fire-and-forget)
	if d.notifRepo != nil {
		go func() {
			bgCtx := context.Background()
			if err := d.notifRepo.Create(bgCtx, notif); err != nil {
				d.logger.Error().Err(err).Str("user_id", userID).Str("type", notifType).Msg("Failed to save notification to DB")
			}
		}()
	}

	// 3. Envoyer en temps réel via WebSocket
	d.sendWebSocketNotification(context.Background(), userID, notifType, title, message, data)
}

// ============================================================
// MÉTHODES D'ENVOI ASYNCHRONES (Canaux)
// ============================================================

// sendWebSocketNotification envoie une notification via WebSocket de manière asynchrone
func (d *NotificationDispatcher) sendWebSocketNotification(_ context.Context, userID string, eventType, title, message string, data map[string]interface{}) {
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
		bgCtx := context.Background()
		if err := d.wsHub.SendToUser(bgCtx, userID, msg); err != nil {
			d.logger.Error().Err(err).Str("user_id", userID).Str("event", eventType).Msg("Failed to send WebSocket notification")
		} else {
			d.logger.Debug().Str("user_id", userID).Str("event", eventType).Msg("WebSocket notification sent successfully")
		}
	}()
}

// sendEmailNotification envoie une notification par email de manière asynchrone
func (d *NotificationDispatcher) sendEmailNotification(_ context.Context, userEmail, title, message string, data map[string]interface{}) {
	if d.emailProvider == nil || userEmail == "" {
		return
	}

	go func() {
		bgCtx := context.Background()
		if err := d.emailProvider.SendClientNotification(bgCtx, userEmail, title, message, data); err != nil {
			d.logger.Error().Err(err).Str("email", userEmail).Msg("Failed to send email notification")
		} else {
			d.logger.Info().Str("email", userEmail).Msg("Email notification sent successfully")
		}
	}()
}

// sendTelegramNotification envoie une notification via Telegram de manière asynchrone
func (d *NotificationDispatcher) sendTelegramNotification(_ context.Context, chatID, title, message string, data map[string]interface{}) {
	if d.telegramService == nil || chatID == "" {
		return
	}

	telegramMsg := fmt.Sprintf("<b>🔔 %s</b>\n\n%s", title, message)
	if len(data) > 0 {
		telegramMsg += "\n📊 <b>Détails :</b>\n"
		for key, value := range data {
			telegramMsg += fmt.Sprintf("• %s : %v\n", key, value)
		}
	}

	go func() {
		bgCtx := context.Background()
		if err := d.telegramService.SendMessage(bgCtx, chatID, telegramMsg); err != nil {
			d.logger.Error().Err(err).Str("chat_id", chatID).Msg("Failed to send Telegram notification")
		} else {
			d.logger.Info().Str("chat_id", chatID).Msg("Telegram notification sent successfully")
		}
	}()
}

// ============================================================
// HELPERS DE RÉCUPÉRATION D'UTILISATEURS
// ============================================================

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

	if userID != "" {
		eventType := "client_order_" + string(order.Status)
		d.saveAndNotify(userID, eventType, title, message, data) // 🆕 Sauvegarde + WS
		d.sendEmailNotification(ctx, userEmail, title, message, data)
		d.sendTelegramNotification(ctx, d.telegramService.adminChatID, title, message, data) // Alerte admin
	}

	shop, err := d.shopRepo.FindByID(ctx, shopID)
	if err == nil && shop != nil {
		eventType := "merchant_order_" + string(order.Status)
		d.saveAndNotify(shop.OwnerID, eventType, title, message, data) // 🆕 Sauvegarde + WS
		d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	}

	return nil
}

func (d *NotificationDispatcher) NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	title := "Nouvelle commande reçue"
	message := fmt.Sprintf("Vous avez une nouvelle commande en attente (#%s).", order.ID)
	data := map[string]interface{}{"order_id": order.ID}

	d.saveAndNotify(shop.OwnerID, string(service.NotificationMerchantOrderReceived), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error {
	title := "Commande confirmée"
	message := "Votre commande a été acceptée par le marchand."
	data := map[string]interface{}{"order_id": order.ID}

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID != "" {
		d.saveAndNotify(userID, string(service.NotificationClientOrderConfirmed), title, message, data) // 🆕
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
		d.saveAndNotify(userID, string(service.NotificationClientOrderRejected), title, message, data) // 🆕
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
		d.saveAndNotify(userID, string(service.NotificationClientOrderExpired), title, message, data) // 🆕
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, order.CustomerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	title := "Commande prête à être livrée"
	message := fmt.Sprintf("La commande #%s est prête pour la livraison.", order.ID)
	data := map[string]interface{}{"order_id": order.ID}

	d.saveAndNotify(shop.OwnerID, string(service.NotificationMerchantDeliveryReady), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error {
	title := "Commande livrée"
	message := fmt.Sprintf("Votre commande a été livrée avec succès. Montant reçu : %d FCFA", amountReceived/100)
	data := map[string]interface{}{"order_id": order.ID, "amount": amountReceived}

	userID := d.getUserIDFromCustomerID(ctx, order.CustomerID)
	if userID != "" {
		d.saveAndNotify(userID, string(service.NotificationClientOrderDelivered), title, message, data) // 🆕
	}
	d.sendEmailNotification(ctx, d.getUserEmail(ctx, order.CustomerID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error {
	title := "Commission prélevée"
	message := fmt.Sprintf("Une commission de %d FCFA a été prélevée sur la commande #%s.", commissionCents/100, order.ID)
	data := map[string]interface{}{"order_id": order.ID, "commission": commissionCents}

	d.saveAndNotify(shop.OwnerID, string(service.NotificationMerchantCommissionPaid), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getShopEmail(ctx, shop.ID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyClientDisputeResolved(ctx context.Context, customerID, orderID, resolution string, refundedAmount int64) error {
	title := "Litige résolu"
	message := fmt.Sprintf("Votre litige concernant la commande #%s a été traité en votre faveur. Un remboursement de %d FCFA a été initié.", orderID, refundedAmount/100)
	data := map[string]interface{}{"order_id": orderID, "resolution": resolution, "refunded_amount": refundedAmount / 100}

	userID := d.getUserIDFromCustomerID(ctx, customerID)
	if userID != "" {
		d.saveAndNotify(userID, string(service.NotificationClientDisputeResolved), title, message, data) // 🆕
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
		d.saveAndNotify(shop.OwnerID, string(service.NotificationMerchantDisputeResolved), title, message, data) // 🆕
		d.sendEmailNotification(ctx, d.getShopEmail(ctx, shopUUID), title, message, data)
	} else {
		d.logger.Warn().Err(err).Str("shop_id", shopID).Msg("Cannot send notification: shop not found")
	}
	return nil
}

func (d *NotificationDispatcher) SendNotification(ctx context.Context, req *service.NotificationRequest) error {
	title := string(req.Type)
	message := "Notification"

	// Pour SendNotification générique, on essaie de sauvegarder si on a un userID (ici on utilise RecipientPhone comme fallback userID pour l'exemple, à adapter selon ton usage)
	if req.RecipientPhone != "" {
		d.saveAndNotify(req.RecipientPhone, string(req.Type), title, message, req.Data)
	}
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

	d.saveAndNotify(userID, string(service.NotificationTontineCyclePaid), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyTontineTurnSoon(ctx context.Context, userID, groupName, turnDate string) error {
	title := "Votre tour approche !"
	message := fmt.Sprintf("Préparez-vous, votre tour de recevoir le bien du groupe '%s' est prévu le %s.", groupName, turnDate)
	data := map[string]interface{}{"group_name": groupName, "turn_date": turnDate}

	d.saveAndNotify(userID, string(service.NotificationTontineTurnSoon), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyTontineVoucherReady(ctx context.Context, userID, groupName, voucherCode, amountStr string) error {
	title := "🎉 C'est votre tour !"
	message := fmt.Sprintf("Félicitations ! Votre tour est arrivé pour le groupe '%s'. Votre voucher (code: %s) d'un montant de %s FCFA est prêt.", groupName, voucherCode, amountStr)
	data := map[string]interface{}{"group_name": groupName, "voucher_code": voucherCode, "amount": amountStr}

	d.saveAndNotify(userID, string(service.NotificationTontineVoucherReady), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyTontineMerchantCycleCompleted(ctx context.Context, ownerUserID, groupName, amountStr, voucherCode string) error {
	title := "Cycle Tontine soldé"
	message := fmt.Sprintf("Le cycle du groupe '%s' est terminé. Le montant de %s FCFA a été crédité sur votre wallet. Voucher généré : %s.", groupName, amountStr, voucherCode)
	data := map[string]interface{}{"group_name": groupName, "amount": amountStr, "voucher_code": voucherCode}

	d.saveAndNotify(ownerUserID, string(service.NotificationTontineMerchantCycleCompleted), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(ownerUserID), title, message, data)
	return nil
}

// ============================================================
// 🆕 NOUVELLES MÉTHODES POUR LE CRÉDIT
// ============================================================

func (d *NotificationDispatcher) NotifyCreditInstallmentDue(ctx context.Context, userID, contractID, amount string, daysLeft int) error {
	title := "Rappel d'échéance de crédit"
	message := fmt.Sprintf("Votre échéance de %s FCFA arrive dans %d jour(s). Pensez à effectuer votre paiement pour éviter les pénalités.", amount, daysLeft)
	data := map[string]interface{}{"contract_id": contractID, "amount": amount, "days_left": daysLeft}

	d.saveAndNotify(userID, string(service.NotificationCreditInstallmentDue), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyCreditInstallmentPaid(ctx context.Context, userID, contractID, amount string) error {
	title := "Paiement de crédit reçu"
	message := fmt.Sprintf("Merci ! Votre paiement de %s FCFA a été enregistré avec succès.", amount)
	data := map[string]interface{}{"contract_id": contractID, "amount": amount}

	d.saveAndNotify(userID, string(service.NotificationCreditInstallmentPaid), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}

func (d *NotificationDispatcher) NotifyCreditOverdue(ctx context.Context, userID, contractID, amount string, daysOverdue int) error {
	title := "⚠️ Alerte : Retard de paiement"
	message := fmt.Sprintf("Votre échéance de %s FCFA est en retard de %d jour(s). Veuillez régulariser votre situation dès que possible.", amount, daysOverdue)
	data := map[string]interface{}{"contract_id": contractID, "amount": amount, "days_overdue": daysOverdue}

	d.saveAndNotify(userID, string(service.NotificationCreditOverdue), title, message, data) // 🆕
	d.sendEmailNotification(ctx, d.getUserEmailByID(userID), title, message, data)
	return nil
}
