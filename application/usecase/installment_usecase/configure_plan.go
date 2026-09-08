package installmentusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog/log"
)

type ConfigureInstallmentPlanUsecase struct {
	planRepo    repository.InstallmentPlanRepository
	productRepo repository.ProductRepository
}

func NewConfigureInstallmentPlanUsecase(
	planRepo repository.InstallmentPlanRepository,
	productRepo repository.ProductRepository,
) *ConfigureInstallmentPlanUsecase {
	return &ConfigureInstallmentPlanUsecase{
		planRepo:    planRepo,
		productRepo: productRepo,
	}
}

type ConfigurePlanRequest struct {
	ProductID  string
	NbTranches int
	DelaiJours int
}

func (uc *ConfigureInstallmentPlanUsecase) Execute(ctx context.Context, req ConfigurePlanRequest) (*entity.InstallmentPlan, error) {
	log.Info().Str("product_id", req.ProductID).Msg("Configuring installment plan")

	// 1. Validation que le produit existe.
	// La sécurité multi-tenant est gérée automatiquement par le repository via le context.
	// Si le produit n'appartient pas au shop du context, FindByID retournera une erreur "not found".
	_, err := uc.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("produit introuvable ou accès non autorisé: %w", err)
	}

	// 2. Création de l'entité.
	// Le ShopID sera injecté automatiquement par le repository lors du Create via le context.
	plan, err := entity.NewInstallmentPlan(req.ProductID, "", req.NbTranches, req.DelaiJours)
	if err != nil {
		return nil, fmt.Errorf("configuration invalide: %w", err)
	}

	// 3. Sauvegarde (Upsert)
	existingPlan, _ := uc.planRepo.GetByProductID(ctx, req.ProductID)
	if existingPlan != nil {
		plan.ID = existingPlan.ID
		err = uc.planRepo.Update(ctx, plan)
	} else {
		err = uc.planRepo.Create(ctx, plan)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to save installment plan")
		return nil, fmt.Errorf("échec de la sauvegarde du plan: %w", err)
	}

	log.Info().Str("plan_id", plan.ID).Msg("Installment plan configured successfully")
	return plan, nil
}
