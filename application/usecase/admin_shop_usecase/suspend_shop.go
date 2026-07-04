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
// 🆕 v4.2.0 : SUSPEND SHOP USECASE
// ============================================================
//
// 🎯 Objectif :
//   Suspendre une boutique avec raison obligatoire.
//   Inspiré d'Amazon Seller Central > Account Suspension.
//
// 📋 Actions effectuées :
//   1. Vérifier que le shop existe
//   2. Vérifier que le shop n'est pas déjà suspendu
//   3. Mettre à jour le shop (suspended_at, suspended_by, reason, is_active=false)
//   4. Recalculer le Health Score (-500 points)
//   5. Logger l'action dans l'audit trail
//   6. Retourner les détails de la suspension
//
// 🔐 Sécurité :
//   - Réservé aux super_admin et admin
//   - Raison obligatoire (min 10 caractères)
//   - Audit trail complet (IP, User-Agent, Request-ID)
//
// ============================================================

// SuspendShopUsecase gère la suspension d'une boutique
type SuspendShopUsecase struct {
	shopRepo   repository.ShopRepository
	actionRepo repository.ShopAdminActionRepository
}

// NewSuspendShopUsecase crée une nouvelle instance
func NewSuspendShopUsecase(
	shopRepo repository.ShopRepository,
	actionRepo repository.ShopAdminActionRepository,
) *SuspendShopUsecase {
	return &SuspendShopUsecase{
		shopRepo:   shopRepo,
		actionRepo: actionRepo,
	}
}

// SuspendShopRequest représente la requête
type SuspendShopRequest struct {
	ShopID string `json:"shop_id"`
	Reason string `json:"reason"` // Obligatoire, min 10 caractères
}

// SuspendShopResponse représente la réponse
type SuspendShopResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	Shop          *SuspendedShopInfo `json:"shop"`
	Suspension    *SuspensionInfo    `json:"suspension"`
	HealthImpact  *HealthImpactInfo  `json:"health_impact"`
	AuditActionID string             `json:"audit_action_id"`
}

// SuspendedShopInfo contient les infos de la boutique suspendue
type SuspendedShopInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	OwnerID  string `json:"owner_id"`
	Plan     string `json:"plan"`
	IsActive bool   `json:"is_active"` // Toujours false après suspension
}

// SuspensionInfo contient les détails de la suspension
type SuspensionInfo struct {
	SuspendedAt   time.Time `json:"suspended_at"`
	SuspendedBy   string    `json:"suspended_by"`
	AdminEmail    string    `json:"admin_email"`
	AdminRole     string    `json:"admin_role"`
	Reason        string    `json:"reason"`
	DaysSuspended int       `json:"days_suspended"` // 0 au moment de la suspension
}

// HealthImpactInfo contient l'impact sur le health score
type HealthImpactInfo struct {
	PreviousScore int    `json:"previous_score"`
	PreviousLevel string `json:"previous_level"`
	PreviousEmoji string `json:"previous_emoji"`
	NewScore      int    `json:"new_score"`
	NewLevel      string `json:"new_level"`
	NewEmoji      string `json:"new_emoji"`
	ScoreChange   int    `json:"score_change"` // -500
	LevelChanged  bool   `json:"level_changed"`
}

