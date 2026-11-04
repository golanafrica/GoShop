package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

const (
	freezeJobAdvisoryLockID = 99999
	configKeyEnabled        = "wallet.freeze_job.enabled"
	configKeyPolicy         = "wallet.freeze_job.policy"
	configKeyMaxPerRun      = "wallet.freeze_job.max_per_run"
	configKeyBatchSize      = "wallet.freeze_job.batch_size"

	defaultPageSize  = 100
	defaultMaxPerRun = 500
)

type WalletGraceExpiryScheduler struct {
	db           *sql.DB
	walletRepo   repository.MerchantWalletRepository
	notifService service.NotificationService
	logger       zerolog.Logger
}

func NewWalletGraceExpiryScheduler(
	db *sql.DB,
	walletRepo repository.MerchantWalletRepository,
	notifService service.NotificationService,
	logger zerolog.Logger,
) *WalletGraceExpiryScheduler {
	return &WalletGraceExpiryScheduler{
		db:           db,
		walletRepo:   walletRepo,
		notifService: notifService,
		logger:       logger.With().Str("scheduler", "wallet_grace_expiry").Logger(),
	}
}

// Run — exécution planifiée (cron). Respecte HARD_DISABLED + enabled DB.
func (s *WalletGraceExpiryScheduler) Run() {
	ctx := context.Background()
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("🔍 Starting wallet grace expiry job")

	if os.Getenv("FREEZE_JOB_HARD_DISABLED") == "true" {
		log.Warn().Msg("⛔ Job hard disabled via FREEZE_JOB_HARD_DISABLED=true, skipping")
		return
	}

	enabled, err := s.getConfigBool(ctx, configKeyEnabled)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read config from DB, skipping")
		return
	}
	if !enabled {
		log.Info().Msg("ℹ️  Job disabled in DB (wallet.freeze_job.enabled=false), skipping")
		return
	}

	policy, _ := s.getConfigString(ctx, configKeyPolicy)
	if policy == "" {
		policy = "alert_only"
	}
	maxPerRun, _ := s.getConfigInt(ctx, configKeyMaxPerRun)
	if maxPerRun <= 0 {
		maxPerRun = defaultMaxPerRun
	}
	pageSize, _ := s.getConfigInt(ctx, configKeyBatchSize)
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	// 🛡️ A07 FIX : Ajout de l'acteur "system_cron"
	runID, err := s.createJobRun(ctx, "scheduled", "live", "system_cron")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create job run record")
		return
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire DB connection for advisory lock")
		s.finishJobRun(ctx, runID, 0, 0, 0, err.Error())
		return
	}
	defer conn.Close()

	acquired, err := s.acquireAdvisoryLockOnConn(ctx, conn)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire advisory lock")
		s.finishJobRun(ctx, runID, 0, 0, 0, err.Error())
		return
	}
	if !acquired {
		log.Info().Msg("🔒 Another instance is running, skipping")
		s.finishJobRun(ctx, runID, 0, 0, 0, "another instance running")
		return
	}
	defer s.releaseAdvisoryLockOnConn(ctx, conn)

	s.executeJobRun(ctx, runID, "live", policy, maxPerRun, pageSize)
}

// 🛡️ A07 FIX : Retourne le runID et lance l'exécution en arrière-plan avec un contexte limité dans le temps.
func (s *WalletGraceExpiryScheduler) RunManual(ctx context.Context, mode string, actor string) (string, error) {
	log := s.logger.With().Str("mode", mode).Str("actor", actor).Logger()
	log.Info().Msg("🔧 Starting MANUAL wallet grace expiry job")

	if os.Getenv("FREEZE_JOB_HARD_DISABLED") == "true" {
		log.Warn().Msg("⛔ Job hard disabled via FREEZE_JOB_HARD_DISABLED=true, manual run aborted")
		return "", fmt.Errorf("job hard disabled")
	}

	runID, err := s.createJobRun(ctx, "manual", mode, actor)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create manual job run record")
		return "", fmt.Errorf("failed to create job run: %w", err)
	}

	// Lancement asynchrone avec un timeout de sécurité (15 min) pour éviter les fuites de goroutines
	workCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	go func() {
		defer cancel()

		policy, _ := s.getConfigString(workCtx, configKeyPolicy)
		if policy == "" {
			policy = "alert_only"
		}
		maxPerRun, _ := s.getConfigInt(workCtx, configKeyMaxPerRun)
		if maxPerRun <= 0 {
			maxPerRun = defaultMaxPerRun
		}
		pageSize, _ := s.getConfigInt(workCtx, configKeyBatchSize)
		if pageSize <= 0 {
			pageSize = defaultPageSize
		}

		s.executeJobRun(workCtx, runID, mode, policy, maxPerRun, pageSize)
	}()

	return runID, nil
}

