package notification

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

// NoopNotificationService est une implémentation no-op de NotificationService
// qui logue les notifications au lieu de les envoyer réellement.
// Utilisé en développement et comme fallback quand aucun provider n'est configuré.
type NoopNotificationService struct {
	logger zerolog.Logger
}

// NewNoopNotificationService crée une nouvelle instance du service no-op
func NewNoopNotificationService(logger zerolog.Logger) *NoopNotificationService {
	return &NoopNotificationService{
		logger: logger.With().Str("component", "notification_service").Logger(),
	}
}

// NotifyMerchantOrderReceived logue la notification au marchand
func (s *NoopNotificationService) NotifyMerchantOrderReceived(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationMerchantOrderReceived)).
		Str("shop_id", shop.ID.String()).
		Str("shop_name", shop.Name).
		Str("order_id", order.ID).
		Int64("total_cents", order.TotalCents).
		Msg("📱 [NO-OP] Notification marchand : nouvelle commande cash reçue")
	return nil
}

// NotifyClientOrderConfirmed logue la notification au client
func (s *NoopNotificationService) NotifyClientOrderConfirmed(ctx context.Context, order *entity.Order, customerPhone string) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationClientOrderConfirmed)).
		Str("order_id", order.ID).
		Str("customer_phone", customerPhone).
		Int64("total_cents", order.TotalCents).
		Msg("📱 [NO-OP] Notification client : commande confirmée")
	return nil
}

// NotifyClientOrderRejected logue la notification de rejet
func (s *NoopNotificationService) NotifyClientOrderRejected(ctx context.Context, order *entity.Order, customerPhone string, reason string) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationClientOrderRejected)).
		Str("order_id", order.ID).
		Str("customer_phone", customerPhone).
		Str("reason", reason).
		Msg("📱 [NO-OP] Notification client : commande rejetée")
	return nil
}

// NotifyClientOrderExpired logue la notification d'expiration
func (s *NoopNotificationService) NotifyClientOrderExpired(ctx context.Context, order *entity.Order, customerPhone string) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationClientOrderExpired)).
		Str("order_id", order.ID).
		Str("customer_phone", customerPhone).
		Msg("📱 [NO-OP] Notification client : commande expirée")
	return nil
}

// NotifyMerchantDeliveryReady logue la notification de préparation
func (s *NoopNotificationService) NotifyMerchantDeliveryReady(ctx context.Context, shop *entity.Shop, order *entity.Order) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationMerchantDeliveryReady)).
		Str("shop_id", shop.ID.String()).
		Str("order_id", order.ID).
		Msg("📱 [NO-OP] Notification marchand : préparer la commande pour livraison")
	return nil
}

// NotifyClientOrderDelivered logue la notification de livraison
func (s *NoopNotificationService) NotifyClientOrderDelivered(ctx context.Context, order *entity.Order, customerPhone string, amountReceived int64) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationClientOrderDelivered)).
		Str("order_id", order.ID).
		Str("customer_phone", customerPhone).
		Int64("amount_received", amountReceived).
		Msg("📱 [NO-OP] Notification client : commande livrée et payée")
	return nil
}

// NotifyMerchantCommissionPaid logue la notification de commission
func (s *NoopNotificationService) NotifyMerchantCommissionPaid(ctx context.Context, shop *entity.Shop, order *entity.Order, commissionCents int64) error {
	s.logger.Info().
		Str("notification_type", string(service.NotificationMerchantCommissionPaid)).
		Str("shop_id", shop.ID.String()).
		Str("order_id", order.ID).
		Int64("commission_cents", commissionCents).
		Str("commission_xof", fmt.Sprintf("%.2f", float64(commissionCents)/100)).
		Msg("📱 [NO-OP] Notification marchand : commission GoShop prélevée")
	return nil
}

// SendNotification logue une notification générique
func (s *NoopNotificationService) SendNotification(ctx context.Context, req *service.NotificationRequest) error {
	s.logger.Info().
		Str("notification_type", string(req.Type)).
		Str("recipient_phone", req.RecipientPhone).
		Str("recipient_email", req.RecipientEmail).
		Str("shop_id", req.ShopID).
		Str("order_id", req.OrderID).
		Interface("data", req.Data).
		Msg("📱 [NO-OP] Notification générique envoyée")
	return nil
}
