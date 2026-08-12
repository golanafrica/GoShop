package disputeusecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// PaymentRegistry évite une dépendance circulaire usecase → registry concret.
type PaymentRegistry interface {
	Get(providerCode entity.PaymentProvider) (payment.Provider, error)
}

// Erreurs métier concurrentes → mappées en HTTP 409 par le handler.
var (
	ErrDisputeAlreadyResolved = errors.New("dispute is already resolved or cancelled")
	ErrEscrowClaimConflict    = errors.New("escrow already claimed/released by concurrent resolve")
	ErrEscrowNotDisputed      = errors.New("cannot resolve dispute: escrow is not in disputed status")
)

// ============================================================
// RESOLVE DISPUTE USECASE
// Anti double-crédit :
//  - ClaimRelease atomique disputed → released (merchant_wins)
//  - Pré-check FindByReferenceIDAdmin(dispute_resolution, disputeID)
//  - Unique index uq_wallet_txn_ref_completed en filet
// ============================================================

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
	Resolution string // "merchant_wins" | "customer_wins"
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
		return nil, ErrDisputeAlreadyResolved
	}

	orderIDStr := dispute.OrderID.String()
	escrow, err := uc.escrowRepo.FindByOrderID(ctx, orderIDStr)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for dispute order: %w", err)
	}

	if escrow.Status != entity.EscrowAccountDisputed {
		return nil, fmt.Errorf("%w: status is %s", ErrEscrowNotDisputed, escrow.Status)
	}

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	disputeRepoTx := uc.disputeRepo.WithTX(tx)
	escrowRepoTx := uc.escrowRepo.WithTX(tx)
	walletRepoTx := uc.walletRepo.WithTX(tx)
	walletTxnRepoTx := uc.walletTxnRepo.WithTX(tx)

	var refundedAmount int64
	disputeIDStr := dispute.ID.String()

	switch req.Resolution {
	case "merchant_wins":
		dispute.Status = entity.DisputeStatusResolvedMerchant

		merchantAmount := escrow.GetMerchantAmount()

		// Claim atomique disputed → released (anti race multi-instance / double resolve)
		claimed, claimErr := escrowRepoTx.ClaimRelease(ctx, escrow.ID, entity.EscrowAccountDisputed, merchantAmount)
		if claimErr != nil {
			return nil, fmt.Errorf("claim release failed: %w", claimErr)
		}
		if !claimed {
			return nil, ErrEscrowClaimConflict
		}

		shopIDStr := dispute.ShopID.String()
		refType := "dispute_resolution"

		// Idempotence pré-check (même TX)
		existing, findErr := walletTxnRepoTx.FindByReferenceIDAdmin(ctx, refType, disputeIDStr)
		if findErr == nil && existing != nil && existing.Status == entity.WalletTxCompleted {
			logger.Info().
				Str("dispute_id", disputeIDStr).
				Str("existing_txn", existing.ID).
				Msg("⏭️ Wallet already credited for this dispute — skip credit")
		} else {
			wallet, wErr := walletRepoTx.FindByShopIDForUpdateAdmin(ctx, shopIDStr)
			if wErr != nil {
				if wErr.Error() == "merchant wallet not found" {
					wallet = entity.NewMerchantWallet(shopIDStr)
					if err := walletRepoTx.CreateAdmin(ctx, wallet); err != nil {
						return nil, fmt.Errorf("failed to create missing merchant wallet: %w", err)
					}
				} else {
					return nil, fmt.Errorf("failed to find merchant wallet: %w", wErr)
				}
			}

			if err := wallet.Credit(merchantAmount); err != nil {
				return nil, fmt.Errorf("failed to credit wallet: %w", err)
			}
			if err := walletRepoTx.UpdateAdmin(ctx, wallet); err != nil {
				return nil, fmt.Errorf("failed to update wallet: %w", err)
			}

			desc := fmt.Sprintf("Litige résolu en faveur du marchand (Order: %s)", orderIDStr)
			txn := &entity.WalletTransaction{
				ID:                uuid.New().String(),
				ShopID:            shopIDStr,
				TransactionType:   entity.WalletTxSaleCredit,
				AmountCents:       merchantAmount,
				BalanceAfterCents: wallet.BalanceCents,
				ReferenceType:     &refType,
				ReferenceID:       &disputeIDStr,
				Description:       &desc,
				Status:            entity.WalletTxCompleted,
				CreatedAt:         time.Now().UTC(),
			}
			if err := walletTxnRepoTx.CreateAdmin(ctx, txn); err != nil {
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "duplicate key") ||
					strings.Contains(msg, "unique constraint") ||
					strings.Contains(msg, "uq_wallet_txn_ref_completed") ||
					strings.Contains(msg, "23505") {
					logger.Info().
						Str("dispute_id", disputeIDStr).
						Msg("⏭️ Unique constraint — concurrent credit, treat as success")
				} else {
					return nil, fmt.Errorf("failed to create wallet transaction: %w", err)
				}
			}
		}

	case "customer_wins":
		dispute.Status = entity.DisputeStatusResolvedCustomer

		payments, err := uc.paymentRepo.FindByOrderIDUnscoped(ctx, dispute.OrderID)
		if err != nil {
			return nil, fmt.Errorf("failed to find payments for order: %w", err)
		}

		var successPayment *entity.Payment
		for _, p := range payments {
			if p.Status == entity.PaymentStatusSuccess && p.ProviderRef != nil && *p.ProviderRef != "" {
				successPayment = p
				break
			}
		}
		if successPayment == nil {
			return nil, fmt.Errorf("no successful payment with provider reference found for this order")
		}

		provider, err := uc.paymentRegistry.Get(successPayment.Provider)
		if err != nil {
			return nil, fmt.Errorf("payment provider not found: %w", err)
		}

		refundAmount := successPayment.AmountCents
		if refundAmount <= 0 {
			refundAmount = escrow.TotalAmountCents
		}
		refundedAmount = refundAmount

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

		// Refund provider AVANT écritures finales
		if err := provider.Refund(ctx, *successPayment.ProviderRef, refundAmount, customerPhone, operator); err != nil {
			logger.Error().Err(err).Msg("Failed to process refund with provider")
			return nil, fmt.Errorf("failed to process refund with provider: %w", err)
		}

		// disputed → refunded (domain)
		if err := escrow.ResolveDispute(false); err != nil {
			return nil, fmt.Errorf("failed to resolve escrow for customer: %w", err)
		}
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
			Msg("✅ Funds refunded to customer via provider")

	default:
		return nil, fmt.Errorf("invalid resolution: must be 'merchant_wins' or 'customer_wins'")
	}

	dispute.ResolutionNotes = &req.Notes
	dispute.UpdatedAt = time.Now().UTC()
	if err := disputeRepoTx.Update(ctx, dispute); err != nil {
		return nil, fmt.Errorf("failed to update dispute: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Notifications hors TX
	if uc.notificationSvc != nil {
		shop := &entity.Shop{ID: dispute.ShopID}
		tenantCtx := tenant.WithTenant(context.Background(), shop)

		order, findErr := uc.orderRepo.FindByID(tenantCtx, dispute.OrderID.String())
		if findErr == nil && order != nil {
			if notifyErr := uc.notificationSvc.NotifyClientDisputeResolved(tenantCtx, order.CustomerID, orderIDStr, req.Resolution, refundedAmount); notifyErr != nil {
				logger.Warn().Err(notifyErr).Msg("Failed to send client dispute notification")
			}
			if notifyErr := uc.notificationSvc.NotifyMerchantDisputeResolved(tenantCtx, dispute.ShopID.String(), orderIDStr, req.Resolution); notifyErr != nil {
				logger.Warn().Err(notifyErr).Msg("Failed to send merchant dispute notification")
			}
			logger.Info().Str("order_id", orderIDStr).Msg("✅ Dispute resolution notifications dispatched")
		} else if findErr != nil {
			logger.Warn().Err(findErr).Msg("Failed to fetch order for dispute notifications")
		}
	}

	logger.Info().
		Str("dispute_id", dispute.ID.String()).
		Str("resolution", req.Resolution).
		Msg("Dispute resolved successfully")

	return dispute, nil
}
