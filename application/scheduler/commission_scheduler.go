package scheduler

import (
	"context"
	"fmt"
	"time"

	codusecase "Goshop/application/usecase/cod_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// SCHEDULER DE COMMISSIONS
// ============================================================

// CommissionScheduler orchestre la collecte automatique des commissions
type CommissionScheduler struct {
	codRepo    repository.CODProofRepository
	batchRepo  repository.CommissionBatchRepository
	collectUC  *codusecase.CollectCommissionUsecase
	shopRepo   repository.ShopRepository
	batchSize  int
	maxRetries int
	logger     zerolog.Logger
}

// NewCommissionScheduler crée une nouvelle instance
func NewCommissionScheduler(
	codRepo repository.CODProofRepository,
	batchRepo repository.CommissionBatchRepository,
	collectUC *codusecase.CollectCommissionUsecase,
	shopRepo repository.ShopRepository,
	logger zerolog.Logger,
) *CommissionScheduler {
	return &CommissionScheduler{
		codRepo:    codRepo,
		batchRepo:  batchRepo,
		collectUC:  collectUC,
		shopRepo:   shopRepo,
		batchSize:  100,
		maxRetries: 3,
		logger:     logger.With().Str("component", "commission_scheduler").Logger(),
	}
}

// RunNightlyCollection exécute la collecte nocturne
func (s *CommissionScheduler) RunNightlyCollection(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().
		Time("started_at", startTime).
		Int("batch_size", s.batchSize).
		Msg("🌙 Starting nightly commission collection")

	// 1. Créer un nouveau batch
	batch := &repository.CommissionBatch{
		ID:          uuid.New().String(),
		StartedAt:   startTime,
		Status:      "running",
		TriggeredBy: "scheduler",
		CreatedAt:   startTime,
	}

	if err := s.batchRepo.CreateBatch(ctx, batch); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create batch")
		return fmt.Errorf("failed to create batch: %w", err)
	}

	// 2. Traiter les preuves par lots
	for {
		// Récupérer un lot de preuves en attente
		proofs, err := s.batchRepo.FindPendingProofsForCollection(ctx, s.batchSize)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to fetch pending proofs")
			batch.Status = "failed"
			errMsg := err.Error()
			batch.ErrorMessage = &errMsg
			s.batchRepo.UpdateBatch(ctx, batch)
			return fmt.Errorf("failed to fetch proofs: %w", err)
		}

		if len(proofs) == 0 {
			s.logger.Info().Msg("No more pending proofs to process")
			break
		}

		s.logger.Info().
			Int("batch_count", len(proofs)).
			Msg("Processing batch of proofs")

		// Traiter chaque preuve
		for _, proof := range proofs {
			s.processProof(ctx, batch, proof)
		}

		// Si on a traité moins que batchSize, on a fini
		if len(proofs) < s.batchSize {
			break
		}
	}

	// 3. Finaliser le batch
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	durationMs := int(duration.Milliseconds())

	batch.CompletedAt = &endTime
	batch.DurationMs = &durationMs
	batch.Status = "completed"

	if err := s.batchRepo.UpdateBatch(ctx, batch); err != nil {
		s.logger.Error().Err(err).Msg("Failed to update batch")
		return fmt.Errorf("failed to update batch: %w", err)
	}

	// 4. Logger le résumé
	s.logger.Info().
		Str("batch_id", batch.ID).
		Int("total_proofs", batch.TotalProofs).
		Int("successful", batch.SuccessfulCollections).
		Int("failed", batch.FailedCollections).
		Int("skipped", batch.SkippedProofs).
		Int64("collected_cents", batch.CollectedCommissionCents).
		Int64("failed_cents", batch.FailedCommissionCents).
		Int("duration_ms", durationMs).
		Msg("✅ Nightly commission collection completed")

	return nil
}

