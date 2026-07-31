package disputeusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// PaymentRegistry interface pour éviter les dépendances circulaires
type PaymentRegistry interface {
	Get(providerCode entity.PaymentProvider) (payment.Provider, error)
}

type ResolveDisputeUsecase struct {
	disputeRepo     repository.DisputeRepository
	escrowRepo      repository.EscrowAccountRepository
	walletRepo      repository.MerchantWalletRepository
	walletTxnRepo   repository.WalletTransactionRepository
	txManager       repository.TxManager
	paymentRepo     repository.PaymentRepository
	paymentRegistry PaymentRegistry
	orderRepo       repository.OrderRepository
	notificationSvc service.NotificationService
}

func NewResolveDisputeUsecase(
	disputeRepo repository.DisputeRepository,
	escrowRepo repository.EscrowAccountRepository,
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
	paymentRepo repository.PaymentRepository,
	paymentRegistry PaymentRegistry,
	orderRepo repository.OrderRepository,
	notificationSvc service.NotificationService,
) *ResolveDisputeUsecase {
	return &ResolveDisputeUsecase{
		disputeRepo:     disputeRepo,
		escrowRepo:      escrowRepo,
		walletRepo:      walletRepo,
		walletTxnRepo:   walletTxnRepo,
		txManager:       txManager,
		paymentRepo:     paymentRepo,
		paymentRegistry: paymentRegistry,
		orderRepo:       orderRepo,
		notificationSvc: notificationSvc,
	}
}

type ResolveDisputeRequest struct {
	DisputeID  uuid.UUID
	Resolution string // "merchant_wins" ou "customer_wins"
	Notes      string
	ResolverID uuid.UUID
}

