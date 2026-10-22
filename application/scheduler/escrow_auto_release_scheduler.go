package scheduler

import (
	"context"
	"database/sql"
	"errors"
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
// Anti race multi-instance & Atomicité (PR 2 Audit) :
//  1. Pour les Tontines : ClaimRelease immédiat (le crédit wallet est géré ailleurs et est idempotent).
//  2. Pour les Orders : ClaimRelease + Commission + Ledger + Wallet.Update dans UNE SEULE transaction SQL.
//     → Plus de divergence comptable en cas de panne. Si le crédit échoue, le claim est rollbacké.
//  3. FindByReferenceIDAdmin avant crédit pour l'idempotence.
//  4. Si claim déjà fait ailleurs mais ledger absent → reprise du crédit uniquement (orphan recovery).
// ============================================================

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
	txManager           repository.TxManager
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
	txManager repository.TxManager,
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
		txManager:           txManager,
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

	// 🛡️ CAS TONTINE : Le crédit wallet est géré ailleurs (idempotent). On claim ici directement.
	if proof.TontineVoucherID != nil {
		claimed, err := s.escrowRepo.ClaimRelease(ctx, escrow.ID, entity.EscrowAccountFundsHeld, merchantAmount)
		if err != nil {
			return 0, fmt.Errorf("claim release failed: %w", err)
		}
		if !claimed {
			itemLogger.Info().Msg("Escrow already claimed by another instance — skip")
			return 0, nil
		}

		itemLogger.Info().Msg("Tontine proof: escrow claimed, wallet already credited at cycle — skip credit")
		if err := proof.ReleaseFunds(); err != nil {
			itemLogger.Warn().Err(err).Msg("proof.ReleaseFunds in-memory failed")
		}
		if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
			itemLogger.Error().Err(err).Msg("failed to update delivery proof after tontine claim")
		}
		return merchantAmount, nil
	}

	// 🛡️ CAS ORDER : ClaimRelease + crédit dans la même TX (PR 2 Audit).
	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		err := s.creditMerchantWallet(ctx, shopID, merchantAmount, escrow.CommissionCents, proof.ID, escrow.ID, escrow.Status)
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
		return 0, fmt.Errorf("failed to credit merchant wallet after %d attempts: %w", s.maxRetries, lastErr)
	}

	if err := proof.ReleaseFunds(); err != nil {
		itemLogger.Warn().Err(err).Msg("proof.ReleaseFunds in-memory failed (may already be released)")
	}
	if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
		itemLogger.Error().Err(err).Msg("failed to update delivery proof after successful release — reconcile manually")
	}

	itemLogger.Info().
		Int64("merchant_amount", merchantAmount).
		Msg("Escrow auto-released and merchant wallet credited atomically (debt sweep if any)")

	return merchantAmount, nil
}

