package orderusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// OutForDeliveryUsecase gère le passage en livraison d'une commande
type OutForDeliveryUsecase struct {
	orderRepo    repository.OrderRepository
	notifService service.NotificationService
	txManager    repository.TxManager
}

// NewOutForDeliveryUsecase crée une nouvelle instance
func NewOutForDeliveryUsecase(
	orderRepo repository.OrderRepository,
	txManager repository.TxManager,
	notifService service.NotificationService,
) *OutForDeliveryUsecase {
	return &OutForDeliveryUsecase{
		orderRepo:    orderRepo,
		notifService: notifService,
		txManager:    txManager,
	}
}

// Execute passe la commande en livraison
func (uc *OutForDeliveryUsecase) Execute(ctx context.Context, orderID string) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().Str("order_id", orderID).Str("shop_id", shop.ID.String()).Msg("Marking order as out for delivery")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	orderRepoTx := uc.orderRepo.WithTX(tx)

	order, err := orderRepoTx.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	if order.PaymentMethod != string(entity.PaymentMethodCashOnDelivery) && order.PaymentMethod != string(entity.PaymentMethodMobileMoney) {
		return nil, fmt.Errorf("order payment method not supported for this flow")
	}

	if !order.CanTransitionTo(entity.OrderStatusOutForDelivery) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	if err = order.MarkOutForDelivery(); err != nil {
		return nil, fmt.Errorf("failed to mark out for delivery: %w", err)
	}

	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 🆕 NOTIFICATIONS TEMPS RÉEL (APPEL SYNCHRONE)
	// ✅ CORRECTION : Pas de "go func()" ici !
	// La méthode NotifyOrderStatusChange fera les requêtes DB avec le ctx valide,
	// puis gérera elle-même l'asynchronisme (go func) pour l'envoi WebSocket/Email.
	_ = uc.notifService.NotifyOrderStatusChange(ctx, order, order.CustomerID, shop.ID)

	logger.Info().Str("order_id", order.ID).Str("status", order.Status).Msg("Order marked as out for delivery")
	return order, nil
}
