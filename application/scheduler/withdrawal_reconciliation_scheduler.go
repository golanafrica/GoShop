package scheduler

import (
	"context"
	"time"

	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

type WithdrawalReconciliationScheduler struct {
	withdrawalRepo repository.WithdrawalRepository
	notifService   service.NotificationService
	logger         zerolog.Logger
	maxPerRun      int
	minAgeMinutes  int
}

func NewWithdrawalReconciliationScheduler(
	withdrawalRepo repository.WithdrawalRepository,
	notifService service.NotificationService,
	logger zerolog.Logger,
) *WithdrawalReconciliationScheduler {
	return &WithdrawalReconciliationScheduler{
		withdrawalRepo: withdrawalRepo,
		notifService:   notifService,
		logger:         logger.With().Str("scheduler", "withdrawal_reconciliation").Logger(),
		maxPerRun:      50,
		minAgeMinutes:  15, // Attendre 15 min avant de considérer un retrait comme "coincé"
	}
}

func (s *WithdrawalReconciliationScheduler) Run(ctx context.Context) error {
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting withdrawal reconciliation job")

	stuckWithdrawals, err := s.withdrawalRepo.FindStuckPendingWithdrawals(ctx, s.maxPerRun, s.minAgeMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find stuck withdrawals")
		return err
	}

	log.Info().Int("count", len(stuckWithdrawals)).Msg("Found stuck pending withdrawals")

	for _, w := range stuckWithdrawals {
		log.Warn().
			Str("withdrawal_id", w.ID.String()).
			Str("shop_id", w.ShopID.String()).
			Msg("CRITICAL: Withdrawal is stuck. Flagging for manual reconciliation (NO AUTO REVERSE).")

		// 🛡️ Transition atomique : pending|processing → reconciliation_required
		// Ignore si le statut a déjà changé (webhook concurrent / autre job).
		// Aucun reverse automatique — vérification manuelle uniquement.
		reason := "stuck_pending_manual_review"
		ok, err := s.withdrawalRepo.TransitionToReconciliationRequired(ctx, w.ID, reason)
		if err != nil {
			log.Error().
				Err(err).
				Str("withdrawal_id", w.ID.String()).
				Msg("Failed to transition withdrawal to reconciliation_required")
			continue
		}
		if !ok {
			log.Info().
				Str("withdrawal_id", w.ID.String()).
				Msg("Skipped: status already changed (webhook or concurrent job)")
			continue
		}

		// Alerte Admin pour vérification manuelle du dashboard prestataire
		if s.notifService != nil {
			req := &service.NotificationRequest{
				Type:   "admin_withdrawal_reconciliation_required",
				ShopID: w.ShopID.String(),
				Data: map[string]interface{}{
					"withdrawal_id": w.ID.String(),
					"amount_cents":  w.AmountCents,
					"message":       "Withdrawal stuck. Check provider dashboard before taking any manual action.",
				},
			}
			if sendErr := s.notifService.SendNotification(ctx, req); sendErr != nil {
				log.Error().Err(sendErr).Msg("Failed to send admin reconciliation alert")
			}
		}
	}

	log.Info().Msg("Withdrawal reconciliation job completed")
	return nil
}
