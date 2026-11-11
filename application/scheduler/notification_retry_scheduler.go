package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/rs/zerolog"
)

type NotificationRetryScheduler struct {
	failedNotifRepo repository.FailedNotificationRepository
	notifService    service.NotificationService
	logger          zerolog.Logger
	maxPerRun       int
}

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

func (s *NotificationRetryScheduler) Run(ctx context.Context) error {
	log := s.logger.With().Time("started_at", time.Now().UTC()).Logger()
	log.Info().Msg("Starting notification retry job")

	notifications, err := s.failedNotifRepo.FetchAndClaimRetryableNotifications(ctx, s.maxPerRun)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find and claim retryable notifications")
		return fmt.Errorf("failed to find and claim retryable notifications: %w", err)
	}

	log.Info().Int("count", len(notifications)).Msg("Found and claimed retryable notifications")

	var jobErrs []error

	for _, notif := range notifications {
		var req service.NotificationRequest
		if err := json.Unmarshal(notif.Payload, &req); err != nil {
			s.logger.Error().Err(err).Str("notification_id", notif.ID).Msg("Failed to unmarshal payload")

			if markErr := s.failedNotifRepo.MarkAsPermanentFailure(ctx, notif.ID); markErr != nil {
				errMsg := fmt.Sprintf("failed to mark malformed notification %s as permanent failure: %v", notif.ID, markErr)
				s.logger.Error().Err(markErr).Str("notification_id", notif.ID).Msg("CRITICAL: " + errMsg)
				jobErrs = append(jobErrs, errors.New(errMsg))
			}
			continue
		}

		err := s.notifService.SendNotification(ctx, &req)

		if err != nil {
			if retryErr := s.failedNotifRepo.IncrementRetryCount(ctx, notif.ID, err.Error()); retryErr != nil {
				errMsg := fmt.Sprintf("failed to increment retry count for %s: %v", notif.ID, retryErr)
				s.logger.Error().Err(retryErr).Str("notification_id", notif.ID).Msg("CRITICAL: " + errMsg)
				jobErrs = append(jobErrs, errors.New(errMsg))
			} else {
				s.logger.Warn().Err(err).Str("notification_id", notif.ID).Msg("Notification retry failed, will retry later")
			}
		} else {
			if successErr := s.failedNotifRepo.MarkAsSuccess(ctx, notif.ID); successErr != nil {
				errMsg := fmt.Sprintf("failed to mark notification %s as success: %v", notif.ID, successErr)
				s.logger.Error().Err(successErr).Str("notification_id", notif.ID).Msg("CRITICAL: " + errMsg)
				jobErrs = append(jobErrs, errors.New(errMsg))
			} else {
				s.logger.Info().Str("notification_id", notif.ID).Msg("Notification retry dispatched successfully")
			}
		}
	}

	if len(jobErrs) > 0 {
		return fmt.Errorf("notification retry job completed with %d persistence errors", len(jobErrs))
	}

	log.Info().Msg("Notification retry job completed")
	return nil
}
