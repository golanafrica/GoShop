package codusecase

import (
	"context"
	"fmt"

	"Goshop/application/dto/cod_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

// GetAdminCODDashboardUsecase récupère les statistiques globales des commissions COD pour les admins
type GetAdminCODDashboardUsecase struct {
	codProofRepo repository.CODProofRepository
}

// NewGetAdminCODDashboardUsecase crée une nouvelle instance
func NewGetAdminCODDashboardUsecase(codProofRepo repository.CODProofRepository) *GetAdminCODDashboardUsecase {
	return &GetAdminCODDashboardUsecase{
		codProofRepo: codProofRepo,
	}
}

// Execute agrège toutes les données nécessaires au dashboard admin
func (uc *GetAdminCODDashboardUsecase) Execute(ctx context.Context) (*cod_dto.CODCommissionDashboardResponse, error) {
	// 1. Récupérer les sommes globales (méthodes déjà existantes dans ton repo !)
	pendingCents, err := uc.codProofRepo.SumTotalCommissionPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending commissions: %w", err)
	}

	dueCents, err := uc.codProofRepo.SumTotalCommissionDue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get due commissions: %w", err)
	}

	collectedCents, err := uc.codProofRepo.SumTotalCommissionCollected(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collected commissions: %w", err)
	}

	// 2. Récupérer les preuves "dues" (priorité haute pour l'admin)
	proofs, err := uc.codProofRepo.FindCommissionDue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get due proofs: %w", err)
	}

	// Si pas assez de preuves "dues", on complète avec les preuves "confirmed" (en attente de collecte)
	if len(proofs) < 20 {
		pendingProofs, err := uc.codProofRepo.FindByStatus(ctx, entity.CODProofConfirmed)
		if err == nil {
			for _, p := range pendingProofs {
				if len(proofs) >= 20 {
					break
				}
				// Éviter les doublons
				found := false
				for _, existing := range proofs {
					if existing.ID == p.ID {
						found = true
						break
					}
				}
				if !found {
					proofs = append(proofs, p)
				}
			}
		}
	}

	// 3. Mapper vers le DTO
	var recentProofs []cod_dto.RecentCODProof
	for _, p := range proofs {
		// Calculer une date de collecte estimée (création + 7 jours par défaut pour l'affichage)
		estimatedCollection := p.CreatedAt.AddDate(0, 0, 7)

		recentProofs = append(recentProofs, cod_dto.RecentCODProof{
			ID:                p.ID,
			OrderID:           p.OrderID,
			ShopID:            p.ShopID,
			CustomerID:        p.CustomerID,
			Status:            p.Status,
			CommissionStatus:  p.CommissionStatus,
			CommissionCents:   p.CommissionCents,
			CreatedAt:         p.CreatedAt,
			CommissionDueDate: &estimatedCollection,
		})
	}

	return &cod_dto.CODCommissionDashboardResponse{
		TotalPendingCents:   pendingCents,
		TotalDueCents:       dueCents,
		TotalCollectedCents: collectedCents,
		RecentProofs:        recentProofs,
	}, nil
}
