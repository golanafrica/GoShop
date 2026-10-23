package scheduler

import (
	"context"
	"encoding/json"
	"time"

	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

// NotificationRetryScheduler gère la réexpédition des notifications en échec
type NotificationRetryScheduler struct {
	failedNotifRepo repository.FailedNotificationRepository
	notifService    service.NotificationService
	logger          zerolog.Logger
	maxPerRun       int
}

// NewNotificationRetryScheduler crée une nouvelle instance du scheduler de retry
func NewNotificationRetryScheduler(
	failedNotifRepo repository.FailedNotificationRepository,
	notifService service.NotificationService,
	logger zerolog.Logger,
) *NotificationRetryScheduler {
	return &NotificationRetryScheduler{
		failedNotifRepo: failedNotifRepo,
		notifService:    notifService,
		logger:          logger.With().Str("scheduler", "notification_retry").Logger(),
		maxPerRun:       50,
	}
}

// Run exécute le job de retry des notifications
func (s *NotificationRetryScheduler) Run(ctx context.Context) error {
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting notification retry job")

	// 1. Récupérer ET verrouiller atomiquement les notifications (lease de 1h)
	notifications, err := s.failedNotifRepo.FetchAndClaimRetryableNotifications(ctx, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find and claim retryable notifications")
		return err
	}

	log.Info().Int("count", len(notifications)).Msg("Found and claimed retryable notifications")

	for _, notif := range notifications {
		var req service.NotificationRequest
		if err := json.Unmarshal(notif.Payload, &req); err != nil {
			s.logger.Error().Err(err).Str("notification_id", notif.ID).Msg("Failed to unmarshal payload, marking as permanent failure")
			_ = s.failedNotifRepo.MarkAsPermanentFailure(ctx, notif.ID)
			continue
		}

		// 2. Tenter d'envoyer la notification
		err := s.notifService.SendNotification(ctx, &req)

		if err != nil {
			// 3a. Échec : on met à jour le compteur et on recalcule le prochain retry (backoff)
			// Cela écrase le "lease" de 1 heure par le vrai délai de backoff
			_ = s.failedNotifRepo.IncrementRetryCount(ctx, notif.ID, err.Error())
			s.logger.Warn().Err(err).Str("notification_id", notif.ID).Msg("Notification retry failed, will retry later")
		} else {
			// 3b. Succès : on supprime l'entrée de la file d'attente
			_ = s.failedNotifRepo.MarkAsSuccess(ctx, notif.ID)
			s.logger.Info().Str("notification_id", notif.ID).Msg("Notification retry dispatched successfully")
		}
	}

	log.Info().Msg("Notification retry job completed")
	return nil
}
