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
	Execute(ctx context.Context, paymentID string) (interface{}, error)
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

	// ============================================================
	// 🆕 C3 AUDIT : Niveau A - Gérer les paiements bloqués SANS provider_ref
	// ============================================================
	missingRefPayments, err := s.paymentRepo.FindStuckPaymentsMissingProviderRef(ctx, s.minAgeMinutes, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find stuck payments missing provider_ref")
	} else {
		for _, p := range missingRefPayments {
			log.Error().
				Str("payment_id", p.ID.String()).
				Str("status", string(p.Status)).
				Msg("CRITICAL: Stuck payment without provider_ref. Manual reconciliation required. DO NOT MarkFailed.")

			if p.Metadata == nil {
				p.Metadata = make(map[string]interface{})
			}
			p.Metadata["reconciliation_reason"] = "missing_provider_ref"
			p.Metadata["reconciliation_at"] = time.Now().UTC().Format(time.RFC3339)

			if updateErr := s.paymentRepo.Update(ctx, p); updateErr != nil {
				log.Error().Err(updateErr).Str("payment_id", p.ID.String()).Msg("Failed to update stuck payment metadata")
			}
		}
	}

	// ============================================================
	// Logique existante : Gérer les paiements bloqués AVEC provider_ref
	// ============================================================
	stuck, err := s.paymentRepo.FindStuckNonTerminalPayments(ctx, s.minAgeMinutes, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list stuck payments with provider_ref")
		return err
	}

	log.Info().Int("count", len(stuck)).Msg("Stuck non-terminal payments with provider_ref found")

	for _, p := range stuck {
		shop, err := s.shopRepo.FindByID(ctx, p.ShopID)
		if err != nil {
			log.Error().Err(err).Str("payment_id", p.ID.String()).Msg("Shop not found, skip")
			continue
		}
		shopCtx := tenant.WithTenant(ctx, shop)

		if _, err := s.statusChecker.Execute(shopCtx, p.ID.String()); err != nil {
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
