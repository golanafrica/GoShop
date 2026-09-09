package scheduler

import (
	"context"
	"fmt"
	"time"

	installmentusecase "Goshop/application/usecase/installment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type InstallmentAutoReleaseScheduler struct {
	orderRepo       repository.OrderRepository
	installmentRepo repository.OrderInstallmentRepository
	disputeRepo     repository.DisputeRepository
	releaseEscrowUC *installmentusecase.ReleaseEscrowFundsUsecase
	notifService    service.NotificationService
	logger          zerolog.Logger
}

func NewInstallmentAutoReleaseScheduler(
	orderRepo repository.OrderRepository,
	installmentRepo repository.OrderInstallmentRepository,
	disputeRepo repository.DisputeRepository,
	releaseEscrowUC *installmentusecase.ReleaseEscrowFundsUsecase,
	notifService service.NotificationService,
	logger zerolog.Logger,
) *InstallmentAutoReleaseScheduler {
	return &InstallmentAutoReleaseScheduler{
		orderRepo:       orderRepo,
		installmentRepo: installmentRepo,
		disputeRepo:     disputeRepo,
		releaseEscrowUC: releaseEscrowUC,
		notifService:    notifService,
		logger:          logger,
	}
}

func (s *InstallmentAutoReleaseScheduler) RunAutoRelease(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().Time("started_at", startTime).Msg("🚀 Starting installment auto-release check")

	orders, err := s.orderRepo.FindAll(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch orders for installment auto-release")
		return err
	}

	releasedCount := 0
	notifiedCount := 0

	for _, order := range orders {
		// 🆕 Vérification robuste : on ne traite que les commandes qui ont des tranches
		installments, err := s.installmentRepo.GetByOrderID(ctx, order.ID)
		if err != nil || len(installments) == 0 {
			continue // Ce n'est pas une commande en tranches
		}

		allPaid := true
		for _, inst := range installments {
			if !inst.IsPaid() {
				allPaid = false
				break
			}
		}

		if !allPaid {
			continue
		}

		var releaseDate time.Time
		if order.DeliveredAt != nil {
			releaseDate = order.DeliveredAt.AddDate(0, 0, order.InstallmentReleaseDelayDays)
		} else {
			releaseDate = order.CreatedAt.AddDate(0, 0, 30) // Fallback de sécurité
		}

		now := time.Now().UTC()
		hoursUntilRelease := releaseDate.Sub(now).Hours()

		// Vérifier si on est dans la fenêtre de 24h avant libération
		if hoursUntilRelease > 0 && hoursUntilRelease <= 24 {
			orderUUID, parseErr := uuid.Parse(order.ID)
			if parseErr == nil {
				hasDispute, _ := s.disputeRepo.ExistsByOrderID(ctx, orderUUID)
				if !hasDispute {
					go s.notifyClientReleaseSoon(context.Background(), order, releaseDate)
					notifiedCount++
				}
			}
			continue
		}

		if now.Before(releaseDate) {
			continue
		}

		// Vérifier litige avant libération
		orderUUID, err := uuid.Parse(order.ID)
		if err != nil {
			s.logger.Warn().Err(err).Str("order_id", order.ID).Msg("Invalid order UUID, skipping")
			continue
		}

		hasDispute, err := s.disputeRepo.ExistsByOrderID(ctx, orderUUID)
		if err != nil {
			s.logger.Warn().Err(err).Str("order_id", order.ID).Msg("Failed to check dispute, skipping")
			continue
		}
		if hasDispute {
			s.logger.Info().Str("order_id", order.ID).Msg("⏭️ Auto-release skipped: active dispute")
			continue
		}

		// Libérer les fonds
		_, err = s.releaseEscrowUC.Execute(ctx, order.ID)
		if err != nil {
			s.logger.Error().Err(err).Str("order_id", order.ID).Msg("Failed to auto-release")
			continue
		}

		releasedCount++
		s.logger.Info().Str("order_id", order.ID).Msg("✅ Installment funds auto-released")
	}

	duration := time.Since(startTime)
	s.logger.Info().
		Int("released_count", releasedCount).
		Int("notified_count", notifiedCount).
		Int("duration_ms", int(duration.Milliseconds())).
		Msg("✅ Installment auto-release check completed")

	return nil
}

func (s *InstallmentAutoReleaseScheduler) notifyClientReleaseSoon(ctx context.Context, order *entity.Order, releaseDate time.Time) {
	if s.notifService == nil {
		return
	}

	subject := fmt.Sprintf("⏰ Rappel : Vérifiez votre commande #%s", order.ID[:8])
	message := fmt.Sprintf(
		"La libération des fonds de votre commande #%s est prévue le %s. Si vous avez un problème avec le produit, signalez-le avant cette date.",
		order.ID[:8], releaseDate.Format("02/01/2006 à 15h04"),
	)

	req := &service.NotificationRequest{
		Type:    "installment_release_soon",
		OrderID: order.ID,
		Data: map[string]interface{}{
			"subject": subject,
			"message": message,
		},
	}

	if err := s.notifService.SendNotification(ctx, req); err != nil {
		s.logger.Warn().Err(err).Str("order_id", order.ID).Msg("Failed to send installment release notification")
	} else {
		s.logger.Info().
			Str("order_id", order.ID).
			Str("customer_id", order.CustomerID).
			Msg("📧 Client notification sent successfully: release in 24h")
	}
}
