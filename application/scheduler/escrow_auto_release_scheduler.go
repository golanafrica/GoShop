package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// ESCROW AUTO-RELEASE SCHEDULER
// ============================================================
//
// Anti race multi-instance :
//  1. ClaimRelease atomique (funds_held → released)
//  2. FindByReferenceIDAdmin avant crédit
//  3. Unique partial index uq_wallet_txn_ref_completed
//  4. CreditWithDebtSweep : dette d'abord, puis solde disponible

type EscrowAutoReleaseScheduler struct {
	deliveryProofRepo   repository.DeliveryProofRepository
	escrowRepo          repository.EscrowAccountRepository
	orderRepo           repository.OrderRepository
	tontineVoucherRepo  repository.TontineVoucherRepository
	tontineGroupRepo    repository.TontineGroupRepository
	shopRepo            repository.ShopRepository
	walletRepo          repository.MerchantWalletRepository
	walletTxnRepo       repository.WalletTransactionRepository
	platformRevenueRepo repository.PlatformRevenueRepository
	creditWalletUC      *walletusecase.CreditWalletUsecase
	batchSize           int
	maxRetries          int
	logger              zerolog.Logger
}

func NewEscrowAutoReleaseScheduler(
	deliveryProofRepo repository.DeliveryProofRepository,
	escrowRepo repository.EscrowAccountRepository,
	orderRepo repository.OrderRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	tontineGroupRepo repository.TontineGroupRepository,
	shopRepo repository.ShopRepository,
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
	platformRevenueRepo repository.PlatformRevenueRepository,
	creditWalletUC *walletusecase.CreditWalletUsecase,
	logger zerolog.Logger,
) *EscrowAutoReleaseScheduler {
	return &EscrowAutoReleaseScheduler{
		deliveryProofRepo:   deliveryProofRepo,
		escrowRepo:          escrowRepo,
		orderRepo:           orderRepo,
		tontineVoucherRepo:  tontineVoucherRepo,
		tontineGroupRepo:    tontineGroupRepo,
		shopRepo:            shopRepo,
		walletRepo:          walletRepo,
		walletTxnRepo:       walletTxnRepo,
		platformRevenueRepo: platformRevenueRepo,
		creditWalletUC:      creditWalletUC,
		batchSize:           100,
		maxRetries:          3,
		logger:              logger.With().Str("component", "escrow_auto_release_scheduler").Logger(),
	}
}

func (s *EscrowAutoReleaseScheduler) RunAutoRelease(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Msg("Starting escrow auto-release")

	proofs, err := s.deliveryProofRepo.FindAutoReleaseEligible(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch eligible proofs")
		return fmt.Errorf("failed to fetch eligible proofs: %w", err)
	}

	if len(proofs) == 0 {
		s.logger.Info().Msg("No proofs eligible for auto-release")
		return nil
	}

	s.logger.Info().
		Int("proof_count", len(proofs)).
		Msg("Processing auto-release for proofs")

	var releasedCount, skippedCount, failedCount int
	var releasedCents int64

	for _, proof := range proofs {
		released, err := s.processProof(ctx, proof)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("proof_id", proof.ID).
				Msg("Failed to auto-release proof")
			failedCount++
			continue
		}
		if released == 0 {
			skippedCount++
			continue
		}
		releasedCount++
		releasedCents += released
	}

	duration := time.Since(startTime)
	s.logger.Info().
		Int("released_count", releasedCount).
		Int("skipped_count", skippedCount).
		Int("failed_count", failedCount).
		Int64("released_cents", releasedCents).
		Int("duration_ms", int(duration.Milliseconds())).
		Msg("Escrow auto-release completed")

	return nil
}