// processProof traite une preuve COD individuelle
func (s *CommissionScheduler) processProof(
	ctx context.Context,
	batch *repository.CommissionBatch,
	proof *entity.CODProof,
) {
	itemLogger := s.logger.With().
		Str("proof_id", proof.ID).
		Str("order_id", proof.OrderID).
		Str("shop_id", proof.ShopID).
		Logger()

	// Incrémenter le compteur
	batch.TotalProofs++
	batch.TotalCommissionCents += proof.CommissionCents

	// Créer l'item du batch
	item := &repository.CommissionBatchItem{
		ID:              uuid.New().String(),
		BatchID:         batch.ID,
		CODProofID:      proof.ID,
		OrderID:         proof.OrderID,
		ShopID:          proof.ShopID,
		CustomerID:      proof.CustomerID,
		CommissionCents: proof.CommissionCents,
		ProcessedAt:     time.Now(),
	}

	// Vérifier que la preuve est dans un état valide
	if !s.isProofReadyForCollection(proof) {
		itemLogger.Warn().
			Str("status", string(proof.Status)).
			Str("commission_status", string(proof.CommissionStatus)).
			Msg("Skipping proof - not ready for collection")
		item.Status = "skipped"
		batch.SkippedProofs++
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// ✅ CORRECTION : Convertir string → uuid.UUID
	shopUUID, err := uuid.Parse(proof.ShopID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Invalid shop UUID")
		item.Status = "failed"
		errMsg := fmt.Sprintf("invalid shop UUID: %s", proof.ShopID)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		batch.FailedCommissionCents += proof.CommissionCents
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// Récupérer le shop pour créer le contexte multi-tenant
	shop, err := s.shopRepo.FindByID(ctx, shopUUID)
	if err != nil {
		itemLogger.Error().Err(err).Msg("Failed to find shop")
		item.Status = "failed"
		errMsg := fmt.Sprintf("shop not found: %s", proof.ShopID)
		item.ErrorMessage = &errMsg
		batch.FailedCollections++
		batch.FailedCommissionCents += proof.CommissionCents
		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// Créer le contexte avec le tenant
	shopCtx := tenant.WithTenant(ctx, shop)

	// Tenter la collecte avec retries
	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		req := &codusecase.CollectCommissionRequest{
			OrderID:      proof.OrderID,
			ForceCollect: false,
			CollectedBy:  "scheduler-" + batch.ID[:8],
		}

		// ✅ APRÈS
		resp, err := s.collectUC.Execute(shopCtx, req)
		if err != nil {
			lastErr = err
			itemLogger.Warn().
				Err(err).
				Int("attempt", attempt).
				Msg("Collection attempt failed")

			if attempt < s.maxRetries {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			continue
		}

		// ✅ NOUVEAU : Vérifier que la commission a été réellement collectée
		if resp.CommissionStatus != entity.CODCommissionCollected {
			lastErr = fmt.Errorf("commission not collected: status=%s", resp.CommissionStatus)
			itemLogger.Warn().
				Str("status", string(resp.CommissionStatus)).
				Msg("Commission collection failed - insufficient balance")

			if attempt < s.maxRetries {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			continue
		}

		// Succès réel !
		item.Status = "success"
		item.WalletBalanceBefore = &resp.WalletBalanceBefore
		item.WalletBalanceAfter = &resp.WalletBalanceAfter
		item.AccountFrozen = resp.AccountFrozen

		batch.SuccessfulCollections++
		batch.CollectedCommissionCents += proof.CommissionCents

		itemLogger.Info().
			Int64("commission_cents", proof.CommissionCents).
			Int64("balance_before", resp.WalletBalanceBefore).
			Int64("balance_after", resp.WalletBalanceAfter).
			Bool("account_frozen", resp.AccountFrozen).
			Msg("✅ Commission collected successfully")

		s.batchRepo.CreateBatchItem(ctx, item)
		return
	}

	// Échec après tous les retries
	errMsg := lastErr.Error()
	item.Status = "failed"
	item.ErrorMessage = &errMsg

	batch.FailedCollections++
	batch.FailedCommissionCents += proof.CommissionCents

	itemLogger.Error().
		Err(lastErr).
		Int("max_retries", s.maxRetries).
		Msg("❌ Commission collection failed after all retries")

	s.batchRepo.CreateBatchItem(ctx, item)
}

// isProofReadyForCollection vérifie si une preuve est prête
func (s *CommissionScheduler) isProofReadyForCollection(proof *entity.CODProof) bool {
	// La preuve doit être confirmée (cohérente)
	if proof.Status != entity.CODProofConfirmed {
		return false
	}
	// La commission ne doit pas être déjà collectée
	if proof.CommissionStatus == entity.CODCommissionCollected {
		return false
	}
	// La commission doit être en attente ou due
	if proof.CommissionStatus != entity.CODCommissionPending &&
		proof.CommissionStatus != entity.CODCommissionDue {
		return false
	}
	return true
}