// Execute suspend une boutique
func (uc *SuspendShopUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *SuspendShopRequest,
) (*SuspendShopResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if err := uc.validateRequest(req); err != nil {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Err(err).
			Msg("❌ Validation requête échouée")
		return nil, err
	}

	// 2. Valider l'ID
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
		Str("reason", req.Reason).
		Msg("⛔ Début suspension boutique")

	// 3. Récupérer la shop
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

	// 4. Vérifier que la shop n'est pas déjà suspendue
	if shop.IsSuspended() {
		logger.Warn().
			Str("shop_id", shopID.String()).
			Time("suspended_at", *shop.SuspendedAt).
			Msg("❌ Shop déjà suspendue")
		return nil, entity.ErrShopAlreadySuspended
	}

	// 5. Sauvegarder l'état précédent (pour audit trail)
	previousScore := shop.HealthScore
	previousLevel := shop.HealthLevel
	previousEmoji := shop.GetHealthEmoji()

	// 6. Suspendre la boutique
	if err := uc.shopRepo.SuspendShop(ctx, shopID, admin.AdminID, req.Reason); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur suspension shop")
		return nil, fmt.Errorf("suspend shop: %w", err)
	}

	// 7. Recalculer le Health Score
	// Recharger la shop pour avoir les données à jour
	updatedShop, err := uc.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur rechargement shop")
		return nil, fmt.Errorf("reload shop: %w", err)
	}

	// Recalculer le score
	newScore := updatedShop.CalculateHealthScore()
	newLevel := updatedShop.GetHealthLevelFromScore(newScore)
	newEmoji := updatedShop.GetHealthEmoji()

	// Mettre à jour le score en DB
	if err := uc.shopRepo.UpdateHealthScore(ctx, shopID, newScore, newLevel); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur mise à jour health score")
		// Non bloquant, on continue
	}

	// 8. Logger l'action dans l'audit trail
	action, err := entity.NewShopAdminAction(
		shopID,
		admin.AdminID,
		admin.AdminEmail,
		admin.AdminRole,
		entity.ShopActionSuspend,
		req.Reason,
	)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur création action admin")
		return nil, fmt.Errorf("create admin action: %w", err)
	}

	// Ajouter les détails
	action.OldValue = map[string]interface{}{
		"is_active":    true,
		"suspended_at": nil,
		"health_score": previousScore,
		"health_level": string(previousLevel),
	}
	action.NewValue = map[string]interface{}{
		"is_active":    false,
		"suspended_at": updatedShop.SuspendedAt,
		"health_score": newScore,
		"health_level": string(newLevel),
	}
	action.IPAddress = admin.IPAddress
	action.UserAgent = admin.UserAgent
	action.RequestID = admin.RequestID

	if err := uc.actionRepo.Create(ctx, action); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur log action admin")
		// Non bloquant, on continue
	}

	// 9. Construire la réponse
	response := &SuspendShopResponse{
		Success: true,
		Message: fmt.Sprintf("Boutique '%s' suspendue avec succès. Le marchand a été notifié.", shop.Name),
		Shop: &SuspendedShopInfo{
			ID:       shop.ID.String(),
			Name:     shop.Name,
			Slug:     shop.Slug,
			OwnerID:  shop.OwnerID,
			Plan:     string(shop.Plan),
			IsActive: false,
		},
		Suspension: &SuspensionInfo{
			SuspendedAt:   *updatedShop.SuspendedAt,
			SuspendedBy:   admin.AdminID,
			AdminEmail:    admin.AdminEmail,
			AdminRole:     admin.AdminRole,
			Reason:        req.Reason,
			DaysSuspended: 0,
		},
		HealthImpact: &HealthImpactInfo{
			PreviousScore: previousScore,
			PreviousLevel: string(previousLevel),
			PreviousEmoji: previousEmoji,
			NewScore:      newScore,
			NewLevel:      string(newLevel),
			NewEmoji:      newEmoji,
			ScoreChange:   newScore - previousScore,
			LevelChanged:  newLevel != previousLevel,
		},
		AuditActionID: action.ID.String(),
	}

	logger.Info().
		Str("shop_id", shopID.String()).
		Str("shop_name", shop.Name).
		Str("admin_id", admin.AdminID).
		Int("previous_score", previousScore).
		Int("new_score", newScore).
		Int("score_change", newScore-previousScore).
		Str("audit_action_id", action.ID.String()).
		Msg("✅ Boutique suspendue avec succès")

	return response, nil
}

// ============================================================
// VALIDATION
// ============================================================

func (uc *SuspendShopUsecase) validateRequest(req *SuspendShopRequest) error {
	if req.ShopID == "" {
		return errors.New("shop_id is required")
	}

	if req.Reason == "" {
		return errors.New("reason is required")
	}

	// Raison minimum 10 caractères
	if len(req.Reason) < 10 {
		return fmt.Errorf("reason must be at least 10 characters (current: %d)", len(req.Reason))
	}

	// Raison maximum 1000 caractères
	if len(req.Reason) > 1000 {
		return fmt.Errorf("reason must be at most 1000 characters (current: %d)", len(req.Reason))
	}

	return nil
}
