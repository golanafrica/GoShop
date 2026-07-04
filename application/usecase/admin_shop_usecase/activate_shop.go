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
// 🆕 v4.2.0 : ACTIVATE SHOP USECASE
// ============================================================
//
// 🎯 Objectif :
//   Réactiver une boutique suspendue.
//   Inspiré d'Amazon Seller Central > Account Reinstatement.
//
// 📋 Actions effectuées :
//   1. Vérifier que le shop existe
//   2. Vérifier que le shop EST suspendue (sinon erreur)
//   3. Mettre à jour le shop (suspended_at=NULL, is_active=true)
//   4. Recalculer le Health Score (+500 points)
//   5. Logger l'action dans l'audit trail
//   6. Retourner les détails de la réactivation
//
// 🔐 Sécurité :
//   - Réservé aux super_admin et admin
//   - Raison optionnelle (contrairement à suspend)
//   - Audit trail complet (IP, User-Agent, Request-ID)
//
// ============================================================

// ActivateShopUsecase gère la réactivation d'une boutique
type ActivateShopUsecase struct {
	shopRepo   repository.ShopRepository
	actionRepo repository.ShopAdminActionRepository
}

// NewActivateShopUsecase crée une nouvelle instance
func NewActivateShopUsecase(
	shopRepo repository.ShopRepository,
	actionRepo repository.ShopAdminActionRepository,
) *ActivateShopUsecase {
	return &ActivateShopUsecase{
		shopRepo:   shopRepo,
		actionRepo: actionRepo,
	}
}

// ActivateShopRequest représente la requête
type ActivateShopRequest struct {
	ShopID string  `json:"shop_id"`
	Reason *string `json:"reason,omitempty"` // Optionnel (contrairement à suspend)
}

// ActivateShopResponse représente la réponse
type ActivateShopResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	Shop          *ActivatedShopInfo `json:"shop"`
	Reactivation  *ReactivationInfo  `json:"reactivation"`
	HealthImpact  *HealthImpactInfo  `json:"health_impact"`
	AuditActionID string             `json:"audit_action_id"`
}

// ActivatedShopInfo contient les infos de la boutique réactivée
type ActivatedShopInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	OwnerID  string `json:"owner_id"`
	Plan     string `json:"plan"`
	IsActive bool   `json:"is_active"` // Toujours true après réactivation
}

// ReactivationInfo contient les détails de la réactivation
type ReactivationInfo struct {
	ReactivatedAt      time.Time               `json:"reactivated_at"`
	ReactivatedBy      string                  `json:"reactivated_by"`
	AdminEmail         string                  `json:"admin_email"`
	AdminRole          string                  `json:"admin_role"`
	Reason             *string                 `json:"reason,omitempty"`
	PreviousSuspension *PreviousSuspensionInfo `json:"previous_suspension"`
}

// PreviousSuspensionInfo contient les infos de la suspension précédente
type PreviousSuspensionInfo struct {
	SuspendedAt   time.Time `json:"suspended_at"`
	SuspendedBy   string    `json:"suspended_by"`
	Reason        string    `json:"reason"`
	DaysSuspended int       `json:"days_suspended"`
}

// Execute réactive une boutique
func (uc *ActivateShopUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *ActivateShopRequest,
) (*ActivateShopResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation de la requête
	if req.ShopID == "" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Msg("❌ shop_id manquant")
		return nil, errors.New("shop_id is required")
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
		Msg("✅ Début réactivation boutique")

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

	// 4. Vérifier que la shop EST suspendue
	if !shop.IsSuspended() {
		logger.Warn().
			Str("shop_id", shopID.String()).
			Msg("❌ Shop n'est pas suspendue")
		return nil, entity.ErrShopNotSuspended
	}

	// 5. Sauvegarder l'état précédent (pour audit trail)
	previousScore := shop.HealthScore
	previousLevel := shop.HealthLevel
	previousEmoji := shop.GetHealthEmoji()

	// Sauvegarder les infos de suspension (pour la réponse)
	previousSuspension := &PreviousSuspensionInfo{
		SuspendedAt:   *shop.SuspendedAt,
		SuspendedBy:   *shop.SuspendedBy,
		Reason:        *shop.SuspensionReason,
		DaysSuspended: int(time.Since(*shop.SuspendedAt).Hours() / 24),
	}

	// 6. Réactiver la boutique
	if err := uc.shopRepo.ActivateShop(ctx, shopID, admin.AdminID); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur réactivation shop")
		return nil, fmt.Errorf("activate shop: %w", err)
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
		entity.ShopActionActivate,
		getReasonOrDefault(req.Reason),
	)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur création action admin")
		return nil, fmt.Errorf("create admin action: %w", err)
	}

	// Ajouter les détails
	action.OldValue = map[string]interface{}{
		"is_active":         false,
		"suspended_at":      previousSuspension.SuspendedAt,
		"suspended_by":      previousSuspension.SuspendedBy,
		"suspension_reason": previousSuspension.Reason,
		"health_score":      previousScore,
		"health_level":      string(previousLevel),
	}
	action.NewValue = map[string]interface{}{
		"is_active":    true,
		"suspended_at": nil,
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
	response := &ActivateShopResponse{
		Success: true,
		Message: fmt.Sprintf("Boutique '%s' réactivée avec succès. Le marchand a été notifié.", shop.Name),
		Shop: &ActivatedShopInfo{
			ID:       shop.ID.String(),
			Name:     shop.Name,
			Slug:     shop.Slug,
			OwnerID:  shop.OwnerID,
			Plan:     string(shop.Plan),
			IsActive: true,
		},
		Reactivation: &ReactivationInfo{
			ReactivatedAt:      time.Now(),
			ReactivatedBy:      admin.AdminID,
			AdminEmail:         admin.AdminEmail,
			AdminRole:          admin.AdminRole,
			Reason:             req.Reason,
			PreviousSuspension: previousSuspension,
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
		Int("days_suspended", previousSuspension.DaysSuspended).
		Str("audit_action_id", action.ID.String()).
		Msg("✅ Boutique réactivée avec succès")

	return response, nil
}

// ============================================================
// HELPERS
// ============================================================

// getReasonOrDefault retourne la raison ou une valeur par défaut
func getReasonOrDefault(reason *string) string {
	if reason == nil || *reason == "" {
		return "Réactivation admin (aucune raison fournie)"
	}
	return *reason
}
