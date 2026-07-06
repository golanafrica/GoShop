package middl

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.3 : API KEY AUTHENTICATION MIDDLEWARE
// ============================================================
//
// 🎯 Objectif :
//   Authentifier les requêtes via une clé API dans le header
//   Authorization: Bearer gsk_live_... ou X-API-Key: gsk_live_...
//
// 🔐 Flux :
//   1. Extraire la clé API du header
//   2. Hasher la clé avec SHA-256
//   3. Vérifier en base si la clé existe et est valide
//   4. Vérifier le scope requis (si spécifié)
//   5. Injecter les infos de la clé dans le contexte
//   6. Logger l'utilisation (non bloquant)
//
// 📋 Usage :
//   - Routes API publiques nécessitant une clé API
//   - Webhooks entrants
//   - Intégrations tierces
//
// ============================================================

// Context keys pour API Key
type contextKeyAPIKey string

const (
	APIKeyContextKey       contextKeyAPIKey = "api_key"
	APIKeyIDContextKey     contextKeyAPIKey = "api_key_id"
	APIKeyUserIDContextKey contextKeyAPIKey = "api_key_user_id"
	APIKeyScopesContextKey contextKeyAPIKey = "api_key_scopes"
)

// APIKeyAuthConfig configuration du middleware
type APIKeyAuthConfig struct {
	APIKeyRepo    repository.APIKeyRepository
	RequiredScope entity.APIKeyScope // Scope requis (optionnel)
}

// APIKeyAuth crée un middleware d'authentification par clé API
func APIKeyAuth(config APIKeyAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())
			start := time.Now()

			// 1. Extraire la clé API du header
			apiKey := extractAPIKey(r)
			if apiKey == "" {
				logger.Debug().Msg("❌ API Key manquante")
				utils.WriteAppError(w, utils.NewAppError(
					"API_KEY_MISSING",
					"API key is required. Use 'Authorization: Bearer gsk_live_...' or 'X-API-Key: gsk_live_...'",
					http.StatusUnauthorized,
				))
				return
			}

			// 2. Valider le format de la clé
			if err := entity.ValidateKey(apiKey); err != nil {
				logger.Debug().
					Err(err).
					Msg("❌ Format de clé API invalide")
				utils.WriteAppError(w, utils.NewAppError(
					"API_KEY_INVALID_FORMAT",
					"Invalid API key format. Must start with 'gsk_live_' or 'gsk_test_'",
					http.StatusUnauthorized,
				))
				return
			}

			// 3. Hasher la clé
			keyHash := entity.HashAPIKey(apiKey)

			// 4. Vérifier le scope requis (si spécifié)
			if config.RequiredScope != "" {
				if err := config.APIKeyRepo.CheckScope(r.Context(), keyHash, config.RequiredScope); err != nil {
					logger.Warn().
						Err(err).
						Str("required_scope", string(config.RequiredScope)).
						Msg("❌ Scope insuffisant")

					statusCode, errorCode := mapAPIKeyError(err)

					utils.WriteAppError(w, utils.NewAppError(
						errorCode,
						err.Error(),
						statusCode,
					))
					return
				}
			} else {
				// Juste valider la clé (sans vérifier le scope)
				if _, err := config.APIKeyRepo.ValidateKey(r.Context(), keyHash); err != nil {
					logger.Warn().
						Err(err).
						Msg("❌ Clé API invalide")

					statusCode, errorCode := mapAPIKeyError(err)

					utils.WriteAppError(w, utils.NewAppError(
						errorCode,
						err.Error(),
						statusCode,
					))
					return
				}
			}

			// 5. Récupérer la clé complète pour injecter dans le contexte
			apiKeyEntity, err := config.APIKeyRepo.FindByHash(r.Context(), keyHash)
			if err != nil {
				logger.Error().Err(err).Msg("❌ Erreur récupération clé API")
				utils.WriteAppError(w, utils.ErrInternalServer)
				return
			}

			// 6. Injecter les infos dans le contexte
			ctx := r.Context()
			ctx = context.WithValue(ctx, APIKeyContextKey, apiKeyEntity)
			ctx = context.WithValue(ctx, APIKeyIDContextKey, apiKeyEntity.ID)
			ctx = context.WithValue(ctx, APIKeyUserIDContextKey, apiKeyEntity.UserID)
			ctx = context.WithValue(ctx, APIKeyScopesContextKey, apiKeyEntity.Scopes)

			// 7. Logger l'utilisation (non bloquant, en goroutine)
			go func() {
				bgCtx := context.Background()

				// Marquer comme utilisée
				if err := config.APIKeyRepo.MarkKeyUsed(bgCtx, keyHash); err != nil {
					logger.Debug().Err(err).Msg("⚠️ Erreur MAJ last_used_at")
				}

				// Logger l'utilisation
				log := &entity.APIKeyUsageLog{
					APIKeyID:     apiKeyEntity.ID,
					Method:       r.Method,
					Path:         r.URL.Path,
					StatusCode:   200, // Sera mis à jour par le response writer
					IPAddress:    extractIPWithoutPort(r.RemoteAddr),
					UserAgent:    r.UserAgent(),
					ResponseTime: int(time.Since(start).Milliseconds()),
				}

				if err := config.APIKeyRepo.LogUsage(bgCtx, log); err != nil {
					logger.Debug().Err(err).Msg("⚠️ Erreur log usage")
				}
			}()

			logger.Debug().
				Str("api_key_id", apiKeyEntity.ID).
				Str("user_id", apiKeyEntity.UserID).
				Str("key_prefix", apiKeyEntity.KeyPrefix).
				Dur("auth_duration_ms", time.Since(start)).
				Msg("✅ API Key authentifiée")

			// 8. Continuer vers le handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ============================================================
