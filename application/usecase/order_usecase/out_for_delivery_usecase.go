package orderusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// OutForDeliveryUsecase gère le passage en livraison d'une commande
type OutForDeliveryUsecase struct {
	orderRepo repository.OrderRepository
	txManager repository.TxManager
}

// NewOutForDeliveryUsecase crée une nouvelle instance
func NewOutForDeliveryUsecase(
	orderRepo repository.OrderRepository,
	txManager repository.TxManager,
) *OutForDeliveryUsecase {
	return &OutForDeliveryUsecase{
		orderRepo: orderRepo,
		txManager: txManager,
	}
}

// Execute passe la commande en livraison
func (uc *OutForDeliveryUsecase) Execute(ctx context.Context, orderID string) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shop.ID.String()).
		Msg("Marking order as out for delivery")

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

	// 3. Attacher le repository à la transaction
	orderRepoTx := uc.orderRepo.WithTX(tx)

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
	if !order.CanTransitionTo(entity.OrderStatusOutForDelivery) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	// 7. Marquer comme en livraison
	if err = order.MarkOutForDelivery(); err != nil {
		return nil, fmt.Errorf("failed to mark out for delivery: %w", err)
	}

	// 8. Mettre à jour en base
	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	// 9. Commit
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("order_id", order.ID).
		Str("status", order.Status).
		Msg("Order marked as out for delivery")

	return order, nil
}
