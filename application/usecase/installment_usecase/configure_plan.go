package installmentusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/rs/zerolog/log"
)

type ConfigureInstallmentPlanUsecase struct {
	planRepo        repository.InstallmentPlanRepository
	productRepo     repository.ProductRepository
	deliveryZoneSvc service.DeliveryZoneService // 🆕 Injecté
}

func NewConfigureInstallmentPlanUsecase(
	planRepo repository.InstallmentPlanRepository,
	productRepo repository.ProductRepository,
	deliveryZoneSvc service.DeliveryZoneService, // 🆕 Ajouté
) *ConfigureInstallmentPlanUsecase {
	return &ConfigureInstallmentPlanUsecase{
		planRepo:        planRepo,
		productRepo:     productRepo,
		deliveryZoneSvc: deliveryZoneSvc,
	}
}

type ConfigurePlanRequest struct {
	ProductID        string
	NbTranches       int
	DelaiJours       int
	DeliveryZoneCode string // 🆕 Code de la zone (ex: "BF-OUAGA-URB")
}

func (uc *ConfigureInstallmentPlanUsecase) Execute(ctx context.Context, req ConfigurePlanRequest) (*entity.InstallmentPlan, error) {
	log.Info().Str("product_id", req.ProductID).Msg("Configuring installment plan")

	_, err := uc.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("produit introuvable ou accès non autorisé: %w", err)
	}

	// 🆕 Récupérer le délai dynamique depuis le service de zone
	releaseDelayDays := 7 // Valeur par défaut de secours
	if req.DeliveryZoneCode != "" {
		delay, err := uc.deliveryZoneSvc.GetInstallmentReleaseDelay(ctx, req.DeliveryZoneCode)
		if err == nil {
			releaseDelayDays = delay
		} else {
			log.Warn().Err(err).Str("zone_code", req.DeliveryZoneCode).Msg("Fallback to default release delay")
		}
	}

	plan, err := entity.NewInstallmentPlan(req.ProductID, "", req.NbTranches, req.DelaiJours)
	if err != nil {
		return nil, fmt.Errorf("configuration invalide: %w", err)
	}

	// 🆕 Assigner les nouveaux champs
	plan.DeliveryZoneID = &req.DeliveryZoneCode
	plan.InstallmentReleaseDelayDays = releaseDelayDays

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

	log.Info().Str("plan_id", plan.ID).Int("release_delay_days", releaseDelayDays).Msg("Installment plan configured successfully")
	return plan, nil
}
