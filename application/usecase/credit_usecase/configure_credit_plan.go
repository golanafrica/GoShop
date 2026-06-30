package creditusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ============================================================
// CONFIGURE CREDIT PLAN USECASE
// ============================================================

// ConfigureCreditPlanRequest représente la requête pour configurer un plan de crédit
type ConfigureCreditPlanRequest struct {
	// Identifiants
	ProductID string `json:"product_id"`

	// Activation
	IsEnabled bool `json:"is_enabled"`

	// Configuration
	MinDownPaymentPercent int `json:"min_down_payment_percent"`
	MaxDurationMonths     int `json:"max_duration_months"`
	InterestRateBps       int `json:"interest_rate_bps"`
	PenaltyRateBps        int `json:"penalty_rate_bps"`

	// Score minimum requis
	MinCreditScore int `json:"min_credit_score"`
}

// ConfigureCreditPlanResponse représente la réponse après configuration
type ConfigureCreditPlanResponse struct {
	// Plan configuré
	ID                    string `json:"id"`
	ProductID             string `json:"product_id"`
	ShopID                string `json:"shop_id"`
	IsEnabled             bool   `json:"is_enabled"`
	MinDownPaymentPercent int    `json:"min_down_payment_percent"`
	MaxDurationMonths     int    `json:"max_duration_months"`
	InterestRateBps       int    `json:"interest_rate_bps"`
	PenaltyRateBps        int    `json:"penalty_rate_bps"`
	MinCreditScore        int    `json:"min_credit_score"`

	// Affichage lisible
	InterestRatePercent float64 `json:"interest_rate_percent"`
	PenaltyRatePercent  float64 `json:"penalty_rate_percent"`

	// Action effectuée
	Action string `json:"action"` // "created" ou "updated"
}

// Validate valide la requête
func (r *ConfigureCreditPlanRequest) Validate() error {
	if r.ProductID == "" {
		return fmt.Errorf("product_id is required")
	}
	if r.MinDownPaymentPercent < entity.MinDownPaymentPercent || r.MinDownPaymentPercent > entity.MaxDownPaymentPercent {
		return fmt.Errorf("min_down_payment_percent must be between %d and %d",
			entity.MinDownPaymentPercent, entity.MaxDownPaymentPercent)
	}
	if r.MaxDurationMonths < entity.MinDurationMonths || r.MaxDurationMonths > entity.MaxDurationMonths {
		return fmt.Errorf("max_duration_months must be between %d and %d",
			entity.MinDurationMonths, entity.MaxDurationMonths)
	}
	if r.InterestRateBps < entity.MinInterestRateBps || r.InterestRateBps > entity.MaxInterestRateBps {
		return fmt.Errorf("interest_rate_bps must be between %d and %d (max 15%%)",
			entity.MinInterestRateBps, entity.MaxInterestRateBps)
	}
	if r.PenaltyRateBps < entity.MinPenaltyRateBps || r.PenaltyRateBps > entity.MaxPenaltyRateBps {
		return fmt.Errorf("penalty_rate_bps must be between %d and %d",
			entity.MinPenaltyRateBps, entity.MaxPenaltyRateBps)
	}
	if r.MinCreditScore < entity.MinCreditScore || r.MinCreditScore > entity.MaxCreditScore {
		return fmt.Errorf("min_credit_score must be between %d and %d",
			entity.MinCreditScore, entity.MaxCreditScore)
	}
	return nil
}

// ConfigureCreditPlanUsecase permet à un marchand de configurer un plan de crédit
type ConfigureCreditPlanUsecase struct {
	planRepo    repository.CreditPlanRepository
	productRepo repository.ProductRepository
	walletRepo  repository.MerchantWalletRepository
	txManager   repository.TxManager
}

