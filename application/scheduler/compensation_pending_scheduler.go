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
	compensationJobAdvisoryLockID = 99998 // distinct du freeze job (99999)
	defaultCompMaxPerRun          = 50
	defaultCompMaxAttempts        = 10
	defaultCompMinAgeSeconds      = 60
)

// CompensationPendingScheduler retente le reverse des retraits en compensation_pending.
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

// Run - point d'entree cron.
func (s *CompensationPendingScheduler) Run() {
	ctx := context.Background()
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting compensation_pending retry job")

	if os.Getenv("COMPENSATION_JOB_HARD_DISABLED") == "true" {
		log.Warn().Msg("Job hard disabled via COMPENSATION_JOB_HARD_DISABLED=true, skipping")
		return
	}
	if s.creditWalletUC == nil {
		log.Error().Msg("creditWalletUC not configured, aborting")
		return
	}
	if s.withdrawalRepo == nil {
		log.Error().Msg("withdrawalRepo not configured, aborting")
		return
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire DB conn for advisory lock")
		return
	}
	defer conn.Close()

	acquired, err := s.tryLock(ctx, conn)
	if err != nil {
		log.Error().Err(err).Msg("Advisory lock error")
		return
	}
	if !acquired {
		log.Info().Msg("Another instance holds the lock, skipping")
		return
	}
	defer s.unlock(ctx, conn)

	items, err := s.withdrawalRepo.FindCompensationPending(ctx, s.maxPerRun, s.maxAttempts)
	if err != nil {
		log.Error().Err(err).Msg("FindCompensationPending failed")
		return
	}

	log.Info().Int("count", len(items)).Msg("Loaded compensation_pending withdrawals")

	resolved, failed, skipped := 0, 0, 0
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
			continue
		}
		resolved++
	}

	log.Info().
		Int("resolved", resolved).
		Int("failed", failed).
		Int("skipped", skipped).
		Msg("Compensation_pending job completed")
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

	// 🛡️ CORRECTION CRITIQUE : Le CreditWalletUsecase exige un contexte avec un tenant.
	// Nous devons reconstruire un contexte valide à partir du ShopID du retrait.
	shopUUID, err := uuid.Parse(w.ShopID.String())
	if err != nil {
		return fmt.Errorf("invalid shop UUID in withdrawal: %w", err)
	}

	// On crée un objet Shop minimal juste pour satisfaire le middleware de tenant
	shop := &entity.Shop{ID: shopUUID}
	shopCtx := tenant.WithTenant(ctx, shop)

	refType := "withdrawal_reversal"
	refID := w.ID.String()
	desc := fmt.Sprintf("Compensation retry reverse for withdrawal %s", w.ID.String())

	// 🛡️ On utilise shopCtx au lieu de ctx pour que le repository accepte la requête
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
		// Si la méthode RecordCompensationAttempt n'existe pas sur ton entité,
		// remplace ces 2 lignes par :
		// w.CompensationAttempts++
		// now := time.Now().UTC()
		// w.LastCompensationAt = &now
		// reason := fmt.Sprintf("Retry failed: %v", err)
		// w.FailureReason = &reason

		w.RecordCompensationAttempt(err.Error()) // (Si cette méthode existe dans ton entity/withdrawal.go)

		if updErr := s.withdrawalRepo.Update(ctx, w); updErr != nil {
			log.Error().Err(updErr).Msg("Failed to persist compensation attempt")
		}
		if w.CompensationAttempts >= s.maxAttempts {
			s.alertAdmin(ctx, w, err)
		}
		return fmt.Errorf("credit reverse: %w", err)
	}

	msg := "compensation resolved by retry job (payout reverse applied)"
	if markErr := w.MarkFailed(msg); markErr != nil {
		return fmt.Errorf("mark failed after reverse: %w", markErr)
	}
	if updErr := s.withdrawalRepo.Update(ctx, w); updErr != nil {
		return fmt.Errorf("update after resolve: %w", updErr)
	}

	log.Info().
		Int64("amount_cents", w.AmountCents).
		Int("attempts", w.CompensationAttempts).
		Msg("Compensation resolved - withdrawal marked failed, balance restored")
	return nil
}

func (s *CompensationPendingScheduler) alertAdmin(ctx context.Context, w *entity.Withdrawal, lastErr error) {
	s.logger.Error().
		Str("withdrawal_id", w.ID.String()).
		Str("shop_id", w.ShopID.String()).
		Int("attempts", w.CompensationAttempts).
		Err(lastErr).
		Msg("CRITICAL: compensation_pending max attempts reached - manual intervention required")

	if s.notifService == nil {
		return
	}

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
		s.logger.Error().Err(sendErr).Msg("Failed to send admin compensation alert")
	}
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
