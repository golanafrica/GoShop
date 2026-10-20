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
			Msg("CRITICAL: Withdrawal is pending but wallet was debited >15 min ago. Flagging for manual reconciliation.")

		// 🛡️ Action sûre : Passer en compensation_pending pour alerter l'admin
		// et empêcher l'utilisateur de relancer un retrait pendant que l'admin vérifie le dashboard.
		reason := "Stuck in pending after wallet debit. Requires manual provider verification."
		w.MarkCompensationPending(reason)

		if err := s.withdrawalRepo.Update(ctx, w); err != nil {
			log.Error().Err(err).Str("withdrawal_id", w.ID.String()).Msg("Failed to update stuck withdrawal")
			continue
		}

		// Alerte Admin
		if s.notifService != nil {
			req := &service.NotificationRequest{
				Type:   "admin_withdrawal_stuck",
				ShopID: w.ShopID.String(),
				Data: map[string]interface{}{
					"withdrawal_id": w.ID.String(),
					"amount_cents":  w.AmountCents,
					"message":       "Withdrawal debited but stuck in pending. Check provider dashboard.",
				},
			}
			if sendErr := s.notifService.SendNotification(ctx, req); sendErr != nil {
				log.Error().Err(sendErr).Msg("Failed to send admin withdrawal stuck alert")
			}
		}
	}

	log.Info().Msg("Withdrawal reconciliation job completed")
	return nil
}