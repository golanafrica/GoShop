package scheduler

import (
	"context"

	appscheduler "Goshop/application/scheduler"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

// CronScheduler gère les tâches planifiées avec cron
type CronScheduler struct {
	cron                  *cron.Cron
	commissionSched       *appscheduler.CommissionScheduler
	onlinePaymentSched    *appscheduler.OnlinePaymentScheduler
	tontineSched          *appscheduler.TontineScheduler
	creditSched           *appscheduler.CreditScheduler // 🆕 v3.4.0
	logger                zerolog.Logger
	scheduleCron          string
	onlinePaymentSchedule string
	tontineSchedule       string
	creditSchedule        string // 🆕 v3.4.0
}

// NewCronScheduler crée une nouvelle instance
func NewCronScheduler(
	commissionSched *appscheduler.CommissionScheduler,
	onlinePaymentSched *appscheduler.OnlinePaymentScheduler,
	tontineSched *appscheduler.TontineScheduler,
	creditSched *appscheduler.CreditScheduler, // 🆕 v3.4.0
	logger zerolog.Logger,
	scheduleCron string,
	onlinePaymentSchedule string,
	tontineSchedule string,
	creditSchedule string, // 🆕 v3.4.0
) *CronScheduler {
	if scheduleCron == "" {
		scheduleCron = "0 2 * * *"
	}
	if onlinePaymentSchedule == "" {
		onlinePaymentSchedule = "0 */1 * * *"
	}
	if tontineSchedule == "" {
		tontineSchedule = "*/30 * * * *" // Toutes les 30 min
	}
	if creditSchedule == "" {
		creditSchedule = "0 3 * * *" // 🆕 v3.4.0 : Tous les jours à 3h du matin
	}

	return &CronScheduler{
		cron:                  cron.New(),
		commissionSched:       commissionSched,
		onlinePaymentSched:    onlinePaymentSched,
		tontineSched:          tontineSched,
		creditSched:           creditSched, // 🆕 v3.4.0
		logger:                logger.With().Str("component", "cron_scheduler").Logger(),
		scheduleCron:          scheduleCron,
		onlinePaymentSchedule: onlinePaymentSchedule,
		tontineSchedule:       tontineSchedule,
		creditSchedule:        creditSchedule, // 🆕 v3.4.0
	}
}

// Start démarre le scheduler
func (s *CronScheduler) Start() error {
	s.logger.Info().
		Str("cod_schedule", s.scheduleCron).
		Str("online_payment_schedule", s.onlinePaymentSchedule).
		Str("tontine_schedule", s.tontineSchedule).
		Str("credit_schedule", s.creditSchedule). // 🆕 v3.4.0
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

	// 🆕 4. Credit (tous les jours à 3h)
	if s.creditSched != nil {
		_, err = s.cron.AddFunc(s.creditSchedule, func() {
			s.logger.Info().Msg("⏰ Cron trigger: Credit commission collection")
			ctx := context.Background()
			if err := s.creditSched.RunCollection(ctx); err != nil {
				s.logger.Error().Err(err).Msg("❌ Credit commission collection failed")
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
