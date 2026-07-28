package orderusecase

import (
	"context"
	"database/sql"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DeliverOrderUsecase gère la livraison et le déblocage des fonds séquestre
type DeliverOrderUsecase struct {
	orderRepo     repository.OrderRepository
	paymentRepo   repository.PaymentRepository
	shopRepo      repository.ShopRepository
	escrowRepo    repository.EscrowAccountRepository
	walletRepo    repository.MerchantWalletRepository
	walletTxnRepo repository.WalletTransactionRepository
	notifService  service.NotificationService
	txManager     repository.TxManager
	disputeRepo   repository.DisputeRepository // 🆕 AJOUTÉ pour la sécurité métier
}

// NewDeliverOrderUsecase crée une nouvelle instance
func NewDeliverOrderUsecase(
	orderRepo repository.OrderRepository,
	paymentRepo repository.PaymentRepository,
	shopRepo repository.ShopRepository,
	escrowRepo repository.EscrowAccountRepository,
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
	notifService service.NotificationService,
	txManager repository.TxManager,
	disputeRepo repository.DisputeRepository, // 🆕 AJOUTÉ
) *DeliverOrderUsecase {
	return &DeliverOrderUsecase{
		orderRepo:     orderRepo,
		paymentRepo:   paymentRepo,
		shopRepo:      shopRepo,
		escrowRepo:    escrowRepo,
		walletRepo:    walletRepo,
		walletTxnRepo: walletTxnRepo,
		notifService:  notifService,
		txManager:     txManager,
		disputeRepo:   disputeRepo, // 🆕 AJOUTÉ
	}
}

// DeliverRequest représente la requête de livraison
type DeliverRequest struct {
	AmountReceived int64  // Montant reçu en cash (centimes)
	Notes          string // Notes de livraison
}

// Execute marque la commande comme livrée et libère les fonds du séquestre vers le wallet
func (uc *DeliverOrderUsecase) Execute(ctx context.Context, orderID string, req *DeliverRequest) (*entity.Order, error) {
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("order_id", orderID).
		Str("shop_id", shop.ID.String()).
		Int64("amount_received", req.AmountReceived).
		Msg("Delivering order and releasing escrow")

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
	paymentRepoTx := uc.paymentRepo.WithTX(tx)
	escrowRepoTx := uc.escrowRepo.WithTX(tx)
	walletRepoTx := uc.walletRepo.WithTX(tx)
	walletTxnRepoTx := uc.walletTxnRepo.WithTX(tx)

	// 1. Récupérer et vérifier la commande
	order, err := orderRepoTx.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	// 🛡️ SÉCURITÉ MÉTIER : Bloquer la livraison si un litige actif existe sur cette commande
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}

	if uc.disputeRepo != nil {
		hasActiveDispute, _ := uc.disputeRepo.ExistsByOrderID(ctx, orderUUID)
		if hasActiveDispute {
			return nil, fmt.Errorf("cannot deliver order: an active dispute is pending resolution")
		}
	}

	// 🆕 Autoriser la livraison pour Mobile Money (pour libérer l'Escrow) ET Cash on Delivery
	if order.PaymentMethod != string(entity.PaymentMethodMobileMoney) && order.PaymentMethod != string(entity.PaymentMethodCashOnDelivery) {
		return nil, fmt.Errorf("order payment method not supported for this delivery flow")
	}

	if !order.CanTransitionTo(entity.OrderStatusDelivered) {
		return nil, fmt.Errorf("invalid status transition from %s", order.Status)
	}

	if err = order.MarkDelivered(req.AmountReceived, req.Notes); err != nil {
		return nil, fmt.Errorf("failed to mark delivered: %w", err)
	}

	// 2. 🆕 Gestion du paiement (Uniquement pour COD, car Mobile Money est déjà payé)
	if order.PaymentMethod == string(entity.PaymentMethodCashOnDelivery) {
		settings, err := uc.shopRepo.WithTX(tx).GetPaymentSettings(ctx, shop.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get shop settings: %w", err)
		}

		commissionRate := settings.GetCashCommissionRate()
		commissionFees, netAmount := order.CalculateCashCommission(commissionRate)

		payment, err := entity.NewPayment(shop.ID, orderUUID, entity.ProviderCash, req.AmountReceived)
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
	}

	// 3. 🛡️ LOGIQUE DE DÉBLOCAGE DU SÉQUESTRE (ESCROW) - Idempotente
	escrow, err := escrowRepoTx.FindByOrderID(ctx, order.ID)
	if err == nil && escrow != nil {
		if escrow.Status == entity.EscrowAccountFundsHeld {
			// Marquer comme libéré
			if err := escrow.ReleaseFunds(); err != nil {
				return nil, fmt.Errorf("failed to release escrow funds: %w", err)
			}
			if err := escrowRepoTx.Update(ctx, escrow); err != nil {
				return nil, fmt.Errorf("failed to update escrow status: %w", err)
			}

			// Créditer le wallet du marchand avec le montant NET (Total - Commission)
			merchantAmount := escrow.GetMerchantAmount()

			wallet, err := walletRepoTx.FindByShopIDForUpdate(ctx, shop.ID.String())
			if err != nil {
				if err.Error() == "merchant wallet not found" {
					wallet = entity.NewMerchantWallet(shop.ID.String())
					if err := walletRepoTx.Create(ctx, wallet); err != nil {
						return nil, fmt.Errorf("failed to create wallet: %w", err)
					}
				} else {
					return nil, fmt.Errorf("failed to find wallet: %w", err)
				}
			}

			if err := wallet.Credit(merchantAmount); err != nil {
				return nil, fmt.Errorf("failed to credit wallet: %w", err)
			}
			if err := walletRepoTx.Update(ctx, wallet); err != nil {
				return nil, fmt.Errorf("failed to update wallet: %w", err)
			}

			// Enregistrer la transaction
			txnID := uuid.New().String()
			refType := "order"
			txn := &entity.WalletTransaction{
				ID:                txnID,
				ShopID:            shop.ID.String(),
				TransactionType:   entity.WalletTxSaleCredit,
				AmountCents:       merchantAmount,
				BalanceAfterCents: wallet.BalanceCents,
				ReferenceType:     &refType,
				ReferenceID:       &order.ID,
				Description:       func() *string { s := fmt.Sprintf("Escrow release for order %s", order.ID); return &s }(),
				Status:            entity.WalletTxCompleted,
			}

			if err := walletTxnRepoTx.Create(ctx, txn); err != nil {
				return nil, fmt.Errorf("failed to create wallet transaction: %w", err)
			}

			logger.Info().
				Str("order_id", order.ID).
				Str("escrow_id", escrow.ID).
				Int64("released_amount", merchantAmount).
				Msg("✅ Escrow successfully released and wallet credited")
		} else {
			logger.Warn().Str("escrow_status", string(escrow.Status)).Msg("Escrow already processed, skipping wallet credit (Idempotence)")
		}
	} else if err != nil && err != sql.ErrNoRows {
		logger.Warn().Err(err).Msg("Error checking escrow, proceeding with order update but wallet might not be credited")
	}

	// 4. Mettre à jour la commande
	if err = orderRepoTx.UpdateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	// 5. Commit de la transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 6. Notifications (hors transaction)
	customerPhone := "" // TODO: récupérer depuis customer repo si nécessaire
	if err = uc.notifService.NotifyClientOrderDelivered(ctx, order, customerPhone, req.AmountReceived); err != nil {
		logger.Warn().Err(err).Msg("failed to send client notification")
	}

	logger.Info().Str("order_id", order.ID).Str("status", order.Status).Msg("Order delivered successfully")
	return order, nil
}
