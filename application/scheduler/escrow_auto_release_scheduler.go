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

// EscrowAutoReleaseScheduler libère automatiquement les fonds après 3 jours sans litige.
// Anti race multi-instance :
//  1. ClaimRelease atomique (funds_held → released) — un seul gagnant
//  2. FindByReferenceIDAdmin avant crédit — no-op si déjà crédité
//  3. Unique partial index uq_wallet_txn_ref_completed — filet DB
type EscrowAutoReleaseScheduler struct {
	deliveryProofRepo  repository.DeliveryProofRepository
	escrowRepo         repository.EscrowAccountRepository
	orderRepo          repository.OrderRepository
	tontineVoucherRepo repository.TontineVoucherRepository
	tontineGroupRepo   repository.TontineGroupRepository
	shopRepo           repository.ShopRepository
	walletRepo         repository.MerchantWalletRepository
	walletTxnRepo      repository.WalletTransactionRepository
	creditWalletUC     *walletusecase.CreditWalletUsecase
	batchSize          int
	maxRetries         int
	logger             zerolog.Logger
}

// NewEscrowAutoReleaseScheduler crée une nouvelle instance
func NewEscrowAutoReleaseScheduler(
	deliveryProofRepo repository.DeliveryProofRepository,
	escrowRepo repository.EscrowAccountRepository,
	orderRepo repository.OrderRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	tontineGroupRepo repository.TontineGroupRepository,
	shopRepo repository.ShopRepository,
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
	creditWalletUC *walletusecase.CreditWalletUsecase,
	logger zerolog.Logger,
) *EscrowAutoReleaseScheduler {
	return &EscrowAutoReleaseScheduler{
		deliveryProofRepo:  deliveryProofRepo,
		escrowRepo:         escrowRepo,
		orderRepo:          orderRepo,
		tontineVoucherRepo: tontineVoucherRepo,
		tontineGroupRepo:   tontineGroupRepo,
		shopRepo:           shopRepo,
		walletRepo:         walletRepo,
		walletTxnRepo:      walletTxnRepo,
		creditWalletUC:     creditWalletUC,
		batchSize:          100,
		maxRetries:         3,
		logger:             logger.With().Str("component", "escrow_auto_release_scheduler").Logger(),
	}
}

// RunAutoRelease exécute le déblocage automatique des fonds
func (s *EscrowAutoReleaseScheduler) RunAutoRelease(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Msg("🚀 Starting escrow auto-release")

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
			// Claim perdu ou déjà crédité → skip (pas une erreur)
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
		Msg("✅ Escrow auto-release completed")

	return nil
}

