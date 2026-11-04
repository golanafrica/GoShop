package scheduler

import (
	"context"
	"errors"
	"fmt"
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

	var reconciliationErrs []error

	// ============================================================
	// 🆕 C3 AUDIT (A08) : Gérer les paiements bloqués SANS provider_ref
	// ============================================================
	missingRefPayments, err := s.paymentRepo.FindStuckPaymentsMissingProviderRef(ctx, s.minAgeMinutes, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find stuck payments missing provider_ref")
		// 🛡️ A08 FIX : Fail-fast sur erreur de lecture DB
		return fmt.Errorf("failed to find stuck payments missing provider_ref: %w", err)
	}

	for _, p := range missingRefPayments {
		log.Error().
			Str("payment_id", p.ID.String()).
			Str("status", string(p.Status)).
			Msg("CRITICAL: Stuck payment without provider_ref. Manual reconciliation required. DO NOT MarkFailed.")

		if p.Metadata == nil {
			p.Metadata = make(map[string]interface{})
		}

		// 🛡️ A08 FIX : Ajouter un compteur de tentatives
		attempts := 1
		if prevAttempts, ok := p.Metadata["reconciliation_attempts"].(int); ok {
			attempts = prevAttempts + 1
		}

		p.Metadata["reconciliation_attempts"] = attempts
		p.Metadata["reconciliation_reason"] = "missing_provider_ref"
		p.Metadata["reconciliation_at"] = time.Now().UTC().Format(time.RFC3339)

		if updateErr := s.paymentRepo.Update(ctx, p); updateErr != nil {
			log.Error().Err(updateErr).Str("payment_id", p.ID.String()).Msg("Failed to update stuck payment metadata")
			// 🛡️ A08 FIX : Agréger l'erreur au lieu de l'ignorer
			reconciliationErrs = append(reconciliationErrs, fmt.Errorf("failed to update metadata for payment %s: %w", p.ID.String(), updateErr))
		}
	}

	// ============================================================
	// Logique existante : Gérer les paiements bloqués AVEC provider_ref
	// ============================================================
	stuck, err := s.paymentRepo.FindStuckNonTerminalPayments(ctx, s.minAgeMinutes, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list stuck payments with provider_ref")
		// 🛡️ A08 FIX : Fail-fast sur erreur de lecture DB
		return fmt.Errorf("failed to list stuck payments with provider_ref: %w", err)
	}

	log.Info().Int("count", len(stuck)).Msg("Stuck non-terminal payments with provider_ref found")

	for _, p := range stuck {
		shop, err := s.shopRepo.FindByID(ctx, p.ShopID)
		if err != nil {
			log.Error().Err(err).Str("payment_id", p.ID.String()).Msg("Shop not found, skip")
			reconciliationErrs = append(reconciliationErrs, fmt.Errorf("shop not found for payment %s: %w", p.ID.String(), err))
			continue
		}
		shopCtx := tenant.WithTenant(ctx, shop)

		if _, err := s.statusChecker.Execute(shopCtx, p.ID.String()); err != nil {
			log.Error().
				Err(err).
				Str("payment_id", p.ID.String()).
				Str("status", string(p.Status)).
				Msg("Reconciliation failed for payment")
			// 🛡️ A08 FIX : Agréger l'erreur pour que le job remonte un échec partiel
			reconciliationErrs = append(reconciliationErrs, fmt.Errorf("reconciliation failed for payment %s: %w", p.ID.String(), err))
			continue
		}
		log.Info().Str("payment_id", p.ID.String()).Msg("Payment reconciled successfully")
	}

	// 🛡️ A08 FIX : Retourner une erreur agrégée si des opérations ont échoué
	// Cela permet aux outils de monitoring (Prometheus, Datadog, K8s CronJob) de détecter l'échec.
	if len(reconciliationErrs) > 0 {
		log.Error().
			Int("error_count", len(reconciliationErrs)).
			Err(errors.Join(reconciliationErrs...)).
			Msg("Payment reconciliation completed with errors")
		return fmt.Errorf("payment reconciliation completed with %d errors: %w", len(reconciliationErrs), errors.Join(reconciliationErrs...))
	}

	log.Info().Msg("ONE_STEP payment reconciliation completed successfully")
	return nil
}