// executeJobRun contient la logique réelle d'exécution (extraite pour être réutilisée par Run et RunManual)
func (s *WalletGraceExpiryScheduler) executeJobRun(ctx context.Context, runID string, mode string, policy string, maxPerRun int, pageSize int) {
	log := s.logger.With().Str("run_id", runID).Str("mode", mode).Logger()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire DB connection for advisory lock")
		s.finishJobRun(ctx, runID, 0, 0, 0, err.Error())
		return
	}
	defer conn.Close()

	acquired, err := s.acquireAdvisoryLockOnConn(ctx, conn)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire advisory lock")
		s.finishJobRun(ctx, runID, 0, 0, 0, err.Error())
		return
	}
	if !acquired {
		log.Warn().Msg("🔒 Another instance is running, run skipped")
		s.finishJobRun(ctx, runID, 0, 0, 0, "another instance running")
		return
	}
	defer s.releaseAdvisoryLockOnConn(ctx, conn)

	processed, skipped, scanned := 0, 0, 0
	offset := 0

	for processed < maxPerRun {
		wallets, err := s.walletRepo.FindGracePeriodExpiredPaginated(ctx, pageSize, offset)
		if err != nil {
			log.Error().Err(err).Msg("Failed to find expired wallets")
			s.finishJobRun(ctx, runID, scanned, processed, skipped, err.Error())
			return
		}
		if len(wallets) == 0 {
			break
		}

		for _, wallet := range wallets {
			if processed >= maxPerRun {
				break
			}
			scanned++

			if mode == "dry_run" {
				processed++
				log.Info().Str("shop_id", wallet.ShopID).Msg("🔍 [DRY RUN] Would process wallet")
				continue
			}

			alreadyProcessed, err := s.isAlreadyProcessed(ctx, wallet.ShopID, wallet.FrozenUntil)
			if err != nil {
				log.Error().Err(err).Str("shop_id", wallet.ShopID).Msg("Failed to check idempotence")
				skipped++
				continue
			}
			if alreadyProcessed {
				skipped++
				continue
			}

			if err := s.processExpiredWallet(ctx, runID, wallet, policy); err != nil {
				log.Error().Err(err).Str("shop_id", wallet.ShopID).Msg("Failed to process expired wallet")
				skipped++
				continue
			}
			processed++
		}

		offset += len(wallets)
		if len(wallets) < pageSize {
			break
		}
	}

	s.finishJobRun(ctx, runID, scanned, processed, skipped, "")
	log.Info().Int("processed", processed).Int("skipped", skipped).Msg("✅ Job completed")
}

func (s *WalletGraceExpiryScheduler) processExpiredWallet(ctx context.Context, runID string, wallet *entity.MerchantWallet, policy string) error {
	log := s.logger.With().Str("shop_id", wallet.ShopID).Logger()

	if err := s.insertJobAction(ctx, runID, wallet.ShopID, wallet.FrozenUntil, "grace_expired_detected", policy); err != nil {
		return fmt.Errorf("failed to insert job action: %w", err)
	}

	log.Warn().
		Interface("frozen_until", wallet.FrozenUntil).
		Int64("debt_cents", wallet.DebtCents).
		Str("policy", policy).
		Msg("🚨 Wallet grace period expired")

	if policy == "alert_only" {
		s.notifyAdmin(ctx, wallet)
	}

	return nil
}

func (s *WalletGraceExpiryScheduler) notifyAdmin(ctx context.Context, wallet *entity.MerchantWallet) {
	if s.notifService == nil {
		s.logger.Warn().Msg("Notification service not available, skipping admin notification")
		return
	}

	frozenUntilStr := "unknown"
	if wallet.FrozenUntil != nil {
		frozenUntilStr = wallet.FrozenUntil.Format("2006-01-02 15:04:05")
	}

	message := fmt.Sprintf(
		"🚨 Wallet grace period expired for shop %s\nFrozen until: %s\nDebt: %d cents\nAction required: Review and escalate if necessary",
		wallet.ShopID, frozenUntilStr, wallet.DebtCents,
	)

	req := &service.NotificationRequest{
		Type:   service.NotificationType("wallet_grace_expired"),
		ShopID: wallet.ShopID,
		Data: map[string]interface{}{
			"shop_id":      wallet.ShopID,
			"subject":      "🚨 Action Required: Wallet Grace Period Expired",
			"message":      message,
			"debt_cents":   wallet.DebtCents,
			"frozen_until": frozenUntilStr,
		},
	}

	if err := s.notifService.SendNotification(ctx, req); err != nil {
		s.logger.Error().Err(err).Str("shop_id", wallet.ShopID).Msg("Failed to send admin notification for expired grace period")
	} else {
		s.logger.Info().Str("shop_id", wallet.ShopID).Msg("📧 Admin notification sent successfully for expired grace period")
	}
}

