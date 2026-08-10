package scheduler

import (
	"context"
	"fmt"
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

// EscrowAutoReleaseScheduler libère automatiquement les fonds après 3 jours sans litige
type EscrowAutoReleaseScheduler struct {
	deliveryProofRepo  repository.DeliveryProofRepository
	escrowRepo         repository.EscrowAccountRepository
	orderRepo          repository.OrderRepository
	tontineVoucherRepo repository.TontineVoucherRepository
	tontineGroupRepo   repository.TontineGroupRepository
	shopRepo           repository.ShopRepository // pour créer le tenant context
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

	// 1. Récupérer les preuves éligibles (delivered + 3 jours + pas de litige actif côté proof)
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

	// 2. Traiter chaque preuve
	var releasedCount, failedCount int
	var releasedCents int64

	for _, proof := range proofs {
		released, err := s.processProof(ctx, proof)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("proof_id", proof.ID).
				Msg("Failed to auto-release proof")
			failedCount++
		} else {
			releasedCount++
			if released > 0 {
				releasedCents += released
			}
		}
	}

	// 3. Logger le résumé
	duration := time.Since(startTime)
	s.logger.Info().
		Int("released_count", releasedCount).
		Int("failed_count", failedCount).
		Int64("released_cents", releasedCents).
		Int("duration_ms", int(duration.Milliseconds())).
		Msg("✅ Escrow auto-release completed")

	return nil
}

// processProof traite une preuve individuelle
// Utilise FindByIDAdmin pour bypasser le multi-tenant (contexte scheduler).
func (s *EscrowAutoReleaseScheduler) processProof(ctx context.Context, proof *entity.DeliveryProof) (int64, error) {
	itemLogger := s.logger.With().
		Str("proof_id", proof.ID).
		Str("proof_escrow_status", string(proof.EscrowStatus)).
		Logger()

	// 1. Vérifier que la preuve est bien éligible
	if !proof.AutoReleaseEligible() {
		return 0, fmt.Errorf("proof not eligible for auto-release (status: %s)", proof.EscrowStatus)
	}

	// 2. Déterminer le shop_id selon le type de référence
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

	// 3. Trouver l'EscrowAccount AVANT de muter la proof
	//    (si litige / terminal → on ne touche ni proof ni wallet)
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
		Str("escrow_account_status", string(escrow.Status)).
		Int64("total_cents", escrow.TotalAmountCents).
		Int64("commission_cents", escrow.CommissionCents).
		Logger()

	// ────────────────────────────────────────────────────────────
	// Phase 3.1 — sécurité litige / double release
	// Ne JAMAIS auto-libérer un escrow disputed ou déjà terminal.
	// funds_held reste autorisé (happy path après preuves).
	// ────────────────────────────────────────────────────────────
	switch escrow.Status {
	case entity.EscrowAccountDisputed:
		itemLogger.Warn().Msg("skip auto-release: escrow is disputed")
		return 0, fmt.Errorf("escrow is disputed — auto-release blocked until admin resolution")
	case entity.EscrowAccountFullyReleased, entity.EscrowAccountRefunded:
		itemLogger.Info().Msg("skip auto-release: escrow already terminal")
		return 0, fmt.Errorf("escrow already terminal (status: %s)", escrow.Status)
	}

	// 4. Marquer la DeliveryProof comme released
	if err := proof.ReleaseFunds(); err != nil {
		return 0, fmt.Errorf("failed to release delivery proof: %w", err)
	}

	// 5. Libérer les fonds de l'escrow
	if err := escrow.ReleaseFunds(); err != nil {
		return 0, fmt.Errorf("failed to release escrow: %w", err)
	}

	// 6. Montant net marchand = TotalAmountCents - CommissionCents (GoShop)
	merchantAmount := escrow.GetMerchantAmount()

	// 7. Créditer le wallet du marchand (avec retry)
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
		return 0, fmt.Errorf("failed to credit merchant wallet after %d attempts: %w", s.maxRetries, lastErr)
	}

	// 8. Persister la preuve
	if err := s.deliveryProofRepo.Update(ctx, proof); err != nil {
		return 0, fmt.Errorf("failed to update delivery proof: %w", err)
	}

	// 9. Persister l'escrow
	if err := s.escrowRepo.Update(ctx, escrow); err != nil {
		return 0, fmt.Errorf("failed to update escrow: %w", err)
	}

	itemLogger.Info().
		Int64("merchant_amount", merchantAmount).
		Msg("✅ Escrow auto-released and merchant wallet credited")

	return merchantAmount, nil
}

// creditMerchantWallet crédite le wallet du marchand
// Crée un contexte avec tenant pour les appels wallet.
func (s *EscrowAutoReleaseScheduler) creditMerchantWallet(
	ctx context.Context,
	shopID string,
	amountCents int64,
	proofID string,
	escrowID string,
) error {
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
	refType := "escrow_auto_release"
	desc := fmt.Sprintf("Auto-release after 3 days (Proof: %s, Escrow: %s)", proofID, escrowID)

	txn := &entity.WalletTransaction{
		ID:                txnID,
		ShopID:            shopID,
		TransactionType:   entity.WalletTxSaleCredit,
		AmountCents:       amountCents,
		BalanceAfterCents: wallet.BalanceCents,
		ReferenceType:     &refType,
		ReferenceID:       &proofID,
		Description:       &desc,
		Status:            entity.WalletTxCompleted,
		CreatedAt:         time.Now().UTC(),
	}

	if err := s.walletTxnRepo.Create(shopCtx, txn); err != nil {
		return fmt.Errorf("failed to create wallet transaction: %w", err)
	}

	if err := s.walletRepo.Update(shopCtx, wallet); err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	return nil
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
