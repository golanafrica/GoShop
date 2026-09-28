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
// Clawback post-release (customer_wins + escrow released) :
//  - ApplyClawbackToDebt : balance puis debt_cents (pas de freeze)
//  - Ledger clawback + debt_add si reste
// merchant_wins :
//  - CreditWithDebtSweep (intercepte rentrées si dette)
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

	// Litige classique : escrow disputed.
	// Post-release : escrow released → customer_wins + clawback/debt uniquement.
	escrowAlreadyReleased := escrow.Status == entity.EscrowAccountFullyReleased
	if escrow.Status != entity.EscrowAccountDisputed && !escrowAlreadyReleased {
		return nil, fmt.Errorf("%w: status is %s", ErrEscrowNotDisputed, escrow.Status)
	}
	if escrowAlreadyReleased && req.Resolution != "customer_wins" {
		return nil, fmt.Errorf(
			"escrow already released: only customer_wins (clawback) is allowed, got %s",
			req.Resolution,
		)
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

		claimed, claimErr := escrowRepoTx.ClaimRelease(ctx, escrow.ID, entity.EscrowAccountDisputed, merchantAmount)
		if claimErr != nil {
			return nil, fmt.Errorf("claim release failed: %w", claimErr)
		}
		if !claimed {
			return nil, ErrEscrowClaimConflict
		}

		shopIDStr := dispute.ShopID.String()
		refType := "dispute_resolution"

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

			net, swept, cErr := wallet.CreditWithDebtSweep(merchantAmount)
			if cErr != nil {
				return nil, fmt.Errorf("failed to credit wallet with debt sweep: %w", cErr)
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

			if swept > 0 {
				sweepRef := "dispute_resolution_debt_sweep"
				sweepDesc := fmt.Sprintf(
					"Debt sweep on merchant_wins credit (Order: %s, swept=%d net=%d)",
					orderIDStr, swept, net,
				)
				sweepAmt := -swept
				sweepTxn := &entity.WalletTransaction{
					ID:                uuid.New().String(),
					ShopID:            shopIDStr,
					TransactionType:   entity.WalletTxDebtSweep,
					AmountCents:       sweepAmt,
					BalanceAfterCents: wallet.BalanceCents,
					ReferenceType:     &sweepRef,
					ReferenceID:       &disputeIDStr,
					Description:       &sweepDesc,
					Status:            entity.WalletTxCompleted,
					CreatedAt:         time.Now().UTC(),
				}
				if err := walletTxnRepoTx.CreateAdmin(ctx, sweepTxn); err != nil {
					msg := strings.ToLower(err.Error())
					if !(strings.Contains(msg, "duplicate key") ||
						strings.Contains(msg, "unique constraint") ||
						strings.Contains(msg, "23505")) {
						return nil, fmt.Errorf("failed to create debt_sweep transaction: %w", err)
					}
				}
				logger.Info().
					Int64("swept", swept).
					Int64("net_to_balance", net).
					Int64("debt_after", wallet.DebtCents).
					Msg("✅ merchant_wins credit applied with debt sweep")
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

		// Client = net escrow (brut - frais Yenga). Frais PSP non remboursés.
		refundAmount := escrow.TotalAmountCents
		if refundAmount <= 0 {
			refundAmount = successPayment.AmountCents - successPayment.ProviderFeesCents
			if refundAmount <= 0 {
				refundAmount = successPayment.AmountCents
			}
		}
		refundedAmount = refundAmount

		customerPhone, operator, sourceHint := resolveRefundDestination(successPayment)

		logger.Info().
			Int64("payment_amount_cents", successPayment.AmountCents).
			Int64("provider_fees_cents", successPayment.ProviderFeesCents).
			Int64("escrow_total_cents", escrow.TotalAmountCents).
			Int64("refund_amount_cents", refundAmount).
			Str("customer_phone", customerPhone).
			Str("operator", operator).
			Str("source_hint", sourceHint).
			Msg("customer_wins refund = escrow net, same channel as pay-in when known")

		if customerPhone == "" {
			return nil, fmt.Errorf("cannot refund: missing customer phone on payment (pay-in channel unknown)")
		}

		if err := provider.Refund(ctx, *successPayment.ProviderRef, refundAmount, customerPhone, operator); err != nil {
			logger.Error().Err(err).Msg("Failed to process refund with provider")
			return nil, fmt.Errorf("failed to process refund with provider: %w", err)
		}

		shopIDStr := dispute.ShopID.String()
		merchantAmount := escrow.GetMerchantAmount()

		switch escrow.Status {
		case entity.EscrowAccountDisputed:
			// Fonds encore en séquestre → refund escrow classique
			if err := escrow.ResolveDispute(false); err != nil {
				return nil, fmt.Errorf("failed to resolve escrow for customer: %w", err)
			}
			if err := escrowRepoTx.Update(ctx, escrow); err != nil {
				return nil, fmt.Errorf("failed to update escrow to refunded: %w", err)
			}

		case entity.EscrowAccountFullyReleased:
			// Post-release : clawback balance + debt_cents si insuffisant
			now := time.Now().UTC()
			escrow.Status = entity.EscrowAccountRefunded
			escrow.ReleasedAmountCents = 0
			escrow.FundsReleasedAt = &now
			escrow.UpdatedAt = now
			if err := escrowRepoTx.Update(ctx, escrow); err != nil {
				return nil, fmt.Errorf("failed to mark escrow refunded after release: %w", err)
			}

			clawRefType := "dispute_clawback"
			debtRefType := "dispute_debt_add"

			existingClaw, _ := walletTxnRepoTx.FindByReferenceIDAdmin(ctx, clawRefType, disputeIDStr)
			existingDebt, _ := walletTxnRepoTx.FindByReferenceIDAdmin(ctx, debtRefType, disputeIDStr)
			alreadyDone := (existingClaw != nil && existingClaw.Status == entity.WalletTxCompleted) ||
				(existingDebt != nil && existingDebt.Status == entity.WalletTxCompleted)

			if alreadyDone {
				logger.Info().
					Str("dispute_id", disputeIDStr).
					Msg("⏭️ Clawback/debt already applied for this dispute — skip")
			} else if merchantAmount > 0 {
				wallet, wErr := walletRepoTx.FindByShopIDForUpdateAdmin(ctx, shopIDStr)
				if wErr != nil {
					return nil, fmt.Errorf("failed to find merchant wallet for clawback: %w", wErr)
				}

				fromBalance, toDebt, clawErr := wallet.ApplyClawbackToDebt(merchantAmount)
				if clawErr != nil {
					return nil, fmt.Errorf("failed to apply clawback to debt: %w", clawErr)
				}
				if err := walletRepoTx.UpdateAdmin(ctx, wallet); err != nil {
					return nil, fmt.Errorf("failed to update wallet after clawback: %w", err)
				}

				if fromBalance > 0 {
					desc := fmt.Sprintf(
						"Clawback litige customer_wins post-release (Order: %s, from_balance=%d)",
						orderIDStr, fromBalance,
					)
					clawAmount := -fromBalance
					txn := &entity.WalletTransaction{
						ID:                uuid.New().String(),
						ShopID:            shopIDStr,
						TransactionType:   entity.WalletTxClawback,
						AmountCents:       clawAmount,
						BalanceAfterCents: wallet.BalanceCents,
						ReferenceType:     &clawRefType,
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
								Msg("⏭️ Clawback concurrent — treat as success")
						} else {
							return nil, fmt.Errorf("failed to create clawback wallet transaction: %w", err)
						}
					}
				} else {
					logger.Info().
						Str("dispute_id", disputeIDStr).
						Msg("No balance to claw — residual goes to debt_cents only")
				}

				if toDebt > 0 {
					debtDesc := fmt.Sprintf(
						"Dette marchand post clawback customer_wins (Order: %s, debt_add=%d)",
						orderIDStr, toDebt,
					)
					debtAmt := -toDebt
					debtTxn := &entity.WalletTransaction{
						ID:                uuid.New().String(),
						ShopID:            shopIDStr,
						TransactionType:   entity.WalletTxDebtAdd,
						AmountCents:       debtAmt,
						BalanceAfterCents: wallet.BalanceCents,
						ReferenceType:     &debtRefType,
						ReferenceID:       &disputeIDStr,
						Description:       &debtDesc,
						Status:            entity.WalletTxCompleted,
						CreatedAt:         time.Now().UTC(),
					}
					if err := walletTxnRepoTx.CreateAdmin(ctx, debtTxn); err != nil {
						msg := strings.ToLower(err.Error())
						if !(strings.Contains(msg, "duplicate key") ||
							strings.Contains(msg, "unique constraint") ||
							strings.Contains(msg, "23505")) {
							return nil, fmt.Errorf("failed to create debt_add transaction: %w", err)
						}
					}
				}

				logger.Info().
					Str("shop_id", shopIDStr).
					Int64("clawback_total", merchantAmount).
					Int64("from_balance", fromBalance).
					Int64("to_debt", toDebt).
					Int64("balance_after", wallet.BalanceCents).
					Int64("debt_cents", wallet.DebtCents).
					Msg("✅ ClawbackToDebt post-release applied (no freeze)")
			}
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

// resolveRefundDestination priorise le vrai MSISDN Yenga (metadata) sur le seed E2E.
func resolveRefundDestination(p *entity.Payment) (phone string, operator string, sourceHint string) {
	if p.Metadata != nil {
		phone = firstMetaString(p.Metadata, "customer_number", "customerNumber", "customer_phone", "customerPhone")
		if isSeedPhoneLocal(phone) {
			phone = ""
		}

		sourceHint = firstMetaString(p.Metadata, "payment_source", "paymentSource", "payment_source_raw")
		operator = mapPayInSourceToCashoutOperator(sourceHint)
		if operator == "" {
			operator = mapPayInSourceToCashoutOperator(
				firstMetaString(p.Metadata, "operator", "cashout_method", "cashoutMethod"),
			)
		}
	}

	if phone == "" && p.CustomerPhone != nil {
		cand := strings.TrimSpace(*p.CustomerPhone)
		if cand != "" && !isSeedPhoneLocal(cand) {
			phone = cand
		}
	}

	phone = normalizeMSISDN(phone)
	return phone, operator, sourceHint
}

func isSeedPhoneLocal(phone string) bool {
	d := digitsOnlyLocal(phone)
	if d == "" {
		return true
	}
	seeds := []string{
		"70000000",
		"70123456",
		"70707070",
		"78787878",
		"76658060",
	}
	for _, s := range seeds {
		if strings.HasSuffix(d, s) {
			return true
		}
	}
	return false
}

func digitsOnlyLocal(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstMetaString(meta map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := meta[k]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func mapPayInSourceToCashoutOperator(src string) string {
	s := strings.ToUpper(strings.TrimSpace(src))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "_")

	switch {
	case strings.Contains(s, "ORANGE"):
		return "ORANGE"
	case strings.Contains(s, "MOOV"):
		return "MOOV"
	case strings.Contains(s, "TELECEL"):
		return "TELECEL"
	case strings.Contains(s, "CORIS"):
		return "CORIS"
	case strings.Contains(s, "SANK"):
		return "SANK"
	default:
		switch s {
		case "ORANGE_MONEY", "MOOV_MONEY", "TELECEL", "CORIS_MONEY", "SANK_MONEY":
			return s
		}
		return s
	}
}

func normalizeMSISDN(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "00") {
		phone = "+" + phone[2:]
	}
	if len(phone) == 0 {
		return ""
	}
	if phone[0] != '+' {
		d := digitsOnlyLocal(phone)
		switch {
		case len(d) == 8:
			phone = "+226" + d
		case strings.HasPrefix(d, "226") && len(d) >= 11:
			phone = "+" + d
		default:
			if d != "" {
				phone = "+" + d
			}
		}
	}
	return phone
}
