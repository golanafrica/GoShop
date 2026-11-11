package scheduler

import (
	"context"
	"fmt"
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
		minAgeMinutes:  15,
	}
}

func (s *WithdrawalReconciliationScheduler) Run(ctx context.Context) error {
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting withdrawal reconciliation job")

	stuckWithdrawals, err := s.withdrawalRepo.FindStuckPendingWithdrawals(ctx, s.maxPerRun, s.minAgeMinutes)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find stuck withdrawals")
		return fmt.Errorf("failed to find stuck withdrawals: %w", err)
	}

	log.Info().Int("count", len(stuckWithdrawals)).Msg("Found stuck pending withdrawals")

	var jobErrs []error
	successCount := 0

	for _, w := range stuckWithdrawals {
		log.Warn().
			Str("withdrawal_id", w.ID.String()).
			Str("shop_id", w.ShopID.String()).
			Msg("CRITICAL: Withdrawal is stuck. Flagging for manual reconciliation (NO AUTO REVERSE).")

		reason := "stuck_pending_manual_review"
		ok, err := s.withdrawalRepo.TransitionToReconciliationRequired(ctx, w.ID, reason)
		if err != nil {
			log.Error().
				Err(err).
				Str("withdrawal_id", w.ID.String()).
				Msg("Failed to transition withdrawal to reconciliation_required")
			jobErrs = append(jobErrs, fmt.Errorf("withdrawal %s transition failed: %w", w.ID, err))
			continue
		}
		if !ok {
			log.Info().
				Str("withdrawal_id", w.ID.String()).
				Msg("Skipped: status already changed (webhook or concurrent job)")
			continue
		}
		successCount++

		// 🛡️ PR-E FIX : Gestion robuste des alertes admin avec reprise durable visible.
		if s.notifService == nil {
			log.Error().Str("withdrawal_id", w.ID.String()).Msg("CRITICAL: Notification service is nil. Admin will NOT be alerted.")
		} else {
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
				// La reprise durable est assurée par le notification_dispatcher qui enregistre
				// automatiquement les échecs dans la table 'failed_notifications' (outbox).
				log.Error().Err(sendErr).Str("withdrawal_id", w.ID.String()).Msg("Failed to send admin reconciliation alert. Durable retry will be handled by notification dispatcher outbox.")
			}
		}
	}

	log.Info().
		Int("processed", successCount).
		Int("errors", len(jobErrs)).
		Msg("Withdrawal reconciliation job completed")

	if len(jobErrs) > 0 {
		return fmt.Errorf("withdrawal reconciliation job completed with %d errors", len(jobErrs))
	}

	return nil
}