// NewConfigureCreditPlanUsecase crée une nouvelle instance
func NewConfigureCreditPlanUsecase(
	planRepo repository.CreditPlanRepository,
	productRepo repository.ProductRepository,
	walletRepo repository.MerchantWalletRepository,
	txManager repository.TxManager,
) *ConfigureCreditPlanUsecase {
	return &ConfigureCreditPlanUsecase{
		planRepo:    planRepo,
		productRepo: productRepo,
		walletRepo:  walletRepo,
		txManager:   txManager,
	}
}

// Execute configure le plan de crédit
func (uc *ConfigureCreditPlanUsecase) Execute(ctx context.Context, req *ConfigureCreditPlanRequest) (*ConfigureCreditPlanResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid configure credit plan request")
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Vérifier le multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Multi-tenant error")
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Vérifier que le produit existe (multi-tenant géré par le repository)
	_, err = uc.productRepo.WithTX(tx).FindByID(ctx, req.ProductID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find product")
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// 5. Vérifier si le plan existe déjà
	existingPlan, err := uc.planRepo.WithTX(tx).FindByProductID(ctx, req.ProductID)
	action := "created"

	var plan *entity.CreditPlan

	if err != nil {
		// Plan n'existe pas, le créer
		plan = entity.NewCreditPlan(req.ProductID, shopID)
		plan.IsEnabled = req.IsEnabled
		plan.MinDownPaymentPercent = req.MinDownPaymentPercent
		plan.MaxDurationMonths = req.MaxDurationMonths
		plan.InterestRateBps = req.InterestRateBps
		plan.PenaltyRateBps = req.PenaltyRateBps
		plan.MinCreditScore = req.MinCreditScore

		// Valider le plan
		if err := plan.Validate(); err != nil {
			logger.Error().Err(err).Msg("Invalid credit plan")
			return nil, fmt.Errorf("validation error: %w", err)
		}

		// Créer le plan
		if err := uc.planRepo.WithTX(tx).Create(ctx, plan); err != nil {
			logger.Error().Err(err).Msg("Failed to create credit plan")
			return nil, fmt.Errorf("failed to create credit plan: %w", err)
		}

		action = "created"

	} else {
		// Plan existe, le mettre à jour
		plan = existingPlan
		plan.IsEnabled = req.IsEnabled
		plan.MinDownPaymentPercent = req.MinDownPaymentPercent
		plan.MaxDurationMonths = req.MaxDurationMonths
		plan.InterestRateBps = req.InterestRateBps
		plan.PenaltyRateBps = req.PenaltyRateBps
		plan.MinCreditScore = req.MinCreditScore

		// Valider le plan
		if err := plan.Validate(); err != nil {
			logger.Error().Err(err).Msg("Invalid credit plan")
			return nil, fmt.Errorf("validation error: %w", err)
		}

		// Mettre à jour le plan
		if err := uc.planRepo.WithTX(tx).Update(ctx, plan); err != nil {
			logger.Error().Err(err).Msg("Failed to update credit plan")
			return nil, fmt.Errorf("failed to update credit plan: %w", err)
		}

		action = "updated"
	}

	// 6. Si le crédit est activé, s'assurer que le wallet existe
	if req.IsEnabled {
		wallet, err := uc.walletRepo.WithTX(tx).FindByShopID(ctx, shopID)
		if err != nil {
			// Wallet n'existe pas, le créer
			logger.Info().
				Str("shop_id", shopID).
				Msg("Creating wallet for shop enabling credit")

			wallet = entity.NewMerchantWallet(shopID)
			if err := uc.walletRepo.WithTX(tx).Create(ctx, wallet); err != nil {
				logger.Error().Err(err).Msg("Failed to create wallet")
				return nil, fmt.Errorf("failed to create wallet: %w", err)
			}
		}

		// Vérifier que le wallet n'est pas gelé
		if wallet.IsFrozen {
			logger.Warn().
				Str("shop_id", shopID).
				Msg("Shop wallet is frozen, credit may not work properly")
			// On continue quand même, le marchand peut activer le crédit
			// mais les collectes de commission échoueront
		}
	}

	// 7. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 8. Logger le succès
	logger.Info().
		Str("product_id", req.ProductID).
		Str("shop_id", shopID).
		Bool("is_enabled", req.IsEnabled).
		Int("interest_rate_bps", req.InterestRateBps).
		Int("max_duration_months", req.MaxDurationMonths).
		Str("action", action).
		Msg("Credit plan configured successfully")

	// 9. Construire la réponse
	return &ConfigureCreditPlanResponse{
		ID:                    plan.ID,
		ProductID:             plan.ProductID,
		ShopID:                plan.ShopID,
		IsEnabled:             plan.IsEnabled,
		MinDownPaymentPercent: plan.MinDownPaymentPercent,
		MaxDurationMonths:     plan.MaxDurationMonths,
		InterestRateBps:       plan.InterestRateBps,
		PenaltyRateBps:        plan.PenaltyRateBps,
		MinCreditScore:        plan.MinCreditScore,
		InterestRatePercent:   plan.InterestRatePercent(),
		PenaltyRatePercent:    float64(plan.PenaltyRateBps) / 100.0,
		Action:                action,
	}, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// EnableCreditPlan active le crédit pour un produit
func (uc *ConfigureCreditPlanUsecase) EnableCreditPlan(
	ctx context.Context,
	productID string,
	interestRateBps int,
	maxDurationMonths int,
) (*ConfigureCreditPlanResponse, error) {
	req := &ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             true,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     maxDurationMonths,
		InterestRateBps:       interestRateBps,
		PenaltyRateBps:        500, // 5% par défaut
		MinCreditScore:        300,
	}

	return uc.Execute(ctx, req)
}

// DisableCreditPlan désactive le crédit pour un produit
func (uc *ConfigureCreditPlanUsecase) DisableCreditPlan(
	ctx context.Context,
	productID string,
) (*ConfigureCreditPlanResponse, error) {
	// Récupérer le plan existant pour conserver la config
	plan, err := uc.planRepo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("credit plan not found: %w", err)
	}

	req := &ConfigureCreditPlanRequest{
		ProductID:             productID,
		IsEnabled:             false,
		MinDownPaymentPercent: plan.MinDownPaymentPercent,
		MaxDurationMonths:     plan.MaxDurationMonths,
		InterestRateBps:       plan.InterestRateBps,
		PenaltyRateBps:        plan.PenaltyRateBps,
		MinCreditScore:        plan.MinCreditScore,
	}

	return uc.Execute(ctx, req)
}

