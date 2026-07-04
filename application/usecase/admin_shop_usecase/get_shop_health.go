package adminshopusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.2.0 : GET SHOP HEALTH USECASE
// ============================================================
//
// 🎯 Objectif :
//   Récupérer le Health Score détaillé d'une boutique.
//   Inspiré d'Amazon Account Health Dashboard.
//
// 📋 Informations retournées :
//   - Score actuel + niveau + emoji
//   - Breakdown détaillé (comment le score est calculé)
//   - Recommendations (actions correctives)
//   - Comparaison plateforme (moyenne, percentile)
//   - Score recalculé (en temps réel)
//
// 🔐 Sécurité :
//   - Réservé aux super_admin et admin
//   - Audit trail automatique
//
// ============================================================

// GetShopHealthUsecase gère la récupération du health score détaillé
type GetShopHealthUsecase struct {
	shopRepo repository.ShopRepository
}

// NewGetShopHealthUsecase crée une nouvelle instance
func NewGetShopHealthUsecase(shopRepo repository.ShopRepository) *GetShopHealthUsecase {
	return &GetShopHealthUsecase{shopRepo: shopRepo}
}

// GetShopHealthRequest représente la requête
type GetShopHealthRequest struct {
	ShopID string `json:"shop_id"`
}

// GetShopHealthResponse représente la réponse complète
type GetShopHealthResponse struct {
	// Score actuel
	Current *HealthScoreCurrent `json:"current"`

	// Score recalculé en temps réel
	Recalculated *HealthScoreRecalculated `json:"recalculated"`

	// Breakdown détaillé
	Breakdown *HealthBreakdown `json:"breakdown"`

	// Recommendations
	Recommendations []*HealthRecommendation `json:"recommendations"`

	// Comparaison plateforme
	PlatformComparison *PlatformComparison `json:"platform_comparison"`

	// Historique (si disponible)
	History []*HealthHistoryItem `json:"history,omitempty"`
}

