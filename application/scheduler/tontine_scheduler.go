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
// SCHEDULER TONTINE
// ============================================================
// Phase 1.2 (held/net model) :
// La commission plateforme est retenue UNE FOIS sur le TOTAL du cycle
// dans process_tontine_webhook.checkAndCompleteCycle (crédit NET).
// Ce scheduler NE DOIT PLUS débiter commission_debit par cotisation,
// sinon double prélèvement (net déjà réduit + N × debit).
//
// Comportement actuel : no-op documenté (batch vide / skip global).
// Plus tard (Phase settlement) : ce job pourra gérer uniquement les
// retries / cas pathologiques, pas la collecte nominale.
// ============================================================

// TontineScheduler historiquement collectait les commissions cotisation par cotisation.
// Phase 1.2 : collecte par ligne DÉSACTIVÉE.
type TontineScheduler struct {
	paymentRepo repository.TontinePaymentRepository
	groupRepo   repository.TontineGroupRepository
	batchRepo   repository.CommissionBatchRepository
	rateRepo    repository.CommissionRateRepository
	debitUC     *walletusecase.DebitWalletUsecase
	freezeUC    *walletusecase.FreezeAccountUsecase
	batchSize   int
	maxRetries  int
	logger      zerolog.Logger

	// collectPerCotisation : false = Phase 1.2 (défaut). true = ancien comportement (debug only).
	collectPerCotisation bool
}

// NewTontineScheduler crée une nouvelle instance (collecte par cotisation OFF).
func NewTontineScheduler(
	paymentRepo repository.TontinePaymentRepository,
	groupRepo repository.TontineGroupRepository,
	batchRepo repository.CommissionBatchRepository,
	rateRepo repository.CommissionRateRepository,
	debitUC *walletusecase.DebitWalletUsecase,
	freezeUC *walletusecase.FreezeAccountUsecase,
	logger zerolog.Logger,
) *TontineScheduler {
	return &TontineScheduler{
		paymentRepo:          paymentRepo,
		groupRepo:            groupRepo,
		batchRepo:            batchRepo,
		rateRepo:             rateRepo,
		debitUC:              debitUC,
		freezeUC:             freezeUC,
		batchSize:            100,
		maxRetries:           3,
		logger:               logger.With().Str("component", "tontine_scheduler").Logger(),
		collectPerCotisation: false, // Phase 1.2
	}
}

// RunCollection exécute le job cron tontine.
// Phase 1.2 : ne prélève plus de commission par paiement DONE.
func (s *TontineScheduler) RunCollection(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Bool("collect_per_cotisation", s.collectPerCotisation).
		Str("type", "tontine").
		Msg("🎯 Starting tontine scheduler run")

	batch := &repository.CommissionBatch{
		ID:          uuid.New().String(),
		StartedAt:   startTime,
		Status:      "running",
		TriggeredBy: "tontine_scheduler",
		CreatedAt:   startTime,
	}

	if err := s.batchRepo.CreateBatch(ctx, batch); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create batch")
		return fmt.Errorf("failed to create batch: %w", err)
	}

	// ── Phase 1.2 : short-circuit ──────────────────────────────────────────
	if !s.collectPerCotisation {
		s.logger.Info().
			Str("batch_id", batch.ID).
			Msg("Phase 1.2: per-cotisation commission collection DISABLED — commission already withheld via net cycle credit in process_tontine_webhook")
		s.finalizeBatch(ctx, batch, startTime)
		return nil
	}

	// ── Ancien chemin (uniquement si collectPerCotisation = true) ──────────
	payments, err := s.paymentRepo.FindDoneWithoutCommission(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch tontine payments")
		batch.Status = "failed"
		errMsg := err.Error()
		batch.ErrorMessage = &errMsg
		_ = s.batchRepo.UpdateBatch(ctx, batch)
		return fmt.Errorf("failed to fetch payments: %w", err)
	}

	if len(payments) == 0 {
		s.logger.Info().Msg("No pending tontine payments to process")
		s.finalizeBatch(ctx, batch, startTime)
		return nil
	}

	s.logger.Info().Int("payment_count", len(payments)).Msg("Processing tontine payments (legacy per-cotisation mode)")

	for _, payment := range payments {
		s.processPaymentLegacy(ctx, batch, payment)
	}

	s.finalizeBatch(ctx, batch, startTime)
	return nil
}

