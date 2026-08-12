package notification

import (
	"context"

	"Goshop/domain/entity"
	"Goshop/domain/service"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// NoopNotificationService est une implémentation no-op de NotificationService
// qui logue les notifications au lieu de les envoyer réellement.
type NoopNotificationService struct {
	logger zerolog.Logger
}

// NewNoopNotificationService crée une nouvelle instance du service no-op
func NewNoopNotificationService(logger zerolog.Logger) service.NotificationService {
	return &NoopNotificationService{
		logger: logger.With().Str("component", "notification_service").Logger(),
	}
}

func (s *NoopNotificationService) NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	s.logger.Info().Str("notification_type", string(service.NotificationMerchantOrderReceived)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification marchand : nouvelle commande reçue")
	return nil
}

func (s *NoopNotificationService) NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error {
	s.logger.Info().Str("notification_type", string(service.NotificationClientOrderConfirmed)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification client : commande confirmée")
	return nil
}

func (s *NoopNotificationService) NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error {
	s.logger.Info().Str("notification_type", string(service.NotificationClientOrderRejected)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification client : commande rejetée")
	return nil
}

func (s *NoopNotificationService) NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error {
	s.logger.Info().Str("notification_type", string(service.NotificationClientOrderExpired)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification client : commande expirée")
	return nil
}

func (s *NoopNotificationService) NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	s.logger.Info().Str("notification_type", string(service.NotificationMerchantDeliveryReady)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification marchand : préparer la commande")
	return nil
}

func (s *NoopNotificationService) NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error {
	s.logger.Info().Str("notification_type", string(service.NotificationClientOrderDelivered)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification client : commande livrée")
	return nil
}

func (s *NoopNotificationService) NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error {
	s.logger.Info().Str("notification_type", string(service.NotificationMerchantCommissionPaid)).Str("order_id", order.ID).Msg("📱 [NO-OP] Notification marchand : commission prélevée")
	return nil
}

func (s *NoopNotificationService) NotifyClientDisputeResolved(ctx context.Context, customerID, orderID, resolution string, refundedAmount int64) error {
	s.logger.Info().Str("notification_type", "client_dispute_resolved").Str("order_id", orderID).Int64("refunded_amount", refundedAmount).Msg("📱 [NO-OP] Notification client : litige résolu")
	return nil
}

func (s *NoopNotificationService) NotifyMerchantDisputeResolved(ctx context.Context, shopID, orderID, resolution string) error {
	s.logger.Info().Str("notification_type", "merchant_dispute_resolved").Str("order_id", orderID).Msg("📱 [NO-OP] Notification marchand : litige résolu")
	return nil
}

func (s *NoopNotificationService) SendNotification(ctx context.Context, req *service.NotificationRequest) error {
	s.logger.Info().Str("notification_type", string(req.Type)).Msg("📱 [NO-OP] Notification générique envoyée")
	return nil
}

func (s *NoopNotificationService) NotifyOrderStatusChange(ctx context.Context, order *entity.Order, customerID string, shopID uuid.UUID) error {
	s.logger.Info().Str("notification_type", "order_status_change").Str("order_id", order.ID).Msg("📱 [NO-OP] Notification : changement de statut de commande")
	return nil
}

// ============================================================
// 🆕 NOUVELLES MÉTHODES POUR LA TONTINE (NO-OP)
// ============================================================

func (s *NoopNotificationService) NotifyTontineCyclePaid(ctx context.Context, userID, payerName, groupName, amount string) error {
	s.logger.Info().Str("notification_type", "tontine_cycle_paid").Str("user_id", userID).Str("payer", payerName).Str("group", groupName).Msg("📱 [NO-OP] Notification Tontine : cotisation reçue")
	return nil
}

func (s *NoopNotificationService) NotifyTontineTurnSoon(ctx context.Context, userID, groupName, turnDate string) error {
	s.logger.Info().Str("notification_type", "tontine_turn_soon").Str("user_id", userID).Str("group", groupName).Msg("📱 [NO-OP] Notification Tontine : tour approche")
	return nil
}

func (s *NoopNotificationService) NotifyTontineVoucherReady(ctx context.Context, userID, groupName, voucherCode, amountStr string) error {
	s.logger.Info().Str("notification_type", "tontine_voucher_ready").Str("user_id", userID).Str("group", groupName).Str("voucher", voucherCode).Msg("📱 [NO-OP] Notification Tontine : voucher prêt pour le bénéficiaire")
	return nil
}

func (s *NoopNotificationService) NotifyTontineMerchantCycleCompleted(ctx context.Context, ownerUserID, groupName, amountStr, voucherCode string) error {
	s.logger.Info().Str("notification_type", "tontine_merchant_cycle_completed").Str("owner_id", ownerUserID).Str("group", groupName).Str("voucher", voucherCode).Msg("📱 [NO-OP] Notification Tontine : cycle soldé pour le marchand")
	return nil
}

// ============================================================
// 🆕 NOUVELLES MÉTHODES POUR LE CRÉDIT (NO-OP)
// ============================================================

func (s *NoopNotificationService) NotifyCreditInstallmentDue(ctx context.Context, userID, contractID, amount string, daysLeft int) error {
	s.logger.Info().Str("notification_type", "credit_installment_due").Str("user_id", userID).Str("contract_id", contractID).Int("days_left", daysLeft).Msg("📱 [NO-OP] Notification Crédit : échéance à venir")
	return nil
}

func (s *NoopNotificationService) NotifyCreditInstallmentPaid(ctx context.Context, userID, contractID, amount string) error {
	s.logger.Info().Str("notification_type", "credit_installment_paid").Str("user_id", userID).Str("contract_id", contractID).Msg("📱 [NO-OP] Notification Crédit : paiement reçu")
	return nil
}

func (s *NoopNotificationService) NotifyCreditOverdue(ctx context.Context, userID, contractID, amount string, daysOverdue int) error {
	s.logger.Info().Str("notification_type", "credit_overdue").Str("user_id", userID).Str("contract_id", contractID).Int("days_overdue", daysOverdue).Msg("📱 [NO-OP] Notification Crédit : retard de paiement")
	return nil
}
