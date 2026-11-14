package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	compensationJobAdvisoryLockID = 99998
	defaultCompMaxPerRun          = 50
	defaultCompMaxAttempts        = 10
	defaultCompMinAgeSeconds      = 60
)

type CompensationPendingScheduler struct {
	db             *sql.DB
	withdrawalRepo repository.WithdrawalRepository
	creditWalletUC *walletusecase.CreditWalletUsecase
	notifService   service.NotificationService
	logger         zerolog.Logger

	maxPerRun     int
	maxAttempts   int
	minAgeSeconds int
}

func NewCompensationPendingScheduler(
	db *sql.DB,
	withdrawalRepo repository.WithdrawalRepository,
	creditWalletUC *walletusecase.CreditWalletUsecase,
	notifService service.NotificationService,
	logger zerolog.Logger,
) *CompensationPendingScheduler {
	return &CompensationPendingScheduler{
		db:             db,
		withdrawalRepo: withdrawalRepo,
		creditWalletUC: creditWalletUC,
		notifService:   notifService,
		logger:         logger.With().Str("scheduler", "compensation_pending").Logger(),
		maxPerRun:      defaultCompMaxPerRun,
		maxAttempts:    defaultCompMaxAttempts,
		minAgeSeconds:  defaultCompMinAgeSeconds,
	}
}

func (s *CompensationPendingScheduler) Run(ctx context.Context) error {
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting compensation_pending retry job")

	if os.Getenv("COMPENSATION_JOB_HARD_DISABLED") == "true" {
		log.Warn().Msg("Job hard disabled via COMPENSATION_JOB_HARD_DISABLED=true, skipping")
		return nil
	}
	if s.creditWalletUC == nil {
		log.Error().Msg("creditWalletUC not configured, aborting")
		return fmt.Errorf("creditWalletUC not configured")
	}
	if s.withdrawalRepo == nil {
		log.Error().Msg("withdrawalRepo not configured, aborting")
		return fmt.Errorf("withdrawalRepo not configured")
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire DB conn for advisory lock")
		return fmt.Errorf("failed to acquire DB conn: %w", err)
	}
	defer conn.Close()

	acquired, err := s.tryLock(ctx, conn)
	if err != nil {
		log.Error().Err(err).Msg("Advisory lock error")
		return fmt.Errorf("advisory lock error: %w", err)
	}
	if !acquired {
		log.Info().Msg("Another instance holds the lock, skipping")
		return nil
	}
	defer s.unlock(ctx, conn)

	items, err := s.withdrawalRepo.FindCompensationPending(ctx, s.maxPerRun, s.maxAttempts)
	if err != nil {
		log.Error().Err(err).Msg("FindCompensationPending failed")
		return fmt.Errorf("FindCompensationPending failed: %w", err)
	}

	log.Info().Int("count", len(items)).Msg("Loaded compensation_pending withdrawals")

	resolved, failed, skipped := 0, 0, 0
	var jobErrs []error

	for _, w := range items {
		if s.tooRecent(w) {
			skipped++
			continue
		}
		if err := s.processOne(ctx, w); err != nil {
			log.Error().
				Err(err).
				Str("withdrawal_id", w.ID.String()).
				Str("shop_id", w.ShopID.String()).
				Int("attempts", w.CompensationAttempts).
				Msg("Compensation retry failed")
			failed++
			jobErrs = append(jobErrs, fmt.Errorf("withdrawal %s: %w", w.ID, err))
			continue
		}
		resolved++
	}

	log.Info().
		Int("resolved", resolved).
		Int("failed", failed).
		Int("skipped", skipped).
		Msg("Compensation_pending job completed")

	if len(jobErrs) > 0 {
		return fmt.Errorf("compensation job completed with %d failures", len(jobErrs))
	}

	return nil
}

func (s *CompensationPendingScheduler) tooRecent(w *entity.Withdrawal) bool {
	if w.LastCompensationAt == nil {
		return false
	}
	return time.Since(*w.LastCompensationAt) < time.Duration(s.minAgeSeconds)*time.Second
}