// HELPER : Mapping des erreurs API Key
// ============================================================

// mapAPIKeyError convertit une erreur API Key en (statusCode, errorCode)
// Utilise un tagged switch pour une meilleure lisibilité (fix QF1003)
func mapAPIKeyError(err error) (int, string) {
	switch {
	case errors.Is(err, entity.ErrAPIKeyRevoked):
		return http.StatusGone, "API_KEY_REVOKED"
	case errors.Is(err, entity.ErrAPIKeyExpired):
		return http.StatusGone, "API_KEY_EXPIRED"
	case errors.Is(err, repository.ErrAPIKeyNotFound):
		return http.StatusUnauthorized, "API_KEY_NOT_FOUND"
	case errors.Is(err, entity.ErrAPIKeyInsufficientScope):
		return http.StatusForbidden, "API_KEY_INSUFFICIENT_SCOPE"
	default:
		return http.StatusUnauthorized, "API_KEY_INVALID"
	}
}

// ============================================================
// HELPERS
// ============================================================

// extractAPIKey extrait la clé API du header
// Supporte deux formats :
// - Authorization: Bearer gsk_live_...
// - X-API-Key: gsk_live_...
func extractAPIKey(r *http.Request) string {
	// 1. Essayer le header X-API-Key (priorité)
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return strings.TrimSpace(apiKey)
	}

	// 2. Essayer le header Authorization (format Bearer)
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}
	}

	// 3. Essayer le query param (pour webhooks, pas recommandé)
	if apiKey := r.URL.Query().Get("api_key"); apiKey != "" {
		return strings.TrimSpace(apiKey)
	}

	return ""
}

// extractIPWithoutPort extrait l'IP sans le port
func extractIPWithoutPort(addr string) string {
	if addr == "" {
		return ""
	}

	// Gérer IPv6 [::1]:port
	if strings.HasPrefix(addr, "[") {
		if idx := strings.LastIndex(addr, "]"); idx != -1 {
			return addr[1:idx]
		}
	}

	// Gérer IPv4 192.168.1.1:port
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}

	return addr
}

// ============================================================
// GETTERS POUR LE CONTEXTE
// ============================================================

// GetAPIKeyFromContext récupère l'entité APIKey depuis le contexte
func GetAPIKeyFromContext(ctx context.Context) (*entity.APIKey, bool) {
	apiKey, ok := ctx.Value(APIKeyContextKey).(*entity.APIKey)
	return apiKey, ok
}

// GetAPIKeyIDFromContext récupère l'ID de la clé API depuis le contexte
func GetAPIKeyIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(APIKeyIDContextKey).(string)
	return id, ok
}

// GetAPIKeyUserIDFromContext récupère l'ID du user propriétaire de la clé
func GetAPIKeyUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(APIKeyUserIDContextKey).(string)
	return userID, ok
}

// GetAPIKeyScopesFromContext récupère les scopes de la clé API
func GetAPIKeyScopesFromContext(ctx context.Context) ([]entity.APIKeyScope, bool) {
	scopes, ok := ctx.Value(APIKeyScopesContextKey).([]entity.APIKeyScope)
	return scopes, ok
}

// HasAPIKeyScope vérifie si la clé API a le scope requis
func HasAPIKeyScope(ctx context.Context, requiredScope entity.APIKeyScope) bool {
	scopes, ok := GetAPIKeyScopesFromContext(ctx)
	if !ok {
		return false
	}

	for _, scope := range scopes {
		if scope == entity.ScopeAdmin || scope == requiredScope {
			return true
		}
	}

	return false
}

// IsAPIKeyAuthenticated vérifie si la requête est authentifiée par API Key
func IsAPIKeyAuthenticated(ctx context.Context) bool {
	_, ok := GetAPIKeyFromContext(ctx)
	return ok
}