// processProof traite une preuve individuelle avec claim atomique anti-race.
//
// Ordre critique :
//  1. Résoudre shop_id + escrow
//  2. ClaimRelease atomique (funds_held → released) — un seul gagnant multi-instance
//  3. Crédit wallet idempotent (FindByReferenceIDAdmin + unique index)
//  4. Update delivery proof
//
// processProof traite une preuve individuelle avec claim atomique anti-race.
//
// Ordre critique :
//  1. Résoudre shop_id + escrow
//  2. ClaimRelease atomique (funds_held → released) — un seul gagnant multi-instance
//  3. Crédit wallet idempotent UNIQUEMENT pour les orders
//     Tontine : déjà crédité NET+held en fin de cycle → skip crédit ici
//  4. Update delivery proof
func (s *EscrowAutoReleaseScheduler) processProof(ctx context.Context, proof *entity.DeliveryProof) (int64, error) {
	itemLogger := s.logger.With().
		Str("proof_id", proof.ID).
		Str("escrow_status", string(proof.EscrowStatus)).
		Logger()

	// 1. Éligibilité (filtre applicatif ; le claim DB est la source de vérité)
	if !proof.AutoReleaseEligible() {
		return 0, fmt.Errorf("proof not eligible for auto-release (status: %s)", proof.EscrowStatus)
	}

	// 2. Résoudre shop_id
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

	// 3. Trouver l'EscrowAccount
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

	// 4. CLAIM ATOMIQUE — seul gagnant multi-instance
	merchantAmount := escrow.GetMerchantAmount()
	claimed, err := s.escrowRepo.ClaimRelease(ctx, escrow.ID, entity.EscrowAccountFundsHeld, merchantAmount)
	if err != nil {
		return 0, fmt.Errorf("claim release failed: %w", err)
	}
	if !claimed {
		itemLogger.Info().Msg("⏭️ Escrow already claimed by another instance — skip")
		return 0, nil
	}

	itemLogger.Info().Msg("🔒 Escrow claimed successfully")

	// 5. Crédit wallet UNIQUEMENT pour les commandes (orders).
	//    Tontine : déjà crédité NET + held à la fin de cycle (CreditFromTontine).
	//    Redeem libère le held — ne jamais re-créditer ici.
	if proof.TontineVoucherID != nil {
		itemLogger.Info().
			Str("voucher_id", *proof.TontineVoucherID).
			Msg("⏭️ Tontine proof: escrow claimed, wallet already credited at cycle completion — skip credit")

		if err := proof.ReleaseFunds(); err != nil {
			itemLogger.Warn().Err(err).Msg("proof.ReleaseFunds in-memory failed")
		}
		if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
			itemLogger.Error().Err(err).Msg("failed to update delivery proof after tontine claim")
		}
		return merchantAmount, nil
	}

	// Orders only
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

	// 6. Mettre à jour la delivery proof
	if err := proof.ReleaseFunds(); err != nil {
		itemLogger.Warn().Err(err).Msg("proof.ReleaseFunds in-memory failed (may already be released)")
	}
	if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
		itemLogger.Error().Err(err).Msg("failed to update delivery proof after successful release — reconcile manually")
	}

	itemLogger.Info().
		Int64("merchant_amount", merchantAmount).
		Msg("✅ Escrow auto-released and merchant wallet credited")

	return merchantAmount, nil
}

// creditMerchantWallet crédite le wallet du marchand de façon idempotente.
//
// Garde-fous :
//  1. FindByReferenceIDAdmin("escrow_auto_release", proofID) → no-op si déjà crédité
//  2. Unique index uq_wallet_txn_ref_completed → erreur unique → traité comme succès
func (s *EscrowAutoReleaseScheduler) creditMerchantWallet(
	ctx context.Context,
	shopID string,
	amountCents int64,
	proofID string,
	escrowID string,
) error {
	const refType = "escrow_auto_release"

	// --- Idempotence pré-check (admin, sans tenant) ---
	existing, err := s.walletTxnRepo.FindByReferenceIDAdmin(ctx, refType, proofID)
	if err == nil && existing != nil {
		s.logger.Info().
			Str("proof_id", proofID).
			Str("existing_txn_id", existing.ID).
			Msg("⏭️ Wallet already credited for this proof — skip")
		return nil
	}
	// err != nil ou existing == nil → on continue (pas encore crédité)

	// --- Tenant context pour wallet ---
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

	if err := wallet.Credit(amountCents); err != nil {
		return fmt.Errorf("failed to credit wallet: %w", err)
	}

	txnID := uuid.New().String()
	refTypeCopy := refType
	desc := fmt.Sprintf("Auto-release after 3 days (Proof: %s, Escrow: %s)", proofID, escrowID)

	txn := &entity.WalletTransaction{
		ID:                txnID,
		ShopID:            shopID,
		TransactionType:   entity.WalletTxSaleCredit,
		AmountCents:       amountCents,
		BalanceAfterCents: wallet.BalanceCents,
		ReferenceType:     &refTypeCopy,
		ReferenceID:       &proofID,
		Description:       &desc,
		Status:            entity.WalletTxCompleted,
		CreatedAt:         time.Now().UTC(),
	}

	if err := s.walletTxnRepo.Create(shopCtx, txn); err != nil {
		// Unique violation → déjà crédité par une autre instance concurrente
		if isUniqueViolation(err) {
			s.logger.Info().
				Str("proof_id", proofID).
				Msg("⏭️ Unique constraint hit — wallet already credited concurrently")
			return nil
		}
		return fmt.Errorf("failed to create wallet transaction: %w", err)
	}

	if err := s.walletRepo.Update(shopCtx, wallet); err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	return nil
}

// isUniqueViolation détecte une violation d'unicité PostgreSQL (code 23505)
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

// findTontineGroupByVoucher trouve le groupe tontine à partir d'un voucher
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
