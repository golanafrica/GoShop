package apikeyusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.3 : API KEY MANAGEMENT USECASES
// ============================================================
//
// 🎯 Objectif :
//   Gérer tous les cas d'utilisation des clés API :
//   - CreateAPIKey : Créer une nouvelle clé (affichée UNE SEULE FOIS)
//   - ListAPIKeys : Lister les clés d'un utilisateur
//   - RevokeAPIKey : Révoquer une clé spécifique
//   - RevokeAllAPIKeys : Révoquer toutes les clés d'un user
//   - GetAPIKeyStats : Statistiques globales (admin)
//
// 🔐 Sécurité :
//   - Un user ne peut créer que SES propres clés
//   - super_admin peut créer/révoquer n'importe quelle clé
//   - La clé complète est affichée UNE SEULE FOIS à la création
//   - Audit trail complet (IP, User Agent)
//
// ============================================================

// ============================================================
// STRUCTURES COMMUNES
// ============================================================

// AdminContext représente le contexte de l'admin authentifié
type AdminContext struct {
	AdminID    string
	AdminEmail string
	AdminRole  string
	IPAddress  string
	UserAgent  string
}

// ============================================================
// USECASE 1 : CREATE API KEY
// ============================================================

// CreateAPIKeyUsecase crée une nouvelle clé API
type CreateAPIKeyUsecase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewCreateAPIKeyUsecase crée une nouvelle instance
func NewCreateAPIKeyUsecase(
	apiKeyRepo repository.APIKeyRepository,
) *CreateAPIKeyUsecase {
	return &CreateAPIKeyUsecase{
		apiKeyRepo: apiKeyRepo,
	}
}

// CreateAPIKeyRequest représente la requête
type CreateAPIKeyRequest struct {
	UserID          string               `json:"user_id"`           // Optionnel (admin peut créer pour un autre user)
	Name            string               `json:"name"`              // Requis
	Description     string               `json:"description"`       // Optionnel
	Scopes          []entity.APIKeyScope `json:"scopes"`            // Requis
	IsTest          bool                 `json:"is_test"`           // true = gsk_test_, false = gsk_live_
	LifetimeDays    int                  `json:"lifetime_days"`     // Optionnel (défaut 365)
	RateLimitMinute int                  `json:"rate_limit_minute"` // Optionnel (défaut 60)
	RateLimitDay    int                  `json:"rate_limit_day"`    // Optionnel (défaut 10000)
}

// CreateAPIKeyResponse représente la réponse
type CreateAPIKeyResponse struct {
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	APIKey  *entity.APIKeySummary `json:"api_key"`
	FullKey string                `json:"full_key"` // ⚠️ UNE SEULE FOIS
	Warning string                `json:"warning"`
}

