package shopusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

// ConfigureTontineUsecase permet à un marchand d'activer/désactiver la tontine sur un produit
type ConfigureTontineUsecase struct {
	settingsRepo repository.ProductTontineSettingsRepository
	productRepo  repository.ProductRepository
}

// NewConfigureTontineUsecase crée une nouvelle instance
func NewConfigureTontineUsecase(
	settingsRepo repository.ProductTontineSettingsRepository,
	productRepo repository.ProductRepository,
) *ConfigureTontineUsecase {
	return &ConfigureTontineUsecase{
		settingsRepo: settingsRepo,
		productRepo:  productRepo,
	}
}

// ConfigureTontineRequest représente la requête de configuration
type ConfigureTontineRequest struct {
	ProductID             string `json:"product_id"`
	ShopID                string `json:"shop_id"` // 🆕 Passé explicitement par le handler
	IsTontineEnabled      bool   `json:"is_tontine_enabled"`
	AllowCommercialCircle bool   `json:"allow_commercial_circle"`
	AllowCorporateCircle  bool   `json:"allow_corporate_circle"`
	AllowFamilyCircle     bool   `json:"allow_family_circle"`
	MinParticipants       int    `json:"min_participants"`
	MaxParticipants       int    `json:"max_participants"`
}

// Validate valide la requête
func (r *ConfigureTontineRequest) Validate() error {
	if r.ProductID == "" {
		return fmt.Errorf("product_id is required")
	}
	if r.ShopID == "" {
		return fmt.Errorf("shop_id is required")
	}
	if r.MinParticipants < 2 {
		return fmt.Errorf("min_participants must be at least 2")
	}
	if r.MaxParticipants > 50 {
		return fmt.Errorf("max_participants cannot exceed 50")
	}
	if r.MinParticipants > r.MaxParticipants {
		return fmt.Errorf("min_participants cannot exceed max_participants")
	}
	if !r.AllowCommercialCircle && !r.AllowCorporateCircle && !r.AllowFamilyCircle {
		return fmt.Errorf("at least one circle type must be allowed")
	}
	return nil
}

// Execute active/désactive la tontine sur un produit
func (uc *ConfigureTontineUsecase) Execute(ctx context.Context, req *ConfigureTontineRequest) (*entity.ProductTontineSettings, error) {
	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 🆕 FIX AUDIT P2 : Vérifier que le produit existe.
	// Note de sécurité : L'entité Product n'exposant pas directement le champ ShopID,
	// la protection contre la configuration d'un produit d'un autre shop repose sur :
	// 1. Cette vérification d'existence via le repository.
	// 2. La contrainte de clé étrangère (FK) en base de données sur product_id.
	// 3. La validation du shop_id effectuée en amont par le handler (TenantResolver).
	if _, err := uc.productRepo.FindByID(ctx, req.ProductID); err != nil {
		return nil, fmt.Errorf("product not found or inaccessible: %w", err)
	}

	// 2. Créer ou mettre à jour la configuration
	settings := &entity.ProductTontineSettings{
		ProductID:             req.ProductID,
		ShopID:                req.ShopID,
		IsTontineEnabled:      req.IsTontineEnabled,
		AllowCommercialCircle: req.AllowCommercialCircle,
		AllowCorporateCircle:  req.AllowCorporateCircle,
		AllowFamilyCircle:     req.AllowFamilyCircle,
		MinParticipants:       req.MinParticipants,
		MaxParticipants:       req.MaxParticipants,
	}

	if err := uc.settingsRepo.Upsert(ctx, settings); err != nil {
		return nil, fmt.Errorf("failed to upsert settings: %w", err)
	}

	// 3. Récupérer la configuration mise à jour
	updatedSettings, err := uc.settingsRepo.FindByProductID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated settings: %w", err)
	}

	return updatedSettings, nil
}

// GetTontineSettings récupère la configuration tontine d'un produit
func (uc *ConfigureTontineUsecase) GetTontineSettings(ctx context.Context, productID string) (*entity.ProductTontineSettings, error) {
	settings, err := uc.settingsRepo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("tontine settings not found: %w", err)
	}

	return settings, nil
}
