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

		// 🛡️ CORRECTION CRITIQUE P1 : Utiliser le nouveau statut qui NE déclenche PAS de reverse automatique
		reason := "Stuck in pending/processing after wallet debit. Provider status unknown. Manual verification required."
		if err := w.MarkReconciliationRequired(reason); err != nil {
			log.Error().Err(err).Str("withdrawal_id", w.ID.String()).Msg("Failed to mark withdrawal as reconciliation_required")
			continue
		}

		if err := s.withdrawalRepo.Update(ctx, w); err != nil {
			log.Error().Err(err).Str("withdrawal_id", w.ID.String()).Msg("Failed to update withdrawal status to reconciliation_required")
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