// creditMerchantWallet exécute le ClaimRelease, le crédit commission et le crédit wallet dans UNE SEULE transaction SQL.
// Si le claim a déjà été pris ailleurs mais le ledger est absent, complète uniquement le crédit (orphan recovery).
func (s *EscrowAutoReleaseScheduler) creditMerchantWallet(
	ctx context.Context,
	shopID string,
	merchantAmountCents int64,
	commissionCents int64,
	proofID string,
	escrowID string,
	currentEscrowStatus entity.EscrowAccountStatus,
) error {
	const refType = "escrow_auto_release"

	if s.txManager == nil {
		return fmt.Errorf("txManager is required for atomic escrow wallet credit (proof=%s)", proofID)
	}

	// 1. Idempotence check AVANT la transaction (optimisation)
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

	tx, txErr := s.txManager.BeginTx(ctx)
	if txErr != nil {
		return fmt.Errorf("failed to begin tx: %w", txErr)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			s.logger.Error().Err(rbErr).Str("proof_id", proofID).Msg("unexpected error during transaction rollback")
		}
	}()

	// 🛡️ 2. ClaimRelease DANS la transaction (Atomicité garantie - PR 2 Audit)
	escrowRepoTx := s.escrowRepo.WithTX(tx)
	claimed, err := escrowRepoTx.ClaimRelease(ctx, escrowID, currentEscrowStatus, merchantAmountCents)
	if err != nil {
		return fmt.Errorf("claim release failed in tx: %w", err)
	}
	if !claimed {
		// Une autre instance (ou un run précédent interrompu) a déjà claimé.
		existingRetry, errRetry := s.walletTxnRepo.FindByReferenceIDAdmin(ctx, refType, proofID)
		if errRetry == nil && existingRetry != nil {
			s.logger.Info().
				Str("proof_id", proofID).
				Msg("Escrow already claimed and wallet already credited — skip")
			return nil
		}
		// 🛡️ PR2 : claim OK ailleurs, ledger absent → on complète le crédit
		// SANS re-claim (statut déjà "released"). Même TX, étapes commission + wallet.
		s.logger.Warn().
			Str("proof_id", proofID).
			Str("escrow_id", escrowID).
			Msg("Escrow already claimed but wallet NOT credited — recovering credit only")
		// fall-through : pas de return, on enchaîne commission + wallet ci-dessous
	}

	// 🛡️ 3. Créditer la commission plateforme DANS la transaction
	if commissionCents > 0 && s.platformRevenueRepo != nil {
		const commRefType = "escrow_auto_release_commission"
		if txRepo, ok := s.platformRevenueRepo.(interface {
			WithTx(repository.Tx) repository.PlatformRevenueRepository
		}); ok {
			if err := txRepo.WithTx(tx).CreditRevenue(ctx, commissionCents, commRefType, proofID); err != nil {
				return fmt.Errorf("failed to credit platform commission: %w", err)
			}
		} else {
			if err := s.platformRevenueRepo.CreditRevenue(ctx, commissionCents, commRefType, proofID); err != nil {
				return fmt.Errorf("failed to credit platform commission: %w", err)
			}
		}
		s.logger.Info().
			Str("proof_id", proofID).
			Int64("commission_cents", commissionCents).
			Msg("Platform revenue credited atomically")
	}

	// 🛡️ 4. Créditer le wallet marchand DANS la transaction
	wallet, err := s.walletRepo.WithTX(tx).FindByShopID(shopCtx, shopID)
	if err != nil {
		if err.Error() == "merchant wallet not found" {
			wallet = entity.NewMerchantWallet(shopID)
			if err := s.walletRepo.WithTX(tx).Create(shopCtx, wallet); err != nil {
				return fmt.Errorf("failed to create merchant wallet: %w", err)
			}
		} else {
			return fmt.Errorf("failed to find merchant wallet: %w", err)
		}
	}

	netToBalance, swept, err := wallet.CreditWithDebtSweep(merchantAmountCents)
	if err != nil {
		return fmt.Errorf("failed to credit wallet with debt sweep: %w", err)
	}

	refTypeCopy := refType
	desc := fmt.Sprintf("Auto-release (Proof: %s, Escrow: %s) gross=%d net=%d swept=%d",
		proofID, escrowID, merchantAmountCents, netToBalance, swept)

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
		if err := s.walletTxnRepo.WithTX(tx).Create(shopCtx, txn); err != nil {
			if isUniqueViolation(err) {
				s.logger.Info().
					Str("proof_id", proofID).
					Msg("Unique constraint hit — wallet already credited concurrently")
				return nil
			}
			return fmt.Errorf("failed to create wallet transaction: %w", err)
		}
	} else {
		s.logger.Info().
			Str("proof_id", proofID).
			Int64("swept", swept).
			Msg("Full debt sweep — no sale_credit line (net=0)")
	}

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
		if err := s.walletTxnRepo.WithTX(tx).Create(shopCtx, sweepTxn); err != nil {
			if !isUniqueViolation(err) {
				return fmt.Errorf("failed to create debt_sweep transaction: %w", err)
			}
		}
	}

	if err := s.walletRepo.WithTX(tx).Update(shopCtx, wallet); err != nil {
		return fmt.Errorf("failed to update wallet balance: %w", err)
	}

	// 🛡️ 5. Commit de TOUTES les opérations (Claim + Commission + Wallet, ou credit-only recovery)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	s.logger.Info().
		Str("proof_id", proofID).
		Str("shop_id", shopID).
		Int64("gross_cents", merchantAmountCents).
		Int64("commission_cents", commissionCents).
		Int64("net_to_balance", netToBalance).
		Int64("swept_cents", swept).
		Int64("balance_cents", wallet.BalanceCents).
		Msg("Commission + Wallet credited atomically with debt sweep")

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