// GetCreditPlan retourne la configuration d'un plan de crédit
func (uc *ConfigureCreditPlanUsecase) GetCreditPlan(
	ctx context.Context,
	productID string,
) (*entity.CreditPlan, error) {
	return uc.planRepo.FindByProductID(ctx, productID)
}

// ListCreditPlansByShop retourne tous les plans de crédit d'une boutique
func (uc *ConfigureCreditPlanUsecase) ListCreditPlansByShop(
	ctx context.Context,
	shopID string,
) ([]*entity.CreditPlan, error) {
	return uc.planRepo.FindByShopID(ctx, shopID)
}

// ListEnabledCreditPlansByShop retourne les plans activés d'une boutique
func (uc *ConfigureCreditPlanUsecase) ListEnabledCreditPlansByShop(
	ctx context.Context,
	shopID string,
) ([]*entity.CreditPlan, error) {
	return uc.planRepo.FindEnabledByShopID(ctx, shopID)
}

// IsCreditEnabled vérifie si le crédit est activé pour un produit
func (uc *ConfigureCreditPlanUsecase) IsCreditEnabled(
	ctx context.Context,
	productID string,
) (bool, error) {
	plan, err := uc.planRepo.FindByProductID(ctx, productID)
	if err != nil {
		// Plan n'existe pas = crédit non activé
		return false, nil
	}
	return plan.IsEnabled, nil
}
