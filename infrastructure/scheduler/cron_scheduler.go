package scheduler

import (
	"context"

	appscheduler "Goshop/application/scheduler"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

// CronScheduler gère les tâches planifiées avec cron
type CronScheduler struct {
	cron                        *cron.Cron
	commissionSched             *appscheduler.CommissionScheduler
	onlinePaymentSched          *appscheduler.OnlinePaymentScheduler
	tontineSched                *appscheduler.TontineScheduler
	escrowAutoReleaseSched      *appscheduler.EscrowAutoReleaseScheduler
	installmentAutoReleaseSched *appscheduler.InstallmentAutoReleaseScheduler // 🆕 Ajouté
	logger                      zerolog.Logger
	scheduleCron                string
	onlinePaymentSchedule       string
	tontineSchedule             string
	escrowAutoReleaseSchedule   string
	installmentSchedule         string // 🆕 Ajouté
}

// NewCronScheduler crée une nouvelle instance
func NewCronScheduler(
	commissionSched *appscheduler.CommissionScheduler,
	onlinePaymentSched *appscheduler.OnlinePaymentScheduler,
	tontineSched *appscheduler.TontineScheduler,
	escrowAutoReleaseSched *appscheduler.EscrowAutoReleaseScheduler,
	installmentAutoReleaseSched *appscheduler.InstallmentAutoReleaseScheduler, // 🆕 Ajouté
	logger zerolog.Logger,
	scheduleCron string,
	onlinePaymentSchedule string,
	tontineSchedule string,
	escrowAutoReleaseSchedule string,
	installmentSchedule string, // 🆕 Ajouté
) *CronScheduler {

	if scheduleCron == "" {
		scheduleCron = "0 2 * * *"
	}
	if onlinePaymentSchedule == "" {
		onlinePaymentSchedule = "0 */1 * * *"
	}
	if tontineSchedule == "" {
		tontineSchedule = "*/30 * * * *"
	}
	if escrowAutoReleaseSchedule == "" {
		escrowAutoReleaseSchedule = "0 */6 * * *"
	}
	if installmentSchedule == "" {
		installmentSchedule = "0 */6 * * *" // 🆕 Par défaut toutes les 6 heures
	}

	return &CronScheduler{
		cron:                        cron.New(),
		commissionSched:             commissionSched,
		onlinePaymentSched:          onlinePaymentSched,
		tontineSched:                tontineSched,
		escrowAutoReleaseSched:      escrowAutoReleaseSched,
		installmentAutoReleaseSched: installmentAutoReleaseSched, // 🆕 Ajouté
		logger:                      logger.With().Str("component", "cron_scheduler").Logger(),
		scheduleCron:                scheduleCron,
		onlinePaymentSchedule:       onlinePaymentSchedule,
		tontineSchedule:             tontineSchedule,
		escrowAutoReleaseSchedule:   escrowAutoReleaseSchedule,
		installmentSchedule:         installmentSchedule, // 🆕 Ajouté
	}
}

// Start démarre le scheduler
func (s *CronScheduler) Start() error {
	s.logger.Info().
		Str("cod_schedule", s.scheduleCron).
		Str("online_payment_schedule", s.onlinePaymentSchedule).
		Str("tontine_schedule", s.tontineSchedule).
		Str("escrow_auto_release_schedule", s.escrowAutoReleaseSchedule).
		Str("installment_schedule", s.installmentSchedule). // 🆕 Ajouté
		Msg("🕐 Starting cron scheduler")

	// 1. COD (tous les jours à 2h)
	_, err := s.cron.AddFunc(s.scheduleCron, func() {
		s.logger.Info().Msg("⏰ Cron trigger: COD commission collection")
		ctx := context.Background()
		if err := s.commissionSched.RunNightlyCollection(ctx); err != nil {
			s.logger.Error().Err(err).Msg("❌ COD commission collection failed")
		}
	})
	if err != nil {
		return err
	}

	// 2. Online Payment (toutes les heures)
	if s.onlinePaymentSched != nil {
		_, err = s.cron.AddFunc(s.onlinePaymentSchedule, func() {
			s.logger.Info().Msg("⏰ Cron trigger: Online payment commission collection")
			ctx := context.Background()
			if err := s.onlinePaymentSched.RunCollection(ctx); err != nil {
				s.logger.Error().Err(err).Msg("❌ Online payment commission collection failed")
			}
		})
		if err != nil {
			return err
		}
	}

	// 3. Tontine (toutes les 30 min)
	if s.tontineSched != nil {
		_, err = s.cron.AddFunc(s.tontineSchedule, func() {
			s.logger.Info().Msg("⏰ Cron trigger: Tontine commission collection")
			ctx := context.Background()
			if err := s.tontineSched.RunCollection(ctx); err != nil {
				s.logger.Error().Err(err).Msg("❌ Tontine commission collection failed")
			}
		})
		if err != nil {
			return err
		}
	}

	// 4. Escrow Auto-Release (toutes les 6 heures)
	if s.escrowAutoReleaseSched != nil {
		_, err = s.cron.AddFunc(s.escrowAutoReleaseSchedule, func() {
			s.logger.Info().Msg("⏰ Cron trigger: Escrow auto-release")
			ctx := context.Background()
			if err := s.escrowAutoReleaseSched.RunAutoRelease(ctx); err != nil {
				s.logger.Error().Err(err).Msg("❌ Escrow auto-release failed")
			}
		})
		if err != nil {
			return err
		}
	}

	// 5. 🆕 Installment Auto-Release (toutes les 6 heures)
	if s.installmentAutoReleaseSched != nil {
		_, err = s.cron.AddFunc(s.installmentSchedule, func() {
			s.logger.Info().Msg("⏰ Cron trigger: Installment auto-release")
			ctx := context.Background()
			if err := s.installmentAutoReleaseSched.RunAutoRelease(ctx); err != nil {
				s.logger.Error().Err(err).Msg("❌ Installment auto-release failed")
			}
		})
		if err != nil {
			return err
		}
	}

	s.cron.Start()
	s.logger.Info().Msg("✅ Cron scheduler started successfully")
	return nil
}

// Stop arrête proprement le scheduler
func (s *CronScheduler) Stop() {
	s.logger.Info().Msg("🛑 Stopping cron scheduler...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info().Msg("✅ Cron scheduler stopped")
}