// HealthScoreCurrent contient le score actuel stocké
type HealthScoreCurrent struct {
	Score           int        `json:"score"`
	Level           string     `json:"level"`
	Emoji           string     `json:"emoji"`
	Description     string     `json:"description"`
	NeedsAction     bool       `json:"needs_action"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	DaysSinceUpdate int        `json:"days_since_update"`
}

// HealthScoreRecalculated contient le score recalculé en temps réel
type HealthScoreRecalculated struct {
	Score       int    `json:"score"`
	Level       string `json:"level"`
	Emoji       string `json:"emoji"`
	Description string `json:"description"`
	IsDifferent bool   `json:"is_different"` // true si différent du score stocké
}

// HealthBreakdown contient le détail du calcul
type HealthBreakdown struct {
	BaseScore       int                 `json:"base_score"`       // 1000
	Adjustments     []*HealthAdjustment `json:"adjustments"`      // Liste des ajustements
	TotalAdjustment int                 `json:"total_adjustment"` // Somme des ajustements
	FinalScore      int                 `json:"final_score"`      // Score final
}

// HealthAdjustment représente un ajustement du score
type HealthAdjustment struct {
	Factor      string `json:"factor"`      // "kyc_status", "is_active", etc.
	Description string `json:"description"` // Description lisible
	Value       int    `json:"value"`       // Valeur de l'ajustement (+/-)
	Status      string `json:"status"`      // "good", "warning", "critical"
	Icon        string `json:"icon"`        // Emoji
}

// HealthRecommendation représente une action recommandée
type HealthRecommendation struct {
	Priority    string `json:"priority"`    // "critical", "high", "medium", "low"
	Title       string `json:"title"`       // Titre de l'action
	Description string `json:"description"` // Description détaillée
	Impact      int    `json:"impact"`      // Impact sur le score (+X points)
	ActionURL   string `json:"action_url"`  // URL vers l'action (optionnel)
}

// PlatformComparison contient la comparaison avec la plateforme
type PlatformComparison struct {
	PlatformAverage   int `json:"platform_average"`    // Moyenne plateforme
	PlatformMedian    int `json:"platform_median"`     // Médiane plateforme
	ShopPercentile    int `json:"shop_percentile"`     // Percentile du shop (0-100)
	BetterThanPercent int `json:"better_than_percent"` // % de shops meilleurs
	WorseThanPercent  int `json:"worse_than_percent"`  // % de shops moins bons
	TotalShops        int `json:"total_shops"`         // Nombre total de shops
}

// HealthHistoryItem représente un item de l'historique
type HealthHistoryItem struct {
	Date   time.Time `json:"date"`
	Score  int       `json:"score"`
	Level  string    `json:"level"`
	Change int       `json:"change"` // Changement depuis précédent
	Reason string    `json:"reason"` // Raison du changement
}

// Execute récupère le health score détaillé
func (uc *GetShopHealthUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *GetShopHealthRequest,
) (*GetShopHealthResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider l'ID
	shopID, err := uuid.Parse(req.ShopID)
	if err != nil {
		logger.Warn().
			Str("shop_id", req.ShopID).
			Msg("❌ ID shop invalide")
		return nil, errors.New("invalid shop_id format")
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("shop_id", shopID.String()).
		Msg("🏥 Récupération health score détaillé")

	// 2. Récupérer la shop
	shop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération shop")
		return nil, fmt.Errorf("find shop: %w", err)
	}
	if shop == nil {
		logger.Warn().
			Str("shop_id", shopID.String()).
			Msg("❌ Shop non trouvée")
		return nil, errors.New("shop not found")
	}

	// 3. Récupérer les stats plateforme
	stats, err := uc.shopRepo.GetHealthStats(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur récupération stats plateforme (non bloquant)")
		stats = nil
	}

	// 4. Construire la réponse
	response := &GetShopHealthResponse{
		Current:            uc.buildCurrentScore(shop),
		Recalculated:       uc.buildRecalculatedScore(shop),
		Breakdown:          uc.buildBreakdown(shop),
		Recommendations:    uc.buildRecommendations(shop),
		PlatformComparison: uc.buildPlatformComparison(shop, stats),
		History:            []*HealthHistoryItem{}, // Historique vide pour l'instant
	}

	logger.Info().
		Str("shop_id", shopID.String()).
		Int("current_score", shop.HealthScore).
		Int("recalculated_score", response.Recalculated.Score).
		Str("health_level", string(shop.HealthLevel)).
		Int("recommendations_count", len(response.Recommendations)).
		Msg("✅ Health score détaillé récupéré")

	return response, nil
}

// ============================================================
// BUILDERS
// ============================================================

func (uc *GetShopHealthUsecase) buildCurrentScore(shop *entity.Shop) *HealthScoreCurrent {
	daysSinceUpdate := 0
	if shop.HealthUpdatedAt != nil {
		daysSinceUpdate = int(time.Since(*shop.HealthUpdatedAt).Hours() / 24)
	}

	return &HealthScoreCurrent{
		Score:           shop.HealthScore,
		Level:           string(shop.HealthLevel),
		Emoji:           shop.GetHealthEmoji(),
		Description:     shop.GetHealthDescription(),
		NeedsAction:     shop.NeedsImmediateAction(),
		UpdatedAt:       shop.HealthUpdatedAt,
		DaysSinceUpdate: daysSinceUpdate,
	}
}

func (uc *GetShopHealthUsecase) buildRecalculatedScore(shop *entity.Shop) *HealthScoreRecalculated {
	// Recalculer le score en temps réel
	recalculatedScore := shop.CalculateHealthScore()
	recalculatedLevel := shop.GetHealthLevelFromScore(recalculatedScore)

	return &HealthScoreRecalculated{
		Score:       recalculatedScore,
		Level:       string(recalculatedLevel),
		Emoji:       getHealthEmoji(recalculatedLevel),
		Description: getHealthDescription(recalculatedLevel),
		IsDifferent: recalculatedScore != shop.HealthScore,
	}
}

func (uc *GetShopHealthUsecase) buildBreakdown(shop *entity.Shop) *HealthBreakdown {
	baseScore := 1000
	adjustments := []*HealthAdjustment{}
	totalAdjustment := 0

	// 1. KYC Status
	if shop.KYCStatus != entity.ShopKYCStatusVerified {
		adjustment := &HealthAdjustment{
			Factor:      "kyc_status",
			Description: "KYC non vérifié",
			Value:       -200,
			Status:      "critical",
			Icon:        "❌",
		}
		adjustments = append(adjustments, adjustment)
		totalAdjustment += adjustment.Value
	} else {
		adjustment := &HealthAdjustment{
			Factor:      "kyc_status",
			Description: "KYC vérifié",
			Value:       0,
			Status:      "good",
			Icon:        "✅",
		}
		adjustments = append(adjustments, adjustment)
	}

	// 2. Shop Active
	if !shop.IsActive {
		adjustment := &HealthAdjustment{
			Factor:      "is_active",
			Description: "Boutique inactive",
			Value:       -300,
			Status:      "critical",
			Icon:        "🔴",
		}
		adjustments = append(adjustments, adjustment)
		totalAdjustment += adjustment.Value
	} else {
		adjustment := &HealthAdjustment{
			Factor:      "is_active",
			Description: "Boutique active",
			Value:       0,
			Status:      "good",
			Icon:        "🟢",
		}
		adjustments = append(adjustments, adjustment)
	}

	// 3. Suspension
	if shop.IsSuspended() {
		adjustment := &HealthAdjustment{
			Factor:      "suspended",
			Description: "Boutique suspendue",
			Value:       -500,
			Status:      "critical",
			Icon:        "⛔",
		}
		adjustments = append(adjustments, adjustment)
		totalAdjustment += adjustment.Value
	} else {
		adjustment := &HealthAdjustment{
			Factor:      "suspended",
			Description: "Boutique non suspendue",
			Value:       0,
			Status:      "good",
			Icon:        "✅",
		}
		adjustments = append(adjustments, adjustment)
	}

	// 4. Ancienneté
	daysSinceCreation := int(time.Since(shop.CreatedAt).Hours() / 24)
	if daysSinceCreation > 180 { // > 6 mois
		adjustment := &HealthAdjustment{
			Factor:      "seniority",
			Description: fmt.Sprintf("Boutique ancienne (%d jours)", daysSinceCreation),
			Value:       100,
			Status:      "good",
			Icon:        "⭐",
		}
		adjustments = append(adjustments, adjustment)
		totalAdjustment += adjustment.Value
	} else {
		adjustment := &HealthAdjustment{
			Factor:      "seniority",
			Description: fmt.Sprintf("Boutique récente (%d jours)", daysSinceCreation),
			Value:       0,
			Status:      "neutral",
			Icon:        "🆕",
		}
		adjustments = append(adjustments, adjustment)
	}

	// Calculer le score final
	finalScore := baseScore + totalAdjustment
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 1000 {
		finalScore = 1000
	}

	return &HealthBreakdown{
		BaseScore:       baseScore,
		Adjustments:     adjustments,
		TotalAdjustment: totalAdjustment,
		FinalScore:      finalScore,
	}
}

func (uc *GetShopHealthUsecase) buildRecommendations(shop *entity.Shop) []*HealthRecommendation {
	recommendations := []*HealthRecommendation{}

	// 1. KYC non vérifié
	if shop.KYCStatus != entity.ShopKYCStatusVerified {
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "critical",
			Title:       "Vérifier le KYC du marchand",
			Description: "Le marchand n'a pas encore vérifié son identité. Cela bloque les retraits et réduit le score de santé de 200 points.",
			Impact:      200,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/kyc", shop.ID.String()),
		})
	}

	// 2. Boutique inactive
	if !shop.IsActive {
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "critical",
			Title:       "Réactiver la boutique",
			Description: "La boutique est inactive. Cela réduit le score de santé de 300 points et empêche les ventes.",
			Impact:      300,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/activate", shop.ID.String()),
		})
	}

	// 3. Boutique suspendue
	if shop.IsSuspended() {
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "critical",
			Title:       "Lever la suspension",
			Description: "La boutique est suspendue. Cela réduit le score de santé de 500 points et bloque toutes les activités.",
			Impact:      500,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/activate", shop.ID.String()),
		})
	}

	// 4. Score < 400 (critical)
	// 4. Score < 400 (critical)
	if shop.HealthScore < 400 && !shop.IsSuspended() && shop.IsActive {
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "high",
			Title:       "Score de santé critique",
			Description: "Le score de santé est inférieur à 400. Une action immédiate est requise pour éviter la suspension automatique.",
			Impact:      0,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/health", shop.ID.String()),
		})
	}

	// 5. Score < 600 (warning)
	if shop.HealthScore >= 400 && shop.HealthScore < 600 {
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "medium",
			Title:       "Score de santé à améliorer",
			Description: "Le score de santé est inférieur à 600. Des actions correctives sont recommandées.",
			Impact:      0,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/health", shop.ID.String()),
		})
	}

	// 6. Jamais revue
	if shop.LastReviewedAt == nil {
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "low",
			Title:       "Effectuer une revue admin",
			Description: "Cette boutique n'a jamais été revue par un administrateur. Une revue est recommandée pour vérifier la conformité.",
			Impact:      0,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/review", shop.ID.String()),
		})
	}

	// 7. Dernière revue > 30 jours
	if shop.LastReviewedAt != nil && time.Since(*shop.LastReviewedAt) > 30*24*time.Hour {
		daysSinceReview := int(time.Since(*shop.LastReviewedAt).Hours() / 24)
		recommendations = append(recommendations, &HealthRecommendation{
			Priority:    "low",
			Title:       "Revue admin requise",
			Description: fmt.Sprintf("La dernière revue admin date de %d jours. Une nouvelle revue est recommandée.", daysSinceReview),
			Impact:      0,
			ActionURL:   fmt.Sprintf("/admin/shops/%s/review", shop.ID.String()),
		})
	}

	return recommendations
}

func (uc *GetShopHealthUsecase) buildPlatformComparison(
	shop *entity.Shop,
	stats *repository.ShopHealthStats,
) *PlatformComparison {
	if stats == nil || stats.TotalShops == 0 {
		return &PlatformComparison{
			PlatformAverage:   0,
			PlatformMedian:    0,
			ShopPercentile:    0,
			BetterThanPercent: 0,
			WorseThanPercent:  0,
			TotalShops:        0,
		}
	}

	// Calculer le percentile (approximatif)
	// Pour une vraie implémentation, il faudrait requêter la DB
	betterThanPercent := 0
	worseThanPercent := 0

	if shop.HealthScore >= int(stats.AverageScore) {
		betterThanPercent = 50 + (shop.HealthScore-int(stats.AverageScore))/10
		worseThanPercent = 100 - betterThanPercent
	} else {
		worseThanPercent = 50 + (int(stats.AverageScore)-shop.HealthScore)/10
		betterThanPercent = 100 - worseThanPercent
	}

	// Limiter entre 0 et 100
	if betterThanPercent < 0 {
		betterThanPercent = 0
	}
	if betterThanPercent > 100 {
		betterThanPercent = 100
	}
	if worseThanPercent < 0 {
		worseThanPercent = 0
	}
	if worseThanPercent > 100 {
		worseThanPercent = 100
	}

	return &PlatformComparison{
		PlatformAverage:   int(stats.AverageScore),
		PlatformMedian:    int(stats.AverageScore), // Approximation
		ShopPercentile:    betterThanPercent,
		BetterThanPercent: betterThanPercent,
		WorseThanPercent:  worseThanPercent,
		TotalShops:        stats.TotalShops,
	}
}

// ============================================================
// HELPERS
// ============================================================

func getHealthEmoji(level entity.ShopHealthLevel) string {
	switch level {
	case entity.ShopHealthExcellent:
		return "🟢"
	case entity.ShopHealthGood:
		return "🔵"
	case entity.ShopHealthWarning:
		return "🟡"
	case entity.ShopHealthCritical:
		return "🔴"
	default:
		return "⚪"
	}
}

func getHealthDescription(level entity.ShopHealthLevel) string {
	switch level {
	case entity.ShopHealthExcellent:
		return "Excellent - Aucune action requise"
	case entity.ShopHealthGood:
		return "Bon - Surveillance recommandée"
	case entity.ShopHealthWarning:
		return "Attention - Actions correctives recommandées"
	case entity.ShopHealthCritical:
		return "Critique - Action immédiate requise"
	default:
		return "Inconnu"
	}
}
