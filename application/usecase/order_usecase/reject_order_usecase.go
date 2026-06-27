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

// RejectOrderUsecase gère le rejet d'une commande cash par le marchand
type RejectOrderUsecase struct {
	orderRepo    repository.OrderRepository
	productRepo  repository.ProductRepository
	notifService service.NotificationService
	txManager    repository.TxManager
}

// NewRejectOrderUsecase crée une nouvelle instance
func NewRejectOrderUsecase(
	orderRepo repository.OrderRepository,
	productRepo repository.ProductRepository,
	notifService service.NotificationService,
	txManager repository.TxManager,
) *RejectOrderUsecase {
	return &RejectOrderUsecase{
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		notifService: notifService,
		txManager:    txManager,
	}
}

// Execute rejette une commande cash
func (uc *RejectOrderUsecase) Execute(ctx context.Context, orderID string, reason string) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shop.ID.String()).
		Str("reason", reason).
		Msg("Rejecting cash order")

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

	// 5. Vérifier que c'est une commande cash
	if !order.IsCashOnDelivery() {
		return nil, fmt.Errorf("order is not cash on delivery")
	}

	// 6. Vérifier la transition autorisée
	if !order.CanTransitionTo(entity.OrderStatusRejected) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	// 7. Marquer comme rejetée
	if err = order.MarkRejected(); err != nil {
		return nil, fmt.Errorf("failed to mark rejected: %w", err)
	}

	// 8. Mettre à jour en base
	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	// 9. Réincrémentation du stock (IMPORTANT)
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
			Msg("Stock restored")
	}

	// 10. Commit
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 11. Notification (hors transaction)
	customerPhone := "" // TODO: récupérer depuis customer repo
	if err = uc.notifService.NotifyClientOrderRejected(ctx, order, customerPhone, reason); err != nil {
		logger.Warn().Err(err).Msg("failed to send notification")
	}

	logger.Info().
		Str("order_id", order.ID).
		Str("status", order.Status).
		Msg("Order rejected successfully")

	return order, nil
}