func (s *CompensationPendingScheduler) processOne(ctx context.Context, w *entity.Withdrawal) error {
	log := s.logger.With().
		Str("withdrawal_id", w.ID.String()).
		Str("shop_id", w.ShopID.String()).
		Logger()

	// 🛡️ PR-F FIX : Contrôle du statut avant toute compensation pour éviter de compenser
	// un retrait qui aurait été résolu manuellement ou par webhook entre-temps.
	currentW, err := s.withdrawalRepo.FindByID(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch current withdrawal state: %w", err)
	}
	if currentW.Status != entity.WithdrawalStatusCompensationPending {
		log.Info().Str("current_status", string(currentW.Status)).Msg("Withdrawal status changed, skipping compensation")
		return nil
	}

	shopUUID, err := uuid.Parse(w.ShopID.String())
	if err != nil {
		return fmt.Errorf("invalid shop UUID in withdrawal: %w", err)
	}

	shop := &entity.Shop{ID: shopUUID}
	shopCtx := tenant.WithTenant(ctx, shop)

	refType := "withdrawal_reversal"
	refID := w.ID.String()
	desc := fmt.Sprintf("Compensation retry reverse for withdrawal %s", w.ID.String())

	_, err = s.creditWalletUC.Execute(shopCtx, &walletusecase.CreditWalletRequest{
		ShopID:          w.ShopID.String(),
		AmountCents:     w.AmountCents,
		TransactionType: entity.WalletTxDeposit,
		ReferenceType:   &refType,
		ReferenceID:     &refID,
		Description:     &desc,
		HoldAfterCredit: false,
	})

	if err != nil {
		w.RecordCompensationAttempt(err.Error())

		if updErr := s.withdrawalRepo.Update(ctx, w); updErr != nil {
			log.Error().Err(updErr).Msg("CRITICAL: Failed to persist compensation attempt")
			return fmt.Errorf("failed to persist compensation attempt for %s: %w", w.ID, updErr)
		}

		if w.CompensationAttempts >= s.maxAttempts {
			// 🛡️ PR-F FIX : Retourner une erreur si l'alerte échoue, pour permettre au job
			// de réessayer l'alerte au prochain run (le statut reste 'compensation_pending').
			if alertErr := s.alertAdmin(ctx, w, err); alertErr != nil {
				log.Error().Err(alertErr).Msg("CRITICAL: Failed to alert admin after max attempts. Will retry on next job run.")
				return fmt.Errorf("failed to alert admin after max attempts: %w", alertErr)
			}
		}
		return fmt.Errorf("credit reverse failed: %w", err)
	}

	// 🛡️ PR-F FIX : Mise à jour atomique pour éviter d'écraser un statut 'success'
	// qui aurait été défini par un webhook pendant l'exécution du crédit wallet.
	msg := "compensation resolved by retry job (payout reverse applied)"
	updated, err := s.withdrawalRepo.MarkFailedIfCompensationPending(ctx, w.ID, msg)
	if err != nil {
		return fmt.Errorf("failed to check/update withdrawal status after compensation: %w", err)
	}

	if !updated {
		// Le statut n'était plus 'compensation_pending'. Il est probablement passé à 'success' via webhook.
		// Cela signifie un double paiement potentiel (webhook + compensation).
		log.Error().
			Str("withdrawal_id", w.ID.String()).
			Str("shop_id", w.ShopID.String()).
			Int64("amount_cents", w.AmountCents).
			Msg("CRITICAL: Withdrawal status changed DURING compensation. Potential double payment detected. Manual reconciliation required.")

		// On alerte l'admin immédiatement avec un type d'alerte spécifique
		if alertErr := s.alertAdminDoublePayment(ctx, w); alertErr != nil {
			log.Error().Err(alertErr).Msg("CRITICAL: Failed to alert admin about potential double payment")
		}

		// On ne force pas le statut à 'failed' car il est probablement 'success'.
		// On laisse le statut tel quel pour que l'admin investigue.
		return fmt.Errorf("withdrawal status changed during compensation, flagged for manual reconciliation")
	}

	log.Info().
		Int64("amount_cents", w.AmountCents).
		Int("attempts", w.CompensationAttempts).
		Msg("Compensation resolved - withdrawal marked failed, balance restored")
	return nil
}

// 🛡️ PR-F FIX : Retourne désormais une error pour permettre la réessai durable par le scheduler.
func (s *CompensationPendingScheduler) alertAdmin(ctx context.Context, w *entity.Withdrawal, lastErr error) error {
	if s.notifService == nil {
		return fmt.Errorf("notification service not configured, cannot alert admin")
	}

	s.logger.Error().
		Str("withdrawal_id", w.ID.String()).
		Str("shop_id", w.ShopID.String()).
		Int("attempts", w.CompensationAttempts).
		Err(lastErr).
		Msg("CRITICAL: compensation_pending max attempts reached - manual intervention required")

	req := &service.NotificationRequest{
		Type:   "admin_compensation_pending_exhausted",
		ShopID: w.ShopID.String(),
		Data: map[string]interface{}{
			"withdrawal_id":         w.ID.String(),
			"amount_cents":          w.AmountCents,
			"compensation_attempts": w.CompensationAttempts,
			"last_error":            lastErr.Error(),
		},
	}

	if sendErr := s.notifService.SendNotification(ctx, req); sendErr != nil {
		return fmt.Errorf("failed to send admin compensation alert: %w", sendErr)
	}

	s.logger.Info().Str("withdrawal_id", w.ID.String()).Msg("Admin compensation alert sent successfully")
	return nil
}

// 🆕 PR-F FIX : Alerte spécifique pour les cas de double paiement potentiel
func (s *CompensationPendingScheduler) alertAdminDoublePayment(ctx context.Context, w *entity.Withdrawal) error {
	if s.notifService == nil {
		return fmt.Errorf("notification service not configured")
	}

	req := &service.NotificationRequest{
		Type:   "admin_compensation_double_payment_risk",
		ShopID: w.ShopID.String(),
		Data: map[string]interface{}{
			"withdrawal_id": w.ID.String(),
			"amount_cents":  w.AmountCents,
			"message":       "CRITICAL: Withdrawal was compensated, but status changed during the process. Check if merchant was paid twice (webhook + compensation).",
		},
	}

	if sendErr := s.notifService.SendNotification(ctx, req); sendErr != nil {
		return fmt.Errorf("failed to send double payment risk alert: %w", sendErr)
	}

	s.logger.Info().Str("withdrawal_id", w.ID.String()).Msg("Admin double payment risk alert sent successfully")
	return nil
}

func (s *CompensationPendingScheduler) tryLock(ctx context.Context, conn *sql.Conn) (bool, error) {
	var acquired bool
	err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, compensationJobAdvisoryLockID).Scan(&acquired)
	return acquired, err
}

func (s *CompensationPendingScheduler) unlock(ctx context.Context, conn *sql.Conn) {
	_, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, compensationJobAdvisoryLockID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to release compensation advisory lock")
	}
}
