package scheduler

import (
	"context"
	"fmt"
	"time"

	creditusecase "Goshop/application/usecase/credit_usecase"
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

type CreditScheduler struct {
	installmentRepo  repository.CreditInstallmentRepository
	contractRepo     repository.CreditContractRepository
	customerRepo     repository.CustomerRepositoryInterface
	payInstallmentUC *creditusecase.PayInstallmentUsecase
	batchRepo        repository.CommissionBatchRepository
	rateRepo         repository.CommissionRateRepository
	debitUC          *walletusecase.DebitWalletUsecase
	freezeUC         *walletusecase.FreezeAccountUsecase
	batchSize        int
	maxRetries       int
	logger           zerolog.Logger
}

func NewCreditScheduler(
	installmentRepo repository.CreditInstallmentRepository,
	contractRepo repository.CreditContractRepository,
	customerRepo repository.CustomerRepositoryInterface,
	payInstallmentUC *creditusecase.PayInstallmentUsecase,
	batchRepo repository.CommissionBatchRepository,
	rateRepo repository.CommissionRateRepository,
	debitUC *walletusecase.DebitWalletUsecase,
	freezeUC *walletusecase.FreezeAccountUsecase,
	logger zerolog.Logger,
) *CreditScheduler {
	return &CreditScheduler{
		installmentRepo:  installmentRepo,
		contractRepo:     contractRepo,
		customerRepo:     customerRepo,
		payInstallmentUC: payInstallmentUC,
		batchRepo:        batchRepo,
		rateRepo:         rateRepo,
		debitUC:          debitUC,
		freezeUC:         freezeUC,
		batchSize:        100,
		maxRetries:       3,
		logger:           logger.With().Str("component", "credit_scheduler").Logger(),
	}
}

// TriggerDueInstallments déclenche les demandes de paiement pour les échéances dues
func (s *CreditScheduler) TriggerDueInstallments(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Msg("🔔 Triggering due credit installment payments")

	// 1. Récupérer les échéances dues (status pending/late et due_date <= aujourd'hui)
	installments, err := s.installmentRepo.FindDueInstallments(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch due installments")
		return fmt.Errorf("failed to fetch due installments: %w", err)
	}

	if len(installments) == 0 {
		s.logger.Info().Msg("No due installments to process")
		return nil
	}

	s.logger.Info().
		Int("installment_count", len(installments)).
		Msg("Processing due installments")

	// 2. Traiter chaque échéance
	for _, installment := range installments {
		s.triggerInstallmentPayment(ctx, installment)
	}

	s.logger.Info().
		Int("processed_count", len(installments)).
		Dur("duration_ms", time.Since(startTime)).
		Msg("✅ Due installment payment triggers completed")

	return nil
}

func (s *CreditScheduler) triggerInstallmentPayment(ctx context.Context, installment *entity.CreditInstallment) {
	itemLogger := s.logger.With().
		Str("installment_id", installment.ID).
		Str("contract_id", installment.ContractID).
		Logger()

	// 1. Récupérer le contrat pour avoir le CustomerID et ShopID
	contract, err := s.contractRepo.FindByID(ctx, installment.ContractID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to find contract")
		return
	}

	// 2. Récupérer le client pour avoir le numéro de téléphone
	customer, err := s.customerRepo.FindByCustomerID(ctx, contract.CustomerID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to find customer")
		return
	}

	// ✅ CORRECTION : Utilise le champ PhoneNumber que nous venons d'ajouter à l'entité Customer
	if customer.PhoneNumber == "" {
		itemLogger.Warn().Msg("Customer has no phone number, skipping payment trigger")
		return
	}

	// 3. Déterminer l'opérateur (par défaut ORANGE, ou basé sur une logique métier)
	operator := "ORANGE"

	// 4. Initier le paiement via le usecase existant
	req := &creditusecase.PayInstallmentRequest{
		InstallmentID: installment.ID,
		PhoneNumber:   customer.PhoneNumber, // ✅ CORRECTION : Utilise le bon champ
		Operator:      operator,
		// Pas d'OTP ici, car c'est un push qui demandera au client de confirmer sur son téléphone
	}

	// On utilise un contexte avec le tenant du shop
	shopUUID, err := uuid.Parse(contract.ShopID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Invalid shop UUID")
		return
	}
	shop := &entity.Shop{ID: shopUUID}
	shopCtx := tenant.WithTenant(ctx, shop)

	_, err = s.payInstallmentUC.Execute(shopCtx, req)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to trigger installment payment")
		return
	}

	itemLogger.Info().
		Str("phone", customer.PhoneNumber). // ✅ CORRECTION : Utilise le bon champ
		Str("operator", operator).
		Msg("✅ Installment payment push triggered successfully")
}

