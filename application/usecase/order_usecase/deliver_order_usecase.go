package orderusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DeliverOrderUsecase gère la livraison et le paiement cash
type DeliverOrderUsecase struct {
	orderRepo    repository.OrderRepository
	paymentRepo  repository.PaymentRepository
	shopRepo     repository.ShopRepository
	notifService service.NotificationService
	txManager    repository.TxManager
}

// NewDeliverOrderUsecase crée une nouvelle instance
func NewDeliverOrderUsecase(
	orderRepo repository.OrderRepository,
	paymentRepo repository.PaymentRepository,
	shopRepo repository.ShopRepository,
	notifService service.NotificationService,
	txManager repository.TxManager,
) *DeliverOrderUsecase {
	return &DeliverOrderUsecase{
		orderRepo:    orderRepo,
		paymentRepo:  paymentRepo,
		shopRepo:     shopRepo,
		notifService: notifService,
		txManager:    txManager,
	}
}

// DeliverRequest représente la requête de livraison
type DeliverRequest struct {
	AmountReceived int64  // Montant reçu en cash (centimes)
	Notes          string // Notes de livraison
}

// Execute marque la commande comme livrée et crée le paiement cash
func (uc *DeliverOrderUsecase) Execute(ctx context.Context, orderID string, req *DeliverRequest) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shop.ID.String()).
		Int64("amount_received", req.AmountReceived).
		Msg("Delivering cash order")

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
	paymentRepoTx := uc.paymentRepo.WithTX(tx)
	shopRepoTx := uc.shopRepo.WithTX(tx)

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
	if !order.CanTransitionTo(entity.OrderStatusDelivered) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	// 7. Marquer comme livrée
	if err = order.MarkDelivered(req.AmountReceived, req.Notes); err != nil {
		return nil, fmt.Errorf("failed to mark delivered: %w", err)
	}

	// 8. Récupérer la config de la boutique pour la commission
	settings, err := shopRepoTx.GetPaymentSettings(ctx, shop.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop settings: %w", err)
	}

	commissionRate := settings.GetCashCommissionRate() // basis points
	commissionFees, netAmount := order.CalculateCashCommission(commissionRate)

	logger.Info().
		Str("order_id", order.ID).
		Int("commission_rate_bp", commissionRate).
		Int64("commission_fees", commissionFees).
		Int64("net_amount", netAmount).
		Msg("Commission calculated")

	// 9. Créer le Payment cash
	orderUUID, err := uuid.Parse(order.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}

	payment, err := entity.NewPayment(
		shop.ID,
		orderUUID,
		entity.ProviderCash,
		req.AmountReceived,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment entity: %w", err)
	}

	payment.Status = entity.PaymentStatusSuccess
	payment.Metadata = map[string]interface{}{
		"payment_type":    "cash_on_delivery",
		"commission_fees": commissionFees,
		"commission_rate": commissionRate,
		"net_amount":      netAmount,
		"delivery_notes":  req.Notes,
	}

	if err = paymentRepoTx.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// 10. Mettre à jour la commande
	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	// 11. Commit
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 12. Notifications (hors transaction)
	customerPhone := "" // TODO: récupérer depuis customer repo
	if err = uc.notifService.NotifyClientOrderDelivered(ctx, order, customerPhone, req.AmountReceived); err != nil {
		logger.Warn().Err(err).Msg("failed to send client notification")
	}

	if err = uc.notifService.NotifyMerchantCommissionPaid(ctx, shop, order, commissionFees); err != nil {
		logger.Warn().Err(err).Msg("failed to send merchant notification")
	}

	logger.Info().
		Str("order_id", order.ID).
		Str("payment_id", payment.ID.String()).
		Str("status", order.Status).
		Int64("commission_fees", commissionFees).
		Msg("Order delivered and payment created")

	return order, nil
}
