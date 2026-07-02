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
// SCHEDULER CRÉDIT (ÉCHÉANCES MENSUELLES)
// ============================================================

// CreditScheduler collecte automatiquement les commissions
// sur les échéances de crédit payées (0.5% par échéance)
type CreditScheduler struct {
	installmentRepo repository.CreditInstallmentRepository
	batchRepo       repository.CommissionBatchRepository
	rateRepo        repository.CommissionRateRepository
	debitUC         *walletusecase.DebitWalletUsecase
	freezeUC        *walletusecase.FreezeAccountUsecase
	batchSize       int
	maxRetries      int
	logger          zerolog.Logger
}

// NewCreditScheduler crée une nouvelle instance
func NewCreditScheduler(
	installmentRepo repository.CreditInstallmentRepository,
	batchRepo repository.CommissionBatchRepository,
	rateRepo repository.CommissionRateRepository,
	debitUC *walletusecase.DebitWalletUsecase,
	freezeUC *walletusecase.FreezeAccountUsecase,
	logger zerolog.Logger,
) *CreditScheduler {
	return &CreditScheduler{
		installmentRepo: installmentRepo,
		batchRepo:       batchRepo,
		rateRepo:        rateRepo,
		debitUC:         debitUC,
		freezeUC:        freezeUC,
		batchSize:       100,
		maxRetries:      3,
		logger:          logger.With().Str("component", "credit_scheduler").Logger(),
	}
}

// RunCollection exécute la collecte des commissions sur échéances de crédit
func (s *CreditScheduler) RunCollection(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Str("type", "credit").
		Msg("💰 Starting credit commission collection")

	// 1. Créer un nouveau batch
	batch := &repository.CommissionBatch{
		ID:          uuid.New().String(),
		StartedAt:   startTime,
		Status:      "running",
		TriggeredBy: "credit_scheduler",
		CreatedAt:   startTime,
	}

	if err := s.batchRepo.CreateBatch(ctx, batch); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create batch")
		return fmt.Errorf("failed to create batch: %w", err)
	}

	// 2. Récupérer les échéances payées sans commission
	installments, err := s.installmentRepo.FindPaidWithoutCommission(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch paid installments")
		batch.Status = "failed"
		errMsg := err.Error()
		batch.ErrorMessage = &errMsg
		s.batchRepo.UpdateBatch(ctx, batch)
		return fmt.Errorf("failed to fetch installments: %w", err)
	}

	if len(installments) == 0 {
		s.logger.Info().Msg("No pending credit installments to process")
		s.finalizeBatch(ctx, batch, startTime)
		return nil
	}

	s.logger.Info().
		Int("installment_count", len(installments)).
		Msg("Processing credit installments")

	// 3. Traiter chaque échéance
	for _, installment := range installments {
		s.processInstallment(ctx, batch, installment)
	}

	// 4. Finaliser le batch
	s.finalizeBatch(ctx, batch, startTime)

	return nil
}

// processInstallment traite une échéance de crédit individuelle
func (s *CreditScheduler) processInstallment(
	ctx context.Context,
	batch *repository.CommissionBatch,
	installment *entity.CreditInstallment,
) {
	itemLogger := s.logger.With().
		Str("installment_id", installment.ID).
		Str("contract_id", installment.ContractID).
		Str("shop_id", installment.ShopID).
		Int("installment_number", installment.InstallmentNumber).
		Logger()

	batch.TotalProofs++

	// Créer l'item du batch
	item := &repository.CommissionBatchItem{
		ID:              uuid.New().String(),
		BatchID:         batch.ID,
		CODProofID:      installment.ID, // On réutilise ce champ pour le installment ID
		OrderID:         installment.ContractID,
		ShopID:          installment.ShopID,
		CustomerID:      "", // Sera récupéré via le contrat si nécessaire
		CommissionCents: 0,
		ProcessedAt:     time.Now(),
	}

	// 🆕 v3.4.0 : Vérifier que le shop_id est présent
	if installment.ShopID == "" {
		itemLogger.Error().Msg("Missing shop_id in installment")
		item.Status = "failed"
		errMsg := "missing shop_id in installment"
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// 🆕 v3.4.0 : Créer un contexte avec le tenant (shop)
	shopUUID, err := uuid.Parse(installment.ShopID)
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

	// 1. Récupérer le taux de commission (type 'credit' = 0.5%)
	rate, err := s.rateRepo.GetDefaultRate(shopCtx, installment.ShopID, entity.TransactionTypeCredit)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to get commission rate")
		item.Status = "failed"
		errMsg := fmt.Sprintf("failed to get rate: %v", err)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// 2. Calculer la commission
	commissionCents := rate.CalculateCommission(installment.AmountCents)
	if commissionCents <= 0 {
		itemLogger.Warn().
			Int64("amount_cents", installment.AmountCents).
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
		Int64("amount_cents", installment.AmountCents).
		Int64("commission_cents", commissionCents).
		Int("rate_bps", rate.RateBps).
		Msg("Processing credit commission")

	// 3. Tenter de débiter le wallet avec retries
	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		debitReq := &walletusecase.DebitWalletRequest{
			ShopID:          installment.ShopID,
			AmountCents:     commissionCents,
			TransactionType: entity.WalletTxCommissionDebit,
			AllowNegative:   true, // Permet le négatif, puis freeze automatique
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

		// 4. Mettre à jour le statut de commission dans l'échéance
		batchID := batch.ID
		if err := s.installmentRepo.UpdateCreditCommissionStatus(
			shopCtx,
			installment.ID,
			entity.CreditCommissionCollected,
			commissionCents,
			&batchID,
		); err != nil {
			itemLogger.Error().Err(err).Msg("Failed to update installment commission status")
			// On continue, la commission est collectée mais le statut n'est pas à jour
		}

		// 5. Si le wallet est négatif, le geler
		if resp.IsNowNegative && s.freezeUC != nil {
			freezeDetails := fmt.Sprintf("Credit commission (installment #%d): %d FCFA",
				installment.InstallmentNumber, commissionCents/100)
			freezeReq := &walletusecase.FreezeAccountRequest{
				ShopID:         installment.ShopID,
				Reason:         entity.FreezeReasonNegativeBalance,
				AmountDueCents: -resp.BalanceAfterCents,
				Details:        &freezeDetails,
			}

			if _, err := s.freezeUC.Execute(shopCtx, freezeReq); err != nil {
				itemLogger.Warn().Err(err).Msg("Failed to freeze account")
				// On continue, ce n'est pas bloquant
			}
		}

		itemLogger.Info().
			Int64("commission_cents", commissionCents).
			Int64("balance_before", resp.PreviousBalance).
			Int64("balance_after", resp.BalanceAfterCents).
			Bool("account_frozen", resp.IsNowNegative).
			Msg("✅ Credit commission collected")

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
	s.installmentRepo.UpdateCreditCommissionStatus(
		shopCtx,
		installment.ID,
		entity.CreditCommissionFailed,
		commissionCents,
		&batchID,
	)

	itemLogger.Error().
		Err(lastErr).
		Int("max_retries", s.maxRetries).
		Msg("❌ Credit commission collection failed")

	s.batchRepo.CreateBatchItem(ctx, item)
}

// finalizeBatch finalise le batch
func (s *CreditScheduler) finalizeBatch(
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
		Msg("✅ Credit commission collection completed")
}