// RunCollection exécute la collecte des commissions sur échéances de crédit (existant)
func (s *CreditScheduler) RunCollection(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Str("type", "credit").
		Msg("💰 Starting credit commission collection")

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

	s.logger.Info().Int("installment_count", len(installments)).Msg("Processing credit installments")

	for _, installment := range installments {
		s.processInstallment(ctx, batch, installment)
	}

	s.finalizeBatch(ctx, batch, startTime)
	return nil
}

// processInstallment traite une échéance de crédit individuelle (existant, inchangé)
func (s *CreditScheduler) processInstallment(ctx context.Context, batch *repository.CommissionBatch, installment *entity.CreditInstallment) {
	itemLogger := s.logger.With().
		Str("installment_id", installment.ID).
		Str("contract_id", installment.ContractID).
		Str("shop_id", installment.ShopID).
		Int("installment_number", installment.InstallmentNumber).
		Logger()

	batch.TotalProofs++

	item := &repository.CommissionBatchItem{
		ID:              uuid.New().String(),
		BatchID:         batch.ID,
		CODProofID:      installment.ID,
		OrderID:         installment.ContractID,
		ShopID:          installment.ShopID,
		CustomerID:      "",
		CommissionCents: 0,
		ProcessedAt:     time.Now(),
	}

	if installment.ShopID == "" {
		itemLogger.Error().Msg("Missing shop_id in installment")
		item.Status = "failed"
		errMsg := "missing shop_id in installment"
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

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

	commissionCents := rate.CalculateCommission(installment.AmountCents)
	if commissionCents <= 0 {
		itemLogger.Warn().Int64("amount_cents", installment.AmountCents).Int("rate_bps", rate.RateBps).Msg("Commission is zero, skipping")
		item.Status = "skipped"
		batch.SkippedProofs++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	item.CommissionCents = commissionCents
	batch.TotalCommissionCents += commissionCents

	itemLogger.Info().Int64("amount_cents", installment.AmountCents).Int64("commission_cents", commissionCents).Int("rate_bps", rate.RateBps).Msg("Processing credit commission")

	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		debitReq := &walletusecase.DebitWalletRequest{
			ShopID:          installment.ShopID,
			AmountCents:     commissionCents,
			TransactionType: entity.WalletTxCommissionDebit,
			AllowNegative:   true,
		}

		resp, err := s.debitUC.Execute(shopCtx, debitReq)
		if err != nil {
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
		if err := s.installmentRepo.UpdateCreditCommissionStatus(shopCtx, installment.ID, entity.CreditCommissionCollected, commissionCents, &batchID); err != nil {
			itemLogger.Error().Err(err).Msg("Failed to update installment commission status")
		}

		if resp.IsNowNegative && s.freezeUC != nil {
			freezeDetails := fmt.Sprintf("Credit commission (installment #%d): %d FCFA", installment.InstallmentNumber, commissionCents/100)
			freezeReq := &walletusecase.FreezeAccountRequest{
				ShopID:         installment.ShopID,
				Reason:         entity.FreezeReasonNegativeBalance,
				AmountDueCents: -resp.BalanceAfterCents,
				Details:        &freezeDetails,
			}
			if _, err := s.freezeUC.Execute(shopCtx, freezeReq); err != nil {
				itemLogger.Warn().Err(err).Msg("Failed to freeze account")
			}
		}

		itemLogger.Info().Int64("commission_cents", commissionCents).Int64("balance_before", resp.PreviousBalance).Int64("balance_after", resp.BalanceAfterCents).Bool("account_frozen", resp.IsNowNegative).Msg("✅ Credit commission collected")
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	errMsg := lastErr.Error()
	item.Status = "failed"
	item.ErrorMessage = &errMsg
	batch.FailedCollections++
	batch.FailedCommissionCents += commissionCents

	batchID := batch.ID
	s.installmentRepo.UpdateCreditCommissionStatus(shopCtx, installment.ID, entity.CreditCommissionFailed, commissionCents, &batchID)

	itemLogger.Error().Err(lastErr).Int("max_retries", s.maxRetries).Msg("❌ Credit commission collection failed")
	s.batchRepo.CreateBatchItem(ctx, item)
}

func (s *CreditScheduler) finalizeBatch(ctx context.Context, batch *repository.CommissionBatch, startTime time.Time) {
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
