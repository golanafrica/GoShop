package scheduler

import (
	"context"
	"time"

	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// PaymentStatusChecker abstrait CheckPaymentStatusUsecase (évite import cycle).
type PaymentStatusChecker interface {
	Execute(ctx context.Context, paymentID string) error
}

type PaymentReconciliationScheduler struct {
	paymentRepo   repository.PaymentRepository
	shopRepo      repository.ShopRepository
	statusChecker PaymentStatusChecker
	logger        zerolog.Logger
	minAgeMinutes int
	maxPerRun     int
}

func NewPaymentReconciliationScheduler(
	paymentRepo repository.PaymentRepository,
	shopRepo repository.ShopRepository,
	statusChecker PaymentStatusChecker,
	logger zerolog.Logger,
) *PaymentReconciliationScheduler {
	return &PaymentReconciliationScheduler{
		paymentRepo:   paymentRepo,
		shopRepo:      shopRepo,
		statusChecker: statusChecker,
		logger:        logger.With().Str("scheduler", "payment_reconciliation").Logger(),
		minAgeMinutes: 5,
		maxPerRun:     50,
	}
}

func (s *PaymentReconciliationScheduler) Run(ctx context.Context) error {
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting ONE_STEP payment reconciliation")

	stuck, err := s.paymentRepo.FindStuckNonTerminalPayments(ctx, s.minAgeMinutes, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list stuck payments")
		return err
	}

	log.Info().Int("count", len(stuck)).Msg("Stuck non-terminal payments found")

	for _, p := range stuck {
		shop, err := s.shopRepo.FindByID(ctx, p.ShopID)
		if err != nil {
			log.Error().Err(err).Str("payment_id", p.ID.String()).Msg("Shop not found, skip")
			continue
		}
		shopCtx := tenant.WithTenant(ctx, shop)

		if err := s.statusChecker.Execute(shopCtx, p.ID.String()); err != nil {
			log.Error().
				Err(err).
				Str("payment_id", p.ID.String()).
				Str("status", string(p.Status)).
				Msg("Reconciliation failed for payment")
			continue
		}
		log.Info().Str("payment_id", p.ID.String()).Msg("Payment reconciled")
	}

	log.Info().Msg("ONE_STEP payment reconciliation completed")
	return nil
}
