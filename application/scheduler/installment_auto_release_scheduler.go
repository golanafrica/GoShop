package scheduler

import (
	"context"
	"time"

	installmentusecase "Goshop/application/usecase/installment_usecase"
	"Goshop/domain/repository"

	"github.com/google/uuid" // 🆕 Import ajouté pour uuid.Parse
	"github.com/rs/zerolog"
)

type InstallmentAutoReleaseScheduler struct {
	orderRepo       repository.OrderRepository
	installmentRepo repository.OrderInstallmentRepository
	disputeRepo     repository.DisputeRepository
	releaseEscrowUC *installmentusecase.ReleaseEscrowFundsUsecase
	logger          zerolog.Logger
}

func NewInstallmentAutoReleaseScheduler(
	orderRepo repository.OrderRepository,
	installmentRepo repository.OrderInstallmentRepository,
	disputeRepo repository.DisputeRepository,
	releaseEscrowUC *installmentusecase.ReleaseEscrowFundsUsecase,
	logger zerolog.Logger,
) *InstallmentAutoReleaseScheduler {
	return &InstallmentAutoReleaseScheduler{
		orderRepo:       orderRepo,
		installmentRepo: installmentRepo,
		disputeRepo:     disputeRepo,
		releaseEscrowUC: releaseEscrowUC,
		logger:          logger,
	}
}

// RunAutoRelease exécute le déblocage automatique des fonds pour les commandes en tranches
func (s *InstallmentAutoReleaseScheduler) RunAutoRelease(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info().Time("started_at", startTime).Msg("🚀 Starting installment auto-release check")

	// 1. Trouver toutes les commandes en tranches (payment_method = 'installment')
	// Note: Adapte avec ta méthode de liste avec filtre si nécessaire, ici on utilise FindAll
	orders, err := s.orderRepo.FindAll(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch orders for installment auto-release")
		return err
	}

	releasedCount := 0
	for _, order := range orders {
		if order.PaymentMethod != "installment" {
			continue
		}

		// 2. Vérifier si TOUTES les tranches sont payées
		installments, err := s.installmentRepo.GetByOrderID(ctx, order.ID)
		if err != nil || len(installments) == 0 {
			continue
		}

		allPaid := true
		for _, inst := range installments {
			if !inst.IsPaid() {
				allPaid = false
				break
			}
		}

		if !allPaid {
			continue // Pas encore toutes payées
		}

		// 3. Vérifier le délai dynamique
		// Si DeliveredAt est nil, on utilise CreatedAt + 30 jours (fallback de sécurité)
		var releaseDate time.Time
		if order.DeliveredAt != nil {
			releaseDate = order.DeliveredAt.AddDate(0, 0, order.InstallmentReleaseDelayDays)
		} else {
			releaseDate = order.CreatedAt.AddDate(0, 0, 30) // Fallback
		}

		if time.Now().UTC().Before(releaseDate) {
			continue // Délai pas encore écoulé
		}

		// 4. Vérifier l'absence de litige actif
		orderUUID, err := uuid.Parse(order.ID)
		if err != nil {
			s.logger.Warn().Err(err).Str("order_id", order.ID).Msg("Invalid order UUID, skipping")
			continue
		}

		hasDispute, err := s.disputeRepo.ExistsByOrderID(ctx, orderUUID)
		if err != nil {
			s.logger.Warn().Err(err).Str("order_id", order.ID).Msg("Failed to check dispute status, skipping")
			continue
		}
		if hasDispute {
			s.logger.Info().Str("order_id", order.ID).Msg("⏭️ Auto-release skipped: active dispute")
			continue
		}

		// 5. Libérer les fonds !
		_, err = s.releaseEscrowUC.Execute(ctx, order.ID)
		if err != nil {
			s.logger.Error().Err(err).Str("order_id", order.ID).Msg("Failed to auto-release installment funds")
			continue
		}

		releasedCount++
		s.logger.Info().Str("order_id", order.ID).Msg("✅ Installment funds auto-released successfully")
	}

	duration := time.Since(startTime)
	s.logger.Info().
		Int("released_count", releasedCount).
		Int("duration_ms", int(duration.Milliseconds())).
		Msg("✅ Installment auto-release check completed")

	return nil
}
