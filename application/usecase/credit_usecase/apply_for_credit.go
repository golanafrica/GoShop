package creditusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// APPLY FOR CREDIT USECASE
// ============================================================

// ApplyForCreditRequest représente la requête pour demander un crédit
type ApplyForCreditRequest struct {
	// Identifiants
	CustomerID string `json:"customer_id"`
	ProductID  string `json:"product_id"`

	// Durée demandée (en mois)
	RequestedDurationMonths int `json:"requested_duration_months"`
}

// ApplyForCreditResponse représente la réponse après demande
type ApplyForCreditResponse struct {
	// Demande créée
	ApplicationID string                         `json:"application_id"`
	CustomerID    string                         `json:"customer_id"`
	ProductID     string                         `json:"product_id"`
	ShopID        string                         `json:"shop_id"`
	Status        entity.CreditApplicationStatus `json:"status"`

	// Calculs financiers
	ProductPriceCents   int64 `json:"product_price_cents"`
	DownPaymentCents    int64 `json:"down_payment_cents"`
	FinancedAmountCents int64 `json:"financed_amount_cents"`
	InterestAmountCents int64 `json:"interest_amount_cents"`
	TotalAmountCents    int64 `json:"total_amount_cents"`
	MonthlyPaymentCents int64 `json:"monthly_payment_cents"`

	// Durée
	RequestedDurationMonths int `json:"requested_duration_months"`

	// Score de crédit
	CreditScoreAtApplication int    `json:"credit_score_at_application"`
	CreditScoreLevel         string `json:"credit_score_level"` // "excellent", "good", "medium", "bad"

	// Plan de crédit utilisé
	PlanInterestRateBps int `json:"plan_interest_rate_bps"`
	PlanMinCreditScore  int `json:"plan_min_credit_score"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// Validate valide la requête
func (r *ApplyForCreditRequest) Validate() error {
	if r.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if r.ProductID == "" {
		return fmt.Errorf("product_id is required")
	}
	if r.RequestedDurationMonths < entity.MinDurationMonths || r.RequestedDurationMonths > entity.MaxDurationMonths {
		return fmt.Errorf("requested_duration_months must be between %d and %d",
			entity.MinDurationMonths, entity.MaxDurationMonths)
	}
	return nil
}

// ApplyForCreditUsecase permet à un client de demander un crédit
type ApplyForCreditUsecase struct {
	applicationRepo repository.CreditApplicationRepository
	planRepo        repository.CreditPlanRepository
	productRepo     repository.ProductRepository
	scoreRepo       repository.CreditScoreRepository
	txManager       repository.TxManager
}

// NewApplyForCreditUsecase crée une nouvelle instance
func NewApplyForCreditUsecase(
	applicationRepo repository.CreditApplicationRepository,
	planRepo repository.CreditPlanRepository,
	productRepo repository.ProductRepository,
	scoreRepo repository.CreditScoreRepository,
	txManager repository.TxManager,
) *ApplyForCreditUsecase {
	return &ApplyForCreditUsecase{
		applicationRepo: applicationRepo,
		planRepo:        planRepo,
		productRepo:     productRepo,
		scoreRepo:       scoreRepo,
		txManager:       txManager,
	}
}

// Execute crée une demande de crédit
func (uc *ApplyForCreditUsecase) Execute(ctx context.Context, req *ApplyForCreditRequest) (*ApplyForCreditResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid apply for credit request")
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

	// 4. Récupérer le produit
	product, err := uc.productRepo.WithTX(tx).FindByID(ctx, req.ProductID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find product")
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// 5. Récupérer le plan de crédit
	plan, err := uc.planRepo.WithTX(tx).FindByProductID(ctx, req.ProductID)
	if err != nil {
		logger.Error().Err(err).Msg("Credit plan not found")
		return nil, fmt.Errorf("credit not available for this product: %w", err)
	}

	// 6. Vérifier que le crédit est activé
	if !plan.IsEnabled {
		return nil, fmt.Errorf("credit is not enabled for this product")
	}

	// 7. Vérifier que la durée demandée est dans les limites du plan
	if req.RequestedDurationMonths > plan.MaxDurationMonths {
		return nil, fmt.Errorf("requested duration (%d months) exceeds maximum allowed (%d months)",
			req.RequestedDurationMonths, plan.MaxDurationMonths)
	}

	// 8. Vérifier qu'il n'y a pas déjà une demande en cours
	existingApp, err := uc.applicationRepo.WithTX(tx).FindByCustomerAndProduct(ctx, req.CustomerID, req.ProductID)
	if err == nil && existingApp != nil {
		if existingApp.Status == entity.CreditApplicationPending {
			return nil, fmt.Errorf("you already have a pending application for this product")
		}
		if existingApp.Status == entity.CreditApplicationApproved {
			return nil, fmt.Errorf("you already have an approved application for this product")
		}
		// Si rejetée ou annulée, on peut créer une nouvelle demande
	}

	// 9. Récupérer ou créer le score de crédit du client
	score, err := uc.scoreRepo.WithTX(tx).FindByCustomerAndShop(ctx, req.CustomerID, shopID)
	if err != nil {
		// Score n'existe pas, le créer avec valeur par défaut (500)
		logger.Info().
			Str("customer_id", req.CustomerID).
			Str("shop_id", shopID).
			Msg("Creating default credit score for customer")

		score = entity.NewCreditScore(req.CustomerID, shopID)
		if err := uc.scoreRepo.WithTX(tx).Create(ctx, score); err != nil {
			logger.Error().Err(err).Msg("Failed to create credit score")
			return nil, fmt.Errorf("failed to create credit score: %w", err)
		}
	}

	// 10. Vérifier que le score atteint le minimum requis
	if !score.MeetsMinRequirement(plan.MinCreditScore) {
		logger.Warn().
			Str("customer_id", req.CustomerID).
			Int("customer_score", score.Score).
			Int("min_required", plan.MinCreditScore).
			Msg("Credit score too low")
		return nil, fmt.Errorf("credit score too low: your score is %d, minimum required is %d",
			score.Score, plan.MinCreditScore)
	}

	// 11. Calculer les détails du crédit
	// Note: NewCreditApplication fait déjà les calculs en interne,
	// on a juste besoin de monthlyPaymentCents pour le log
	_, _, _, _, monthlyPaymentCents := entity.CalculateCreditDetails(
		product.PriceCents,
		plan.MinDownPaymentPercent,
		req.RequestedDurationMonths,
		plan.InterestRateBps,
	)

	// 12. Créer la demande
	application, err := entity.NewCreditApplication(
		req.CustomerID,
		req.ProductID,
		shopID,
		req.RequestedDurationMonths,
		score.Score,
		product.PriceCents,
		plan.MinDownPaymentPercent,
		plan.InterestRateBps,
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create credit application entity")
		return nil, fmt.Errorf("failed to create credit application: %w", err)
	}

	// Forcer l'ID (car NewCreditApplication ne le génère pas)
	application.ID = uuid.New().String()

	// 13. Sauvegarder la demande
	if err := uc.applicationRepo.WithTX(tx).Create(ctx, application); err != nil {
		logger.Error().Err(err).Msg("Failed to save credit application")
		return nil, fmt.Errorf("failed to save credit application: %w", err)
	}

	// 14. Enregistrer le nouveau contrat dans les stats du score
	score.RecordNewContract()
	if err := uc.scoreRepo.WithTX(tx).Update(ctx, score); err != nil {
		logger.Error().Err(err).Msg("Failed to update credit score stats")
		// On continue quand même, la demande a été créée
	}

	// 15. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 16. Déterminer le niveau de score
	scoreLevel := determineScoreLevel(score.Score)

	// 17. Logger le succès
	logger.Info().
		Str("application_id", application.ID).
		Str("customer_id", req.CustomerID).
		Str("product_id", req.ProductID).
		Str("shop_id", shopID).
		Int("credit_score", score.Score).
		Int64("monthly_payment_cents", monthlyPaymentCents).
		Msg("Credit application created successfully")

	// 18. Construire la réponse
	return &ApplyForCreditResponse{
		ApplicationID:            application.ID,
		CustomerID:               application.CustomerID,
		ProductID:                application.ProductID,
		ShopID:                   application.ShopID,
		Status:                   application.Status,
		ProductPriceCents:        application.ProductPriceCents,
		DownPaymentCents:         application.DownPaymentCents,
		FinancedAmountCents:      application.FinancedAmountCents,
		InterestAmountCents:      application.InterestAmountCents,
		TotalAmountCents:         application.TotalAmountCents,
		MonthlyPaymentCents:      application.MonthlyPaymentCents,
		RequestedDurationMonths:  application.RequestedDurationMonths,
		CreditScoreAtApplication: application.CreditScoreAtApplication,
		CreditScoreLevel:         scoreLevel,
		PlanInterestRateBps:      plan.InterestRateBps,
		PlanMinCreditScore:       plan.MinCreditScore,
		CreatedAt:                application.CreatedAt,
	}, nil
}

// ============================================================
// HELPERS
// ============================================================

// determineScoreLevel retourne le niveau de score en string
func determineScoreLevel(score int) string {
	switch {
	case score >= entity.ScoreExcellent:
		return "excellent"
	case score >= entity.ScoreGood:
		return "good"
	case score >= entity.ScoreMedium:
		return "medium"
	default:
		return "bad"
	}
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// SimulateCreditCalculation simule les calculs sans créer de demande
func (uc *ApplyForCreditUsecase) SimulateCreditCalculation(
	ctx context.Context,
	productID string,
	requestedDurationMonths int,
) (*ApplyForCreditResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le produit
	product, err := uc.productRepo.FindByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// 2. Récupérer le plan de crédit
	plan, err := uc.planRepo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("credit not available for this product: %w", err)
	}

	// 3. Vérifier que le crédit est activé
	if !plan.IsEnabled {
		return nil, fmt.Errorf("credit is not enabled for this product")
	}

	// 4. Calculer les détails
	downPaymentCents, financedAmountCents, interestAmountCents, totalAmountCents, monthlyPaymentCents := entity.CalculateCreditDetails(
		product.PriceCents,
		plan.MinDownPaymentPercent,
		requestedDurationMonths,
		plan.InterestRateBps,
	)

	// 5. Logger
	logger.Info().
		Str("product_id", productID).
		Int("duration_months", requestedDurationMonths).
		Int64("monthly_payment_cents", monthlyPaymentCents).
		Msg("Credit simulation completed")

	// 6. Retourner une réponse simulée
	return &ApplyForCreditResponse{
		ProductID:               productID,
		ProductPriceCents:       product.PriceCents,
		DownPaymentCents:        downPaymentCents,
		FinancedAmountCents:     financedAmountCents,
		InterestAmountCents:     interestAmountCents,
		TotalAmountCents:        totalAmountCents,
		MonthlyPaymentCents:     monthlyPaymentCents,
		RequestedDurationMonths: requestedDurationMonths,
		PlanInterestRateBps:     plan.InterestRateBps,
		PlanMinCreditScore:      plan.MinCreditScore,
	}, nil
}

// GetCustomerCreditScore retourne le score de crédit d'un client pour une boutique
func (uc *ApplyForCreditUsecase) GetCustomerCreditScore(
	ctx context.Context,
	customerID string,
	shopID string,
) (*entity.CreditScore, error) {
	return uc.scoreRepo.FindByCustomerAndShop(ctx, customerID, shopID)
}

// CanApplyForCredit vérifie si un client peut demander un crédit pour un produit
func (uc *ApplyForCreditUsecase) CanApplyForCredit(
	ctx context.Context,
	customerID string,
	productID string,
) (bool, string, error) {
	// 1. Récupérer le plan de crédit
	plan, err := uc.planRepo.FindByProductID(ctx, productID)
	if err != nil {
		return false, "credit not available for this product", nil
	}

	// 2. Vérifier que le crédit est activé
	if !plan.IsEnabled {
		return false, "credit is not enabled for this product", nil
	}

	// 3. Récupérer le contexte multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return false, "multi-tenant error", err
	}
	shopID := shop.ID.String()

	// 4. Récupérer le score de crédit
	score, err := uc.scoreRepo.FindByCustomerAndShop(ctx, customerID, shopID)
	if err != nil {
		return false, "no credit history yet", nil
	}

	// 5. Vérifier le score
	if !score.MeetsMinRequirement(plan.MinCreditScore) {
		return false, fmt.Sprintf("credit score too low: %d (minimum: %d)",
			score.Score, plan.MinCreditScore), nil
	}

	// 6. Vérifier qu'il n'y a pas déjà une demande en cours
	existingApp, err := uc.applicationRepo.FindByCustomerAndProduct(ctx, customerID, productID)
	if err == nil && existingApp != nil {
		if existingApp.Status == entity.CreditApplicationPending {
			return false, "you already have a pending application", nil
		}
		if existingApp.Status == entity.CreditApplicationApproved {
			return false, "you already have an approved application", nil
		}
	}

	return true, "eligible", nil
}
