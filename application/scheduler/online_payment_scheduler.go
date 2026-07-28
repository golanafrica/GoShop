package scheduler

import (
	"context"
	"fmt"
	"strings" // 🆕 AJOUTÉ pour la détection de l'erreur
	"time"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// SCHEDULER PAIEMENTS EN LIGNE (Orange/Moov/Yenga)
// ============================================================

// OnlinePaymentScheduler collecte automatiquement les commissions
// sur les paiements en ligne (Orange Money, Moov Money, Yenga Pay)
type OnlinePaymentScheduler struct {
	paymentRepo   repository.PaymentRepository
	batchRepo     repository.CommissionBatchRepository
	rateRepo      repository.CommissionRateRepository
	walletRepo    repository.MerchantWalletRepository
	walletTxnRepo repository.WalletTransactionRepository
	debitUC       *walletusecase.DebitWalletUsecase
	freezeUC      *walletusecase.FreezeAccountUsecase
	batchSize     int
	maxRetries    int
	logger        zerolog.Logger
}

// NewOnlinePaymentScheduler crée une nouvelle instance
func NewOnlinePaymentScheduler(
	paymentRepo repository.PaymentRepository,
	batchRepo repository.CommissionBatchRepository,
	rateRepo repository.CommissionRateRepository,
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
	debitUC *walletusecase.DebitWalletUsecase,
	freezeUC *walletusecase.FreezeAccountUsecase,
	logger zerolog.Logger,
) *OnlinePaymentScheduler {
	return &OnlinePaymentScheduler{
		paymentRepo:   paymentRepo,
		batchRepo:     batchRepo,
		rateRepo:      rateRepo,
		walletRepo:    walletRepo,
		walletTxnRepo: walletTxnRepo,
		debitUC:       debitUC,
		freezeUC:      freezeUC,
		batchSize:     100,
		maxRetries:    3,
		logger:        logger.With().Str("component", "online_payment_scheduler").Logger(),
	}
}

// RunCollection exécute la collecte des commissions sur paiements en ligne
func (s *OnlinePaymentScheduler) RunCollection(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Str("type", "online_payment").
		Msg("💳 Starting online payment commission collection")

	// 1. Créer un nouveau batch
	batch := &repository.CommissionBatch{
		ID:          uuid.New().String(),
		StartedAt:   startTime,
		Status:      "running",
		TriggeredBy: "online_payment_scheduler",
		CreatedAt:   startTime,
	}

	if err := s.batchRepo.CreateBatch(ctx, batch); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create batch")
		return fmt.Errorf("failed to create batch: %w", err)
	}

	// 2. Récupérer les paiements completed sans commission
	payments, err := s.paymentRepo.FindCompletedWithoutCommission(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch payments")
		batch.Status = "failed"
		errMsg := err.Error()
		batch.ErrorMessage = &errMsg
		s.batchRepo.UpdateBatch(ctx, batch)
		return fmt.Errorf("failed to fetch payments: %w", err)
	}

	if len(payments) == 0 {
		s.logger.Info().Msg("No pending online payments to process")
		s.finalizeBatch(ctx, batch, startTime)
		return nil
	}

	s.logger.Info().
		Int("payment_count", len(payments)).
		Msg("Processing online payments")

	// 3. Traiter chaque paiement
	for _, payment := range payments {
		s.processPayment(ctx, batch, payment)
	}

	// 4. Finaliser le batch
	s.finalizeBatch(ctx, batch, startTime)

	return nil
}

// processPayment traite un paiement individuel
func (s *OnlinePaymentScheduler) processPayment(
	ctx context.Context,
	batch *repository.CommissionBatch,
	payment *entity.Payment,
) {
	itemLogger := s.logger.With().
		Str("payment_id", payment.ID.String()).
		Str("order_id", payment.OrderID.String()).
		Str("shop_id", payment.ShopID.String()).
		Str("provider", string(payment.Provider)).
		Logger()

	batch.TotalProofs++

	// Créer l'item du batch
	item := &repository.CommissionBatchItem{
		ID:              uuid.New().String(),
		BatchID:         batch.ID,
		CODProofID:      payment.ID.String(), // On réutilise ce champ pour le payment ID
		OrderID:         payment.OrderID.String(),
		ShopID:          payment.ShopID.String(),
		CustomerID:      "", // Pas de customer ID direct dans Payment
		CommissionCents: 0,
		ProcessedAt:     time.Now(),
	}

	// 1. Récupérer le taux de commission de la boutique
	rate, err := s.rateRepo.GetDefaultRate(ctx, payment.ShopID.String(), entity.TransactionTypeOnlinePayment)
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
		Int64("amount_cents", payment.AmountCents).
		Int64("commission_cents", commissionCents).
		Int("rate_bps", rate.RateBps).
		Msg("Processing commission")

	// 3. Créer le contexte multi-tenant
	shopUUID, err := uuid.Parse(payment.ShopID.String())
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

	// 4. Tenter de débiter le wallet avec retries
	var lastErr error
	skippedNoWallet := false // 🆕 Flag pour gérer le skip silencieux

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		debitReq := &walletusecase.DebitWalletRequest{
			ShopID:          payment.ShopID.String(),
			AmountCents:     commissionCents,
			TransactionType: entity.WalletTxCommissionDebit,
			AllowNegative:   true, // Permet le négatif, puis freeze automatique
		}

		resp, err := s.debitUC.Execute(shopCtx, debitReq)
		if err != nil {
			// 🆕 AMÉLIORATION : Skipper silencieusement si le wallet n'existe pas
			if strings.Contains(err.Error(), "merchant wallet not found") {
				itemLogger.Info().Msg("Merchant wallet not found, skipping commission collection")
				skippedNoWallet = true
				break
			}

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

		// 5. Mettre à jour le statut de la commission dans le paiement
		if err := s.paymentRepo.UpdateCommissionStatus(
			ctx,
			payment.ID.String(),
			entity.CommissionStatusCollected,
			commissionCents,
		); err != nil {
			itemLogger.Error().Err(err).Msg("Failed to update payment commission status")
			// On continue, la commission est collectée mais le statut n'est pas à jour
		}

		// 6. Si le wallet est négatif, le geler
		if resp.IsNowNegative && s.freezeUC != nil {
			freezeDetails := fmt.Sprintf("Online payment commission: %d FCFA", commissionCents/100)
			freezeReq := &walletusecase.FreezeAccountRequest{
				ShopID:         payment.ShopID.String(),
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
			Msg("✅ Online payment commission collected")

		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// 🆕 AMÉLIORATION : Gestion du skip si pas de wallet
	if skippedNoWallet {
		item.Status = "skipped"
		batch.SkippedProofs++
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
	s.paymentRepo.UpdateCommissionStatus(
		ctx,
		payment.ID.String(),
		entity.CommissionStatusFailed,
		commissionCents,
	)

	itemLogger.Error().
		Err(lastErr).
		Int("max_retries", s.maxRetries).
		Msg("❌ Online payment commission collection failed")

	s.batchRepo.CreateBatchItem(ctx, item)
}

// finalizeBatch finalise le batch
func (s *OnlinePaymentScheduler) finalizeBatch(
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
		Msg("✅ Online payment commission collection completed")
}
