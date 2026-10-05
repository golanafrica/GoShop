package installmentusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog/log"
)

type ConfigureInstallmentPlanUsecase struct {
	planRepo         repository.InstallmentPlanRepository
	productRepo      repository.ProductRepository
	deliveryZoneRepo repository.DeliveryZoneRepository // 🆕 Injecté pour récupérer l'UUID de la zone
}

func NewConfigureInstallmentPlanUsecase(
	planRepo repository.InstallmentPlanRepository,
	productRepo repository.ProductRepository,
	deliveryZoneRepo repository.DeliveryZoneRepository, // 🆕 Remplace deliveryZoneSvc
) *ConfigureInstallmentPlanUsecase {
	return &ConfigureInstallmentPlanUsecase{
		planRepo:         planRepo,
		productRepo:      productRepo,
		deliveryZoneRepo: deliveryZoneRepo,
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

	// 1. Récupérer le ShopID du contexte (Multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("contexte multi-tenant manquant: %w", err)
	}

	_, err = uc.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("produit introuvable ou accès non autorisé: %w", err)
	}

	// 2. Récupérer le délai dynamique et l'ID de la zone depuis le repository
	releaseDelayDays := 7 // Valeur par défaut de secours
	var deliveryZoneID *string = nil

	if req.DeliveryZoneCode != "" {
		zone, err := uc.deliveryZoneRepo.FindByCode(ctx, req.DeliveryZoneCode)
		if err == nil && zone != nil {
			deliveryZoneID = &zone.ID // 🆕 On stocke l'UUID, pas le code texte
			releaseDelayDays = zone.InstallmentReleaseDelayDays
		} else {
			log.Warn().Err(err).Str("zone_code", req.DeliveryZoneCode).Msg("Fallback to default release delay")
		}
	}

	// 🆕 CORRECTION CRITIQUE : Passer le vrai ShopID au lieu de ""
	plan, err := entity.NewInstallmentPlan(req.ProductID, shop.ID.String(), req.NbTranches, req.DelaiJours)
	if err != nil {
		return nil, fmt.Errorf("configuration invalide: %w", err)
	}

	// 🆕 Assigner l'ID de la zone (UUID) et le délai
	plan.DeliveryZoneID = deliveryZoneID
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
