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

// CancelOrderUsecase gère l'annulation d'une commande
type CancelOrderUsecase struct {
	orderRepo    repository.OrderRepository
	productRepo  repository.ProductRepository
	notifService service.NotificationService
	txManager    repository.TxManager
}

// NewCancelOrderUsecase crée une nouvelle instance
func NewCancelOrderUsecase(
	orderRepo repository.OrderRepository,
	productRepo repository.ProductRepository,
	notifService service.NotificationService,
	txManager repository.TxManager,
) *CancelOrderUsecase {
	return &CancelOrderUsecase{
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		notifService: notifService,
		txManager:    txManager,
	}
}

// Execute annule une commande
func (uc *CancelOrderUsecase) Execute(ctx context.Context, orderID string) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shop.ID.String()).
		Msg("Cancelling order")

	// 2. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 3. Attacher les repositories à la transaction
	orderRepoTx := uc.orderRepo.WithTX(tx)
	productRepoTx := uc.productRepo.WithTX(tx)

	// 4. Récupérer la commande
	order, err := orderRepoTx.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	// 5. Vérifier la transition autorisée
	if !order.CanTransitionTo(entity.OrderStatusCancelled) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	// 6. Marquer comme annulée
	if err = order.MarkCancelled(); err != nil {
		return nil, fmt.Errorf("failed to mark cancelled: %w", err)
	}

	// 7. Mettre à jour en base
	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	// 8. Réincrémentation du stock (si commande cash en pending_confirmation)
	if order.IsCashOnDelivery() && order.Status == string(entity.OrderStatusPendingConfirmation) {
		for _, item := range order.Items {
			product, err := productRepoTx.FindByID(ctx, item.ProductID)
			if err != nil {
				return nil, fmt.Errorf("failed to find product %s: %w", item.ProductID, err)
			}
			product.Stock += item.Quantity
			if _, err = productRepoTx.Update(ctx, product); err != nil {
				return nil, fmt.Errorf("failed to update product %s: %w", item.ProductID, err)
			}
			logger.Debug().
				Str("product_id", item.ProductID).
				Int("quantity_restored", item.Quantity).
				Msg("Stock restored on cancellation")
		}
	}

	// 9. Commit
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("order_id", order.ID).
		Str("status", order.Status).
		Msg("Order cancelled successfully")

	return order, nil
}