func (uc *ResolveDisputeUsecase) Execute(ctx context.Context, req *ResolveDisputeRequest) (*entity.Dispute, error) {
	logger := zerolog.Ctx(ctx)

	dispute, err := uc.disputeRepo.FindByID(ctx, req.DisputeID)
	if err != nil {
		return nil, fmt.Errorf("dispute not found: %w", err)
	}

	if dispute.Status != entity.DisputeStatusPending && dispute.Status != entity.DisputeStatusUnderReview {
		return nil, fmt.Errorf("dispute is already resolved or cancelled")
	}

	orderIDStr := dispute.OrderID.String()
	escrow, err := uc.escrowRepo.FindByOrderID(ctx, orderIDStr)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for dispute order: %w", err)
	}

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	escrowRepoTx := uc.escrowRepo.WithTX(tx)
	walletRepoTx := uc.walletRepo.WithTX(tx)
	walletTxnRepoTx := uc.walletTxnRepo.WithTX(tx)

	// 🆕 Variable pour stocker le montant remboursé (utilisé plus bas pour les notifications)
	var refundedAmount int64 = 0

	switch req.Resolution {
	case "merchant_wins":
		dispute.Status = entity.DisputeStatusResolvedMerchant

		if err := escrow.ReleaseFunds(); err != nil {
			return nil, fmt.Errorf("failed to release escrow: %w", err)
		}
		if err := escrowRepoTx.Update(ctx, escrow); err != nil {
			return nil, fmt.Errorf("failed to update escrow: %w", err)
		}

		merchantAmount := escrow.GetMerchantAmount()
		shopIDStr := dispute.ShopID.String()

		wallet, err := walletRepoTx.FindByShopIDForUpdateAdmin(ctx, shopIDStr)
		if err != nil {
			if err.Error() == "merchant wallet not found" {
				wallet = entity.NewMerchantWallet(shopIDStr)
				if err := walletRepoTx.CreateAdmin(ctx, wallet); err != nil {
					return nil, fmt.Errorf("failed to create missing merchant wallet: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to find merchant wallet: %w", err)
			}
		}

		if err := wallet.Credit(merchantAmount); err != nil {
			return nil, fmt.Errorf("failed to credit wallet: %w", err)
		}
		if err := walletRepoTx.UpdateAdmin(ctx, wallet); err != nil {
			return nil, fmt.Errorf("failed to update wallet: %w", err)
		}

		txnID := uuid.New().String()
		refType := "dispute_resolution"
		desc := fmt.Sprintf("Litige résolu en faveur du marchand (Order: %s)", orderIDStr)

		txn := &entity.WalletTransaction{
			ID:                txnID,
			ShopID:            shopIDStr,
			TransactionType:   entity.WalletTxSaleCredit,
			AmountCents:       merchantAmount,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &refType,
			ReferenceID:       func() *string { s := dispute.ID.String(); return &s }(),
			Description:       &desc,
			Status:            entity.WalletTxCompleted,
			CreatedAt:         time.Now().UTC(),
		}

		if err := walletTxnRepoTx.CreateAdmin(ctx, txn); err != nil {
			return nil, fmt.Errorf("failed to create wallet transaction: %w", err)
		}

	case "customer_wins":
		dispute.Status = entity.DisputeStatusResolvedCustomer

		payments, err := uc.paymentRepo.FindByOrderIDUnscoped(ctx, dispute.OrderID)
		if err != nil {
			return nil, fmt.Errorf("failed to find payments for order: %w", err)
		}

		var successPayment *entity.Payment
		for _, p := range payments {
			if p.Status == entity.PaymentStatusSuccess && p.ProviderRef != nil {
				successPayment = p
				break
			}
		}

		if successPayment == nil || successPayment.ProviderRef == nil {
			return nil, fmt.Errorf("no successful payment with provider reference found for this order")
		}

		provider, err := uc.paymentRegistry.Get(successPayment.Provider)
		if err != nil {
			return nil, fmt.Errorf("payment provider not found: %w", err)
		}

		refundAmount := escrow.TotalAmountCents
		refundedAmount = refundAmount // 🆕 On capture le montant pour la notification

		customerPhone := ""
		if successPayment.CustomerPhone != nil {
			customerPhone = *successPayment.CustomerPhone
		}

		operator := ""
		if successPayment.Metadata != nil {
			if op, ok := successPayment.Metadata["operator"].(string); ok {
				operator = op
			}
		}

		if err := provider.Refund(ctx, *successPayment.ProviderRef, refundAmount, customerPhone, operator); err != nil {
			logger.Error().Err(err).Msg("Failed to process refund with provider")
			return nil, fmt.Errorf("failed to process refund with provider: %w", err)
		}

		escrow.Status = "refunded"
		if err := escrowRepoTx.Update(ctx, escrow); err != nil {
			return nil, fmt.Errorf("failed to update escrow to refunded: %w", err)
		}

		logger.Info().
			Str("order_id", orderIDStr).
			Str("dispute_id", dispute.ID.String()).
			Str("provider_ref", *successPayment.ProviderRef).
			Str("customer_phone", customerPhone).
			Str("operator", operator).
			Int64("refunded_amount", refundAmount).
			Msg("✅ Funds successfully refunded to customer via YengaPay Cash-Out")

	default:
		return nil, fmt.Errorf("invalid resolution: must be 'merchant_wins' or 'customer_wins'")
	}

	dispute.ResolutionNotes = &req.Notes
	dispute.UpdatedAt = time.Now().UTC()

	if err := uc.disputeRepo.Update(ctx, dispute); err != nil {
		return nil, fmt.Errorf("failed to update dispute: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// ============================================================
	// 🆕 NOTIFICATIONS TEMPS RÉEL (Hors transaction, après succès)
	// ============================================================
	shop := &entity.Shop{ID: dispute.ShopID}
	tenantCtx := tenant.WithTenant(context.Background(), shop)

	order, err := uc.orderRepo.FindByID(tenantCtx, dispute.OrderID.String())
	if err == nil && order != nil {
		// 🆕 Notifier le client avec le montant remboursé
		if notifyErr := uc.notificationSvc.NotifyClientDisputeResolved(tenantCtx, order.CustomerID, orderIDStr, req.Resolution, refundedAmount); notifyErr != nil {
			logger.Warn().Err(notifyErr).Msg("Failed to send client dispute notification")
		}
		// Notifier le marchand
		if notifyErr := uc.notificationSvc.NotifyMerchantDisputeResolved(tenantCtx, dispute.ShopID.String(), orderIDStr, req.Resolution); notifyErr != nil {
			logger.Warn().Err(notifyErr).Msg("Failed to send merchant dispute notification")
		}
		logger.Info().Str("order_id", orderIDStr).Msg("✅ Dispute resolution notifications dispatched")
	} else {
		logger.Warn().Err(err).Msg("Failed to fetch order for dispute notifications")
	}

	logger.Info().
		Str("dispute_id", dispute.ID.String()).
		Str("resolution", req.Resolution).
		Msg("Dispute resolved successfully")

	return dispute, nil
}