func (s *EscrowAutoReleaseScheduler) processProof(ctx context.Context, proof *entity.DeliveryProof) (int64, error) {
	itemLogger := s.logger.With().
		Str("proof_id", proof.ID).
		Str("escrow_status", string(proof.EscrowStatus)).
		Logger()

	if !proof.AutoReleaseEligible() {
		return 0, fmt.Errorf("proof not eligible for auto-release (status: %s)", proof.EscrowStatus)
	}

	var shopID string
	var err error

	if proof.OrderID != nil {
		order, err := s.orderRepo.FindByIDAdmin(ctx, *proof.OrderID)
		if err != nil {
			return 0, fmt.Errorf("failed to find order: %w", err)
		}
		shopID = order.ShopID
	} else if proof.TontineVoucherID != nil {
		voucher, err := s.tontineVoucherRepo.FindByIDAdmin(ctx, *proof.TontineVoucherID)
		if err != nil {
			return 0, fmt.Errorf("failed to find tontine voucher: %w", err)
		}
		shopID = voucher.ShopID
	} else {
		return 0, fmt.Errorf("proof has no valid reference (order or tontine voucher)")
	}

	itemLogger = itemLogger.With().Str("shop_id", shopID).Logger()

	var escrow *entity.EscrowAccount
	if proof.OrderID != nil {
		escrow, err = s.escrowRepo.FindByOrderID(ctx, *proof.OrderID)
	} else if proof.TontineVoucherID != nil {
		voucher, _ := s.tontineVoucherRepo.FindByIDAdmin(ctx, *proof.TontineVoucherID)
		if voucher != nil {
			group, gErr := s.findTontineGroupByVoucher(ctx, voucher)
			if gErr != nil {
				return 0, fmt.Errorf("failed to find tontine group: %w", gErr)
			}
			escrow, err = s.escrowRepo.FindByTontineGroupID(ctx, group.ID)
		}
	}

	if err != nil || escrow == nil {
		return 0, fmt.Errorf("failed to find escrow account: %w", err)
	}

	itemLogger = itemLogger.With().
		Str("escrow_id", escrow.ID).
		Int64("total_cents", escrow.TotalAmountCents).
		Int64("commission_cents", escrow.CommissionCents).
		Logger()

	merchantAmount := escrow.GetMerchantAmount()
	claimed, err := s.escrowRepo.ClaimRelease(ctx, escrow.ID, entity.EscrowAccountFundsHeld, merchantAmount)
	if err != nil {
		return 0, fmt.Errorf("claim release failed: %w", err)
	}
	if !claimed {
		itemLogger.Info().Msg("Escrow already claimed by another instance — skip")
		return 0, nil
	}

	itemLogger.Info().Msg("Escrow claimed successfully")

	if escrow.CommissionCents > 0 && s.platformRevenueRepo != nil {
		const refType = "escrow_auto_release"
		if err := s.platformRevenueRepo.CreditRevenue(ctx, escrow.CommissionCents, refType, proof.ID); err != nil {
			itemLogger.Error().Err(err).
				Int64("commission_cents", escrow.CommissionCents).
				Str("reference_type", refType).
				Str("reference_id", proof.ID).
				Msg("Failed to credit platform revenue")
		} else {
			itemLogger.Info().
				Int64("commission_cents", escrow.CommissionCents).
				Msg("Platform revenue credited (Split)")
		}
	} else if escrow.CommissionCents > 0 && s.platformRevenueRepo == nil {
		itemLogger.Warn().Msg("platformRevenueRepo is nil — commission not credited")
	}

	if proof.TontineVoucherID != nil {
		itemLogger.Info().
			Str("voucher_id", *proof.TontineVoucherID).
			Msg("Tontine proof: escrow claimed, wallet already credited at cycle — skip credit")

		if err := proof.ReleaseFunds(); err != nil {
			itemLogger.Warn().Err(err).Msg("proof.ReleaseFunds in-memory failed")
		}
		if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
			itemLogger.Error().Err(err).Msg("failed to update delivery proof after tontine claim")
		}
		return merchantAmount, nil
	}

	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		err := s.creditMerchantWallet(ctx, shopID, merchantAmount, proof.ID, escrow.ID)
		if err != nil {
			lastErr = err
			itemLogger.Warn().
				Err(err).
				Int("attempt", attempt).
				Msg("Wallet credit attempt failed")
			if attempt < s.maxRetries {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			continue
		}
		lastErr = nil
		break
	}

	if lastErr != nil {
		return 0, fmt.Errorf("failed to credit merchant wallet after %d attempts (escrow already claimed): %w", s.maxRetries, lastErr)
	}

	if err := proof.ReleaseFunds(); err != nil {
		itemLogger.Warn().Err(err).Msg("proof.ReleaseFunds in-memory failed (may already be released)")
	}
	if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
		itemLogger.Error().Err(err).Msg("failed to update delivery proof after successful release — reconcile manually")
	}

	itemLogger.Info().
		Int64("merchant_amount", merchantAmount).
		Msg("Escrow auto-released and merchant wallet credited (debt sweep if any)")

	return merchantAmount, nil
}

