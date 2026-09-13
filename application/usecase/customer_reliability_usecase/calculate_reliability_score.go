package customerreliabilityusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// CalculateReliabilityScoreUsecase calcule et met à jour le score de fiabilité d'un client
type CalculateReliabilityScoreUsecase struct {
	scoreRepo    repository.CustomerReliabilityScoreRepository
	customerRepo repository.CustomerRepositoryInterface
	logger       zerolog.Logger
}

// NewCalculateReliabilityScoreUsecase crée une nouvelle instance
func NewCalculateReliabilityScoreUsecase(
	scoreRepo repository.CustomerReliabilityScoreRepository,
	customerRepo repository.CustomerRepositoryInterface,
	logger zerolog.Logger,
) *CalculateReliabilityScoreUsecase {
	return &CalculateReliabilityScoreUsecase{
		scoreRepo:    scoreRepo,
		customerRepo: customerRepo,
		logger:       logger.With().Str("component", "calculate_reliability_score").Logger(),
	}
}

// Execute calcule le score de fiabilité pour un client spécifique
func (uc *CalculateReliabilityScoreUsecase) Execute(ctx context.Context, customerID string) (*entity.CustomerReliabilityScore, error) {
	uc.logger.Info().Str("customer_id", customerID).Msg("Starting reliability score calculation")

	// 1. Récupérer ou créer le score existant
	score, err := uc.scoreRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find reliability score: %w", err)
	}

	if score == nil {
		// Nouveau client : vérifier si KYC vérifié
		customer, err := uc.customerRepo.FindByCustomerID(ctx, customerID)
		if err != nil {
			return nil, fmt.Errorf("failed to find customer: %w", err)
		}

		if customer.KYCLevel == entity.KYCLevelVerified {
			score = entity.NewCustomerReliabilityScoreWithKYC(customerID)
		} else {
			score = entity.NewCustomerReliabilityScore(customerID)
		}
	} else {
		// Réinitialiser le score pour recalcul complet
		score.Score = entity.BaseScoreWithoutKYC
		score.Tier = entity.TierBronze
	}

	// 2. Bonus KYC (si pas déjà appliqué lors de la création)
	customer, err := uc.customerRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find customer: %w", err)
	}
	if customer.KYCLevel == entity.KYCLevelVerified && score.Score < entity.BaseScoreWithKYC {
		score.ApplyBonus(entity.BonusKYCVerified)
		uc.logger.Info().Int("new_score", score.Score).Msg("Applied KYC bonus")
	}

	// 3. Bonus commandes COD réussies (derniers 90 jours)
	// On vérifie si le client est dans la liste des clients ayant des COD réussis
	codCustomerIDs, err := uc.scoreRepo.GetCustomersWithSuccessfulCODOrders(ctx, 90)
	if err != nil {
		uc.logger.Warn().Err(err).Msg("Failed to get successful COD orders")
	} else {
		for _, id := range codCustomerIDs {
			if id == customerID {
				score.ApplyBonus(entity.BonusSuccessfulCODOrder)
				uc.logger.Info().Int("bonus", entity.BonusSuccessfulCODOrder).Msg("Applied COD success bonus")
				break // On applique le bonus une fois par exécution de cette requête
			}
		}
	}

	// 4. Bonus cycles tontine complétés à l'heure (derniers 180 jours)
	tontineCustomerIDs, err := uc.scoreRepo.GetCustomersWithCompletedTontineCycles(ctx, 180)
	if err != nil {
		uc.logger.Warn().Err(err).Msg("Failed to get completed tontine cycles")
	} else {
		for _, id := range tontineCustomerIDs {
			if id == customerID {
				score.ApplyBonus(entity.BonusTontineCycleOnTime)
				uc.logger.Info().Int("bonus", entity.BonusTontineCycleOnTime).Msg("Applied tontine bonus")
				break
			}
		}
	}

	// 5. Malus tranches en retard (+15 jours)
	overdueCustomerIDs, err := uc.scoreRepo.GetCustomersWithOverdueInstallments(ctx)
	if err != nil {
		uc.logger.Warn().Err(err).Msg("Failed to get overdue installments")
	} else {
		for _, id := range overdueCustomerIDs {
			if id == customerID {
				score.ApplyPenalty(entity.PenaltyLateInstallment)
				uc.logger.Warn().Int("penalty", entity.PenaltyLateInstallment).Msg("Applied late installment penalty")
				break
			}
		}
	}

	// 6. Sauvegarder le score mis à jour
	score.LastCalculatedAt = time.Now().UTC()
	err = uc.scoreRepo.Upsert(ctx, score)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert reliability score: %w", err)
	}

	uc.logger.Info().
		Str("customer_id", customerID).
		Int("final_score", score.Score).
		Str("tier", string(score.Tier)).
		Msg("✅ Reliability score calculated successfully")

	return score, nil
}

// CalculateAllScores calcule le score pour tous les clients (tâche cron)
func (uc *CalculateReliabilityScoreUsecase) CalculateAllScores(ctx context.Context) error {
	uc.logger.Info().Msg("Starting batch reliability score calculation for all customers")

	// Note: FindAllCustomers retourne tous les clients.
	// Pour des raisons de performance en prod, on pourrait ajouter une pagination ici.
	customers, err := uc.customerRepo.FindAllCustomers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all customers: %w", err)
	}

	successCount := 0
	errorCount := 0

	for _, customer := range customers {
		_, err := uc.Execute(ctx, customer.ID)
		if err != nil {
			uc.logger.Error().Err(err).Str("customer_id", customer.ID).Msg("Failed to calculate score")
			errorCount++
		} else {
			successCount++
		}
	}

	uc.logger.Info().
		Int("success_count", successCount).
		Int("error_count", errorCount).
		Msg("✅ Batch reliability score calculation completed")

	return nil
}