// processPaymentLegacy = ancien débit par cotisation (conservé pour debug / rollback).
// Ne pas activer en prod tant que le crédit net cycle est en place.
func (s *TontineScheduler) processPaymentLegacy(
	ctx context.Context,
	batch *repository.CommissionBatch,
	payment *entity.TontinePayment,
) {
	itemLogger := s.logger.With().
		Str("payment_id", payment.ID).
		Str("group_id", payment.GroupID).
		Str("customer_id", payment.CustomerID).
		Str("shop_id", payment.ShopID).
		Logger()

	batch.TotalProofs++

	item := &repository.CommissionBatchItem{
		ID:              uuid.New().String(),
		BatchID:         batch.ID,
		CODProofID:      payment.ID,
		OrderID:         payment.GroupID,
		ShopID:          payment.ShopID,
		CustomerID:      payment.CustomerID,
		CommissionCents: 0,
		ProcessedAt:     time.Now(),
	}

	if payment.ShopID == "" {
		itemLogger.Error().Msg("Missing shop_id in payment")
		item.Status = "failed"
		errMsg := "missing shop_id in payment"
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	shopUUID, err := uuid.Parse(payment.ShopID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Invalid shop UUID")
		item.Status = "failed"
		errMsg := fmt.Sprintf("invalid shop UUID: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	shop := &entity.Shop{ID: shopUUID}
	shopCtx := tenant.WithTenant(ctx, shop)

	group, err := s.groupRepo.FindByID(shopCtx, payment.GroupID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to fetch group")
		item.Status = "failed"
		errMsg := fmt.Sprintf("failed to fetch group: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	transactionType := s.mapCircleTypeToTransactionType(group.CircleType)

	rate, err := s.rateRepo.GetDefaultRate(shopCtx, payment.ShopID, transactionType)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to get commission rate")
		item.Status = "failed"
		errMsg := fmt.Sprintf("failed to get rate: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	commissionCents := rate.CalculateCommission(payment.AmountCents)
	if commissionCents <= 0 {
		itemLogger.Warn().
			Int64("amount_cents", payment.AmountCents).
			Int("rate_bps", rate.RateBps).
			Msg("Commission is zero, skipping")
		item.Status = "skipped"
		batch.SkippedProofs++
		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	item.CommissionCents = commissionCents
	batch.TotalCommissionCents += commissionCents

	itemLogger.Info().
		Str("circle_type", group.CircleType).
		Str("transaction_type", transactionType).
		Int64("amount_cents", payment.AmountCents).
		Int64("commission_cents", commissionCents).
		Int("rate_bps", rate.RateBps).
		Msg("Processing tontine commission (LEGACY per-cotisation)")

	var lastErr error
	skippedNoWallet := false

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		debitReq := &walletusecase.DebitWalletRequest{
			ShopID:          payment.ShopID,
			AmountCents:     commissionCents,
			TransactionType: entity.WalletTxCommissionDebit,
			AllowNegative:   true,
		}

		resp, err := s.debitUC.Execute(shopCtx, debitReq)
		if err != nil {
			if containsWalletNotFound(err.Error()) {
				itemLogger.Info().Msg("Merchant wallet not found, skipping commission collection")
				skippedNoWallet = true
				break
			}

			lastErr = err
			itemLogger.Warn().Err(err).Int("attempt", attempt).Msg("Debit attempt failed")
			if attempt < s.maxRetries {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			continue
		}

		item.Status = "success"
		item.WalletBalanceBefore = &resp.PreviousBalance
		item.WalletBalanceAfter = &resp.BalanceAfterCents
		item.AccountFrozen = resp.IsNowNegative

		batch.SuccessfulCollections++
		batch.CollectedCommissionCents += commissionCents

		batchID := batch.ID
		if err := s.paymentRepo.UpdateTontineCommissionStatus(
			shopCtx,
			payment.ID,
			"collected",
			&batchID,
		); err != nil {
			itemLogger.Error().Err(err).Msg("Failed to update payment commission status")
		}

		if resp.IsNowNegative && s.freezeUC != nil {
			freezeDetails := fmt.Sprintf("Tontine commission (%s): %d FCFA", group.CircleType, commissionCents/100)
			freezeReq := &walletusecase.FreezeAccountRequest{
				ShopID:         payment.ShopID,
				Reason:         entity.FreezeReasonNegativeBalance,
				AmountDueCents: -resp.BalanceAfterCents,
				Details:        &freezeDetails,
			}
			if _, err := s.freezeUC.Execute(shopCtx, freezeReq); err != nil {
				itemLogger.Warn().Err(err).Msg("Failed to freeze account")
			}
		}

		itemLogger.Info().
			Int64("commission_cents", commissionCents).
			Int64("balance_before", resp.PreviousBalance).
			Int64("balance_after", resp.BalanceAfterCents).
			Bool("account_frozen", resp.IsNowNegative).
			Msg("✅ Tontine commission collected (LEGACY)")

		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	if skippedNoWallet {
		item.Status = "skipped"
		batch.SkippedProofs++
		_ = s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	errMsg := "unknown error"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	item.Status = "failed"
	item.ErrorMessage = &errMsg

	batch.FailedCollections++
	batch.FailedCommissionCents += commissionCents

	batchID := batch.ID
	_ = s.paymentRepo.UpdateTontineCommissionStatus(
		shopCtx,
		payment.ID,
		"failed",
		&batchID,
	)

	itemLogger.Error().
		Err(lastErr).
		Int("max_retries", s.maxRetries).
		Msg("❌ Tontine commission collection failed (LEGACY)")

	_ = s.batchRepo.CreateBatchItem(ctx, item)
}

func containsWalletNotFound(msg string) bool {
	return len(msg) > 0 && (containsFold(msg, "merchant wallet not found") || containsFold(msg, "wallet not found"))
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		indexFold(s, substr) >= 0)
}

func indexFold(s, substr string) int {
	// simple case-insensitive search without importing strings (kept light)
	ls, lsub := len(s), len(substr)
	if lsub == 0 {
		return 0
	}
	for i := 0; i+lsub <= ls; i++ {
		ok := true
		for j := 0; j < lsub; j++ {
			a, b := s[i+j], substr[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func (s *TontineScheduler) mapCircleTypeToTransactionType(circleType string) string {
	switch circleType {
	case entity.TontineCircleCommercial:
		return "tontine_commercial"
	case entity.TontineCircleCorporate:
		return "tontine_corporate"
	case entity.TontineCircleFamily:
		return "tontine_family"
	default:
		return "tontine_commercial"
	}
}

func (s *TontineScheduler) finalizeBatch(
	ctx context.Context,
	batch *repository.CommissionBatch,
	startTime time.Time,
) {
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	durationMs := int(duration.Milliseconds())

	batch.CompletedAt = &endTime
	batch.DurationMs = &durationMs
	batch.Status = "completed"

	if err := s.batchRepo.UpdateBatch(ctx, batch); err != nil {
		s.logger.Error().Err(err).Msg("Failed to update batch")
		return
	}

	s.logger.Info().
		Str("batch_id", batch.ID).
		Int("total_proofs", batch.TotalProofs).
		Int("successful", batch.SuccessfulCollections).
		Int("failed", batch.FailedCollections).
		Int("skipped", batch.SkippedProofs).
		Int64("collected_cents", batch.CollectedCommissionCents).
		Int64("failed_cents", batch.FailedCommissionCents).
		Int("duration_ms", durationMs).
		Msg("✅ Tontine scheduler run completed")
}
