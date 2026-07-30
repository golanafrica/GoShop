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

// AcceptOrderUsecase gère l'acceptation d'une commande par le marchand
type AcceptOrderUsecase struct {
	orderRepo    repository.OrderRepository
	productRepo  repository.ProductRepository
	notifService service.NotificationService
	txManager    repository.TxManager
}

// NewAcceptOrderUsecase crée une nouvelle instance
func NewAcceptOrderUsecase(
	orderRepo repository.OrderRepository,
	productRepo repository.ProductRepository,
	notifService service.NotificationService,
	txManager repository.TxManager,
) *AcceptOrderUsecase {
	return &AcceptOrderUsecase{
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		notifService: notifService,
		txManager:    txManager,
	}
}

// Execute accepte une commande
func (uc *AcceptOrderUsecase) Execute(ctx context.Context, orderID string) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().Str("order_id", orderID).Str("shop_id", shop.ID.String()).Msg("Accepting order")

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
	productRepoTx := uc.productRepo.WithTX(tx)

	order, err := orderRepoTx.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	if order.PaymentMethod != string(entity.PaymentMethodCashOnDelivery) && order.PaymentMethod != string(entity.PaymentMethodMobileMoney) {
		return nil, fmt.Errorf("order payment method not supported for this flow")
	}

	if order.IsExpired() {
		logger.Warn().Str("order_id", order.ID).Time("reserved_until", *order.ReservedUntil).Msg("Order has expired, auto-expiring and restoring stock")

		if err = order.MarkExpired(); err != nil {
			return nil, fmt.Errorf("failed to mark expired: %w", err)
		}
		if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to update order: %w", err)
		}
		if err = uc.restoreStock(ctx, productRepoTx, order); err != nil {
			return nil, fmt.Errorf("failed to restore stock: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return nil, fmt.Errorf("order has expired")
	}

	if !order.CanTransitionTo(entity.OrderStatusConfirmed) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	if err = order.MarkAccepted(); err != nil {
		return nil, fmt.Errorf("failed to mark accepted: %w", err)
	}

	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 🆕 NOTIFICATION TEMPS RÉEL (Fire-and-forget)
	go func() {
		bgCtx := context.Background()
		_ = uc.notifService.NotifyOrderStatusChange(bgCtx, order, order.CustomerID, shop.ID)
	}()

	logger.Info().Str("order_id", order.ID).Str("status", order.Status).Msg("Order accepted successfully")
	return order, nil
}

func (uc *AcceptOrderUsecase) restoreStock(ctx context.Context, productRepo repository.ProductRepository, order *entity.Order) error {
	logger := zerolog.Ctx(ctx)
	logger.Info().Str("order_id", order.ID).Int("items_count", len(order.Items)).Msg("Restoring stock for expired order")

	for _, item := range order.Items {
		product, err := productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			return fmt.Errorf("failed to find product %s: %w", item.ProductID, err)
		}
		product.Stock += item.Quantity
		if _, err = productRepo.Update(ctx, product); err != nil {
			return fmt.Errorf("failed to update product %s: %w", item.ProductID, err)
		}
		logger.Info().Str("product_id", item.ProductID).Int("quantity_restored", item.Quantity).Int("new_stock", product.Stock).Msg("✅ Stock restored on expiration")
	}
	return nil
}