func (s *WalletGraceExpiryScheduler) getConfigBool(ctx context.Context, key string) (bool, error) {
	query := `SELECT value FROM platform_settings WHERE key = $1`
	var raw []byte
	err := s.db.QueryRowContext(ctx, query, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func (s *WalletGraceExpiryScheduler) getConfigString(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM platform_settings WHERE key = $1`
	var raw []byte
	err := s.db.QueryRowContext(ctx, query, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return string(raw), nil
	}
	return str, nil
}

func (s *WalletGraceExpiryScheduler) getConfigInt(ctx context.Context, key string) (int, error) {
	query := `SELECT value FROM platform_settings WHERE key = $1`
	var raw []byte
	err := s.db.QueryRowContext(ctx, query, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var num int
	if err := json.Unmarshal(raw, &num); err != nil {
		return 0, err
	}
	return num, nil
}

// 🛡️ A07 FIX : Ajout du paramètre 'actor' pour la traçabilité.
// NOTE: Si ta table n'a pas encore la colonne 'triggered_by', exécute :
// ALTER TABLE wallet_freeze_job_runs ADD COLUMN triggered_by VARCHAR(255);
func (s *WalletGraceExpiryScheduler) createJobRun(ctx context.Context, runType, mode, actor string) (string, error) {
	query := `
		INSERT INTO wallet_freeze_job_runs (run_type, mode, triggered_by, started_at) 
		VALUES ($1, $2, $3, NOW()) 
		RETURNING id
	`
	var runID string
	err := s.db.QueryRowContext(ctx, query, runType, mode, actor).Scan(&runID)
	return runID, err
}

func (s *WalletGraceExpiryScheduler) finishJobRun(ctx context.Context, runID string, scanned, processed, skipped int, errorMsg string) {
	query := `
		UPDATE wallet_freeze_job_runs
		SET wallets_scanned = $1,
		    wallets_processed = $2,
		    wallets_skipped = $3,
		    finished_at = NOW(),
		    duration_ms = (EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000)::int,
		    error_message = NULLIF($4, '')
		WHERE id = $5
	`
	_, err := s.db.ExecContext(ctx, query, scanned, processed, skipped, errorMsg, runID)
	if err != nil {
		s.logger.Error().Err(err).Str("run_id", runID).Msg("Failed to finish job run")
	}
}

func (s *WalletGraceExpiryScheduler) isAlreadyProcessed(ctx context.Context, shopID string, frozenUntil *time.Time) (bool, error) {
	if frozenUntil == nil {
		return false, nil
	}
	query := `
		SELECT EXISTS(
			SELECT 1 FROM wallet_freeze_job_actions
			WHERE shop_id = $1 AND frozen_until = $2 AND action = 'grace_expired_detected'
		)
	`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, shopID, *frozenUntil).Scan(&exists)
	return exists, err
}

func (s *WalletGraceExpiryScheduler) insertJobAction(ctx context.Context, runID, shopID string, frozenUntil *time.Time, action, policy string) error {
	query := `
		INSERT INTO wallet_freeze_job_actions (run_id, shop_id, frozen_until, action, policy)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (shop_id, frozen_until, action)
		WHERE (action = 'grace_expired_detected')
		DO NOTHING
	`
	result, err := s.db.ExecContext(ctx, query, runID, shopID, frozenUntil, action, policy)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		s.logger.Info().Str("shop_id", shopID).Str("action", action).Msg("📝 Inserted action into audit table")
	} else {
		s.logger.Warn().Str("shop_id", shopID).Str("action", action).Msg("⏭️ Action skipped (already processed via idempotence)")
	}
	return nil
}

func (s *WalletGraceExpiryScheduler) acquireAdvisoryLockOnConn(ctx context.Context, conn *sql.Conn) (bool, error) {
	var acquired bool
	err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, freezeJobAdvisoryLockID).Scan(&acquired)
	return acquired, err
}

func (s *WalletGraceExpiryScheduler) releaseAdvisoryLockOnConn(ctx context.Context, conn *sql.Conn) {
	_, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, freezeJobAdvisoryLockID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to release advisory lock")
	}
}