// creditMerchantWallet : CreditWithDebtSweep + ledger sale_credit (net) + debt_sweep.
func (s *EscrowAutoReleaseScheduler) creditMerchantWallet(
	ctx context.Context,
	shopID string,
	amountCents int64,
	proofID string,
	escrowID string,
) error {
	const refType = "escrow_auto_release"

	existing, err := s.walletTxnRepo.FindByReferenceIDAdmin(ctx, refType, proofID)
	if err == nil && existing != nil {
		s.logger.Info().
			Str("proof_id", proofID).
			Str("existing_txn_id", existing.ID).
			Msg("Wallet already credited for this proof — skip")
		return nil
	}

	shopUUID, err := uuid.Parse(shopID)
	if err != nil {
		return fmt.Errorf("invalid shop UUID: %w", err)
	}
	shop, err := s.shopRepo.FindByID(ctx, shopUUID)
	if err != nil {
		return fmt.Errorf("failed to find shop for wallet credit: %w", err)
	}
	shopCtx := tenant.WithTenant(ctx, shop)

	wallet, err := s.walletRepo.FindByShopID(shopCtx, shopID)
	if err != nil {
		if err.Error() == "merchant wallet not found" {
			wallet = entity.NewMerchantWallet(shopID)
			if err := s.walletRepo.Create(shopCtx, wallet); err != nil {
				return fmt.Errorf("failed to create merchant wallet: %w", err)
			}
		} else {
			return fmt.Errorf("failed to find merchant wallet: %w", err)
		}
	}

	netToBalance, swept, err := wallet.CreditWithDebtSweep(amountCents)
	if err != nil {
		return fmt.Errorf("failed to credit wallet with debt sweep: %w", err)
	}

	refTypeCopy := refType
	desc := fmt.Sprintf("Auto-release (Proof: %s, Escrow: %s) gross=%d net=%d swept=%d",
		proofID, escrowID, amountCents, netToBalance, swept)

	// 1) sale_credit net (skip si 0 — full sweep)
	if netToBalance > 0 {
		txn := &entity.WalletTransaction{
			ID:                uuid.New().String(),
			ShopID:            shopID,
			TransactionType:   entity.WalletTxSaleCredit,
			AmountCents:       netToBalance,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &refTypeCopy,
			ReferenceID:       &proofID,
			Description:       &desc,
			Status:            entity.WalletTxCompleted,
			CreatedAt:         time.Now().UTC(),
		}
		if err := s.walletTxnRepo.Create(shopCtx, txn); err != nil {
			if isUniqueViolation(err) {
				s.logger.Info().
					Str("proof_id", proofID).
					Msg("Unique constraint hit — wallet already credited concurrently")
				return nil
			}
			return fmt.Errorf("failed to create wallet transaction: %w", err)
		}
	} else {
		// Full debt sweep : marqueur d'idempotence via debt_sweep ref escrow_auto_release
		// On crée quand même une ligne sale_credit? Non (amount 0 interdit).
		// Filet : si concurrent retry, FindByReferenceIDAdmin ne voit rien —
		// on s'appuie sur ClaimRelease déjà consommé + unique debt_sweep optionnel.
		s.logger.Info().
			Str("proof_id", proofID).
			Int64("swept", swept).
			Msg("Full debt sweep — no sale_credit line (net=0)")
	}

	// 2) debt_sweep audit
	if swept > 0 {
		sweepRefType := "debt_sweep"
		sweepDesc := fmt.Sprintf("Debt sweep on auto-release proof=%s escrow=%s", proofID, escrowID)
		sweepAmt := -swept
		sweepTxn := &entity.WalletTransaction{
			ID:                uuid.New().String(),
			ShopID:            shopID,
			TransactionType:   entity.WalletTxDebtSweep,
			AmountCents:       sweepAmt,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &sweepRefType,
			ReferenceID:       &proofID,
			Description:       &sweepDesc,
			Status:            entity.WalletTxCompleted,
			CreatedAt:         time.Now().UTC(),
		}
		if err := s.walletTxnRepo.Create(shopCtx, sweepTxn); err != nil {
			if !isUniqueViolation(err) {
				s.logger.Warn().Err(err).Str("proof_id", proofID).Msg("Failed to create debt_sweep audit transaction")
			}
		}
	}

	// 3) Persister wallet (après ledger)
	if err := s.walletRepo.Update(shopCtx, wallet); err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	s.logger.Info().
		Str("proof_id", proofID).
		Str("shop_id", shopID).
		Int64("gross_cents", amountCents).
		Int64("net_to_balance", netToBalance).
		Int64("swept_cents", swept).
		Int64("balance_cents", wallet.BalanceCents).
		Int64("debt_cents", wallet.DebtCents).
		Msg("Merchant wallet credited with debt sweep")

	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "uq_wallet_txn_ref_completed") ||
		strings.Contains(msg, "23505")
}

func (s *EscrowAutoReleaseScheduler) findTontineGroupByVoucher(
	ctx context.Context,
	voucher *entity.TontineVoucher,
) (*entity.TontineGroup, error) {
	if voucher.GroupID == "" {
		return nil, fmt.Errorf("voucher has no group_id")
	}

	group, err := s.tontineGroupRepo.FindByID(ctx, voucher.GroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to find tontine group %s: %w", voucher.GroupID, err)
	}

	return group, nil
}
