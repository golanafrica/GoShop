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
// SCHEDULER TONTINE (COMMERCIAL / CORPORATE / FAMILY)
// ============================================================

// TontineScheduler collecte automatiquement les commissions
// sur les paiements de tontine (cotisations)
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
}

// NewTontineScheduler crée une nouvelle instance
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
		paymentRepo: paymentRepo,
		groupRepo:   groupRepo,
		batchRepo:   batchRepo,
		rateRepo:    rateRepo,
		debitUC:     debitUC,
		freezeUC:    freezeUC,
		batchSize:   100,
		maxRetries:  3,
		logger:      logger.With().Str("component", "tontine_scheduler").Logger(),
	}
}

// RunCollection exécute la collecte des commissions sur paiements tontine
func (s *TontineScheduler) RunCollection(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Str("type", "tontine").
		Msg("🎯 Starting tontine commission collection")

	// 1. Créer un nouveau batch
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

	// 2. Récupérer les paiements DONE sans commission
	payments, err := s.paymentRepo.FindDoneWithoutCommission(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch tontine payments")
		batch.Status = "failed"
		errMsg := err.Error()
		batch.ErrorMessage = &errMsg
		s.batchRepo.UpdateBatch(ctx, batch)
		return fmt.Errorf("failed to fetch payments: %w", err)
	}

	if len(payments) == 0 {
		s.logger.Info().Msg("No pending tontine payments to process")
		s.finalizeBatch(ctx, batch, startTime)
		return nil
	}

	s.logger.Info().
		Int("payment_count", len(payments)).
		Msg("Processing tontine payments")

	// 3. Traiter chaque paiement
	for _, payment := range payments {
		s.processPayment(ctx, batch, payment)
	}

	// 4. Finaliser le batch
	s.finalizeBatch(ctx, batch, startTime)

	return nil
}

// processPayment traite un paiement tontine individuel
func (s *TontineScheduler) processPayment(
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

	// Créer l'item du batch
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

	// 🆕 v3.3.0 : Vérifier que le shop_id est présent
	if payment.ShopID == "" {
		itemLogger.Error().Msg("Missing shop_id in payment")
		item.Status = "failed"
		errMsg := "missing shop_id in payment"
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// 🆕 v3.3.0 : Créer un contexte avec le tenant (shop)
	shopUUID, err := uuid.Parse(payment.ShopID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Invalid shop UUID")
		item.Status = "failed"
		errMsg := fmt.Sprintf("invalid shop UUID: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	shop := &entity.Shop{ID: shopUUID}
	shopCtx := tenant.WithTenant(ctx, shop)

	// 1. Récupérer le groupe pour connaître le type de cercle
	group, err := s.groupRepo.FindByID(shopCtx, payment.GroupID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to fetch group")
		item.Status = "failed"
		errMsg := fmt.Sprintf("failed to fetch group: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// 2. Mapper le type de cercle vers le type de transaction
	transactionType := s.mapCircleTypeToTransactionType(group.CircleType)

	// 3. Récupérer le taux de commission
	rate, err := s.rateRepo.GetDefaultRate(shopCtx, payment.ShopID, transactionType)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to get commission rate")
		item.Status = "failed"
		errMsg := fmt.Sprintf("failed to get rate: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// 4. Calculer la commission
	commissionCents := rate.CalculateCommission(payment.AmountCents)
	if commissionCents <= 0 {
		itemLogger.Warn().
			Int64("amount_cents", payment.AmountCents).
			Int("rate_bps", rate.RateBps).
			Msg("Commission is zero, skipping")
		item.Status = "skipped"
		batch.SkippedProofs++
		s.batchRepo.CreateBatchItem(ctx, item)
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
		Msg("Processing tontine commission")

	// 5. Tenter de débiter le wallet avec retries
	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		debitReq := &walletusecase.DebitWalletRequest{
			ShopID:          payment.ShopID,
			AmountCents:     commissionCents,
			TransactionType: entity.WalletTxCommissionDebit,
			AllowNegative:   true,
		}

		resp, err := s.debitUC.Execute(shopCtx, debitReq)
		if err != nil {
			lastErr = err
			itemLogger.Warn().
				Err(err).
				Int("attempt", attempt).
				Msg("Debit attempt failed")

			if attempt < s.maxRetries {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			continue
		}

		// Succès !
		item.Status = "success"
		item.WalletBalanceBefore = &resp.PreviousBalance
		item.WalletBalanceAfter = &resp.BalanceAfterCents
		item.AccountFrozen = resp.IsNowNegative

		batch.SuccessfulCollections++
		batch.CollectedCommissionCents += commissionCents

		// 6. Mettre à jour le statut de commission
		batchID := batch.ID
		if err := s.paymentRepo.UpdateTontineCommissionStatus(
			shopCtx,
			payment.ID,
			"collected",
			&batchID,
		); err != nil {
			itemLogger.Error().Err(err).Msg("Failed to update payment commission status")
		}

		// 7. Si le wallet est négatif, le geler
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
			Msg("✅ Tontine commission collected")

		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// Échec après tous les retries
	errMsg := lastErr.Error()
	item.Status = "failed"
	item.ErrorMessage = &errMsg

	batch.FailedCollections++
	batch.FailedCommissionCents += commissionCents

	// Mettre à jour le statut comme failed
	batchID := batch.ID
	s.paymentRepo.UpdateTontineCommissionStatus(
		shopCtx,
		payment.ID,
		"failed",
		&batchID,
	)

	itemLogger.Error().
		Err(lastErr).
		Int("max_retries", s.maxRetries).
		Msg("❌ Tontine commission collection failed")

	s.batchRepo.CreateBatchItem(ctx, item)
}

// mapCircleTypeToTransactionType convertit le type de cercle en type de transaction
func (s *TontineScheduler) mapCircleTypeToTransactionType(circleType string) string {
	switch circleType {
	case entity.TontineCircleCommercial:
		return "tontine_commercial"
	case entity.TontineCircleCorporate:
		return "tontine_corporate"
	case entity.TontineCircleFamily:
		return "tontine_family"
	default:
		return "tontine_commercial" // Fallback
	}
}

// finalizeBatch finalise le batch
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
		Msg("✅ Tontine commission collection completed")
}