// Execute crée une nouvelle clé API
func (uc *CreateAPIKeyUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *CreateAPIKeyRequest,
) (*CreateAPIKeyResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation du nom
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	// 2. Validation des scopes
	if len(req.Scopes) == 0 {
		return nil, errors.New("at least one scope is required")
	}

	for _, scope := range req.Scopes {
		if !entity.IsValidScope(string(scope)) {
			return nil, fmt.Errorf("invalid scope: %s", scope)
		}
	}

	// 3. Déterminer le user cible
	targetUserID := req.UserID
	if targetUserID == "" {
		targetUserID = admin.AdminID
	}

	// 4. Vérifier permissions
	if targetUserID != admin.AdminID {
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("target_user_id", targetUserID).
				Msg("❌ Permission refusée : user ne peut créer que ses propres clés")
			return nil, errors.New("insufficient permissions: can only create own API keys")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Str("target_user_id", targetUserID).
			Msg("ℹ️ Admin crée une clé API pour un autre user")
	}

	// 5. Vérifier la limite de clés par user
	activeCount, err := uc.apiKeyRepo.CountActiveByUserID(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("count active keys: %w", err)
	}

	// Limite : max 10 clés actives par user
	if activeCount >= 10 {
		logger.Warn().
			Str("user_id", targetUserID).
			Int("active_count", activeCount).
			Msg("❌ Limite de clés API atteinte")
		return nil, errors.New("maximum API keys limit reached (10 active keys per user)")
	}

	// 6. Calculer la durée de vie
	var lifetime time.Duration
	if req.LifetimeDays > 0 {
		lifetime = time.Duration(req.LifetimeDays) * 24 * time.Hour
	} else {
		lifetime = entity.APIKeyDefaultLifetime
	}

	// 7. Créer la clé
	apiKey, fullKey, err := entity.NewAPIKey(
		targetUserID,
		req.Name,
		req.Description,
		req.Scopes,
		req.IsTest,
		lifetime,
		admin.IPAddress,
		admin.UserAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	// 8. Appliquer les rate limits personnalisés
	if req.RateLimitMinute > 0 && req.RateLimitMinute <= entity.APIKeyMaxRateLimitMinute {
		apiKey.RateLimitPerMinute = req.RateLimitMinute
	}
	if req.RateLimitDay > 0 && req.RateLimitDay <= entity.APIKeyMaxRateLimitDay {
		apiKey.RateLimitPerDay = req.RateLimitDay
	}

	// 9. Sauvegarder en base
	if err := uc.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("save api key: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", targetUserID).
		Str("api_key_id", apiKey.ID).
		Str("key_prefix", apiKey.KeyPrefix).
		Str("name", req.Name).
		Int("scopes_count", len(req.Scopes)).
		Bool("is_test", req.IsTest).
		Msg("✅ Clé API créée avec succès")

	return &CreateAPIKeyResponse{
		Success: true,
		Message: "Clé API créée avec succès. Conservez la clé complète, elle ne sera plus affichée.",
		APIKey:  apiKey.ToSummary(),
		FullKey: fullKey, // ⚠️ UNE SEULE FOIS
		Warning: "⚠️ IMPORTANT : Cette clé ne sera JAMAIS réaffichée. Copiez-la et stockez-la en lieu sûr immédiatement.",
	}, nil
}

// ============================================================
// USECASE 2 : LIST API KEYS
// ============================================================

// ListAPIKeysUsecase liste les clés API d'un utilisateur
type ListAPIKeysUsecase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewListAPIKeysUsecase crée une nouvelle instance
func NewListAPIKeysUsecase(
	apiKeyRepo repository.APIKeyRepository,
) *ListAPIKeysUsecase {
	return &ListAPIKeysUsecase{
		apiKeyRepo: apiKeyRepo,
	}
}

// ListAPIKeysRequest représente la requête
type ListAPIKeysRequest struct {
	UserID     string `json:"user_id"`     // Optionnel (admin peut voir d'autres users)
	ActiveOnly bool   `json:"active_only"` // true = seulement clés actives
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

// ListAPIKeysResponse représente la réponse
type ListAPIKeysResponse struct {
	Success    bool                     `json:"success"`
	Message    string                   `json:"message"`
	APIKeys    []*entity.APIKeySummary  `json:"api_keys"`
	Total      int                      `json:"total"`
	Statistics *entity.APIKeyStatistics `json:"statistics,omitempty"`
}

// Execute liste les clés API
func (uc *ListAPIKeysUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *ListAPIKeysRequest,
) (*ListAPIKeysResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Déterminer le user cible
	targetUserID := req.UserID
	if targetUserID == "" {
		targetUserID = admin.AdminID
	}

	// 2. Vérifier permissions
	if targetUserID != admin.AdminID {
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("target_user_id", targetUserID).
				Msg("❌ Permission refusée")
			return nil, errors.New("insufficient permissions: can only view own API keys")
		}
	}

	logger.Debug().
		Str("target_user_id", targetUserID).
		Bool("active_only", req.ActiveOnly).
		Msg("📋 List API keys")

	// 3. Récupérer les clés
	var apiKeys []*entity.APIKey
	var err error

	if req.ActiveOnly {
		apiKeys, err = uc.apiKeyRepo.FindActiveByUserID(ctx, targetUserID)
	} else {
		apiKeys, err = uc.apiKeyRepo.FindByUserID(ctx, targetUserID)
	}

	if err != nil {
		return nil, fmt.Errorf("find api keys: %w", err)
	}

	// 4. Convertir en summaries
	summaries := make([]*entity.APIKeySummary, 0, len(apiKeys))
	for _, k := range apiKeys {
		summaries = append(summaries, k.ToSummary())
	}

	// 5. Récupérer les statistiques
	stats, err := uc.apiKeyRepo.GetStatisticsByUser(ctx, targetUserID)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur récupération statistiques")
	}

	return &ListAPIKeysResponse{
		Success:    true,
		Message:    fmt.Sprintf("%d clé(s) API trouvée(s)", len(summaries)),
		APIKeys:    summaries,
		Total:      len(summaries),
		Statistics: stats,
	}, nil
}

// ============================================================
// USECASE 3 : REVOKE API KEY
// ============================================================

// RevokeAPIKeyUsecase révoque une clé API spécifique
type RevokeAPIKeyUsecase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewRevokeAPIKeyUsecase crée une nouvelle instance
func NewRevokeAPIKeyUsecase(
	apiKeyRepo repository.APIKeyRepository,
) *RevokeAPIKeyUsecase {
	return &RevokeAPIKeyUsecase{
		apiKeyRepo: apiKeyRepo,
	}
}

// RevokeAPIKeyRequest représente la requête
type RevokeAPIKeyRequest struct {
	APIKeyID string `json:"api_key_id"` // Clé à révoquer
	Reason   string `json:"reason"`     // Raison de la révocation
}

// RevokeAPIKeyResponse représente la réponse
type RevokeAPIKeyResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	RevokedKeyID string `json:"revoked_key_id"`
}

// Execute révoque une clé API
func (uc *RevokeAPIKeyUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *RevokeAPIKeyRequest,
) (*RevokeAPIKeyResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation
	if req.APIKeyID == "" {
		return nil, errors.New("api_key_id is required")
	}

	// 2. Récupérer la clé
	apiKey, err := uc.apiKeyRepo.FindByID(ctx, req.APIKeyID)
	if err != nil {
		if errors.Is(err, repository.ErrAPIKeyNotFound) {
			return nil, errors.New("API key not found")
		}
		return nil, fmt.Errorf("find api key: %w", err)
	}

	// 3. Vérifier permissions
	if apiKey.UserID != admin.AdminID {
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("key_owner", apiKey.UserID).
				Msg("❌ Permission refusée")
			return nil, errors.New("insufficient permissions: can only revoke own API keys")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Str("key_owner", apiKey.UserID).
			Msg("ℹ️ Admin révoque la clé d'un autre user")
	}

	// 4. Vérifier si déjà révoquée
	if apiKey.IsRevoked() {
		return nil, errors.New("API key is already revoked")
	}

	// 5. Raison par défaut
	reason := req.Reason
	if reason == "" {
		reason = "Révoqué par l'utilisateur"
	}

	// 6. Révoquer la clé
	if err := uc.apiKeyRepo.RevokeAPIKey(ctx, req.APIKeyID, admin.AdminID, reason); err != nil {
		return nil, fmt.Errorf("revoke api key: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("api_key_id", req.APIKeyID).
		Str("key_owner", apiKey.UserID).
		Str("reason", reason).
		Msg("✅ Clé API révoquée avec succès")

	return &RevokeAPIKeyResponse{
		Success:      true,
		Message:      "Clé API révoquée avec succès. Elle ne peut plus être utilisée.",
		RevokedKeyID: req.APIKeyID,
	}, nil
}

// ============================================================
// USECASE 4 : REVOKE ALL API KEYS
// ============================================================

// RevokeAllAPIKeysUsecase révoque toutes les clés d'un user
type RevokeAllAPIKeysUsecase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewRevokeAllAPIKeysUsecase crée une nouvelle instance
func NewRevokeAllAPIKeysUsecase(
	apiKeyRepo repository.APIKeyRepository,
) *RevokeAllAPIKeysUsecase {
	return &RevokeAllAPIKeysUsecase{
		apiKeyRepo: apiKeyRepo,
	}
}

// RevokeAllAPIKeysRequest représente la requête
type RevokeAllAPIKeysRequest struct {
	UserID string `json:"user_id"` // Optionnel (admin peut révoquer pour un autre user)
	Reason string `json:"reason"`  // Raison de la révocation
}

// RevokeAllAPIKeysResponse représente la réponse
type RevokeAllAPIKeysResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	RevokedCount int    `json:"revoked_count"`
}

// Execute révoque toutes les clés
func (uc *RevokeAllAPIKeysUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *RevokeAllAPIKeysRequest,
) (*RevokeAllAPIKeysResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Déterminer le user cible
	targetUserID := req.UserID
	if targetUserID == "" {
		targetUserID = admin.AdminID
	}

	// 2. Vérifier permissions
	if targetUserID != admin.AdminID {
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("target_user_id", targetUserID).
				Msg("❌ Permission refusée")
			return nil, errors.New("insufficient permissions: can only revoke own API keys")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Str("target_user_id", targetUserID).
			Msg("ℹ️ Admin révoque toutes les clés d'un autre user")
	}

	// 3. Compter les clés actives avant révocation
	activeCount, err := uc.apiKeyRepo.CountActiveByUserID(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("count active keys: %w", err)
	}

	// 4. Raison par défaut
	reason := req.Reason
	if reason == "" {
		reason = "Toutes les clés révoquées par l'utilisateur"
	}

	// 5. Révoquer toutes les clés (pas d'exclusion car c'est une révocation totale)
	if err := uc.apiKeyRepo.RevokeAllUserAPIKeys(ctx, targetUserID, "", admin.AdminID, reason); err != nil {
		return nil, fmt.Errorf("revoke all api keys: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", targetUserID).
		Int("revoked_count", activeCount).
		Str("reason", reason).
		Msg("✅ Toutes les clés API révoquées")

	return &RevokeAllAPIKeysResponse{
		Success:      true,
		Message:      fmt.Sprintf("%d clé(s) API révoquée(s). Aucune clé active ne reste.", activeCount),
		RevokedCount: activeCount,
	}, nil
}

// ============================================================
// USECASE 5 : GET API KEY STATISTICS
// ============================================================

// GetAPIKeyStatsUsecase récupère les statistiques globales
type GetAPIKeyStatsUsecase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewGetAPIKeyStatsUsecase crée une nouvelle instance
func NewGetAPIKeyStatsUsecase(
	apiKeyRepo repository.APIKeyRepository,
) *GetAPIKeyStatsUsecase {
	return &GetAPIKeyStatsUsecase{
		apiKeyRepo: apiKeyRepo,
	}
}

// GetAPIKeyStatsRequest représente la requête
type GetAPIKeyStatsRequest struct {
	UserID string `json:"user_id"` // Optionnel (admin peut voir stats globales)
}

// GetAPIKeyStatsResponse représente la réponse
type GetAPIKeyStatsResponse struct {
	Success      bool                          `json:"success"`
	Message      string                        `json:"message"`
	Statistics   *entity.APIKeyStatistics      `json:"statistics"`
	MostUsedKeys []repository.APIKeyUsageCount `json:"most_used_keys,omitempty"`
	GeneratedAt  time.Time                     `json:"generated_at"`
}

// Execute récupère les statistiques
func (uc *GetAPIKeyStatsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *GetAPIKeyStatsRequest,
) (*GetAPIKeyStatsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation des permissions (admin seulement)
	if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("❌ Permission refusée : statistiques réservées aux admins")
		return nil, errors.New("insufficient permissions: admin role required")
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Msg("📊 Get API key statistics")

	// 2. Récupérer les statistiques
	var stats *entity.APIKeyStatistics
	var err error

	if req.UserID != "" {
		stats, err = uc.apiKeyRepo.GetStatisticsByUser(ctx, req.UserID)
	} else {
		stats, err = uc.apiKeyRepo.GetStatistics(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("get statistics: %w", err)
	}

	// 3. Récupérer les clés les plus utilisées (stats globales seulement)
	var mostUsed []repository.APIKeyUsageCount
	if req.UserID == "" {
		mostUsed, err = uc.apiKeyRepo.GetMostUsedKeys(ctx, 10)
		if err != nil {
			logger.Warn().Err(err).Msg("⚠️ Erreur récupération clés les plus utilisées")
		}
	}

	return &GetAPIKeyStatsResponse{
		Success:      true,
		Message:      "Statistiques récupérées avec succès",
		Statistics:   stats,
		MostUsedKeys: mostUsed,
		GeneratedAt:  time.Now(),
	}, nil
}

// ============================================================
// USECASE 6 : VALIDATE API KEY (pour middleware)
// ============================================================

// ValidateAPIKeyUsecase valide une clé API (utilisé par le middleware)
type ValidateAPIKeyUsecase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewValidateAPIKeyUsecase crée une nouvelle instance
func NewValidateAPIKeyUsecase(
	apiKeyRepo repository.APIKeyRepository,
) *ValidateAPIKeyUsecase {
	return &ValidateAPIKeyUsecase{
		apiKeyRepo: apiKeyRepo,
	}
}

// ValidateAPIKeyRequest représente la requête
type ValidateAPIKeyRequest struct {
	FullKey       string             `json:"full_key"`       // Clé complète
	RequiredScope entity.APIKeyScope `json:"required_scope"` // Scope requis
}

// ValidateAPIKeyResponse représente la réponse
type ValidateAPIKeyResponse struct {
	Valid  bool           `json:"valid"`
	APIKey *entity.APIKey `json:"api_key,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// Execute valide une clé API
func (uc *ValidateAPIKeyUsecase) Execute(
	ctx context.Context,
	req *ValidateAPIKeyRequest,
) (*ValidateAPIKeyResponse, error) {
	// 1. Validation du format
	if err := entity.ValidateKey(req.FullKey); err != nil {
		return &ValidateAPIKeyResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	// 2. Hasher la clé
	keyHash := entity.HashAPIKey(req.FullKey)

	// 3. Vérifier le scope si requis
	if req.RequiredScope != "" {
		if err := uc.apiKeyRepo.CheckScope(ctx, keyHash, req.RequiredScope); err != nil {
			return &ValidateAPIKeyResponse{
				Valid: false,
				Error: err.Error(),
			}, nil
		}
	} else {
		// Juste valider la clé
		apiKey, err := uc.apiKeyRepo.ValidateKey(ctx, keyHash)
		if err != nil {
			return &ValidateAPIKeyResponse{
				Valid: false,
				Error: err.Error(),
			}, nil
		}

		// 4. Marquer comme utilisée (non bloquant)
		go func() {
			bgCtx := context.Background()
			if err := uc.apiKeyRepo.MarkKeyUsed(bgCtx, keyHash); err != nil {
				// Non bloquant
				_ = err
			}
		}()

		return &ValidateAPIKeyResponse{
			Valid:  true,
			APIKey: apiKey,
		}, nil
	}

	// 5. Récupérer la clé
	apiKey, err := uc.apiKeyRepo.FindByHash(ctx, keyHash)
	if err != nil {
		return &ValidateAPIKeyResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	// 6. Vérifier validité
	if !apiKey.IsValid() {
		if apiKey.IsRevoked() {
			return &ValidateAPIKeyResponse{
				Valid: false,
				Error: "API key has been revoked",
			}, nil
		}
		return &ValidateAPIKeyResponse{
			Valid: false,
			Error: "API key has expired",
		}, nil
	}

	// 7. Marquer comme utilisée (non bloquant)
	go func() {
		bgCtx := context.Background()
		if err := uc.apiKeyRepo.MarkKeyUsed(bgCtx, keyHash); err != nil {
			_ = err
		}
	}()

	return &ValidateAPIKeyResponse{
		Valid:  true,
		APIKey: apiKey,
	}, nil
}
